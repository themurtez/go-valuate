package closechecklist_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
)

func timePtr(t time.Time) *time.Time { return &t }

func exceptionTemplate() closechecklist.Template {
	tmpl := simpleTemplate()
	tmpl.Tasks[1].EvidencePolicy = closechecklist.EvidencePolicy{Type: closechecklist.EvidenceAtLeastOne}
	tmpl.Tasks[1].ExceptionPolicy = closechecklist.ExceptionAllowedWithApproval
	return tmpl
}

func TestException_ValidPermitted_SuppressesEffectiveBlockingButKeepsBlocker(t *testing.T) {
	tmpl := exceptionTemplate()
	in := closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted}, // missing evidence
		},
		Exceptions: []closechecklist.Exception{
			{
				ExceptionID: "EX-1", TaskCode: "t2", ReasonCode: "EVIDENCE_WAIVED",
				ApprovedByRef: "controller@example.com", ApprovedAt: date("2025-01-30"),
			},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())

	var t2 closechecklist.TaskResult
	for _, tr := range res.Tasks {
		if tr.TaskCode == "t2" {
			t2 = tr
		}
	}
	if !t2.ExceptionApplied {
		t.Errorf("expected ExceptionApplied=true")
	}
	if t2.EffectiveBlocking {
		t.Errorf("expected EffectiveBlocking=false once exception applied")
	}
	if len(t2.Blockers) == 0 {
		t.Fatalf("expected the underlying blocker to remain visible even with a valid exception")
	}
	for _, b := range t2.Blockers {
		if b.EffectiveBlocking {
			t.Errorf("blocker %+v should have EffectiveBlocking=false", b)
		}
		if !b.ExceptionApplied {
			t.Errorf("blocker %+v should have ExceptionApplied=true", b)
		}
	}
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Errorf("readiness = %s, want READY_TO_CLOSE (exception suppresses the only blocker)", res.Readiness)
	}
}

func TestException_Expired_DoesNotSuppress(t *testing.T) {
	tmpl := exceptionTemplate()
	in := closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted},
		},
		Exceptions: []closechecklist.Exception{
			{
				ExceptionID: "EX-1", TaskCode: "t2", ReasonCode: "EVIDENCE_WAIVED",
				ApprovedByRef: "controller@example.com", ApprovedAt: date("2025-01-01"),
				ExpiresAt: timePtr(date("2025-01-15")),
			},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())

	var t2 closechecklist.TaskResult
	for _, tr := range res.Tasks {
		if tr.TaskCode == "t2" {
			t2 = tr
		}
	}
	if t2.ExceptionApplied {
		t.Errorf("expired exception should not report ExceptionApplied=true")
	}
	if !t2.EffectiveBlocking {
		t.Errorf("expired exception should not suppress effective blocking")
	}
	foundExpired := false
	for _, f := range res.Findings {
		if f.Code == closechecklist.FindingExpiredException {
			foundExpired = true
		}
	}
	if !foundExpired {
		t.Errorf("expected EXPIRED_EXCEPTION finding, got %+v", res.Findings)
	}
}

func TestException_NotAllowedOnTask_RemainsEffective(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[0].EvidencePolicy = closechecklist.EvidencePolicy{Type: closechecklist.EvidenceAtLeastOne}
	// ExceptionPolicy left unset -> NOT_ALLOWED.
	in := closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted},
		},
		Exceptions: []closechecklist.Exception{
			{ExceptionID: "EX-1", TaskCode: "t1", ReasonCode: "WAIVED", ApprovedByRef: "controller", ApprovedAt: date("2025-01-30")},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	var t1 closechecklist.TaskResult
	for _, tr := range res.Tasks {
		if tr.TaskCode == "t1" {
			t1 = tr
		}
	}
	if t1.ExceptionApplied {
		t.Errorf("exception should not apply when task's ExceptionPolicy is NOT_ALLOWED")
	}
	if !t1.EffectiveBlocking {
		t.Errorf("blocker should remain effective when exceptions are not allowed on this task")
	}
}

func TestException_AllowedWithApproval_MissingApprover_NotPermitted(t *testing.T) {
	tmpl := exceptionTemplate()
	in := closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted},
		},
		Exceptions: []closechecklist.Exception{
			{ExceptionID: "EX-1", TaskCode: "t2", ReasonCode: "WAIVED", ApprovedAt: date("2025-01-30")}, // no ApprovedByRef
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	var t2 closechecklist.TaskResult
	for _, tr := range res.Tasks {
		if tr.TaskCode == "t2" {
			t2 = tr
		}
	}
	if t2.ExceptionApplied {
		t.Errorf("exception without ApprovedByRef should not be permitted under ALLOWED_WITH_APPROVAL")
	}
}
