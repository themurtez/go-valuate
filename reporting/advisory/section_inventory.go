package advisory

// buildInventorySection composes INVENTORY from
// Input.Operating.Inventory — task section 23. Uses the point-in-time
// (AsOfDate) portion for portfolio/aging/stock-policy facts, and the
// periodic portion (Periods) for turnover/DIO — accounting/inventory
// carries both, per the researched contract.
func buildInventorySection(in Input, policy Policy) Section {
	inv := in.Operating.Inventory
	if !inv.Available {
		return newUnavailableSection(SectionInventory, StatusNotSupplied)
	}

	period := currentPeriodLabel(in)
	if inv.AsOfDate != "" {
		period = inv.AsOfDate
	}

	var metricsOut []Metric
	metricsOut = append(metricsOut, newMetric(metricCodeInventoryValue, "Inventory Value", AvailableValue(inv.Portfolio.TotalInventoryValue), UnitCurrency, period, "inventory", "portfolio.total_inventory_value"))

	if latest, ok := latestInventoryPeriod(inv); ok {
		if latest.Turnover.Available {
			metricsOut = append(metricsOut, newMetric("inventory_turnover", "Inventory Turnover", AvailableValue(latest.Turnover.Value), UnitMultiple, latest.Period.Period, "inventory", "turnover.value"))
		}
		if latest.DIO.Available {
			metricsOut = append(metricsOut, newMetric(metricCodeDIO, "Days Inventory Outstanding", AvailableValue(latest.DIO.Value), UnitDays, latest.Period.Period, "inventory", "dio.value"))
		}
	}

	if inv.Aging.Available {
		if inv.Aging.SlowMovingPercent.Available {
			metricsOut = append(metricsOut, newMetric("slow_moving_percent", "Slow-Moving Inventory %", AvailableValue(inv.Aging.SlowMovingPercent.Amount), UnitPercent, period, "inventory", "aging.slow_moving_percent"))
		}
		if inv.Aging.NonMovingPercent.Available {
			metricsOut = append(metricsOut, newMetric("non_moving_percent", "Non-Moving Inventory %", AvailableValue(inv.Aging.NonMovingPercent.Amount), UnitPercent, period, "inventory", "aging.non_moving_percent"))
		}
	}

	var findings []Insight
	var actions []ActionItem
	sourceRef := []SourceRef{{Module: "inventory", Period: period}}

	if inv.Aging.Available && inv.Aging.SlowMovingMateriality.Material {
		findings = append(findings, Insight{
			Code: "SLOW_MOVING_INVENTORY", Category: string(SectionInventory), Severity: SeverityMedium,
			Title: "Slow-moving inventory", Statement: statementValueChanged("Slow-moving inventory value", 0, inv.Aging.SlowMovingValue.Amount),
			Period: period, Current: AvailableValue(inv.Aging.SlowMovingValue.Amount),
			SourceModule: "inventory", SourceCode: "aging.slow_moving_materiality", SourceRefs: sourceRef,
		})
		actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewSlowMovingInventory], PriorityMedium, "inventory", "slow_moving", sourceRef, "", period))
	}

	if inv.Reconciliation.Available && !inv.Reconciliation.AllReconciled {
		actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewInventoryControlDifference], PriorityHigh, "inventory", "reconciliation", sourceRef, "", period))
	}

	for _, sp := range inv.StockPolicyResults {
		if sp.BelowMinimum.Available && sp.BelowMinimum.Value || sp.AboveMaximum.Available && sp.AboveMaximum.Value {
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewStockPolicyException], PriorityLow, "inventory", "stock_policy_exception", []SourceRef{{Module: "inventory", Ref: sp.ItemID, Period: period}}, sp.ItemID, period))
		}
	}

	for _, f := range inv.Flags {
		findings = append(findings, Insight{
			Code: string(f.Code), Category: string(SectionInventory), Severity: SeverityMedium,
			Title: "Inventory flag", Statement: f.Message, Period: period, EntityRef: f.ItemID,
			SourceModule: "inventory", SourceCode: string(f.Code),
			SourceRefs: []SourceRef{{Module: "inventory", Code: string(f.Code), Ref: f.ItemID, Period: period}},
		})
	}

	return Section{
		Code: SectionInventory, Availability: StatusAvailable,
		Metrics: metricsOut, Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(dedupeActions(actions), policy),
		Sources: sourceRef,
	}
}
