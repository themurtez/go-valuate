package closechecklist_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
)

func simpleTemplate() closechecklist.Template {
	return closechecklist.Template{
		TemplateID: "t1",
		Name:       "Simple",
		Version:    "1.0",
		Sections: []closechecklist.SectionDefinition{
			{SectionCode: "A", Name: "Section A"},
			{SectionCode: "B", Name: "Section B"},
		},
		Tasks: []closechecklist.TaskDefinition{
			{TaskCode: "t1", SectionCode: "A", Name: "Task 1", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways}},
			{TaskCode: "t2", SectionCode: "B", Name: "Task 2", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				Dependencies: []closechecklist.TaskDependency{{DependsOnTaskCode: "t1", Type: closechecklist.DependencyMustBeCompleted}}},
		},
	}
}

func simplePeriod() closechecklist.Period {
	return closechecklist.Period{
		PeriodID:        "2025-01",
		StartDate:       date("2025-01-01"),
		EndDate:         date("2025-01-31"),
		TargetCloseDate: date("2025-02-05"),
	}
}

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCalculate_EmptyInstance_NotStarted(t *testing.T) {
	in := closechecklist.Instance{
		Template:       simpleTemplate(),
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistNotStarted {
		t.Fatalf("readiness = %s, want NOT_STARTED", res.Readiness)
	}
	if len(res.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(res.Tasks))
	}
	for _, tr := range res.Tasks {
		if tr.ReportedStatus != closechecklist.TaskNotStarted {
			t.Errorf("task %s reported_status = %s, want NOT_STARTED (missing-state default)", tr.TaskCode, tr.ReportedStatus)
		}
	}
}

func TestCalculate_AllCompleted_ReadyToClose(t *testing.T) {
	in := closechecklist.Instance{
		Template:       simpleTemplate(),
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Fatalf("readiness = %s, want READY_TO_CLOSE; blockers=%+v", res.Readiness, res.Blockers)
	}
	if res.Completion.SatisfiedRequiredTaskCount != 2 {
		t.Errorf("satisfied required = %d, want 2", res.Completion.SatisfiedRequiredTaskCount)
	}
}

func TestCalculate_DependencyBlocksDownstream(t *testing.T) {
	in := closechecklist.Instance{
		Template:       simpleTemplate(),
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t2", Status: closechecklist.TaskCompleted}, // t1 not completed
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	var t2 closechecklist.TaskResult
	for _, tr := range res.Tasks {
		if tr.TaskCode == "t2" {
			t2 = tr
		}
	}
	if t2.Satisfied {
		t.Errorf("t2 satisfied = true, want false (t1 dependency incomplete)")
	}
	if t2.ReadyToStart {
		t.Errorf("t2 ready_to_start = true, want false")
	}
	found := false
	for _, b := range t2.Blockers {
		if b.ReasonCode == closechecklist.BlockerDependencyIncomplete && b.DependencyTaskCode == "t1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a DEPENDENCY_INCOMPLETE blocker referencing t1, got %+v", t2.Blockers)
	}
}

func TestCalculate_NotApplicableExcludedFromDenominator(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[1].Applicability = closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityCallerFlag, FlagKey: "has_t2"}
	in := closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted},
		},
		ApplicabilityFlags: map[string]bool{"has_t2": false},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Completion.RequiredTaskCount != 1 {
		t.Errorf("required task count = %d, want 1 (t2 excluded as NOT_APPLICABLE)", res.Completion.RequiredTaskCount)
	}
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Errorf("readiness = %s, want READY_TO_CLOSE", res.Readiness)
	}
}

func TestCalculate_CompletedWithMissingEvidence_NotSatisfied(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[0].EvidencePolicy = closechecklist.EvidencePolicy{Type: closechecklist.EvidenceAtLeastOne}
	in := closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted}, // no evidence
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	var t1 closechecklist.TaskResult
	for _, tr := range res.Tasks {
		if tr.TaskCode == "t1" {
			t1 = tr
		}
	}
	if t1.Satisfied {
		t.Errorf("t1 satisfied = true, want false (missing required evidence)")
	}
	foundFinding := false
	for _, f := range t1.Findings {
		if f.Code == closechecklist.FindingTaskMarkedCompleteWithMissingRequirements {
			foundFinding = true
		}
	}
	if !foundFinding {
		t.Errorf("expected TASK_MARKED_COMPLETE_WITH_MISSING_REQUIREMENTS finding, got %+v", t1.Findings)
	}
}

func TestCalculate_CompletedWithEvidence_Satisfied(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[0].EvidencePolicy = closechecklist.EvidencePolicy{Type: closechecklist.EvidenceAtLeastOne}
	in := closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted, Evidence: []closechecklist.EvidenceRef{{EvidenceID: "E1"}}},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Fatalf("readiness = %s, want READY_TO_CLOSE", res.Readiness)
	}
}

func TestCalculate_RequiredSignOffMissing_ReadyForReview(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[1].ReviewPolicy = closechecklist.ReviewPolicy{Type: closechecklist.ReviewPreparerAndReviewer, RequireDistinctActors: true}
	in := closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted},
			{
				TaskCode: "t2", Status: closechecklist.TaskCompleted,
				SignOffs: []closechecklist.SignOff{{SignOffID: "S1", Role: closechecklist.SignOffPreparer, ActorRef: "alice", SignedAt: date("2025-01-31")}},
			},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistReadyForReview {
		t.Fatalf("readiness = %s, want READY_FOR_REVIEW; blockers=%+v", res.Readiness, res.Blockers)
	}
}

func TestCalculate_SkippedRequiredTaskNotAllowed_Blocking(t *testing.T) {
	tmpl := simpleTemplate()
	in := closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskSkipped},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness == closechecklist.ChecklistReadyToClose {
		t.Errorf("readiness = READY_TO_CLOSE, want not ready (required task skipped without AllowSkip)")
	}
	foundFinding := false
	for _, f := range res.Findings {
		if f.Code == closechecklist.FindingRequiredTaskSkipped {
			foundFinding = true
		}
	}
	if !foundFinding {
		t.Errorf("expected REQUIRED_TASK_SKIPPED finding, got %+v", res.Findings)
	}
}

func TestCalculate_SkippedRequiredTaskAllowed_Satisfied(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[0].AllowSkip = true
	in := closechecklist.Instance{
		Template:       tmpl,
		Period:         simplePeriod(),
		EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskSkipped},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Fatalf("readiness = %s, want READY_TO_CLOSE; blockers=%+v", res.Readiness, res.Blockers)
	}
}
