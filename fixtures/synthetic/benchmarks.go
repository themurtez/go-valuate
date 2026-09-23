package synthetic

import "github.com/themurtez/go-valuate/analytics/benchmarks"

// BuildBenchmarkInput returns analytics/benchmarks.Input comparing Meridian
// SaaS's 2025 gross margin and EBITDA margin against a synthetic SaaS
// industry benchmark, using two different BenchmarkForm shapes
// (percentile bands and quartiles) so both interpolation paths are
// exercised. Gross margin ((8724000-1210000)/8724000 ≈ 86.1%) and EBITDA
// margin (2854600/8724000 ≈ 32.7%) are computed from the same figures
// BuildDataset/meridian2025EBITDA use.
func BuildBenchmarkInput() benchmarks.Input {
	return benchmarks.Input{
		Metrics: []benchmarks.MetricRequest{
			{
				MetricID:     "GROSS_MARGIN",
				Label:        "Gross Margin %",
				CompanyValue: benchmarks.AvailableValue(0.8612),
				Period:       "2025",
				Direction:    benchmarks.DirectionHigherIsBetter,
				Benchmark: benchmarks.BenchmarkSet{
					Form: benchmarks.FormPercentileBands,
					PercentileBands: []benchmarks.PercentilePoint{
						{Percentile: 10, Value: 0.62},
						{Percentile: 25, Value: 0.70},
						{Percentile: 50, Value: 0.76},
						{Percentile: 75, Value: 0.82},
						{Percentile: 90, Value: 0.88},
					},
					Source: benchmarks.BenchmarkSource{
						Name:          "Synthetic SaaS Industry Benchmark Set",
						EffectiveDate: "2025",
						Population:    "SaaS companies, $5M-$15M revenue",
						SampleSize:    240,
					},
				},
			},
			{
				MetricID:     "EBITDA_MARGIN",
				Label:        "EBITDA Margin %",
				CompanyValue: benchmarks.AvailableValue(0.3272),
				Period:       "2025",
				Direction:    benchmarks.DirectionHigherIsBetter,
				Benchmark: benchmarks.BenchmarkSet{
					Form: benchmarks.FormQuartiles,
					Quartiles: benchmarks.Quartiles{
						Q1:     benchmarks.AvailableValue(0.08),
						Median: benchmarks.AvailableValue(0.16),
						Q3:     benchmarks.AvailableValue(0.24),
					},
					Source: benchmarks.BenchmarkSource{
						Name:          "Synthetic SaaS Industry Benchmark Set",
						EffectiveDate: "2025",
						Population:    "SaaS companies, $5M-$15M revenue",
						SampleSize:    240,
					},
				},
			},
			{
				MetricID:     "REVENUE_PER_CUSTOMER",
				Label:        "Annual Revenue per Customer",
				CompanyValue: benchmarks.AvailableValue(8724000.0 / 14.0),
				Period:       "2025",
				Direction:    benchmarks.DirectionHigherIsBetter,
				Benchmark: benchmarks.BenchmarkSet{
					Form:             benchmarks.FormPeerObservations,
					PeerObservations: BuildBenchmarkPeers(100),
					Source: benchmarks.BenchmarkSource{
						Name:          "Synthetic SaaS Industry Benchmark Set",
						EffectiveDate: "2025",
						Population:    "SaaS companies, $5M-$15M revenue",
						SampleSize:    100,
					},
				},
			},
		},
	}
}

// BuildBenchmarkPeers returns n synthetic PeerObservation values for the
// REVENUE_PER_CUSTOMER metric, deterministically generated (no
// randomness) so repeated calls and repeated test runs are byte-for-byte
// identical. Values span a realistic $250K-$950K range using a fixed
// linear-congruential-style deterministic spread — used both as
// BuildBenchmarkInput's default peer set and directly by performance
// benchmarks needing a large peer count (see this module's
// BenchmarkXxx_100Peers-style tests).
func BuildBenchmarkPeers(n int) []benchmarks.PeerObservation {
	peers := make([]benchmarks.PeerObservation, 0, n)
	for i := 0; i < n; i++ {
		// Deterministic pseudo-spread: a simple triangular wave across
		// [250000, 950000], never calling math/rand, so output is stable
		// across Go versions/platforms without needing a fixed seed.
		frac := float64(i%50) / 49.0
		if (i/50)%2 == 1 {
			frac = 1 - frac
		}
		value := 250_000 + frac*700_000
		peers = append(peers, benchmarks.PeerObservation{
			PeerKey: syntheticPeerKey(i),
			Value:   value,
		})
	}
	return peers
}

func syntheticPeerKey(i int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	return "peer-" + string(letters[i%26]) + string(letters[(i/26)%26]) + string(rune('0'+i%10))
}
