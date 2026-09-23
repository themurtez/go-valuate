package concentration

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// FuzzCalculate_ObservationAmounts proves Calculate never panics and never
// leaks NaN/Inf into Result, regardless of how malformed
// Observation.Amount is — including negative, zero, NaN, +/-Inf, and
// extreme-magnitude values, all of which validateObservations must
// either accept or reject via IssueInvalidObservation, never let through
// unguarded into HHI/share arithmetic.
func FuzzCalculate_ObservationAmounts(f *testing.F) {
	seeds := []float64{
		0, 1, -1, 100_000, -100_000,
		math.NaN(), math.Inf(1), math.Inf(-1),
		math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, amount float64) {
		in := Input{
			Basis: BasisCustomerRevenue,
			Observations: []Observation{
				{EntityKey: "a", Period: "2024", Amount: amount},
				{EntityKey: "b", Period: "2024", Amount: 500_000},
				{EntityKey: "a", Period: "2025", Amount: amount},
				{EntityKey: "b", Period: "2025", Amount: 600_000},
			},
			PeriodMeta: map[financial.Period]PeriodInfo{
				"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
				"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
			},
		}

		// The call under test: must never panic regardless of amount.
		res := Calculate(in, Options{})

		for _, pc := range res.History {
			checkNoNaNOrInf(t, "HHI", pc.HHI)
			checkNoNaNOrInf(t, "LargestEntityShare", pc.LargestEntityShare)
			for _, s := range pc.TopNShares {
				checkNoNaNOrInf(t, "TopNShares.Share", s.Share)
			}
		}
	})
}

func checkNoNaNOrInf(t *testing.T, name string, v ConcentrationValue) {
	t.Helper()
	if !v.Available {
		return
	}
	if math.IsNaN(v.Value) || math.IsInf(v.Value, 0) {
		t.Fatalf("%s leaked NaN/Inf into an Available ConcentrationValue: %v", name, v.Value)
	}
}
