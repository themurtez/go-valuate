package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestReconcile_ExactGLTie verifies task section 37: subledger exactly
// matches the supplied GL control balance.
func TestReconcile_ExactGLTie(t *testing.T) {
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), 100, 5)},
		GLControls: []inventory.GLControl{
			{AsOfDate: "2025-06-30", Balance: 500},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Reconciliation.Available {
		t.Fatalf("expected Reconciliation.Available=true, got %+v", result.Reconciliation)
	}
	if !result.Reconciliation.AllReconciled {
		t.Errorf("expected AllReconciled=true, got %+v", result.Reconciliation)
	}
	if hasFlagCode(result.Flags, inventory.FlagInventoryGLMismatch) {
		t.Errorf("expected no GL mismatch flag, got %+v", result.Flags)
	}
}

// TestReconcile_WithinTolerance verifies a small difference within
// AbsoluteTolerance still reconciles.
func TestReconcile_WithinTolerance(t *testing.T) {
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), 100, 5)},
		GLControls: []inventory.GLControl{
			{AsOfDate: "2025-06-30", Balance: 505}, // $5 off.
		},
	}
	policy := inventory.Policy{ReconciliationTolerance: inventory.ReconciliationTolerance{AbsoluteTolerance: 10}}
	result := inventory.Calculate(in, policy)
	if !result.Reconciliation.AllReconciled {
		t.Errorf("expected AllReconciled=true within tolerance, got %+v", result.Reconciliation)
	}
}

// TestReconcile_Mismatch verifies a material difference triggers
// FlagInventoryGLMismatch and AllReconciled=false, without adjusting
// either side.
func TestReconcile_Mismatch(t *testing.T) {
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), 100, 5)},
		GLControls: []inventory.GLControl{
			{AsOfDate: "2025-06-30", Balance: 700},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if result.Reconciliation.AllReconciled {
		t.Errorf("expected AllReconciled=false, got %+v", result.Reconciliation)
	}
	if !hasFlagCode(result.Flags, inventory.FlagInventoryGLMismatch) {
		t.Errorf("expected FlagInventoryGLMismatch, got %+v", result.Flags)
	}
	comp := result.Reconciliation.Components[0]
	if comp.SubledgerInventoryValue != 500 {
		t.Errorf("SubledgerInventoryValue = %v, want 500 (never adjusted)", comp.SubledgerInventoryValue)
	}
	if comp.GLInventoryBalance != 700 {
		t.Errorf("GLInventoryBalance = %v, want 700 (never adjusted)", comp.GLInventoryBalance)
	}
}

// TestReconcile_MultiComponent verifies task section 38: caller-defined
// components (e.g. matching InventoryClass) reconcile independently.
func TestReconcile_MultiComponent(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items: []inventory.Item{
			{ID: "RM-1", Category: "Materials", Class: inventory.ClassRawMaterial, Active: true, Currency: "USD", UnitOfMeasure: "EA"},
			{ID: "FG-1", Category: "Products", Class: inventory.ClassFinishedGood, Active: true, Currency: "USD", UnitOfMeasure: "EA"},
		},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "RM-1", mustDate(t, "2025-06-30"), 100, 2), // $200
			snapshot("SNAP-2", "FG-1", mustDate(t, "2025-06-30"), 50, 10), // $500
		},
		GLControls: []inventory.GLControl{
			{Component: "RAW_MATERIAL", Balance: 200},
			{Component: "FINISHED_GOOD", Balance: 500},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Reconciliation.AllReconciled {
		t.Errorf("expected AllReconciled=true, got %+v", result.Reconciliation)
	}
	if len(result.Reconciliation.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(result.Reconciliation.Components))
	}
}

// TestReconcile_QuantityRollforward verifies task section 53: an exact
// Beginning + Inbound - Outbound +/- Adjustments = Ending tie.
func TestReconcile_QuantityRollforward(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Periods: []inventory.PeriodInfo{
			{Period: "2025-01", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31")},
		},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-BEGIN", "ITEM-1", mustDate(t, "2024-12-31"), 100, 5),
			snapshot("SNAP-END", "ITEM-1", mustDate(t, "2025-01-31"), 130, 5),
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementPurchaseReceipt, 50, 5),
			movement("MV-2", "ITEM-1", mustDate(t, "2025-01-20"), inventory.MovementCustomerShipment, 20, 5),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.Periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(result.Periods))
	}
	roll := result.Periods[0].QuantityRollforward
	if !roll.Available {
		t.Fatalf("expected QuantityRollforward.Available=true, got %+v", roll)
	}
	// 100 + 50 - 20 = 130.
	if !roll.Reconciled {
		t.Errorf("expected Reconciled=true, got %+v", roll)
	}
	if roll.ExpectedEnding.Amount != 130 {
		t.Errorf("ExpectedEnding = %v, want 130", roll.ExpectedEnding.Amount)
	}
}

