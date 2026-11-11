package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func main() {
	count := flag.Int("events", 100, "number of TLS record events")
	output := flag.String("output", "", "output JSON file")
	flag.Parse()
	if *count < 2 || *count > 10000 || *output == "" {
		fmt.Fprintln(os.Stderr, "use -events 2..10000 and -output PATH")
		os.Exit(2)
	}
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))
	flow := replay.Flow{ID: fmt.Sprintf("generated-%d", seed), Events: make([]replay.Event, *count)}
	for i := range flow.Events {
		direction := "client_to_server"
		if i == 1 || (i > 1 && rng.Intn(2) == 1) {
			direction = "server_to_client"
		}
		flow.Events[i] = replay.Event{Direction: direction, Size: 64 + rng.Intn(1397)}
	}
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(replay.Input{Flows: []replay.Flow{flow}}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(flow.ID)
}
