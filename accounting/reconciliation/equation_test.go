package reconciliation

import "testing"

func TestEquation_FullFormula(t *testing.T) {
	book := Balance{EndingAvailable: true, EndingBalance: 1000}
	external := Balance{EndingAvailable: true, EndingBalance: 950}
	items := []ReconcilingItem{
		{ItemID: "R1", Type: ReconcilingOutstandingCheck, Side: ReconcilingSideBook, Amount: -50},
	}
	eq := applyReconciliationEquation(book, external, items, 0.01)
	if !eq.Available {
		t.Fatalf("expected equation available")
	}
	// AdjustedBook = 1000 + (-50) = 950; AdjustedExternal = 950 + 0 = 950; Difference = 0
	if eq.AdjustedBookBalance != 950 {
		t.Fatalf("expected adjusted book balance 950, got %v", eq.AdjustedBookBalance)
	}
	if eq.AdjustedExternalBalance != 950 {
		t.Fatalf("expected adjusted external balance 950, got %v", eq.AdjustedExternalBalance)
	}
	if eq.Difference != 0 {
		t.Fatalf("expected difference 0, got %v", eq.Difference)
	}
	if !eq.WithinTolerance {
		t.Fatalf("expected within tolerance")
	}
}

func TestEquation_UnavailableWithoutBothEndingBalances(t *testing.T) {
	book := Balance{EndingAvailable: true, EndingBalance: 1000}
	external := Balance{} // no ending balance
	eq := applyReconciliationEquation(book, external, nil, 0.01)
	if eq.Available {
		t.Fatalf("expected equation unavailable when external ending balance is missing")
	}
	// Reconciling effects are still computed even when the equation
	// itself is unavailable (task section 26's formula components should
	// not silently vanish).
	if eq.BookReconcilingEffects.Count != 0 {
		t.Fatalf("expected zero reconciling effects with no items supplied")
	}
}

func TestEquation_ExternalSideReconcilingEffects(t *testing.T) {
	book := Balance{EndingAvailable: true, EndingBalance: 1000}
	external := Balance{EndingAvailable: true, EndingBalance: 1000}
	items := []ReconcilingItem{
		{ItemID: "R1", Type: ReconcilingUnrecordedFee, Side: ReconcilingSideExternal, Amount: -20},
	}
	eq := applyReconciliationEquation(book, external, items, 0.01)
	if eq.ExternalReconcilingEffects.Total != -20 {
		t.Fatalf("expected external reconciling effect -20, got %v", eq.ExternalReconcilingEffects.Total)
	}
	// AdjustedExternal = 1000 + (-20) = 980; AdjustedBook = 1000; diff = 20
	if eq.Difference != 20 {
		t.Fatalf("expected difference 20, got %v", eq.Difference)
	}
}

func TestStatus_DecisionTable(t *testing.T) {
	tests := []struct {
		name string
		in   statusInputs
		want Status
	}{
		{"invalid on bad key", statusInputs{reconciliationKeyValid: false, asOfDateValid: true}, StatusInvalid},
		{"invalid on bad as-of-date", statusInputs{reconciliationKeyValid: true, asOfDateValid: false}, StatusInvalid},
		{"incomplete with no data", statusInputs{reconciliationKeyValid: true, asOfDateValid: true, balanceDataSufficient: false}, StatusIncomplete},
		{"reconciled when balances tie and nothing outstanding", statusInputs{
			reconciliationKeyValid: true, asOfDateValid: true, balanceDataSufficient: true,
			equationAvailable: true, withinTolerance: true,
		}, StatusReconciled},
		{"reconciled with items when reconciling items remain", statusInputs{
			reconciliationKeyValid: true, asOfDateValid: true, balanceDataSufficient: true,
			equationAvailable: true, withinTolerance: true, hasUnresolvedReconcilingItems: true,
		}, StatusReconciledWithItems},
		{"unreconciled on balance mismatch", statusInputs{
			reconciliationKeyValid: true, asOfDateValid: true, balanceDataSufficient: true,
			equationAvailable: true, withinTolerance: false,
		}, StatusUnreconciled},
		{"unreconciled on material unmatched despite tie", statusInputs{
			reconciliationKeyValid: true, asOfDateValid: true, balanceDataSufficient: true,
			equationAvailable: true, withinTolerance: true, hasUnresolvedMaterialUnmatched: true,
		}, StatusUnreconciled},
		{"unreconciled on ambiguous matches despite tie", statusInputs{
			reconciliationKeyValid: true, asOfDateValid: true, balanceDataSufficient: true,
			equationAvailable: true, withinTolerance: true, hasAmbiguousMatches: true,
		}, StatusUnreconciled},
		{"reconciled when transaction-only (no balances) and nothing outstanding", statusInputs{
			reconciliationKeyValid: true, asOfDateValid: true, balanceDataSufficient: true,
			equationAvailable: false,
		}, StatusReconciled},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveStatus(tc.in)
			if got != tc.want {
				t.Errorf("deriveStatus(%+v) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

// TestInvariant_ReconciledImpliesWithinTolerance locks the core status
// invariant from task section 77: "RECONCILED implies balance difference
// within tolerance."
func TestInvariant_ReconciledImpliesWithinTolerance(t *testing.T) {
	book := 1000.0
	external := 1000.5
	in := Input{
		AccountID:       "A1",
		AsOfDate:        "2025-01-01",
		BookBalance:     BalanceInput{EndingBalance: &book},
		ExternalBalance: BalanceInput{EndingBalance: &external},
		Policy:          MatchingPolicy{AmountTolerance: 0}, // exact equality required
	}
	r := Calculate(in)
	if r.Status == StatusReconciled && !r.Equation.WithinTolerance {
		t.Fatalf("invariant violated: RECONCILED but not within tolerance: %+v", r.Equation)
	}
	if r.Status != StatusReconciled && r.Equation.WithinTolerance && r.Equation.Available {
		// Not itself a violation of the stated invariant (which is one-
		// directional), but worth asserting the expected direction here
		// too: with balances tying exactly and no other findings, status
		// should indeed be RECONCILED.
		t.Fatalf("expected RECONCILED when balances tie within tolerance and nothing else is outstanding, got %s", r.Status)
	}
}
