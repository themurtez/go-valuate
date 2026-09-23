package management

import (
	"encoding/json"
	"testing"
)

// TestReport_JSONRoundTrip proves Report marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested section
// type.
func TestReport_JSONRoundTrip(t *testing.T) {
	report := Calculate(fullFixture())
	if !report.Available {
		t.Fatalf("expected Available, errors=%+v", report.Errors)
	}

	first, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped Report
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

// TestReport_JSONRoundTrip_Empty covers the degenerate zero-Input case,
// ensuring an unavailable Report still round-trips cleanly.
func TestReport_JSONRoundTrip_Empty(t *testing.T) {
	report := Calculate(Input{})
	if report.Available {
		t.Fatalf("expected Available == false for a zero-value Input")
	}

	first, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped Report
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

// TestReport_JSONRoundTrip_Partial covers a partially-populated Input
// (only Metrics), ensuring a mix of available and unavailable sections
// round-trips cleanly.
func TestReport_JSONRoundTrip_Partial(t *testing.T) {
	in := Input{Metrics: fullFixture().Metrics, PeriodMeta: fullFixture().PeriodMeta}
	report := Calculate(in)
	if !report.Available {
		t.Fatalf("expected Available, errors=%+v", report.Errors)
	}

	first, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped Report
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
