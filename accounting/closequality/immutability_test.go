package closequality_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
)

// TestImmutability_InputAndPolicyNotMutated proves Calculate never
// mutates its Input or Policy arguments, by snapshotting both to JSON
// before and after the call.
func TestImmutability_InputAndPolicyNotMutated(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()

	beforeIn := mustMarshal(t, in)
	beforePolicy := mustMarshal(t, policy)

	_ = closequality.Calculate(in, policy)

	afterIn := mustMarshal(t, in)
	afterPolicy := mustMarshal(t, policy)

	if beforeIn != afterIn {
		t.Errorf("Input was mutated by Calculate")
	}
	if beforePolicy != afterPolicy {
		t.Errorf("Policy was mutated by Calculate")
	}
}

// TestImmutability_PriorResultNotMutated proves CalculateWithPrior never
// mutates the prior Result argument.
func TestImmutability_PriorResultNotMutated(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()
	prior := closequality.Calculate(in, policy)

	beforePrior := mustMarshal(t, prior)

	mutated := fixtures.CleanCloseInput()
	mutated.AR = fixtures.ARResultControlMismatch()
	_ = closequality.CalculateWithPrior(mutated, policy, prior)

	afterPrior := mustMarshal(t, prior)
	if beforePrior != afterPrior {
		t.Errorf("prior Result was mutated by CalculateWithPrior")
	}
}

// TestImmutability_ResultFindingSlicesAreIndependent proves that
// modifying a slice returned in Result.Blockers does not affect a
// second, independent Calculate call's output (guards against
// accidental slice-aliasing back into shared backing arrays such as
// policy.AccountExpectations or upstream Result fields).
func TestImmutability_ResultFindingSlicesAreIndependent(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.AR = fixtures.ARResultControlMismatch()
	policy := fixtures.CleanPolicy()

	first := closequality.Calculate(in, policy)
	if len(first.Blockers) == 0 {
		t.Fatal("expected at least one blocker to mutate for this test")
	}
	original := first.Blockers[0].Message
	first.Blockers[0].Message = "mutated for test"

	second := closequality.Calculate(in, policy)
	if len(second.Blockers) == 0 {
		t.Fatal("expected at least one blocker in second result")
	}
	if second.Blockers[0].Message != original {
		t.Errorf("mutating first result's Blockers affected second Calculate call's output: got %q, want %q", second.Blockers[0].Message, original)
	}
}
