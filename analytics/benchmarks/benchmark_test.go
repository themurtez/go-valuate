package benchmarks

import "testing"

// benchmarkPeers deterministically generates n PeerObservation values
// spanning a realistic range, no randomness (matches the technique
// fixtures/synthetic.BuildBenchmarkPeers uses).
func benchmarkPeers(n int) []PeerObservation {
	peers := make([]PeerObservation, 0, n)
	for i := 0; i < n; i++ {
		frac := float64(i%50) / 49.0
		if (i/50)%2 == 1 {
			frac = 1 - frac
		}
		peers = append(peers, PeerObservation{Value: 100_000 + frac*900_000})
	}
	return peers
}

// benchmarkMetrics builds metricCount MetricRequests, each comparing
// against its own 100-peer BenchmarkSet, exercising this package's
// FormPeerObservations path (the richest/most expensive form) at scale.
func benchmarkMetrics(metricCount, peerCount int) []MetricRequest {
	metrics := make([]MetricRequest, 0, metricCount)
	for i := 0; i < metricCount; i++ {
		metrics = append(metrics, MetricRequest{
			MetricID:     "METRIC",
			CompanyValue: AvailableValue(500_000),
			Direction:    DirectionHigherIsBetter,
			Benchmark: BenchmarkSet{
				Form:             FormPeerObservations,
				PeerObservations: benchmarkPeers(peerCount),
			},
		})
	}
	return metrics
}

// BenchmarkCalculate_10MetricsX100Peers exercises this package's
// representative workload: 10 metrics, each benchmarked against 100 peer
// observations (1,000 peer comparisons total).
func BenchmarkCalculate_10MetricsX100Peers(b *testing.B) {
	in := Input{Metrics: benchmarkMetrics(10, 100)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(in)
	}
}
