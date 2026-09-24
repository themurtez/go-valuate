package profitability

// validatePeriods checks Input.Periods for validity/duplicates and
// returns the deduplicated, chronologically sorted list plus any Issues.
// Only the first occurrence (input order) of a duplicate Period label is
// used.
func validatePeriods(periods []PeriodInfo) ([]Issue, []PeriodInfo) {
	var issues []Issue
	seen := map[string]bool{}
	var out []PeriodInfo
	for _, p := range periods {
		if !p.Valid() {
			issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: SeverityError,
				Message: "period " + p.Period + " has an empty label, missing dates, or end date before start date", Period: p.Period})
			continue
		}
		if seen[p.Period] {
			issues = append(issues, Issue{Code: IssueDuplicatePeriod, Severity: SeverityError,
				Message: "period " + p.Period + " appears more than once; only the first occurrence is used", Period: p.Period})
			continue
		}
		seen[p.Period] = true
		out = append(out, p)
	}
	sortPeriodInfos(out)
	return issues, out
}

// entityKey identifies one Entity's identity slot.
type entityKey struct {
	Dimension Dimension
	EntityID  string
}

// validateEntities checks Input.Entities for validity/duplicates and
// returns the map of valid entities keyed by (Dimension, EntityID) plus
// any Issues. Only the first occurrence of a duplicate key is used.
func validateEntities(entities []Entity) ([]Issue, map[entityKey]Entity) {
	var issues []Issue
	byKey := map[entityKey]Entity{}
	for _, e := range entities {
		if !isRecognizedDimension(e.Dimension) || e.EntityID == "" {
			issues = append(issues, Issue{Code: IssueInvalidAttribution, Severity: SeverityError,
				Message: "entity has an unrecognized dimension or empty entity_id", SourceID: e.EntityID, Dimension: e.Dimension})
			continue
		}
		k := entityKey{Dimension: e.Dimension, EntityID: e.EntityID}
		if _, exists := byKey[k]; exists {
			issues = append(issues, Issue{Code: IssueDuplicateEntity, Severity: SeverityError,
				Message:  "entity " + e.EntityID + " for dimension " + string(e.Dimension) + " appears more than once; only the first occurrence is used",
				SourceID: e.EntityID, Dimension: e.Dimension})
			continue
		}
		byKey[k] = e
	}
	return issues, byKey
}

// validateFacts checks Input.Facts for validity/duplicates and returns
// the valid, deduplicated facts (with each Fact's Attributions validated/
// filtered independently) plus any Issues. A Fact with a structurally
// invalid top-level field (bad component, non-finite/negative amount) is
// excluded entirely; a Fact with an invalid Attribution has just that
// Attribution dropped (the Fact's amount remains, now more unattributed).
func validateFacts(facts []Fact, entities map[entityKey]Entity, entitiesKnown bool, reportingCurrency string, tolerance float64) ([]Issue, []Fact) {
	var issues []Issue
	seenID := map[string]bool{}
	var out []Fact
	for _, f := range facts {
		if f.FactID == "" {
			issues = append(issues, Issue{Code: IssueInvalidComponent, Severity: SeverityError,
				Message: "fact has an empty fact_id"})
			continue
		}
		if seenID[f.FactID] {
			issues = append(issues, Issue{Code: IssueDuplicateFact, Severity: SeverityError,
				Message: "fact " + f.FactID + " appears more than once; only the first occurrence is used", SourceID: f.FactID})
			continue
		}
		if !isRecognizedComponent(f.Component) {
			issues = append(issues, Issue{Code: IssueInvalidComponent, Severity: SeverityError,
				Message: "fact " + f.FactID + " has an unrecognized component", SourceID: f.FactID, Period: f.Period})
			continue
		}
		if isNonFinite(f.Amount) {
			issues = append(issues, Issue{Code: IssueNonFiniteAmount, Severity: SeverityError,
				Message: "fact " + f.FactID + " has a non-finite amount", SourceID: f.FactID, Period: f.Period})
			continue
		}
		if f.Amount < 0 {
			issues = append(issues, Issue{Code: IssueNegativeAmount, Severity: SeverityError,
				Message: "fact " + f.FactID + " has a negative amount; amounts must be non-negative magnitudes (Component determines effect)", SourceID: f.FactID, Period: f.Period})
			continue
		}
		if f.Currency != "" && reportingCurrency != "" && f.Currency != reportingCurrency {
			issues = append(issues, Issue{Code: IssueMixedCurrency, Severity: SeverityWarning,
				Message: "fact " + f.FactID + " currency " + f.Currency + " does not match reporting currency " + reportingCurrency + "; excluded", SourceID: f.FactID, Period: f.Period})
			continue
		}
		seenID[f.FactID] = true

		attrIssues, validAttrs := validateAttributions(f.FactID, f.Period, f.Attributions, entities, entitiesKnown, tolerance)
		issues = append(issues, attrIssues...)

		fc := f
		fc.Attributions = validAttrs
		out = append(out, fc)
	}
	return issues, out
}

