package closechecklist

import "time"

// TaskStatus is the caller-reported status of one task, as tracked by
// the caller's own workflow. This package never overwrites a
// caller-reported TaskStatus — see [TaskResult.ReportedStatus] and
// section 15's "Do not overwrite caller-reported status."
type TaskStatus string

const (
	TaskNotStarted    TaskStatus = "NOT_STARTED"
	TaskInProgress    TaskStatus = "IN_PROGRESS"
	TaskCompleted     TaskStatus = "COMPLETED"
	TaskBlocked       TaskStatus = "BLOCKED"
	TaskSkipped       TaskStatus = "SKIPPED"
	TaskNotApplicable TaskStatus = "NOT_APPLICABLE"
)

func isRecognizedTaskStatus(s TaskStatus) bool {
	switch s {
	case "", TaskNotStarted, TaskInProgress, TaskCompleted, TaskBlocked, TaskSkipped, TaskNotApplicable:
		return true
	default:
		return false
	}
}

// TaskState is the caller-supplied, period-specific state of one task.
// Missing state for an applicable template task deterministically means
// NOT_STARTED — see [defaultTaskState] — so a caller never needs to
// pre-populate a TaskState for every task before an instance is usable.
type TaskState struct {
	TaskCode string     `json:"task_code"`
	Status   TaskStatus `json:"status"`

	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// OwnerRef is opaque — no user-management logic.
	OwnerRef string `json:"owner_ref,omitempty"`

	Evidence []EvidenceRef `json:"evidence,omitempty"`
	SignOffs []SignOff     `json:"sign_offs,omitempty"`

	Note string `json:"note,omitempty"`
}

// defaultTaskState returns the deterministic NOT_STARTED default used
// when a Template task has no corresponding TaskState in the Instance
// (section 5's "Missing state ... should deterministically mean
// NOT_STARTED").
func defaultTaskState(taskCode string) TaskState {
	return TaskState{TaskCode: taskCode, Status: TaskNotStarted}
}
