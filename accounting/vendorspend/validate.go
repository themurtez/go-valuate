package vendorspend

import "sort"

// validatePeriods checks each Period for structural validity and returns
// issues plus a map of Period label -> Period for the first occurrence of
// each label (later duplicates are flagged but not used).
func validatePeriods(periods []Period) ([]Issue, map[string]Period) {
	var issues []Issue
	byLabel := map[string]Period{}
	seen := map[string]bool{}

	for _, p := range periods {
		if !p.Valid() {
			issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: SeverityError,
				Message: "period failed validation: missing label, missing start/end date, or end before start", Period: p.Period})
			continue
		}
		if seen[p.Period] {
			issues = append(issues, Issue{Code: IssueDuplicatePeriod, Severity: SeverityError,
				Message: "duplicate period label: " + p.Period, Period: p.Period})
			continue
		}
		seen[p.Period] = true
		byLabel[p.Period] = p
	}

	return issues, byLabel
}

// validateSuppliers checks each Supplier for structural validity, detects
// duplicate SupplierIDs and ParentID cycles, and returns issues plus a
// map of SupplierID -> Supplier for the first occurrence of each ID.
func validateSuppliers(suppliers []Supplier) ([]Issue, map[string]Supplier) {
	var issues []Issue
	byID := map[string]Supplier{}
	seen := map[string]bool{}

	for _, s := range suppliers {
		if s.SupplierID == "" {
			continue // structurally unusable; silently excluded like accounting/ap's identical empty-ID handling.
		}
		if seen[s.SupplierID] {
			issues = append(issues, Issue{Code: IssueDuplicateSupplier, Severity: SeverityError,
				Message: "duplicate supplier ID: " + s.SupplierID, SupplierID: s.SupplierID})
			continue
		}
		seen[s.SupplierID] = true
		byID[s.SupplierID] = s
	}

	// ParentID validation: must reference a known supplier (if non-empty)
	// and must not participate in a cycle.
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		s := byID[id]
		if s.ParentID == "" {
			continue
		}
		if _, ok := byID[s.ParentID]; !ok {
			issues = append(issues, Issue{Code: IssueInvalidSupplierParent, Severity: SeverityWarning,
				Message: "supplier parent ID does not reference a known supplier", SupplierID: id})
			continue
		}
		if parentCycle(byID, id) {
			issues = append(issues, Issue{Code: IssueInvalidSupplierParent, Severity: SeverityWarning,
				Message: "supplier parent reference forms a cycle", SupplierID: id})
		}
	}

	return issues, byID
}

// parentCycle walks the ParentID chain starting at id and reports whether
// it revisits a supplier already seen (a cycle), bounded by the total
// supplier count so a broken chain can never loop forever.
func parentCycle(byID map[string]Supplier, id string) bool {
	visited := map[string]bool{id: true}
	cur := byID[id].ParentID
	for i := 0; i < len(byID)+1; i++ {
		if cur == "" {
			return false
		}
		if visited[cur] {
			return true
		}
		visited[cur] = true
		next, ok := byID[cur]
		if !ok {
			return false
		}
		cur = next.ParentID
	}
	return true
}

// validationInputs bundles everything validateSpendRecords needs.
type validationInputs struct {
	suppliers   map[string]Supplier
	periods     map[string]Period
	singleBasis bool
}

