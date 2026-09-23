package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestVelocity_OutboundQuantityPerDay verifies task section 24.
func TestVelocity_OutboundQuantityPerDay(t *testing.T) {
	windowStart := mustDate(t, "2025-01-01")
	windowEnd := mustDate(t, "2025-01-10") // 10-day window.
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", mustDate(t, "2025-01-05"), inventory.MovementCustomerShipment, 100, 5),
		},
		VelocityWindows: map[string]inventory.VelocityWindow{
			"ITEM-1": {StartDate: windowStart, EndDate: windowEnd},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.Velocity) != 1 {
		t.Fatalf("expected 1 velocity row, got %d", len(result.Velocity))
	}
	v := result.Velocity[0]
	if !v.OutboundQuantityPerDay.Available || v.OutboundQuantityPerDay.Amount != 10 {
		t.Errorf("OutboundQuantityPerDay = %+v, want Available=true Amount=10 (100/10 days)", v.OutboundQuantityPerDay)
	}
	if !v.OutboundQuantityPerWeek.Available || v.OutboundQuantityPerWeek.Amount != 70 {
		t.Errorf("OutboundQuantityPerWeek = %+v, want Available=true Amount=70", v.OutboundQuantityPerWeek)
	}
}

// TestVelocity_SupplyDurationAvailable verifies task section 25's normal
// path.
func TestVelocity_SupplyDurationAvailable(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-01-31"), 100, 5),
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", mustDate(t, "2025-01-05"), inventory.MovementCustomerShipment, 20, 5),
		},
		VelocityWindows: map[string]inventory.VelocityWindow{
			"ITEM-1": {StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-10")}, // 10 days, 20 units = 2/day.
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.SupplyDuration) != 1 {
		t.Fatalf("expected 1 supply duration row, got %d", len(result.SupplyDuration))
	}
	sd := result.SupplyDuration[0]
	if !sd.DaysOfSupply.Available {
		t.Fatalf("expected DaysOfSupply.Available=true, got %+v", sd)
	}
	// 100 on hand / 2 per day = 50 days.
	if sd.DaysOfSupply.Amount != 50 {
		t.Errorf("DaysOfSupply = %v, want 50", sd.DaysOfSupply.Amount)
	}
}

// TestVelocity_SupplyDurationUnavailableNotInfinite verifies task section
// 25: zero recent outbound movement yields Unavailable, never Inf.
func TestVelocity_SupplyDurationUnavailableNotInfinite(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-01-31"), 100, 5),
		},
		VelocityWindows: map[string]inventory.VelocityWindow{
			"ITEM-1": {StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-10")}, // no outbound movements in window.
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	sd := result.SupplyDuration[0]
	if sd.DaysOfSupply.Available {
		t.Errorf("expected DaysOfSupply.Available=false for zero recent outbound movement, got %+v", sd.DaysOfSupply)
	}
	if isInfOrNaN(sd.DaysOfSupply.Amount) {
		t.Errorf("DaysOfSupply.Amount must not be Inf/NaN, got %v", sd.DaysOfSupply.Amount)
	}
}
