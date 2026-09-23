package inventory

import (
	"time"

	"github.com/themurtez/go-valuate/analytics/concentration"
)

// Calculate derives a full Result from in under policy. It never mutates
// any slice/map within in or policy, and performs no I/O — see
// immutability_test.go. Calculate is pure and deterministic — see
// determinism_test.go.
func Calculate(in Input, policy Policy) Result {
	policy = resolvePolicy(policy)

	var issues []Issue
	issues = append(issues, validatePolicy(policy)...)

	buckets := resolvedBuckets(policy.Buckets)
	issues = append(issues, validateBucketDefinitions(buckets)...)

	itemIssues, itemsByID, itemOrder := validateItems(in.Items)
	issues = append(issues, itemIssues...)

	reportingCurrency, currencyIssues := resolveReportingCurrency(in.Items, policy.ReportingCurrency)
	issues = append(issues, currencyIssues...)
	// Items in a different currency than the resolved reportingCurrency
	// are excluded from every downstream aggregate (task section 61:
	// "do not silently sum currencies") rather than merely flagged — an
	// item with no stated Currency at all is included, since this
	// package cannot know it conflicts.
	itemsByID, itemOrder = filterItemsByCurrency(itemsByID, itemOrder, reportingCurrency)

	snapIssues, validSnapshots := validateSnapshots(in.Snapshots, itemsByID, policy.NegativeInventoryHandling)
	issues = append(issues, snapIssues...)

	moveIssues, validMovements := validateMovements(in.Movements, itemsByID)
	issues = append(issues, moveIssues...)

	policyIssues := validateStockPolicies(in.StockPolicies, itemsByID)
	issues = append(issues, policyIssues...)

	periodIssues, periods := validatePeriods(in.Periods)
	issues = append(issues, periodIssues...)

	finIssues := validateFinancials(in.Financials)
	issues = append(issues, finIssues...)

	glIssues := validateGLControls(in.GLControls)
	issues = append(issues, glIssues...)

	mvIssues := validateMarketValues(in.MarketValues)
	issues = append(issues, mvIssues...)

	result := Result{SchemaVersion: SchemaVersion, FormulaVersion: FormulaVersion}

	asOf, asOfErr := parseAsOfDate(in.AsOfDate)
	if asOfErr {
		issues = append(issues, Issue{Code: IssueInvalidAsOfDate, Severity: SeverityError, Message: "AsOfDate is required and must be a valid date"})
	}

	if asOfErr && len(periods) == 0 {
		result.Issues = issues
		return result
	}

	// If items were supplied but every one of them was structurally
	// invalid (e.g. all missing IDs), there is nothing to build item
	// states from; downstream sections simply report Unavailable rather
	// than erroring further.
	uomTable := buildUOMConversionTable(policy.UOMConversions)

	var states map[string]*itemState
	if !asOfErr {
		states = buildItemStates(itemsByID, itemOrder, validSnapshots, validMovements, asOf, uomTable)
		result.AsOfDate = asOf.Format("2006-01-02")
	}

	stockPoliciesByItem := map[string]StockPolicy{}
	for _, p := range in.StockPolicies {
		if p.ItemID != "" {
			if _, exists := stockPoliciesByItem[p.ItemID]; !exists {
				stockPoliciesByItem[p.ItemID] = p
			}
		}
	}

	marketValuesByItem := map[string]MarketValue{}
	for _, mv := range in.MarketValues {
		if mv.ItemID != "" {
			if _, exists := marketValuesByItem[mv.ItemID]; !exists {
				marketValuesByItem[mv.ItemID] = mv
			}
		}
	}

	if states != nil {
		result.Portfolio = buildPortfolioSummary(itemOrder, states, policy.TopN)
		result.Aging = buildAgingSummary(itemOrder, states, buckets, asOf, policy)
		result.Composition = buildCompositionSummary(itemOrder, states)
		result.Concentration = buildConcentrationSummary(itemOrder, states, resolvedConcentrationPolicy(policy.TopN))
		result.ExpirySummary = buildExpirySummary(itemOrder, states, asOf, policy.ExpiryWarningDays)

		for _, id := range itemOrder {
			result.LastMovement = append(result.LastMovement, buildItemLastMovement(id, states[id], asOf))
		}

		for _, id := range itemOrder {
			policyForItem, ok := stockPoliciesByItem[id]
			if !ok {
				continue
			}
			result.StockPolicyResults = append(result.StockPolicyResults, buildStockPolicyResult(id, states[id].quantity, policyForItem))
		}

		if len(in.VelocityWindows) > 0 {
			windowedItems := make([]string, 0, len(in.VelocityWindows))
			for id := range in.VelocityWindows {
				windowedItems = append(windowedItems, id)
			}
			sortStrings(windowedItems)
			for _, id := range windowedItems {
				window := in.VelocityWindows[id]
				v := buildItemVelocity(id, window, validMovements)
				result.Velocity = append(result.Velocity, v)
				if st, ok := states[id]; ok {
					result.SupplyDuration = append(result.SupplyDuration, buildSupplyDuration(id, st.quantity, v))
				}
			}
		}

		for _, id := range itemOrder {
			mv, ok := marketValuesByItem[id]
			if !ok {
				continue
			}
			result.MarketValueComparisons = append(result.MarketValueComparisons, buildMarketValueComparison(id, states[id].value, mv))
		}

		result.PossibleDuplicateMovements = findPossibleDuplicateMovements(validMovements)

		subledgerByComponent := buildSubledgerByComponent(result.Portfolio.TotalInventoryValue, result.Composition)
		result.Reconciliation = buildReconciliationSummary(subledgerByComponent, in.GLControls, policy.ReconciliationTolerance)
	}

	financialsByPeriod := map[string]PeriodFinancials{}
	for _, f := range in.Financials {
		if _, exists := financialsByPeriod[f.Period]; !exists {
			financialsByPeriod[f.Period] = f
		}
	}

	materialityAbs := resolveMaterialityAbsolute(policy.Materiality, policy.MaterialityPercent, result.Portfolio.TotalInventoryValue)

	basisByPeriod := map[string]periodInventoryBasis{}
	adjustmentCountsByItem := countAdjustmentsByItem(validMovements)

	for _, p := range periods {
		fin, hasFin := financialsByPeriod[p.Period]
		var finPtr *PeriodFinancials
		if hasFin {
			finPtr = &fin
		}

		periodMovements := movementsInRange(validMovements, p.StartDate, p.EndDate)
		hasMovementData := len(periodMovements) > 0

		beginQty, beginVal, endQty, endVal := resolvePeriodBeginEndQuantityValue(validSnapshots, itemsByID, p, uomTable)

		basis := resolvePeriodInventoryBasis(beginVal, endVal, fin, hasFin, resolvedPeriodDays(p))
		basisByPeriod[p.Period] = basis

		purchases := resolvedPurchaseSummary(buildPeriodMovementTotals(periodMovements, p.StartDate, p.EndDate), hasMovementData, finPtr)
		usage := resolvedUsageSummary(buildPeriodMovementTotals(periodMovements, p.StartDate, p.EndDate), hasMovementData, finPtr)
		purchaseVsUsage := buildPurchaseVsUsageTrend(p.Period, purchases, usage)

		adjustments := buildAdjustmentSummary(periodMovements, p.StartDate, p.EndDate, basis.average, materialityAbs, policy.MaterialityPercent, policy.PeriodEndAdjustmentWindowDays)

		qtyRoll := buildQuantityRollforward(beginQty, endQty, periodMovements, policy.RollforwardTolerance.AbsoluteTolerance)
		valRoll := buildValueRollforward(beginVal, endVal, periodMovements, policy.RollforwardTolerance.AbsoluteTolerance)

		result.Periods = append(result.Periods, PeriodSummary{
			Period:              p,
			Turnover:            calculateTurnover(p.Period, basis),
			DIO:                 calculateDIO(p.Period, basis),
			Purchases:           purchases,
			Usage:               usage,
			PurchaseVsUsage:     purchaseVsUsage,
			Adjustments:         adjustments,
			QuantityRollforward: qtyRoll,
			ValueRollforward:    valRoll,
		})
	}

	result.TurnoverHistory = buildTurnoverHistory(periods, basisByPeriod)

	result.Coverage = buildCoverage(itemOrder, states, validMovements, in.Financials, in.GLControls, stockPoliciesByItem, result.Aging.Rows)

	result.Flags = computeFlags(flagInputs{
		states:                 states,
		itemOrder:              itemOrder,
		aging:                  result.Aging,
		stockPolicyResults:     result.StockPolicyResults,
		periods:                result.Periods,
		reconciliation:         result.Reconciliation,
		expiry:                 result.ExpirySummary,
		possibleDuplicates:     result.PossibleDuplicateMovements,
		adjustmentCountsByItem: adjustmentCountsByItem,
		policy:                 policy,
	})

	// Per task section 61, this package performs no per-currency
	// conversion (no FX fetch, no silent cross-currency summation) —
	// items outside reportingCurrency were already excluded above
	// (filterItemsByCurrency), and IssueMixedCurrency records the
	// mismatch for auditability.

	result.Issues = issues
	result.Available = true
	return result
}