// validateAttributions validates one Fact's Attributions, evaluated
// independently per Dimension — task section 11. Returns the subset that
// passed validation (invalid entries dropped, valid entries for a
// dimension retained even if that dimension's sum exceeds 1, since the
// caller needs to see which entries were involved — however an
// ATTRIBUTION_EXCEEDS_100_PERCENT dimension is dropped entirely rather
// than partially honored, since this package never renormalizes and
// silently keeping some entries would understate the true attribution
// intent).
func validateAttributions(factID, period string, attrs []Attribution, entities map[entityKey]Entity, entitiesKnown bool, tolerance float64) ([]Issue, []Attribution) {
	var issues []Issue
	byDimension := map[Dimension][]Attribution{}
	for _, a := range attrs {
		if !isRecognizedDimension(a.Dimension) || a.EntityID == "" {
			issues = append(issues, Issue{Code: IssueInvalidAttribution, Severity: SeverityError,
				Message: "fact " + factID + " has an attribution with an unrecognized dimension or empty entity_id", SourceID: factID, Period: period})
			continue
		}
		if isNonFinite(a.Share) || a.Share < 0 || a.Share > 1 {
			issues = append(issues, Issue{Code: IssueInvalidAttribution, Severity: SeverityError,
				Message: "fact " + factID + " has an attribution share outside [0,1] for dimension " + string(a.Dimension), SourceID: factID, Dimension: a.Dimension, Period: period})
			continue
		}
		if entitiesKnown {
			if _, ok := entities[entityKey{Dimension: a.Dimension, EntityID: a.EntityID}]; !ok {
				issues = append(issues, Issue{Code: IssueUnknownEntity, Severity: SeverityWarning,
					Message: "fact " + factID + " attributes to unknown entity " + a.EntityID + " for dimension " + string(a.Dimension), SourceID: factID, Dimension: a.Dimension, Period: period})
				continue
			}
		}
		byDimension[a.Dimension] = append(byDimension[a.Dimension], a)
	}

	var out []Attribution
	for _, dim := range dimensionOrder {
		group := byDimension[dim]
		if len(group) == 0 {
			continue
		}
		seenEntity := map[string]bool{}
		var dedup []Attribution
		var sum float64
		for _, a := range group {
			if seenEntity[a.EntityID] {
				issues = append(issues, Issue{Code: IssueInvalidAttribution, Severity: SeverityError,
					Message: "fact " + factID + " attributes to entity " + a.EntityID + " more than once for dimension " + string(dim), SourceID: factID, Dimension: dim, Period: period})
				continue
			}
			seenEntity[a.EntityID] = true
			dedup = append(dedup, a)
			sum += a.Share
		}
		if sum > 1+tolerance {
			issues = append(issues, Issue{Code: IssueAttributionExceeds100Percent, Severity: SeverityError,
				Message: "fact " + factID + " attribution shares for dimension " + string(dim) + " sum to more than 1", SourceID: factID, Dimension: dim, Period: period})
			continue
		}
		out = append(out, dedup...)
	}
	return issues, out
}

