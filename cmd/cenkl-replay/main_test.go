package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func TestSaveFlowsRecordsListAndProtectsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated.json")
	in, err := replay.Generate(2, 3, 10, 20, 42)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveFlows(path, in); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got replay.Input
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Flows) != 2 || len(got.Flows[0].Events) != 3 {
		t.Fatalf("unexpected saved flow list: %+v", got)
	}
	if err := saveFlows(path, in); err == nil {
		t.Fatal("existing file should not be overwritten")
	}
}
