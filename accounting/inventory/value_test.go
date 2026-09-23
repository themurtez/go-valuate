package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestValue_QuantityTimesCost verifies task section 8: value is computed
// analytically when only Quantity x UnitCost is supplied.
func TestValue_QuantityTimesCost(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), 50, 4),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if result.Portfolio.TotalInventoryValue != 200 {
		t.Errorf("TotalInventoryValue = %v, want 200", result.Portfolio.TotalInventoryValue)
	}
}

// TestValue_ExplicitValuePreserved verifies task section 4/8: an explicit
// InventoryValue is preserved and used as-is, without needing Quantity or
// UnitCost at all.
func TestValue_ExplicitValuePreserved(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-06-30"),
				InventoryValue: inventory.AvailableValue(999), Currency: "USD"},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if result.Portfolio.TotalInventoryValue != 999 {
		t.Errorf("TotalInventoryValue = %v, want 999", result.Portfolio.TotalInventoryValue)
	}
	if hasIssueCode(result.Issues, inventory.IssueValueQuantityCostMismatch) {
		t.Errorf("unexpected mismatch issue when quantity/cost were never supplied: %+v", result.Issues)
	}
}

// TestValue_MismatchFlagged verifies task section 8: when both an
// explicit InventoryValue and Quantity x UnitCost are supplied and
// materially disagree, the explicit value wins but a structured issue is
// reported — never silently overwritten either way.
func TestValue_MismatchFlagged(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-06-30"),
				QuantityOnHand: inventory.AvailableQty(50, "EA"),
				UnitCost:       inventory.AvailableValue(4),
				InventoryValue: inventory.AvailableValue(500), // disagrees with 50*4=200
				Currency:       "USD"},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if result.Portfolio.TotalInventoryValue != 500 {
		t.Errorf("TotalInventoryValue = %v, want 500 (explicit value wins)", result.Portfolio.TotalInventoryValue)
	}
	if !hasIssueCode(result.Issues, inventory.IssueValueQuantityCostMismatch) {
		t.Errorf("expected IssueValueQuantityCostMismatch, got %+v", result.Issues)
	}
}

// TestValue_ZeroInventory verifies a known-zero quantity/value is treated
// as a real fact, not "unavailable."
func TestValue_ZeroInventory(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), 0, 4),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if result.Portfolio.TotalInventoryValue != 0 {
		t.Errorf("TotalInventoryValue = %v, want 0", result.Portfolio.TotalInventoryValue)
	}
	if result.Portfolio.ItemsWithZeroStock != 1 {
		t.Errorf("ItemsWithZeroStock = %d, want 1", result.Portfolio.ItemsWithZeroStock)
	}
}

// TestValue_NegativeInventoryAllowWithWarning verifies task section 9's
// default policy: negative inventory is included with a warning, never
// auto-corrected to zero.
func TestValue_NegativeInventoryAllowWithWarning(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), -10, 4),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	if result.Portfolio.TotalInventoryValue != -40 {
		t.Errorf("TotalInventoryValue = %v, want -40 (never auto-corrected to zero)", result.Portfolio.TotalInventoryValue)
	}
	if result.Portfolio.ItemsWithNegativeStock != 1 {
		t.Errorf("ItemsWithNegativeStock = %d, want 1", result.Portfolio.ItemsWithNegativeStock)
	}
	if !hasFlagCode(result.Flags, inventory.FlagNegativeInventory) {
		t.Errorf("expected FlagNegativeInventory, got %+v", result.Flags)
	}
}

// TestValue_NegativeInventoryReject verifies the REJECT policy excludes
// the row entirely with an error issue.
func TestValue_NegativeInventoryReject(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), -10, 4),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{NegativeInventoryHandling: inventory.NegativeInventoryReject})
	if !inventory.HasErrors(result.Issues) {
		t.Fatalf("expected an error issue for rejected negative inventory, got %+v", result.Issues)
	}
	if result.Portfolio.TotalInventoryValue != 0 {
		t.Errorf("TotalInventoryValue = %v, want 0 (row excluded)", result.Portfolio.TotalInventoryValue)
	}
}

