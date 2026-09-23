package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

func baseStockPolicyInput(t *testing.T, qty float64) inventory.Input {
	return inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), qty, 5),
		},
	}
}

// TestStockPolicy_BelowMinimum verifies task section 26.
func TestStockPolicy_BelowMinimum(t *testing.T) {
	in := baseStockPolicyInput(t, 5)
	in.StockPolicies = []inventory.StockPolicy{{ItemID: "ITEM-1", MinimumQuantitySet: true, MinimumQuantity: 10}}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.StockPolicyResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.StockPolicyResults))
	}
	r := result.StockPolicyResults[0]
	if !r.BelowMinimum.Available || !r.BelowMinimum.Value {
		t.Errorf("BelowMinimum = %+v, want Available=true Value=true", r.BelowMinimum)
	}
	if !r.ShortfallVsMinimum.Available || r.ShortfallVsMinimum.Amount != 5 {
		t.Errorf("ShortfallVsMinimum = %+v, want Available=true Amount=5", r.ShortfallVsMinimum)
	}
	if !hasFlagCode(result.Flags, inventory.FlagBelowCallerMinimum) {
		t.Errorf("expected FlagBelowCallerMinimum, got %+v", result.Flags)
	}
}

// TestStockPolicy_AboveMaximum verifies task section 26.
func TestStockPolicy_AboveMaximum(t *testing.T) {
	in := baseStockPolicyInput(t, 50)
	in.StockPolicies = []inventory.StockPolicy{{ItemID: "ITEM-1", MaximumQuantitySet: true, MaximumQuantity: 20}}
	result := inventory.Calculate(in, inventory.Policy{})
	r := result.StockPolicyResults[0]
	if !r.AboveMaximum.Available || !r.AboveMaximum.Value {
		t.Errorf("AboveMaximum = %+v, want Available=true Value=true", r.AboveMaximum)
	}
	if !r.ExcessQuantityVsMaximum.Available || r.ExcessQuantityVsMaximum.Amount != 30 {
		t.Errorf("ExcessQuantityVsMaximum = %+v, want Available=true Amount=30", r.ExcessQuantityVsMaximum)
	}
	if !hasFlagCode(result.Flags, inventory.FlagAboveCallerMaximum) {
		t.Errorf("expected FlagAboveCallerMaximum, got %+v", result.Flags)
	}
}

// TestStockPolicy_ReorderPoint verifies task section 26.
func TestStockPolicy_ReorderPoint(t *testing.T) {
	in := baseStockPolicyInput(t, 8)
	in.StockPolicies = []inventory.StockPolicy{{ItemID: "ITEM-1", ReorderPointSet: true, ReorderPoint: 15}}
	result := inventory.Calculate(in, inventory.Policy{})
	r := result.StockPolicyResults[0]
	if !r.BelowReorderPoint.Available || !r.BelowReorderPoint.Value {
		t.Errorf("BelowReorderPoint = %+v, want Available=true Value=true", r.BelowReorderPoint)
	}
}

// TestStockPolicy_NoPolicyNoOverUnderstockClaim verifies task section 27:
// without an explicit StockPolicy, this package makes no
// overstock/understock claim — no StockPolicyResult is even produced for
// that item.
func TestStockPolicy_NoPolicyNoOverUnderstockClaim(t *testing.T) {
	in := baseStockPolicyInput(t, 100000) // an enormous quantity that would look "overstocked" under any guessed threshold.
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.StockPolicyResults) != 0 {
		t.Errorf("expected no StockPolicyResults without a supplied StockPolicy, got %+v", result.StockPolicyResults)
	}
	if hasFlagCode(result.Flags, inventory.FlagAboveCallerMaximum) || hasFlagCode(result.Flags, inventory.FlagBelowCallerMinimum) {
		t.Errorf("expected no stock-level flags without a supplied StockPolicy, got %+v", result.Flags)
	}
}

// TestStockPolicy_ZeroMinimumVsAbsent verifies the explicit *Set
// distinction: a genuine zero minimum behaves differently from "no
// minimum supplied at all."
func TestStockPolicy_ZeroMinimumVsAbsent(t *testing.T) {
	in := baseStockPolicyInput(t, -5) // negative, so "below zero" is meaningfully different from "no claim."
	in.StockPolicies = []inventory.StockPolicy{{ItemID: "ITEM-1", MinimumQuantitySet: true, MinimumQuantity: 0}}
	result := inventory.Calculate(in, inventory.Policy{NegativeInventoryHandling: inventory.NegativeInventoryAllowWithWarning})
	r := result.StockPolicyResults[0]
	if !r.BelowMinimum.Available || !r.BelowMinimum.Value {
		t.Errorf("BelowMinimum = %+v, want Available=true Value=true (below an explicit zero minimum)", r.BelowMinimum)
	}
}

// TestStockPolicy_WithinRangeNoFlags verifies an item within
// minimum/maximum produces no over/understock flags.
func TestStockPolicy_WithinRangeNoFlags(t *testing.T) {
	in := baseStockPolicyInput(t, 15)
	in.StockPolicies = []inventory.StockPolicy{{ItemID: "ITEM-1", MinimumQuantitySet: true, MinimumQuantity: 5, MaximumQuantitySet: true, MaximumQuantity: 25}}
	result := inventory.Calculate(in, inventory.Policy{})
	r := result.StockPolicyResults[0]
	if r.BelowMinimum.Available && r.BelowMinimum.Value {
		t.Errorf("expected BelowMinimum=false, got %+v", r.BelowMinimum)
	}
	if r.AboveMaximum.Available && r.AboveMaximum.Value {
		t.Errorf("expected AboveMaximum=false, got %+v", r.AboveMaximum)
	}
	if hasFlagCode(result.Flags, inventory.FlagAboveCallerMaximum) || hasFlagCode(result.Flags, inventory.FlagBelowCallerMinimum) {
		t.Errorf("expected no stock-level flags for an in-range quantity, got %+v", result.Flags)
	}
}
