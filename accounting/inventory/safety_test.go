package inventory_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

func safetySampleInput(t *testing.T) inventory.Input {
	t.Helper()
	asOf := mustDate(t, "2025-06-30")
	received := asOf.AddDate(0, 0, -45)
	return inventory.Input{
		AsOfDate: "2025-06-30",
		Items: []inventory.Item{
			{ID: "ITEM-1", SKU: "SKU-1", Name: "Widget", Category: "Widgets", Class: inventory.ClassMerchandise, Active: true, Currency: "USD", UnitOfMeasure: "EA", Location: "WH1"},
			{ID: "ITEM-2", SKU: "SKU-2", Name: "Gadget", Category: "Gadgets", Class: inventory.ClassFinishedGood, Active: true, Currency: "USD", UnitOfMeasure: "EA", Location: "WH2"},
		},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: asOf, Location: "WH1", ReceivedDate: &received,
				QuantityOnHand: inventory.AvailableQty(100, "EA"), UnitCost: inventory.AvailableValue(5), Currency: "USD"},
			{ID: "SNAP-2", ItemID: "ITEM-2", AsOfDate: asOf, Location: "WH2",
				QuantityOnHand: inventory.AvailableQty(30, "EA"), UnitCost: inventory.AvailableValue(20), Currency: "USD"},
		},
		Movements: []inventory.Movement{
			movement("MV-1", "ITEM-1", asOf.AddDate(0, 0, -45), inventory.MovementPurchaseReceipt, 100, 5),
			movement("MV-2", "ITEM-1", asOf.AddDate(0, 0, -10), inventory.MovementCustomerShipment, 20, 5),
			movement("MV-3", "ITEM-2", asOf.AddDate(0, 0, -5), inventory.MovementAdjustmentDecrease, 2, 20),
		},
		StockPolicies: []inventory.StockPolicy{
			{ItemID: "ITEM-1", MinimumQuantitySet: true, MinimumQuantity: 10, MaximumQuantitySet: true, MaximumQuantity: 200},
		},
		Periods: []inventory.PeriodInfo{
			{Period: "2025-06", StartDate: mustDate(t, "2025-06-01"), EndDate: mustDate(t, "2025-06-30")},
		},
		Financials: []inventory.PeriodFinancials{
			{Period: "2025-06", BeginningInventoryValue: inventory.AvailableValue(600), EndingInventoryValue: inventory.AvailableValue(1100), COGS: inventory.AvailableValue(3000)},
		},
		GLControls: []inventory.GLControl{{Period: "2025-06", Balance: 1100}},
	}
}

// TestSafety_NoMutationOfInput verifies task section 71: every
// caller-owned slice/map is byte-for-byte unchanged after Calculate.
func TestSafety_NoMutationOfInput(t *testing.T) {
	in := safetySampleInput(t)

	itemsBefore, _ := json.Marshal(in.Items)
	snapshotsBefore, _ := json.Marshal(in.Snapshots)
	movementsBefore, _ := json.Marshal(in.Movements)
	policiesBefore, _ := json.Marshal(in.StockPolicies)
	periodsBefore, _ := json.Marshal(in.Periods)
	financialsBefore, _ := json.Marshal(in.Financials)
	glBefore, _ := json.Marshal(in.GLControls)

	policy := inventory.Policy{SlowMovingDays: 30, NonMovingDays: 90}
	policyBefore, _ := json.Marshal(policy)

	_ = inventory.Calculate(in, policy)

	itemsAfter, _ := json.Marshal(in.Items)
	snapshotsAfter, _ := json.Marshal(in.Snapshots)
	movementsAfter, _ := json.Marshal(in.Movements)
	policiesAfter, _ := json.Marshal(in.StockPolicies)
	periodsAfter, _ := json.Marshal(in.Periods)
	financialsAfter, _ := json.Marshal(in.Financials)
	glAfter, _ := json.Marshal(in.GLControls)
	policyAfter, _ := json.Marshal(policy)

	checks := []struct {
		name          string
		before, after []byte
	}{
		{"Items", itemsBefore, itemsAfter},
		{"Snapshots", snapshotsBefore, snapshotsAfter},
		{"Movements", movementsBefore, movementsAfter},
		{"StockPolicies", policiesBefore, policiesAfter},
		{"Periods", periodsBefore, periodsAfter},
		{"Financials", financialsBefore, financialsAfter},
		{"GLControls", glBefore, glAfter},
		{"Policy", policyBefore, policyAfter},
	}
	for _, c := range checks {
		if string(c.before) != string(c.after) {
			t.Errorf("%s was mutated:\nbefore: %s\nafter:  %s", c.name, c.before, c.after)
		}
	}
}

// TestSafety_DeterministicRepeatedExecution proves identical input always
// produces byte-for-byte identical JSON output across repeated calls —
// task section 70.
func TestSafety_DeterministicRepeatedExecution(t *testing.T) {
	in := safetySampleInput(t)
	policy := inventory.Policy{SlowMovingDays: 30, NonMovingDays: 90, ExpiryWarningDays: 30}

	first := inventory.Calculate(in, policy)
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	for i := 0; i < 10; i++ {
		result := inventory.Calculate(in, policy)
		resultJSON, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(resultJSON) != string(firstJSON) {
			t.Fatalf("run %d: output diverged from first run", i)
		}
	}
}