// validateSummaries checks Input.EntityPeriodSummaries for validity/
// duplicates and returns the valid, deduplicated rows plus any Issues.
func validateSummaries(rows []EntityPeriodSummaryInput, entities map[entityKey]Entity, entitiesKnown bool) ([]Issue, map[summaryKey]EntityPeriodSummaryInput) {
	var issues []Issue
	out := map[summaryKey]EntityPeriodSummaryInput{}
	for _, s := range rows {
		if !isRecognizedDimension(s.Dimension) || s.EntityID == "" || s.Period == "" {
			issues = append(issues, Issue{Code: IssueInvalidSummaryRow, Severity: SeverityError,
				Message: "summary row has an unrecognized dimension or empty entity_id/period", SourceID: s.EntityID, Dimension: s.Dimension, Period: s.Period})
			continue
		}
		if entitiesKnown {
			if _, ok := entities[entityKey{Dimension: s.Dimension, EntityID: s.EntityID}]; !ok {
				issues = append(issues, Issue{Code: IssueUnknownEntity, Severity: SeverityWarning,
					Message: "summary row references unknown entity " + s.EntityID + " for dimension " + string(s.Dimension), SourceID: s.EntityID, Dimension: s.Dimension, Period: s.Period})
				continue
			}
		}
		vals := []Value{s.GrossRevenue, s.Returns, s.Discounts, s.OtherRevenue,
			s.DirectMaterial, s.DirectLabor, s.DirectSubcontractor, s.DirectFulfillment, s.DirectOther,
			s.VariableCommission, s.VariablePaymentFees, s.VariableOther}
		invalid := false
		for _, v := range vals {
			if v.Available && isNonFinite(v.Amount) {
				invalid = true
			}
		}
		if invalid {
			issues = append(issues, Issue{Code: IssueInvalidSummaryRow, Severity: SeverityError,
				Message:  "summary row for entity " + s.EntityID + " dimension " + string(s.Dimension) + " period " + s.Period + " has a non-finite amount",
				SourceID: s.EntityID, Dimension: s.Dimension, Period: s.Period})
			continue
		}
		k := summaryKey{Dimension: s.Dimension, EntityID: s.EntityID, Period: s.Period}
		if _, exists := out[k]; exists {
			issues = append(issues, Issue{Code: IssueInvalidSummaryRow, Severity: SeverityWarning,
				Message:  "summary row for entity " + s.EntityID + " dimension " + string(s.Dimension) + " period " + s.Period + " appears more than once; only the first occurrence is used",
				SourceID: s.EntityID, Dimension: s.Dimension, Period: s.Period})
			continue
		}
		out[k] = s
	}
	return issues, out
}

// detectInputModeConflicts flags any (Dimension, EntityID, Period) slot
// present in BOTH the detailed Fact attribution population and the
// summary-row population — task section 17. The summary row is excluded
// from computation for that slot (the detailed facts win, since they are
// the more granular source); an IssueInputModeConflict is always
// recorded.
func detectInputModeConflicts(factSlots map[summaryKey]bool, summaries map[summaryKey]EntityPeriodSummaryInput) ([]Issue, map[summaryKey]EntityPeriodSummaryInput) {
	var issues []Issue
	out := map[summaryKey]EntityPeriodSummaryInput{}
	for k, s := range summaries {
		if factSlots[k] {
			issues = append(issues, Issue{Code: IssueInputModeConflict, Severity: SeverityError,
				Message:  "entity " + k.EntityID + " dimension " + string(k.Dimension) + " period " + k.Period + " has both detailed facts and a summary row; the summary row is excluded",
				SourceID: k.EntityID, Dimension: k.Dimension, Period: k.Period})
			continue
		}
		out[k] = s
	}
	return issues, out
}

// factSlotsFromAttributions returns the set of (Dimension, EntityID,
// Period) slots any valid Fact attributes to, used to detect
// Fact/summary overlap.
func factSlotsFromAttributions(facts []Fact) map[summaryKey]bool {
	out := map[summaryKey]bool{}
	for _, f := range facts {
		for _, a := range f.Attributions {
			out[summaryKey{Dimension: a.Dimension, EntityID: a.EntityID, Period: f.Period}] = true
		}
	}
	return out
}

