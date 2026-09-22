package ratios

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// loadFixtureDataset loads a financial.FinancialDataset from fixtures/<name>,
// mirroring workingcapital/fixtures_test.go's helper of the same name/shape.
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
// friends (mirroring workingcapital/qoe's identical threeYearMeta helper).
func threeYearMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// datasetBuilder accumulates NormalizedItem values for hand-built test
// datasets, sized to hit specific ratio scenarios (zero denominators,
// negative equity, negative earnings, missing balance sheet) that the
// repository's realistic multi-year fixtures aren't shaped to exercise
// precisely — mirroring workingcapital/fixtures_test.go's identical builder.
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

// incomeStatement adds a minimal complete income statement for one period:
// revenue, COGS, opex, D&A, interest, tax — sized so every income-statement-
// derived Snapshot field (TotalRevenue, GrossProfit, EBIT, EBITDA,
// NetIncome) is Available.
func (b *datasetBuilder) incomeStatement(period string, revenue, cogs, opex, depreciation, interestExpense, tax float64) *datasetBuilder {
	return b.
		add(financial.CodeRevProduct, period, revenue).
		add(financial.CodeCogsMaterial, period, cogs).
		add(financial.CodeOpexOther, period, opex).
		add(financial.CodeDepreciation, period, depreciation).
		add(financial.CodeInterestExpense, period, interestExpense).
		add(financial.CodeIncomeTax, period, tax)
}

// balanceSheet adds a minimal complete balance sheet for one period: cash,
// AR, inventory, current liabilities, debt, and equity — sized so every
// balance-sheet-derived ratio in this package is Available.
func (b *datasetBuilder) balanceSheet(period string, cash, ar, inventory, ap, shortTermDebt, longTermDebt, equity float64) *datasetBuilder {
	return b.
		add(financial.CodeBsCash, period, cash).
		add(financial.CodeBsAccountsReceivable, period, ar).
		add(financial.CodeBsInventory, period, inventory).
		add(financial.CodeBsAccountsPayable, period, ap).
		add(financial.CodeBsShortTermDebt, period, shortTermDebt).
		add(financial.CodeBsLongTermDebt, period, longTermDebt).
		add(financial.CodeBsOwnerEquity, period, equity)
}
