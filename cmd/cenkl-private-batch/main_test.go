package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func TestRunCollectsInterleavedConnections(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "in.tsv"), filepath.Join(dir, "out.json")
	data := "conn_id\tidx\ttimestamp\tdirection\tsize\na\t0\t1.0\t1\t100\nb\t0\t1.1\t0\t19\na\t1\t1.2\t0\t20\n"
	if err := os.WriteFile(input, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(input, output, 0, 1); err != nil {
		t.Fatal(err)
	}
	got, err := replay.Load(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Flows) != 1 || got.Flows[0].ID != "a" || len(got.Flows[0].Events) != 2 || got.Flows[0].Events[1].Size != 25 {
		t.Fatalf("wrong batch: %+v", got)
	}
}

func TestRunSkipsUnsupportedConnection(t *testing.T) {
	dir := t.TempDir()
	input, output := filepath.Join(dir, "in.tsv"), filepath.Join(dir, "out.json")
	data := "conn_id\tidx\ttimestamp\tdirection\tsize\na\t0\t1.0\t1\t100\nb\t0\t1.1\t1\t100\na\t1\t1.2\t0\t16408\n"
	if err := os.WriteFile(input, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(input, output, 0, 2); err != nil {
		t.Fatal(err)
	}
	got, err := replay.Load(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Flows) != 1 || got.Flows[0].ID != "b" {
		t.Fatalf("wrong filtered batch: %+v", got)
	}
}
