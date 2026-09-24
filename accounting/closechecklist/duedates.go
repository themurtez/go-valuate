package closechecklist

import "time"

// DueRuleType is the fixed set of due-date anchoring kinds a task may
// declare.
type DueRuleType string

const (
	DueRuleNone         DueRuleType = ""
	DueRulePeriodEnd    DueRuleType = "PERIOD_END"
	DueRuleTargetClose  DueRuleType = "TARGET_CLOSE_DATE"
	DueRuleExplicitDate DueRuleType = "EXPLICIT_DATE"
)

func isRecognizedDueRuleType(t DueRuleType) bool {
	switch t {
	case DueRuleNone, DueRulePeriodEnd, DueRuleTargetClose, DueRuleExplicitDate:
		return true
	default:
		return false
	}
}

// DueRule declares how one task's due date is computed. OffsetDays is
// calendar-day (not business-day) arithmetic added to the anchor date —
// see the package spec's "calendar-day behavior is sufficient for V1"
// note. A negative OffsetDays is valid (due before the anchor).
type DueRule struct {
	Type         DueRuleType `json:"type,omitempty"`
	OffsetDays   int         `json:"offset_days,omitempty"`
	ExplicitDate time.Time   `json:"explicit_date,omitempty"`
}

// resolveDueDate computes r's concrete due date given the period's
// anchors. Returns (date, true) when resolvable, (zero, false) when
// unresolvable (DueRuleNone, or an anchor rule whose anchor date is
// itself zero).
func resolveDueDate(r DueRule, p Period) (time.Time, bool) {
	switch r.Type {
	case DueRulePeriodEnd:
		if isZeroTime(p.EndDate) {
			return time.Time{}, false
		}
		return p.EndDate.AddDate(0, 0, r.OffsetDays), true
	case DueRuleTargetClose:
		if isZeroTime(p.TargetCloseDate) {
			return time.Time{}, false
		}
		return p.TargetCloseDate.AddDate(0, 0, r.OffsetDays), true
	case DueRuleExplicitDate:
		if isZeroTime(r.ExplicitDate) {
			return time.Time{}, false
		}
		return r.ExplicitDate, true
	default:
		return time.Time{}, false
	}
}

// DueStatus is the factual due-state of one task as of a caller-supplied
// EvaluationDate. Never computed via time.Now().
type DueStatus string

const (
	DueStatusNotDue          DueStatus = "NOT_DUE"
	DueStatusDueToday        DueStatus = "DUE_TODAY"
	DueStatusOverdue         DueStatus = "OVERDUE"
	DueStatusCompletedOnTime DueStatus = "COMPLETED_ON_TIME"
	DueStatusCompletedLate   DueStatus = "COMPLETED_LATE"
	DueStatusUnavailable     DueStatus = "UNAVAILABLE"
)

// computeDueStatus derives DueStatus for one task. completedAt is nil
// when the task has not been completed. evaluationDate must be non-zero
// for any non-UNAVAILABLE result (required explicit EvaluationDate, per
// section 10).
func computeDueStatus(dueDate time.Time, dueResolved bool, completedAt *time.Time, evaluationDate time.Time) DueStatus {
	if !dueResolved || isZeroTime(evaluationDate) {
		return DueStatusUnavailable
	}
	dueDay := truncateDay(dueDate)
	evalDay := truncateDay(evaluationDate)

	if completedAt != nil {
		completedDay := truncateDay(*completedAt)
		if !completedDay.After(dueDay) {
			return DueStatusCompletedOnTime
		}
		return DueStatusCompletedLate
	}

	switch {
	case evalDay.Before(dueDay):
		return DueStatusNotDue
	case evalDay.Equal(dueDay):
		return DueStatusDueToday
	default:
		return DueStatusOverdue
	}
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
