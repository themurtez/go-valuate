package advisory

import "github.com/themurtez/go-valuate/accounting/labor"

// buildLaborSection composes LABOR from Input.Operating.Labor — task
// section 22. labor.Result has no top-level Available field (confirmed:
// this package treats a non-empty Periods slice as the availability
// signal, per the researched contract).
func buildLaborSection(in Input, policy Policy) Section {
	l := in.Operating.Labor
	if len(l.Periods) == 0 {
		return newUnavailableSection(SectionLabor, StatusNotSupplied)
	}

	latest := l.Periods[len(l.Periods)-1]
	period := latest.Period.Period
	var prior *labor.PeriodSummary
	if len(l.Periods) >= 2 {
		p := l.Periods[len(l.Periods)-2]
		prior = &p
	}

	var metricsOut []Metric
	metricsOut = append(metricsOut, newMetric("total_labor_cost", "Total Labor Cost", AvailableValue(latest.LaborCostBridge.TotalLaborCost), UnitCurrency, period, "labor", "labor_cost_bridge.total_labor_cost"))
	if latest.Productivity.LaborCostPercentRevenue.Available {
		curPct := AvailableValue(latest.Productivity.LaborCostPercentRevenue.Amount)
		m := newMetric("labor_cost_percent_revenue", "Labor Cost % of Revenue", curPct, UnitPercent, period, "labor", "productivity.labor_cost_percent_revenue")
		if prior != nil && prior.Productivity.LaborCostPercentRevenue.Available {
			priorPct := AvailableValue(prior.Productivity.LaborCostPercentRevenue.Amount)
			m.Prior = priorPct
			m.Change = computeChange(curPct, priorPct, true)
		}
		metricsOut = append(metricsOut, m)
	}
	if latest.FTE.Available {
		metricsOut = append(metricsOut, newMetric("fte", "FTE", AvailableValue(latest.FTE.FTE), UnitCount, period, "labor", "fte.fte"))
	}
	if latest.Headcount.Available {
		metricsOut = append(metricsOut, newMetric("headcount", "Headcount", AvailableValue(float64(latest.Headcount.EndingHeadcount)), UnitCount, period, "labor", "headcount.ending_headcount"))
	}
	if latest.Productivity.RevenuePerFTE.Available {
		metricsOut = append(metricsOut, newMetric("revenue_per_fte", "Revenue per FTE", AvailableValue(latest.Productivity.RevenuePerFTE.Amount), UnitCurrency, period, "labor", "productivity.revenue_per_fte"))
	}
	if latest.Productivity.GrossProfitPerFTE.Available {
		metricsOut = append(metricsOut, newMetric("gross_profit_per_fte", "Gross Profit per FTE", AvailableValue(latest.Productivity.GrossProfitPerFTE.Amount), UnitCurrency, period, "labor", "productivity.gross_profit_per_fte"))
	}
	if latest.Overtime.Available {
		metricsOut = append(metricsOut, newMetric("overtime_hours_percent", "Overtime Hours %", AvailableValue(latest.Overtime.OvertimeHoursPercent.Amount), UnitPercent, period, "labor", "overtime.overtime_hours_percent"))
	}
	if latest.ContractorMix.ContractorShareOfLaborCost.Available {
		metricsOut = append(metricsOut, newMetric("contractor_share", "Contractor Share of Labor Cost", AvailableValue(latest.ContractorMix.ContractorShareOfLaborCost.Amount), UnitPercent, period, "labor", "contractor_mix.contractor_share_of_labor_cost"))
	}

	var findings []Insight
	var actions []ActionItem
	sourceRef := []SourceRef{{Module: "labor", Period: period}}
	for _, f := range l.Flags {
		findings = append(findings, Insight{
			Code: string(f.Code), Category: string(SectionLabor), Severity: severityFromLaborFlag(f.Severity),
			Title: "Labor flag", Statement: f.Message, Period: period,
			SourceModule: "labor", SourceCode: string(f.Code),
			SourceRefs: []SourceRef{{Module: "labor", Code: string(f.Code), Period: period}},
		})
		switch f.Code {
		case "HIGH_OVERTIME_SHARE", "OVERTIME_INCREASING":
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewOvertimeTrend], PriorityMedium, "labor", string(f.Code), sourceRef, "", period))
		case "LABOR_COST_PERCENT_REVENUE_INCREASING":
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewLaborCostChange], PriorityMedium, "labor", string(f.Code), sourceRef, "", period))
		case "PAYROLL_GL_MISMATCH":
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewPayrollControlDifference], PriorityHigh, "labor", string(f.Code), sourceRef, "", period))
		}
	}

	return Section{
		Code: SectionLabor, Availability: StatusAvailable,
		Metrics: metricsOut, Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(dedupeActions(actions), policy),
		Sources: sourceRef,
	}
}

func severityFromLaborFlag(s labor.FlagSeverity) Severity {
	switch s {
	case labor.FlagSeverityCritical:
		return SeverityHigh
	case labor.FlagSeverityWarning:
		return SeverityMedium
	default:
		return SeverityInfo
	}
}
