package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestSmoke_BasicSnapshotValue is a fast end-to-end sanity check exercised
// first, before the full test suite — one item, one snapshot, quantity x
// unit cost value resolution, portfolio totals.
func TestSmoke_BasicSnapshotValue(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items: []inventory.Item{
			{ID: "ITEM-1", Name: "Widget", Category: "Widgets", Active: true, Currency: "USD", UnitOfMeasure: "EA"},
		},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-06-30"),
				QuantityOnHand: inventory.AvailableQty(100, "EA"),
				UnitCost:       inventory.AvailableValue(10),
			},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	if inventory.HasErrors(result.Issues) {
		t.Fatalf("unexpected error issues: %+v", result.Issues)
	}
	if result.Portfolio.TotalInventoryValue != 1000 {
		t.Errorf("TotalInventoryValue = %v, want 1000", result.Portfolio.TotalInventoryValue)
	}
	if result.Portfolio.ItemsWithPositiveStock != 1 {
		t.Errorf("ItemsWithPositiveStock = %d, want 1", result.Portfolio.ItemsWithPositiveStock)
	}
	if len(result.Portfolio.TopItemsByValue) != 1 || result.Portfolio.TopItemsByValue[0].Value != 1000 {
		t.Errorf("TopItemsByValue = %+v, want one item valued 1000", result.Portfolio.TopItemsByValue)
	}
}

// TestSmoke_PeriodTurnoverDIO exercises the summary-input path (task
// section 2B): PeriodFinancials-only, no items/snapshots at all.
func TestSmoke_PeriodTurnoverDIO(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Periods: []inventory.PeriodInfo{
			{Period: "2025", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-12-31")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025",
				BeginningInventoryValue: inventory.AvailableValue(100000),
				EndingInventoryValue:    inventory.AvailableValue(120000),
				COGS:                    inventory.AvailableValue(880000),
			},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	if len(result.Periods) != 1 {
		t.Fatalf("expected 1 period summary, got %d", len(result.Periods))
	}
	ps := result.Periods[0]
	if !ps.Turnover.Available {
		t.Fatalf("expected Turnover.Available, got %+v", ps.Turnover)
	}
	wantAvg := (100000.0 + 120000.0) / 2
	wantTurnover := 880000.0 / wantAvg
	if absDiff(ps.Turnover.Value, wantTurnover) > 0.0001 {
		t.Errorf("Turnover = %v, want %v", ps.Turnover.Value, wantTurnover)
	}
	if !ps.DIO.Available {
		t.Fatalf("expected DIO.Available, got %+v", ps.DIO)
	}
	wantDIO := (wantAvg / 880000.0) * 365
	if absDiff(ps.DIO.Value, wantDIO) > 0.01 {
		t.Errorf("DIO = %v, want %v", ps.DIO.Value, wantDIO)
	}
}

// TestSmoke_ZeroCOGSUnavailableNotInf verifies task section 14's explicit
// "DIO unavailable, not Inf" rule for zero COGS. Turnover, by contrast, is
// a legitimate COGS/AverageInventory ratio: a known-zero COGS against a
// nonzero average inventory correctly yields Turnover=0 (Available=true)
// — zero turnover is a real economic fact distinct from "unavailable,"
// unlike DIO's division BY COGS, which is genuinely undefined at COGS=0.
func TestSmoke_ZeroCOGSUnavailableNotInf(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Periods: []inventory.PeriodInfo{
			{Period: "2025", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-12-31")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025",
				BeginningInventoryValue: inventory.AvailableValue(100000),
				EndingInventoryValue:    inventory.AvailableValue(100000),
				COGS:                    inventory.AvailableValue(0),
			},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Available || len(result.Periods) != 1 {
		t.Fatalf("expected 1 available period, got %+v", result)
	}
	dio := result.Periods[0].DIO
	if dio.Available {
		t.Fatalf("expected DIO.Available=false for zero COGS, got %+v", dio)
	}
	if isInfOrNaN(dio.Value) {
		t.Errorf("DIO.Value must not be Inf/NaN even when unavailable, got %v", dio.Value)
	}
	turnover := result.Periods[0].Turnover
	if !turnover.Available {
		t.Fatalf("expected Turnover.Available=true (zero COGS / nonzero average = 0, a real ratio), got %+v", turnover)
	}
	if turnover.Value != 0 {
		t.Errorf("Turnover.Value = %v, want 0", turnover.Value)
	}
}

// TestSmoke_TurnoverUnavailableWhenAverageZero verifies the genuinely
// undefined case for Turnover: a zero AverageInventory denominator.
func TestSmoke_TurnoverUnavailableWhenAverageZero(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Periods: []inventory.PeriodInfo{
			{Period: "2025", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-12-31")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025",
				BeginningInventoryValue: inventory.AvailableValue(0),
				EndingInventoryValue:    inventory.AvailableValue(0),
				COGS:                    inventory.AvailableValue(500000),
			},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !result.Available || len(result.Periods) != 1 {
		t.Fatalf("expected 1 available period, got %+v", result)
	}
	turnover := result.Periods[0].Turnover
	if turnover.Available {
		t.Fatalf("expected Turnover.Available=false for zero average inventory, got %+v", turnover)
	}
	if isInfOrNaN(turnover.Value) {
		t.Errorf("Turnover.Value must not be Inf/NaN even when unavailable, got %v", turnover.Value)
	}
}
