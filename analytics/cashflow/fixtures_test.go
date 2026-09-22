package cashflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// loadFixtureDataset loads a financial.FinancialDataset from fixtures/<name>,
// mirroring analytics/workingcapital/fixtures_test.go's helper of the same
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
// friends (mirroring analytics/workingcapital/fixtures_test.go's
// threeYearMeta).
func threeYearMeta() map[financial.Period]metrics.PeriodInfo {
	return map[financial.Period]metrics.PeriodInfo{
		"2023": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// datasetBuilder accumulates NormalizedItem values for hand-built test
// datasets, sized to hit specific cash-flow scenarios (strong/weak
// conversion, capex-heavy, working-capital build, negative cash flow) the
// repository's realistic multi-year fixtures aren't shaped to exercise
// precisely — mirroring analytics/workingcapital/fixtures_test.go's
// identical helper.
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

// incomeStatementPeriod adds a minimal profitable income statement for one
// period: revenue, COGS, a payroll opex line, and depreciation, sized so
// EBITDA is a clean, easy-to-verify-by-hand figure.
//
// EBITDA = revenue - cogs - payroll (depreciation is added back, so it
// nets out of EBITDA but is still present so financial/metrics.EBITDA
// exercises its normal Depreciation-add-back path rather than a
// trivial 0).
func (b *datasetBuilder) incomeStatementPeriod(period string, revenue, cogs, payroll, depreciation float64) *datasetBuilder {
	return b.
		add(financial.CodeRevProduct, period, revenue).
		add(financial.CodeCogsMaterial, period, cogs).
		add(financial.CodeOpexPayroll, period, payroll).
		add(financial.CodeDepreciation, period, depreciation)
}

// balanceSheetPeriod adds the default-InclusionPolicy-covered
// current-asset/liability codes for one period (AR, inventory, prepaid,
// AP, other current liability) — cash and short-term debt deliberately
// omitted, matching analytics/workingcapital's default policy.
func (b *datasetBuilder) balanceSheetPeriod(period string, ar, inventory, prepaid, ap, otherCL float64) *datasetBuilder {
	return b.
		add(financial.CodeBsAccountsReceivable, period, ar).
		add(financial.CodeBsInventory, period, inventory).
		add(financial.CodeBsPrepaid, period, prepaid).
		add(financial.CodeBsAccountsPayable, period, ap).
		add(financial.CodeBsCurrentLiabilityOther, period, otherCL)
}
