package main

import (
	"strings"
	"testing"
)

func TestConvertGroupsAndAddsTLSHeader(t *testing.T) {
	input := "conn_id\tidx\ttimestamp\tdirection\tsize\n" +
		"a\t0\t1.100\t0\t19\n" +
		"b\t0\t1.200\t1\t100\n" +
		"a\t1\t1.300\t1\t24\n"
	got, err := convert(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Flows) != 2 || got.Flows[0].ID != "a" || got.Flows[0].Events[0].Size != 24 || got.Flows[0].Events[0].Direction != "server_to_client" || got.Flows[0].Events[0].Timestamp != "1.100" || got.Flows[0].Events[1].Size != 29 || got.Flows[1].Events[0].Size != 105 {
		t.Fatalf("unexpected conversion: %+v", got)
	}
	if _, err := convert(strings.NewReader(strings.Replace(input, "a\t1", "a\t2", 1))); err == nil {
		t.Fatal("expected out-of-order index error")
	}
}
