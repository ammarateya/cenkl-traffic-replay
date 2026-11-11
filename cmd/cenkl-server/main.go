package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/recordshape"
	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9000", "address to listen on")
	cert := flag.String("cert", "work/server-cert.pem", "TLS certificate PEM file")
	key := flag.String("key", "work/server-key.pem", "TLS private key PEM file")
	input := flag.String("input", "examples/flows.json", "ordered TLS record flow JSON")
	timeout := flag.Duration("timeout", 30*time.Second, "maximum time per flow")
	flag.Parse()
	if *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "-timeout must be positive")
		os.Exit(2)
	}
	flows, err := replay.Load(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	pair, err := tls.LoadX509KeyPair(*cert, *key)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer listener.Close()
	fmt.Printf("TLS record replay server listening on %s\n", listener.Addr())
	config := &tls.Config{
		Certificates: []tls.Certificate{pair},
		MinVersion:   tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		DynamicRecordSizingDisabled: true, SessionTicketsDisabled: true,
	}
	for _, flow := range flows.Flows {
		raw, err := listener.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := recordshape.ServeFlow(raw, config, flow, *timeout, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
