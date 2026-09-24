package profitability

// Input bundles every caller-supplied source Calculate needs. Only
// Periods is effectively required for any period-scoped output; entities/
// facts/summaries/pools/rules/controls are all optional and Calculate
// computes whatever subset of the analysis the supplied input supports.
type Input struct {
	// Periods is the explicit set of periods to analyze. Required for any
	// EntityPeriodResult/BusinessTotals output.
	Periods []PeriodInfo `json:"periods"`

	// Entities is the customer/job/product master list. Required for a
	// Fact's Attribution.EntityID to resolve to a known entity —
	// IssueUnknownEntity is raised for an Attribution referencing an
	// EntityID absent here.
	Entities []Entity `json:"entities,omitempty"`

	// Facts is the detailed economic-fact population — see Fact. Mutually
	// exclusive per (Dimension, EntityID, Period) with
	// EntityPeriodSummaries — see IssueInputModeConflict.
	Facts []Fact `json:"facts,omitempty"`

	// EntityPeriodSummaries is the caller-pre-aggregated path — see
	// EntityPeriodSummaryInput.
	EntityPeriodSummaries []EntityPeriodSummaryInput `json:"entity_period_summaries,omitempty"`

	// Drivers is optional activity-driver data — see DriverObservation.
	Drivers []DriverObservation `json:"drivers,omitempty"`

	// SharedCostPools is optional caller-declared overhead pools — see
	// SharedCostPool. Never allocated unless AllocationRules supplies a
	// matching rule — allocation is opt-in (task section 21).
	SharedCostPools []SharedCostPool `json:"shared_cost_pools,omitempty"`

	// AllocationRules is optional caller-declared allocation policy — see
	// AllocationRule.
	AllocationRules []AllocationRule `json:"allocation_rules,omitempty"`

	// Controls is optional per-period business/GL control totals — see
	// ControlTotals.
	Controls []ControlTotals `json:"controls,omitempty"`
}
