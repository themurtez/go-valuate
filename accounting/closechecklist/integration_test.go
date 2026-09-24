package closechecklist_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closechecklist/fixtures"
)

// TestIntegration_CleanClose_IsReadyToClose builds the full
// ledger -> reconciliation -> closequality -> closechecklist chain via
// real upstream package outputs and asserts a clean period comes back
// READY_TO_CLOSE with no blockers — task spec section 31.
func TestIntegration_CleanClose_IsReadyToClose(t *testing.T) {
	res := closechecklist.Calculate(fixtures.CleanInstance(), closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Fatalf("readiness = %s, want READY_TO_CLOSE; blockers=%+v issues=%+v", res.Readiness, res.Blockers, res.Issues)
	}
	if len(res.Blockers) != 0 {
		t.Errorf("expected no blockers, got %+v", res.Blockers)
	}
	if len(res.Issues) != 0 {
		t.Errorf("expected no issues, got %+v", res.Issues)
	}
}

// TestIntegration_FailedBankReconciliation_BlocksDownstream — section 32:
// an UNRECONCILED bank gate must block bank_rec itself plus every
// downstream task (journal_review, statement_review, controller_review,
// final_approval), and readiness must not be READY_TO_CLOSE.
func TestIntegration_FailedBankReconciliation_BlocksDownstream(t *testing.T) {
	in := fixtures.CleanInstance()
	in.Gates = fixtures.GatesWithFailedBankReconciliation()

	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness == closechecklist.ChecklistReadyToClose {
		t.Fatalf("readiness = READY_TO_CLOSE, want blocked")
	}

	blocked := map[string]bool{}
	for _, b := range res.Blockers {
		blocked[b.TaskCode] = true
	}
	for _, want := range []string{"bank_rec", "journal_review", "statement_review", "controller_review", "final_approval"} {
		if !blocked[want] {
			t.Errorf("expected task %s to have a blocker, blockers=%+v", want, res.Blockers)
		}
	}

	var bankRec closechecklist.TaskResult
	for _, tr := range res.Tasks {
		if tr.TaskCode == "bank_rec" {
			bankRec = tr
		}
	}
	foundGateBlocker := false
	for _, b := range bankRec.Blockers {
		if b.ReasonCode == closechecklist.BlockerGateFailed && b.GateCode == "reconciliation.cash_main" {
			foundGateBlocker = true
		}
	}
	if !foundGateBlocker {
		t.Errorf("expected bank_rec to have a GATE_FAILED blocker on reconciliation.cash_main, got %+v", bankRec.Blockers)
	}
}

// TestIntegration_WarningGate_StillReadyUnderPassOrWarning — section 33
// first half: closequality READY_WITH_WARNINGS with a PASS_OR_WARNING
// gate rule stays ready.
func TestIntegration_WarningGate_StillReadyUnderPassOrWarning(t *testing.T) {
	in := fixtures.CleanInstance()
	in.Gates = fixtures.GatesWithWarnings()

	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Fatalf("readiness = %s, want READY_TO_CLOSE; blockers=%+v", res.Readiness, res.Blockers)
	}
}

// TestIntegration_WarningGate_BlocksUnderStrictPass — section 33 second
// half: the same READY_WITH_WARNINGS closequality result blocks a task
// whose GateRule strictly requires PASS.
func TestIntegration_WarningGate_BlocksUnderStrictPass(t *testing.T) {
	tmpl := fixtures.CleanCloseTemplate()
	for i := range tmpl.Tasks {
		if tmpl.Tasks[i].TaskCode == "controller_review" {
			tmpl.Tasks[i].GateRules = []closechecklist.GateRule{
				{GateCode: "closequality.overall", Require: closechecklist.GateRequirePass, Required: true},
			}
		}
	}

	in := fixtures.CleanInstance()
	in.Template = tmpl
	in.Gates = fixtures.GatesWithWarnings()

	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness == closechecklist.ChecklistReadyToClose {
		t.Fatalf("readiness = READY_TO_CLOSE, want blocked under strict PASS gate requirement")
	}

	var controllerReview closechecklist.TaskResult
	for _, tr := range res.Tasks {
		if tr.TaskCode == "controller_review" {
			controllerReview = tr
		}
	}
	if controllerReview.Satisfied {
		t.Errorf("controller_review should not be satisfied when closequality.overall is WARNING under a strict PASS gate rule")
	}
}

