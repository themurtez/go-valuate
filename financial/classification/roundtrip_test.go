package classification

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestResult_JSONRoundTrip proves a classification Result (and a
// []Result, as ClassifyBatch returns) marshals, unmarshals, and
// re-marshals to byte-identical output.
func TestResult_JSONRoundTrip(t *testing.T) {
	raws := []financial.RawLineItem{
		{ID: "row-1", StatementType: financial.StatementIncomeStatement, Label: "Advertising & Promotion", Values: map[financial.Period]float64{"2025": 42000}},
		{ID: "row-2", StatementType: financial.StatementIncomeStatement, Label: "Total Operating Expenses", Values: map[financial.Period]float64{"2025": 250000}},
	}
	cfg := Config{Rules: DefaultRules()}
	results := ClassifyBatch(raws, cfg)

	first, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped []Result
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("[]Result did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}
