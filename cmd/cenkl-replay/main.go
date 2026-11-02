package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func main() {
	input := flag.String("input", "", "path to flow JSON")
	endpoint := flag.String("endpoint", "127.0.0.1:9000", "TCP host:port")
	timeout := flag.Duration("timeout", 5*time.Second, "connect and write timeout")
	flag.Parse()
	if *input == "" || *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "-input is required and -timeout must be positive")
		os.Exit(2)
	}
	flows, err := replay.Load(*input)
	if err == nil {
		err = replay.Run(flows, *endpoint, *timeout, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
