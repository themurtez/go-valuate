package advisory

// buildWorkingCapitalSection composes WORKING_CAPITAL from
// Input.Operating.AR/AP/Inventory and Input.Financial.Ratios/
// WorkingCapital — task section 18. Where a metric exists in more than
// one supplied source (DSO: ar vs. ratios; DPO: ap vs. ratios; DIO:
// inventory vs. ratios), resolveSourcedMetric applies Policy's explicit
// precedence — task section 19/59's "do not average conflicting
// versions" rule.
func buildWorkingCapitalSection(in Input, policy Policy) (Section, []Issue) {
	ar, hasAR := in.Operating.AR, in.Operating.AR.Available
	ap, hasAP := in.Operating.AP, in.Operating.AP.Available
	period := currentPeriodLabel(in)

	if !hasAR && !hasAP && !in.Financial.WorkingCapital.Available && !in.Financial.Ratios.Available && !in.Operating.Inventory.Available {
		return newUnavailableSection(SectionWorkingCapital, StatusNotSupplied), nil
	}

	var metricsOut []Metric
	var findings []Insight
	var actions []ActionItem
	var issues []Issue
	usedModules := map[string]bool{}

	// DSO: ar.Result.DSO vs analytics/ratios latest period — source
	// precedence example from task section 19.
	dsoCandidates := map[string]candidateValue{}
	if hasAR && ar.DSO.Available {
		dsoCandidates["ar"] = candidateValue{Value: AvailableValue(ar.DSO.Value), SourceCode: "dso"}
		usedModules["ar"] = true
	}
	if r, ok := latestPeriodRatios(in.Financial.Ratios, period); ok && r.DaysSalesOutstanding.Value.Available {
		dsoCandidates["ratios"] = candidateValue{Value: AvailableValue(r.DaysSalesOutstanding.Value.Value), SourceCode: "days_sales_outstanding"}
	}
	if m, iss := resolveSourcedMetric(metricCodeDSO, "Days Sales Outstanding", UnitDays, period, dsoCandidates, defaultOrder(SourceOrderDSO), policy); m != nil {
		metricsOut = append(metricsOut, *m)
	} else if iss != nil {
		issues = append(issues, *iss)
	}

	// DPO: ap.Result.DPO vs analytics/ratios.
	dpoCandidates := map[string]candidateValue{}
	if hasAP && ap.DPO.Available {
		dpoCandidates["ap"] = candidateValue{Value: AvailableValue(ap.DPO.Value), SourceCode: "dpo"}
		usedModules["ap"] = true
	}
	if r, ok := latestPeriodRatios(in.Financial.Ratios, period); ok && r.DaysPayableOutstanding.Value.Available {
		dpoCandidates["ratios"] = candidateValue{Value: AvailableValue(r.DaysPayableOutstanding.Value.Value), SourceCode: "days_payable_outstanding"}
	}
	if m, iss := resolveSourcedMetric(metricCodeDPO, "Days Payable Outstanding", UnitDays, period, dpoCandidates, defaultOrder(SourceOrderDPO), policy); m != nil {
		metricsOut = append(metricsOut, *m)
	} else if iss != nil {
		issues = append(issues, *iss)
	}

	// DIO: accounting/inventory turnover vs analytics/ratios.
	dioCandidates := map[string]candidateValue{}
	if in.Operating.Inventory.Available {
		if latest, ok := latestInventoryPeriod(in.Operating.Inventory); ok && latest.DIO.Available {
			dioCandidates["inventory"] = candidateValue{Value: AvailableValue(latest.DIO.Value), SourceCode: "dio"}
			usedModules["inventory"] = true
		}
	}
	if r, ok := latestPeriodRatios(in.Financial.Ratios, period); ok && r.DaysInventoryOutstanding.Value.Available {
		dioCandidates["ratios"] = candidateValue{Value: AvailableValue(r.DaysInventoryOutstanding.Value.Value), SourceCode: "days_inventory_outstanding"}
	}
	if m, iss := resolveSourcedMetric(metricCodeDIO, "Days Inventory Outstanding", UnitDays, period, dioCandidates, defaultOrder(SourceOrderDIO), policy); m != nil {
		metricsOut = append(metricsOut, *m)
	} else if iss != nil {
		issues = append(issues, *iss)
	}

	if r, ok := latestPeriodRatios(in.Financial.Ratios, period); ok && r.CashConversionCycle.Value.Available {
		usedModules["ratios"] = true
		metricsOut = append(metricsOut, newMetric("cash_conversion_cycle", "Cash Conversion Cycle", AvailableValue(r.CashConversionCycle.Value.Value), UnitDays, period, "ratios", "cash_conversion_cycle"))
	}

	if hasAR {
		usedModules["ar"] = true
		metricsOut = append(metricsOut, newMetric(metricCodeAR, "AR Open Balance", AvailableValue(ar.PortfolioSummary.TotalOpenReceivables), UnitCurrency, period, "ar", "portfolio_summary.total_open_receivables"))
		if ar.PortfolioSummary.PercentOverdue.Available {
			metricsOut = append(metricsOut, newMetric("ar_percent_overdue", "AR % Overdue", AvailableValue(ar.PortfolioSummary.PercentOverdue.Value), UnitPercent, period, "ar", "portfolio_summary.percent_overdue"))
		}
		ins, act := findingsFromARFlags(ar, period)
		findings = append(findings, ins...)
		actions = append(actions, act...)
		if bucket90, ok := ar90PlusShare(ar); ok && bucket90 > 0 {
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewOverdueAR], PriorityMedium, "ar", "over_90_days", []SourceRef{{Module: "ar", Period: period}}, "", period))
		}
		if !ar.AgingReconciliation.Balanced {
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewARControlReconciliation], PriorityHigh, "ar", "aging_reconciliation", []SourceRef{{Module: "ar", Period: period}}, "", period))
		}
		if ar.ControlAccountReconciliation.Available && !ar.ControlAccountReconciliation.Reconciled {
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewARControlReconciliation], PriorityHigh, "ar", "control_account_reconciliation", []SourceRef{{Module: "ar", Period: period}}, "", period))
		}
	}

	if hasAP {
		usedModules["ap"] = true
		metricsOut = append(metricsOut, newMetric(metricCodeAP, "AP Open Balance", AvailableValue(ap.PortfolioSummary.TotalOpenPayables), UnitCurrency, period, "ap", "portfolio_summary.total_open_payables"))
		if ap.PortfolioSummary.PercentOverdue.Available {
			metricsOut = append(metricsOut, newMetric("ap_percent_overdue", "AP % Overdue", AvailableValue(ap.PortfolioSummary.PercentOverdue.Value), UnitPercent, period, "ap", "portfolio_summary.percent_overdue"))
		}
		ins, act := findingsFromAPFlags(ap, period)
		findings = append(findings, ins...)
		actions = append(actions, act...)
		if !ap.AgingReconciliation.Balanced {
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewAPControlReconciliation], PriorityHigh, "ap", "aging_reconciliation", []SourceRef{{Module: "ap", Period: period}}, "", period))
		}
		if ap.ControlAccountReconciliation.Available && !ap.ControlAccountReconciliation.Reconciled {
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewAPControlReconciliation], PriorityHigh, "ap", "control_account_reconciliation", []SourceRef{{Module: "ap", Period: period}}, "", period))
		}
	}

	if in.Operating.Inventory.Available {
		usedModules["inventory"] = true
		metricsOut = append(metricsOut, newMetric(metricCodeInventoryValue, "Inventory Value", AvailableValue(in.Operating.Inventory.Portfolio.TotalInventoryValue), UnitCurrency, period, "inventory", "portfolio.total_inventory_value"))
	}

	if in.Financial.WorkingCapital.Available && len(in.Financial.WorkingCapital.History) > 0 {
		usedModules["working_capital"] = true
		latest := in.Financial.WorkingCapital.History[len(in.Financial.WorkingCapital.History)-1]
		if latest.NWC.Available {
			metricsOut = append(metricsOut, newMetric("nwc", "Net Working Capital", AvailableValue(latest.NWC.Value), UnitCurrency, string(latest.Period), "working_capital", "nwc"))
		}
	}

	sources := make([]SourceRef, 0, len(usedModules))
	for m := range usedModules {
		sources = append(sources, SourceRef{Module: m, Period: period})
	}

	return Section{
		Code: SectionWorkingCapital, Availability: StatusAvailable,
		Metrics: metricsOut, Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(dedupeActions(actions), policy),
		Sources: sortedSourceRefs(sources),
	}, issues
}
