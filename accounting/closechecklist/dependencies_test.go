package closechecklist_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
)

func TestDependencies_UnknownDependency_ReportsIssue(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[1].Dependencies = []closechecklist.TaskDependency{{DependsOnTaskCode: "ghost"}}
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	found := false
	for _, iss := range res.Issues {
		if iss.Code == closechecklist.IssueUnknownDependency {
			found = true
		}
	}
	if !found {
		t.Errorf("expected UNKNOWN_DEPENDENCY issue, got %+v", res.Issues)
	}
}

func TestDependencies_SelfDependency_ReportsIssue(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[0].Dependencies = []closechecklist.TaskDependency{{DependsOnTaskCode: "t1"}}
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	found := false
	for _, iss := range res.Issues {
		if iss.Code == closechecklist.IssueSelfDependency {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SELF_DEPENDENCY issue, got %+v", res.Issues)
	}
}

func TestDependencies_DuplicateDependency_ReportsIssue(t *testing.T) {
	tmpl := simpleTemplate()
	tmpl.Tasks[1].Dependencies = []closechecklist.TaskDependency{
		{DependsOnTaskCode: "t1"}, {DependsOnTaskCode: "t1"},
	}
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	found := false
	for _, iss := range res.Issues {
		if iss.Code == closechecklist.IssueDuplicateDependency {
			found = true
		}
	}
	if !found {
		t.Errorf("expected DUPLICATE_DEPENDENCY issue, got %+v", res.Issues)
	}
}

// TestDependencies_Cycle_NeverSilentlyBroken — section 7: a cycle must
// be reported (DEPENDENCY_CYCLE Issue) and every task inside the cycle
// must never resolve ReadyToStart/Satisfied via its cyclic dependency —
// no infinite loop, no silently-broken edge, no impossible READY_TO_CLOSE.
func TestDependencies_Cycle_NeverSilentlyBroken(t *testing.T) {
	tmpl := closechecklist.Template{
		TemplateID: "cyclic", Version: "1.0",
		Sections: []closechecklist.SectionDefinition{{SectionCode: "A"}},
		Tasks: []closechecklist.TaskDefinition{
			{TaskCode: "a", SectionCode: "A", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				Dependencies: []closechecklist.TaskDependency{{DependsOnTaskCode: "b"}}},
			{TaskCode: "b", SectionCode: "A", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				Dependencies: []closechecklist.TaskDependency{{DependsOnTaskCode: "c"}}},
			{TaskCode: "c", SectionCode: "A", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				Dependencies: []closechecklist.TaskDependency{{DependsOnTaskCode: "a"}}},
		},
	}
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		TaskStates: []closechecklist.TaskState{
			{TaskCode: "a", Status: closechecklist.TaskCompleted},
			{TaskCode: "b", Status: closechecklist.TaskCompleted},
			{TaskCode: "c", Status: closechecklist.TaskCompleted},
		},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())

	foundCycle := false
	for _, iss := range res.Issues {
		if iss.Code == closechecklist.IssueDependencyCycle {
			foundCycle = true
		}
	}
	if !foundCycle {
		t.Fatalf("expected DEPENDENCY_CYCLE issue, got %+v", res.Issues)
	}
	if res.Readiness == closechecklist.ChecklistReadyToClose {
		t.Errorf("readiness = READY_TO_CLOSE, want not ready: a cycle can never be satisfied")
	}
	for _, tr := range res.Tasks {
		if tr.Satisfied {
			t.Errorf("task %s satisfied = true inside a dependency cycle, want false", tr.TaskCode)
		}
	}
}

// TestDependencies_OrderIndependence — section 43: shuffling task
// definition order must never change readiness semantics.
func TestDependencies_OrderIndependence(t *testing.T) {
	tmpl1 := simpleTemplate()
	tmpl2 := simpleTemplate()
	tmpl2.Tasks[0], tmpl2.Tasks[1] = tmpl2.Tasks[1], tmpl2.Tasks[0]

	states := []closechecklist.TaskState{
		{TaskCode: "t2", Status: closechecklist.TaskCompleted}, // t1 not completed
	}

	res1 := closechecklist.Calculate(closechecklist.Instance{Template: tmpl1, Period: simplePeriod(), EvaluationDate: date("2025-02-01"), TaskStates: states}, closechecklist.DefaultPolicy())
	res2 := closechecklist.Calculate(closechecklist.Instance{Template: tmpl2, Period: simplePeriod(), EvaluationDate: date("2025-02-01"), TaskStates: states}, closechecklist.DefaultPolicy())

	if res1.Readiness != res2.Readiness {
		t.Errorf("readiness differs by template task order: %s vs %s", res1.Readiness, res2.Readiness)
	}
	sat1 := map[string]bool{}
	for _, tr := range res1.Tasks {
		sat1[tr.TaskCode] = tr.Satisfied
	}
	for _, tr := range res2.Tasks {
		if tr.Satisfied != sat1[tr.TaskCode] {
			t.Errorf("task %s satisfied differs by template task order", tr.TaskCode)
		}
	}
}

// TestDependencies_MustBeResolved_AllowsNotApplicable exercises the
// MUST_BE_RESOLVED dependency type: a dependency that resolves
// NOT_APPLICABLE counts as resolved.
func TestDependencies_MustBeResolved_AllowsNotApplicable(t *testing.T) {
	tmpl := closechecklist.Template{
		TemplateID: "t", Version: "1.0",
		Sections: []closechecklist.SectionDefinition{{SectionCode: "A"}},
		Tasks: []closechecklist.TaskDefinition{
			{TaskCode: "inv_rec", SectionCode: "A", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityCallerFlag, FlagKey: "has_inventory"}},
			{TaskCode: "inv_adj", SectionCode: "A", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				Dependencies: []closechecklist.TaskDependency{{DependsOnTaskCode: "inv_rec", Type: closechecklist.DependencyMustBeResolved}}},
		},
	}
	in := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01"),
		ApplicabilityFlags: map[string]bool{"has_inventory": false},
		TaskStates:         []closechecklist.TaskState{{TaskCode: "inv_adj", Status: closechecklist.TaskCompleted}},
	}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	var invAdj closechecklist.TaskResult
	for _, tr := range res.Tasks {
		if tr.TaskCode == "inv_adj" {
			invAdj = tr
		}
	}
	if !invAdj.Satisfied {
		t.Errorf("inv_adj satisfied = false, want true: MUST_BE_RESOLVED dependency on a NOT_APPLICABLE task should resolve")
	}
}
