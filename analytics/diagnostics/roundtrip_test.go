package diagnostics

import (
	"encoding/json"
	"testing"
)

func assertJSONRoundTrip(t *testing.T, res Result) {
	t.Helper()
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

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output for both the healthy and stressed
// fixtures, exercising every nested type (findings, strengths, concerns,
// opportunities, missing data areas, coverage, score, issues).
func TestResult_JSONRoundTrip(t *testing.T) {
	for name, in := range map[string]Input{"healthy": healthyFixture(), "stressed": stressedFixture()} {
		t.Run(name, func(t *testing.T) {
			res := Calculate(in)
			if !res.Available {
				t.Fatalf("expected Available, errors=%+v", res.Errors)
			}
			assertJSONRoundTrip(t, res)
		})
	}
}

// TestResult_JSONRoundTrip_Empty covers the degenerate zero-Input case,
// ensuring an unavailable Result still round-trips cleanly.
func TestResult_JSONRoundTrip_Empty(t *testing.T) {
	assertJSONRoundTrip(t, Calculate(Input{}))
}

// TestValue_JSONRoundTrip proves the package's own Value type round-trips
// for both the available and unavailable cases.
func TestValue_JSONRoundTrip(t *testing.T) {
	for _, v := range []Value{Unavailable(), AvailableValue(0), AvailableValue(-42.5), AvailableValue(1_000_000)} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var got Value
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		if got != v {
			t.Fatalf("round-trip mismatch: got %+v, want %+v", got, v)
		}
	}
}
