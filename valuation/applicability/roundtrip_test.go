package applicability

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/valuation/profile"
)

// TestResults_JSONRoundTrip proves applicability.Results marshals,
// unmarshals, and re-marshals to byte-identical output — the same
// Marshal-Unmarshal-Marshal pattern every sibling package's own
// roundtrip_test.go uses. This package previously had no serialization
// test coverage at all, despite Results being a JSON-serializable,
// RulesVersion-stamped pipeline output (see orchestrator.Request.Applicability
// and README.md's Versioning strategy table).
func TestResults_JSONRoundTrip(t *testing.T) {
	p := profile.Profile{
		Industry:       profile.IndustryTrades,
		OwnerOperated:  boolPtr(true),
		AnnualRevenue:  floatPtr(900_000),
		EmployeeCount:  intPtr(6),
		AssetIntensity: floatPtr(0.2),
	}
	results := Calculate(p)

	first, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped Results
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Results did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}

// TestResults_JSONRoundTrip_DCFHardBlock proves the DCF hard-block path
// (HardBlockReason, Score 0, NOT_APPLICABLE — see scoreDCF) round-trips
// correctly too, since it's a structurally different Result shape from
// every other method's ordinary scored path.
func TestResults_JSONRoundTrip_DCFHardBlock(t *testing.T) {
	results := Calculate(profile.Profile{}) // empty profile: DataAvailability.HasForecast is false

	first, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped Results
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Results (DCF hard-block case) did not round-trip byte-for-byte")
	}

	dcfResult, ok := roundTripped.ForMethod("DCF")
	if !ok {
		t.Fatal("expected a DCF result to be present after round-trip")
	}
	if dcfResult.HardBlockReason == "" {
		t.Error("expected HardBlockReason to survive round-trip for an empty Profile's DCF result")
	}
}