// validateDrivers checks Input.Drivers for validity/duplicates and
// returns the valid, deduplicated observations plus any Issues.
func validateDrivers(drivers []DriverObservation, entities map[entityKey]Entity, entitiesKnown bool) ([]Issue, []DriverObservation) {
	var issues []Issue
	seenID := map[string]bool{}
	seenSlot := map[driverObsKey]bool{}
	var out []DriverObservation
	for _, d := range drivers {
		if d.ObservationID == "" || !isRecognizedDimension(d.Dimension) || d.EntityID == "" || d.Period == "" || d.DriverKey == "" {
			issues = append(issues, Issue{Code: IssueInvalidDriver, Severity: SeverityError,
				Message: "driver observation has an empty id/entity_id/period/driver_key or unrecognized dimension", SourceID: d.ObservationID, Dimension: d.Dimension, Period: d.Period})
			continue
		}
		if isNonFinite(d.Value) || d.Value < 0 {
			issues = append(issues, Issue{Code: IssueInvalidDriver, Severity: SeverityError,
				Message: "driver observation " + d.ObservationID + " has a non-finite or negative value", SourceID: d.ObservationID, Dimension: d.Dimension, Period: d.Period})
			continue
		}
		if seenID[d.ObservationID] {
			issues = append(issues, Issue{Code: IssueDuplicateDriver, Severity: SeverityError,
				Message: "driver observation " + d.ObservationID + " appears more than once; only the first occurrence is used", SourceID: d.ObservationID, Period: d.Period})
			continue
		}
		if entitiesKnown {
			if _, ok := entities[entityKey{Dimension: d.Dimension, EntityID: d.EntityID}]; !ok {
				issues = append(issues, Issue{Code: IssueUnknownEntity, Severity: SeverityWarning,
					Message: "driver observation " + d.ObservationID + " references unknown entity " + d.EntityID, SourceID: d.ObservationID, Dimension: d.Dimension, Period: d.Period})
				continue
			}
		}
		k := driverObsKey{Dimension: d.Dimension, EntityID: d.EntityID, Period: d.Period, DriverKey: d.DriverKey}
		if seenSlot[k] {
			issues = append(issues, Issue{Code: IssueDuplicateDriver, Severity: SeverityError,
				Message:  "driver observation for entity " + d.EntityID + " driver_key " + d.DriverKey + " period " + d.Period + " appears more than once; only the first occurrence is used",
				SourceID: d.ObservationID, Dimension: d.Dimension, Period: d.Period})
			continue
		}
		seenID[d.ObservationID] = true
		seenSlot[k] = true
		out = append(out, d)
	}
	return issues, out
}

// validatePools checks Input.SharedCostPools for validity/duplicates.
func validatePools(pools []SharedCostPool) ([]Issue, map[string]SharedCostPool) {
	var issues []Issue
	byID := map[string]SharedCostPool{}
	for _, p := range pools {
		if p.PoolID == "" || p.Period == "" {
			issues = append(issues, Issue{Code: IssueInvalidSharedCostPool, Severity: SeverityError,
				Message: "shared cost pool has an empty pool_id or period", SourceID: p.PoolID, Period: p.Period})
			continue
		}
		if isNonFinite(p.Amount) || p.Amount < 0 {
			issues = append(issues, Issue{Code: IssueInvalidSharedCostPool, Severity: SeverityError,
				Message: "shared cost pool " + p.PoolID + " has a non-finite or negative amount", SourceID: p.PoolID, Period: p.Period})
			continue
		}
		if _, exists := byID[p.PoolID]; exists {
			issues = append(issues, Issue{Code: IssueDuplicatePool, Severity: SeverityError,
				Message: "shared cost pool " + p.PoolID + " appears more than once; only the first occurrence is used", SourceID: p.PoolID, Period: p.Period})
			continue
		}
		byID[p.PoolID] = p
	}
	return issues, byID
}

