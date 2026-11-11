package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompareLogChecksEveryTurn(t *testing.T) {
	flow, clients, servers := generateFlow(100, 42)
	if clients+servers != 100 || clients == 0 || servers == 0 {
		t.Fatal("expected both directions")
	}
	var log strings.Builder
	for i, event := range flow.Flows[0].Events {
		fmt.Fprintf(&log, "server-1 | %s event=%d direction=%s expected_record=%d observed_record=%d\n", flow.Flows[0].ID, i+1, event.Direction, event.Size, event.Size)
	}
	log.WriteString("server-1 | verified 100 TLS records\n")
	if got, err := compareLog(flow, log.String()); err != nil || got != 100 {
		t.Fatalf("matched=%d error=%v", got, err)
	}
	altered := strings.Replace(log.String(), "observed_record=", "observed_record=9", 1)
	if _, err := compareLog(flow, altered); err == nil {
		t.Fatal("expected mismatch")
	}
}
