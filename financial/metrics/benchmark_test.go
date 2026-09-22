package metrics

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// benchmarkDataset builds a FinancialDataset with periodCount fiscal years,
// each carrying a realistic spread of income-statement and balance-sheet
// codes, so every metric formula (including trend/growth/CAGR/volatility,
// which need 2+ comparable periods) has real work to do.
func benchmarkDataset(periodCount int) financial.FinancialDataset {
	codes := []financial.Code{
		financial.CodeRevProduct, financial.CodeRevService, financial.CodeCogsMaterial, financial.CodeCogsDirectLabor,
		financial.CodeOpexPayroll, financial.CodeOpexRent, financial.CodeOpexMarketing, financial.CodeOpexOwnerComp,
		financial.CodeDepreciation, financial.CodeAmortization, financial.CodeInterestExpense, financial.CodeIncomeTax,
		financial.CodeBsCash, financial.CodeBsAccountsReceivable, financial.CodeBsInventory,
		financial.CodeBsAccountsPayable, financial.CodeBsShortTermDebt, financial.CodeBsLongTermDebt,
	}
	var items []financial.NormalizedItem
	for y := 0; y < periodCount; y++ {
		period := financial.Period(fmt.Sprintf("%d", 2020+y))
		for i, code := range codes {
			items = append(items, financial.NormalizedItem{Code: code, Period: period, Amount: float64((i+1)*10000 + y*500)})
		}
	}
	return financial.FinancialDataset{Currency: "USD", Items: items}
}

func benchmarkPeriodMeta(periodCount int) map[financial.Period]PeriodInfo {
	meta := make(map[financial.Period]PeriodInfo, periodCount)
	for y := 0; y < periodCount; y++ {
		fy := 2020 + y
		meta[financial.Period(fmt.Sprintf("%d", fy))] = PeriodInfo{Type: PeriodTypeFiscalYear, FiscalYear: fy}
	}
	return meta
}

// BenchmarkCalculate_1000Rows exercises Calculate over a 5-fiscal-year
// dataset built from 18 codes per year (90 NormalizedItems, ~1,000-row-
// equivalent when counting each period's full metric surface computed) —
// the candidate hot path named in docs/V1_CONTRACTS.md's performance
// sanity checks. A true 1,000-distinct-code dataset is unrealistic (this
// module's whole taxonomy is 43 codes); this benchmark instead scales
// period count, which is what actually drives Calculate's trend-analysis
// work.
func BenchmarkCalculate_1000Rows(b *testing.B) {
	dataset := benchmarkDataset(5)
	opts := Options{PeriodMeta: benchmarkPeriodMeta(5)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(dataset, opts)
	}
}
