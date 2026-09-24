package closechecklist_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
)

func TestEvidence_SpecificTypes_MissingType(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[0].EvidencePolicy = closechecklist.EvidencePolicy{Type: closechecklist.EvidenceSpecificTypes, RequiredTypes: []string{"WORKPAPER", "SCREENSHOT"}}
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted, Evidence: []closechecklist.EvidenceRef{{EvidenceID: "E1", Type: "WORKPAPER"}}},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	tr := taskResultByCode(res, "t1")
	if tr.Satisfied {
		t.Errorf("expected not satisfied: missing required SCREENSHOT evidence type")
	}
	var missingType string
	for _, b := range tr.Blockers {
		if b.ReasonCode == closechecklist.BlockerEvidenceMissing {
			missingType = b.MissingEvidenceType
		}
	}
	if missingType != "SCREENSHOT" {
		t.Errorf("missing_evidence_type = %q, want SCREENSHOT", missingType)
	}
}

func TestEvidence_SpecificTypes_AllPresent(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[0].EvidencePolicy = closechecklist.EvidencePolicy{Type: closechecklist.EvidenceSpecificTypes, RequiredTypes: []string{"WORKPAPER", "SCREENSHOT"}}
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted, Evidence: []closechecklist.EvidenceRef{
				{EvidenceID: "E1", Type: "WORKPAPER"}, {EvidenceID: "E2", Type: "SCREENSHOT"},
			}},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Fatalf("readiness = %s, want READY_TO_CLOSE", res.Readiness)
	}
}

func TestApplicability_ExternalGatePresent(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[1].Applicability = closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityExternalGatePresent, GateCode: "some.gate"}

	// Gate absent -> NOT_APPLICABLE.
	in1 := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{{TaskCode: "t1", Status: closechecklist.TaskCompleted}},
	}
	res1 := closechecklist.Calculate(in1, closechecklist.DefaultPolicy())
	if taskResultByCode(res1, "t2").Applicability != closechecklist.ApplicableNo {
		t.Errorf("expected t2 NOT_APPLICABLE with no gate present")
	}

	// Gate present (any status) -> APPLICABLE.
	in2 := in1
	in2.Gates = []closechecklist.GateFact{{GateCode: "some.gate", Status: closechecklist.GateFail}}
	res2 := closechecklist.Calculate(in2, closechecklist.DefaultPolicy())
	if taskResultByCode(res2, "t2").Applicability != closechecklist.ApplicableYes {
		t.Errorf("expected t2 APPLICABLE with gate present (regardless of its status)")
	}
}

func TestApplicability_CallerFlag_MissingIsUndetermined(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[1].Applicability = closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityCallerFlag, FlagKey: "unset_flag"}
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if taskResultByCode(res, "t2").Applicability != closechecklist.ApplicableUndetermined {
		t.Errorf("expected UNDETERMINED for a caller-flag rule with no supplied flag")
	}
}

func TestApplicability_ExplicitOverride_WinsOverRule(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[1].Applicability = closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityCallerFlag, FlagKey: "x"}
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		ApplicabilityFlags:     map[string]bool{"x": true},
		ApplicabilityOverrides: []closechecklist.ApplicabilityOverride{{TaskCode: "t2", Applicability: closechecklist.ApplicableNo, Reason: "manual override"}},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if taskResultByCode(res, "t2").Applicability != closechecklist.ApplicableNo {
		t.Errorf("expected explicit override to win over the caller-flag rule")
	}
}

func TestPeriodConsistency_ClosedWithIncompleteRequired(t *testing.T) {
	tmpl := simpleTemplate()
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		PeriodState: closechecklist.PeriodStateClosed,
		TaskStates:  []closechecklist.TaskState{{TaskCode: "t1", Status: closechecklist.TaskCompleted}}, // t2 not started
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if len(res.PeriodConsistency) == 0 {
		t.Fatalf("expected a period consistency finding for CLOSED with incomplete required tasks")
	}
	if res.PeriodConsistency[0].Code != string(closechecklist.FindingPeriodMarkedClosedWithIncompleteRequiredTasks) {
		t.Errorf("code = %s, want PERIOD_MARKED_CLOSED_WITH_INCOMPLETE_REQUIRED_TASKS", res.PeriodConsistency[0].Code)
	}
	if res.Readiness != closechecklist.ChecklistClosedWithExceptions {
		t.Errorf("readiness = %s, want CLOSED_WITH_EXCEPTIONS", res.Readiness)
	}
}

func TestPeriodConsistency_OpenAndReady_NoInconsistency(t *testing.T) {
	tmpl := simpleTemplate()
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		PeriodState: closechecklist.PeriodStateOpen,
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if len(res.PeriodConsistency) != 0 {
		t.Errorf("expected no period consistency finding, got %+v", res.PeriodConsistency)
	}
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Errorf("readiness = %s, want READY_TO_CLOSE", res.Readiness)
	}
}

func TestValidation_UnknownSection_ReportsIssue(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[0].SectionCode = "GHOST"
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	found := false
	for _, iss := range res.Issues {
		if iss.Code == closechecklist.IssueUnknownSection {
			found = true
		}
	}
	if !found {
		t.Errorf("expected UNKNOWN_SECTION issue, got %+v", res.Issues)
	}
}

