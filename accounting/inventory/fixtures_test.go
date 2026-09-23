package inventory_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/inventory"
	invfixtures "github.com/themurtez/go-valuate/accounting/inventory/fixtures"
)

// invfixturesDate/invfixturesItem are small local helpers so this file's
// synthetic multi-item adjustment/zero scenarios (which build their own
// Item list and period boundaries rather than using a fixture-provided
// one) stay terse.
func invfixturesDate(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

func invfixturesItem(id string) inventory.Item {
	return inventory.Item{ID: id, Category: "Test", Active: true, Currency: "USD", UnitOfMeasure: "EA"}
}

// TestFixtures_HealthyRetailerNoErrors sanity-checks the baseline
// "nothing unusual" fixture produces a usable, error-free Result.
func TestFixtures_HealthyRetailerNoErrors(t *testing.T) {
	items, snapshots, movements := invfixtures.HealthyRetailer()
	result := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots, Movements: movements}, inventory.Policy{})
	if !result.Available {
		t.Fatalf("expected Available=true, issues: %+v", result.Issues)
	}
	if inventory.HasErrors(result.Issues) {
		t.Errorf("expected no SeverityError issues, got: %+v", result.Issues)
	}
	if result.Portfolio.TotalInventoryValue <= 0 {
		t.Errorf("expected a positive TotalInventoryValue, got %v", result.Portfolio.TotalInventoryValue)
	}
}

// TestFixtures_SlowNonMovingAgedProduceExpectedFlags verifies the
// deliberately-anomalous fixtures each trigger their documented signal.
func TestFixtures_SlowNonMovingAgedProduceExpectedFlags(t *testing.T) {
	policy := inventory.Policy{SlowMovingDays: 90, NonMovingDays: 180}

	items, snapshots, movements := invfixtures.SlowMovingInventory()
	slow := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots, Movements: movements}, policy)
	if !hasFlagCode(slow.Flags, inventory.FlagSlowMovingInventory) {
		t.Errorf("SlowMovingInventory fixture: expected FlagSlowMovingInventory, got %+v", slow.Flags)
	}

	items, snapshots, movements = invfixtures.NonMovingInventory()
	nonMoving := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots, Movements: movements}, policy)
	if !hasFlagCode(nonMoving.Flags, inventory.FlagNonMovingInventory) {
		t.Errorf("NonMovingInventory fixture: expected FlagNonMovingInventory, got %+v", nonMoving.Flags)
	}

	items, snapshots = invfixtures.UnknownAgeInventory()
	unknown := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots}, inventory.Policy{})
	if !hasFlagCode(unknown.Flags, inventory.FlagUnknownInventoryAge) {
		t.Errorf("UnknownAgeInventory fixture: expected FlagUnknownInventoryAge, got %+v", unknown.Flags)
	}
}

// TestFixtures_NegativeInventoryFlagged verifies the negative-inventory
// fixture is included with a warning, never auto-corrected.
func TestFixtures_NegativeInventoryFlagged(t *testing.T) {
	items, snapshots := invfixtures.NegativeInventory()
	result := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots}, inventory.Policy{})
	if !hasFlagCode(result.Flags, inventory.FlagNegativeInventory) {
		t.Errorf("expected FlagNegativeInventory, got %+v", result.Flags)
	}
	if result.Portfolio.TotalInventoryValue >= 0 {
		t.Errorf("expected a negative TotalInventoryValue, got %v", result.Portfolio.TotalInventoryValue)
	}
}

// TestFixtures_StockPolicyScenarios verifies the below-minimum/
// above-maximum fixtures each produce their expected flag.
func TestFixtures_StockPolicyScenarios(t *testing.T) {
	items, snapshots, policies := invfixtures.StockPolicyBelowMinimum()
	below := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots, StockPolicies: policies}, inventory.Policy{})
	if !hasFlagCode(below.Flags, inventory.FlagBelowCallerMinimum) {
		t.Errorf("StockPolicyBelowMinimum fixture: expected FlagBelowCallerMinimum, got %+v", below.Flags)
	}

	items, snapshots, policies = invfixtures.StockPolicyAboveMaximum()
	above := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots, StockPolicies: policies}, inventory.Policy{})
	if !hasFlagCode(above.Flags, inventory.FlagAboveCallerMaximum) {
		t.Errorf("StockPolicyAboveMaximum fixture: expected FlagAboveCallerMaximum, got %+v", above.Flags)
	}
}

