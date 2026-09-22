package consensus

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, including basis
// conversion (Conversions/BasisExclusions ordering) and Statistics.
func TestCalculate_Deterministic(t *testing.T) {
	inputs := []Input{
		{
			Method: valuation.CodeEBITDAMultiple, Value: 1_800_000, ValueType: valuation.ValueTypeEnterprise,
			Bridge: valuation.Bridge{Available: true, EnterpriseValue: 1_800_000, ExcessCash: 100_000, TotalDebt: 300_000, EquityValue: 1_600_000},
			Weight: 1,
		},
		{Method: valuation.CodeSDEMultiple, Value: 900_000, ValueType: valuation.ValueTypeEquity, Weight: 1},
		{Method: valuation.CodeAdjustedNetAssetValue, Value: 1_100_000, ValueType: valuation.ValueTypeAsset, Weight: 1},
	}
	opts := Options{TargetBasis: valuation.ValueTypeEquity}

	first, err := json.Marshal(Calculate(inputs, opts))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(inputs, opts))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}
