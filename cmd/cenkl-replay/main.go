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
	serverName := flag.String("server-name", "", "TLS certificate name when connecting through a local forwarder")
	proxy := flag.String("socks5", "", "optional local SOCKS5 proxy host:port")
	concurrency := flag.Int("concurrency", 1, "number of independent flows to send concurrently")
	caCert := flag.String("ca-cert", "work/server-cert.pem", "trusted PEM certificate for the TLS server")
	timeout := flag.Duration("timeout", 30*time.Second, "maximum time per flow")
	flag.Parse()
	if *timeout <= 0 || *concurrency < 1 {
		fmt.Fprintln(os.Stderr, "-timeout and -concurrency must be positive")
		os.Exit(2)
	}
	flows, err := replay.Load(*input)
	if err == nil {
		var cert []byte
		cert, err = os.ReadFile(*caCert)
		if err == nil {
			err = recordshape.RunClientConcurrent(flows, *endpoint, *serverName, *proxy, *timeout, cert, os.Stdout, *concurrency)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
