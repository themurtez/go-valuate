package closequality_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
)

// TestComparison_Improvement proves a prior period with a blocker that
// is now resolved reports TrendImproving with the blocker in
// ResolvedBlockers.
func TestComparison_Improvement(t *testing.T) {
	policy := fixtures.CleanPolicy()

	priorInput := fixtures.CleanCloseInput()
	priorInput.AR = fixtures.ARResultControlMismatch()
	prior := closequality.Calculate(priorInput, policy)
	if prior.Status != closequality.StatusNotReady {
		t.Fatalf("prior setup: status = %s, want NOT_READY", prior.Status)
	}

	current := closequality.CalculateWithPrior(fixtures.CleanCloseInput(), policy, prior)

	if current.Comparison == nil {
		t.Fatal("expected Comparison to be populated")
	}
	if current.Comparison.Trend != closequality.TrendImproving {
		t.Errorf("trend = %s, want IMPROVING", current.Comparison.Trend)
	}
	if !hasFindingCode(current.Comparison.ResolvedBlockers, closequality.FindingARControlMismatch) {
		t.Errorf("expected FindingARControlMismatch among ResolvedBlockers, got %+v", current.Comparison.ResolvedBlockers)
	}
	if len(current.Comparison.NewBlockers) != 0 {
		t.Errorf("expected no new blockers, got %+v", current.Comparison.NewBlockers)
	}
}

// TestComparison_Deterioration proves a newly-introduced blocker
// reports TrendDeteriorating with the blocker in NewBlockers.
func TestComparison_Deterioration(t *testing.T) {
	policy := fixtures.CleanPolicy()
	prior := closequality.Calculate(fixtures.CleanCloseInput(), policy)

	currentInput := fixtures.CleanCloseInput()
	currentInput.AP = fixtures.APResultControlMismatch()
	current := closequality.CalculateWithPrior(currentInput, policy, prior)

	if current.Comparison.Trend != closequality.TrendDeteriorating {
		t.Errorf("trend = %s, want DETERIORATING", current.Comparison.Trend)
	}
	if !hasFindingCode(current.Comparison.NewBlockers, closequality.FindingAPControlMismatch) {
		t.Errorf("expected FindingAPControlMismatch among NewBlockers, got %+v", current.Comparison.NewBlockers)
	}
	if len(current.Comparison.ResolvedBlockers) != 0 {
		t.Errorf("expected no resolved blockers, got %+v", current.Comparison.ResolvedBlockers)
	}
}

// TestComparison_Stable proves an unchanged period reports TrendStable
// with no deltas.
func TestComparison_Stable(t *testing.T) {
	policy := fixtures.CleanPolicy()
	prior := closequality.Calculate(fixtures.CleanCloseInput(), policy)
	current := closequality.CalculateWithPrior(fixtures.CleanCloseInput(), policy, prior)

	if current.Comparison.Trend != closequality.TrendStable {
		t.Errorf("trend = %s, want STABLE", current.Comparison.Trend)
	}
	if len(current.Comparison.NewBlockers) != 0 || len(current.Comparison.ResolvedBlockers) != 0 ||
		len(current.Comparison.NewWarnings) != 0 || len(current.Comparison.ResolvedWarnings) != 0 {
		t.Errorf("expected no deltas for an unchanged period, got %+v", current.Comparison)
	}
}

// TestComparison_NeverChangesCurrentCalculation proves supplying a prior
// Result never changes the current period's own Status/Blockers/
// Warnings — comparison is report-only.
func TestComparison_NeverChangesCurrentCalculation(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.AR = fixtures.ARResultControlMismatch()
	policy := fixtures.CleanPolicy()

	withoutPrior := closequality.Calculate(in, policy)

	prior := closequality.Calculate(fixtures.CleanCloseInput(), policy)
	withPrior := closequality.CalculateWithPrior(in, policy, prior)

	if withoutPrior.Status != withPrior.Status {
		t.Errorf("Status differs with/without prior: %s vs %s", withoutPrior.Status, withPrior.Status)
	}
	if len(withoutPrior.Blockers) != len(withPrior.Blockers) {
		t.Errorf("Blockers count differs with/without prior: %d vs %d", len(withoutPrior.Blockers), len(withPrior.Blockers))
	}
}
