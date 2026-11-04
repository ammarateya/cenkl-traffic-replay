package replay

import (
	"reflect"
	"testing"
)

func TestGenerateDeterministicAndBounded(t *testing.T) {
	first, err := Generate(3, 8, 100, 200, 42)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(3, 8, 100, 200, 42)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same seed produced different flows")
	}
	for _, flow := range first.Flows {
		if flow.Events[0].Direction != "client_to_server" {
			t.Fatal("flow has no guaranteed client write")
		}
		for _, event := range flow.Events {
			if event.Size < 100 || event.Size > 200 {
				t.Fatalf("size out of range: %d", event.Size)
			}
		}
	}
}

func TestGenerateRejectsInvalidRange(t *testing.T) {
	if _, err := Generate(1, 1, 200, 100, 1); err == nil {
		t.Fatal("expected invalid range error")
	}
}
