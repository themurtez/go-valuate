package synthetic

import "github.com/themurtez/go-valuate/analytics/forecast"

// BuildForecastInput returns analytics/forecast.Input for Meridian SaaS: a
// 3-year horizon from the 2025 base period, two scenarios (Base Case
// continuing the historical ~28% revenue growth and stable ~86% gross
// margin; Downside with growth stalling to 8% and margin compression).
// Both scenarios share the same tax rate (24%, matching BuildDataset's
// taxRate) and working-capital-as-percent-of-revenue assumption, so the
// suite-level smoke test can compare scenario outputs meaningfully.
func BuildForecastInput() forecast.Input {
	return forecast.Input{
		Dataset:              BuildDataset(),
		PeriodMeta:           ForecastPeriodMeta(),
		Horizon:              3,
		ForecastPeriodLabels: []string{"2026", "2027", "2028"},
		Scenarios: []forecast.Scenario{
			{
				Name: "Base Case",
				Type: forecast.ScenarioTypeBase,
				Assumptions: forecast.Assumptions{
					Revenue: []forecast.RevenuePeriodAssumption{
						{Method: forecast.RevenueMethodGrowthRate, GrowthRate: 0.28},
						{Method: forecast.RevenueMethodGrowthRate, GrowthRate: 0.25},
						{Method: forecast.RevenueMethodGrowthRate, GrowthRate: 0.22},
					},
					COGS: []forecast.COGSPeriodAssumption{
						{Method: forecast.COGSMethodGrossMarginPercent, GrossMarginPercent: 0.86},
						{Method: forecast.COGSMethodGrossMarginPercent, GrossMarginPercent: 0.865},
						{Method: forecast.COGSMethodGrossMarginPercent, GrossMarginPercent: 0.87},
					},
					Opex: []forecast.OpexPeriodAssumption{
						{Method: forecast.OpexMethodGrowthRate, GrowthRate: 0.22},
						{Method: forecast.OpexMethodGrowthRate, GrowthRate: 0.20},
						{Method: forecast.OpexMethodGrowthRate, GrowthRate: 0.18},
					},
					DepreciationAmortization: []forecast.DepreciationAmortizationAssumption{
						{Depreciation: forecast.AvailableValue(62_000), Amortization: forecast.AvailableValue(72_000)},
						{Depreciation: forecast.AvailableValue(70_000), Amortization: forecast.AvailableValue(79_000)},
						{Depreciation: forecast.AvailableValue(78_000), Amortization: forecast.AvailableValue(86_000)},
					},
					Capex: []forecast.CapexAssumption{
						{Capex: forecast.AvailableValue(180_000)},
						{Capex: forecast.AvailableValue(210_000)},
						{Capex: forecast.AvailableValue(240_000)},
					},
					WorkingCapital: []forecast.WorkingCapitalPeriodAssumption{
						{Method: forecast.WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.055},
						{Method: forecast.WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.055},
						{Method: forecast.WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.055},
					},
					Tax: []forecast.TaxPeriodAssumption{
						{Method: forecast.TaxMethodPercentOfPretaxIncome, TaxRate: 0.24},
						{Method: forecast.TaxMethodPercentOfPretaxIncome, TaxRate: 0.24},
						{Method: forecast.TaxMethodPercentOfPretaxIncome, TaxRate: 0.24},
					},
				},
			},
			{
				Name: "Downside — Growth Stalls",
				Type: forecast.ScenarioTypeDownside,
				Assumptions: forecast.Assumptions{
					Revenue: []forecast.RevenuePeriodAssumption{
						{Method: forecast.RevenueMethodGrowthRate, GrowthRate: 0.08},
						{Method: forecast.RevenueMethodGrowthRate, GrowthRate: 0.05},
						{Method: forecast.RevenueMethodGrowthRate, GrowthRate: 0.05},
					},
					COGS: []forecast.COGSPeriodAssumption{
						{Method: forecast.COGSMethodGrossMarginPercent, GrossMarginPercent: 0.83},
						{Method: forecast.COGSMethodGrossMarginPercent, GrossMarginPercent: 0.82},
						{Method: forecast.COGSMethodGrossMarginPercent, GrossMarginPercent: 0.81},
					},
					Opex: []forecast.OpexPeriodAssumption{
						{Method: forecast.OpexMethodGrowthRate, GrowthRate: 0.12},
						{Method: forecast.OpexMethodGrowthRate, GrowthRate: 0.08},
						{Method: forecast.OpexMethodGrowthRate, GrowthRate: 0.06},
					},
					DepreciationAmortization: []forecast.DepreciationAmortizationAssumption{
						{Depreciation: forecast.AvailableValue(62_000), Amortization: forecast.AvailableValue(72_000)},
						{Depreciation: forecast.AvailableValue(66_000), Amortization: forecast.AvailableValue(75_000)},
						{Depreciation: forecast.AvailableValue(70_000), Amortization: forecast.AvailableValue(78_000)},
					},
					Capex: []forecast.CapexAssumption{
						{Capex: forecast.AvailableValue(120_000)},
						{Capex: forecast.AvailableValue(100_000)},
						{Capex: forecast.AvailableValue(90_000)},
					},
					WorkingCapital: []forecast.WorkingCapitalPeriodAssumption{
						{Method: forecast.WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.065},
						{Method: forecast.WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.065},
						{Method: forecast.WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.065},
					},
					Tax: []forecast.TaxPeriodAssumption{
						{Method: forecast.TaxMethodPercentOfPretaxIncome, TaxRate: 0.24},
						{Method: forecast.TaxMethodPercentOfPretaxIncome, TaxRate: 0.24},
						{Method: forecast.TaxMethodPercentOfPretaxIncome, TaxRate: 0.24},
					},
				},
			},
		},
	}
}
