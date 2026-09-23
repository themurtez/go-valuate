package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// TestPurchaseVsUsage_Balanced verifies no flag fires when purchases and
// usage are roughly in line.
func TestPurchaseVsUsage_Balanced(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-05"), inventory.MovementPurchaseReceipt, 100, 5),
		movement("MV-2", "ITEM-1", mustDate(t, "2025-01-20"), inventory.MovementCustomerShipment, 95, 5),
	})
	result := inventory.Calculate(in, inventory.Policy{})
	ps := result.Periods[0]
	if !ps.PurchaseVsUsage.Available {
		t.Fatalf("expected PurchaseVsUsage.Available=true, got %+v", ps.PurchaseVsUsage)
	}
	if hasFlagCode(result.Flags, inventory.FlagPurchasesOutpaceUsage) {
		t.Errorf("expected no FlagPurchasesOutpaceUsage for balanced purchases/usage, got %+v", result.Flags)
	}
}

// TestPurchaseVsUsage_PurchasesOutpaceUsage verifies task section 32/57's
// flag fires when purchases substantially exceed usage value.
func TestPurchaseVsUsage_PurchasesOutpaceUsage(t *testing.T) {
	in := periodInput(t, []inventory.Movement{
		movement("MV-1", "ITEM-1", mustDate(t, "2025-01-05"), inventory.MovementPurchaseReceipt, 200, 5), // $1000
		movement("MV-2", "ITEM-1", mustDate(t, "2025-01-20"), inventory.MovementCustomerShipment, 50, 5), // $250
	})
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasFlagCode(result.Flags, inventory.FlagPurchasesOutpaceUsage) {
		t.Errorf("expected FlagPurchasesOutpaceUsage, got %+v", result.Flags)
	}
}

// TestPurchaseVsUsage_InventoryBuildWithFlatCOGS verifies task section 33:
// ending inventory materially increases while COGS is flat/declining.
func TestPurchaseVsUsage_InventoryBuildWithFlatCOGS(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Periods: []inventory.PeriodInfo{
			{Period: "2025-Q1", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-03-31")},
			{Period: "2025-Q2", StartDate: mustDate(t, "2025-04-01"), EndDate: mustDate(t, "2025-06-30")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025-Q1", BeginningInventoryValue: inventory.AvailableValue(100000), EndingInventoryValue: inventory.AvailableValue(100000), COGS: inventory.AvailableValue(300000)},
			{Period: "2025-Q2", BeginningInventoryValue: inventory.AvailableValue(100000), EndingInventoryValue: inventory.AvailableValue(180000), COGS: inventory.AvailableValue(295000)},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasFlagCode(result.Flags, inventory.FlagInventoryBuildWithoutMatchingCOGSGrowth) {
		t.Errorf("expected FlagInventoryBuildWithoutMatchingCOGSGrowth, got %+v", result.Flags)
	}
}

// TestPurchaseVsUsage_NoFlagWithoutCOGSData verifies task section 33: the
// build-without-matching-growth signal requires compatible COGS/sales
// data and never fires without it.
func TestPurchaseVsUsage_NoFlagWithoutCOGSData(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-12-31",
		Periods: []inventory.PeriodInfo{
			{Period: "2025-Q1", StartDate: mustDate(t, "2025-01-01"), EndDate: mustDate(t, "2025-03-31")},
			{Period: "2025-Q2", StartDate: mustDate(t, "2025-04-01"), EndDate: mustDate(t, "2025-06-30")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025-Q1", BeginningInventoryValue: inventory.AvailableValue(100000), EndingInventoryValue: inventory.AvailableValue(100000)},
			{Period: "2025-Q2", BeginningInventoryValue: inventory.AvailableValue(100000), EndingInventoryValue: inventory.AvailableValue(300000)},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if hasFlagCode(result.Flags, inventory.FlagInventoryBuildWithoutMatchingCOGSGrowth) {
		t.Errorf("expected no build flag without COGS data, got %+v", result.Flags)
	}
}
