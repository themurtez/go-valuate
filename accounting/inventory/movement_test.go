package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

func periodInput(t *testing.T, movements []inventory.Movement) inventory.Input {
	return inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Periods: []inventory.PeriodInfo{
			{Period: "2025-01", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31")},
		},
		Movements: movements,
	}
}

// TestMovement_Purchase verifies a PURCHASE_RECEIPT contributes to
// PurchaseSummary, not UsageSummary.
func TestMovement_Purchase(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementPurchaseReceipt, 100, 2),
	})
	result := inventory.Calculate(in, inventory.Policy{})
	ps := result.Periods[0]
	if !ps.Purchases.Available || ps.Purchases.PurchaseValue.Amount != 200 {
		t.Errorf("Purchases = %+v, want PurchaseValue=200", ps.Purchases)
	}
	if ps.Purchases.ReceiptCount != 1 {
		t.Errorf("ReceiptCount = %d, want 1", ps.Purchases.ReceiptCount)
	}
	if ps.Usage.Available {
		t.Errorf("expected Usage.Available=false for a purchase-only period, got %+v", ps.Usage)
	}
}

// TestMovement_Shipment verifies a CUSTOMER_SHIPMENT contributes to
// UsageSummary.CustomerShipmentValue.
func TestMovement_Shipment(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementCustomerShipment, 50, 3),
	})
	result := inventory.Calculate(in, inventory.Policy{})
	ps := result.Periods[0]
	if !ps.Usage.Available || ps.Usage.OutboundValue.Amount != 150 {
		t.Errorf("Usage = %+v, want OutboundValue=150", ps.Usage)
	}
	if !ps.Usage.CustomerShipmentValue.Available || ps.Usage.CustomerShipmentValue.Amount != 150 {
		t.Errorf("CustomerShipmentValue = %+v, want Available=true Amount=150", ps.Usage.CustomerShipmentValue)
	}
}

// TestMovement_Transfer verifies TRANSFER_OUT/TRANSFER_IN are tracked
// separately and TRANSFER_OUT counts as outbound usage.
func TestMovement_Transfer(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementTransferOut, 20, 5),
		movement("MV-2", "ITEM-1", mustDate(t, "2025-01-11"), inventory.MovementTransferIn, 20, 5),
	})
	result := inventory.Calculate(in, inventory.Policy{})
	ps := result.Periods[0]
	if !ps.Usage.TransferOutValue.Available || ps.Usage.TransferOutValue.Amount != 100 {
		t.Errorf("TransferOutValue = %+v, want Available=true Amount=100", ps.Usage.TransferOutValue)
	}
}

// TestMovement_Return verifies RETURN_IN/RETURN_OUT.
func TestMovement_Return(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementReturnOut, 5, 4),
	})
	result := inventory.Calculate(in, inventory.Policy{})
	ps := result.Periods[0]
	if !ps.Usage.ReturnOutValue.Available || ps.Usage.ReturnOutValue.Amount != 20 {
		t.Errorf("ReturnOutValue = %+v, want Available=true Amount=20", ps.Usage.ReturnOutValue)
	}
}

