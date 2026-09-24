package vendorspend_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/inventory"
	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

// TestInventoryAdapter_UnavailableWithoutReceipts verifies task section
// 26's "if current inventory data lacks supplier identity, adapter must
// be unavailable" rule: with an empty receipts map (no caller-supplied
// SupplierID attribution at all), the adapter reports ok == false.
func TestInventoryAdapter_UnavailableWithoutReceipts(t *testing.T) {
	movements := []inventory.Movement{
		{ID: "MV-1", ItemID: "ITEM-1", Date: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC), Type: inventory.MovementPurchaseReceipt,
			Quantity: inventory.AvailableQty(100, "EA"), Currency: "USD"},
	}
	records, ok := vendorspend.SpendRecordsFromInventoryReceipts(movements, nil)
	if ok {
		t.Errorf("expected ok == false with no receipts supplied")
	}
	if records != nil {
		t.Errorf("expected nil records, got %+v", records)
	}
}

// TestInventoryAdapter_AvailableWithExplicitAttribution verifies a
// Movement WITH a caller-supplied InventoryPurchaseReceipt converts
// successfully.
func TestInventoryAdapter_AvailableWithExplicitAttribution(t *testing.T) {
	movements := []inventory.Movement{
		{ID: "MV-1", ItemID: "ITEM-1", Date: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC), Type: inventory.MovementPurchaseReceipt,
			Quantity: inventory.AvailableQty(100, "EA"), Currency: "USD"},
		{ID: "MV-2", ItemID: "ITEM-2", Date: time.Date(2025, 3, 2, 0, 0, 0, 0, time.UTC), Type: inventory.MovementPurchaseReceipt,
			Quantity: inventory.AvailableQty(50, "EA"), Currency: "USD"},
	}
	receipts := map[string]vendorspend.InventoryPurchaseReceipt{
		"MV-1": {MovementID: "MV-1", SupplierID: "SUP-INV", Amount: 5000, Period: "2025-03"},
		// MV-2 intentionally has no receipt entry — must be skipped, not
		// guessed.
	}
	records, ok := vendorspend.SpendRecordsFromInventoryReceipts(movements, receipts)
	if !ok {
		t.Fatalf("expected ok == true with at least one attributed movement")
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly 1 converted record (only MV-1 had attribution), got %d: %+v", len(records), records)
	}
	if records[0].SupplierID != "SUP-INV" || records[0].Amount != 5000 {
		t.Errorf("record = %+v, want SupplierID=SUP-INV Amount=5000", records[0])
	}
	if records[0].Basis != vendorspend.BasisReceipt {
		t.Errorf("Basis = %v, want BasisReceipt", records[0].Basis)
	}
}
