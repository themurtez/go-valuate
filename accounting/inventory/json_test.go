package inventory_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

func roundTrip[T any](t *testing.T, v T) T {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "NaN") || strings.Contains(string(data), "Inf") {
		t.Fatalf("JSON output contains NaN/Inf: %s", data)
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return out
}

func TestJSON_Item(t *testing.T) {
	it := inventory.Item{
		ID: "ITEM-1", SKU: "SKU-1", Name: "Widget", Category: "Widgets", Subcategory: "Blue",
		Class: inventory.ClassMerchandise, UnitOfMeasure: "EA", Active: true, Currency: "USD", Location: "WH1",
		SourceRef: inventory.SourceRef{System: "erp", ID: "abc"},
	}
	got := roundTrip(t, it)
	if got != it {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, it)
	}
}

func TestJSON_InventorySnapshot(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	received := mustDate(t, "2025-05-01")
	expiry := mustDate(t, "2026-01-01")
	s := inventory.InventorySnapshot{
		ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: asOf,
		QuantityOnHand: inventory.AvailableQty(100, "EA"), UnitCost: inventory.AvailableValue(5), InventoryValue: inventory.AvailableValue(500),
		Location: "WH1", LotID: "LOT-1", ReceivedDate: &received, ExpiryDate: &expiry, Currency: "USD",
		SourceRef: inventory.SourceRef{System: "erp", ID: "s1"},
	}
	got := roundTrip(t, s)
	if got.ID != s.ID || got.QuantityOnHand != s.QuantityOnHand || !got.AsOfDate.Equal(s.AsOfDate) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, s)
	}
	if got.ReceivedDate == nil || !got.ReceivedDate.Equal(*s.ReceivedDate) {
		t.Errorf("ReceivedDate round-trip mismatch: got %v, want %v", got.ReceivedDate, s.ReceivedDate)
	}
}

func TestJSON_Movement(t *testing.T) {
	m := inventory.Movement{
		ID: "MV-1", ItemID: "ITEM-1", Date: mustDate(t, "2025-06-01"), Type: inventory.MovementPurchaseReceipt,
		Quantity: inventory.AvailableQty(50, "EA"), UnitCost: inventory.AvailableValue(4), Amount: inventory.AvailableValue(200),
		Location: "WH1", ReferenceID: "PO-1", ReasonCode: "TEST", Currency: "USD",
		SourceRef: inventory.SourceRef{System: "erp", ID: "m1"},
	}
	got := roundTrip(t, m)
	if got.ID != m.ID || got.Type != m.Type || got.Quantity != m.Quantity {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, m)
	}
}

func TestJSON_StockPolicy(t *testing.T) {
	p := inventory.StockPolicy{
		ItemID: "ITEM-1", MinimumQuantity: 10, MinimumQuantitySet: true,
		TargetQuantity: 50, TargetQuantitySet: true, MaximumQuantity: 100, MaximumQuantitySet: true,
		ReorderPoint: 20, ReorderPointSet: true,
	}
	got := roundTrip(t, p)
	if got != p {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, p)
	}
}

func TestJSON_AgingSummary(t *testing.T) {
	in := inventory.Input{
		AsOfDate:  "2025-06-30",
		Items:     []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), 10, 5)},
	}
	result := inventory.Calculate(in, inventory.Policy{SlowMovingDays: 30})
	got := roundTrip(t, result.Aging)
	if got.Available != result.Aging.Available || got.TotalValue != result.Aging.TotalValue {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, result.Aging)
	}
}

func TestJSON_Reconciliation(t *testing.T) {
	in := inventory.Input{
		AsOfDate:   "2025-06-30",
		Items:      []inventory.Item{item("ITEM-1", "Widgets")},
		Snapshots:  []inventory.InventorySnapshot{snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), 10, 5)},
		GLControls: []inventory.GLControl{{Balance: 50}},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	got := roundTrip(t, result.Reconciliation)
	if got.Available != result.Reconciliation.Available || got.AllReconciled != result.Reconciliation.AllReconciled {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, result.Reconciliation)
	}
}

func TestJSON_PeriodSummary(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-10"), inventory.MovementPurchaseReceipt, 100, 2),
	})
	result := inventory.Calculate(in, inventory.Policy{})
	if len(result.Periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(result.Periods))
	}
	got := roundTrip(t, result.Periods[0])
	if got.Period.Period != result.Periods[0].Period.Period {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, result.Periods[0])
	}
}

func TestJSON_FullResult(t *testing.T) {
	in := safetySampleInput(t)
	result := inventory.Calculate(in, inventory.Policy{SlowMovingDays: 30, NonMovingDays: 90})
	got := roundTrip(t, result)
	if got.SchemaVersion != result.SchemaVersion || got.Available != result.Available {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, result)
	}
	if len(got.Flags) != len(result.Flags) {
		t.Errorf("Flags round-trip mismatch: got %d, want %d", len(got.Flags), len(result.Flags))
	}
	if len(got.Issues) != len(result.Issues) {
		t.Errorf("Issues round-trip mismatch: got %d, want %d", len(got.Issues), len(result.Issues))
	}
}

// TestJSON_NoNaNInfEverInOutput sweeps a scenario deliberately designed
// to hit every "unavailable, not Inf" code path (zero COGS, zero average
// inventory, zero recent outbound velocity) and confirms none of it
// leaks NaN/Inf into the serialized JSON.
func TestJSON_NoNaNInfEverInOutput(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-01-31",
		Items:    []inventory.Item{item("ITEM-1", "Widgets")},
		Periods: []inventory.PeriodInfo{
			{Period: "2025-01", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025-01", BeginningInventoryValue: inventory.AvailableValue(0), EndingInventoryValue: inventory.AvailableValue(0), COGS: inventory.AvailableValue(0)},
		},
		VelocityWindows: map[string]inventory.VelocityWindow{
			"ITEM-1": {StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-01-31")},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "NaN") || strings.Contains(string(data), "Inf") {
		t.Fatalf("JSON output contains NaN/Inf: %s", data)
	}
}
