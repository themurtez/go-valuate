package inventory

// StockPolicy is a caller-supplied explicit target/min/max/reorder set
// for one item — task section 26. This package never infers a reorder
// point, minimum, or maximum from historical usage; without an explicit
// StockPolicy for an item, this package makes no overstock/understock
// claim about it at all (task section 27) — old/slow-moving findings
// remain independently available.
type StockPolicy struct {
	ItemID string `json:"item_id"`

	// MinimumQuantity/TargetQuantity/MaximumQuantity/ReorderPoint are all
	// optional; a zero value is ambiguous with "not supplied" for a
	// quantity policy (a caller who truly wants a minimum of exactly zero
	// units gets identical treatment to one who supplied no minimum at
	// all — both mean "no minimum floor to compare against"), so each has
	// a companion *Set bool distinguishing the two, mirroring this
	// package's Value/Qty explicit-availability convention. See
	// stockpolicy_test.go for the exact-zero-vs-absent distinction this
	// preserves.
	MinimumQuantity    float64 `json:"minimum_quantity,omitempty"`
	MinimumQuantitySet bool    `json:"minimum_quantity_set,omitempty"`
	TargetQuantity     float64 `json:"target_quantity,omitempty"`
	TargetQuantitySet  bool    `json:"target_quantity_set,omitempty"`
	MaximumQuantity    float64 `json:"maximum_quantity,omitempty"`
	MaximumQuantitySet bool    `json:"maximum_quantity_set,omitempty"`
	ReorderPoint       float64 `json:"reorder_point,omitempty"`
	ReorderPointSet    bool    `json:"reorder_point_set,omitempty"`
}

// StockPolicyResult is one item's on-hand quantity compared against its
// StockPolicy, analytical only (task section 26: "do not create purchase
// orders").
type StockPolicyResult struct {
	ItemID string `json:"item_id"`

	QuantityOnHand Qty         `json:"quantity_on_hand"`
	Policy         StockPolicy `json:"policy"`

	// BelowMinimum/AboveMaximum/BelowReorderPoint are Value{Available:
	// false} (not merely false) when the corresponding policy field was
	// not set (*Set == false) or QuantityOnHand.Available is false — task
	// section 27's "without explicit min/max/target, do not call
	// inventory overstocked/understocked" rule. Amount is 1 (true) or 0
	// (false) when available, so this stays a JSON-safe Value rather than
	// introducing a fourth bool-availability wrapper type.
	BelowMinimum      BoolResult `json:"below_minimum"`
	AboveMaximum      BoolResult `json:"above_maximum"`
	BelowReorderPoint BoolResult `json:"below_reorder_point"`

	// ShortfallVsMinimum is Minimum - QuantityOnHand when positive (0 if
	// at or above minimum), Unavailable if MinimumQuantitySet is false.
	ShortfallVsMinimum Qty `json:"shortfall_vs_minimum"`
	// ExcessQuantityVsMaximum is QuantityOnHand - Maximum when positive,
	// Unavailable if MaximumQuantitySet is false.
	ExcessQuantityVsMaximum Qty `json:"excess_quantity_vs_maximum"`
}

// BoolResult is a boolean figure that may or may not be available — this
// package's explicit-availability wrapper for a fact that is otherwise
// indistinguishable between "computed false" and "not evaluated," used
// only where a caller-supplied policy field must gate a comparison
// (StockPolicyResult).
type BoolResult struct {
	Available bool `json:"available"`
	Value     bool `json:"value"`
}

// UnavailableBool is the canonical zero-information BoolResult.
func UnavailableBool() BoolResult { return BoolResult{} }

// AvailableBool reports a known boolean result.
func AvailableBool(v bool) BoolResult { return BoolResult{Available: true, Value: v} }

// buildStockPolicyResult compares qty against policy. qty must be a
// same-UOM-resolved Qty (see resolveItemQuantity); if !qty.Available, every
// comparison is unavailable regardless of policy.
func buildStockPolicyResult(itemID string, qty Qty, policy StockPolicy) StockPolicyResult {
	r := StockPolicyResult{ItemID: itemID, QuantityOnHand: qty, Policy: policy}
	if !qty.Available {
		return r
	}
	if policy.MinimumQuantitySet {
		r.BelowMinimum = AvailableBool(qty.Amount < policy.MinimumQuantity)
		shortfall := policy.MinimumQuantity - qty.Amount
		if shortfall < 0 {
			shortfall = 0
		}
		r.ShortfallVsMinimum = AvailableQty(shortfall, qty.UnitOfMeasure)
	}
	if policy.MaximumQuantitySet {
		r.AboveMaximum = AvailableBool(qty.Amount > policy.MaximumQuantity)
		excess := qty.Amount - policy.MaximumQuantity
		if excess < 0 {
			excess = 0
		}
		r.ExcessQuantityVsMaximum = AvailableQty(excess, qty.UnitOfMeasure)
	}
	if policy.ReorderPointSet {
		r.BelowReorderPoint = AvailableBool(qty.Amount < policy.ReorderPoint)
	}
	return r
}
