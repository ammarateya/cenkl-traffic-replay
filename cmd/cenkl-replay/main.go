package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/recordshape"
	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func main() {
	input := flag.String("input", "examples/flows.json", "ordered TLS record flow JSON")
	endpoint := flag.String("endpoint", "127.0.0.1:9000", "TLS server host:port")
	caCert := flag.String("ca-cert", "work/server-cert.pem", "trusted PEM certificate for the TLS server")
	timeout := flag.Duration("timeout", 30*time.Second, "maximum time per flow")
	flag.Parse()
	if *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "-timeout must be positive")
		os.Exit(2)
	}
	flows, err := replay.Load(*input)
	if err == nil {
		var cert []byte
		cert, err = os.ReadFile(*caCert)
		if err == nil {
			err = recordshape.RunClient(flows, *endpoint, *timeout, cert, os.Stdout)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
