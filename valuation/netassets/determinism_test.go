package netassets

import (
	"encoding/json"
	"testing"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output.
func TestCalculate_Deterministic(t *testing.T) {
	in := Input{
		Assets: []AssetItem{
			{Label: "Cash", Amount: 238000},
			{Label: "Fixed Assets (net book value)", Amount: 1595000},
			{Label: "Fixed Assets (appraisal fair-value adjustment)", Amount: 405000, IsOverride: true},
		},
		Liabilities: []LiabilityItem{{Label: "Long-Term Debt", Amount: 825000}},
	}
	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}