func TestValidation_DuplicateTaskDefinition_ReportsIssue(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks = append(tmpl.Tasks, tmpl.Tasks[0])
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	found := false
	for _, iss := range res.Issues {
		if iss.Code == closechecklist.IssueDuplicateTaskDefinition {
			found = true
		}
	}
	if !found {
		t.Errorf("expected DUPLICATE_TASK_DEFINITION issue, got %+v", res.Issues)
	}
}

func TestValidation_InvalidPeriod_StructurallyInvalid(t *testing.T) {
	tmpl := simpleTemplate()
	in := closechecklist.Instance{Template: tmpl, EvaluationDate: date("2025-02-01")} // no Period dates
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistInvalid {
		t.Fatalf("readiness = %s, want INVALID", res.Readiness)
	}
	found := false
	for _, iss := range res.Issues {
		if iss.Code == closechecklist.IssueInvalidPeriod {
			found = true
		}
	}
	if !found {
		t.Errorf("expected INVALID_PERIOD issue, got %+v", res.Issues)
	}
}

func TestValidation_UnknownExceptionTask_ReportsIssue(t *testing.T) {
	tmpl := simpleTemplate()
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		Exceptions: []closechecklist.Exception{{ExceptionID: "EX-1", TaskCode: "ghost", ApprovedAt: date("2025-01-01")}},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	found := false
	for _, iss := range res.Issues {
		if iss.Code == closechecklist.IssueUnknownExceptionTask {
			found = true
		}
	}
	if !found {
		t.Errorf("expected UNKNOWN_EXCEPTION_TASK issue, got %+v", res.Issues)
	}
}

func TestComparison_PriorClose_NewAndResolvedBlockers(t *testing.T) {
	tmpl := simpleTemplate()
	priorIn := closechecklist.Instance{
		Template: tmpl, Period: closechecklist.Period{PeriodID: "2024-12", StartDate: date("2024-12-01"), EndDate: date("2024-12-31"), TargetCloseDate: date("2025-01-05")},
		EvaluationDate: date("2025-01-02"),
		TaskStates:     []closechecklist.TaskState{{TaskCode: "t2", Status: closechecklist.TaskCompleted}}, // t1 blocker in prior
	}
	priorResult := closechecklist.Calculate(priorIn, closechecklist.DefaultPolicy())
	prior := closechecklist.PriorClose{Instance: priorIn, Result: priorResult}

	currentIn := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted},
			{TaskCode: "t2", Status: closechecklist.TaskCompleted},
		},
	}
	res := closechecklist.CalculateWithPrior(currentIn, closechecklist.DefaultPolicy(), prior)
	if res.PriorCloseComparison == nil {
		t.Fatalf("expected a PriorCloseComparison")
	}
	found := false
	for _, code := range res.PriorCloseComparison.ResolvedBlockerTaskCodes {
		if code == "t2" { // t2's dependency-incomplete blocker in the prior period is now resolved
			found = true
		}
	}
	if !found {
		t.Errorf("expected t2 in resolved_blocker_task_codes, got %+v", res.PriorCloseComparison.ResolvedBlockerTaskCodes)
	}
}

func TestComparison_TemplateVersion_AddedRemovedChanged(t *testing.T) {
	priorTmpl := simpleTemplate()
	priorTmpl.Version = "1.0"

	currentTmpl := simpleTemplate()
	currentTmpl.Version = "2.0"
	currentTmpl.Tasks[0].Required = false // changed
	currentTmpl.Tasks = append(currentTmpl.Tasks, closechecklist.TaskDefinition{
		TaskCode: "t3", SectionCode: "A", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
	})
	currentTmpl.Tasks = currentTmpl.Tasks[:len(currentTmpl.Tasks)-2] // drop t2, keep t1(changed)+t3... adjust below

	// Rebuild explicitly for clarity: current has t1(changed) and t3 (new); t2 removed.
	currentTmpl.Tasks = []closechecklist.TaskDefinition{
		{TaskCode: "t1", SectionCode: "A", Required: false, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways}},
		{TaskCode: "t3", SectionCode: "A", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways}},
	}

	priorIn := closechecklist.Instance{
		Template: priorTmpl, Period: closechecklist.Period{PeriodID: "2024-12", StartDate: date("2024-12-01"), EndDate: date("2024-12-31"), TargetCloseDate: date("2025-01-05")},
		EvaluationDate: date("2025-01-02"),
	}
	priorResult := closechecklist.Calculate(priorIn, closechecklist.DefaultPolicy())
	prior := closechecklist.PriorClose{Instance: priorIn, Result: priorResult}

	currentIn := closechecklist.Instance{Template: currentTmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.CalculateWithPrior(currentIn, closechecklist.DefaultPolicy(), prior)

	if res.TemplateVersionComparison == nil {
		t.Fatalf("expected a TemplateVersionComparison")
	}
	tv := res.TemplateVersionComparison
	if !containsStr(tv.AddedTaskCodes, "t3") {
		t.Errorf("expected t3 in added_task_codes, got %+v", tv.AddedTaskCodes)
	}
	if !containsStr(tv.RemovedTaskCodes, "t2") {
		t.Errorf("expected t2 in removed_task_codes, got %+v", tv.RemovedTaskCodes)
	}
	if !containsStr(tv.ChangedTaskCodes, "t1") {
		t.Errorf("expected t1 in changed_task_codes (Required flipped), got %+v", tv.ChangedTaskCodes)
	}
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
