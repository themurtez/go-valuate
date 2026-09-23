package concentration

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// benchmarkObservations builds entityCount entities across yearCount
// fiscal years, with a realistic long-tail revenue distribution (a
// handful of large entities, a long tail of small ones) so HHI/top-N-share
// computation has real, non-degenerate work to do at scale.
func benchmarkObservations(entityCount, yearCount int) ([]Observation, map[financial.Period]PeriodInfo) {
	meta := make(map[financial.Period]PeriodInfo, yearCount)
	var obs []Observation
	for y := 0; y < yearCount; y++ {
		period := financial.Period(fmt.Sprintf("%d", 2016+y))
		meta[period] = PeriodInfo{Type: PeriodTypeFiscalYear, FiscalYear: 2016 + y}
		for i := 0; i < entityCount; i++ {
			// Long-tail distribution: entity i's base amount shrinks as
			// 1/(i+1), then grows ~5%/year — deterministic, no randomness.
			base := 1_000_000.0 / float64(i+1)
			amount := base * pow105(y)
			obs = append(obs, Observation{
				EntityKey: fmt.Sprintf("entity-%04d", i),
				Period:    period,
				Amount:    amount,
			})
		}
	}
	return obs, meta
}

func pow105(y int) float64 {
	v := 1.0
	for i := 0; i < y; i++ {
		v *= 1.05
	}
	return v
}

// BenchmarkCalculate_1000EntitiesX10Years exercises this package's largest
// representative workload: 1,000 customers/entities across 10 fiscal
// years (10,000 observations total).
func BenchmarkCalculate_1000EntitiesX10Years(b *testing.B) {
	obs, meta := benchmarkObservations(1000, 10)
	in := Input{
		Basis:        BasisCustomerRevenue,
		Observations: obs,
		PeriodMeta:   meta,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(in, Options{})
	}
}

// BenchmarkCalculate_100EntitiesX3Years is a smaller, more typical
// workload for comparison against the 1000x10 case above.
func BenchmarkCalculate_100EntitiesX3Years(b *testing.B) {
	obs, meta := benchmarkObservations(100, 3)
	in := Input{
		Basis:        BasisCustomerRevenue,
		Observations: obs,
		PeriodMeta:   meta,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(in, Options{})
	}
}
