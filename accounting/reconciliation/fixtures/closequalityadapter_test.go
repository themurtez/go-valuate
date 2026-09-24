package fixtures

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

// minimalCloseQualityPeriod is a bare PeriodInfo shared by both
// integration tests below.
func minimalCloseQualityPeriod() closequality.PeriodInfo {
	return closequality.PeriodInfo{Period: "2025-01", StartDate: "2025-01-01", EndDate: "2025-01-31"}
}

// TestCloseQualityIntegration_CriticalAccountReconciledDoesNotBlock
// proves task section 59: a real reconciliation.Result (status
// RECONCILED) converted via CloseQualityReconciliationStatus and fed
// into a real closequality.Calculate call, with that account declared
// both required and critical, does NOT block readiness.
func TestCloseQualityIntegration_CriticalAccountReconciledDoesNotBlock(t *testing.T) {
	book := 1000.0
	external := 1000.0
	recResult := reconciliation.Calculate(reconciliation.Input{
		AccountID:       "1000",
		AsOfDate:        "2025-01-31",
		BookBalance:     reconciliation.BalanceInput{EndingBalance: &book},
		ExternalBalance: reconciliation.BalanceInput{EndingBalance: &external},
		Policy:          reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	})
	if recResult.Status != reconciliation.StatusReconciled {
		t.Fatalf("expected the underlying reconciliation to be RECONCILED, got %s", recResult.Status)
	}

	status := reconciliation.CloseQualityReconciliationStatus(recResult)
	if status.Status != closequality.ReconciliationReconciled {
		t.Fatalf("expected closequality.ReconciliationReconciled, got %s", status.Status)
	}

	cqPolicy := closequality.DefaultPolicy()
	cqPolicy.CriticalAccounts = []string{"1000"}
	cqPolicy.RequiredReconciliationAccounts = []string{"1000"}
	cqPolicy.Applicability.ReconciliationsRequired = true

	cqResult := closequality.Calculate(closequality.Input{
		Period:          minimalCloseQualityPeriod(),
		Reconciliations: []closequality.ReconciliationStatus{status},
	}, cqPolicy)

	for _, f := range cqResult.Blockers {
		if f.Code == closequality.FindingUnreconciledCriticalAccount {
			t.Fatalf("expected no blocking finding for a reconciled critical account, got %+v", f)
		}
	}
}

// TestCloseQualityIntegration_CriticalAccountUnreconciledBlocksReadiness
// proves the other half of task section 59: an UNRECONCILED critical
// account produces NOT_READY under a policy that treats missing/failed
// required input as blocking.
func TestCloseQualityIntegration_CriticalAccountUnreconciledBlocksReadiness(t *testing.T) {
	book := 1000.0
	external := 1200.0 // genuine mismatch
	recResult := reconciliation.Calculate(reconciliation.Input{
		AccountID:       "1000",
		AsOfDate:        "2025-01-31",
		BookBalance:     reconciliation.BalanceInput{EndingBalance: &book},
		ExternalBalance: reconciliation.BalanceInput{EndingBalance: &external},
		Policy:          reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	})
	if recResult.Status != reconciliation.StatusUnreconciled {
		t.Fatalf("expected the underlying reconciliation to be UNRECONCILED, got %s", recResult.Status)
	}

	status := reconciliation.CloseQualityReconciliationStatus(recResult)
	if status.Status != closequality.ReconciliationUnreconciled {
		t.Fatalf("expected closequality.ReconciliationUnreconciled, got %s", status.Status)
	}
	if status.Difference.Amount != -200 {
		t.Fatalf("expected preserved difference -200, got %+v", status.Difference)
	}

	cqPolicy := closequality.DefaultPolicy()
	cqPolicy.CriticalAccounts = []string{"1000"}
	cqPolicy.RequiredReconciliationAccounts = []string{"1000"}
	cqPolicy.Applicability.ReconciliationsRequired = true

	cqResult := closequality.Calculate(closequality.Input{
		Period:          minimalCloseQualityPeriod(),
		Reconciliations: []closequality.ReconciliationStatus{status},
	}, cqPolicy)

	if cqResult.Status != closequality.StatusNotReady {
		t.Fatalf("expected NOT_READY for an unreconciled critical account, got %s (blockers=%+v)", cqResult.Status, cqResult.Blockers)
	}
	found := false
	for _, f := range cqResult.Blockers {
		if f.Code == closequality.FindingUnreconciledCriticalAccount {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected FindingUnreconciledCriticalAccount among blockers, got %+v", cqResult.Blockers)
	}
}

// TestCloseQualityAdapter_PreservesSourceRef confirms
// CloseQualityReconciliationStatus preserves ExternalAccountID as
// SourceRef — task section 58: "Preserve: ... source ref."
func TestCloseQualityAdapter_PreservesSourceRef(t *testing.T) {
	book := 100.0
	external := 100.0
	recResult := reconciliation.Calculate(reconciliation.Input{
		AccountID:         "1000",
		ExternalAccountID: "BANK-XYZ",
		AsOfDate:          "2025-01-31",
		BookBalance:       reconciliation.BalanceInput{EndingBalance: &book},
		ExternalBalance:   reconciliation.BalanceInput{EndingBalance: &external},
		Policy:            reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	})
	status := reconciliation.CloseQualityReconciliationStatus(recResult)
	if status.SourceRef != "BANK-XYZ" {
		t.Fatalf("expected source_ref 'BANK-XYZ', got %q", status.SourceRef)
	}
	if status.AsOfDate != "2025-01-31" {
		t.Fatalf("expected as_of_date preserved, got %q", status.AsOfDate)
	}
}
