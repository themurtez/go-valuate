package closechecklist

// FindingSeverity is a Finding's review priority — never a probability
// or a performance judgment.
type FindingSeverity string

const (
	FindingSeverityBlocking FindingSeverity = "BLOCKING"
	FindingSeverityWarning  FindingSeverity = "WARNING"
	FindingSeverityInfo     FindingSeverity = "INFO"
)

var findingSeverityRank = map[FindingSeverity]int{
	FindingSeverityBlocking: 0,
	FindingSeverityWarning:  1,
	FindingSeverityInfo:     2,
}

// FindingCode is a stable identifier for one valid close-checklist
// condition needing attention. Only codes this package actually emits
// are defined here.
type FindingCode string

const (
	FindingRequiredTaskIncomplete                        FindingCode = "REQUIRED_TASK_INCOMPLETE"
	FindingTaskBlockedByDependency                       FindingCode = "TASK_BLOCKED_BY_DEPENDENCY"
	FindingTaskBlockedByGate                             FindingCode = "TASK_BLOCKED_BY_GATE"
	FindingRequiredEvidenceMissing                       FindingCode = "REQUIRED_EVIDENCE_MISSING"
	FindingRequiredSignOffMissing                        FindingCode = "REQUIRED_SIGNOFF_MISSING"
	FindingSignOffRoleConflict                           FindingCode = "SIGNOFF_ROLE_CONFLICT"
	FindingTaskMarkedCompleteWithMissingRequirements     FindingCode = "TASK_MARKED_COMPLETE_WITH_MISSING_REQUIREMENTS"
	FindingRequiredTaskSkipped                           FindingCode = "REQUIRED_TASK_SKIPPED"
	FindingTaskOverdue                                   FindingCode = "TASK_OVERDUE"
	FindingTaskCompletedLate                             FindingCode = "TASK_COMPLETED_LATE"
	FindingTaskCompletedBeforeDependency                 FindingCode = "TASK_COMPLETED_BEFORE_DEPENDENCY"
	FindingSignOffBeforeTaskCompletion                   FindingCode = "SIGNOFF_BEFORE_TASK_COMPLETION"
	FindingPeriodMarkedClosedWithIncompleteRequiredTasks FindingCode = "PERIOD_MARKED_CLOSED_WITH_INCOMPLETE_REQUIRED_TASKS"
	FindingValidExceptionApplied                         FindingCode = "VALID_EXCEPTION_APPLIED"
	FindingExpiredException                              FindingCode = "EXPIRED_EXCEPTION"
	FindingExternalGateWarning                           FindingCode = "EXTERNAL_GATE_WARNING"
)

// findingOrder is the fixed declaration/output order for FindingCode
// used as a sort tiebreak — never Go map order.
var findingOrder = []FindingCode{
	FindingRequiredTaskIncomplete,
	FindingTaskBlockedByDependency,
	FindingTaskBlockedByGate,
	FindingRequiredEvidenceMissing,
	FindingRequiredSignOffMissing,
	FindingSignOffRoleConflict,
	FindingTaskMarkedCompleteWithMissingRequirements,
	FindingRequiredTaskSkipped,
	FindingTaskOverdue,
	FindingTaskCompletedLate,
	FindingTaskCompletedBeforeDependency,
	FindingSignOffBeforeTaskCompletion,
	FindingPeriodMarkedClosedWithIncompleteRequiredTasks,
	FindingValidExceptionApplied,
	FindingExpiredException,
	FindingExternalGateWarning,
}

var findingRank = func() map[FindingCode]int {
	m := make(map[FindingCode]int, len(findingOrder))
	for i, c := range findingOrder {
		m[c] = i
	}
	return m
}()

// Finding is one factual, neutrally-worded close-checklist condition.
// Message text is always factual (see docs/PERIOD_CLOSE_CHECKLIST.md's
// neutral-language section and neutrallanguage_test.go) — never a
// performance or misconduct judgment.
type Finding struct {
	Code     FindingCode     `json:"code"`
	Severity FindingSeverity `json:"severity"`
	Message  string          `json:"message"`
	TaskCode string          `json:"task_code,omitempty"`
	Ref      string          `json:"ref,omitempty"`
}