// TestIntegration_GatePassButEvidenceMissing_NotSatisfied — section 34:
// a reconciliation gate can PASS while the checklist task still lacks
// required workpaper evidence; Satisfied must be false regardless.
func TestIntegration_GatePassButEvidenceMissing_NotSatisfied(t *testing.T) {
	tmpl := fixtures.CleanCloseTemplate()
	for i := range tmpl.Tasks {
		if tmpl.Tasks[i].TaskCode == "bank_rec" {
			tmpl.Tasks[i].EvidencePolicy = closechecklist.EvidencePolicy{Type: closechecklist.EvidenceAtLeastOne, RequiredTypes: []string{"WORKPAPER"}}
		}
	}

	in := fixtures.CleanInstance()
	in.Template = tmpl
	// TaskStates already mark bank_rec COMPLETED with no evidence.

	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())

	var bankRec closechecklist.TaskResult
	for _, tr := range res.Tasks {
		if tr.TaskCode == "bank_rec" {
			bankRec = tr
		}
	}
	if bankRec.Satisfied {
		t.Fatalf("bank_rec should not be satisfied: gate PASS does not substitute for missing required evidence")
	}
	if bankRec.EvidenceStatus != closechecklist.EvidenceStatusMissing {
		t.Errorf("evidence_status = %s, want MISSING", bankRec.EvidenceStatus)
	}
}

// TestIntegration_PreparationCompleteReviewMissing_ReadyForReview —
// section 35: every preparation task complete but required reviewer
// sign-off missing resolves READY_FOR_REVIEW, not READY_TO_CLOSE.
func TestIntegration_PreparationCompleteReviewMissing_ReadyForReview(t *testing.T) {
	in := fixtures.CleanInstance()
	states := fixtures.CleanCloseTaskStates()
	for i := range states {
		if states[i].TaskCode == "controller_review" {
			states[i].SignOffs = nil // strip the controller sign-off
		}
	}
	in.TaskStates = states

	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistReadyForReview {
		t.Fatalf("readiness = %s, want READY_FOR_REVIEW; blockers=%+v", res.Readiness, res.Blockers)
	}
}

// TestIntegration_Exceptions — section 36: a valid permitted exception,
// an expired exception, and an exception on a non-exceptable task.
func TestIntegration_Exceptions(t *testing.T) {
	t.Run("valid permitted exception unblocks final_approval", func(t *testing.T) {
		in := fixtures.CleanInstance()
		states := fixtures.CleanCloseTaskStates()
		for i := range states {
			if states[i].TaskCode == "final_approval" {
				states[i].SignOffs = nil // strip required approver sign-off
			}
		}
		in.TaskStates = states
		in.Exceptions = []closechecklist.Exception{
			{
				ExceptionID: "EX-1", TaskCode: "final_approval", ReasonCode: "APPROVER_UNAVAILABLE",
				ApprovedByRef: "controller@example.com", ApprovedAt: date("2025-02-03"),
			},
		}

		res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
		if res.Readiness != closechecklist.ChecklistReadyToClose {
			t.Fatalf("readiness = %s, want READY_TO_CLOSE; blockers=%+v", res.Readiness, res.Blockers)
		}
	})

	t.Run("expired exception leaves task blocked", func(t *testing.T) {
		in := fixtures.CleanInstance()
		states := fixtures.CleanCloseTaskStates()
		for i := range states {
			if states[i].TaskCode == "final_approval" {
				states[i].SignOffs = nil
			}
		}
		in.TaskStates = states
		in.Exceptions = []closechecklist.Exception{
			{
				ExceptionID: "EX-1", TaskCode: "final_approval", ReasonCode: "APPROVER_UNAVAILABLE",
				ApprovedByRef: "controller@example.com", ApprovedAt: date("2025-01-01"),
				ExpiresAt: timePtr(date("2025-01-15")),
			},
		}

		res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
		if res.Readiness == closechecklist.ChecklistReadyToClose {
			t.Fatalf("readiness = READY_TO_CLOSE, want blocked (exception expired)")
		}
	})

	t.Run("exception on non-exceptable task remains effective", func(t *testing.T) {
		in := fixtures.CleanInstance()
		states := fixtures.CleanCloseTaskStates()
		for i := range states {
			if states[i].TaskCode == "bank_rec" {
				states[i].Status = closechecklist.TaskNotStarted
				states[i].CompletedAt = nil
			}
		}
		in.TaskStates = states
		// bank_rec has no ExceptionPolicy set (NOT_ALLOWED by default).
		in.Exceptions = []closechecklist.Exception{
			{ExceptionID: "EX-1", TaskCode: "bank_rec", ReasonCode: "SKIP", ApprovedByRef: "controller", ApprovedAt: date("2025-02-03")},
		}

		res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
		if res.Readiness == closechecklist.ChecklistReadyToClose {
			t.Fatalf("readiness = READY_TO_CLOSE, want blocked: bank_rec's ExceptionPolicy does not allow exceptions")
		}
	})
}
