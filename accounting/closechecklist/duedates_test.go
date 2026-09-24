package closechecklist_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
)

func dueTemplate(rule closechecklist.DueRule) closechecklist.Template {
	return closechecklist.Template{
		TemplateID: "due", Version: "1.0",
		Sections: []closechecklist.SectionDefinition{{SectionCode: "A"}},
		Tasks: []closechecklist.TaskDefinition{
			{TaskCode: "t1", SectionCode: "A", Required: true, Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways}, DueRule: rule},
		},
	}
}

func TestDueDates_PeriodEnd_NotDue(t *testing.T) {
	tmpl := dueTemplate(closechecklist.DueRule{Type: closechecklist.DueRulePeriodEnd, OffsetDays: 3})
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-01-15")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	tr := taskResultByCode(res, "t1")
	if tr.DueStatus != closechecklist.DueStatusNotDue {
		t.Errorf("due_status = %s, want NOT_DUE", tr.DueStatus)
	}
	if tr.DueDate == nil || *tr.DueDate != "2025-02-03" {
		t.Errorf("due_date = %v, want 2025-02-03", tr.DueDate)
	}
}

func TestDueDates_PeriodEnd_DueToday(t *testing.T) {
	tmpl := dueTemplate(closechecklist.DueRule{Type: closechecklist.DueRulePeriodEnd, OffsetDays: 3})
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-03")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	tr := taskResultByCode(res, "t1")
	if tr.DueStatus != closechecklist.DueStatusDueToday {
		t.Errorf("due_status = %s, want DUE_TODAY", tr.DueStatus)
	}
}

func TestDueDates_PeriodEnd_Overdue(t *testing.T) {
	tmpl := dueTemplate(closechecklist.DueRule{Type: closechecklist.DueRulePeriodEnd, OffsetDays: 3})
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-10")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	tr := taskResultByCode(res, "t1")
	if tr.DueStatus != closechecklist.DueStatusOverdue {
		t.Errorf("due_status = %s, want OVERDUE", tr.DueStatus)
	}
}

func TestDueDates_CompletedOnTimeAndLate(t *testing.T) {
	tmpl := dueTemplate(closechecklist.DueRule{Type: closechecklist.DueRulePeriodEnd, OffsetDays: 3})

	onTime := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-10"),
		TaskStates: []closechecklist.TaskState{{TaskCode: "t1", Status: closechecklist.TaskCompleted, CompletedAt: timePtr(date("2025-02-02"))}},
	}
	res := closechecklist.Calculate(onTime, closechecklist.DefaultPolicy())
	tr := taskResultByCode(res, "t1")
	if tr.DueStatus != closechecklist.DueStatusCompletedOnTime {
		t.Errorf("due_status = %s, want COMPLETED_ON_TIME", tr.DueStatus)
	}

	late := closechecklist.Instance{
		Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-10"),
		TaskStates: []closechecklist.TaskState{{TaskCode: "t1", Status: closechecklist.TaskCompleted, CompletedAt: timePtr(date("2025-02-08"))}},
	}
	res2 := closechecklist.Calculate(late, closechecklist.DefaultPolicy())
	tr2 := taskResultByCode(res2, "t1")
	if tr2.DueStatus != closechecklist.DueStatusCompletedLate {
		t.Errorf("due_status = %s, want COMPLETED_LATE", tr2.DueStatus)
	}
}

func TestDueDates_TargetClose(t *testing.T) {
	tmpl := dueTemplate(closechecklist.DueRule{Type: closechecklist.DueRuleTargetClose})
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	tr := taskResultByCode(res, "t1")
	if tr.DueDate == nil || *tr.DueDate != "2025-02-05" {
		t.Errorf("due_date = %v, want 2025-02-05 (TargetCloseDate)", tr.DueDate)
	}
}

func TestDueDates_ExplicitDate(t *testing.T) {
	tmpl := dueTemplate(closechecklist.DueRule{Type: closechecklist.DueRuleExplicitDate, ExplicitDate: date("2025-03-01")})
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	tr := taskResultByCode(res, "t1")
	if tr.DueDate == nil || *tr.DueDate != "2025-03-01" {
		t.Errorf("due_date = %v, want 2025-03-01", tr.DueDate)
	}
}

func TestDueDates_NoEvaluationDate_Unavailable(t *testing.T) {
	tmpl := dueTemplate(closechecklist.DueRule{Type: closechecklist.DueRulePeriodEnd})
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod()} // no EvaluationDate
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	tr := taskResultByCode(res, "t1")
	if tr.DueStatus != closechecklist.DueStatusUnavailable {
		t.Errorf("due_status = %s, want UNAVAILABLE without an EvaluationDate", tr.DueStatus)
	}
}

func TestDueDates_NoRule_Unavailable(t *testing.T) {
	tmpl := dueTemplate(closechecklist.DueRule{})
	in := closechecklist.Instance{Template: tmpl, Period: simplePeriod(), EvaluationDate: date("2025-02-01")}
	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())
	tr := taskResultByCode(res, "t1")
	if tr.DueStatus != closechecklist.DueStatusUnavailable {
		t.Errorf("due_status = %s, want UNAVAILABLE with no due rule", tr.DueStatus)
	}
	if tr.DueDate != nil {
		t.Errorf("due_date should be nil with no due rule, got %v", tr.DueDate)
	}
}
