package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ammarateya/cenkl-traffic-replay/internal/recordshape"
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
	save := flag.String("save", "", "path for generated flow JSON; default is generated-flows-<seed>.json")
	useTLS := flag.Bool("tls", false, "send over TLS")
	targetSegments := flag.Bool("target-segments", false, "treat sizes as target TCP payload lengths with TLS 1.3")
	tlsRecords := flag.Bool("tls-records", false, "replay and verify full TLS 1.3 record sizes in both directions")
	caCert := flag.String("ca-cert", "", "trusted PEM certificate for the TLS server")
	endpoint := flag.String("endpoint", "127.0.0.1:9000", "TCP host:port")
	timeout := flag.Duration("timeout", 5*time.Second, "connect and write timeout")
	flag.Parse()
	if *timeout <= 0 || (*random == (*input != "")) || (*save != "" && !*random) || (*useTLS != (*caCert != "")) || (*targetSegments && !*useTLS) || (*tlsRecords && !*useTLS) || (*tlsRecords && *targetSegments) {
		fmt.Fprintln(os.Stderr, "choose exactly one of -input or -random; -save requires -random; -tls requires -ca-cert; -tls-records and -target-segments require -tls and cannot be combined; -timeout must be positive")
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
		if err == nil {
			if *save == "" {
				*save = fmt.Sprintf("generated-flows-%d.json", *seed)
			}
			err = saveFlows(*save, flows)
			if err == nil {
				fmt.Fprintf(os.Stdout, "saved flow list=%s\n", *save)
			}
		}
	} else {
		flows, err = replay.Load(*input)
	}
	if err == nil {
		if *useTLS {
			var cert []byte
			cert, err = os.ReadFile(*caCert)
			if err == nil {
				if *tlsRecords {
					err = recordshape.RunClient(flows, *endpoint, *timeout, cert, os.Stdout)
				} else if *targetSegments {
					err = replay.RunTLSTarget(flows, *endpoint, *timeout, os.Stdout, cert)
				} else {
					err = replay.RunTLS(flows, *endpoint, *timeout, os.Stdout, cert)
				}
			}
		} else {
			err = replay.Run(flows, *endpoint, *timeout, os.Stdout)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func saveFlows(path string, flows replay.Input) error {
	data, err := json.MarshalIndent(flows, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("flow file already exists: %s", path)
		}
		return err
	}
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
