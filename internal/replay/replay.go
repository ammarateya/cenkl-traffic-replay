package replay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type Event struct {
	Direction string `json:"direction"`
	Size      int    `json:"size"`
	Timestamp string `json:"timestamp,omitempty"`
}

type Flow struct {
	ID     string  `json:"id"`
	Events []Event `json:"events"`
}

type Input struct {
	Flows []Flow `json:"flows"`
}

// Load reads a list of ordered TLS record sizes and directions.
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
			if event.Size < 23 || event.Size > 16406 {
				return Input{}, fmt.Errorf("flow %q event %d: full TLS 1.3 record size must be 23..16406", flow.ID, j)
			}
		}
	}
	return in, nil
}
