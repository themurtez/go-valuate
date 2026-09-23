package covenants

import (
	"math"
	"testing"
)

// FuzzCalculate_ActualAndThreshold proves Calculate never panics and
// never leaks NaN/Inf into Result, regardless of how malformed
// CovenantTest.Actual/Threshold are.
func FuzzCalculate_ActualAndThreshold(f *testing.F) {
	seeds := []struct{ actual, threshold float64 }{
		{1.5, 1.25}, {0, 0}, {1.25, 0}, {0, 1.25}, {-1.5, 1.25},
		{math.NaN(), 1.25}, {1.5, math.NaN()},
		{math.Inf(1), 1.25}, {1.5, math.Inf(-1)},
		{math.MaxFloat64, math.SmallestNonzeroFloat64},
	}
	for _, s := range seeds {
		f.Add(s.actual, s.threshold)
	}

	f.Fuzz(func(t *testing.T, actual, threshold float64) {
		in := Input{
			Tests: []CovenantTest{
				{
					CovenantID:           "T1",
					Metric:               MetricDSCR,
					Operator:             OperatorGTE,
					Threshold:            threshold,
					Actual:               AvailableValue(actual),
					Period:               "2025",
					WarningBufferPercent: 0.10,
				},
			},
		}

		res := Calculate(in)

		for _, tr := range res.Tests {
			checkNoNaNOrInf(t, "Headroom", tr.Headroom)
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
