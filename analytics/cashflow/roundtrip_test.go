package cashflow

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (History, Conversion, Trend, RecurringDrains, CashRunway, Flags) —
// mirroring analytics/workingcapital/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")
	meta := threeYearMeta()

	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: meta,
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2024": Reported(300_000),
			"2025": Reported(320_000),
		},
		Capex: map[financial.Period]CashFlowValue{
			"2025": Reported(40_000),
		},
		DebtService: map[financial.Period]DebtServiceFigure{
			"2025": {Principal: Reported(50_000), Interest: Reported(10_000)},
		},
		OwnerDistributions: map[financial.Period]CashFlowValue{
			"2025": Reported(100_000),
		},
		CashBalance: map[financial.Period]CashFlowValue{
			"2025": Reported(200_000),
		},
	}, Options{AllowEBITDAEstimate: true})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

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

// TestResult_JSONRoundTrip_Unavailable proves the zero-history
// Available == false shape also round-trips cleanly.
func TestResult_JSONRoundTrip_Unavailable(t *testing.T) {
	res := Calculate(Input{}, Options{})
	first, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
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
		t.Fatal("unavailable Result did not round-trip byte-for-byte")
	}
}
