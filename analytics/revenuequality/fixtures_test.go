package revenuequality

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
// "2023", "2024", "2025", matching the shared multi-year fixtures
// (mirroring analytics/workingcapital/fixtures_test.go's threeYearMeta).
func threeYearMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// datasetBuilder accumulates NormalizedItem values for hand-built test
// datasets, sized to hit specific scenarios (volatile revenue, one-period
// spikes) that the repository's realistic multi-year fixtures aren't
// shaped to exercise precisely — mirroring
// workingcapital/fixtures_test.go's datasetBuilder exactly.
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

// fiveYearMeta builds a PeriodInfo map for five consecutive fiscal years
// "2021".."2025", used by scenarios needing more history than the shared
// three-year fixtures provide (volatility, CAGR span).
func fiveYearMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2021": {Type: PeriodTypeFiscalYear, FiscalYear: 2021},
		"2022": {Type: PeriodTypeFiscalYear, FiscalYear: 2022},
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// boolPtr returns a pointer to v, for populating
// CustomerPeriodRevenue.RecurringFlag in tests.
func boolPtr(v bool) *bool { return &v }
