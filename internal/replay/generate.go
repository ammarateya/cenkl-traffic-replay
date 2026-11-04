package replay

import (
	"fmt"
	"math/rand"
)

// Generate creates a reproducible synthetic shape. The first event of each flow
// is client-to-server so every generated flow sends at least one write.
func Generate(flowCount, eventsPerFlow, minSize, maxSize int, seed int64) (Input, error) {
	if flowCount < 1 || flowCount > 10000 || eventsPerFlow < 1 || eventsPerFlow > 10000 {
		return Input{}, fmt.Errorf("flows and events must each be between 1 and 10000")
	}
	if minSize < 1 || maxSize > maxEventSize || minSize > maxSize {
		return Input{}, fmt.Errorf("size range must be within 1..%d", maxEventSize)
	}
	rng := rand.New(rand.NewSource(seed))
	in := Input{Flows: make([]Flow, flowCount)}
	for i := range in.Flows {
		flow := &in.Flows[i]
		flow.ID = fmt.Sprintf("random-%d", i+1)
		flow.Events = make([]Event, eventsPerFlow)
		for j := range flow.Events {
			direction := "client_to_server"
			if j > 0 && rng.Intn(2) == 1 {
				direction = "server_to_client"
			}
			flow.Events[j] = Event{Direction: direction, Size: minSize + rng.Intn(maxSize-minSize+1)}
		}
	}
	return in, nil
}
