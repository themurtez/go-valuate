package closechecklist

// Blocker is one typed, explainable top-level blocking condition —
// section 21. No prose parsing: a caller can filter/group on ReasonCode
// alone.
type Blocker struct {
	TaskCode   string        `json:"task_code"`
	ReasonCode BlockerReason `json:"reason_code"`
	Message    string        `json:"message"`

	DependencyTaskCode  string `json:"dependency_task_code,omitempty"`
	GateCode            string `json:"gate_code,omitempty"`
	MissingEvidenceType string `json:"missing_evidence_type,omitempty"`
	MissingSignOffRole  string `json:"missing_sign_off_role,omitempty"`

	// ExceptionApplied/EffectiveBlocking mirror the same-named
	// TaskResult fields for this specific blocker — see section 22: a
	// valid exception never deletes the underlying blocker, only marks
	// it non-effective.
	ExceptionApplied  bool `json:"exception_applied"`
	EffectiveBlocking bool `json:"effective_blocking"`
}

// BlockerReason is the fixed set of top-level blocker reason codes.
type BlockerReason string

const (
	BlockerDependencyIncomplete BlockerReason = "DEPENDENCY_INCOMPLETE"
	BlockerGateFailed           BlockerReason = "GATE_FAILED"
	BlockerGateUnavailable      BlockerReason = "GATE_UNAVAILABLE"
	BlockerEvidenceMissing      BlockerReason = "EVIDENCE_MISSING"
	BlockerSignOffMissing       BlockerReason = "SIGNOFF_MISSING"
	BlockerCallerMarkedBlocked  BlockerReason = "CALLER_MARKED_BLOCKED"
	BlockerExceptionExpired     BlockerReason = "EXCEPTION_EXPIRED"
)

// TaskReadiness is the computed readiness status for one checklist
// instance — kept separate from the caller-reported TaskStatus.
type TaskReadiness string

const (
	ReadinessInvalid              TaskReadiness = "INVALID"
	ReadinessNotStarted           TaskReadiness = "NOT_STARTED"
	ReadinessInProgress           TaskReadiness = "IN_PROGRESS"
	ReadinessReadyForReview       TaskReadiness = "READY_FOR_REVIEW"
	ReadinessReadyToClose         TaskReadiness = "READY_TO_CLOSE"
	ReadinessClosed               TaskReadiness = "CLOSED"
	ReadinessClosedWithExceptions TaskReadiness = "CLOSED_WITH_EXCEPTIONS"
)

// TaskResult is one applicable-or-not task's full computed readiness —
// section 15. ReportedStatus is always the caller's own TaskState.Status
// (or the NOT_STARTED default), never overwritten.
type TaskResult struct {
	TaskCode    string `json:"task_code"`
	SectionCode string `json:"section_code"`

	Applicability Applicability `json:"applicability"`
	Required      bool          `json:"required"`

	ReportedStatus TaskStatus `json:"reported_status"`

	ReadyToStart    bool `json:"ready_to_start"`
	ReadyToComplete bool `json:"ready_to_complete"`
	Satisfied       bool `json:"satisfied"`

	EvidenceStatus TaskEvidenceStatus `json:"evidence_status"`
	ReviewStatus   TaskReviewStatus   `json:"review_status"`
	DueStatus      DueStatus          `json:"due_status"`

	DueDate     *string `json:"due_date,omitempty"`
	CompletedAt *string `json:"completed_at,omitempty"`

	Blockers []Blocker `json:"blockers,omitempty"`
	Findings []Finding `json:"findings,omitempty"`

	// ExceptionApplied/EffectiveBlocking summarize this task's own
	// exception state, if any (see exceptions.go).
	ExceptionApplied  bool `json:"exception_applied"`
	EffectiveBlocking bool `json:"effective_blocking"`

	// DownstreamRequiredTaskCount is the count of other Required
	// applicable tasks that (transitively) depend on this task — see
	// bottleneck.go, section 28.
	DownstreamRequiredTaskCount int `json:"downstream_required_task_count"`
}

// TaskEvidenceStatus is the factual evidence-coverage state for one task.
type TaskEvidenceStatus string

const (
	EvidenceStatusNotRequired TaskEvidenceStatus = "NOT_REQUIRED"
	EvidenceStatusSatisfied   TaskEvidenceStatus = "SATISFIED"
	EvidenceStatusMissing     TaskEvidenceStatus = "MISSING"
)

// TaskReviewStatus is the factual sign-off/review state for one task.
type TaskReviewStatus string

const (
	ReviewStatusNotRequired TaskReviewStatus = "NOT_REQUIRED"
	ReviewStatusSatisfied   TaskReviewStatus = "SATISFIED"
	ReviewStatusMissing     TaskReviewStatus = "MISSING"
	ReviewStatusConflict    TaskReviewStatus = "CONFLICT"
)

// SectionStatus is one section's own rollup status.
type SectionStatus string

const (
	SectionNotStarted     SectionStatus = "NOT_STARTED"
	SectionInProgress     SectionStatus = "IN_PROGRESS"
	SectionReadyForReview SectionStatus = "READY_FOR_REVIEW"
	SectionComplete       SectionStatus = "COMPLETE"
)

// SectionResult aggregates one section's tasks — section 19.
type SectionResult struct {
	SectionCode string `json:"section_code"`
	Name        string `json:"name,omitempty"`

	ApplicableTasks int `json:"applicable_tasks"`
	RequiredTasks   int `json:"required_tasks"`
	SatisfiedTasks  int `json:"satisfied_tasks"`
	InProgressTasks int `json:"in_progress_tasks"`
	BlockedTasks    int `json:"blocked_tasks"`
	OverdueTasks    int `json:"overdue_tasks"`

	CompletionPercent float64       `json:"completion_percent"`
	Status            SectionStatus `json:"status"`
}

