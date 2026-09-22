package revenuequality

import (
	"encoding/json"
	"testing"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (TotalRevenueHistory, Statistics, Trend, CAGR, Volatility,
// CustomerHistory, CustomerTransitions, ConcentrationSummary, Flags) —
// mirroring workingcapital/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_saas_multi_year.json")
	meta := threeYearMeta()
	customers := []CustomerPeriodRevenue{
		{CustomerKey: "alpha", Period: "2023", Amount: 40000, Segment: "enterprise", RecurringFlag: boolPtr(true)},
		{CustomerKey: "beta", Period: "2023", Amount: 20000, Segment: "smb"},
		{CustomerKey: "alpha", Period: "2024", Amount: 35000, Segment: "enterprise", RecurringFlag: boolPtr(true)},
		{CustomerKey: "gamma", Period: "2024", Amount: 25000, Segment: "smb"},
		{CustomerKey: "alpha", Period: "2025", Amount: 30000, Segment: "enterprise", RecurringFlag: boolPtr(true)},
	}

	res := Calculate(Input{
		Dataset:         ds,
		PeriodMeta:      meta,
		CustomerRevenue: customers,
	}, Options{})
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
