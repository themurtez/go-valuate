package profitability

import "github.com/themurtez/go-valuate/accounting/inventory"

// ProductCostFactFromInventorySnapshot converts one accounting/inventory
// InventorySnapshot's authoritative UnitCost into a single
// DIRECT_MATERIAL Fact for one sale quantity, explicitly attributed to
// attributions — task section 41. This is a typed adapter, not a
// compile-time dependency baked into Calculate: this package's core
// types never import accounting/inventory, and a caller with no
// inventory package usage never needs this function.
//
// Only InventorySnapshot.UnitCost — a fact the caller directly supplied —
// is used; this adapter never invents a per-unit cost from turnover,
// value, or any other accounting/inventory analytic (task section 41's
// explicit "do not invent item-level COGS" instruction). If UnitCost is
// Unavailable, this function returns (Fact{}, false): the caller must
// treat product cost as unavailable for that item rather than receiving
// a Fact with a fabricated amount.
func ProductCostFactFromInventorySnapshot(snapshot inventory.InventorySnapshot, saleQuantity float64, factID, period string, attributions []Attribution) (Fact, bool) {
	if !snapshot.UnitCost.Available {
		return Fact{}, false
	}
	return Fact{
		FactID:       factID,
		Period:       period,
		Component:    ComponentDirectMaterial,
		Amount:       snapshot.UnitCost.Amount * saleQuantity,
		Attributions: attributions,
		SourceType:   "accounting/inventory.InventorySnapshot",
		SourceID:     snapshot.ID,
		Currency:     snapshot.Currency,
	}, true
}
