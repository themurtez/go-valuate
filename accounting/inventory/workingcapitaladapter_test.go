package inventory_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
)

// TestAdapter_InventoryValueFeedsWorkingCapitalComponent shows inventory
// -> working-capital component reconciliation (task sections 40, 75):
// this package's own PortfolioSummary.TotalInventoryValue, when fed into
// a financial.FinancialDataset as CodeBsInventory, contributes exactly
// that amount to analytics/workingcapital's OperatingCurrentAssets under
// DefaultInclusionPolicy (which includes inventory by default). A test
// adapter, not a hard package dependency — accounting/inventory never
// imports analytics/workingcapital in its own source. Mirrors
// accounting/ar's and accounting/ap's identical adapter-test pattern for
// their own working-capital components.
func TestAdapter_InventoryValueFeedsWorkingCapitalComponent(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items: []inventory.Item{
			item("ITEM-1", "Widgets"),
			item("ITEM-2", "Gadgets"),
		},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", asOf, 100, 5),
			snapshot("SNAP-2", "ITEM-2", asOf, 50, 20),
		},
	}
	invResult := inventory.Calculate(in, inventory.Policy{})
	if !invResult.Available {
		t.Fatalf("expected inventory.Result.Available=true, issues: %+v", invResult.Issues)
	}
	// 100*5 + 50*20 = 1500.
	if invResult.Portfolio.TotalInventoryValue != 1500 {
		t.Fatalf("expected TotalInventoryValue=1500, got %v", invResult.Portfolio.TotalInventoryValue)
	}

	const period financial.Period = "2025"
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeBsInventory, Period: period, Amount: invResult.Portfolio.TotalInventoryValue},
		},
	}
	wcResult := workingcapital.Calculate(workingcapital.Input{Dataset: ds}, workingcapital.Options{})
	if !wcResult.Available || len(wcResult.History) != 1 {
		t.Fatalf("expected workingcapital.Result.Available with 1 period, got %+v", wcResult)
	}

	assets := wcResult.History[0].OperatingCurrentAssets
	if !assets.Available {
		t.Fatalf("expected OperatingCurrentAssets.Available=true")
	}
	if assets.Value != invResult.Portfolio.TotalInventoryValue {
		t.Errorf("OperatingCurrentAssets (%v) does not match inventory total (%v)", assets.Value, invResult.Portfolio.TotalInventoryValue)
	}
}
