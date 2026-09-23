package diagnostics

import (
	"encoding/json"
	"testing"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (findings, counts, coverage, policy, issues).
func TestResult_JSONRoundTrip(t *testing.T) {
	res := Calculate(fullFixture())
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.Findings) == 0 {
		t.Fatalf("expected at least one finding to exercise Finding's JSON shape")
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
		t.Fatalf("round-trip mismatch:\nfirst:  %s\nsecond: %s", first, second)
	}
}

// TestResult_JSONRoundTrip_Empty covers the degenerate zero-Input case,
// ensuring an unavailable Result still round-trips cleanly.
func TestResult_JSONRoundTrip_Empty(t *testing.T) {
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
		t.Fatalf("round-trip mismatch:\nfirst:  %s\nsecond: %s", first, second)
	}
}

// TestBusinessSnapshot_JSONRoundTrip_WithPrior proves a BusinessSnapshot
// carrying a nested Prior pointer round-trips cleanly, including the
// nested Prior's own summaries.
func TestBusinessSnapshot_JSONRoundTrip_WithPrior(t *testing.T) {
	b := decliningBusiness("rt1")

	first, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped BusinessSnapshot
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if roundTripped.Prior == nil {
		t.Fatalf("expected Prior to survive round-trip")
	}
	if roundTripped.Prior.Metrics.Revenue != b.Prior.Metrics.Revenue {
		t.Fatalf("Prior.Metrics.Revenue mismatch after round-trip: got %+v, want %+v", roundTripped.Prior.Metrics.Revenue, b.Prior.Metrics.Revenue)
	}

	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("round-trip mismatch:\nfirst:  %s\nsecond: %s", first, second)
	}
}
