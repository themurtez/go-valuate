package kpi

// Coverage returns factual counts only — task section 39's explicit "no
// opaque health score" instruction. Every field is a plain count a caller
// can act on directly; this package never combines them into a single
// score or verdict.
type Coverage struct {
	DefinitionsSupplied int `json:"definitions_supplied"`
	DefinitionsValid    int `json:"definitions_valid"`
	DefinitionsInvalid  int `json:"definitions_invalid"`

	KPIsEvaluated int `json:"kpis_evaluated"`

	AvailableValues   int `json:"available_values"`
	UnavailableValues int `json:"unavailable_values"`

	// MissingSourceMetricCount is the number of distinct (metric code,
	// period, dimension) combinations referenced by at least one formula
	// but not found among the supplied MetricValues.
	MissingSourceMetricCount int `json:"missing_source_metric_count"`

	PeriodsEvaluated         int `json:"periods_evaluated"`
	DimensionGroupsEvaluated int `json:"dimension_groups_evaluated"`
}