// TestReconcile_QuantityRollforwardMismatch verifies a genuine mismatch
// is reported, not silently forced to reconcile.
func TestReconcile_QuantityRollforwardMismatch(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Periods: []inventory.PeriodInfo{
			{Period: "2025-01", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31")},
		},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-BEGIN", "ITEM-1", mustDate(t, "2024-12-31"), 100, 5),
			snapshot("SNAP-END", "ITEM-1", mustDate(t, "2025-01-31"), 200, 5), // does not tie to movements below.
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementPurchaseReceipt, 50, 5),
			movement("MV-2", "ITEM-1", mustDate(t, "2025-01-20"), inventory.MovementCustomerShipment, 20, 5),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	roll := result.Periods[0].QuantityRollforward
	if !roll.Available {
		t.Fatalf("expected QuantityRollforward.Available=true, got %+v", roll)
	}
	if roll.Reconciled {
		t.Errorf("expected Reconciled=false for a genuine mismatch, got %+v", roll)
	}
	if roll.Difference.Amount != 70 { // 200 actual - 130 expected.
		t.Errorf("Difference = %v, want 70", roll.Difference.Amount)
	}
}

// TestReconcile_ValueRollforward verifies task section 54: an exact
// value-side tie when movement values are all compatible.
func TestReconcile_ValueRollforward(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Periods: []inventory.PeriodInfo{
			{Period: "2025-01", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31")},
		},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-BEGIN", "ITEM-1", mustDate(t, "2024-12-31"), 100, 5), // $500
			snapshot("SNAP-END", "ITEM-1", mustDate(t, "2025-01-31"), 130, 5),   // $650
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementPurchaseReceipt, 50, 5),  // +$250
			movement("MV-2", "ITEM-1", mustDate(t, "2025-01-20"), inventory.MovementCustomerShipment, 20, 5), // -$100
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	roll := result.Periods[0].ValueRollforward
	if !roll.Available {
		t.Fatalf("expected ValueRollforward.Available=true, got %+v", roll)
	}
	// 500 + 250 - 100 = 650.
	if !roll.Reconciled {
		t.Errorf("expected Reconciled=true, got %+v", roll)
	}
}

// TestReconcile_ValueRollforwardMismatch verifies a genuine value
// mismatch is reported with explicit evidence.
func TestReconcile_ValueRollforwardMismatch(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Periods: []inventory.PeriodInfo{
			{Period: "2025-01", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31")},
		},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-BEGIN", "ITEM-1", mustDate(t, "2024-12-31"), 100, 5), // $500
			snapshot("SNAP-END", "ITEM-1", mustDate(t, "2025-01-31"), 130, 8),   // $1040, cost basis shifted -> will not tie.
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementPurchaseReceipt, 50, 5),
			movement("MV-2", "ITEM-1", mustDate(t, "2025-01-20"), inventory.MovementCustomerShipment, 20, 5),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	roll := result.Periods[0].ValueRollforward
	if !roll.Available {
		t.Fatalf("expected ValueRollforward.Available=true, got %+v", roll)
	}
	if roll.Reconciled {
		t.Errorf("expected Reconciled=false, got %+v", roll)
	}
}

// TestReconcile_NotForcedWhenIncomplete verifies task section 53: the
// rollforward is not attempted (Available=false) when necessary movement
// data is missing (mixed/unknown UOM breaks the tie).
func TestReconcile_NotForcedWhenIncomplete(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Periods: []inventory.PeriodInfo{
			{Period: "2025-01", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31")},
		},
		// No beginning snapshot supplied at all.
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-END", "ITEM-1", mustDate(t, "2025-01-31"), 130, 5),
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementPurchaseReceipt, 50, 5),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	roll := result.Periods[0].QuantityRollforward
	if roll.Available {
		t.Errorf("expected QuantityRollforward.Available=false without a beginning balance, got %+v", roll)
	}
}
