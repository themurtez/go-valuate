package workingcapital

import (
	"encoding/json"
	"testing"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (History, Statistics, Trend, SeasonalProfile, SuggestedPeg,
// PegComparison) — mirroring analytics/qoe/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")
	meta := threeYearMeta()

	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: meta,
		AsOf:       "2025",
	}, Options{PegMethod: PegMethodSimpleAverage})
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
