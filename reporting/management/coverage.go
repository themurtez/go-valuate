package management

// buildCoverage computes Coverage from in's 12 section-backing modules
// (Consensus is tracked but excluded from the percentage — see
// Coverage.WithConsensus's doc comment).
func buildCoverage(in Input) Coverage {
	c := Coverage{
		WithMetrics:        len(in.Metrics.Snapshots) > 0,
		WithRatios:         in.Ratios.Available,
		WithCashFlow:       in.CashFlow.Available,
		WithWorkingCapital: in.WorkingCapital.Available,
		WithQoE:            in.QoE.Available,
		WithVariance:       in.Variance.Available,
		WithForecast:       in.Forecast.Available,
		WithAnomalies:      in.Anomalies.Available,
		WithConcentration:  in.Concentration.Available,
		WithRevenueQuality: in.RevenueQuality.Available,
		WithDebt:           in.Debt.Available,
		WithCovenants:      in.Covenants.Available,
		WithConsensus:      in.Consensus.Available,
	}

	type module struct {
		name      string
		available bool
	}
	modules := []module{
		{"Metrics", c.WithMetrics},
		{"Ratios", c.WithRatios},
		{"CashFlow", c.WithCashFlow},
		{"WorkingCapital", c.WithWorkingCapital},
		{"QoE", c.WithQoE},
		{"Variance", c.WithVariance},
		{"Forecast", c.WithForecast},
		{"Anomalies", c.WithAnomalies},
		{"Concentration", c.WithConcentration},
		{"RevenueQuality", c.WithRevenueQuality},
		{"Debt", c.WithDebt},
		{"Covenants", c.WithCovenants},
	}

	c.TotalModules = len(modules)
	for _, m := range modules {
		if m.available {
			c.AvailableModules++
		} else {
			c.MissingModules = append(c.MissingModules, m.name)
		}
	}
	c.CoveragePercent = float64(c.AvailableModules) / float64(c.TotalModules)

	return c
}

// buildModuleVersions echoes this package's own FormulaVersion plus every
// contributing sibling's own version, in Coverage's fixed module order
// plus Consensus.
func buildModuleVersions(in Input) ModuleVersions {
	versionIfAvailable := func(available bool, version string) string {
		if !available {
			return ""
		}
		return version
	}

	return ModuleVersions{
		FormulaVersion: FormulaVersion,
		Modules: []ModuleVersion{
			{Module: "metrics", Version: versionIfAvailable(len(in.Metrics.Snapshots) > 0, in.Metrics.FormulaVersion)},
			{Module: "ratios", Version: versionIfAvailable(in.Ratios.Available, in.Ratios.FormulaVersion)},
			{Module: "cash_flow", Version: versionIfAvailable(in.CashFlow.Available, in.CashFlow.FormulaVersion)},
			{Module: "working_capital", Version: versionIfAvailable(in.WorkingCapital.Available, in.WorkingCapital.FormulaVersion)},
			{Module: "qoe", Version: versionIfAvailable(in.QoE.Available, in.QoE.FormulaVersion)},
			{Module: "variance", Version: versionIfAvailable(in.Variance.Available, in.Variance.FormulaVersion)},
			{Module: "forecast", Version: versionIfAvailable(in.Forecast.Available, in.Forecast.FormulaVersion)},
			{Module: "anomalies", Version: versionIfAvailable(in.Anomalies.Available, in.Anomalies.FormulaVersion)},
			{Module: "concentration", Version: versionIfAvailable(in.Concentration.Available, in.Concentration.FormulaVersion)},
			{Module: "revenue_quality", Version: versionIfAvailable(in.RevenueQuality.Available, in.RevenueQuality.FormulaVersion)},
			{Module: "debt", Version: versionIfAvailable(in.Debt.Available, in.Debt.FormulaVersion)},
			{Module: "covenants", Version: versionIfAvailable(in.Covenants.Available, in.Covenants.FormulaVersion)},
			{Module: "consensus", Version: versionIfAvailable(in.Consensus.Available, in.Consensus.FormulaVersion)},
		},
	}
}
