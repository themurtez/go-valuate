package fixtures

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

// TestLaborAdapter_PayrollClearingReconciled uses a real
// labor.GLControlFromLedgerBalances-produced GLPayrollControl (not a
// recalculated stand-in) as the book side of a payroll-clearing
// reconciliation that ties exactly against a supplied GL clearing
// balance.
func TestLaborAdapter_PayrollClearingReconciled(t *testing.T) {
	balances := []labor.LedgerAccountBalance{
		{AccountID: "6000", Balance: 40000}, // gross wages
		{AccountID: "6100", Balance: 5000},  // employer taxes
	}
	mapping := []labor.LedgerAccountMapping{
		{AccountID: "6000", Category: labor.LedgerCategoryGrossWages},
		{AccountID: "6100", Category: labor.LedgerCategoryEmployerTaxes},
	}
	control := labor.GLControlFromLedgerBalances("2025-01", balances, mapping)
	if !control.TotalLaborCost.Available || control.TotalLaborCost.Amount != 45000 {
		t.Fatalf("expected TotalLaborCost 45000, got %+v", control.TotalLaborCost)
	}

	book := reconciliation.PayrollClearingBookBalance(control, "2025-02-01")
	external := reconciliation.PayrollClearingExternalBalance(45000, "2025-02-01")

	r := reconciliation.Calculate(reconciliation.Input{
		AccountID:       "PAYROLL-CONTROL",
		AsOfDate:        "2025-02-01",
		Type:            reconciliation.TypePayrollClearing,
		BookBalance:     book,
		ExternalBalance: external,
		Policy:          reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	})
	if r.Status != reconciliation.StatusReconciled {
		t.Fatalf("expected RECONCILED, got %s (equation=%+v)", r.Status, r.Equation)
	}
}

// TestLaborAdapter_UnavailableWithoutMapping confirms
// PayrollClearingBookBalance never fabricates a balance when
// GLControlFromLedgerBalances had nothing to aggregate (no mapping
// entries matched any balance).
func TestLaborAdapter_UnavailableWithoutMapping(t *testing.T) {
	control := labor.GLControlFromLedgerBalances("2025-01", nil, nil)
	bal := reconciliation.PayrollClearingBookBalance(control, "2025-02-01")
	if bal.EndingBalance != nil {
		t.Fatalf("expected no ending balance when TotalLaborCost is unavailable, got %+v", bal)
	}
}
