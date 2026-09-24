package vendorspend

import "sort"

// Calculate derives a full Result from in under opts. It never mutates
// in.Periods/in.Suppliers/in.SpendRecords/in.Controls/in.Policy (see
// immutability_test.go) and performs no I/O.
func Calculate(in Input, opts Options) Result {
	policy := resolvePolicy(in.Policy)
	thresholds := resolveThresholds(opts.Thresholds)

	result := Result{SchemaVersion: SchemaVersion, FormulaVersion: FormulaVersion, Policy: policy, Thresholds: thresholds}
	// Reported (SeverityError, since a negative amount/percentage would
	// corrupt downstream math) but never fatal by itself — a caller with
	// one invalid Policy field alongside otherwise-good data still gets a
	// full analysis of everything else; HasErrors(result.Issues) lets a
	// caller detect and act on it.
	result.Issues = append(result.Issues, validatePolicy(policy)...)

	if len(in.Periods) == 0 || len(in.SpendRecords) == 0 {
		result.Issues = append(result.Issues, Issue{Code: IssueInvalidPeriod, Severity: SeverityError,
			Message: "at least one period and one spend record are required"})
		return result
	}

	periodIssues, periodsByLabel := validatePeriods(in.Periods)
	result.Issues = append(result.Issues, periodIssues...)
	if len(periodsByLabel) == 0 {
		result.Issues = append(result.Issues, Issue{Code: IssueInvalidPeriod, Severity: SeverityError,
			Message: "no period passed validation"})
		return result
	}

	supplierIssues, suppliersByID := validateSuppliers(in.Suppliers)
	result.Issues = append(result.Issues, supplierIssues...)

	spendIssues, records := validateSpendRecords(in.SpendRecords, validationInputs{suppliers: suppliersByID, periods: periodsByLabel})
	result.Issues = append(result.Issues, spendIssues...)
	if len(records) == 0 {
		result.Issues = append(result.Issues, Issue{Code: IssueInvalidSpendDate, Severity: SeverityError,
			Message: "no spend record passed validation"})
		return result
	}

	reportingCurrency, currencyIssues := resolveReportingCurrency(records, policy.ReportingCurrency)
	result.Issues = append(result.Issues, currencyIssues...)
	result.ReportingCurrency = reportingCurrency
	if reportingCurrency != "" {
		filtered := make([]SpendRecord, 0, len(records))
		for _, r := range records {
			if r.Currency == reportingCurrency {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}
	if len(records) == 0 {
		result.Issues = append(result.Issues, Issue{Code: IssueMixedCurrency, Severity: SeverityError,
			Message: "no spend record remained after resolving a single reporting currency"})
		return result
	}

	basisIssues := checkMixedBasis(records, policy.RequireSingleBasis)
	result.Issues = append(result.Issues, basisIssues...)
	// Checked against basisIssues alone, not the cumulative result.Issues:
	// an unrelated earlier SeverityError (e.g. IssueDuplicateSpend) must
	// never abort an otherwise-single-basis analysis just because
	// RequireSingleBasis happens to be set.
	if policy.RequireSingleBasis && HasErrors(basisIssues) {
		return result
	}

	result.Available = true

	orderedPeriods := chronologicalPeriods(periodsByLabel)
	// Restrict to periods that actually have at least one included
	// record OR were explicitly supplied — Result.Periods echoes every
	// validated period, matching Input.Periods' caller intent, not only
	// periods with data (a caller may supply a full fiscal calendar and
	// expect empty periods to appear with zero spend).
	result.Periods = orderedPeriods

	byPeriod := groupBySpendPeriod(records)

	result.Bridge = computeBridge(records)

	basisTotals := map[SpendBasis]float64{}
	for _, r := range records {
		basisTotals[resolvedBasis(r.Basis)] += netSpendOf(r)
	}
	bases := make([]SpendBasis, 0, len(basisTotals))
	for b := range basisTotals {
		bases = append(bases, b)
	}
	sort.Slice(bases, func(i, j int) bool { return bases[i] < bases[j] })
	for _, b := range bases {
		result.SpendByBasis = append(result.SpendByBasis, BasisSpend{Basis: b, Spend: basisTotals[b]})
	}

	for _, p := range orderedPeriods {
		rows := byPeriod[p]
		if len(rows) == 0 {
			continue
		}
		result.PeriodSummaries = append(result.PeriodSummaries, computePeriodSummary(p, rows))
	}

	for _, p := range orderedPeriods {
		rows := byPeriod[p]
		if len(rows) == 0 {
			continue
		}
		periodBridge := computeBridge(rows)
		result.SupplierSummaries = append(result.SupplierSummaries, computeSupplierPeriodSummaries(p, rows, suppliersByID, periodBridge.NetSpend)...)
	}

	result.CategorySummaries = computeCategorySummaries(records, result.Bridge.NetSpend)
	result.ProductSummaries = computeProductSpend(records)
	result.DepartmentSummaries = groupBy(records, result.Bridge.NetSpend, func(r SpendRecord) string { return r.Department })
	result.LocationSummaries = groupBy(records, result.Bridge.NetSpend, func(r SpendRecord) string { return r.Location })

	periodMeta := buildConcentrationPeriodMeta(periodsByLabel)
	// Concentration's adapter reuses the SupplierSummaries already
	// computed above rather than recomputing them, and analytics/
	// concentration.Calculate itself runs exactly ONCE here — both
	// result.Concentration (most recent period) and concentrationHistory
	// (every period, used only by the increasing-concentration flag
	// check) are derived from this single concentrationResult rather
	// than each re-running the full concentration pipeline.
	concentrationResult := runConcentration(result.SupplierSummaries, periodMeta, policy.TopN)
	result.Concentration = spendConcentrationFromResult(concentrationResult)
	concentrationHistory := concentrationHistoryFromResult(concentrationResult)

	result.DependencySummary = computeDependencySummary(records, suppliersByID)
	result.NewLostSuppliers = computeNewLostSuppliers(orderedPeriods, byPeriod, policy.NewSupplierMaterialAmount, policy.LostSupplierMaterialAmount)
	result.Trends = computeSpendTrends(orderedPeriods, byPeriod)

	result.UnitPricePoints = computeUnitPricePoints(records)
	result.ProductPriceComparisons = computeProductPriceComparisons(result.UnitPricePoints)
	result.PriceVolume = computePriceVolumeDecompositions(orderedPeriods, byPeriod)

	result.RecurringSpend = RecurringSpend{
		Mix:               computeRecurrenceMix(records, result.Bridge.NetSpend),
		ObservedRecurring: computeObservedRecurring(orderedPeriods, byPeriod, policy.ObservedRecurringMinPeriods, policy.ObservedRecurringAmountTolerance),
	}
	result.CommitmentMix = computeCommitmentMix(records, result.Bridge.NetSpend)

	supplierNetSpend := map[string]float64{}
	for _, r := range records {
		supplierNetSpend[r.SupplierID] += netSpendOf(r)
	}
	result.TailSpend = computeTailSpend(supplierNetSpend, result.Bridge.NetSpend, policy.TailSpend)
	result.DuplicateLikeGroups = computeDuplicateLikeGroups(records, policy.DuplicateWindowDays)

	result.NonPreferredSpend, result.OutsideContractedSpend = computeSupplierPolicySpend(supplierNetSpend, suppliersByID, result.Bridge.NetSpend)

	// ControlTolerance's effective value is the greater of the caller's
	// explicit Policy.ControlTolerance and the resolved Materiality
	// threshold — a control mismatch smaller than what this analysis
	// already considers immaterial should not by itself flag
	// FlagControlTotalMismatch. This is the only place Materiality feeds
	// a computation directly; every other flag threshold in this package
	// has its own dedicated Thresholds/Policy field instead (see
	// Thresholds' doc comment).
	controlTolerance := policy.ControlTolerance
	if materialityThreshold := policy.Materiality.resolvedThreshold(result.Bridge.NetSpend); materialityThreshold > controlTolerance {
		controlTolerance = materialityThreshold
	}
	var controlIssues []Issue
	result.ControlReconciliation, controlIssues = computeControlReconciliation(result.Bridge.NetSpend, in.Controls, controlTolerance)
	result.Issues = append(result.Issues, controlIssues...)

	result.CategoryProductCoverage = computeCategoryProductCoverage(records)
	result.Coverage = computeMetadataCoverage(records, in.Controls)

	result.Flags = computeFlags(flagInputs{
		concentration:               result.Concentration,
		concentrationHistory:        concentrationHistory,
		supplierShareIncreasePoints: policy.SupplierShareIncreasePoints,
		newLost:                     result.NewLostSuppliers,
		newMaterial:                 policy.NewSupplierMaterialAmount,
		lostMaterial:                policy.LostSupplierMaterialAmount,
		unitPricePoints:             result.UnitPricePoints,
		unitPriceIncreasePercent:    policy.UnitPriceIncreasePercent,
		productSpend:                result.ProductSummaries,
		observedRecurring:           result.RecurringSpend.ObservedRecurring,
		duplicateGroups:             result.DuplicateLikeGroups,
		categoryProductCoverage:     result.CategoryProductCoverage,
		categoryCoverageThreshold:   policy.CategoryCoverageThreshold,
		productCoverageThreshold:    policy.ProductCoverageThreshold,
		nonPreferred:                result.NonPreferredSpend,
		outsideContracted:           result.OutsideContractedSpend,
		tailSpend:                   result.TailSpend,
		controlReconciliation:       result.ControlReconciliation,
		thresholds:                  thresholds,
	})

	sortIssues(result.Issues)

	return result
}

// sortIssues sorts issues by IssueCode declaration order, then source ID
// — task section 41's "issues code/source ID" rule.
func sortIssues(issues []Issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		ri, rj := issueRank(issues[i].Code), issueRank(issues[j].Code)
		if ri != rj {
			return ri < rj
		}
		return sourceIDOf(issues[i]) < sourceIDOf(issues[j])
	})
}
