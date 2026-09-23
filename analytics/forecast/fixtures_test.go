package forecast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// loadFixtureDataset loads a financial.FinancialDataset from fixtures/<name>,
// mirroring analytics/cashflow/fixtures_test.go's helper of the same
// name/shape.
func loadFixtureDataset(t *testing.T, name string) financial.FinancialDataset {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "fixtures", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var ds financial.FinancialDataset
	if err := json.Unmarshal(b, &ds); err != nil {
		t.Fatalf("unmarshaling fixture %s: %v", name, err)
	}
	return ds
}

// threeYearMeta builds a PeriodInfo map for three consecutive fiscal years
// "2023", "2024", "2025", matching normalized_hvac_multi_year.json and
// friends (mirroring analytics/cashflow/fixtures_test.go's threeYearMeta,
// adapted to this package's own PeriodInfo/PeriodType types).
func threeYearMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// datasetBuilder accumulates NormalizedItem values for hand-built test
// datasets, sized to hit specific forecast scenarios precisely — mirroring
// analytics/cashflow/fixtures_test.go's identical helper.
type datasetBuilder struct {
	items []financial.NormalizedItem
}

func newDataset() *datasetBuilder {
	return &datasetBuilder{}
}

func (b *datasetBuilder) add(code financial.Code, period string, amount float64) *datasetBuilder {
	b.items = append(b.items, financial.NormalizedItem{
		Code:   code,
		Period: financial.Period(period),
		Amount: amount,
	})
	return b
}

func (b *datasetBuilder) build() financial.FinancialDataset {
	return financial.FinancialDataset{Currency: "USD", Items: b.items}
}

// simpleBasePeriod adds a minimal, easy-to-verify-by-hand income statement
// for one period: one revenue code, one COGS code, one opex code, and
// depreciation.
//
// EBITDA = revenue - cogs - opex (depreciation is added back, so it nets
// out of EBITDA but is still present so this package's D&A add-back path
// is exercised rather than a trivial zero).
func (b *datasetBuilder) simpleBasePeriod(period string, revenue, cogs, opex, depreciation float64) *datasetBuilder {
	return b.
		add(financial.CodeRevProduct, period, revenue).
		add(financial.CodeCogsMaterial, period, cogs).
		add(financial.CodeOpexPayroll, period, opex).
		add(financial.CodeDepreciation, period, depreciation)
}

// singlePeriodMeta returns a PeriodInfo map with exactly one fiscal-year
// entry for period.
func singlePeriodMeta(period string, year int) map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		financial.Period(period): {Type: PeriodTypeFiscalYear, FiscalYear: year},
	}
}

// flatGrowth returns n RevenuePeriodAssumption/OpexPeriodAssumption-style
// growth-rate assumptions all set to the same rate — a convenience for
// tests that don't care about period-by-period variation.
func flatRevenueGrowth(rate float64, n int) []RevenuePeriodAssumption {
	out := make([]RevenuePeriodAssumption, n)
	for i := range out {
		out[i] = RevenuePeriodAssumption{Method: RevenueMethodGrowthRate, GrowthRate: rate}
	}
	return out
}

func flatOpexGrowth(rate float64, n int) []OpexPeriodAssumption {
	out := make([]OpexPeriodAssumption, n)
	for i := range out {
		out[i] = OpexPeriodAssumption{Method: OpexMethodGrowthRate, GrowthRate: rate}
	}
	return out
}

func flatGrossMargin(pct float64, n int) []COGSPeriodAssumption {
	out := make([]COGSPeriodAssumption, n)
	for i := range out {
		out[i] = COGSPeriodAssumption{Method: COGSMethodGrossMarginPercent, GrossMarginPercent: pct}
	}
	return out
}