// TestMovement_Adjustment verifies ADJUSTMENT_INCREASE/DECREASE feed
// AdjustmentSummary, never PurchaseSummary/UsageSummary.
func TestMovement_Adjustment(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementAdjustmentIncrease, 10, 2),
		movement("MV-2", "ITEM-1", mustDate(t, "2025-01-11"), inventory.MovementAdjustmentDecrease, 4, 2),
	})
	result := inventory.Calculate(in, inventory.Policy{})
	ps := result.Periods[0]
	if !ps.Adjustments.Available {
		t.Fatalf("expected Adjustments.Available=true, got %+v", ps.Adjustments)
	}
	if ps.Adjustments.AdjustmentIncrease.Amount != 20 {
		t.Errorf("AdjustmentIncrease = %v, want 20", ps.Adjustments.AdjustmentIncrease.Amount)
	}
	if ps.Adjustments.AdjustmentDecrease.Amount != -8 {
		t.Errorf("AdjustmentDecrease = %v, want -8", ps.Adjustments.AdjustmentDecrease.Amount)
	}
	if ps.Adjustments.NetAdjustment.Amount != 12 {
		t.Errorf("NetAdjustment = %v, want 12", ps.Adjustments.NetAdjustment.Amount)
	}
	if ps.Purchases.Available {
		t.Errorf("expected Purchases.Available=false for adjustment-only movements, got %+v", ps.Purchases)
	}
	if ps.Usage.Available {
		t.Errorf("expected Usage.Available=false for adjustment-only movements, got %+v", ps.Usage)
	}
}

// TestMovement_WriteOff verifies WRITE_OFF reduces NetAdjustment and is
// reported separately from AdjustmentIncrease/Decrease and from ordinary
// UsageSummary.
func TestMovement_WriteOff(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementWriteOff, 5, 10),
	})
	result := inventory.Calculate(in, inventory.Policy{})
	ps := result.Periods[0]
	if !ps.Adjustments.WriteOffValue.Available || ps.Adjustments.WriteOffValue.Amount != 50 {
		t.Errorf("WriteOffValue = %+v, want Available=true Amount=50", ps.Adjustments.WriteOffValue)
	}
	if ps.Adjustments.NetAdjustment.Amount != -50 {
		t.Errorf("NetAdjustment = %v, want -50", ps.Adjustments.NetAdjustment.Amount)
	}
	if ps.Usage.Available && ps.Usage.OutboundValue.Available {
		t.Errorf("expected write-off never folded into OutboundValue, got %+v", ps.Usage)
	}
}

// TestMovement_DuplicateLikeMovement verifies task section 36: two
// movements sharing item/date/type/quantity/amount/reference are flagged
// as possible duplicates, without any intent/fraud claim.
func TestMovement_DuplicateLikeMovement(t *testing.T) {
	date := mustDate(t, "2025-01-10")
	in := periodInput(t, []inventory.Movement{
		{ID: "MV-1", ItemID: "ITEM-1", Date: date, Type: inventory.MovementCustomerShipment,
			Quantity: inventory.AvailableQty(10, "EA"), Amount: inventory.AvailableValue(100), ReferenceID: "SO-1"},
		{ID: "MV-2", ItemID: "ITEM-1", Date: date, Type: inventory.MovementCustomerShipment,
			Quantity: inventory.AvailableQty(10, "EA"), Amount: inventory.AvailableValue(100), ReferenceID: "SO-1"},
	})
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.PossibleDuplicateMovements) != 1 {
		t.Fatalf("expected 1 possible duplicate pair, got %+v", result.PossibleDuplicateMovements)
	}
	if !hasFlagCode(result.Flags, inventory.FlagPossibleDuplicateMovement) {
		t.Errorf("expected FlagPossibleDuplicateMovement, got %+v", result.Flags)
	}
}

// TestMovement_ExactDuplicateID verifies duplicate movement IDs are
// structurally rejected (an Issue, not a Flag).
func TestMovement_ExactDuplicateID(t *testing.T) {
	date := mustDate(t, "2025-01-10")
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", date, inventory.MovementCustomerShipment, 10, 5),
		movement("MV-1", "ITEM-1", date, inventory.MovementCustomerShipment, 20, 5), // duplicate ID.
	})
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueDuplicateMovement) {
		t.Errorf("expected IssueDuplicateMovement, got %+v", result.Issues)
	}
	// Only the first occurrence is used.
	if result.Periods[0].Usage.OutboundQuantity.Amount != 10 {
		t.Errorf("OutboundQuantity = %v, want 10 (only first occurrence)", result.Periods[0].Usage.OutboundQuantity.Amount)
	}
}
