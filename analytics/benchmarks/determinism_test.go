package benchmarks

import (
	"encoding/json"
	"testing"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// analytics/covenants/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	in := Input{Metrics: []MetricRequest{
		{
			MetricID:     "GROSS_MARGIN",
			Label:        "Gross Margin %",
			CompanyValue: AvailableValue(0.42),
			Period:       "2025-Q3",
			Direction:    DirectionHigherIsBetter,
			Benchmark: BenchmarkSet{
				Form:           FormPercentileBands,
				IndustryLabel:  "Manufacturing",
				SizeLabel:      "$10M-$50M",
				GeographyLabel: "US",
				PercentileBands: []PercentilePoint{
					{Percentile: 90, Value: 0.50},
					{Percentile: 10, Value: 0.10},
					{Percentile: 50, Value: 0.30},
					{Percentile: 75, Value: 0.40},
					{Percentile: 25, Value: 0.20},
				},
				Source: BenchmarkSource{Name: "Peer Study", EffectiveDate: "2025-Q2", Population: "SMB manufacturers", SampleSize: 84},
			},
		},
		{
			MetricID:     "DSO",
			CompanyValue: AvailableValue(45),
			Period:       "2025-Q3",
			Direction:    DirectionLowerIsBetter,
			Benchmark: BenchmarkSet{
				Form:      FormQuartiles,
				Quartiles: Quartiles{Q1: AvailableValue(30), Median: AvailableValue(40), Q3: AvailableValue(55)},
				Source:    BenchmarkSource{Name: "Trade Association"},
			},
		},
		{
			MetricID:     "REVENUE_PER_EMPLOYEE",
			CompanyValue: AvailableValue(30),
			Benchmark: BenchmarkSet{
				Form: FormPeerObservations,
				PeerObservations: []PeerObservation{
					{PeerKey: "peer-5", Value: 50}, {PeerKey: "peer-1", Value: 10}, {PeerKey: "peer-3", Value: 30},
					{PeerKey: "peer-4", Value: 40}, {PeerKey: "peer-2", Value: 20},
				},
				Source: BenchmarkSource{Name: "Peer Set", SampleSize: 5},
			},
		},
		{
			MetricID:     "OWNER_COMP_RATIO",
			CompanyValue: Unavailable(),
			Benchmark:    BenchmarkSet{Form: FormMedian, Median: AvailableValue(0.08), Source: BenchmarkSource{Name: "X"}},
		},
		{
			CompanyValue: AvailableValue(1),
			Benchmark:    BenchmarkSet{Form: BenchmarkForm("BOGUS")},
		},
	}}

	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}