// validateSpendRecords checks each SpendRecord for structural validity
// and returns issues plus the set of SpendIDs excluded from every
// downstream computation, plus the deduplicated, in-order slice of
// records actually used.
func validateSpendRecords(records []SpendRecord, vi validationInputs) ([]Issue, []SpendRecord) {
	var issues []Issue
	seen := map[string]bool{}
	var kept []SpendRecord

	for _, r := range records {
		if r.SpendID == "" {
			issues = append(issues, Issue{Code: IssueDuplicateSpend, Severity: SeverityError, Message: "spend record missing ID"})
			continue
		}
		if seen[r.SpendID] {
			issues = append(issues, Issue{Code: IssueDuplicateSpend, Severity: SeverityError,
				Message: "duplicate spend ID: " + r.SpendID, SpendID: r.SpendID})
			continue
		}
		seen[r.SpendID] = true

		excluded := false

		if r.SupplierID == "" || vi.suppliers[r.SupplierID].SupplierID == "" {
			issues = append(issues, Issue{Code: IssueUnknownSupplier, Severity: SeverityError,
				Message: "spend record references unknown supplier ID", SpendID: r.SpendID, SupplierID: r.SupplierID})
			excluded = true
		}

		if r.Period == "" || vi.periods[r.Period].Period == "" {
			issues = append(issues, Issue{Code: IssueUnknownPeriod, Severity: SeverityError,
				Message: "spend record references unknown period", SpendID: r.SpendID, Period: r.Period})
			excluded = true
		}

		if r.Date.IsZero() {
			issues = append(issues, Issue{Code: IssueInvalidSpendDate, Severity: SeverityError,
				Message: "spend record missing date", SpendID: r.SpendID})
			excluded = true
		} else if p, ok := vi.periods[r.Period]; ok {
			if r.Date.Before(p.StartDate) || r.Date.After(p.EndDate) {
				issues = append(issues, Issue{Code: IssueInvalidSpendDate, Severity: SeverityWarning,
					Message: "spend date falls outside its declared period's date range", SpendID: r.SpendID, Period: r.Period})
			}
		}

		if isNonFinite(r.Amount) {
			issues = append(issues, Issue{Code: IssueNonFiniteAmount, Severity: SeverityError,
				Message: "non-finite spend amount", SpendID: r.SpendID})
			excluded = true
		} else if r.Amount < 0 {
			issues = append(issues, Issue{Code: IssueNegativeAmount, Severity: SeverityWarning,
				Message: "spend amount is negative; effect/type determines gross/net semantics, not amount sign", SpendID: r.SpendID})
		}

		if r.Effect != "" && !isRecognizedEffect(r.Effect) {
			issues = append(issues, Issue{Code: IssueInvalidEffect, Severity: SeverityError,
				Message: "unrecognized spend effect", SpendID: r.SpendID})
			excluded = true
		}

		if r.SpendType != "" && !isRecognizedSpendType(r.SpendType) {
			issues = append(issues, Issue{Code: IssueInvalidSpendType, Severity: SeverityWarning,
				Message: "unrecognized spend type; treated as OTHER", SpendID: r.SpendID})
		}

		if r.Basis != "" && !isRecognizedBasis(r.Basis) {
			issues = append(issues, Issue{Code: IssueInvalidSpendBasis, Severity: SeverityWarning,
				Message: "unrecognized spend basis; treated as UNKNOWN", SpendID: r.SpendID})
		}

		if r.Currency == "" {
			issues = append(issues, Issue{Code: IssueMixedCurrency, Severity: SeverityError,
				Message: "spend record missing currency", SpendID: r.SpendID})
			excluded = true
		}

		if r.Quantity.Available && (isNonFinite(r.Quantity.Value) || r.Quantity.Value < 0) {
			issues = append(issues, Issue{Code: IssueInvalidQuantity, Severity: SeverityWarning,
				Message: "quantity is negative or non-finite; excluded from unit-price analytics", SpendID: r.SpendID})
		}
		if r.UnitPrice.Available && (isNonFinite(r.UnitPrice.Value) || r.UnitPrice.Value < 0) {
			issues = append(issues, Issue{Code: IssueInvalidUnitPrice, Severity: SeverityWarning,
				Message: "unit price is negative or non-finite; excluded from unit-price analytics", SpendID: r.SpendID})
		}
		if (r.Quantity.Available || r.UnitPrice.Available) && r.UnitOfMeasure == "" {
			issues = append(issues, Issue{Code: IssueInvalidUOM, Severity: SeverityWarning,
				Message: "quantity/unit price supplied without a unit of measure; excluded from unit-price analytics", SpendID: r.SpendID})
		}

		if !excluded {
			kept = append(kept, r)
		}
	}

	return issues, kept
}

// resolveReportingCurrency determines the single currency an analysis
// proceeds under: explicit caller choice if supplied, otherwise the most
// common currency among included records (ties broken by currency code
// ascending). Mirrors accounting/ap.resolveReportingCurrency's identical
// algorithm.
func resolveReportingCurrency(records []SpendRecord, explicit string) (string, []Issue) {
	if explicit != "" {
		return explicit, checkMixedCurrency(records, explicit)
	}

	counts := map[string]int{}
	for _, r := range records {
		if r.Currency == "" {
			continue
		}
		counts[r.Currency]++
	}
	if len(counts) == 0 {
		return "", nil
	}

	codes := make([]string, 0, len(counts))
	for c := range counts {
		codes = append(codes, c)
	}
	sort.Strings(codes)

	best := codes[0]
	for _, c := range codes[1:] {
		if counts[c] > counts[best] {
			best = c
		}
	}
	return best, checkMixedCurrency(records, best)
}

