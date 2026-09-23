package diagnostics

import (
	"fmt"
	"testing"
)

// benchmarkPortfolio builds n businesses, alternating stable/declining
// snapshots (reusing this package's own fixtures_test.go helpers) so
// Calculate has a realistic mix of finding-triggering and
// finding-free businesses at scale.
func benchmarkPortfolio(n int) []BusinessSnapshot {
	portfolio := make([]BusinessSnapshot, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("business-%04d", i)
		if i%2 == 0 {
			portfolio = append(portfolio, stableBusiness(id))
		} else {
			portfolio = append(portfolio, decliningBusiness(id))
		}
	}
	return portfolio
}

// BenchmarkCalculate_100Businesses exercises this package's representative
// workload: a 100-business portfolio, ranking findings across the whole
// book.
func BenchmarkCalculate_100Businesses(b *testing.B) {
	in := Input{Portfolio: benchmarkPortfolio(100)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(in)
	}
}
