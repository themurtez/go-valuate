package benchmarks

import (
	"math"
	"testing"
)

// FuzzCalculate_CompanyValueAndMedian proves Calculate never panics and
// never leaks NaN/Inf into Result, regardless of how malformed
// CompanyValue/Median are — including a zero or near-zero Median, which
// drives RelativeDifference's division (see IssueBenchmarkMedianZero).
func FuzzCalculate_CompanyValueAndMedian(f *testing.F) {
	seeds := []struct{ company, median float64 }{
		{0.5, 0.4}, {0, 0}, {0.5, 0}, {0, 0.5}, {-0.5, 0.5},
		{math.NaN(), 0.4}, {0.5, math.NaN()},
		{math.Inf(1), 0.4}, {0.5, math.Inf(-1)},
		{math.MaxFloat64, math.SmallestNonzeroFloat64},
	}
	for _, s := range seeds {
		f.Add(s.company, s.median)
	}

	f.Fuzz(func(t *testing.T, company, median float64) {
		in := Input{
			Metrics: []MetricRequest{
				{
					MetricID:     "M",
					CompanyValue: AvailableValue(company),
					Direction:    DirectionHigherIsBetter,
					Benchmark: BenchmarkSet{
						Form:   FormMedian,
						Median: AvailableValue(median),
					},
				},
			},
		}

		res := Calculate(in)

		for _, c := range res.Comparisons {
			checkNoNaNOrInf(t, "Difference", c.Difference)
			checkNoNaNOrInf(t, "RelativeDifference", c.RelativeDifference)
		}
	})
}

func checkNoNaNOrInf(t *testing.T, name string, v Value) {
	t.Helper()
	if !v.Available {
		return
	}
	if math.IsNaN(v.Amount) || math.IsInf(v.Amount, 0) {
		t.Fatalf("%s leaked NaN/Inf into an Available Value: %v", name, v.Amount)
	}
}
