package fixtures

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

func TestBankInput_ExactMatchesFound(t *testing.T) {
	r := reconciliation.Calculate(BankInput())
	if len(r.MatchedGroups) != 2 {
		t.Fatalf("expected 2 exact matches (owner contribution, payroll), got %d: %+v", len(r.MatchedGroups), r.MatchedGroups)
	}
}

func TestBankInput_OutstandingCheckAndDepositInTransitExplained(t *testing.T) {
	r := reconciliation.Calculate(BankInput())
	// The rent check (SVC-JE-4/L2) and the invoice deposit (SVC-JE-5/L1)
	// are unmatched but each has a ReconcilingItem explaining it - no
	// MATERIAL_UNMATCHED_BOOK_ITEM finding should fire for either.
	for _, f := range r.Findings {
		if f.Code == reconciliation.FindingMaterialUnmatchedBookItem {
			for _, id := range f.ItemIDs {
				if id == "SVC-JE-4/L2" || id == "SVC-JE-5/L1" {
					t.Fatalf("expected explained item %s to not produce a material-unmatched finding, findings=%+v", id, r.Findings)
				}
			}
		}
	}
}

func TestBankInput_RepeatedAmbiguousAmountLeftUnmatched(t *testing.T) {
	r := reconciliation.Calculate(BankInput())
	foundBoth := 0
	for _, id := range r.UnmatchedExternalItems {
		if id == "STMT-4" || id == "STMT-5" {
			foundBoth++
		}
	}
	if foundBoth != 2 {
		t.Fatalf("expected STMT-4 and STMT-5 both unmatched (no book counterpart), got unmatched=%v", r.UnmatchedExternalItems)
	}
}

func TestBankInput_StaleUnmatchedItemFlagged(t *testing.T) {
	r := reconciliation.Calculate(BankInput())
	found := false
	for _, f := range r.Findings {
		if f.Code == reconciliation.FindingStaleUnmatchedItem {
			for _, id := range f.ItemIDs {
				if id == "STMT-6" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected STMT-6 to be flagged stale, findings=%+v", r.Findings)
	}
}

func TestBankInput_UnreconciledWithSmallExplainedResidual(t *testing.T) {
	r := reconciliation.Calculate(BankInput())
	if r.Status != reconciliation.StatusUnreconciled {
		t.Fatalf("expected UNRECONCILED (genuine unexplained residual from STMT-4/5/6), got %s", r.Status)
	}
	if !r.Equation.Available {
		t.Fatalf("expected equation available")
	}
	// The residual should be exactly the sum of the genuinely unexplained
	// external items (STMT-4 25 + STMT-5 25 + STMT-6 12 = 62).
	if r.Equation.Difference != 62 {
		t.Fatalf("expected difference 62, got %v", r.Equation.Difference)
	}
}

func TestCreditCardInput_ConfirmedMatchAppliedUnderReversedOrientation(t *testing.T) {
	r := reconciliation.Calculate(CreditCardInput())
	found := false
	for _, g := range r.MatchedGroups {
		if g.MatchID == "MANUAL-CC-1" {
			found = true
			if g.MatchReason != reconciliation.ReasonConfirmed {
				t.Fatalf("expected CONFIRMED reason, got %s", g.MatchReason)
			}
			if g.Difference != 0 {
				t.Fatalf("expected confirmed match to tie exactly under reversed orientation, got difference=%v", g.Difference)
			}
		}
	}
	if !found {
		t.Fatalf("expected the confirmed manual match to be applied, matched_groups=%+v", r.MatchedGroups)
	}
}

func TestCreditCardInput_RepeatedAmountAmbiguous(t *testing.T) {
	r := reconciliation.Calculate(CreditCardInput())
	if len(r.AmbiguousCandidates) == 0 {
		t.Fatalf("expected ambiguous candidates for the repeated $60 charges")
	}
	for _, id := range []string{"CC-B4", "CC-B5"} {
		found := false
		for _, u := range r.UnmatchedBookItems {
			if u == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected %s to remain unmatched (ambiguous), got unmatched=%v", id, r.UnmatchedBookItems)
		}
	}
}

func TestLoanInput_MismatchDetectedNoAmortizationAssumed(t *testing.T) {
	r := reconciliation.Calculate(LoanInput())
	if r.Status != reconciliation.StatusUnreconciled {
		t.Fatalf("expected UNRECONCILED (genuine $150 mismatch), got %s", r.Status)
	}
	if r.Equation.Difference != -150 {
		t.Fatalf("expected difference -150 (unrecorded fee), got %v", r.Equation.Difference)
	}
	if r.BookBalance.RollforwardMismatch || r.ExternalBalance.RollforwardMismatch {
		t.Fatalf("expected clean side-specific rollforwards on both sides, book=%+v external=%+v", r.BookBalance, r.ExternalBalance)
	}
}

func TestInventoryControlBalanceOnlyInput_Reconciled(t *testing.T) {
	r := reconciliation.Calculate(InventoryControlBalanceOnlyInput())
	if r.Status != reconciliation.StatusReconciled {
		t.Fatalf("expected RECONCILED, got %s", r.Status)
	}
	if r.TransactionModeAvailable {
		t.Fatalf("expected transaction mode unavailable for balance-only input")
	}
}

func TestPayrollClearingBalanceOnlyInput_Unreconciled(t *testing.T) {
	r := reconciliation.Calculate(PayrollClearingBalanceOnlyInput())
	if r.Status != reconciliation.StatusUnreconciled {
		t.Fatalf("expected UNRECONCILED (small deliberate difference), got %s", r.Status)
	}
}

func TestIntercompanyBalanceOnlyInput_ReversedOrientationReconciles(t *testing.T) {
	r := reconciliation.Calculate(IntercompanyBalanceOnlyInput())
	if r.Status != reconciliation.StatusReconciled {
		t.Fatalf("expected RECONCILED once reversed orientation is applied to the reciprocal balance, got %s (equation=%+v)", r.Status, r.Equation)
	}
}
