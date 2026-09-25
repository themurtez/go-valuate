package advisory

import "github.com/themurtez/go-valuate/analytics/forecast"

// buildForecastSection composes FORECAST_AND_OUTLOOK from
// Input.Financial.Forecast — task section 27. Consumes
// analytics/forecast.Result verbatim; recomputes no projection.
// accounting/cashforecast is deliberately NOT re-consumed here (it
// already drives LIQUIDITY) — task section 27's "keep distinction:
// financial/operating forecast vs 13-week liquidity forecast; do not
// merge them into one model" rule. This section reads only the base
// ("BASE"-type, or the first) scenario's headline figures; a caller
// wanting every scenario reads Input.Financial.Forecast.ScenarioResults
// directly.
func buildForecastSection(in Input, policy Policy) Section {
	f := in.Financial.Forecast
	if !f.Available || len(f.ScenarioResults) == 0 {
		return newUnavailableSection(SectionForecastAndOutlook, StatusNotSupplied)
	}

	base := f.ScenarioResults[0]
	for _, s := range f.ScenarioResults {
		if s.Type == forecast.ScenarioTypeBase {
			base = s
			break
		}
	}
	if len(base.ProjectedPeriods) == 0 {
		return newUnavailableSection(SectionForecastAndOutlook, StatusUnavailable)
	}

	firstPeriod := base.ProjectedPeriods[0]
	lastPeriod := base.ProjectedPeriods[len(base.ProjectedPeriods)-1]

	var metricsOut []Metric
	if lastPeriod.TotalRevenue.Available {
		metricsOut = append(metricsOut, newMetric("forecast_revenue_end_of_horizon", "Forecast Revenue (End of Horizon)", AvailableValue(lastPeriod.TotalRevenue.Value), UnitCurrency, lastPeriod.Period, "forecast", "scenario_results.projected_periods.total_revenue"))
	}
	if lastPeriod.EBITDA.Available {
		metricsOut = append(metricsOut, newMetric("forecast_ebitda_end_of_horizon", "Forecast EBITDA (End of Horizon)", AvailableValue(lastPeriod.EBITDA.Value), UnitCurrency, lastPeriod.Period, "forecast", "scenario_results.projected_periods.ebitda"))
	}
	if firstPeriod.TotalRevenue.Available && lastPeriod.TotalRevenue.Available {
		change := computeChange(AvailableValue(lastPeriod.TotalRevenue.Value), AvailableValue(firstPeriod.TotalRevenue.Value), false)
		metricsOut = append(metricsOut, Metric{
			Code: "forecast_revenue_growth_over_horizon", Label: "Forecast Revenue Growth Over Horizon",
			Value: change.PercentChange, Unit: UnitPercent, Period: lastPeriod.Period,
			Prior: AvailableValue(firstPeriod.TotalRevenue.Value), Change: change,
			SourceModule: "forecast", SourceCode: "scenario_results.projected_periods.total_revenue",
		})
	}

	sourceRef := []SourceRef{{Module: "forecast", Period: lastPeriod.Period}}
	return Section{
		Code: SectionForecastAndOutlook, Availability: StatusAvailable,
		Metrics: metricsOut,
		Sources: sourceRef,
	}
}
