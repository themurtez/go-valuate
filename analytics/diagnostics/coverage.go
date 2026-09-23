package diagnostics

// moduleCategories names, for each of the 15 optional modules, which
// diagnostic Categories it would contribute Findings to when available —
// used only to build MissingDataArea.Categories for an unavailable module,
// never to gate mining itself (each mine* function makes its own
// Available check independently).
var moduleCategories = map[SourceModule][]Category{
	SourceMetrics:        {CategoryProfitability},
	SourceRatios:         {CategoryProfitability, CategoryLiquidity, CategoryLeverage},
	SourceQoE:            {CategoryEarningsQuality, CategoryProfitability},
	SourceWorkingCapital: {CategoryWorkingCapital},
	SourceCashFlow:       {CategoryCashConversion, CategoryLeverage, CategoryWorkingCapital},
	SourceRevenueQuality: {CategoryRevenueQuality, CategoryGrowth},
	SourceConcentration:  {CategoryConcentration},
	SourceAnomalies:      {CategoryOperationalCostControl, CategoryProfitability, CategoryEarningsQuality},
	SourceVariance:       {CategoryOperationalCostControl},
	SourceForecast:       {},
	SourceDebt:           {CategoryLeverage},
	SourceCovenants:      {CategoryLeverage},
	SourceBenchmarks:     {CategoryOperationalCostControl},
	SourceValueDrivers:   {CategoryValuation},
	SourceSaleReadiness:  {CategoryTransactionReadiness},
}

// moduleOrder is Input's field declaration order, used for
// Coverage.MissingModules and Result.MissingDataAreas so both always list
// modules in one fixed, stable sequence rather than Go map order.
var moduleOrder = []SourceModule{
	SourceMetrics,
	SourceRatios,
	SourceQoE,
	SourceWorkingCapital,
	SourceCashFlow,
	SourceRevenueQuality,
	SourceConcentration,
	SourceAnomalies,
	SourceVariance,
	SourceForecast,
	SourceDebt,
	SourceCovenants,
	SourceBenchmarks,
	SourceValueDrivers,
	SourceSaleReadiness,
}

// moduleAvailable reports whether in's field for module is available,
// mirroring salereadiness/management's identical ".Available field check
// for every Result that has one; len(Snapshots) == 0 only for
// metrics.Result" idiom.
func moduleAvailable(in Input, module SourceModule) bool {
	switch module {
	case SourceMetrics:
		return len(in.Metrics.Snapshots) > 0
	case SourceRatios:
		return in.Ratios.Available
	case SourceQoE:
		return in.QoE.Available
	case SourceWorkingCapital:
		return in.WorkingCapital.Available
	case SourceCashFlow:
		return in.CashFlow.Available
	case SourceRevenueQuality:
		return in.RevenueQuality.Available
	case SourceConcentration:
		return in.Concentration.Available
	case SourceAnomalies:
		return in.Anomalies.Available
	case SourceVariance:
		return in.Variance.Available
	case SourceForecast:
		return in.Forecast.Available
	case SourceDebt:
		return in.Debt.Available
	case SourceCovenants:
		return in.Covenants.Available
	case SourceBenchmarks:
		return in.Benchmarks.Available
	case SourceValueDrivers:
		return in.ValueDrivers.Available
	case SourceSaleReadiness:
		return in.SaleReadiness.Available
	default:
		return false
	}
}

// buildCoverage summarizes which of the 15 optional modules were
// available.
func buildCoverage(in Input) Coverage {
	c := Coverage{TotalModules: len(moduleOrder)}
	for _, m := range moduleOrder {
		if !moduleAvailable(in, m) {
			c.MissingModules = append(c.MissingModules, string(m))
			continue
		}
		c.AvailableModules++
		switch m {
		case SourceMetrics:
			c.WithMetrics = true
		case SourceRatios:
			c.WithRatios = true
		case SourceQoE:
			c.WithQoE = true
		case SourceWorkingCapital:
			c.WithWorkingCapital = true
		case SourceCashFlow:
			c.WithCashFlow = true
		case SourceRevenueQuality:
			c.WithRevenueQuality = true
		case SourceConcentration:
			c.WithConcentration = true
		case SourceAnomalies:
			c.WithAnomalies = true
		case SourceVariance:
			c.WithVariance = true
		case SourceForecast:
			c.WithForecast = true
		case SourceDebt:
			c.WithDebt = true
		case SourceCovenants:
			c.WithCovenants = true
		case SourceBenchmarks:
			c.WithBenchmarks = true
		case SourceValueDrivers:
			c.WithValueDrivers = true
		case SourceSaleReadiness:
			c.WithSaleReadiness = true
		}
	}
	c.CoveragePercent = float64(c.AvailableModules) / float64(c.TotalModules)
	return c
}

// buildMissingDataAreas builds one MissingDataArea per entry in
// coverage.MissingModules, in the same order.
func buildMissingDataAreas(coverage Coverage) []MissingDataArea {
	var out []MissingDataArea
	for _, m := range coverage.MissingModules {
		module := SourceModule(m)
		out = append(out, MissingDataArea{
			Module:     module,
			Categories: moduleCategories[module],
			Message:    string(module) + " was not supplied or was unavailable; the categories it would have contributed to have reduced coverage",
		})
	}
	return out
}