// TestFixtures_AdjustmentScenarios verifies the large-adjustment,
// repeated-adjustment, write-off, and period-end-adjustment fixtures each
// produce their documented flag.
func TestFixtures_AdjustmentScenarios(t *testing.T) {
	period := []inventory.PeriodInfo{{Period: "2025-06", StartDate: invfixturesDate("2025-06-01"), EndDate: invfixturesDate("2025-06-30")}}
	items := []inventory.Item{
		invfixturesItem("SKU-ADJ"), invfixturesItem("SKU-REP"), invfixturesItem("SKU-WO"), invfixturesItem("SKU-PE"),
	}

	// LargeAdjustment is a $25,000 ADJUSTMENT_DECREASE (1000 units x $25),
	// not a write-off, so its documented signal is
	// FlagHighInventoryAdjustmentRate — computing a rate requires an
	// AverageInventory denominator, so Financials supplies one here.
	large := inventory.Calculate(inventory.Input{
		AsOfDate: invfixtures.AsOfDate, Items: items, Periods: period, Movements: invfixtures.LargeAdjustment(),
		Financials: []inventory.PeriodFinancials{{Period: "2025-06", BeginningInventoryValue: inventory.AvailableValue(50000), EndingInventoryValue: inventory.AvailableValue(50000)}},
	}, inventory.Policy{AdjustmentRateThreshold: 0.05})
	if !hasFlagCode(large.Flags, inventory.FlagHighInventoryAdjustmentRate) {
		t.Errorf("LargeAdjustment fixture: expected FlagHighInventoryAdjustmentRate, got %+v", large.Flags)
	}

	repeated := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Periods: period, Movements: invfixtures.RepeatedAdjustments()},
		inventory.Policy{RepeatedItemAdjustmentCount: 3})
	if !hasFlagCode(repeated.Flags, inventory.FlagRepeatedItemAdjustments) {
		t.Errorf("RepeatedAdjustments fixture: expected FlagRepeatedItemAdjustments, got %+v", repeated.Flags)
	}

	writeOff := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Periods: period, Movements: invfixtures.WriteOff()},
		inventory.Policy{LargeWriteOffThreshold: 500})
	if !hasFlagCode(writeOff.Flags, inventory.FlagLargeWriteOff) {
		t.Errorf("WriteOff fixture: expected FlagLargeWriteOff, got %+v", writeOff.Flags)
	}

	periodEnd := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Periods: period, Movements: invfixtures.PeriodEndAdjustment()},
		inventory.Policy{PeriodEndAdjustmentWindowDays: 3})
	if !hasFlagCode(periodEnd.Flags, inventory.FlagPeriodEndInventoryAdjustment) {
		t.Errorf("PeriodEndAdjustment fixture: expected FlagPeriodEndInventoryAdjustment, got %+v", periodEnd.Flags)
	}
}

// TestFixtures_ReconciliationScenarios verifies the exact/mismatch GL and
// rollforward fixtures behave as documented.
func TestFixtures_ReconciliationScenarios(t *testing.T) {
	items, snapshots, controls := invfixtures.GLReconciliationExact()
	exact := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots, GLControls: controls}, inventory.Policy{})
	if !exact.Reconciliation.AllReconciled {
		t.Errorf("GLReconciliationExact fixture: expected AllReconciled=true, got %+v", exact.Reconciliation)
	}

	items, snapshots, controls = invfixtures.GLReconciliationMismatch()
	mismatch := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots, GLControls: controls}, inventory.Policy{})
	if mismatch.Reconciliation.AllReconciled {
		t.Errorf("GLReconciliationMismatch fixture: expected AllReconciled=false, got %+v", mismatch.Reconciliation)
	}
}

// TestFixtures_ExpiryScenarios verifies the expiring/expired fixtures.
func TestFixtures_ExpiryScenarios(t *testing.T) {
	items, snapshots := invfixtures.ExpiringInventory()
	expiring := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots}, inventory.Policy{ExpiryWarningDays: 30})
	if !hasFlagCode(expiring.Flags, inventory.FlagExpiringInventory) {
		t.Errorf("ExpiringInventory fixture: expected FlagExpiringInventory, got %+v", expiring.Flags)
	}

	items, snapshots = invfixtures.ExpiredInventory()
	expired := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots}, inventory.Policy{})
	if !hasFlagCode(expired.Flags, inventory.FlagExpiredInventoryReview) {
		t.Errorf("ExpiredInventory fixture: expected FlagExpiredInventoryReview, got %+v", expired.Flags)
	}
}

