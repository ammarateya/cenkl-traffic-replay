package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

type Event struct {
	Direction string `json:"direction"`
	Size      int    `json:"size"`
}
type Flow struct {
	ID     string  `json:"id"`
	Events []Event `json:"events"`
}
type Input struct {
	Flows []Flow `json:"flows"`
}

const maxEventSize = 16 << 20 // Bound memory and reject implausible input.

func Load(path string) (Input, error) {
	f, err := os.Open(path)
	if err != nil {
		return Input{}, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var in Input
	if err := dec.Decode(&in); err != nil {
		return Input{}, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Input{}, errors.New("input must contain exactly one JSON object")
	}
	if len(in.Flows) == 0 {
		return Input{}, errors.New("at least one flow is required")
	}
	for i, flow := range in.Flows {
		if flow.ID == "" || len(flow.Events) == 0 {
			return Input{}, fmt.Errorf("flow %d needs an id and events", i)
		}
		for j, event := range flow.Events {
			if event.Direction != "client_to_server" && event.Direction != "server_to_client" {
				return Input{}, fmt.Errorf("flow %q event %d: invalid direction", flow.ID, j)
			}
			if event.Size <= 0 || event.Size > maxEventSize {
				return Input{}, fmt.Errorf("flow %q event %d: size must be 1..%d", flow.ID, j, maxEventSize)
			}
		}
	}
	return in, nil
}

type Result struct {
	FlowID            string
	BytesSent         int64
	UnsupportedEvents int
}

func Run(in Input, endpoint string, timeout time.Duration, out io.Writer) error {
	for _, flow := range in.Flows {
		conn, err := net.DialTimeout("tcp", endpoint, timeout)
		if err != nil {
			return fmt.Errorf("flow %q: connect: %w", flow.ID, err)
		}
		result, err := runFlow(conn, flow, timeout)
		closeErr := conn.Close()
		if err != nil {
			return fmt.Errorf("flow %q: %w", flow.ID, err)
		}
		if closeErr != nil {
			return fmt.Errorf("flow %q: close: %w", flow.ID, closeErr)
		}
		fmt.Fprintf(out, "%s: sent=%d bytes, unsupported_server_events=%d\n", result.FlowID, result.BytesSent, result.UnsupportedEvents)
	}
	return nil
}

func runFlow(conn net.Conn, flow Flow, timeout time.Duration) (Result, error) {
	result := Result{FlowID: flow.ID}
	buf := make([]byte, 32*1024)
	for _, event := range flow.Events {
		if event.Direction == "server_to_client" {
			result.UnsupportedEvents++
			continue
		}
		if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
			return result, err
		}
		remaining := event.Size
		for remaining > 0 {
			n := remaining
			if n > len(buf) {
				n = len(buf)
			}
			written, err := conn.Write(buf[:n])
			result.BytesSent += int64(written)
			remaining -= written
			if err != nil {
				return result, err
			}
			if written == 0 {
				return result, io.ErrShortWrite
			}
		}
	}
	return result, nil
}
