package consolidation

import "github.com/themurtez/go-valuate/financial"

// item is a terse constructor for a financial.NormalizedItem, for test
// readability.
func item(code financial.Code, period string, amount float64) financial.NormalizedItem {
	return financial.NormalizedItem{Code: code, Period: financial.Period(period), Amount: amount}
}

// twoYearPeriods returns the common two-fiscal-year period set most
// fixtures below consolidate over.
func twoYearPeriods() []financial.Period {
	return []financial.Period{"2024", "2025"}
}

// parentUSD models a US parent company: product revenue, payroll opex, and
// an intercompany management-fee revenue line billed to the subsidiary
// (see subsidiaryEUR's matching OPEX_OTHER expense) across two fiscal
// years, in USD.
func parentUSD() financial.FinancialDataset {
	return financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			item(financial.CodeRevProduct, "2024", 1_000_000),
			item(financial.CodeRevProduct, "2025", 1_200_000),
			item(financial.CodeRevOther, "2024", 50_000), // intercompany management fee billed to subsidiary
			item(financial.CodeRevOther, "2025", 60_000),
			item(financial.CodeOpexPayroll, "2024", 400_000),
			item(financial.CodeOpexPayroll, "2025", 450_000),
		},
	}
}

// subsidiaryEUR models a wholly-distinct European subsidiary: service
// revenue, payroll opex, and the intercompany management fee expense
// mirroring parentUSD's CodeRevOther line, in EUR.
func subsidiaryEUR() financial.FinancialDataset {
	return financial.FinancialDataset{
		Currency: "EUR",
		Items: []financial.NormalizedItem{
			item(financial.CodeRevService, "2024", 500_000),
			item(financial.CodeRevService, "2025", 600_000),
			item(financial.CodeOpexPayroll, "2024", 200_000),
			item(financial.CodeOpexPayroll, "2025", 220_000),
			item(financial.CodeOpexOther, "2024", 45_000), // intercompany management fee paid to parent
			item(financial.CodeOpexOther, "2025", 55_000),
		},
	}
}

// twoEntityUSDInput builds a minimal two-entity, single-currency
// (USD-only) Input covering twoYearPeriods, with no eliminations and full
// consolidation — the simplest valid baseline most tests start from.
func twoEntityUSDInput() Input {
	sub := subsidiaryEUR()
	sub.Currency = "USD" // reuse the same line items under a same-currency scenario
	return Input{
		Entities: []EntityDataset{
			{EntityID: "parent", EntityLabel: "Parent Co", Dataset: parentUSD()},
			{EntityID: "sub", EntityLabel: "Subsidiary Co", Dataset: sub},
		},
		Periods: twoYearPeriods(),
	}
}

// eurToUSDRates supplies one blended annual EUR->USD rate per fiscal year
// in twoYearPeriods.
func eurToUSDRates() []CurrencyRate {
	return []CurrencyRate{
		{FromCurrency: "EUR", ToCurrency: "USD", Period: "2024", Rate: 1.10},
		{FromCurrency: "EUR", ToCurrency: "USD", Period: "2025", Rate: 1.08},
	}
}

// managementFeeEliminations removes the intercompany management fee from
// both sides: parent's CodeRevOther income and subsidiary's CodeOpexOther
// expense, for both fiscal years — the amounts mirror parentUSD's
// CodeRevOther exactly (see parentUSD's doc comment). The subsidiary side
// is expressed in EUR (subsidiaryEUR's native currency), consistent with
// Elimination.Amount's "removed before currency conversion" contract.
func managementFeeEliminations() []Elimination {
	return []Elimination{
		{EntityID: "parent", Code: financial.CodeRevOther, Period: "2024", Amount: 50_000, Description: "intercompany management fee, Sub -> Parent"},
		{EntityID: "parent", Code: financial.CodeRevOther, Period: "2025", Amount: 60_000, Description: "intercompany management fee, Sub -> Parent"},
		{EntityID: "sub", Code: financial.CodeOpexOther, Period: "2024", Amount: 45_000, Description: "intercompany management fee, Sub -> Parent"},
		{EntityID: "sub", Code: financial.CodeOpexOther, Period: "2025", Amount: 55_000, Description: "intercompany management fee, Sub -> Parent"},
	}
}

// ptr returns a pointer to v, for building *float64 fields inline.
func ptr(v float64) *float64 { return &v }

// diffAbs returns the absolute difference between a and b, for
// tolerance-based float64 comparisons in tests (float64 summation order
// can produce last-bit differences from a hand-computed expected value) —
// mirroring analytics/concentration/fixtures_test.go's identical helper.
func diffAbs(a, b float64) float64 {
	d := a - b
	if d < 0 {
		return -d
	}
	return d
}