// parseAsOfDate parses s as "2006-01-02" (or any RFC3339-compatible
// date); returns (zero, true) on empty/invalid input.
func parseAsOfDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, true
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, true
		}
	}
	return t, false
}

// movementsInRange returns the subset of movements with Date in
// [start, end] inclusive.
func movementsInRange(movements []Movement, start, end time.Time) []Movement {
	var out []Movement
	for _, m := range movements {
		if !m.Date.Before(start) && !m.Date.After(end) {
			out = append(out, m)
		}
	}
	return out
}

// resolvePeriodBeginEndQuantityValue finds the latest InventorySnapshot
// per item at or before p.StartDate.AddDate(0,0,-1) (beginning) and at or
// before p.EndDate (ending), aggregated across items exactly like
// buildItemStates does for the analysis AsOfDate — reused here at
// per-period boundaries for rollforward purposes.
func resolvePeriodBeginEndQuantityValue(snapshots []InventorySnapshot, itemsByID map[string]Item, p PeriodInfo, uomTable uomConversionTable) (beginQty Qty, beginVal Value, endQty Qty, endVal Value) {
	order := sortedItemIDs(itemsByID)
	beginningAsOf := p.StartDate.AddDate(0, 0, -1)

	beginStates := buildItemStates(itemsByID, order, snapshots, nil, beginningAsOf, uomTable)
	endStates := buildItemStates(itemsByID, order, snapshots, nil, p.EndDate, uomTable)

	beginQty, beginVal = aggregatePortfolioQuantityValue(order, beginStates)
	endQty, endVal = aggregatePortfolioQuantityValue(order, endStates)
	return
}