// validateAllocationRules checks Input.AllocationRules for validity/
// duplicates against the known pool set.
func validateAllocationRules(rules []AllocationRule, pools map[string]SharedCostPool) ([]Issue, map[ruleKey]AllocationRule) {
	var issues []Issue
	byKey := map[ruleKey]AllocationRule{}
	for _, r := range rules {
		if r.PoolID == "" || !isRecognizedDimension(r.Dimension) {
			issues = append(issues, Issue{Code: IssueInvalidAllocationRule, Severity: SeverityError,
				Message: "allocation rule has an empty pool_id or unrecognized dimension", SourceID: r.PoolID, Dimension: r.Dimension})
			continue
		}
		if _, ok := pools[r.PoolID]; !ok {
			issues = append(issues, Issue{Code: IssueInvalidAllocationRule, Severity: SeverityError,
				Message: "allocation rule references unknown pool " + r.PoolID, SourceID: r.PoolID, Dimension: r.Dimension})
			continue
		}
		if !isRecognizedAllocationBasis(r.Basis) {
			issues = append(issues, Issue{Code: IssueInvalidAllocationRule, Severity: SeverityError,
				Message: "allocation rule for pool " + r.PoolID + " has an unrecognized basis", SourceID: r.PoolID, Dimension: r.Dimension})
			continue
		}
		if r.Basis == AllocationBasisDriver && r.DriverKey == "" {
			issues = append(issues, Issue{Code: IssueInvalidAllocationRule, Severity: SeverityError,
				Message: "allocation rule for pool " + r.PoolID + " uses DRIVER basis but has no driver_key", SourceID: r.PoolID, Dimension: r.Dimension})
			continue
		}
		if r.Basis == AllocationBasisFixedWeight {
			if len(r.FixedWeights) == 0 {
				issues = append(issues, Issue{Code: IssueInvalidFixedWeights, Severity: SeverityError,
					Message: "allocation rule for pool " + r.PoolID + " uses FIXED_WEIGHT basis but has no fixed_weights", SourceID: r.PoolID, Dimension: r.Dimension})
				continue
			}
			invalid, sum := false, 0.0
			seen := map[string]bool{}
			for _, w := range r.FixedWeights {
				if w.EntityID == "" || isNonFinite(w.Weight) || w.Weight < 0 {
					invalid = true
					break
				}
				if seen[w.EntityID] {
					invalid = true
					break
				}
				seen[w.EntityID] = true
				sum += w.Weight
			}
			if invalid || sum <= 0 {
				issues = append(issues, Issue{Code: IssueInvalidFixedWeights, Severity: SeverityError,
					Message: "allocation rule for pool " + r.PoolID + " has invalid fixed_weights (negative/non-finite/duplicate entity, or zero sum)", SourceID: r.PoolID, Dimension: r.Dimension})
				continue
			}
		}
		// AllocationBasisEqual optionally restricts membership via
		// FixedWeights (weights themselves ignored, see the task's
		// section 22 AllocationBasisEqual doc comment) — validate the
		// EntityID list the same way (no empty/duplicate EntityID) even
		// though the numeric Weight field is not used, so a duplicate
		// membership entry cannot silently double an entity's equal
		// share.
		if r.Basis == AllocationBasisEqual && len(r.FixedWeights) > 0 {
			invalid := false
			seen := map[string]bool{}
			for _, w := range r.FixedWeights {
				if w.EntityID == "" || seen[w.EntityID] {
					invalid = true
					break
				}
				seen[w.EntityID] = true
			}
			if invalid {
				issues = append(issues, Issue{Code: IssueInvalidFixedWeights, Severity: SeverityError,
					Message: "allocation rule for pool " + r.PoolID + " uses EQUAL basis with an invalid membership list (empty or duplicate entity_id)", SourceID: r.PoolID, Dimension: r.Dimension})
				continue
			}
		}
		k := ruleKey{PoolID: r.PoolID, Dimension: r.Dimension}
		if _, exists := byKey[k]; exists {
			issues = append(issues, Issue{Code: IssueDuplicateAllocationRule, Severity: SeverityError,
				Message:  "allocation rule for pool " + r.PoolID + " dimension " + string(r.Dimension) + " appears more than once; only the first occurrence is used",
				SourceID: r.PoolID, Dimension: r.Dimension})
			continue
		}
		byKey[k] = r
	}
	return issues, byKey
}

// validateControls checks Input.Controls for validity/duplicates.
func validateControls(controls []ControlTotals) ([]Issue, map[string]ControlTotals) {
	var issues []Issue
	byPeriod := map[string]ControlTotals{}
	for _, c := range controls {
		if c.Period == "" {
			issues = append(issues, Issue{Code: IssueInvalidControlTotal, Severity: SeverityError,
				Message: "control totals row has an empty period"})
			continue
		}
		vals := []Value{c.NetRevenue, c.DirectCost, c.VariableCost, c.SharedCost}
		invalid := false
		for _, v := range vals {
			if v.Available && isNonFinite(v.Amount) {
				invalid = true
			}
		}
		if invalid {
			issues = append(issues, Issue{Code: IssueInvalidControlTotal, Severity: SeverityError,
				Message: "control totals for period " + c.Period + " has a non-finite amount", Period: c.Period})
			continue
		}
		if _, exists := byPeriod[c.Period]; exists {
			issues = append(issues, Issue{Code: IssueInvalidControlTotal, Severity: SeverityWarning,
				Message: "control totals for period " + c.Period + " appears more than once; only the first occurrence is used", Period: c.Period})
			continue
		}
		byPeriod[c.Period] = c
	}
	return issues, byPeriod
}

// resolveReportingCurrency picks the single reporting currency: the
// caller's explicit Policy.Currency if set, otherwise the first non-empty
// Fact.Currency encountered (input order). Returns an empty string (no
// currency filtering applied) if neither is available.
func resolveReportingCurrency(explicit string, facts []Fact) string {
	if explicit != "" {
		return explicit
	}
	for _, f := range facts {
		if f.Currency != "" {
			return f.Currency
		}
	}
	return ""
}
