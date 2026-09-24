package profitability

import "sort"

// Calculate derives a full Result from in under policy. It never mutates
// any slice/map within in or policy, and performs no I/O — see
// immutability_test.go and determinism_test.go.
func Calculate(in Input, policy Policy) Result {
	policy = resolvePolicy(policy)

	var issues []Issue
	issues = append(issues, validatePolicy(policy)...)

	periodIssues, periods := validatePeriods(in.Periods)
	issues = append(issues, periodIssues...)

	entityIssues, entitiesByKey := validateEntities(in.Entities)
	issues = append(issues, entityIssues...)
	entitiesKnown := len(in.Entities) > 0

	reportingCurrency := resolveReportingCurrency(policy.Currency, in.Facts)

	factIssues, validFacts := validateFacts(in.Facts, entitiesByKey, entitiesKnown, reportingCurrency, policy.AttributionTolerance)
	issues = append(issues, factIssues...)

	summaryIssues, validSummaries := validateSummaries(in.EntityPeriodSummaries, entitiesByKey, entitiesKnown)
	issues = append(issues, summaryIssues...)

	factSlots := factSlotsFromAttributions(validFacts)
	conflictIssues, validSummaries2 := detectInputModeConflicts(factSlots, validSummaries)
	issues = append(issues, conflictIssues...)

	driverIssues, validDrivers := validateDrivers(in.Drivers, entitiesByKey, entitiesKnown)
	issues = append(issues, driverIssues...)

	poolIssues, validPools := validatePools(in.SharedCostPools)
	issues = append(issues, poolIssues...)

	ruleIssues, validRules := validateAllocationRules(in.AllocationRules, validPools)
	issues = append(issues, ruleIssues...)

	controlIssues, validControls := validateControls(in.Controls)
	issues = append(issues, controlIssues...)

	periodLabels := make([]string, 0, len(periods))
	for _, p := range periods {
		periodLabels = append(periodLabels, p.Period)
	}

	driversByEntityPeriodKey := indexDrivers(validDrivers)

	result := Result{SchemaVersion: SchemaVersion, FormulaVersion: FormulaVersion}

	// Business totals are computed directly from facts/summaries/pools —
	// never by summing dimension views (task section 35).
	result.BusinessTotals = buildBusinessTotalsResult(periodLabels, validFacts, validSummaries2, validPools)

	views := map[Dimension]DimensionView{}
	var allFlags []Flag
	var allCoverage []AttributionCoverage
	var allDriverCoverage []DriverCoverage
	var allAllocCoverage []AllocationCoverage
	var dimReconciliation []DimensionReconciliation

	for _, dim := range dimensionOrder {
		applicability := applicabilityFor(policy, dim)
		if applicability == ApplicabilityNotApplicable {
			views[dim] = DimensionView{Dimension: dim, Status: ViewStatusNotApplicable}
			continue
		}

		view, flags, viewIssues, cov, driverCov, allocCov, dimRecon := buildDimensionView(dim, periods, periodLabels, validFacts, validSummaries2, entitiesByKey, validDrivers, driversByEntityPeriodKey, validPools, validRules, policy)
		views[dim] = view
		allFlags = append(allFlags, flags...)
		issues = append(issues, viewIssues...)
		allCoverage = append(allCoverage, cov...)
		allDriverCoverage = append(allDriverCoverage, driverCov...)
		allAllocCoverage = append(allAllocCoverage, allocCov...)
		dimReconciliation = append(dimReconciliation, dimRecon...)
	}

	result.CustomerView = views[DimensionCustomer]
	result.JobView = views[DimensionJob]
	result.ProductView = views[DimensionProduct]

	result.SharedCostPools = sortedPools(validPools)
	result.DimensionReconciliation = dimReconciliation

	result.ControlReconciliation = buildControlReconciliationResults(periodLabels, result.BusinessTotals, validControls, policy.ControlTolerance)
	allFlags = append(allFlags, computeControlFlags(result.ControlReconciliation, policy)...)

	result.Coverage = Coverage{
		PeriodsSupplied:         len(in.Periods) > 0,
		EntitiesSupplied:        len(in.Entities) > 0,
		FactsSupplied:           len(in.Facts) > 0,
		SummariesSupplied:       len(in.EntityPeriodSummaries) > 0,
		DriversSupplied:         len(in.Drivers) > 0,
		SharedCostPoolsSupplied: len(in.SharedCostPools) > 0,
		AllocationRulesSupplied: len(in.AllocationRules) > 0,
		ControlsSupplied:        len(in.Controls) > 0,

		AttributionByDimension:      allCoverage,
		DriverCoverage:              allDriverCoverage,
		AllocationCoverage:          allAllocCoverage,
		ControlTotalCoveragePeriods: len(validControls),
	}

	sortFlags(allFlags)
	result.Flags = allFlags
	result.Issues = dedupeAndSortIssues(issues)

	return result
}

// indexDrivers groups DriverObservations by (Dimension, EntityID,
// Period) -> DriverKey -> summed value (summed rather than
// first-occurrence since validateDrivers already deduplicated exact
// (dimension,entity,period,driver_key) slots — this index exists purely
// for O(1) lookup, not further aggregation).
func indexDrivers(drivers []DriverObservation) map[driverObsKey]float64 {
	out := map[driverObsKey]float64{}
	for _, d := range drivers {
		out[driverObsKey{Dimension: d.Dimension, EntityID: d.EntityID, Period: d.Period, DriverKey: d.DriverKey}] = d.Value
	}
	return out
}

func sortedPools(pools map[string]SharedCostPool) []SharedCostPool {
	ids := make([]string, 0, len(pools))
	for id := range pools {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]SharedCostPool, 0, len(ids))
	for _, id := range ids {
		out = append(out, pools[id])
	}
	return out
}
