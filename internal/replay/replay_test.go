package replay

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"flows":[{"id":"x","events":[{"direction":"client_to_server","size":0}]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected size validation error")
	}
}

func TestRunSendsExactBytesAndReportsUnsupported(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	received := make(chan int, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			received <- -1
			return
		}
		defer conn.Close()
		n, _ := io.Copy(io.Discard, conn)
		received <- int(n)
	}()
	var out strings.Builder
	in := Input{Flows: []Flow{{ID: "test", Events: []Event{{Direction: "client_to_server", Size: 1000}, {Direction: "server_to_client", Size: 1400}, {Direction: "client_to_server", Size: 500}}}}}
	if err := Run(in, ln.Addr().String(), time.Second, &out); err != nil {
		t.Fatal(err)
	}
	if got := <-received; got != 1500 {
		t.Fatalf("received %d bytes", got)
	}
	if !strings.Contains(out.String(), "sent=1500 bytes, unsupported_server_events=1") {
		t.Fatal(out.String())
	}
}
