package fixtures

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/inventory"
	inventoryfixtures "github.com/themurtez/go-valuate/accounting/inventory/fixtures"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

// TestInventoryAdapter_ExactMatch uses inventory/fixtures'
// GLReconciliationExact (a real sibling fixture) end to end.
func TestInventoryAdapter_ExactMatch(t *testing.T) {
	items, snapshots, controls := inventoryfixtures.GLReconciliationExact()
	result := inventory.Calculate(inventory.Input{
		AsOfDate:   inventoryfixtures.AsOfDate,
		Items:      items,
		Snapshots:  snapshots,
		GLControls: controls,
	}, inventory.Policy{})
	if !result.Available || !result.Reconciliation.Available {
		t.Fatalf("expected inventory.Result available with reconciliation, got %+v", result)
	}
	if !result.Reconciliation.AllReconciled {
		t.Fatalf("expected inventory's own reconciliation to already tie, got %+v", result.Reconciliation)
	}

	book := reconciliation.InventorySubledgerBookBalance(result, "", inventoryfixtures.AsOfDate)
	external := reconciliation.InventoryControlExternalBalance(result, "", inventoryfixtures.AsOfDate)

	r := reconciliation.Calculate(reconciliation.Input{
		AccountID:       "INV-SUBLEDGER",
		AsOfDate:        inventoryfixtures.AsOfDate,
		Type:            reconciliation.TypeInventoryControl,
		BookBalance:     book,
		ExternalBalance: external,
		Policy:          reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	})
	if r.Status != reconciliation.StatusReconciled {
		t.Fatalf("expected RECONCILED, got %s (equation=%+v)", r.Status, r.Equation)
	}
}

// TestInventoryAdapter_MismatchCase uses inventory/fixtures'
// GLReconciliationMismatch.
func TestInventoryAdapter_MismatchCase(t *testing.T) {
	items, snapshots, controls := inventoryfixtures.GLReconciliationMismatch()
	result := inventory.Calculate(inventory.Input{
		AsOfDate:   inventoryfixtures.AsOfDate,
		Items:      items,
		Snapshots:  snapshots,
		GLControls: controls,
	}, inventory.Policy{})
	if !result.Reconciliation.Available || result.Reconciliation.AllReconciled {
		t.Fatalf("expected inventory's own reconciliation to show a genuine mismatch, got %+v", result.Reconciliation)
	}

	book := reconciliation.InventorySubledgerBookBalance(result, "", inventoryfixtures.AsOfDate)
	external := reconciliation.InventoryControlExternalBalance(result, "", inventoryfixtures.AsOfDate)

	r := reconciliation.Calculate(reconciliation.Input{
		AccountID:       "INV-SUBLEDGER",
		AsOfDate:        inventoryfixtures.AsOfDate,
		Type:            reconciliation.TypeInventoryControl,
		BookBalance:     book,
		ExternalBalance: external,
		Policy:          reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	})
	if r.Status != reconciliation.StatusUnreconciled {
		t.Fatalf("expected UNRECONCILED, got %s", r.Status)
	}
}
