package classification

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestClassifyBatch_Deterministic proves repeated execution against
// identical input (raw rows + config) returns byte-for-byte identical
// output — no rule/alias iteration order, map traversal, or other hidden
// nondeterminism leaks into Result.
func TestClassifyBatch_Deterministic(t *testing.T) {
	raws := []financial.RawLineItem{
		{ID: "row-1", StatementType: financial.StatementIncomeStatement, Label: "Advertising & Promotion", Values: map[financial.Period]float64{"2025": 42000}},
		{ID: "row-2", StatementType: financial.StatementIncomeStatement, Label: "Total Operating Expenses", Values: map[financial.Period]float64{"2025": 250000}},
		{ID: "row-3", StatementType: financial.StatementIncomeStatement, Label: "Some Ambiguous Line", Values: map[financial.Period]float64{"2025": 1000}},
	}
	cfg := Config{
		AliasLayers: []AliasLayer{
			{Name: "global", Aliases: []Alias{{Label: "Advertising & Promotion", Code: financial.CodeOpexMarketing}}},
		},
		Rules: DefaultRules(),
	}

	first, err := json.Marshal(ClassifyBatch(raws, cfg))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(ClassifyBatch(raws, cfg))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: ClassifyBatch output differs from the first run", i)
		}
	}
}
