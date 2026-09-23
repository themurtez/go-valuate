package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestNRV_DirectValue verifies task section 22: an explicit
// NetRealizableValue is compared directly against inventory cost.
func TestNRV_DirectValue(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", asOf, 100, 10)}, // cost = $1000.
		MarketValues: []inventory.MarketValue{
			{ItemID: "ITEM-1", NetRealizableValue: inventory.AvailableValue(800)},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.MarketValueComparisons) != 1 {
		t.Fatalf("expected 1 comparison, got %d", len(result.MarketValueComparisons))
	}
	c := result.MarketValueComparisons[0]
	if !c.Difference.Available || c.Difference.Amount != 200 {
		t.Errorf("Difference = %+v, want Available=true Amount=200", c.Difference)
	}
	if c.Label == "" {
		t.Errorf("expected a non-empty analytical-comparison Label")
	}
}

// TestNRV_DerivedFromSellingPriceAndDisposalCosts verifies task section
// 22's alternative basis.
func TestNRV_DerivedFromSellingPriceAndDisposalCosts(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", asOf, 100, 10)}, // cost = $1000.
		MarketValues: []inventory.MarketValue{
			{ItemID: "ITEM-1", ExpectedSellingPrice: inventory.AvailableValue(1200), DisposalCosts: inventory.AvailableValue(300)},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	c := result.MarketValueComparisons[0]
	// NRV = 1200 - 300 = 900. Difference = 1000 - 900 = 100.
	if !c.NetRealizableValue.Available || c.NetRealizableValue.Amount != 900 {
		t.Errorf("NetRealizableValue = %+v, want Available=true Amount=900", c.NetRealizableValue)
	}
	if !c.Difference.Available || c.Difference.Amount != 100 {
		t.Errorf("Difference = %+v, want Available=true Amount=100", c.Difference)
	}
}

// TestNRV_NoAutomaticWriteDown verifies task section 22: this comparison
// never changes InventoryValue/portfolio totals — it is analytical only.
func TestNRV_NoAutomaticWriteDown(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", asOf, 100, 10)},
		MarketValues: []inventory.MarketValue{
			{ItemID: "ITEM-1", NetRealizableValue: inventory.AvailableValue(1)}, // drastically below cost.
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if result.Portfolio.TotalInventoryValue != 1000 {
		t.Errorf("TotalInventoryValue = %v, want 1000 (never auto-written-down)", result.Portfolio.TotalInventoryValue)
	}
}

// TestNRV_UnavailableWithoutSupply verifies task section 22: no NRV is
// ever inferred without explicit supply.
func TestNRV_UnavailableWithoutSupply(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", asOf, 100, 10)},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.MarketValueComparisons) != 0 {
		t.Errorf("expected no comparisons without supplied MarketValue, got %+v", result.MarketValueComparisons)
	}
}
