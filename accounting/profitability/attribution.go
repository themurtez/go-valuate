package profitability

// entityAmounts accumulates one entity's per-component attributed
// amounts for one period, plus provenance (which FactIDs contributed).
type entityAmounts struct {
	amounts map[Component]float64
	factIDs map[string]bool
}

func newEntityAmounts() *entityAmounts {
	return &entityAmounts{amounts: map[Component]float64{}, factIDs: map[string]bool{}}
}

func (e *entityAmounts) add(c Component, amt float64, factID string) {
	e.amounts[c] += amt
	if factID != "" {
		e.factIDs[factID] = true
	}
}

// dimensionAttributionResult holds the fully-resolved attribution outcome
// for one Dimension across all periods: per (EntityID, Period) attributed
// component amounts, plus per-period unattributed amounts by component.
type dimensionAttributionResult struct {
	// byEntityPeriod maps EntityID -> Period -> *entityAmounts.
	byEntityPeriod map[string]map[string]*entityAmounts
	// unattributedByPeriod maps Period -> Component -> amount left
	// unattributed for this dimension.
	unattributedByPeriod map[string]map[Component]float64
	// factCountByPeriod / revenueByPeriod / directCostByPeriod /
	// variableCostByPeriod / totalByPeriod track coverage inputs — task
	// section 14.
	attributedFactIDsByPeriod      map[string]map[string]bool
	totalFactIDsByPeriod           map[string]map[string]bool
	attributedRevenueByPeriod      map[string]float64
	totalRevenueByPeriod           map[string]float64
	attributedDirectCostByPeriod   map[string]float64
	totalDirectCostByPeriod        map[string]float64
	attributedVariableCostByPeriod map[string]float64
	totalVariableCostByPeriod      map[string]float64
	attributedTotalByPeriod        map[string]float64
	totalAmountByPeriod            map[string]float64
}

func newDimensionAttributionResult() *dimensionAttributionResult {
	return &dimensionAttributionResult{
		byEntityPeriod:                 map[string]map[string]*entityAmounts{},
		unattributedByPeriod:           map[string]map[Component]float64{},
		attributedFactIDsByPeriod:      map[string]map[string]bool{},
		totalFactIDsByPeriod:           map[string]map[string]bool{},
		attributedRevenueByPeriod:      map[string]float64{},
		totalRevenueByPeriod:           map[string]float64{},
		attributedDirectCostByPeriod:   map[string]float64{},
		totalDirectCostByPeriod:        map[string]float64{},
		attributedVariableCostByPeriod: map[string]float64{},
		totalVariableCostByPeriod:      map[string]float64{},
		attributedTotalByPeriod:        map[string]float64{},
		totalAmountByPeriod:            map[string]float64{},
	}
}

// attributeFactsForDimension resolves every valid Fact's attribution for
// one Dimension, in one O(N) pass. A Fact with no Attribution entries at
// all for this Dimension leaves 100% of its Amount unattributed.
// tolerance is Policy.AttributionTolerance — the same tolerance
// validateAttributions uses for "sum of shares <= 1," applied here
// symmetrically so a share sum landing just under 1 within tolerance is
// treated as fully (not partially) attributed, rather than leaving a
// tolerance-sized sliver permanently "unattributed" regardless of the
// caller's configured tolerance.
func attributeFactsForDimension(facts []Fact, dim Dimension, tolerance float64) *dimensionAttributionResult {
	r := newDimensionAttributionResult()

	for _, f := range facts {
		if r.totalFactIDsByPeriod[f.Period] == nil {
			r.totalFactIDsByPeriod[f.Period] = map[string]bool{}
		}
		r.totalFactIDsByPeriod[f.Period][f.FactID] = true
		r.totalAmountByPeriod[f.Period] += f.Amount
		if isRevenueComponent(f.Component) {
			r.totalRevenueByPeriod[f.Period] += f.Amount
		}
		if isDirectCostComponent(f.Component) {
			r.totalDirectCostByPeriod[f.Period] += f.Amount
		}
		if isVariableCostComponent(f.Component) {
			r.totalVariableCostByPeriod[f.Period] += f.Amount
		}

		var dimAttrs []Attribution
		for _, a := range f.Attributions {
			if a.Dimension == dim {
				dimAttrs = append(dimAttrs, a)
			}
		}

		if len(dimAttrs) == 0 {
			if r.unattributedByPeriod[f.Period] == nil {
				r.unattributedByPeriod[f.Period] = map[Component]float64{}
			}
			r.unattributedByPeriod[f.Period][f.Component] += f.Amount
			continue
		}

		var attributedSum float64
		anyContribution := false
		for _, a := range dimAttrs {
			// A zero Share genuinely attributes nothing (skip creating an
			// entry for it), but a zero-Amount Fact with a positive Share
			// is still a complete, valid attribution — e.g. a $0
			// promotional line item explicitly attributed 100% to one
			// customer — and must still mark the fact/entity as
			// attributed, not silently drop it as if never attributed at
			// all (a real bug caught during development: it made a fully-
			// attributed $0 fact look completely unattributed in
			// AttributionCoverage).
			if a.Share == 0 {
				continue
			}
			amt := f.Amount * a.Share
			if r.byEntityPeriod[a.EntityID] == nil {
				r.byEntityPeriod[a.EntityID] = map[string]*entityAmounts{}
			}
			if r.byEntityPeriod[a.EntityID][f.Period] == nil {
				r.byEntityPeriod[a.EntityID][f.Period] = newEntityAmounts()
			}
			r.byEntityPeriod[a.EntityID][f.Period].add(f.Component, amt, f.FactID)
			attributedSum += amt
			anyContribution = true

			if r.attributedFactIDsByPeriod[f.Period] == nil {
				r.attributedFactIDsByPeriod[f.Period] = map[string]bool{}
			}
			r.attributedFactIDsByPeriod[f.Period][f.FactID] = true
		}
		if anyContribution {
			r.attributedTotalByPeriod[f.Period] += attributedSum
			if isRevenueComponent(f.Component) {
				r.attributedRevenueByPeriod[f.Period] += attributedSum
			}
			if isDirectCostComponent(f.Component) {
				r.attributedDirectCostByPeriod[f.Period] += attributedSum
			}
			if isVariableCostComponent(f.Component) {
				r.attributedVariableCostByPeriod[f.Period] += attributedSum
			}
		}
		// remainder is a dollar amount, but tolerance (Policy.
		// AttributionTolerance) is a share fraction — scale it by
		// f.Amount so "shares within tolerance of 1" and "remainder
		// within tolerance of 0" agree for the same Fact, instead of
		// comparing a fraction to a raw dollar amount.
		remainder := f.Amount - attributedSum
		remainderTolerance := tolerance * absFloat(f.Amount)
		if remainder > remainderTolerance || remainder < -remainderTolerance {
			if r.unattributedByPeriod[f.Period] == nil {
				r.unattributedByPeriod[f.Period] = map[Component]float64{}
			}
			r.unattributedByPeriod[f.Period][f.Component] += remainder
		}
	}

	return r
}
