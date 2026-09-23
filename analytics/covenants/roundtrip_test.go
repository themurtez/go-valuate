package covenants

import (
	"encoding/json"
	"testing"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (tests, headroom, warning-buffer status, cure/grace, summary, issues) —
// mirroring analytics/debt/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	in := Input{Tests: []CovenantTest{
		{
			CovenantID:           "MIN_DSCR",
			Label:                "Minimum DSCR",
			Metric:               MetricDSCR,
			Operator:             OperatorGTE,
			Threshold:            1.25,
			Actual:               AvailableValue(1.30),
			Period:               "2025-Q3",
			WarningBufferPercent: 0.10,
			CureGrace:            &CureGrace{Description: "10 business days", CureDays: 10, GraceDays: 5},
		},
		{
			CovenantID: "MAX_LEVERAGE",
			Metric:     MetricDebtToEBITDA,
			Operator:   OperatorLTE,
			Threshold:  4.0,
			Actual:     AvailableValue(4.5),
			Period:     "2025-Q3",
		},
		{
			CovenantID: "MIN_NET_WORTH",
			Metric:     MetricMinimumNetWorth,
			Operator:   OperatorGTE,
			Threshold:  1_000_000,
			Actual:     Unavailable(),
			Period:     "2025-Q3",
		},
	}}
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
