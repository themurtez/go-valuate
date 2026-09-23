package closequality_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
)

func basePeriod() closequality.PeriodInfo {
	return closequality.PeriodInfo{Period: "2025-01", StartDate: "2025-01-01", EndDate: "2025-01-31"}
}

// TestMissingModule_NotRequired_NoPenalty proves an absent optional
// module never produces a finding or lowers coverage denominators when
// not required by Applicability.
func TestMissingModule_NotRequired_NoPenalty(t *testing.T) {
	in := closequality.Input{Period: basePeriod()}
	policy := closequality.DefaultPolicy() // Applicability all false by default

	result := closequality.Calculate(in, policy)

	for _, f := range result.Blockers {
		if f.Code == closequality.FindingMissingRequiredInput {
			t.Errorf("unexpected FindingMissingRequiredInput blocker for non-required module: %+v", f)
		}
	}
	for _, f := range result.Warnings {
		if f.Code == closequality.FindingMissingRequiredInput {
			t.Errorf("unexpected FindingMissingRequiredInput warning for non-required module: %+v", f)
		}
	}
	if len(result.Coverage.MissingModules) != 0 {
		t.Errorf("expected no missing modules when nothing is required, got %v", result.Coverage.MissingModules)
	}
}

// TestMissingModule_Required_ProducesBlockingFinding proves a required
// but absent module produces FindingMissingRequiredInput and forces
// NOT_READY when TreatMissingRequiredInputAsBlocker is true (the
// default).
func TestMissingModule_Required_ProducesBlockingFinding(t *testing.T) {
	tests := []struct {
		name   string
		policy func(closequality.Policy) closequality.Policy
	}{
		{"AR", func(p closequality.Policy) closequality.Policy { p.Applicability.ARRequired = true; return p }},
		{"Statements", func(p closequality.Policy) closequality.Policy { p.Applicability.StatementsRequired = true; return p }},
		{"JournalDiagnostics", func(p closequality.Policy) closequality.Policy {
			p.Applicability.JournalDiagnosticsRequired = true
			return p
		}},
		{"Reconciliations", func(p closequality.Policy) closequality.Policy {
			p.Applicability.ReconciliationsRequired = true
			return p
		}},
		{"CloseTasks", func(p closequality.Policy) closequality.Policy { p.Applicability.CloseTasksRequired = true; return p }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := closequality.Input{Period: basePeriod()}
			policy := tt.policy(closequality.DefaultPolicy())

			result := closequality.Calculate(in, policy)

			if result.Status != closequality.StatusNotReady {
				t.Errorf("status = %s, want NOT_READY", result.Status)
			}
			found := false
			for _, f := range result.Blockers {
				if f.Code == closequality.FindingMissingRequiredInput {
					found = true
				}
			}
			if !found {
				t.Errorf("expected FindingMissingRequiredInput among blockers, got %+v", result.Blockers)
			}
		})
	}
}

// TestMissingModule_Required_WarningWhenNotBlocking proves
// TreatMissingRequiredInputAsBlocker=false downgrades the missing-input
// finding to a warning instead of a blocker.
func TestMissingModule_Required_WarningWhenNotBlocking(t *testing.T) {
	in := closequality.Input{Period: basePeriod()}
	policy := closequality.DefaultPolicy()
	policy.Applicability.ARRequired = true
	policy.TreatMissingRequiredInputAsBlocker = false

	result := closequality.Calculate(in, policy)

	if result.Status != closequality.StatusReadyWithWarnings {
		t.Errorf("status = %s, want READY_WITH_WARNINGS", result.Status)
	}
	found := false
	for _, f := range result.Warnings {
		if f.Code == closequality.FindingMissingRequiredInput {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FindingMissingRequiredInput among warnings, got %+v", result.Warnings)
	}
}

// TestAllInputsMissing_IsUnassessed proves a wholly empty Input with no
// required modules at all comes back UNASSESSED, not READY — there is
// insufficient basis for a determination, per readiness.go's documented
// rule.
func TestAllInputsMissing_IsUnassessed(t *testing.T) {
	in := closequality.Input{Period: basePeriod()}
	policy := closequality.DefaultPolicy()

	result := closequality.Calculate(in, policy)

	if result.Status != closequality.StatusUnassessed {
		t.Errorf("status = %s, want UNASSESSED (no dimension could be assessed); dims=%+v", result.Status, result.Dimensions)
	}
}
