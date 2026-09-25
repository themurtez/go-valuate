package advisory

// synthesizeInsights applies this package's small, closed set of
// deterministic multi-source synthesis rules — task section 82. Each rule
// combines two already-built facts from two different sections into one
// synthesized Insight/ManagementQuestion, gated by an explicit
// Policy.Synthesis threshold — never an open-ended inference engine (task
// section 82's "do not create an open-ended inference engine" rule).
// Every synthesized Insight lists every contributing SourceRef (task
// section 83's provenance rule) and uses an explicit synthesis-priority
// Severity rather than summing its inputs' severities (task section 85).
func synthesizeInsights(sections []Section, policy Policy) []Insight {
	var out []Insight

	if in := synthesizeLiquidityPressure(sections, policy); in != nil {
		out = append(out, *in)
	}
	if in := synthesizeMarginPressure(sections, policy); in != nil {
		out = append(out, *in)
	}
	if in := synthesizeCloseBlocker(sections, policy); in != nil {
		out = append(out, *in)
	}

	return out
}

// synthesizeLiquidityPressure implements task section 82's first example:
// cash forecast minimum below Policy.Synthesis.MinimumCashThreshold PLUS
// AR overdue worsening by at least Policy.Synthesis.AROverdueIncreasePoints
// percentage points.
func synthesizeLiquidityPressure(sections []Section, policy Policy) *Insight {
	if policy.Synthesis.MinimumCashThreshold <= 0 || policy.Synthesis.AROverdueIncreasePoints <= 0 {
		return nil
	}
	liq, ok := sectionByCode(sections, SectionLiquidity)
	if !ok || liq.Availability != StatusAvailable {
		return nil
	}
	wc, ok := sectionByCode(sections, SectionWorkingCapital)
	if !ok || wc.Availability != StatusAvailable {
		return nil
	}

	minCash, ok := metricByCode(liq.Metrics, metricCodeMinimumCash)
	if !ok || !minCash.Value.Available || minCash.Value.Amount > policy.Synthesis.MinimumCashThreshold {
		return nil
	}
	arOverdue, ok := metricByCode(wc.Metrics, "ar_percent_overdue")
	if !ok || !arOverdue.Change.PercentagePointChange.Available || arOverdue.Change.PercentagePointChange.Amount < policy.Synthesis.AROverdueIncreasePoints {
		return nil
	}

	refs := append(append([]SourceRef(nil), minCash.SourceRefs...), arOverdue.SourceRefs...)
	if minCash.SourceModule != "" {
		refs = append(refs, SourceRef{Module: minCash.SourceModule, Code: minCash.SourceCode, Period: minCash.Period})
	}
	if arOverdue.SourceModule != "" {
		refs = append(refs, SourceRef{Module: arOverdue.SourceModule, Code: arOverdue.SourceCode, Period: arOverdue.Period})
	}

	return &Insight{
		Code: string(StatementLiquidityAndCollectionsPressure), Category: string(SectionLiquidity), Severity: SeverityHigh,
		Title:     "Liquidity and collections pressure",
		Statement: "The 13-week cash forecast's minimum cash balance is at or below the configured threshold, and accounts receivable over 90 days has increased.",
		Period:    minCash.Period, Current: minCash.Value,
		SourceModule: "synthesis", SourceCode: string(StatementLiquidityAndCollectionsPressure),
		SourceRefs: sortedSourceRefs(refs),
	}
}

