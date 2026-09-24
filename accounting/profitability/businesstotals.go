package profitability

// buildBusinessTotalsResult computes BusinessTotals directly from the
// fact/summary population and shared-cost pools — task section 35, never
// by summing DimensionView totals.
//
// Facts and summary rows are combined per PERIOD ONLY (dimension-
// independent), NOT gated by whether the period has any fact at all: a
// period can legitimately contain both a JOB-dimension Fact for one
// piece of economic activity and a CUSTOMER-dimension summary row for a
// wholly separate piece of activity that was never also recorded as a
// Fact, and both must count toward that period's business total. What
// must never happen is double counting the SAME activity twice — that
// is prevented one level up, in Calculate, via
// detectInputModeConflicts, which already drops any summary row whose
// own (Dimension, EntityID, Period) slot has a Fact attributing to it
// (see calculate.go's validSummaries2). Since a summary row is
// dimension-scoped by construction, this function still applies the
// task section 35 "each dimension is an independent lens over the same
// population" rule when MULTIPLE dimensions supply summary rows for
// facts that were never also recorded at the Fact level: only the
// first dimension (in dimensionOrder) with a summary row for a given
// (EntityID, Period) contributes it to the business total, so a caller
// who supplied the identical activity as both a CUSTOMER summary and a
// JOB summary (with no underlying Fact) does not have it counted twice.
func buildBusinessTotalsResult(periodLabels []string, facts []Fact, summaries map[summaryKey]EntityPeriodSummaryInput, pools map[string]SharedCostPool) BusinessTotals {
	amountsByPeriod := map[string]map[Component]float64{}
	addAmount := func(period string, c Component, amt float64) {
		if amountsByPeriod[period] == nil {
			amountsByPeriod[period] = map[Component]float64{}
		}
		amountsByPeriod[period][c] += amt
	}

	// Facts contribute their full (unattributed) Amount to the business
	// total exactly once per Fact, regardless of how many dimensions
	// attribute it — a Fact is one economic event.
	for _, f := range facts {
		addAmount(f.Period, f.Component, f.Amount)
	}

	// summaryContributed tracks (EntityID, Period) slots already
	// contributed by an earlier (in dimensionOrder) dimension's summary
	// row, so a later dimension's summary row for the identical
	// EntityID/Period is not double counted into the business total.
	// EntityID alone (not full summaryKey) is the right key here since
	// the whole point is cross-dimension collision detection for what
	// would otherwise look like unrelated summary rows.
	type entityPeriodKey struct{ EntityID, Period string }
	summaryContributed := map[entityPeriodKey]bool{}
	for _, dim := range dimensionOrder {
		for k, s := range summaries {
			if k.Dimension != dim {
				continue
			}
			epk := entityPeriodKey{EntityID: k.EntityID, Period: k.Period}
			if summaryContributed[epk] {
				continue
			}
			summaryContributed[epk] = true
			for c, amt := range componentBridgeFromSummary(s) {
				addAmount(k.Period, c, amt)
			}
		}
	}

	sharedCostByPeriod := map[string]float64{}
	for _, p := range pools {
		sharedCostByPeriod[p.Period] += p.Amount
	}

	var perPeriod []BusinessPeriodTotals
	allAmounts := map[Component]float64{}
	var allSharedCost float64
	for _, period := range periodLabels {
		amounts := amountsByPeriod[period]
		shared := sharedCostByPeriod[period]
		perPeriod = append(perPeriod, buildBusinessPeriodTotals(period, amounts, shared))
		for c, amt := range amounts {
			allAmounts[c] += amt
		}
		allSharedCost += shared
	}

	return BusinessTotals{
		Periods:   perPeriod,
		AllPeriod: buildBusinessPeriodTotals("", allAmounts, allSharedCost),
	}
}

// buildControlReconciliationResults compares each period's computed
// BusinessPeriodTotals against a caller-supplied ControlTotals row, one
// layer at a time — task section 38.
func buildControlReconciliationResults(periodLabels []string, totals BusinessTotals, controls map[string]ControlTotals, tolerance float64) []ControlReconciliation {
	byPeriod := map[string]BusinessPeriodTotals{}
	for _, p := range totals.Periods {
		byPeriod[p.Period] = p
	}
	var out []ControlReconciliation
	for _, period := range periodLabels {
		computed := byPeriod[period]
		var controlPtr *ControlTotals
		if c, ok := controls[period]; ok {
			controlPtr = &c
		}
		out = append(out, buildControlReconciliation(period,
			AvailableValue(computed.RevenueBridge.NetRevenue),
			AvailableValue(computed.DirectCostBridge.Total),
			AvailableValue(computed.VariableCostBridge.Total),
			AvailableValue(computed.SharedCostTotal),
			controlPtr, tolerance))
	}
	return out
}
