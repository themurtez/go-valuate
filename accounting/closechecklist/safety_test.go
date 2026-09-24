package closechecklist_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
)

// buildChainTemplate returns a Template with n tasks in a single linear
// dependency chain t0 <- t1 <- ... <- t(n-1) — section 47's "long
// dependency chain" adversarial case.
func buildChainTemplate(n int) closechecklist.Template {
	tasks := make([]closechecklist.TaskDefinition, n)
	for i := 0; i < n; i++ {
		td := closechecklist.TaskDefinition{
			TaskCode: fmt.Sprintf("t%d", i), SectionCode: "A", Required: true,
			Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
		}
		if i > 0 {
			td.Dependencies = []closechecklist.TaskDependency{{DependsOnTaskCode: fmt.Sprintf("t%d", i-1)}}
		}
		tasks[i] = td
	}
	return closechecklist.Template{
		TemplateID: "chain", Version: "1.0",
		Sections: []closechecklist.SectionDefinition{{SectionCode: "A"}},
		Tasks:    tasks,
	}
}

func TestSafety_LongDependencyChain_NoStackOverflow(t *testing.T) {
	const n = 5000
	tmpl := buildChainTemplate(n)
	states := make([]closechecklist.TaskState, n)
	for i := 0; i < n; i++ {
		states[i] = closechecklist.TaskState{TaskCode: fmt.Sprintf("t%d", i), Status: closechecklist.TaskCompleted}
	}
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: states,
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if res.Readiness != closechecklist.ChecklistReadyToClose {
		t.Fatalf("readiness = %s, want READY_TO_CLOSE for a fully-completed long chain", res.Readiness)
	}
}

// buildFanOutTemplate returns one root task and n leaf tasks each
// depending on it — section 47's "wide dependency fan-out."
func buildFanOutTemplate(n int) closechecklist.Template {
	tasks := []closechecklist.TaskDefinition{
		{TaskCode: "root", SectionCode: "A", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways}},
	}
	for i := 0; i < n; i++ {
		tasks = append(tasks, closechecklist.TaskDefinition{
			TaskCode: fmt.Sprintf("leaf%d", i), SectionCode: "A", Required: true,
			Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
			Dependencies:  []closechecklist.TaskDependency{{DependsOnTaskCode: "root"}},
		})
	}
	return closechecklist.Template{
		TemplateID: "fanout", Version: "1.0",
		Sections: []closechecklist.SectionDefinition{{SectionCode: "A"}},
		Tasks:    tasks,
	}
}

func TestSafety_WideDependencyFanOut(t *testing.T) {
	const n = 5000
	tmpl := buildFanOutTemplate(n)
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{{TaskCode: "root", Status: closechecklist.TaskCompleted}},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	root := taskResultByCode(res, "root")
	if !root.Satisfied {
		t.Fatalf("root not satisfied")
	}
	if root.DownstreamRequiredTaskCount != n {
		t.Errorf("root downstream_required_task_count = %d, want %d", root.DownstreamRequiredTaskCount, n)
	}
}

func taskResultByCode(res closechecklist.Result, code string) closechecklist.TaskResult {
	for _, tr := range res.Tasks {
		if tr.TaskCode == code {
			return tr
		}
	}
	return closechecklist.TaskResult{}
}

// TestSafety_1000MissingTaskStates: applicable tasks with no TaskState at
// all must all deterministically resolve NOT_STARTED — section 5/47.
func TestSafety_1000MissingTaskStates(t *testing.T) {
	const n = 1000
	tasks := make([]closechecklist.TaskDefinition, n)
	for i := 0; i < n; i++ {
		tasks[i] = closechecklist.TaskDefinition{
			TaskCode: fmt.Sprintf("t%d", i), SectionCode: "A", Required: true,
			Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
		}
	}
	tmpl := closechecklist.Template{
		TemplateID: "many", Version: "1.0",
		Sections: []closechecklist.SectionDefinition{{SectionCode: "A"}},
		Tasks:    tasks,
	}
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	if len(res.Tasks) != n {
		t.Fatalf("expected %d tasks, got %d", n, len(res.Tasks))
	}
	for _, tr := range res.Tasks {
		if tr.ReportedStatus != closechecklist.TaskNotStarted {
			t.Fatalf("task %s reported_status = %s, want NOT_STARTED", tr.TaskCode, tr.ReportedStatus)
		}
	}
	if res.Readiness != closechecklist.ChecklistNotStarted {
		t.Errorf("readiness = %s, want NOT_STARTED", res.Readiness)
	}
}

