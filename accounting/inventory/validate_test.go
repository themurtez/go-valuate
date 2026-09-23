package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestValidate_DuplicateItem verifies task section 64: duplicate item
// IDs are flagged, first occurrence wins.
func TestValidate_DuplicateItem(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items: []inventory.Item{
			{ID: "ITEM-1", Category: "A", Currency: "USD"},
			{ID: "ITEM-1", Category: "B", Currency: "USD"},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueDuplicateItem) {
		t.Errorf("expected IssueDuplicateItem, got %+v", result.Issues)
	}
}

// TestValidate_DuplicateSnapshotIdentity verifies task section 52: two
// snapshots for the same item/location/lot/as-of-date is a conflict, not
// an arbitrary pick.
func TestValidate_DuplicateSnapshotIdentity(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: asOf, Location: "WH1", LotID: "LOT-1",
				QuantityOnHand: inventory.AvailableQty(100, "EA"), UnitCost: inventory.AvailableValue(5), Currency: "USD"},
			{ID: "SNAP-2", ItemID: "ITEM-1", AsOfDate: asOf, Location: "WH1", LotID: "LOT-1", // same identity, different ID.
				QuantityOnHand: inventory.AvailableQty(200, "EA"), UnitCost: inventory.AvailableValue(5), Currency: "USD"},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueDuplicateSnapshot) {
		t.Errorf("expected IssueDuplicateSnapshot, got %+v", result.Issues)
	}
}

// TestValidate_SameItemDifferentLotsNoConflict verifies distinct lots at
// the same location/date do NOT conflict.
func TestValidate_SameItemDifferentLotsNoConflict(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: asOf, Location: "WH1", LotID: "LOT-1",
				QuantityOnHand: inventory.AvailableQty(100, "EA"), UnitCost: inventory.AvailableValue(5), Currency: "USD"},
			{ID: "SNAP-2", ItemID: "ITEM-1", AsOfDate: asOf, Location: "WH1", LotID: "LOT-2",
				QuantityOnHand: inventory.AvailableQty(50, "EA"), UnitCost: inventory.AvailableValue(5), Currency: "USD"},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if hasIssueCode(result.Issues, inventory.IssueDuplicateSnapshot) {
		t.Errorf("expected no duplicate-snapshot issue for distinct lots, got %+v", result.Issues)
	}
	if result.Portfolio.TotalInventoryValue != 750 {
		t.Errorf("TotalInventoryValue = %v, want 750 (both lots counted)", result.Portfolio.TotalInventoryValue)
	}
}

// TestValidate_UnknownItemReference verifies task section 64.
func TestValidate_UnknownItemReference(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-UNKNOWN", mustDate(t, "2025-06-30"), 10, 5),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueUnknownItem) {
		t.Errorf("expected IssueUnknownItem, got %+v", result.Issues)
	}
}

// TestValidate_InvalidMovementType verifies an unrecognized MovementType
// is rejected.
func TestValidate_InvalidMovementType(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Movements: []inventory.Movement{
			{ID: "MV-1", ItemID: "ITEM-1", Date: mustDate(t, "2025-06-01"), Type: "NOT_A_REAL_TYPE",
				Quantity: inventory.AvailableQty(10, "EA")},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueInvalidMovement) {
		t.Errorf("expected IssueInvalidMovement, got %+v", result.Issues)
	}
}

// TestValidate_NegativeMovementQuantityRejected verifies task section 5:
// Movement.Quantity is always a non-negative magnitude.
func TestValidate_NegativeMovementQuantityRejected(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Movements: []inventory.Movement{
			{ID: "MV-1", ItemID: "ITEM-1", Date: mustDate(t, "2025-06-01"), Type: inventory.MovementCustomerShipment,
				Quantity: inventory.AvailableQty(-5, "EA")},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueInvalidQuantity) {
		t.Errorf("expected IssueInvalidQuantity, got %+v", result.Issues)
	}
}

// TestValidate_InvalidStockPolicyMinExceedsMax verifies task section 64.
func TestValidate_InvalidStockPolicyMinExceedsMax(t *testing.T) {
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), 10, 5)},
		StockPolicies: []inventory.StockPolicy{
			{ItemID: "ITEM-1", MinimumQuantitySet: true, MinimumQuantity: 100, MaximumQuantitySet: true, MaximumQuantity: 10},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueInvalidStockPolicy) {
		t.Errorf("expected IssueInvalidStockPolicy, got %+v", result.Issues)
	}
}

// TestValidate_InvalidPolicyThresholds verifies task section 59/64:
// invalid Policy configuration (NonMovingDays < SlowMovingDays) is
// flagged.
func TestValidate_InvalidPolicyThresholds(t *testing.T) {
	in := inventory.Input{AsOfDate: "2025-06-30"}
	result := inventory.Calculate(in, inventory.Policy{SlowMovingDays: 90, NonMovingDays: 30})
	if !hasIssueCode(result.Issues, inventory.IssueInvalidPolicy) {
		t.Errorf("expected IssueInvalidPolicy, got %+v", result.Issues)
	}
}

// TestValidate_MissingAsOfDate verifies the top-level required-input
// check.
func TestValidate_MissingAsOfDate(t *testing.T) {
	in := inventory.Input{Items: []inventory.Item{item("ITEM-1", "Widgets")}}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueInvalidAsOfDate) {
		t.Errorf("expected IssueInvalidAsOfDate, got %+v", result.Issues)
	}
	if result.Available {
		t.Errorf("expected Available=false without AsOfDate or Periods")
	}
}

// TestValidate_MissingAsOfDateButPeriodsStillWork verifies the summary
// path can still produce period output even without a valid AsOfDate,
// since AsOfDate only gates snapshot-level analysis.
func TestValidate_MissingAsOfDateButPeriodsStillWork(t *testing.T) {
	in := inventory.Input{
		Periods: []inventory.PeriodInfo{
			{Period: "2025", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-12-31")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025", BeginningInventoryValue: inventory.AvailableValue(1000), EndingInventoryValue: inventory.AvailableValue(2000), COGS: inventory.AvailableValue(5000)},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Available {
		t.Fatalf("expected Available=true (period data alone is sufficient), issues: %+v", result.Issues)
	}
	if len(result.Periods) != 1 {
		t.Errorf("expected 1 period, got %d", len(result.Periods))
	}
}
