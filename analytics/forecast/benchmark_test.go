package forecast

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// benchmarkDataset builds a 3-year historical dataset with a realistic
// spread of income-statement codes, the minimum this package needs to
// identify a base period and project forward from it.
func benchmarkDataset() (financial.FinancialDataset, map[financial.Period]PeriodInfo) {
	codes := []financial.Code{
		financial.CodeRevProduct, financial.CodeCogsMaterial,
		financial.CodeOpexPayroll, financial.CodeOpexMarketing,
		financial.CodeDepreciation, financial.CodeAmortization, financial.CodeInterestExpense,
	}
	var items []financial.NormalizedItem
	meta := make(map[financial.Period]PeriodInfo, 3)
	for y := 0; y < 3; y++ {
		period := financial.Period(fmt.Sprintf("%d", 2023+y))
		meta[period] = PeriodInfo{Type: PeriodTypeFiscalYear, FiscalYear: 2023 + y}
		for i, code := range codes {
			items = append(items, financial.NormalizedItem{Code: code, Period: period, Amount: float64((i+1)*100_000 + y*10_000)})
		}
	}
	return financial.FinancialDataset{Currency: "USD", Items: items}, meta
}

// benchmarkScenarios builds n independent Scenarios, each with a full
// 5-period revenue/COGS/opex assumption set, exercising this package's
// representative "many scenarios" workload.
func benchmarkScenarios(n, horizon int) []Scenario {
	scenarios := make([]Scenario, 0, n)
	for s := 0; s < n; s++ {
		growth := 0.05 + float64(s)*0.001
		revenue := make([]RevenuePeriodAssumption, horizon)
		cogs := make([]COGSPeriodAssumption, horizon)
		opex := make([]OpexPeriodAssumption, horizon)
		for p := 0; p < horizon; p++ {
			revenue[p] = RevenuePeriodAssumption{Method: RevenueMethodGrowthRate, GrowthRate: growth}
			cogs[p] = COGSPeriodAssumption{Method: COGSMethodGrossMarginPercent, GrossMarginPercent: 0.6}
			opex[p] = OpexPeriodAssumption{Method: OpexMethodGrowthRate, GrowthRate: 0.03}
		}
		scenarios = append(scenarios, Scenario{
			Name: fmt.Sprintf("scenario-%02d", s),
			Type: ScenarioTypeCustom,
			Assumptions: Assumptions{
				Revenue: revenue,
				COGS:    cogs,
				Opex:    opex,
			},
		})
	}
	return scenarios
}

// BenchmarkCalculate_20ScenariosX5Periods exercises this package's
// representative workload: 20 independent scenarios, each projecting 5
// forecast periods forward.
func BenchmarkCalculate_20ScenariosX5Periods(b *testing.B) {
	ds, meta := benchmarkDataset()
	in := Input{
		Dataset:    ds,
		PeriodMeta: meta,
		Horizon:    5,
		Scenarios:  benchmarkScenarios(20, 5),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(in)
	}
}