func checkMixedCurrency(records []SpendRecord, reporting string) []Issue {
	for _, r := range records {
		if r.Currency == "" || r.Currency == reporting {
			continue
		}
		return []Issue{{Code: IssueMixedCurrency, Severity: SeverityWarning,
			Message: "spend records use more than one currency; only " + reporting + " included in aggregate totals"}}
	}
	return nil
}

// checkMixedBasis reports a MIXED_SPEND_BASIS issue if included records
// resolve to more than one SpendBasis. Severity depends on
// Policy.RequireSingleBasis (task section 6: default allows mixed basis,
// relying on Result.SpendByBasis for transparency; RequireSingleBasis
// escalates to an error).
func checkMixedBasis(records []SpendRecord, requireSingle bool) []Issue {
	seen := map[SpendBasis]bool{}
	for _, r := range records {
		seen[r.resolvedEffectBasis()] = true
	}
	if len(seen) <= 1 {
		return nil
	}
	sev := SeverityWarning
	if requireSingle {
		sev = SeverityError
	}
	return []Issue{{Code: IssueMixedSpendBasis, Severity: sev,
		Message: "spend records use more than one spend basis; see Result.SpendByBasis for a per-basis breakdown"}}
}

// resolvedEffectBasis is a small alias so checkMixedBasis reads cleanly;
// defined here rather than in types.go since it is validation-only sugar
// over the already-exported resolvedBasis function.
func (r SpendRecord) resolvedEffectBasis() SpendBasis {
	return resolvedBasis(r.Basis)
}

// validatePolicy checks p (the already-resolved Policy — see
// resolvePolicy) for structurally invalid values: a negative amount or
// percentage anywhere in Policy would silently corrupt downstream
// materiality/threshold math (e.g. a negative
// Materiality.AbsoluteAmount would make resolvedThreshold's "greater of"
// comparison favor the WRONG branch). Each invalid field is reported
// individually via IssueInvalidPolicy; this package never silently
// clamps a negative value to zero or otherwise repairs it.
func validatePolicy(p Policy) []Issue {
	var issues []Issue
	invalid := func(field string) {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError,
			Message: "policy field " + field + " must not be negative"})
	}

	if p.Materiality.AbsoluteAmount < 0 {
		invalid("materiality.absolute_amount")
	}
	if p.Materiality.PercentOfTotalSpend < 0 {
		invalid("materiality.percent_of_total_spend")
	}
	if p.SupplierShareIncreasePoints < 0 {
		invalid("supplier_share_increase_points")
	}
	if p.UnitPriceIncreasePercent < 0 {
		invalid("unit_price_increase_percent")
	}
	if p.NewSupplierMaterialAmount < 0 {
		invalid("new_supplier_material_amount")
	}
	if p.LostSupplierMaterialAmount < 0 {
		invalid("lost_supplier_material_amount")
	}
	if p.TailSpend.BelowAmount < 0 {
		invalid("tail_spend.below_amount")
	}
	if p.TailSpend.OutsideTopN < 0 {
		invalid("tail_spend.outside_top_n")
	}
	if p.DuplicateWindowDays < 0 {
		invalid("duplicate_window_days")
	}
	if p.ObservedRecurringMinPeriods < 0 {
		invalid("observed_recurring_min_periods")
	}
	if p.ObservedRecurringAmountTolerance < 0 {
		invalid("observed_recurring_amount_tolerance")
	}
	if p.CategoryCoverageThreshold < 0 {
		invalid("category_coverage_threshold")
	}
	if p.ProductCoverageThreshold < 0 {
		invalid("product_coverage_threshold")
	}
	if p.ControlTolerance < 0 {
		invalid("control_tolerance")
	}

	if p.TailSpend.BelowAmount > 0 && p.TailSpend.OutsideTopN > 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityWarning,
			Message: "tail_spend policy sets both below_amount and outside_top_n; below_amount takes precedence and outside_top_n is ignored (see TailSpend.Definition to confirm which was applied)"})
	}

	return issues
}
