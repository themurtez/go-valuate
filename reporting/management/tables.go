package management

import (
	"github.com/themurtez/go-valuate/analytics/variance"
)

// toVarianceLine converts one analytics/variance.LineVariance into this
// package's condensed VarianceLine.
func toVarianceLine(lv variance.LineVariance) VarianceLine {
	return VarianceLine{
		AccountCode:       lv.AccountCode,
		Label:             lv.Label,
		Period:            lv.Period,
		Actual:            lv.Actual,
		BaselineAvailable: lv.BaselineAvailable,
		Baseline:          lv.Baseline,
		AbsoluteVariance:  Value{Available: lv.AbsoluteVariance.Available, Amount: lv.AbsoluteVariance.Value},
		PercentVariance:   Value{Available: lv.PercentVariance.Available, Amount: lv.PercentVariance.Value},
		Favorability:      string(lv.Favorability),
	}
}

// buildVarianceTables populates VarianceTables from in.Variance, in a fixed
// table order: all line items, top favorable, top unfavorable, material
// exceptions. A table is omitted entirely when its source slice is empty.
func buildVarianceTables(in Input) VarianceTables {
	if !in.Variance.Available {
		return VarianceTables{}
	}

	var tables []VarianceTable
	if len(in.Variance.LineVariances) > 0 {
		lines := make([]VarianceLine, 0, len(in.Variance.LineVariances))
		for _, lv := range in.Variance.LineVariances {
			lines = append(lines, toVarianceLine(lv))
		}
		tables = append(tables, VarianceTable{Label: "All Line Items", Lines: lines})
	}
	if len(in.Variance.TopFavorable) > 0 {
		lines := make([]VarianceLine, 0, len(in.Variance.TopFavorable))
		for _, lv := range in.Variance.TopFavorable {
			lines = append(lines, toVarianceLine(lv))
		}
		tables = append(tables, VarianceTable{Label: "Top Favorable Variances", Lines: lines})
	}
	if len(in.Variance.TopUnfavorable) > 0 {
		lines := make([]VarianceLine, 0, len(in.Variance.TopUnfavorable))
		for _, lv := range in.Variance.TopUnfavorable {
			lines = append(lines, toVarianceLine(lv))
		}
		tables = append(tables, VarianceTable{Label: "Top Unfavorable Variances", Lines: lines})
	}
	if len(in.Variance.MaterialExceptions) > 0 {
		lines := make([]VarianceLine, 0, len(in.Variance.MaterialExceptions))
		for _, me := range in.Variance.MaterialExceptions {
			lines = append(lines, toVarianceLine(me.LineVariance))
		}
		tables = append(tables, VarianceTable{Label: "Material Exceptions", Lines: lines})
	}

	return VarianceTables{
		Available:      true,
		FormulaVersion: in.Variance.FormulaVersion,
		Tables:         tables,
	}
}

// buildForecastTables populates ForecastTables with one ForecastScenarioTable
// per in.Forecast.ScenarioResults entry, in that slice's own order, joining
// each scenario's ProjectedPeriods (P&L) against its CashFlow entries by
// Period label.
func buildForecastTables(in Input) ForecastTables {
	if !in.Forecast.Available || len(in.Forecast.ScenarioResults) == 0 {
		return ForecastTables{}
	}

	scenarios := make([]ForecastScenarioTable, 0, len(in.Forecast.ScenarioResults))
	for _, sr := range in.Forecast.ScenarioResults {
		fcfByPeriod := make(map[string]Value, len(sr.CashFlow))
		for _, cf := range sr.CashFlow {
			fcfByPeriod[cf.Period] = Value{Available: cf.FreeCashFlow.Available, Amount: cf.FreeCashFlow.Value}
		}

		periods := make([]ForecastPeriod, 0, len(sr.ProjectedPeriods))
		for _, pl := range sr.ProjectedPeriods {
			periods = append(periods, ForecastPeriod{
				Period:       pl.Period,
				PeriodNumber: pl.PeriodNumber,
				TotalRevenue: Value{Available: pl.TotalRevenue.Available, Amount: pl.TotalRevenue.Value},
				GrossProfit:  Value{Available: pl.GrossProfit.Available, Amount: pl.GrossProfit.Value},
				EBITDA:       Value{Available: pl.EBITDA.Available, Amount: pl.EBITDA.Value},
				NetIncome:    Value{Available: pl.NetIncome.Available, Amount: pl.NetIncome.Value},
				FreeCashFlow: fcfByPeriod[pl.Period],
			})
		}

		scenarios = append(scenarios, ForecastScenarioTable{
			Name:    sr.Name,
			Type:    string(sr.Type),
			Periods: periods,
		})
	}

	return ForecastTables{
		Available:      true,
		FormulaVersion: in.Forecast.FormulaVersion,
		Horizon:        in.Forecast.Horizon,
		Scenarios:      scenarios,
	}
}
