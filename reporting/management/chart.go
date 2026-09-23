package management

import "github.com/themurtez/go-valuate/analytics/forecast"

// buildChartSeries builds a fixed, ordered set of ChartSeries from
// whichever of report's already-built sections have data: Total Revenue,
// EBITDA, and Net Income from HistoricalSeries; EBITDA Margin from
// ProfitabilitySeries; Free Cash Flow from CashFlowSeries; Net Working
// Capital from WorkingCapitalSeries — each extended with a forecast tail
// (Total Revenue and EBITDA only, since those are the two figures every
// forecast scenario in analytics/forecast.PeriodPL always populates) from
// in.Forecast's first scenario when available. A series with zero points
// is omitted from Series entirely.
func buildChartSeries(in Input, report Report) ChartSeriesSection {
	var series []ChartSeries

	forecastRevenue := func(pl forecast.PeriodPL) Value {
		return Value{Available: pl.TotalRevenue.Available, Amount: pl.TotalRevenue.Value}
	}
	forecastEBITDA := func(pl forecast.PeriodPL) Value {
		return Value{Available: pl.EBITDA.Available, Amount: pl.EBITDA.Value}
	}

	if s := chartFromHistorical(report.HistoricalSeries, "Total Revenue", func(p HistoricalPeriod) Value { return p.TotalRevenue }, forecastRevenue, in); len(s.Points) > 0 {
		series = append(series, s)
	}
	if s := chartFromHistorical(report.HistoricalSeries, "EBITDA", func(p HistoricalPeriod) Value { return p.EBITDA }, forecastEBITDA, in); len(s.Points) > 0 {
		series = append(series, s)
	}
	if s := chartFromHistoricalOnly(report.HistoricalSeries, "Net Income", func(p HistoricalPeriod) Value { return p.NetIncome }); len(s.Points) > 0 {
		series = append(series, s)
	}

	if len(report.ProfitabilitySeries.Periods) > 0 {
		points := make([]ChartPoint, 0, len(report.ProfitabilitySeries.Periods))
		for _, p := range report.ProfitabilitySeries.Periods {
			if p.EBITDAMargin.Available {
				points = append(points, ChartPoint{X: string(p.Period), Y: p.EBITDAMargin})
			}
		}
		if len(points) > 0 {
			series = append(series, ChartSeries{Label: "EBITDA Margin", Unit: UnitPercent, Source: report.ProfitabilitySeries.Source, Points: points})
		}
	}

	if len(report.CashFlowSeries.Periods) > 0 {
		points := make([]ChartPoint, 0, len(report.CashFlowSeries.Periods))
		for _, p := range report.CashFlowSeries.Periods {
			if p.FreeCashFlow.Available {
				points = append(points, ChartPoint{X: string(p.Period), Y: p.FreeCashFlow})
			}
		}
		if len(points) > 0 {
			series = append(series, ChartSeries{Label: "Free Cash Flow", Unit: UnitCurrency, Source: "cash_flow", Points: points})
		}
	}

	if len(report.WorkingCapitalSeries.Periods) > 0 {
		points := make([]ChartPoint, 0, len(report.WorkingCapitalSeries.Periods))
		for _, p := range report.WorkingCapitalSeries.Periods {
			if p.NWC.Available {
				points = append(points, ChartPoint{X: string(p.Period), Y: p.NWC})
			}
		}
		if len(points) > 0 {
			series = append(series, ChartSeries{Label: "Net Working Capital", Unit: UnitCurrency, Source: "working_capital", Points: points})
		}
	}

	return ChartSeriesSection{
		Available: len(series) > 0,
		Series:    series,
	}
}

// chartFromHistoricalOnly builds a ChartSeries purely from
// HistoricalSeries, with no forecast tail — used for figures
// analytics/forecast.PeriodPL does not reliably populate (e.g. NetIncome,
// which depends on interest/tax assumptions a caller may not have
// supplied to Forecast).
func chartFromHistoricalOnly(hs HistoricalSeries, label string, extract func(HistoricalPeriod) Value) ChartSeries {
	if !hs.Available {
		return ChartSeries{Label: label, Unit: UnitCurrency}
	}
	points := make([]ChartPoint, 0, len(hs.Periods))
	for _, p := range hs.Periods {
		if v := extract(p); v.Available {
			points = append(points, ChartPoint{X: string(p.Period), Y: v})
		}
	}
	return ChartSeries{Label: label, Unit: UnitCurrency, Source: "metrics", Points: points}
}

// chartFromHistorical builds a ChartSeries from HistoricalSeries via
// extract, then appends a forecast tail from in.Forecast's first
// ScenarioResults entry via forecastExtract when Forecast is available.
// Taking forecastExtract as an explicit parameter (rather than dispatching
// on label with a string switch) means a caller adding a new
// chartFromHistorical call for a figure with no forecast counterpart, or
// renaming an existing label, cannot silently drop the forecast tail —
// the compiler requires a forecastExtract argument at every call site.
func chartFromHistorical(hs HistoricalSeries, label string, extract func(HistoricalPeriod) Value, forecastExtract func(forecast.PeriodPL) Value, in Input) ChartSeries {
	var points []ChartPoint
	if hs.Available {
		points = make([]ChartPoint, 0, len(hs.Periods))
		for _, p := range hs.Periods {
			if v := extract(p); v.Available {
				points = append(points, ChartPoint{X: string(p.Period), Y: v})
			}
		}
	}
	hasHistoricalPoints := len(points) > 0

	hasForecastPoints := false
	if in.Forecast.Available && len(in.Forecast.ScenarioResults) > 0 {
		sr := in.Forecast.ScenarioResults[0]
		forecastPoints := make([]ChartPoint, 0, len(sr.ProjectedPeriods))
		for _, pl := range sr.ProjectedPeriods {
			if v := forecastExtract(pl); v.Available {
				forecastPoints = append(forecastPoints, ChartPoint{X: pl.Period, Y: v, IsForecast: true})
			}
		}
		if len(forecastPoints) > 0 {
			points = append(points, forecastPoints...)
			hasForecastPoints = true
		}
	}

	source := ""
	switch {
	case hasHistoricalPoints && hasForecastPoints:
		source = "metrics+forecast"
	case hasHistoricalPoints:
		source = "metrics"
	case hasForecastPoints:
		source = "forecast"
	}

	return ChartSeries{Label: label, Unit: UnitCurrency, Source: source, Points: points}
}