// TestSafety_ConcurrentCalls proves Calculate can be called concurrently
// against identical input without data races or inconsistent results (run
// with -race in verification) — task section 78.
func TestSafety_ConcurrentCalls(t *testing.T) {
	in := safetySampleInput(t)
	policy := inventory.Policy{SlowMovingDays: 30}

	const workers = 20
	var wg sync.WaitGroup
	results := make([]inventory.Result, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = inventory.Calculate(in, policy)
		}(i)
	}
	wg.Wait()

	first, err := json.Marshal(results[0])
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for i := 1; i < workers; i++ {
		other, err := json.Marshal(results[i])
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(other) != string(first) {
			t.Errorf("worker %d produced a different result under concurrent execution", i)
		}
	}
}

// TestSafety_NaNInfRejected verifies non-finite amounts/quantities are
// excluded with a structured issue, never propagated into output.
func TestSafety_NaNInfRejected(t *testing.T) {
	nan := mathNaN()
	inf := mathInf()
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items:    []inventory.Item{item("ITEM-1", "Widgets"), item("ITEM-2", "Widgets")},
		Snapshots: []inventory.InventorySnapshot{
			{ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate(t, "2025-06-30"),
				QuantityOnHand: inventory.AvailableQty(nan, "EA"), UnitCost: inventory.AvailableValue(5), Currency: "USD"},
			{ID: "SNAP-2", ItemID: "ITEM-2", AsOfDate: mustDate(t, "2025-06-30"),
				QuantityOnHand: inventory.AvailableQty(10, "EA"), UnitCost: inventory.AvailableValue(inf), Currency: "USD"},
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueNonFiniteQuantity) {
		t.Errorf("expected IssueNonFiniteQuantity, got %+v", result.Issues)
	}
	if isInfOrNaN(result.Portfolio.TotalInventoryValue) {
		t.Errorf("TotalInventoryValue must never be NaN/Inf, got %v", result.Portfolio.TotalInventoryValue)
	}
	// Both rows excluded -> zero value, not NaN-poisoned.
	if result.Portfolio.TotalInventoryValue != 0 {
		t.Errorf("TotalInventoryValue = %v, want 0 (both non-finite rows excluded)", result.Portfolio.TotalInventoryValue)
	}
}

// TestSafety_MixedCurrency verifies task section 61: mixed currencies are
// flagged and only the resolved reporting currency is included in
// aggregates — never silently summed across currencies.
func TestSafety_MixedCurrency(t *testing.T) {
	in := inventory.Input{
		AsOfDate: "2025-06-30",
		Items: []inventory.Item{
			{ID: "ITEM-1", Category: "Widgets", Active: true, Currency: "USD", UnitOfMeasure: "EA"},
			{ID: "ITEM-2", Category: "Widgets", Active: true, Currency: "EUR", UnitOfMeasure: "EA"},
		},
		Snapshots: []inventory.InventorySnapshot{
			snapshot("SNAP-1", "ITEM-1", mustDate(t, "2025-06-30"), 10, 5),
			snapshot("SNAP-2", "ITEM-2", mustDate(t, "2025-06-30"), 10, 5),
		},
	}
	result := inventory.Calculate(in, inventory.Policy{})
	if !hasIssueCode(result.Issues, inventory.IssueMixedCurrency) {
		t.Errorf("expected IssueMixedCurrency, got %+v", result.Issues)
	}
	// Only one currency's item should contribute (USD is more common —
	// tie broken by ascending code when tied; here it's a 1-1 tie, so USD
	// wins alphabetically).
	if result.Portfolio.TotalInventoryValue != 50 {
		t.Errorf("TotalInventoryValue = %v, want 50 (single currency only)", result.Portfolio.TotalInventoryValue)
	}
}

// TestSafety_NeutralLanguageAcrossAllMessages is a broader sweep than
// TestAdjustments_NeutralLanguage: it exercises a richer scenario (many
// flags/issues at once) and checks every single one.
func TestSafety_NeutralLanguageAcrossAllMessages(t *testing.T) {
	in := safetySampleInput(t)
	in.Movements = append(in.Movements, inventory.Movement{
		ID: "MV-SHRINK", ItemID: "ITEM-1", Date: mustDate(t, "2025-06-15"), Type: inventory.MovementWriteOff,
		Quantity: inventory.AvailableQty(50, "EA"), UnitCost: inventory.AvailableValue(5), ReasonCode: "SHRINKAGE",
	})
	policy := inventory.Policy{SlowMovingDays: 1, NonMovingDays: 1, AdjustmentRateThreshold: 0.001, LargeWriteOffThreshold: 1, RepeatedItemAdjustmentCount: 1, PeriodEndAdjustmentWindowDays: 60}
	result := inventory.Calculate(in, policy)
	if len(result.Flags) == 0 {
		t.Fatalf("expected at least one flag for this scenario to make the sweep meaningful")
	}
	forbidden := []string{"shrinkage", "fraud", "theft", "stolen", "steal", "embezzle"}
	checkNoForbiddenLanguage(t, result, forbidden)
}
