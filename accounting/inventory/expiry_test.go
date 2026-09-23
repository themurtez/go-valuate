package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

func expirySnapshot(t *testing.T, id, itemID string, asOf, expiry string, qty, unitCost float64) inventory.InventorySnapshot {
	t.Helper()
	exp := mustDate(t, expiry)
	return inventory.InventorySnapshot{
		ID: id, ItemID: itemID, AsOfDate: mustDate(t, asOf), ExpiryDate: &exp,
		QuantityOnHand: inventory.AvailableQty(qty, "EA"), UnitCost: inventory.AvailableValue(unitCost), Currency: "USD",
	}
}

// TestExpiry_Valid verifies an expiry date far in the future is OK, no
// flags.
func TestExpiry_Valid(t *testing.T) {
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Perishables")},
		Snapshots: []inventory.InventorySnapshot{expirySnapshot(t, "SNAP-1", "ITEM-1", "2025-06-30", "2026-06-30", 10, 5)},
	}
	result := inventory.Calculate(in, inventory.Policy{ExpiryWarningDays: 30})
	if len(result.ExpirySummary.Rows) != 1 {
		t.Fatalf("expected 1 expiry row, got %+v", result.ExpirySummary.Rows)
	}
	if result.ExpirySummary.Rows[0].Status != inventory.ExpiryStatusOK {
		t.Errorf("Status = %q, want OK", result.ExpirySummary.Rows[0].Status)
	}
	if hasFlagCode(result.Flags, inventory.FlagExpiringInventory) || hasFlagCode(result.Flags, inventory.FlagExpiredInventoryReview) {
		t.Errorf("expected no expiry flags, got %+v", result.Flags)
	}
}

// TestExpiry_NearingExpiry verifies task section 48: within the warning
// window triggers FlagExpiringInventory.
func TestExpiry_NearingExpiry(t *testing.T) {
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Perishables")},
		Snapshots: []inventory.InventorySnapshot{expirySnapshot(t, "SNAP-1", "ITEM-1", "2025-06-30", "2025-07-10", 10, 5)},
	}
	result := inventory.Calculate(in, inventory.Policy{ExpiryWarningDays: 30})
	if result.ExpirySummary.Rows[0].Status != inventory.ExpiryStatusExpiring {
		t.Errorf("Status = %q, want EXPIRING", result.ExpirySummary.Rows[0].Status)
	}
	if !hasFlagCode(result.Flags, inventory.FlagExpiringInventory) {
		t.Errorf("expected FlagExpiringInventory, got %+v", result.Flags)
	}
	if !result.ExpirySummary.ExpiringValue.Available || result.ExpirySummary.ExpiringValue.Amount != 50 {
		t.Errorf("ExpiringValue = %+v, want Available=true Amount=50", result.ExpirySummary.ExpiringValue)
	}
}

// TestExpiry_Expired verifies task section 48: a lot already expired as
// of AsOfDate is a finding, not invalid input.
func TestExpiry_Expired(t *testing.T) {
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Perishables")},
		Snapshots: []inventory.InventorySnapshot{expirySnapshot(t, "SNAP-1", "ITEM-1", "2025-06-30", "2025-06-01", 10, 5)},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Available {
		t.Fatalf("expected Available=true (expired inventory is a finding, not invalid input), issues: %+v", result.Issues)
	}
	if result.ExpirySummary.Rows[0].Status != inventory.ExpiryStatusExpired {
		t.Errorf("Status = %q, want EXPIRED", result.ExpirySummary.Rows[0].Status)
	}
	if !hasFlagCode(result.Flags, inventory.FlagExpiredInventoryReview) {
		t.Errorf("expected FlagExpiredInventoryReview, got %+v", result.Flags)
	}
	if !result.ExpirySummary.ExpiredValue.Available || result.ExpirySummary.ExpiredValue.Amount != 50 {
		t.Errorf("ExpiredValue = %+v, want Available=true Amount=50", result.ExpirySummary.ExpiredValue)
	}
}

// TestExpiry_InvalidExpiryBeforeReceipt verifies task section 65: an
// ExpiryDate before ReceivedDate IS invalid input.
func TestExpiry_InvalidExpiryBeforeReceipt(t *testing.T) {
	received := mustDate(t, "2025-06-01")
	expiry := mustDate(t, "2025-05-01") // before received.
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Perishables")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-06-30"), ReceivedDate: &received, ExpiryDate: &expiry,
				QuantityOnHand: inventory.AvailableQty(10, "EA"), UnitCost: inventory.AvailableValue(5), Currency: "USD"},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueInvalidExpiryDate) {
		t.Errorf("expected IssueInvalidExpiryDate, got %+v", result.Issues)
	}
}

// TestExpiry_NoAutomaticWriteOff verifies task section 48: expired
// inventory is never automatically written off — the snapshot's own
// value is preserved unchanged in the portfolio total.
func TestExpiry_NoAutomaticWriteOff(t *testing.T) {
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Perishables")},
		Snapshots: []inventory.InventorySnapshot{expirySnapshot(t, "SNAP-1", "ITEM-1", "2025-06-30", "2025-06-01", 10, 5)},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if result.Portfolio.TotalInventoryValue != 50 {
		t.Errorf("TotalInventoryValue = %v, want 50 (expired value never auto-written-off)", result.Portfolio.TotalInventoryValue)
	}
}
