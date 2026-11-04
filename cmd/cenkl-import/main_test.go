package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ammarateya/cenkl-traffic-replay/internal/replay"
)

func TestConvertSignedPacketSizes(t *testing.T) {
	var out strings.Builder
	if err := convert(strings.NewReader("1000,-1400,500\n-80,200\n"), &out); err != nil {
		t.Fatal(err)
	}
	var got replay.Input
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Flows) != 2 || got.Flows[0].Events[1] != (replay.Event{Direction: "server_to_client", Size: 1400}) || got.Flows[1].Events[0].Size != 80 {
		t.Fatalf("unexpected flows: %+v", got)
	}
}

func TestConvertRejectsNonIntegerOrZeroInput(t *testing.T) {
	for _, input := range []string{"0.25,100\n", "0,100\n", "\n"} {
		if err := convert(strings.NewReader(input), &strings.Builder{}); err == nil {
			t.Fatalf("accepted invalid CSV %q", input)
		}
	}
}
