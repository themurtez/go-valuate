package benchmarks

import (
	"encoding/json"
	"testing"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (comparisons, range, band, favorable, source provenance, summary,
// issues) — mirroring analytics/covenants/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	in := Input{Metrics: []MetricRequest{
		{
			MetricID:     "GROSS_MARGIN",
			Label:        "Gross Margin %",
			CompanyValue: AvailableValue(0.42),
			Period:       "2025-Q3",
			Direction:    DirectionHigherIsBetter,
			Benchmark: BenchmarkSet{
				Form: FormPercentileBands,
				PercentileBands: []PercentilePoint{
					{Percentile: 10, Value: 0.10},
					{Percentile: 50, Value: 0.30},
					{Percentile: 90, Value: 0.50},
				},
				IndustryLabel:  "Manufacturing",
				SizeLabel:      "$10M-$50M",
				GeographyLabel: "US",
				Source:         BenchmarkSource{Name: "Peer Study", EffectiveDate: "2025-Q2", Population: "SMB manufacturers", SampleSize: 84, SourceID: "SRC-1"},
			},
		},
		{
			MetricID:     "OWNER_COMP_RATIO",
			CompanyValue: Unavailable(),
			Benchmark:    BenchmarkSet{Form: FormMedian, Median: AvailableValue(0.08), Source: BenchmarkSource{Name: "X"}},
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
