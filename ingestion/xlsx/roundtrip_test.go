package xlsx_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/ingestion"
)

// roundTripJSON marshals res to JSON and back, then re-marshals the
// result, asserting the two JSON encodings are byte-identical — a cheap
// way to confirm the shape serializes and deserializes without loss.
func roundTripJSON(t *testing.T, res *ingestion.Result) {
	t.Helper()

	first, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded ingestion.Result
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	second, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("JSON round-trip mismatch:\nfirst:  %s\nsecond: %s", first, second)
	}
}
