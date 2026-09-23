package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestAging_ExactBucketBoundaries verifies each default bucket boundary
// (0-30, 31-60, 61-90, 91-180, 181-365, 365+) resolves an item at exactly
// that boundary age into the correct bucket.
func TestAging_ExactBucketBoundaries(t *testing.T) {
	asOf := mustDate(t, "2025-12-31")
	cases := []struct {
		ageDays  int
		wantCode string
	}{
		{0, "0_30"},
		{30, "0_30"},
		{31, "31_60"},
		{60, "31_60"},
		{61, "61_90"},
		{90, "61_90"},
		{91, "91_180"},
		{180, "91_180"},
		{181, "181_365"},
		{365, "181_365"},
		{366, "365_PLUS"},
		{1000, "365_PLUS"},
	}
	for _, c := range cases {
		receivedDate := asOf.AddDate(0, 0, -c.ageDays)
		in := inventory.Input{
			AsOfDate: "2025-12-31",
			Items:    []inventory.Item{item("ITEM-1", "Widgets")},
			Snapshots: []inventory.InventorySnapshot{
				{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: asOf, ReceivedDate: &receivedDate,
					QuantityOnHand: inventory.AvailableQty(10, "EA"), UnitCost: inventory.AvailableValue(1), Currency: "USD"},
			},
		}
		result := inventory.Calculate(in, inventory.Policy{})
		if !result.Aging.Available || len(result.Aging.Rows) != 1 {
			t.Fatalf("ageDays=%d: expected 1 aging row, got %+v", c.ageDays, result.Aging)
		}
		got := result.Aging.Rows[0].BucketCode
		if got != c.wantCode {
			t.Errorf("ageDays=%d: BucketCode = %q, want %q", c.ageDays, got, c.wantCode)
		}
	}
}

// TestAging_CustomBuckets verifies task section 18: a caller-defined
// bucket schema is honored instead of the default.
func TestAging_CustomBuckets(t *testing.T) {
	asOf := mustDate(t, "2025-12-31")
	receivedDate := asOf.AddDate(0, 0, -45)
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: asOf, ReceivedDate: &receivedDate,
				QuantityOnHand: inventory.AvailableQty(10, "EA"), UnitCost: inventory.AvailableValue(1), Currency: "USD"},
		},
	}
	policy := inventory.Policy{Buckets: []inventory.BucketDefinition{
		{Code: "FRESH", MinDays: 0, MaxDays: 60, HasMax: true},
		{Code: "STALE", MinDays: 61, HasMax: false},
	}}
	result := inventory.Calculate(in, policy)
	if !result.Aging.Available || len(result.Aging.Rows) != 1 {
		t.Fatalf("expected 1 aging row, got %+v", result.Aging)
	}
	if result.Aging.Rows[0].BucketCode != "FRESH" {
		t.Errorf("BucketCode = %q, want FRESH", result.Aging.Rows[0].BucketCode)
	}
}

// TestAging_UnknownAge verifies task sections 17/50: an item with no
// ReceivedDate, no receipt movement, and no movement history at all is
// UNKNOWN — never assigned to the youngest or oldest bucket.
func TestAging_UnknownAge(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-12-31"),
				QuantityOnHand: inventory.AvailableQty(10, "EA"), UnitCost: inventory.AvailableValue(5), Currency: "USD"},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.Aging.Rows) != 1 {
		t.Fatalf("expected 1 aging row, got %+v", result.Aging.Rows)
	}
	row := result.Aging.Rows[0]
	if row.Evidence != inventory.AgeEvidenceUnknown {
		t.Errorf("Evidence = %q, want UNKNOWN", row.Evidence)
	}
	if row.BucketCode != inventory.UnknownAgeBucketCode {
		t.Errorf("BucketCode = %q, want %q", row.BucketCode, inventory.UnknownAgeBucketCode)
	}
	if result.Aging.UnknownAgeValue != 50 {
		t.Errorf("UnknownAgeValue = %v, want 50", result.Aging.UnknownAgeValue)
	}
	if !result.Aging.UnknownAgePercent.Available || result.Aging.UnknownAgePercent.Amount != 1 {
		t.Errorf("UnknownAgePercent = %+v, want Available=true Amount=1", result.Aging.UnknownAgePercent)
	}
	if !hasFlagCode(result.Flags, inventory.FlagUnknownInventoryAge) {
		t.Errorf("expected FlagUnknownInventoryAge, got %+v", result.Flags)
	}
}

// TestAging_LastReceiptDateFallback verifies task section 17's fallback
// order: no lot ReceivedDate but a PURCHASE_RECEIPT movement exists.
func TestAging_LastReceiptDateFallback(t *testing.T) {
	asOf := mustDate(t, "2025-12-31")
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", asOf, 10, 5),
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", asOf.AddDate(0, 0, -40), inventory.MovementPurchaseReceipt, 10, 5),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.Aging.Rows) != 1 {
		t.Fatalf("expected 1 aging row, got %+v", result.Aging.Rows)
	}
	row := result.Aging.Rows[0]
	if row.Evidence != inventory.AgeEvidenceLastReceiptDate {
		t.Errorf("Evidence = %q, want LAST_RECEIPT_DATE", row.Evidence)
	}
	if row.BucketCode != "31_60" {
		t.Errorf("BucketCode = %q, want 31_60 (40 days)", row.BucketCode)
	}
}

