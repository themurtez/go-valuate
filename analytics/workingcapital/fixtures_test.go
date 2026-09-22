package workingcapital

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// loadFixtureDataset loads a financial.FinancialDataset from fixtures/<name>,
// mirroring financial/adjustments/fixtures_test.go's helper of the same
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
// friends (mirroring analytics/qoe/fixtures_test.go's threeYearMeta).
func threeYearMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// datasetBuilder accumulates NormalizedItem values for hand-built test
// datasets, sized to hit specific statistical scenarios (seasonal patterns,
// declining trends, negative NWC) that the repository's realistic
// multi-year fixtures aren't shaped to exercise precisely.
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

// bsPeriod adds a standard set of balance-sheet current-asset/liability
// codes for one period, plus product revenue, under the default
// InclusionPolicy's included codes only (cash and short-term debt
// deliberately omitted from most scenarios so DefaultInclusionPolicy's
// exclusion of them is exercised implicitly; tests that need cash/debt add
// them explicitly).
func (b *datasetBuilder) bsPeriod(period string, ar, inventory, prepaid, ap, otherCL, revenue float64) *datasetBuilder {
	return b.
		add(financial.CodeBsAccountsReceivable, period, ar).
		add(financial.CodeBsInventory, period, inventory).
		add(financial.CodeBsPrepaid, period, prepaid).
		add(financial.CodeBsAccountsPayable, period, ap).
		add(financial.CodeBsCurrentLiabilityOther, period, otherCL).
		add(financial.CodeRevProduct, period, revenue)
}