// TestSafety_ManyEvidenceAndSignOffsAndGates exercises many evidence
// refs, sign-offs, and gate facts on a single task — section 47.
func TestSafety_ManyEvidenceAndSignOffsAndGates(t *testing.T) {
	const n = 2000
	var evidence []closechecklist.EvidenceRef
	for i := 0; i < n; i++ {
		evidence = append(evidence, closechecklist.EvidenceRef{EvidenceID: fmt.Sprintf("E%d", i), Type: "WORKPAPER"})
	}
	var signOffs []closechecklist.SignOff
	for i := 0; i < n; i++ {
		signOffs = append(signOffs, closechecklist.SignOff{
			SignOffID: fmt.Sprintf("S%d", i), Role: closechecklist.SignOffReviewer,
			ActorRef: fmt.Sprintf("actor%d", i), SignedAt: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		})
	}
	var gateFacts []closechecklist.GateFact
	var gateRules []closechecklist.GateRule
	for i := 0; i < n/2; i++ {
		code := fmt.Sprintf("gate%d", i)
		gateFacts = append(gateFacts, closechecklist.GateFact{GateCode: code, Status: closechecklist.GatePass})
		gateRules = append(gateRules, closechecklist.GateRule{GateCode: code, Require: closechecklist.GateRequirePass, Required: false})
	}

	tmpl := closechecklist.Template{
		TemplateID: "heavy", Version: "1.0",
		Sections: []closechecklist.SectionDefinition{{SectionCode: "A"}},
		Tasks: []closechecklist.TaskDefinition{
			{
				TaskCode: "t1", SectionCode: "A", Required: true,
				Applicability:  closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				EvidencePolicy: closechecklist.EvidencePolicy{Type: closechecklist.EvidenceMinCount, MinCount: 1},
				ReviewPolicy:   closechecklist.ReviewPolicy{Type: closechecklist.ReviewSpecificRoles, RequiredRoles: []closechecklist.SignOffRole{closechecklist.SignOffReviewer}},
				GateRules:      gateRules,
			},
		},
	}
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{{TaskCode: "t1", Status: closechecklist.TaskCompleted, Evidence: evidence, SignOffs: signOffs}},
		Gates:      gateFacts,
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	t1 := taskResultByCode(res, "t1")
	if !t1.Satisfied {
		t.Fatalf("t1 not satisfied with abundant evidence/sign-offs/gates")
	}
}

// TestSafety_DuplicateIDs exercises duplicate task states, evidence,
// sign-offs, gate facts, and exceptions — every one must resolve to a
// reported Issue and a deterministic (first-wins) outcome, never a
// panic.
func TestSafety_DuplicateIDs(t *testing.T) {
	tmpl := simpleTemplate()
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "t1", Status: closechecklist.TaskCompleted},
			{TaskCode: "t1", Status: closechecklist.TaskNotStarted}, // duplicate, ignored
		},
		Gates: []closechecklist.GateFact{
			{GateCode: "g1", Status: closechecklist.GatePass},
			{GateCode: "g1", Status: closechecklist.GateFail}, // duplicate, ignored
		},
		Exceptions: []closechecklist.Exception{
			{ExceptionID: "EX-1", TaskCode: "t1", ApprovedAt: date("2025-01-01")},
			{ExceptionID: "EX-1", TaskCode: "t1", ApprovedAt: date("2025-01-02")}, // duplicate, ignored
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())

	wantCodes := map[closechecklist.IssueCode]bool{
		closechecklist.IssueDuplicateTaskState: false,
		closechecklist.IssueDuplicateGateFact:  false,
		closechecklist.IssueDuplicateException: false,
	}
	for _, iss := range res.Issues {
		if _, ok := wantCodes[iss.Code]; ok {
			wantCodes[iss.Code] = true
		}
	}
	for code, found := range wantCodes {
		if !found {
			t.Errorf("expected issue code %s, got %+v", code, res.Issues)
		}
	}
}
