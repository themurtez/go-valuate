package closequality

// Status is the overall close-readiness classification. See readiness.go
// for the exact, documented decision rule.
type Status string

const (
	StatusReady             Status = "READY"
	StatusReadyWithWarnings Status = "READY_WITH_WARNINGS"
	StatusNotReady          Status = "NOT_READY"
	StatusUnassessed        Status = "UNASSESSED"
)

// CloseTaskSummary aggregates Input.CloseTasks by status. Counts are
// disjoint (every task is counted exactly once); CompletionPercent is
// computed over Required tasks only (0 when RequiredCount == 0).
type CloseTaskSummary struct {
	TotalCount             int     `json:"total_count"`
	RequiredCount          int     `json:"required_count"`
	CompletedCount         int     `json:"completed_count"`
	InProgressCount        int     `json:"in_progress_count"`
	NotStartedCount        int     `json:"not_started_count"`
	BlockedCount           int     `json:"blocked_count"`
	NotApplicableCount     int     `json:"not_applicable_count"`
	RequiredCompletedCount int     `json:"required_completed_count"`
	CompletionPercent      float64 `json:"completion_percent"`
}

// Result is this package's top-level output for one period.
type Result struct {
	Period            PeriodInfo        `json:"period"`
	Status            Status            `json:"status"`
	Coverage          Coverage          `json:"coverage"`
	DimensionCoverage DimensionCoverage `json:"dimension_coverage"`

	Dimensions []DimensionResult `json:"dimensions"`

	Blockers    []Finding `json:"blockers,omitempty"`
	Warnings    []Finding `json:"warnings,omitempty"`
	Information []Finding `json:"information,omitempty"`

	MissingInputs   []string  `json:"missing_inputs,omitempty"`
	UnresolvedItems []Finding `json:"unresolved_items,omitempty"`

	CloseTaskSummary CloseTaskSummary `json:"close_task_summary"`

	Comparison *ComparisonResult `json:"comparison,omitempty"`

	Versions Versions `json:"versions"`
	Issues   []Issue  `json:"issues,omitempty"`
}
