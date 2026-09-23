package dealstructure

import (
	"math"
	"testing"
)

// FuzzBuild_PurchasePriceAndEquity proves Build never panics and never
// leaks NaN/Inf into Result, regardless of how malformed PurchasePrice/
// BuyerEquity are.
func FuzzBuild_PurchasePriceAndEquity(f *testing.F) {
	seeds := []struct{ price, equity float64 }{
		{19_500_000, 6_000_000}, {0, 0}, {19_500_000, 0}, {0, 6_000_000}, {-1_000_000, 6_000_000},
		{math.NaN(), 6_000_000}, {19_500_000, math.NaN()},
		{math.Inf(1), 6_000_000}, {19_500_000, math.Inf(-1)},
		{math.MaxFloat64, math.SmallestNonzeroFloat64},
	}
	for _, s := range seeds {
		f.Add(s.price, s.equity)
	}

	f.Fuzz(func(t *testing.T, price, equity float64) {
		in := Input{
			PurchasePrice: AvailableValue(price),
			BuyerEquity:   AvailableValue(equity),
		}

		res := Build(in)

		checkNoNaNOrInf(t, "SourcesAndUses.TotalSources", res.SourcesAndUses.TotalSources)
		checkNoNaNOrInf(t, "SourcesAndUses.TotalUses", res.SourcesAndUses.TotalUses)
		checkNoNaNOrInf(t, "SourcesAndUses.FundingGapOrSurplus", res.SourcesAndUses.FundingGapOrSurplus)
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