// Completion is the top-level completion summary — section 19.
type Completion struct {
	ApplicableTaskCount          int `json:"applicable_task_count"`
	RequiredTaskCount            int `json:"required_task_count"`
	SatisfiedRequiredTaskCount   int `json:"satisfied_required_task_count"`
	UnsatisfiedRequiredTaskCount int `json:"unsatisfied_required_task_count"`
	OptionalTaskCount            int `json:"optional_task_count"`
	BlockedTaskCount             int `json:"blocked_task_count"`
	OverdueTaskCount             int `json:"overdue_task_count"`

	RequiredCompletionPercent float64 `json:"required_completion_percent"`
	OverallCompletionPercent  float64 `json:"overall_completion_percent"`
}

// EvidenceCoverage is factual evidence coverage across every applicable
// task — section 20.
type EvidenceCoverage struct {
	RequiredEvidenceCount   int     `json:"required_evidence_count"`
	PresentEvidenceCount    int     `json:"present_evidence_count"`
	MissingEvidenceCount    int     `json:"missing_evidence_count"`
	EvidenceCoveragePercent float64 `json:"evidence_coverage_percent"`
}

// SignOffCoverage is factual sign-off coverage across every applicable
// task — section 20.
type SignOffCoverage struct {
	RequiredSignOffCount    int     `json:"required_sign_off_count"`
	ValidSignOffCount       int     `json:"valid_sign_off_count"`
	MissingSignOffCount     int     `json:"missing_sign_off_count"`
	ConflictingSignOffCount int     `json:"conflicting_sign_off_count"`
	SignOffCoveragePercent  float64 `json:"sign_off_coverage_percent"`
}

// GateCoverage is factual gate coverage across every applicable task's
// GateRules — section 20.
type GateCoverage struct {
	RequiredGateCount  int `json:"required_gate_count"`
	PassCount          int `json:"pass_count"`
	WarningCount       int `json:"warning_count"`
	FailCount          int `json:"fail_count"`
	UnavailableCount   int `json:"unavailable_count"`
	NotApplicableCount int `json:"not_applicable_count"`
}

// TimingAnalytics is section 27's factual timing summary.
type TimingAnalytics struct {
	OnTimeTaskCount  int     `json:"on_time_task_count"`
	LateTaskCount    int     `json:"late_task_count"`
	AverageDaysLate  float64 `json:"average_days_late"`
	MaxDaysLate      int     `json:"max_days_late"`
	DaysToClose      *int    `json:"days_to_close,omitempty"`
	DaysLateVsTarget *int    `json:"days_late_vs_target,omitempty"`
}

// BottleneckSummary is section 28's factual bottleneck counts.
type BottleneckSummary struct {
	BlockedTaskCount     int `json:"blocked_task_count"`
	TasksBlockingOthers  int `json:"tasks_blocking_others"`
	ExternalGateBlockers int `json:"external_gate_blockers"`
	EvidenceBlockers     int `json:"evidence_blockers"`
	SignOffBlockers      int `json:"sign_off_blockers"`
}

// PeriodConsistencyFinding is a factual period-state consistency check
// result — section 23.
type PeriodConsistencyFinding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Result is this package's top-level output for one close [Instance].
type Result struct {
	PeriodID        string `json:"period_id"`
	TemplateID      string `json:"template_id"`
	TemplateVersion string `json:"template_version"`

	Readiness ChecklistReadiness `json:"readiness"`

	Sections []SectionResult `json:"sections"`
	Tasks    []TaskResult    `json:"tasks"`

	Completion       Completion       `json:"completion"`
	EvidenceCoverage EvidenceCoverage `json:"evidence_coverage"`
	SignOffCoverage  SignOffCoverage  `json:"sign_off_coverage"`
	GateCoverage     GateCoverage     `json:"gate_coverage"`

	Blockers []Blocker `json:"blockers,omitempty"`
	Findings []Finding `json:"findings,omitempty"`

	PeriodConsistency []PeriodConsistencyFinding `json:"period_consistency,omitempty"`

	TimingAnalytics TimingAnalytics   `json:"timing_analytics"`
	Bottlenecks     BottleneckSummary `json:"bottlenecks"`

	PriorCloseComparison      *PriorCloseComparison      `json:"prior_close_comparison,omitempty"`
	TemplateVersionComparison *TemplateVersionComparison `json:"template_version_comparison,omitempty"`

	Versions Versions `json:"versions"`
	Issues   []Issue  `json:"issues,omitempty"`
}

// ChecklistReadiness is the checklist-wide computed readiness state —
// section 18.
type ChecklistReadiness string

const (
	ChecklistInvalid              ChecklistReadiness = "INVALID"
	ChecklistNotStarted           ChecklistReadiness = "NOT_STARTED"
	ChecklistInProgress           ChecklistReadiness = "IN_PROGRESS"
	ChecklistReadyForReview       ChecklistReadiness = "READY_FOR_REVIEW"
	ChecklistReadyToClose         ChecklistReadiness = "READY_TO_CLOSE"
	ChecklistClosed               ChecklistReadiness = "CLOSED"
	ChecklistClosedWithExceptions ChecklistReadiness = "CLOSED_WITH_EXCEPTIONS"
)
