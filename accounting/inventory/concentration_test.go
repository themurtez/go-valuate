package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestConcentration_ByItem verifies task section 28: a dominant item's
// value share and HHI are computed via analytics/concentration reuse.
func TestConcentration_ByItem(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items: []inventory.Item{
			item("ITEM-1", "Widgets"),
			item("ITEM-2", "Widgets"),
			item("ITEM-3", "Widgets"),
		},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", asOf, 1000, 1), // $1000 - dominant.
			snapshot("SNAP-2", "ITEM-2", asOf, 10, 1),   // $10
			snapshot("SNAP-3", "ITEM-3", asOf, 10, 1),   // $10
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Concentration.ByItem.Available {
		t.Fatalf("expected ByItem.Available=true, got %+v", result.Concentration.ByItem)
	}
	if len(result.Concentration.ByItem.History) != 1 {
		t.Fatalf("expected 1 concentration period, got %d", len(result.Concentration.ByItem.History))
	}
	largest := result.Concentration.ByItem.History[0].LargestEntityShare
	if !largest.Available || largest.Value < 0.9 {
		t.Errorf("LargestEntityShare = %+v, want >= 0.9 (dominant item)", largest)
	}
	if result.Concentration.Label == "" {
		t.Errorf("expected a non-empty concentration disclaimer Label")
	}
}

// TestConcentration_ByCategory verifies category-level concentration.
func TestConcentration_ByCategory(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items: []inventory.Item{
			item("ITEM-1", "Electronics"),
			item("ITEM-2", "Electronics"),
			item("ITEM-3", "Office Supplies"),
		},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", asOf, 500, 1),
			snapshot("SNAP-2", "ITEM-2", asOf, 500, 1),
			snapshot("SNAP-3", "ITEM-3", asOf, 10, 1),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Concentration.ByCategory.Available {
		t.Fatalf("expected ByCategory.Available=true, got %+v", result.Concentration.ByCategory)
	}
}

// TestConcentration_ByLocation verifies location-level concentration
// using per-snapshot Location, correctly avoiding double-counting when an
// item spans multiple locations.
func TestConcentration_ByLocation(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: asOf, Location: "WH1",
				QuantityOnHand: inventory.AvailableQty(100, "EA"), UnitCost: inventory.AvailableValue(1), Currency: "USD"},
			{ID: "SNAP-2", ItemID: "ITEM-1", AsOfDate: asOf, Location: "WH2",
				QuantityOnHand: inventory.AvailableQty(50, "EA"), UnitCost: inventory.AvailableValue(1), Currency: "USD"},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Concentration.ByLocation.Available {
		t.Fatalf("expected ByLocation.Available=true, got %+v", result.Concentration.ByLocation)
	}
	// Total across locations must equal item total (100+50=150), never
	// double-counted (which would show 300 if WH1/WH2 both got the full
	// item-level $150).
	total := result.Concentration.ByLocation.History[0].TotalAmount
	if !total.Available || total.Value != 150 {
		t.Errorf("ByLocation total = %+v, want Available=true Value=150 (never double-counted)", total)
	}
}