// TestValue_MixedUOMQuantityUnavailable verifies task section 62: summing
// quantities across incompatible units of measure never happens silently;
// the item-level aggregate becomes unavailable while value still
// aggregates normally.
func TestValue_MixedUOMQuantityUnavailable(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{{ID: "ITEM-1", Category: "Chemicals", Active: true, Currency: "USD", UnitOfMeasure: "KG"}},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-06-30"), Location: "WH1",
				QuantityOnHand: inventory.AvailableQty(50, "KG"), UnitCost: inventory.AvailableValue(2), Currency: "USD"},
			{ID: "SNAP-2", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-06-30"), Location: "WH2",
				QuantityOnHand: inventory.AvailableQty(30, "L"), UnitCost: inventory.AvailableValue(3), Currency: "USD"},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	// Value aggregates regardless of UOM: 50*2 + 30*3 = 190.
	if result.Portfolio.TotalInventoryValue != 190 {
		t.Errorf("TotalInventoryValue = %v, want 190", result.Portfolio.TotalInventoryValue)
	}
	if len(result.LastMovement) == 0 {
		t.Fatalf("expected item states to be built")
	}
}

// TestValue_UOMConversionResolvesQuantity verifies task section 63: an
// explicit UOMConversion allows compatible aggregation across snapshots
// with different (but convertible) units — asserted via
// StockPolicyResult.QuantityOnHand, this package's exposed resolved
// item-level quantity.
func TestValue_UOMConversionResolvesQuantity(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{{ID: "ITEM-1", Category: "Boxes", Active: true, Currency: "USD", UnitOfMeasure: "EA"}},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-06-30"), Location: "WH1",
				QuantityOnHand: inventory.AvailableQty(10, "CASE"), UnitCost: inventory.AvailableValue(1), Currency: "USD"},
		},
		StockPolicies: []inventory.StockPolicy{{ItemID: "ITEM-1", MinimumQuantitySet: true, MinimumQuantity: 1}},
	}
	policy := inventory.Policy{UOMConversions: []inventory.UOMConversion{{From: "CASE", To: "EA", Factor: 24}}}
	result := inventory.Calculate(in, policy)
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	if len(result.StockPolicyResults) != 1 {
		t.Fatalf("expected 1 stock policy result, got %d", len(result.StockPolicyResults))
	}
	qty := result.StockPolicyResults[0].QuantityOnHand
	if !qty.Available {
		t.Fatalf("expected QuantityOnHand.Available=true, got %+v", qty)
	}
	if qty.Amount != 240 || qty.UnitOfMeasure != "EA" {
		t.Errorf("QuantityOnHand = %+v, want 240 EA (10 CASE x 24)", qty)
	}
}

// TestValue_MixedUOMWithoutConversionQuantityUnavailable is the negative
// counterpart: without an explicit UOMConversion, incompatible units
// never silently combine — quantity aggregation for the item becomes
// unavailable rather than guessing.
func TestValue_MixedUOMWithoutConversionQuantityUnavailable(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{{ID: "ITEM-1", Category: "Chemicals", Active: true, Currency: "USD", UnitOfMeasure: "KG"}},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-06-30"), Location: "WH1",
				QuantityOnHand: inventory.AvailableQty(50, "KG"), UnitCost: inventory.AvailableValue(2), Currency: "USD"},
			{ID: "SNAP-2", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-06-30"), Location: "WH2",
				QuantityOnHand: inventory.AvailableQty(30, "L"), UnitCost: inventory.AvailableValue(3), Currency: "USD"},
		},
		StockPolicies: []inventory.StockPolicy{{ItemID: "ITEM-1", MinimumQuantitySet: true, MinimumQuantity: 1}},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.StockPolicyResults) != 1 {
		t.Fatalf("expected 1 stock policy result, got %d", len(result.StockPolicyResults))
	}
	qty := result.StockPolicyResults[0].QuantityOnHand
	if qty.Available {
		t.Errorf("expected QuantityOnHand.Available=false for incompatible UOM without conversion, got %+v", qty)
	}
	// The comparison itself must then be unavailable too, never a guess.
	if result.StockPolicyResults[0].BelowMinimum.Available {
		t.Errorf("expected BelowMinimum.Available=false when quantity is unavailable, got %+v", result.StockPolicyResults[0].BelowMinimum)
	}
}
