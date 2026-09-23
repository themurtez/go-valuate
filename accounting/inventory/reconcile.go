package inventory

import "sort"

// ComponentReconciliation is one GL-inventory-account component's
// subledger-vs-control-balance comparison — task sections 37-38. Never
// adjusts either side. Component is "" for a single combined control
// account.
type ComponentReconciliation struct {
	Component string `json:"component,omitempty"`

	SubledgerInventoryValue float64 `json:"subledger_inventory_value"`
	GLInventoryBalance      float64 `json:"gl_inventory_balance"`
	Difference              float64 `json:"difference"`
	Tolerance               float64 `json:"tolerance"`
	Reconciled              bool    `json:"reconciled"`
}

// ReconciliationSummary is the full GL-reconciliation result across every
// supplied component (or the single combined total when the caller
// supplies no Component labels) — task sections 37-39.
type ReconciliationSummary struct {
	Available  bool                      `json:"available"`
	Components []ComponentReconciliation `json:"components,omitempty"`
	// AllReconciled is true only if every component is Reconciled.
	AllReconciled bool `json:"all_reconciled"`
}

// buildReconciliationSummary compares subledgerByComponent (component
// label -> inventory value; "" key for "no component supplied") against
// every caller-supplied GLControl for the matching AsOfDate/period scope.
// Available only if at least one GLControl was supplied.
func buildReconciliationSummary(subledgerByComponent map[string]float64, controls []GLControl, tolerance ReconciliationTolerance) ReconciliationSummary {
	if len(controls) == 0 {
		return ReconciliationSummary{}
	}

	sortedControls := make([]GLControl, len(controls))
	copy(sortedControls, controls)
	sort.SliceStable(sortedControls, func(i, j int) bool { return sortedControls[i].Component < sortedControls[j].Component })

	var comps []ComponentReconciliation
	allReconciled := true
	for _, c := range sortedControls {
		subledger := subledgerByComponent[c.Component]
		diff := subledger - c.Balance
		tol := resolvedReconciliationTolerance(tolerance, c.Balance)
		reconciled := diff > -tol && diff < tol
		if !reconciled {
			allReconciled = false
		}
		comps = append(comps, ComponentReconciliation{
			Component:               c.Component,
			SubledgerInventoryValue: subledger,
			GLInventoryBalance:      c.Balance,
			Difference:              diff,
			Tolerance:               tol,
			Reconciled:              reconciled,
		})
	}
	return ReconciliationSummary{Available: true, Components: comps, AllReconciled: allReconciled}
}

// resolvedReconciliationTolerance returns the absolute-dollar tolerance
// for one comparison: AbsoluteTolerance if set, otherwise
// PercentTolerance * abs(referenceAmount), otherwise 0 (any difference is
// a difference).
func resolvedReconciliationTolerance(t ReconciliationTolerance, referenceAmount float64) float64 {
	if t.AbsoluteTolerance > 0 {
		return t.AbsoluteTolerance
	}
	if t.PercentTolerance > 0 {
		return t.PercentTolerance * absFloat(referenceAmount)
	}
	return amountTolerance // a hairline floating-point tolerance, never a business materiality threshold.
}

// QuantityRollforward is the quantity-side rollforward invariant — task
// sections 53, 68: Beginning + Inbound - Outbound +/- Adjustments =
// Ending.
type QuantityRollforward struct {
	Available bool `json:"available"`

	Beginning      Qty `json:"beginning"`
	Inbound        Qty `json:"inbound"`
	Outbound       Qty `json:"outbound"`
	Adjustments    Qty `json:"adjustments"` // net: increase - decrease - write-off.
	ExpectedEnding Qty `json:"expected_ending"`
	ActualEnding   Qty `json:"actual_ending"`

	Difference Qty     `json:"difference"`
	Tolerance  float64 `json:"tolerance"`
	Reconciled bool    `json:"reconciled"`
}

