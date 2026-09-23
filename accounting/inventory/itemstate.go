package inventory

import (
	"sort"
	"time"
)

// ItemState is this package's internal, resolved as-of-date snapshot of
// one item: its most current InventorySnapshot(s) as of AsOfDate,
// aggregated quantity/value, and last-movement facts. Computed once in
// Calculate and passed to every downstream sub-analysis rather than
// recomputed repeatedly.
type itemState struct {
	item Item

	// snapshots is every valid InventorySnapshot for this item with
	// AsOfDate <= the analysis AsOfDate, most recent first per
	// (location, lot) — task section 12: only the latest snapshot per
	// (item, location, lot) as of the analysis date contributes to
	// on-hand totals; older snapshots remain available for aging/history
	// but are not double-counted into current quantity.
	latestByLocationLot map[[2]string]InventorySnapshot // key: (location, lotID)

	quantity Qty   // aggregated on-hand quantity, UOM-safe (see resolveItemQuantity)
	value    Value // aggregated on-hand value across all latest (location, lot) snapshots

	lastReceiptDate     *time.Time
	lastOutboundDate    *time.Time
	lastAnyMovementDate *time.Time
}

// buildItemStates resolves one itemState per known item, using the
// latest InventorySnapshot per (item, location, lot) with AsOfDate <=
// asOf, and last-movement dates from valid movements with Date <= asOf.
func buildItemStates(items map[string]Item, order []string, snapshots []InventorySnapshot, movements []Movement, asOf time.Time, uomTable uomConversionTable) map[string]*itemState {
	states := make(map[string]*itemState, len(order))
	for _, id := range order {
		states[id] = &itemState{item: items[id], latestByLocationLot: map[[2]string]InventorySnapshot{}}
	}

	for _, s := range snapshots {
		if s.AsOfDate.After(asOf) {
			continue // future-dated snapshot; not yet effective as of this analysis date.
		}
		st, ok := states[s.ItemID]
		if !ok {
			continue
		}
		key := [2]string{s.Location, s.LotID}
		existing, has := st.latestByLocationLot[key]
		if !has || s.AsOfDate.After(existing.AsOfDate) {
			st.latestByLocationLot[key] = s
		}
	}

	for _, m := range movements {
		if m.Date.After(asOf) {
			continue
		}
		st, ok := states[m.ItemID]
		if !ok {
			continue
		}
		d := m.Date
		switch {
		case m.Type == MovementPurchaseReceipt:
			if st.lastReceiptDate == nil || d.After(*st.lastReceiptDate) {
				st.lastReceiptDate = &d
			}
		case isOutboundType(m.Type):
			if st.lastOutboundDate == nil || d.After(*st.lastOutboundDate) {
				st.lastOutboundDate = &d
			}
		}
		if st.lastAnyMovementDate == nil || d.After(*st.lastAnyMovementDate) {
			st.lastAnyMovementDate = &d
		}
	}

	for _, id := range order {
		st := states[id]
		st.quantity, st.value = aggregateItemQuantityValue(st.item, st.latestByLocationLot, uomTable)
	}
	return states
}

// aggregateItemQuantityValue sums value unconditionally (money always
// aggregates in one reporting currency) and sums quantity only when every
// contributing snapshot's UOM matches the item's own UnitOfMeasure (or an
// explicit UOMConversion resolves it) — task section 62's UOM safety rule.
func aggregateItemQuantityValue(item Item, byLocationLot map[[2]string]InventorySnapshot, uomTable uomConversionTable) (Qty, Value) {
	var totalValue float64
	var hasValue bool
	var totalQty float64
	qtyAvailable := true
	qtyUOM := item.UnitOfMeasure

	// Deterministic iteration: sort keys before floating-point
	// accumulation (task section 70's "sort before floating-point
	// accumulation where map-backed collections are involved").
	keys := make([][2]string, 0, len(byLocationLot))
	for k := range byLocationLot {
		keys = append(keys, k)
	}
	sortLocationLotKeys(keys)

	if len(byLocationLot) == 0 {
		qtyAvailable = false
	}

	for _, k := range keys {
		s := byLocationLot[k]
		val, _ := resolveSnapshotValue(s)
		if val.Available {
			totalValue += val.Amount
			hasValue = true
		}

		if !qtyAvailable {
			continue
		}
		if !s.QuantityOnHand.Available {
			qtyAvailable = false
			continue
		}
		amt, ok := resolveCompatibleQuantity(s.QuantityOnHand, qtyUOM, uomTable)
		if !ok {
			qtyAvailable = false
			continue
		}
		totalQty += amt
	}

	qty := UnavailableQty()
	if qtyAvailable && len(byLocationLot) > 0 {
		qty = AvailableQty(totalQty, qtyUOM)
	}
	value := Unavailable()
	if hasValue {
		value = AvailableValue(totalValue)
	}
	return qty, value
}

// resolveCompatibleQuantity returns q's amount expressed in targetUOM: as-
// is if q.UnitOfMeasure == targetUOM (or either is empty, treated as "no
// stated UOM conflict" — a caller who never populates UnitOfMeasure at all
// still gets a working quantity total), or converted via an explicit
// UOMConversion table entry. Returns (0, false) if the units differ and no
// explicit conversion resolves them — task section 62/63: never infer a
// conversion.
func resolveCompatibleQuantity(q Qty, targetUOM string, uomTable uomConversionTable) (float64, bool) {
	if q.UnitOfMeasure == "" || targetUOM == "" || q.UnitOfMeasure == targetUOM {
		return q.Amount, true
	}
	return uomTable.convert(q.Amount, q.UnitOfMeasure, targetUOM)
}

// sortLocationLotKeys sorts keys ascending by (location, lot). keys is
// per-item and typically tiny (distinct location/lot combinations for one
// SKU) — never the O(N-items) hot path benchmark_test.go exercises at
// scale.
func sortLocationLotKeys(keys [][2]string) {
	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
}
