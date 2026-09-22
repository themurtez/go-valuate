package financial

import (
	"encoding/json"
	"testing"
)

// TestFinancialDataset_JSONRoundTrip proves FinancialDataset marshals,
// unmarshals, and re-marshals to byte-identical output.
func TestFinancialDataset_JSONRoundTrip(t *testing.T) {
	ds := FinancialDataset{
		Currency: "USD",
		Items: []NormalizedItem{
			{Code: CodeRevProduct, Period: "2025", Amount: 1000000, Sources: []SourceRef{
				{RowID: "row-1", Label: "Product Sales", Period: "2025", Amount: 1000000},
			}},
			{Code: CodeCogsMaterial, Period: "2025", Amount: 400000},
		},
	}

	first, err := json.Marshal(ds)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped FinancialDataset
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("FinancialDataset did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}