// buildQuantityRollforward computes the rollforward for one item, only
// when beginning, ending, and every movement quantity involved share a
// single UOM (task section 53's "only test when all necessary movement
// data exists" and section 62's UOM safety). Adjustments' net sign
// follows adjustmentSign (increase +, decrease/write-off -).
func buildQuantityRollforward(beginning, ending Qty, movements []Movement, tolerance float64) QuantityRollforward {
	r := QuantityRollforward{Beginning: beginning, ActualEnding: ending}
	if !beginning.Available || !ending.Available {
		return r
	}
	uom := beginning.UnitOfMeasure
	if ending.UnitOfMeasure != uom {
		return r
	}

	var inbound, outbound, adjustments qtyAccumulator
	// Seed each accumulator's UOM so a period with zero movements of one
	// kind (e.g. no adjustments at all) still reports 0-in-uom rather
	// than Unavailable, as long as beginning/ending share a UOM — an
	// empty movement list is a legitimate "no activity this period"
	// fact, not missing data.
	inbound.uom, inbound.seenUOM, inbound.ok = uom, true, true
	outbound.uom, outbound.seenUOM, outbound.ok = uom, true, true
	adjustments.uom, adjustments.seenUOM, adjustments.ok = uom, true, true

	for _, m := range movements {
		if !m.Quantity.Available || m.Quantity.UnitOfMeasure != uom {
			return r // incompatible/missing movement UOM data; rollforward cannot be computed reliably.
		}
		switch {
		case movementDirection(m.Type) == DirectionInbound:
			inbound.sum += m.Quantity.Amount
		case isAdjustmentType(m.Type):
			adjustments.sum += m.Quantity.Amount * adjustmentSign(m.Type)
		case isOutboundType(m.Type):
			outbound.sum += m.Quantity.Amount
		}
	}

	r.Inbound = inbound.result()
	r.Outbound = outbound.result()
	r.Adjustments = adjustments.result()

	expected := beginning.Amount + inbound.sum - outbound.sum + adjustments.sum
	r.ExpectedEnding = AvailableQty(expected, uom)
	diff := ending.Amount - expected
	r.Difference = AvailableQty(diff, uom)
	tol := tolerance
	if tol <= 0 {
		tol = quantityTolerance
	}
	r.Available = true
	r.Reconciled = diff > -tol && diff < tol
	return r
}

// ValueRollforward mirrors QuantityRollforward for extended value — task
// sections 54, 68. Only computed when beginning value, ending value, and
// every contributing movement value are all available and compatible
// (single currency) — this package never reconstructs a missing
// cost-flow value from quantity alone (that would silently assume a unit
// cost this package was never given).
type ValueRollforward struct {
	Available bool `json:"available"`

	Beginning      Value `json:"beginning"`
	Inbound        Value `json:"inbound"`
	Outbound       Value `json:"outbound"`
	Adjustments    Value `json:"adjustments"`
	ExpectedEnding Value `json:"expected_ending"`
	ActualEnding   Value `json:"actual_ending"`

	Difference Value   `json:"difference"`
	Tolerance  float64 `json:"tolerance"`
	Reconciled bool    `json:"reconciled"`
}

func buildValueRollforward(beginning, ending Value, movements []Movement, tolerance float64) ValueRollforward {
	r := ValueRollforward{Beginning: beginning, ActualEnding: ending}
	if !beginning.Available || !ending.Available {
		return r
	}

	var inbound, outbound, adjustments valueAccumulator
	inbound.ok, outbound.ok, adjustments.ok = true, true, true // zero-activity period is a legitimate fact — see buildQuantityRollforward's identical seeding rationale.

	allMovementValuesKnown := true
	for _, m := range movements {
		val, _ := resolveMovementValue(m)
		if !val.Available {
			allMovementValuesKnown = false
			break
		}
		switch {
		case movementDirection(m.Type) == DirectionInbound:
			inbound.sum += val.Amount
		case isAdjustmentType(m.Type):
			adjustments.sum += val.Amount * adjustmentSign(m.Type)
		case isOutboundType(m.Type):
			outbound.sum += val.Amount
		}
	}
	if !allMovementValuesKnown {
		return r
	}

	r.Inbound = inbound.result()
	r.Outbound = outbound.result()
	r.Adjustments = adjustments.result()

	expected := beginning.Amount + inbound.sum - outbound.sum + adjustments.sum
	r.ExpectedEnding = AvailableValue(expected)
	diff := ending.Amount - expected
	r.Difference = AvailableValue(diff)
	tol := tolerance
	if tol <= 0 {
		tol = amountTolerance
	}
	r.Available = true
	r.Reconciled = diff > -tol && diff < tol
	return r
}
