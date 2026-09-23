package consolidation

import (
	"encoding/json"
	"testing"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (Consolidated, EntityContributions, EliminationsApplied,
// CurrencyConversions, ReconciliationIssues) — mirroring
// concentration/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	in := Input{
		Entities: []EntityDataset{
			{EntityID: "parent", EntityLabel: "Parent Co", Dataset: parentUSD()},
			{EntityID: "sub", EntityLabel: "Subsidiary Co", Dataset: subsidiaryEUR(), OwnershipPercent: ptr(0.75)},
		},
		Periods:       twoYearPeriods(),
		CurrencyRates: eurToUSDRates(),
		Eliminations:  managementFeeEliminations(),
		Policy:        Policy{TargetCurrency: "USD"},
	}
	res := Calculate(in)
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

// TestResult_JSONRoundTrip_OwnershipWeighted covers the ModeOwnershipWeighted
// shape specifically, since it populates OwnershipPercent pointers Calculate
// otherwise leaves nil under ModeFullConsolidation.
func TestResult_JSONRoundTrip_OwnershipWeighted(t *testing.T) {
	in := twoEntityUSDInput()
	in.Mode = ModeOwnershipWeighted
	in.Entities[0].OwnershipPercent = ptr(1.0)
	in.Entities[1].OwnershipPercent = ptr(0.6)

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

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
		t.Fatal("ownership-weighted Result did not round-trip byte-for-byte")
	}
}

// TestResult_JSONRoundTrip_Unavailable proves the zero-consolidated
// Available == false shape also round-trips cleanly.
func TestResult_JSONRoundTrip_Unavailable(t *testing.T) {
	res := Calculate(Input{})
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