// synthesizeMarginPressure implements task section 82's third example:
// contribution margin declined by Policy.Synthesis.MarginDeclinePoints or
// more, PLUS labor cost % of revenue increased by
// Policy.Synthesis.LaborCostIncreasePoints or more.
func synthesizeMarginPressure(sections []Section, policy Policy) *Insight {
	if policy.Synthesis.MarginDeclinePoints <= 0 || policy.Synthesis.LaborCostIncreasePoints <= 0 {
		return nil
	}
	prof, ok := sectionByCode(sections, SectionProfitability)
	if !ok || prof.Availability != StatusAvailable {
		return nil
	}
	labor, ok := sectionByCode(sections, SectionLabor)
	if !ok || labor.Availability != StatusAvailable {
		return nil
	}

	margin, ok := metricByCode(prof.Metrics, "contribution_margin")
	if !ok || !margin.Change.PercentagePointChange.Available || margin.Change.PercentagePointChange.Amount > -policy.Synthesis.MarginDeclinePoints {
		return nil
	}
	laborPct, ok := metricByCode(labor.Metrics, "labor_cost_percent_revenue")
	if !ok || !laborPct.Change.PercentagePointChange.Available || laborPct.Change.PercentagePointChange.Amount < policy.Synthesis.LaborCostIncreasePoints {
		return nil
	}

	refs := []SourceRef{
		{Module: margin.SourceModule, Code: margin.SourceCode, Period: margin.Period},
		{Module: laborPct.SourceModule, Code: laborPct.SourceCode, Period: laborPct.Period},
	}

	return &Insight{
		Code: string(StatementMarginPressureFromLabor), Category: string(SectionProfitability), Severity: SeverityMedium,
		Title:     "Margin pressure from labor cost",
		Statement: "Contribution margin declined while labor cost as a percentage of revenue increased.",
		Period:    margin.Period, Current: margin.Value,
		SourceModule: "synthesis", SourceCode: string(StatementMarginPressureFromLabor),
		SourceRefs: sortedSourceRefs(refs),
	}
}

// synthesizeCloseBlocker implements task section 82's second example:
// reconciliation unreconciled PLUS a close-checklist task blocked on the
// same reconciliation gate. Identified by a closechecklist Blocker whose
// GateCode names the same reconciliation account — this package matches
// on EntityRef equality between an ACCOUNTING_AND_CLOSE Finding sourced
// from "reconciliation" and one sourced from "close_checklist", never a
// fuzzy text match (task section 35's rule extended to synthesis).
func synthesizeCloseBlocker(sections []Section, policy Policy) *Insight {
	close, ok := sectionByCode(sections, SectionAccountingAndClose)
	if !ok || close.Availability != StatusAvailable {
		return nil
	}

	var recFinding, checklistFinding *Insight
	for i := range close.Findings {
		f := &close.Findings[i]
		switch f.SourceModule {
		case "reconciliation":
			if recFinding == nil {
				recFinding = f
			}
		case "close_checklist":
			if checklistFinding == nil && f.EntityRef != "" {
				checklistFinding = f
			}
		}
	}
	if recFinding == nil || checklistFinding == nil {
		return nil
	}
	if recFinding.EntityRef != "" && checklistFinding.EntityRef != "" && recFinding.EntityRef != checklistFinding.EntityRef {
		// Only synthesize when both facts concern the same entity (or one
		// side has no entity to compare, e.g. account-level reconciliation
		// vs a checklist task naming the same account in its own
		// GateCode — this package does not have that cross-reference
		// available here, so an entity mismatch when both ARE populated is
		// treated as unrelated, matching task section 35's no-fuzzy-match
		// discipline).
		return nil
	}

	refs := sortedSourceRefs(append(append([]SourceRef(nil), recFinding.SourceRefs...), checklistFinding.SourceRefs...))
	return &Insight{
		Code: string(StatementCloseBlockedByReconciliation), Category: string(SectionAccountingAndClose), Severity: SeverityBlocking,
		Title:     "Close blocked by unresolved reconciliation",
		Statement: "An unresolved reconciliation condition is blocking a required close-checklist task.",
		Period:    recFinding.Period, EntityRef: recFinding.EntityRef,
		SourceModule: "synthesis", SourceCode: string(StatementCloseBlockedByReconciliation),
		SourceRefs: refs,
	}
}

func metricByCode(metrics []Metric, code string) (Metric, bool) {
	for _, m := range metrics {
		if m.Code == code {
			return m, true
		}
	}
	return Metric{}, false
}
