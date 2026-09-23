package inventory

// MarketValue is an optional, explicit caller-supplied net-realizable-
// value/expected-selling-price comparison basis for one item — task
// section 22. This package never sources a selling price, never infers
// net realizable value, and never automatically posts a write-down; it
// only computes a labeled analytical comparison when the caller supplies
// this basis directly.
type MarketValue struct {
	ItemID string `json:"item_id"`

	// NetRealizableValue is the caller-supplied NRV basis for this item's
	// on-hand quantity (extended, not per-unit) — preferred comparison
	// basis when supplied.
	NetRealizableValue Value `json:"net_realizable_value"`
	// ExpectedSellingPrice/DisposalCosts are an alternative, more granular
	// basis: NRV is derived as ExpectedSellingPrice - DisposalCosts when
	// NetRealizableValue itself is not directly supplied.
	ExpectedSellingPrice Value `json:"expected_selling_price"`
	DisposalCosts        Value `json:"disposal_costs"`
}

// MarketValueComparison is the analytical (never accounting-authoritative)
// comparison of an item's inventory cost against its supplied market-value
// basis — task section 22: "label as analytical comparison."
type MarketValueComparison struct {
	ItemID string `json:"item_id"`

	InventoryCost      Value `json:"inventory_cost"`
	NetRealizableValue Value `json:"net_realizable_value"`
	// Difference is InventoryCost - NetRealizableValue; positive means
	// cost exceeds the supplied market-value basis (a candidate for
	// caller review, never a computed write-down).
	Difference Value `json:"difference"`

	// Label is a fixed, always-present disclaimer distinguishing this
	// analytical comparison from an accounting write-down determination.
	Label string `json:"label"`
}

const marketValueComparisonLabel = "analytical cost-vs-supplied-market-value comparison; not a GAAP/IFRS write-down determination"

// resolveNetRealizableValue returns mv's NRV: NetRealizableValue directly
// if available, otherwise ExpectedSellingPrice - DisposalCosts if both
// components are available, otherwise Unavailable. DisposalCosts defaults
// to 0 when unavailable but ExpectedSellingPrice is available (an
// unsupplied disposal cost is not treated as "NRV unavailable" — it is
// simply zero cost to net out, the same zero-vs-absent distinction this
// package elsewhere reserves for quantity/policy fields where zero and
// absent are genuinely ambiguous; here ExpectedSellingPrice alone is
// already a complete, usable NRV basis).
func resolveNetRealizableValue(mv MarketValue) Value {
	if mv.NetRealizableValue.Available {
		return mv.NetRealizableValue
	}
	if mv.ExpectedSellingPrice.Available {
		disposal := 0.0
		if mv.DisposalCosts.Available {
			disposal = mv.DisposalCosts.Amount
		}
		return AvailableValue(mv.ExpectedSellingPrice.Amount - disposal)
	}
	return Unavailable()
}

// buildMarketValueComparison computes one item's cost-vs-NRV comparison.
// Available only if both inventoryCost and the resolved NRV are available.
func buildMarketValueComparison(itemID string, inventoryCost Value, mv MarketValue) MarketValueComparison {
	nrv := resolveNetRealizableValue(mv)
	c := MarketValueComparison{ItemID: itemID, InventoryCost: inventoryCost, NetRealizableValue: nrv, Label: marketValueComparisonLabel}
	if inventoryCost.Available && nrv.Available {
		c.Difference = AvailableValue(inventoryCost.Amount - nrv.Amount)
	}
	return c
}
