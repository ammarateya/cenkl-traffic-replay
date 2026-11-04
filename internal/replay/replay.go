package replay

import (
	"crypto/tls"
	"crypto/x509"
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

const maxEventSize = 16 << 20  // Bound memory and reject implausible input.
const tls13RecordOverhead = 22 // 5-byte header, 1-byte inner content type, 16-byte AEAD tag.

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
	TargetBytes       int64
	UnsupportedEvents int
}

func Run(in Input, endpoint string, timeout time.Duration, out io.Writer) error {
	return run(in, endpoint, timeout, out, nil, false)
}

// RunTLS sends the same dummy application bytes through an authenticated TLS connection.
func RunTLS(in Input, endpoint string, timeout time.Duration, out io.Writer, caCert []byte) error {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caCert) {
		return errors.New("CA certificate must contain a PEM certificate")
	}
	return run(in, endpoint, timeout, out, roots, false)
}

// RunTLSTarget treats each event size as a target TCP payload length for one TLS 1.3 record.
// The observed packetization still must be verified with a packet capture.
func RunTLSTarget(in Input, endpoint string, timeout time.Duration, out io.Writer, caCert []byte) error {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caCert) {
		return errors.New("CA certificate must contain a PEM certificate")
	}
	for _, flow := range in.Flows {
		for i, event := range flow.Events {
			if event.Direction == "client_to_server" && (event.Size <= tls13RecordOverhead || event.Size > 16384+tls13RecordOverhead) {
				return fmt.Errorf("flow %q event %d: TLS target size must be %d..%d", flow.ID, i, tls13RecordOverhead+1, 16384+tls13RecordOverhead)
			}
		}
	}
	return run(in, endpoint, timeout, out, roots, true)
}

func run(in Input, endpoint string, timeout time.Duration, out io.Writer, roots *x509.CertPool, targetSegments bool) error {
	for _, flow := range in.Flows {
		var conn net.Conn
		var err error
		if roots == nil {
			conn, err = net.DialTimeout("tcp", endpoint, timeout)
		} else {
			config := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
			if targetSegments {
				config.MinVersion = tls.VersionTLS13
				config.MaxVersion = tls.VersionTLS13
				config.DynamicRecordSizingDisabled = true
			}
			conn, err = tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", endpoint, config)
		}
		if err != nil {
			return fmt.Errorf("flow %q: connect: %w", flow.ID, err)
		}
		result, err := runFlow(conn, flow, timeout, targetSegments)
		closeErr := conn.Close()
		if err != nil {
			return fmt.Errorf("flow %q: %w", flow.ID, err)
		}
		if closeErr != nil {
			return fmt.Errorf("flow %q: close: %w", flow.ID, closeErr)
		}
		if targetSegments {
			fmt.Fprintf(out, "%s: sent=%d application bytes, target_tcp_payload_bytes=%d, unsupported_server_events=%d\n", result.FlowID, result.BytesSent, result.TargetBytes, result.UnsupportedEvents)
		} else {
			fmt.Fprintf(out, "%s: sent=%d bytes, unsupported_server_events=%d\n", result.FlowID, result.BytesSent, result.UnsupportedEvents)
		}
	}
	return nil
}

func runFlow(conn net.Conn, flow Flow, timeout time.Duration, targetSegments bool) (Result, error) {
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
		if targetSegments {
			result.TargetBytes += int64(event.Size)
			remaining -= tls13RecordOverhead
		}
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
