package kpi

// Input is Calculate's caller-supplied data — never mutated (see
// immutability_test.go).
type Input struct {
	// Definitions is every KPI to evaluate, in caller order (echoed only
	// by Coverage.DefinitionsSupplied's count; evaluation order is always
	// topological — see dependency.go).
	Definitions []Definition `json:"definitions"`
	// Metrics is every source fact available for METRIC references — see
	// metric.go.
	Metrics []MetricValue `json:"metrics"`
	// Periods is every period this analysis may evaluate against — see
	// period.go. Calculate evaluates only the periods listed in
	// Options.Periods (a subset, or all of Input.Periods if
	// Options.Periods is empty), but every Period referenced by a
	// PRIOR_PERIOD/PRIOR_YEAR_SAME_PERIOD/TRAILING_N lookup must still
	// appear here even if not itself a requested evaluation target.
	Periods []Period `json:"periods"`
}

// Options configures one Calculate call's caller-adjustable behavior that
// is not a fixed part of FormulaVersion.
type Options struct {
	// Periods restricts which Period.Code values are actually evaluated
	// (KPIResults produced) — task section 4's "evaluate each KPI
	// independently for each requested period" instruction. Empty means
	// every Input.Periods entry is evaluated.
	Periods []string `json:"periods,omitempty"`
	// Dimensions restricts which DimensionKey groups are evaluated,
	// beyond the always-included business-level (no-dimension) group.
	// Empty means only the business-level group is evaluated — task
	// section 5's strict-default rule extended to which groups Calculate
	// itself produces output for (a caller must opt into a dimensioned
	// KPIResult set explicitly, mirroring MetricRef's own explicit-
	// broadcast convention).
	Dimensions []DimensionKey `json:"dimensions,omitempty"`
	// IncludeTrace, if true, populates each KPIResult.Trace — task
	// section 25. Default false to avoid unnecessary payload.
	IncludeTrace bool `json:"include_trace,omitempty"`
	// IncludeTrend, if true, populates each KPIResult.Trend across every
	// evaluated period — task section 24.
	IncludeTrend bool `json:"include_trend,omitempty"`
	// TrendStabilityTolerance is the absolute tolerance (in the KPI's own
	// Unit) Trend.Direction uses — see resolveTrendDirection. Zero means
	// any nonzero change counts as INCREASING/DECREASING.
	TrendStabilityTolerance float64 `json:"trend_stability_tolerance,omitempty"`
	// FailAllOnDefinitionError, if true, makes Calculate return zero
	// KPIResults entirely (Available == false) when ANY Definition fails
	// validation, instead of the default lenient mode where an invalid
	// Definition only excludes itself (and anything depending on it) —
	// task section 15's optional policy, default false.
	FailAllOnDefinitionError bool `json:"fail_all_on_definition_error,omitempty"`
	// ExpressionLanguageVersion, if non-empty, must equal
	// ExpressionLanguageVersion or every Definition is rejected with
	// IssueUnsupportedExpressionLanguageVersion. Empty means "this
	// engine's current version," the common case.
	ExpressionLanguageVersion string `json:"expression_language_version,omitempty"`
}
