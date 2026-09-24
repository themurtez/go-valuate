package reconciliation

import (
	"math"
	"testing"
)

func TestBalances_RollforwardWithinTolerance(t *testing.T) {
	opening := 1000.0
	ending := 1100.0
	bal, issues := resolveBalance(BalanceInput{OpeningBalance: &opening, EndingBalance: &ending}, []float64{50, 50}, false, 0.01, 1)
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
	if !bal.ExpectedEndingAvailable || bal.ExpectedEnding != 1100 {
		t.Fatalf("expected ExpectedEnding 1100, got %+v", bal)
	}
	if bal.RollforwardMismatch {
		t.Fatalf("expected no rollforward mismatch, got %+v", bal)
	}
}

func TestBalances_RollforwardMismatchDetected(t *testing.T) {
	opening := 1000.0
	ending := 2000.0 // does not equal opening + activity
	bal, _ := resolveBalance(BalanceInput{OpeningBalance: &opening, EndingBalance: &ending}, []float64{50}, false, 0.01, 1)
	if !bal.RollforwardMismatch {
		t.Fatalf("expected rollforward mismatch, got %+v", bal)
	}
	if bal.RollforwardDifference != 2000-1050 {
		t.Fatalf("expected rollforward difference 950, got %v", bal.RollforwardDifference)
	}
}

func TestBalances_DerivationDisabledLeavesEndingUnavailable(t *testing.T) {
	opening := 1000.0
	bal, _ := resolveBalance(BalanceInput{OpeningBalance: &opening}, []float64{50}, false, 0.01, 1)
	if bal.EndingAvailable {
		t.Fatalf("expected ending unavailable when no ending supplied and derivation disabled, got %+v", bal)
	}
}

func TestBalances_DerivationEnabledComputesEnding(t *testing.T) {
	opening := 1000.0
	bal, _ := resolveBalance(BalanceInput{OpeningBalance: &opening}, []float64{50, 25}, true, 0.01, 1)
	if !bal.EndingAvailable || bal.EndingBalance != 1075 {
		t.Fatalf("expected derived ending 1075, got %+v", bal)
	}
	if bal.EndingProvenance != BalanceDerived {
		t.Fatalf("expected DERIVED provenance, got %s", bal.EndingProvenance)
	}
}

func TestBalances_SuppliedNeverSilentlyReplaced(t *testing.T) {
	// Task section 43: "If supplied and derived ending balances disagree,
	// return issue; do not silently replace supplied value."
	opening := 1000.0
	suppliedEnding := 9999.0 // disagrees with derived (1050)
	bal, issues := resolveBalance(BalanceInput{OpeningBalance: &opening, EndingBalance: &suppliedEnding}, []float64{50}, true, 0.01, 1)
	if bal.EndingBalance != 9999 {
		t.Fatalf("expected supplied value preserved (9999), got %v", bal.EndingBalance)
	}
	if bal.EndingProvenance != BalanceSupplied {
		t.Fatalf("expected SUPPLIED provenance preserved, got %s", bal.EndingProvenance)
	}
	if !bal.SuppliedDerivedMismatch {
		t.Fatalf("expected SuppliedDerivedMismatch flagged")
	}
	found := false
	for _, i := range issues {
		if i.Code == IssueSuppliedDerivedBalanceMismatch {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueSuppliedDerivedBalanceMismatch, got %+v", issues)
	}
}

func TestBalances_NonFiniteInputsExcluded(t *testing.T) {
	nan := math.NaN()
	bal, _ := resolveBalance(BalanceInput{OpeningBalance: &nan}, nil, false, 0.01, 1)
	if bal.OpeningAvailable {
		t.Fatalf("expected non-finite opening balance excluded")
	}
}

// TestBalances_OrientationAppliesToSuppliedBalances locks a real bug
// found while building fixtures: a caller-supplied external
// opening/ending balance must be put through the same orientation
// adjustment ExternalItem.OrientedSignedAmount already applies to
// transaction-level items (task section 18), or a REVERSED liability
// reconciliation would silently compare unoriented balance figures.
func TestBalances_OrientationAppliesToSuppliedBalances(t *testing.T) {
	ending := 100.0
	bal, _ := resolveBalance(BalanceInput{EndingBalance: &ending}, nil, false, 0.01, -1)
	if bal.EndingBalance != -100 {
		t.Fatalf("expected orientation multiplier applied to supplied ending balance (-100), got %v", bal.EndingBalance)
	}
}

// TestInvariant_ReversedOrientationBalanceOnlyReconciles confirms a
// balance-only reconciliation between two economically-tied sides
// reaches RECONCILED under REVERSED orientation when the caller's own
// external balance is denominated in the opposite sign convention from
// the book side — the end-to-end regression for the same bug.
func TestInvariant_ReversedOrientationBalanceOnlyReconciles(t *testing.T) {
	book := 500.0
	external := -500.0 // same underlying balance, opposite sign convention
	in := Input{
		AccountID:       "A",
		AsOfDate:        "2025-01-01",
		BookBalance:     BalanceInput{EndingBalance: &book},
		ExternalBalance: BalanceInput{EndingBalance: &external},
		Policy:          MatchingPolicy{AmountTolerance: 0.01, Orientation: OrientationReversed},
	}
	r := Calculate(in)
	if r.Status != StatusReconciled {
		t.Fatalf("expected RECONCILED once orientation is applied to the supplied external balance, got %s (equation=%+v)", r.Status, r.Equation)
	}
}
