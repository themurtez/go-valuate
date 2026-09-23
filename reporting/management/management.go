package management

// Calculate derives a full Report from in. It never mutates any
// caller-owned input and performs no I/O.
//
// Calculate recomputes no figure of any kind — every number in the
// returned Report is read directly from an already-computed sibling
// Result in in, never re-derived by this package's own formula. See the
// package doc comment for the full rationale.
func Calculate(in Input) Report {
	report := Report{FormulaVersion: FormulaVersion}

	hasAnyInput := len(in.Metrics.Snapshots) > 0 || in.Ratios.Available || in.CashFlow.Available ||
		in.WorkingCapital.Available || in.QoE.Available || in.Variance.Available || in.Forecast.Available ||
		in.Anomalies.Available || in.Concentration.Available || in.RevenueQuality.Available ||
		in.Debt.Available || in.Covenants.Available || in.Consensus.Available || len(in.Dataset.Items) > 0
	if !hasAnyInput {
		report.Errors = []Issue{{
			Code:     IssueNoInputSupplied,
			Severity: IssueSeverityError,
			Message:  "no financial or analytical input supplied; nothing to report",
		}}
		return report
	}
	report.Available = true

	var issues []Issue
	if len(in.Metrics.Snapshots) == 0 {
		issues = append(issues, Issue{Code: IssueMetricsUnavailable, Severity: IssueSeverityWarning,
			Message: "Metrics.Snapshots is empty; historical series and several executive KPIs are unavailable"})
	}
	if !in.Ratios.Available {
		issues = append(issues, Issue{Code: IssueRatiosUnavailable, Severity: IssueSeverityWarning,
			Message: "Ratios.Available is false; liquidity/leverage series is unavailable and profitability series falls back to metrics only"})
	}
	if !in.CashFlow.Available {
		issues = append(issues, Issue{Code: IssueCashFlowUnavailable, Severity: IssueSeverityWarning,
			Message: "CashFlow.Available is false; cash-flow series is unavailable"})
	}
	if !in.WorkingCapital.Available {
		issues = append(issues, Issue{Code: IssueWorkingCapitalUnavailable, Severity: IssueSeverityWarning,
			Message: "WorkingCapital.Available is false; working-capital series is unavailable"})
	}
	if !in.QoE.Available {
		issues = append(issues, Issue{Code: IssueQoEUnavailable, Severity: IssueSeverityWarning,
			Message: "QoE.Available is false; QoE-sourced executive KPIs and top issues are unavailable"})
	}
	if !in.Variance.Available {
		issues = append(issues, Issue{Code: IssueVarianceUnavailable, Severity: IssueSeverityWarning,
			Message: "Variance.Available is false; variance tables are unavailable"})
	}
	if !in.Forecast.Available {
		issues = append(issues, Issue{Code: IssueForecastUnavailable, Severity: IssueSeverityWarning,
			Message: "Forecast.Available is false; forecast tables and the forecast portion of chart series are unavailable"})
	}
	if !in.Anomalies.Available {
		issues = append(issues, Issue{Code: IssueAnomaliesUnavailable, Severity: IssueSeverityWarning,
			Message: "Anomalies.Available is false; anomaly-sourced top issues are unavailable"})
	}
	if !in.Concentration.Available {
		issues = append(issues, Issue{Code: IssueConcentrationUnavailable, Severity: IssueSeverityWarning,
			Message: "Concentration.Available is false; concentration-sourced executive KPIs are unavailable"})
	}
	if !in.RevenueQuality.Available {
		issues = append(issues, Issue{Code: IssueRevenueQualityUnavailable, Severity: IssueSeverityWarning,
			Message: "RevenueQuality.Available is false; revenue-quality-sourced executive KPIs are unavailable"})
	}
	if !in.Debt.Available {
		issues = append(issues, Issue{Code: IssueDebtUnavailable, Severity: IssueSeverityWarning,
			Message: "Debt.Available is false; debt-sourced top issues are unavailable"})
	}
	if !in.Covenants.Available {
		issues = append(issues, Issue{Code: IssueCovenantsUnavailable, Severity: IssueSeverityWarning,
			Message: "Covenants.Available is false; covenant-breach top issues are unavailable"})
	}
	if !in.Consensus.Available {
		issues = append(issues, Issue{Code: IssueConsensusUnavailable, Severity: IssueSeverityWarning,
			Message: "Consensus.Available is false; valuation executive KPIs are unavailable"})
	}

	var historicalOrderIssue *Issue
	report.HistoricalSeries, historicalOrderIssue = buildHistoricalSeries(in)
	if historicalOrderIssue != nil {
		issues = append(issues, *historicalOrderIssue)
	}
	report.ProfitabilitySeries = buildProfitabilitySeries(in, report.HistoricalSeries)
	report.LiquidityLeverageSeries = buildLiquidityLeverageSeries(in)
	report.CashFlowSeries = buildCashFlowSeries(in)
	report.WorkingCapitalSeries = buildWorkingCapitalSeries(in)
	report.VarianceTables = buildVarianceTables(in)
	report.ForecastTables = buildForecastTables(in)
	report.TopIssues = buildTopIssues(in)
	report.ExecutiveSummary = buildExecutiveSummary(in, report)
	report.ChartSeries = buildChartSeries(in, report)
	report.Coverage = buildCoverage(in)
	report.Versions = buildModuleVersions(in)

	for i := range issues {
		if issues[i].Severity == IssueSeverityError {
			report.Errors = append(report.Errors, issues[i])
		} else {
			report.Warnings = append(report.Warnings, issues[i])
		}
	}

	return report
}
