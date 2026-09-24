package closechecklist_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
)

func TestTiming_CompletedBeforeDependency_WarningByDefault(t *testing.T) {
	tmpl := simpleTemplate()
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted, CompletedAt: timePtr(date("2025-01-20"))},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted, CompletedAt: timePtr(date("2025-01-15"))}, // before t1
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())

	var finding *closechecklist.Finding
	for _, f := range res.Findings {
		if f.Code == closechecklist.FindingTaskCompletedBeforeDependency {
			f := f
			finding = &f
		}
	}
	if finding == nil {
		t.Fatalf("expected a TASK_COMPLETED_BEFORE_DEPENDENCY finding, got %+v", res.Findings)
	}
	if finding.Severity != closechecklist.FindingSeverityWarning {
		t.Errorf("severity = %s, want WARNING under the default policy", finding.Severity)
	}
	// A warning-only timing finding must never block readiness on its own.
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Errorf("readiness = %s, want READY_TO_CLOSE (timing finding is non-blocking by default)", res.Readiness)
	}
}

func TestTiming_CompletedBeforeDependency_BlockingWhenPolicyOptsIn(t *testing.T) {
	tmpl := simpleTemplate()
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted, CompletedAt: timePtr(date("2025-01-20"))},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted, CompletedAt: timePtr(date("2025-01-15"))},
		},
	}
	policy := closechecklist.Policy{
		TimingWarningsAreFindingsOnly: false,
		BlockingTimingFindingCodes:    []string{string(closechecklist.FindingTaskCompletedBeforeDependency)},
	}
	res := closechecklist.Calculate(in, policy)

	var finding *closechecklist.Finding
	for _, f := range res.Findings {
		if f.Code == closechecklist.FindingTaskCompletedBeforeDependency {
			f := f
			finding = &f
		}
	}
	if finding == nil {
		t.Fatalf("expected a TASK_COMPLETED_BEFORE_DEPENDENCY finding")
	}
	if finding.Severity != closechecklist.FindingSeverityBlocking {
		t.Errorf("severity = %s, want BLOCKING once the policy opts this code in", finding.Severity)
	}
}

func TestTiming_SignOffBeforeCompletion(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[0].ReviewPolicy = closechecklist.ReviewPolicy{Type: closechecklist.ReviewSpecificRoles, RequiredRoles: []closechecklist.SignOffRole{closechecklist.SignOffReviewer}}
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{
				TaskCode: "t1", Status: closechecklist.TaskCompleted, CompletedAt: timePtr(date("2025-01-20")),
				SignOffs: []closechecklist.SignOff{{SignOffID: "S1", Role: closechecklist.SignOffReviewer, ActorRef: "bob", SignedAt: date("2025-01-10")}},
			},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	found := false
	for _, f := range res.Findings {
		if f.Code == closechecklist.FindingSignOffBeforeTaskCompletion {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SIGNOFF_BEFORE_TASK_COMPLETION finding, got %+v", res.Findings)
	}
}
