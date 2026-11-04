package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func main() {
	input := flag.String("input", "", "path to flow JSON")
	random := flag.Bool("random", false, "generate random sizes and directions instead of reading input")
	flowCount := flag.Int("flows", 1, "number of random flows")
	events := flag.Int("events", 10, "events per random flow")
	minSize := flag.Int("min-size", 100, "smallest random event size in bytes")
	maxSize := flag.Int("max-size", 1500, "largest random event size in bytes")
	seed := flag.Int64("seed", 0, "random seed; 0 uses current time")
	save := flag.String("save", "", "save generated flow list as JSON")
	endpoint := flag.String("endpoint", "127.0.0.1:9000", "TCP host:port")
	timeout := flag.Duration("timeout", 5*time.Second, "connect and write timeout")
	flag.Parse()
	if *timeout <= 0 || (*random == (*input != "")) || (*save != "" && !*random) {
		fmt.Fprintln(os.Stderr, "choose exactly one of -input or -random; -save requires -random; -timeout must be positive")
		os.Exit(2)
	}
	var flows replay.Input
	var err error
	if *random {
		if *seed == 0 {
			*seed = time.Now().UnixNano()
		}
		flows, err = replay.Generate(*flowCount, *events, *minSize, *maxSize, *seed)
		if err == nil {
			fmt.Fprintf(os.Stdout, "random seed=%d\n", *seed)
		}
		if err == nil && *save != "" {
			var data []byte
			data, err = json.MarshalIndent(flows, "", "  ")
			if err == nil {
				data = append(data, '\n')
				err = os.WriteFile(*save, data, 0600)
			}
		}
	} else {
		flows, err = replay.Load(*input)
	}
	if err == nil {
		err = replay.Run(flows, *endpoint, *timeout, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
