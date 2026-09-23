package variance

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// FuzzCalculate_ActualAndBaseline proves Calculate never panics and never
// leaks NaN/Inf into Result, regardless of how malformed
// LineObservation.Actual/Baseline are (including a zero or near-zero
// Baseline, which drives PercentVariance's division).
func FuzzCalculate_ActualAndBaseline(f *testing.F) {
	seeds := []struct{ actual, baseline float64 }{
		{100_000, 90_000},
		{0, 0},
		{100_000, 0},
		{0, 100_000},
		{-50_000, 50_000},
		{math.NaN(), 100_000},
		{100_000, math.NaN()},
		{math.Inf(1), 100_000},
		{100_000, math.Inf(-1)},
		{math.MaxFloat64, math.SmallestNonzeroFloat64},
	}
	for _, s := range seeds {
		f.Add(s.actual, s.baseline)
	}

	f.Fuzz(func(t *testing.T, actual, baseline float64) {
		in := Input{
			Lines: []LineObservation{
				{
					AccountCode:  financial.CodeRevProduct,
					Period:       "2025",
					Actual:       actual,
					Baseline:     baseline,
					BaselineType: BaselineTypeBudget,
				},
			},
		}

		res := Calculate(in)

		for _, lv := range res.LineVariances {
			checkNoNaNOrInf(t, "AbsoluteVariance", lv.AbsoluteVariance)
			checkNoNaNOrInf(t, "PercentVariance", lv.PercentVariance)
		}
	})
}

func checkNoNaNOrInf(t *testing.T, name string, v VarianceValue) {
	t.Helper()
	if !v.Available {
		return
	}
	if math.IsNaN(v.Value) || math.IsInf(v.Value, 0) {
		t.Fatalf("%s leaked NaN/Inf into an Available VarianceValue: %v", name, v.Value)
	}
}