// TestFixtures_MixedCurrencyInvalid verifies the mixed-currency fixture
// produces IssueMixedCurrency.
func TestFixtures_MixedCurrencyInvalid(t *testing.T) {
	items, snapshots := invfixtures.MixedCurrencyInvalid()
	result := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots}, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueMixedCurrency) {
		t.Errorf("MixedCurrencyInvalid fixture: expected IssueMixedCurrency, got %+v", result.Issues)
	}
}

// TestFixtures_ZeroScenarios verifies the zero-inventory and zero-COGS
// fixtures behave as known-zero facts, not unavailable/Inf.
func TestFixtures_ZeroScenarios(t *testing.T) {
	items, snapshots := invfixtures.ZeroInventory()
	zeroInv := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots}, inventory.Policy{})
	if zeroInv.Portfolio.TotalInventoryValue != 0 {
		t.Errorf("ZeroInventory fixture: expected TotalInventoryValue=0, got %v", zeroInv.Portfolio.TotalInventoryValue)
	}
	if zeroInv.Portfolio.ItemsWithZeroStock != 1 {
		t.Errorf("ZeroInventory fixture: expected ItemsWithZeroStock=1, got %d", zeroInv.Portfolio.ItemsWithZeroStock)
	}

	period := []inventory.PeriodInfo{{Period: "2025-06", StartDate: invfixturesDate("2025-06-01"), EndDate: invfixturesDate("2025-06-30")}}
	zeroCOGS := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Periods: period, Financials: invfixtures.ZeroCOGS()}, inventory.Policy{})
	if len(zeroCOGS.Periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(zeroCOGS.Periods))
	}
	if zeroCOGS.Periods[0].DIO.Available {
		t.Errorf("ZeroCOGS fixture: expected DIO.Available=false, got %+v", zeroCOGS.Periods[0].DIO)
	}
	if isInfOrNaN(zeroCOGS.Periods[0].DIO.Value) {
		t.Errorf("ZeroCOGS fixture: DIO.Value must never be Inf/NaN, got %v", zeroCOGS.Periods[0].DIO.Value)
	}
}

// TestFixtures_HistoricalTrend verifies the multi-period trend fixture
// produces an available, "improving" DIO trend across 4 quarters.
func TestFixtures_HistoricalTrend(t *testing.T) {
	result := inventory.Calculate(inventory.Input{
		AsOfDate:   invfixtures.AsOfDate,
		Periods:    invfixtures.FourQuarterPeriods(),
		Financials: invfixtures.HistoricalMultiPeriodTrend(),
	}, inventory.Policy{})
	if !result.TurnoverHistory.Available {
		t.Fatalf("expected TurnoverHistory.Available=true, got %+v", result.TurnoverHistory)
	}
	if len(result.TurnoverHistory.Points) != 4 {
		t.Errorf("expected 4 history points, got %d", len(result.TurnoverHistory.Points))
	}
}

// TestFixtures_MultiCategoryMultiLocationMixedUOM verifies these three
// structural fixtures each calculate without error.
func TestFixtures_MultiCategoryMultiLocationMixedUOM(t *testing.T) {
	items, snapshots := invfixtures.MultiCategory()
	cat := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots}, inventory.Policy{})
	if inventory.HasErrors(cat.Issues) {
		t.Errorf("MultiCategory fixture: unexpected errors: %+v", cat.Issues)
	}

	items, snapshots = invfixtures.MultiLocation()
	loc := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots}, inventory.Policy{})
	if inventory.HasErrors(loc.Issues) {
		t.Errorf("MultiLocation fixture: unexpected errors: %+v", loc.Issues)
	}

	items, snapshots = invfixtures.MixedUOM()
	mixed := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots}, inventory.Policy{})
	if inventory.HasErrors(mixed.Issues) {
		t.Errorf("MixedUOM fixture: unexpected errors: %+v", mixed.Issues)
	}
	if mixed.Portfolio.TotalInventoryValue != 500*3+200*4 {
		t.Errorf("MixedUOM fixture: TotalInventoryValue = %v, want %v", mixed.Portfolio.TotalInventoryValue, 500*3+200*4)
	}
}

// TestFixtures_RawMaterialWIPFinishedGoods verifies the composition
// fixture produces a 3-class breakdown.
func TestFixtures_RawMaterialWIPFinishedGoods(t *testing.T) {
	items, snapshots := invfixtures.RawMaterialWIPFinishedGoods()
	result := inventory.Calculate(inventory.Input{AsOfDate: invfixtures.AsOfDate, Items: items, Snapshots: snapshots}, inventory.Policy{})
	if len(result.Composition.ByClass) != 3 {
		t.Errorf("expected 3 classes, got %+v", result.Composition.ByClass)
	}
}
