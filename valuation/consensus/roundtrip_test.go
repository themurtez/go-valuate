package consensus

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

// TestResult_JSONRoundTrip proves consensus.Result marshals, unmarshals,
// and re-marshals to byte-identical output, including the new
// Conversions/BasisExclusions ([]basis.Conversion) fields.
func TestResult_JSONRoundTrip(t *testing.T) {
	res := Calculate([]Input{
		{
			Method: valuation.CodeEBITDAMultiple, Value: 1_800_000, ValueType: valuation.ValueTypeEnterprise,
			Bridge: valuation.Bridge{Available: true, EnterpriseValue: 1_800_000, ExcessCash: 100_000, TotalDebt: 300_000, EquityValue: 1_600_000},
			Weight: 1,
		},
		{Method: valuation.CodeSDEMultiple, Value: 900_000, ValueType: valuation.ValueTypeEquity, Weight: 1},
	}, Options{TargetBasis: valuation.ValueTypeEquity})

	first, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped Result
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Result did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}
