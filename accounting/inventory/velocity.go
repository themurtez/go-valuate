package inventory

import "time"

// VelocityWindow is a caller-supplied historical window used for movement
// velocity and supply-duration analysis — task sections 24-25. This
// package never picks its own window; the caller always supplies one
// explicitly.
type VelocityWindow struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// days returns the inclusive day count of w, or 0 if invalid.
func (w VelocityWindow) days() int {
	if w.StartDate.IsZero() || w.EndDate.IsZero() || w.EndDate.Before(w.StartDate) {
		return 0
	}
	return int(w.EndDate.Sub(w.StartDate).Hours()/24) + 1
}

// ItemVelocity is one item's outbound movement velocity over a
// caller-supplied window — task section 24.
type ItemVelocity struct {
	ItemID string         `json:"item_id"`
	Window VelocityWindow `json:"window"`

	OutboundQuantityPerDay  Qty   `json:"outbound_quantity_per_day"`
	OutboundQuantityPerWeek Qty   `json:"outbound_quantity_per_week"`
	OutboundValuePerDay     Value `json:"outbound_value_per_day"`
}

func buildItemVelocity(itemID string, window VelocityWindow, movements []Movement) ItemVelocity {
	v := ItemVelocity{ItemID: itemID, Window: window}
	days := window.days()
	if days <= 0 {
		return v
	}

	var qty qtyAccumulator
	var val valueAccumulator
	for _, m := range movements {
		if m.ItemID != itemID || !isOutboundType(m.Type) {
			continue
		}
		if m.Date.Before(window.StartDate) || m.Date.After(window.EndDate) {
			continue
		}
		qty.add(m.Quantity)
		amt, _ := resolveMovementValue(m)
		val.add(amt)
	}

	if qty.ok {
		v.OutboundQuantityPerDay = AvailableQty(qty.sum/float64(days), qty.uom)
		v.OutboundQuantityPerWeek = AvailableQty(qty.sum/float64(days)*7, qty.uom)
	}
	if val.ok {
		v.OutboundValuePerDay = AvailableValue(val.sum / float64(days))
	}
	return v
}

// SupplyDuration is the optional "weeks/months of supply" analytical
// metric — task section 25: QuantityOnHand / RecentAverageOutboundQuantity,
// with explicit basis. Not demand forecasting.
type SupplyDuration struct {
	ItemID string         `json:"item_id"`
	Window VelocityWindow `json:"window"`

	QuantityOnHand              Qty `json:"quantity_on_hand"`
	RecentAverageOutboundPerDay Qty `json:"recent_average_outbound_per_day"`

	// DaysOfSupply is unavailable (never infinite) when
	// RecentAverageOutboundPerDay is zero/unavailable — task section 25:
	// "supply duration unavailable / no recent outbound movement, not
	// infinite."
	DaysOfSupply  Value `json:"days_of_supply"`
	WeeksOfSupply Value `json:"weeks_of_supply"`
}

func buildSupplyDuration(itemID string, onHand Qty, velocity ItemVelocity) SupplyDuration {
	s := SupplyDuration{ItemID: itemID, Window: velocity.Window, QuantityOnHand: onHand, RecentAverageOutboundPerDay: velocity.OutboundQuantityPerDay}
	if !onHand.Available || !velocity.OutboundQuantityPerDay.Available {
		return s
	}
	if velocity.OutboundQuantityPerDay.UnitOfMeasure != onHand.UnitOfMeasure {
		return s // incompatible UOM; never silently mixed.
	}
	if velocity.OutboundQuantityPerDay.Amount == 0 {
		return s // "no recent outbound movement" — Unavailable, never Inf.
	}
	days := onHand.Amount / velocity.OutboundQuantityPerDay.Amount
	s.DaysOfSupply = AvailableValue(days)
	s.WeeksOfSupply = AvailableValue(days / 7)
	return s
}