func sortedItemIDs(itemsByID map[string]Item) []string {
	ids := make([]string, 0, len(itemsByID))
	for id := range itemsByID {
		ids = append(ids, id)
	}
	sortStrings(ids)
	return ids
}

// aggregatePortfolioQuantityValue sums every item's on-hand quantity/value
// into one portfolio-wide figure. Quantity is available only if every
// contributing item's Qty shares one UOM (an even stricter UOM-safety bar
// than aggregateItemQuantityValue's per-item aggregation, since summing
// ACROSS items multiplies the chance of an incompatible unit) — used only
// for rollforward purposes, where a single beginning/ending "quantity on
// hand" figure is inherently a whole-portfolio number.
func aggregatePortfolioQuantityValue(order []string, states map[string]*itemState) (Qty, Value) {
	var qty qtyAccumulator
	var val valueAccumulator
	for _, id := range order {
		st, ok := states[id]
		if !ok {
			continue
		}
		qty.add(st.quantity)
		val.add(st.value)
	}
	return qty.result(), val.result()
}

// resolvePeriodInventoryBasis assembles one period's turnover/DIO basis.
// The detailed path's beginVal/endVal (resolved from InventorySnapshot
// data at the period boundaries) is the default; PeriodFinancials'
// BeginningInventoryValue/EndingInventoryValue/COGS override it when
// explicitly supplied — the summary path's own direct facts take
// precedence over a snapshot-derived reconstruction, since a caller who
// supplies period-summary financials is the authority on that period's
// own reported figures.
func resolvePeriodInventoryBasis(beginVal, endVal Value, fin PeriodFinancials, hasFin bool, days int) periodInventoryBasis {
	beginning, ending := beginVal, endVal
	var cogs Value
	if hasFin {
		if fin.BeginningInventoryValue.Available {
			beginning = fin.BeginningInventoryValue
		}
		if fin.EndingInventoryValue.Available {
			ending = fin.EndingInventoryValue
		}
		cogs = fin.COGS
	}
	average, basis := resolveAverageInventory(beginning, ending, Unavailable())
	return periodInventoryBasis{beginning: beginning, ending: ending, average: average, basis: basis, cogs: cogs, days: days}
}

// buildSubledgerByComponent maps a GLControl.Component label to its
// matching subledger inventory value — task section 38: "use
// caller-defined component/category mapping," never a hard-coded
// manufacturing requirement. The "" key is always the single combined
// total. A Component string that matches a known InventoryClass value
// (e.g. "RAW_MATERIAL") resolves against CompositionSummary.ByClass's
// matching total; any other Component label has no subledger equivalent
// this package can derive on its own (there is no caller-defined mapping
// input beyond Item.Class in V1), so it is deliberately left unmapped —
// buildReconciliationSummary then reports that component's subledger
// value as 0, which reconciles only if the caller's own GL balance is
// also 0, correctly surfacing rather than masking the missing mapping.
func buildSubledgerByComponent(total float64, composition CompositionSummary) map[string]float64 {
	m := map[string]float64{"": total}
	for _, cs := range composition.ByClass {
		m[string(cs.Class)] = cs.Value
	}
	return m
}

// resolvedConcentrationPolicy adapts Policy.TopN into an
// analytics/concentration.Policy — mirrors ap's identical adapter,
// reusing DefaultPolicy for every field this package's Policy does not
// separately expose. analytics/concentration.Policy.TopN is a list of
// cutoffs (e.g. top-1/3/5/10), not a single N, so this package's single
// Policy.TopN is used as that list's sole cutoff when set.
func resolvedConcentrationPolicy(topN int) concentration.Policy {
	p := concentration.DefaultPolicy()
	if topN > 0 {
		p.TopN = []int{topN}
	}
	return p
}
