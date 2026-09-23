package closequality_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
)

// TestIntegration_CleanClose_IsReady builds the full
// ledger -> statements -> AR -> AP -> journal diagnostics chain via real
// upstream package outputs (not hand-built fake Results) and asserts a
// clean period comes back READY with no blockers or warnings.
func TestIntegration_CleanClose_IsReady(t *testing.T) {
	result := closequality.Calculate(fixtures.CleanCloseInput(), fixtures.CleanPolicy())

	if result.Status != closequality.StatusReady {
		t.Fatalf("status = %s, want READY; blockers=%+v warnings=%+v issues=%+v",
			result.Status, result.Blockers, result.Warnings, result.Issues)
	}
	if len(result.Blockers) != 0 {
		t.Errorf("expected no blockers, got %+v", result.Blockers)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings, got %+v", result.Warnings)
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected no issues, got %+v", result.Issues)
	}
}

// TestIntegration_ARControlMismatch_IsNotReady mutates only the AR
// control-account balance and proves that alone flips the result to
// NOT_READY.
func TestIntegration_ARControlMismatch_IsNotReady(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.AR = fixtures.ARResultControlMismatch()

	result := closequality.Calculate(in, fixtures.CleanPolicy())

	if result.Status != closequality.StatusNotReady {
		t.Fatalf("status = %s, want NOT_READY", result.Status)
	}
	if !hasFindingCode(result.Blockers, closequality.FindingARControlMismatch) {
		t.Errorf("expected FindingARControlMismatch among blockers, got %+v", result.Blockers)
	}
}

// TestIntegration_APControlMismatch_IsNotReady mirrors the AR test for AP.
func TestIntegration_APControlMismatch_IsNotReady(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.AP = fixtures.APResultControlMismatch()

	result := closequality.Calculate(in, fixtures.CleanPolicy())

	if result.Status != closequality.StatusNotReady {
		t.Fatalf("status = %s, want NOT_READY", result.Status)
	}
	if !hasFindingCode(result.Blockers, closequality.FindingAPControlMismatch) {
		t.Errorf("expected FindingAPControlMismatch among blockers, got %+v", result.Blockers)
	}
}

// TestIntegration_UnbalancedLedger_IsNotReady mutates only the ledger to
// use a deliberately-unbalanced entries fixture.
func TestIntegration_UnbalancedLedger_IsNotReady(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.Ledger = fixtures.UnbalancedLedger()
	in.LedgerValidation = nil

	result := closequality.Calculate(in, fixtures.CleanPolicy())

	if result.Status != closequality.StatusNotReady {
		t.Fatalf("status = %s, want NOT_READY; blockers=%+v", result.Status, result.Blockers)
	}
	if !hasFindingCode(result.Blockers, closequality.FindingLedgerValidationBlocker) {
		t.Errorf("expected FindingLedgerValidationBlocker among blockers, got %+v", result.Blockers)
	}
}

// TestIntegration_MaterialPostCloseEntry_IsPolicyDriven proves post-close
// activity severity is controlled by policy (locked period -> blocking).
func TestIntegration_MaterialPostCloseEntry_IsPolicyDriven(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.JournalDiagnostics = fixtures.JournalDiagnosticsResultWithPostCloseEntry()

	result := closequality.Calculate(in, fixtures.CleanPolicy())

	if result.Status != closequality.StatusNotReady {
		t.Fatalf("status = %s, want NOT_READY (period is CLOSED and TreatLockedPostCloseAsBlocking is true)", result.Status)
	}
	if !hasFindingCode(result.Blockers, closequality.FindingPostCloseActivity) {
		t.Errorf("expected FindingPostCloseActivity among blockers, got %+v", result.Blockers)
	}

	// With the same facts but a policy that does not treat locked-period
	// post-close activity as blocking, it should downgrade to a warning
	// (or info if immaterial) rather than NOT_READY.
	policy := fixtures.CleanPolicy()
	policy.TreatLockedPostCloseAsBlocking = false
	result2 := closequality.Calculate(in, policy)
	if hasFindingCode(result2.Blockers, closequality.FindingPostCloseActivity) {
		t.Errorf("did not expect FindingPostCloseActivity among blockers when TreatLockedPostCloseAsBlocking=false, got %+v", result2.Blockers)
	}
}

// TestIntegration_UnreconciledCriticalAccount_IsNotReady mutates only the
// reconciliation status for the declared critical cash account.
func TestIntegration_UnreconciledCriticalAccount_IsNotReady(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.Reconciliations = fixtures.UnreconciledCashStatus()

	result := closequality.Calculate(in, fixtures.CleanPolicy())

	if result.Status != closequality.StatusNotReady {
		t.Fatalf("status = %s, want NOT_READY", result.Status)
	}
	if !hasFindingCode(result.Blockers, closequality.FindingUnreconciledCriticalAccount) {
		t.Errorf("expected FindingUnreconciledCriticalAccount among blockers, got %+v", result.Blockers)
	}
}

func hasFindingCode(findings []closequality.Finding, code closequality.FindingCode) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
