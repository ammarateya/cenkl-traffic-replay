package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"os"
	"sync"
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
	flowByID := make(map[string]replay.Flow, len(flows.Flows))
	for _, flow := range flows.Flows {
		if _, exists := flowByID[flow.ID]; exists {
			fmt.Fprintln(os.Stderr, "duplicate flow ID:", flow.ID)
			os.Exit(1)
		}
		flowByID[flow.ID] = flow
	}
	config.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		for _, proto := range hello.SupportedProtos {
			if len(proto) >= len("cenkl-") && proto[:len("cenkl-")] == "cenkl-" {
				if _, ok := flowByID[proto[len("cenkl-"):]]; ok {
					selected := config.Clone()
					selected.GetConfigForClient = nil
					selected.NextProtos = []string{proto}
					return selected, nil
				}
			}
		}
		return nil, fmt.Errorf("unknown flow ALPN")
	}
	completed := make(map[string]bool, len(flows.Flows))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for {
		raw, err := listener.Accept()
		if err != nil {
			mu.Lock()
			allDone := len(completed) == len(flows.Flows)
			mu.Unlock()
			if allDone {
				break
			}
			fmt.Fprintln(os.Stderr, err)
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := recordshape.ServeSelectedFlow(raw, config, flowByID, *timeout, os.Stdout)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fmt.Fprintf(os.Stderr, "flow %q attempt failed: %v\n", id, err)
				return
			}
			completed[id] = true
			if len(completed) == len(flows.Flows) {
				listener.Close()
			}
		}()
	}
	wg.Wait()
}