// TestAging_SlowMoving verifies task section 19: items with no outbound
// movement for >= SlowMovingDays are classified slow-moving.
func TestAging_SlowMoving(t *testing.T) {
	asOf := mustDate(t, "2025-12-31")
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets"), item("ITEM-2", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", asOf, 10, 5), // no outbound at all -> uses last-movement fallback path (none), so unavailable for slow/non-moving.
			snapshot("SNAP-2", "ITEM-2", asOf, 10, 5),
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", asOf.AddDate(0, 0, -100), inventory.MovementCustomerShipment, 5, 5),
			movement("MV-2", "ITEM-2", asOf.AddDate(0, 0, -10), inventory.MovementCustomerShipment, 5, 5),
		},
	}
	policy := inventory.Policy{SlowMovingDays: 90}
	result := inventory.Calculate(in, policy)
	if !result.Aging.SlowMovingValue.Available {
		t.Fatalf("expected SlowMovingValue.Available=true, got %+v", result.Aging)
	}
	if result.Aging.SlowMovingValue.Amount != 50 {
		t.Errorf("SlowMovingValue = %v, want 50 (only ITEM-1 qualifies)", result.Aging.SlowMovingValue.Amount)
	}
	if !hasFlagCode(result.Flags, inventory.FlagSlowMovingInventory) {
		t.Errorf("expected FlagSlowMovingInventory, got %+v", result.Flags)
	}
}

// TestAging_NonMoving verifies task section 20: NonMovingDays >=
// SlowMovingDays produces a separate, stricter classification.
func TestAging_NonMoving(t *testing.T) {
	asOf := mustDate(t, "2025-12-31")
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", asOf, 10, 5),
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", asOf.AddDate(0, 0, -200), inventory.MovementCustomerShipment, 5, 5),
		},
	}
	policy := inventory.Policy{SlowMovingDays: 60, NonMovingDays: 180}
	result := inventory.Calculate(in, policy)
	if !result.Aging.SlowMovingValue.Available || result.Aging.SlowMovingValue.Amount != 50 {
		t.Errorf("SlowMovingValue = %+v, want Available=true Amount=50", result.Aging.SlowMovingValue)
	}
	if !result.Aging.NonMovingValue.Available || result.Aging.NonMovingValue.Amount != 50 {
		t.Errorf("NonMovingValue = %+v, want Available=true Amount=50", result.Aging.NonMovingValue)
	}
	if !hasFlagCode(result.Flags, inventory.FlagNonMovingInventory) {
		t.Errorf("expected FlagNonMovingInventory, got %+v", result.Flags)
	}
}

// TestAging_NoDefaultObsolescencePeriod verifies task section 19: without
// SlowMovingDays configured, this package never invents a default —
// SlowMovingValue/NonMovingValue remain Unavailable.
func TestAging_NoDefaultObsolescencePeriod(t *testing.T) {
	asOf := mustDate(t, "2025-12-31")
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", asOf, 10, 5),
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", asOf.AddDate(0, 0, -900), inventory.MovementCustomerShipment, 5, 5),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if result.Aging.SlowMovingValue.Available {
		t.Errorf("expected SlowMovingValue.Available=false with no policy configured, got %+v", result.Aging.SlowMovingValue)
	}
	if result.Aging.NonMovingValue.Available {
		t.Errorf("expected NonMovingValue.Available=false with no policy configured, got %+v", result.Aging.NonMovingValue)
	}
}

// TestAging_LastMovement verifies task section 23: last-receipt/outbound/
// any-movement dates and days-since figures.
func TestAging_LastMovement(t *testing.T) {
	asOf := mustDate(t, "2025-12-31")
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", asOf.AddDate(0, 0, -50), inventory.MovementPurchaseReceipt, 10, 5),
			movement("MV-2", "ITEM-1", asOf.AddDate(0, 0, -20), inventory.MovementCustomerShipment, 4, 5),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.LastMovement) != 1 {
		t.Fatalf("expected 1 last-movement row, got %d", len(result.LastMovement))
	}
	lm := result.LastMovement[0]
	if lm.LastReceiptDate == nil || !lm.LastReceiptDate.Equal(asOf.AddDate(0, 0, -50)) {
		t.Errorf("LastReceiptDate = %v, want %v", lm.LastReceiptDate, asOf.AddDate(0, 0, -50))
	}
	if lm.LastOutboundDate == nil || !lm.LastOutboundDate.Equal(asOf.AddDate(0, 0, -20)) {
		t.Errorf("LastOutboundDate = %v, want %v", lm.LastOutboundDate, asOf.AddDate(0, 0, -20))
	}
	if !lm.DaysSinceOutbound.Available || lm.DaysSinceOutbound.Amount != 20 {
		t.Errorf("DaysSinceOutbound = %+v, want Available=true Amount=20", lm.DaysSinceOutbound)
	}
	if !lm.DaysSinceAnyMovement.Available || lm.DaysSinceAnyMovement.Amount != 20 {
		t.Errorf("DaysSinceAnyMovement = %+v, want Available=true Amount=20", lm.DaysSinceAnyMovement)
	}
}
