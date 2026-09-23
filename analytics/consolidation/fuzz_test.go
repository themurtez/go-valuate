package consolidation

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// FuzzCalculate_CurrencyRate proves Calculate never panics and never
// leaks NaN/Inf into Result, regardless of how malformed
// CurrencyRate.Rate is — this figure multiplies every line item of every
// foreign-currency entity, so a NaN/Inf/negative/zero rate is the single
// highest-leverage fuzz target in this package.
func FuzzCalculate_CurrencyRate(f *testing.F) {
	seeds := []float64{
		1.0, 0.85, 1.35, 0, -1.0,
		math.NaN(), math.Inf(1), math.Inf(-1),
		math.MaxFloat64, math.SmallestNonzeroFloat64,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, rate float64) {
		items := []financial.NormalizedItem{
			{Code: financial.CodeRevProduct, Period: "2025", Amount: 100_000},
			{Code: financial.CodeBsCash, Period: "2025", Amount: 50_000},
		}
		in := Input{
			Entities: []EntityDataset{
				{
					EntityID: "usd-parent",
					Dataset:  financial.FinancialDataset{Currency: "USD", Items: items},
				},
				{
					EntityID: "eur-sub",
					Dataset:  financial.FinancialDataset{Currency: "EUR", Items: items},
				},
			},
			Periods: []financial.Period{"2025"},
			CurrencyRates: []CurrencyRate{
				{FromCurrency: "EUR", ToCurrency: "USD", Period: "2025", Rate: rate},
			},
			Policy: Policy{TargetCurrency: "USD"},
		}

		res := Calculate(in)

		for _, it := range res.Consolidated.Items {
			if math.IsNaN(it.Amount) || math.IsInf(it.Amount, 0) {
				t.Fatalf("Consolidated.Items[%s/%s] leaked NaN/Inf: %v", it.Code, it.Period, it.Amount)
			}
		}
	})
}
