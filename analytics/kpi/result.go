package kpi

// Provenance carries a KPIResult's source lineage — task section 25's
// "even without full trace, preserve SourceMetricCodes/
// DependencyKPICodes/SourceRefs" instruction. Always populated,
// independent of Options.IncludeTrace.
type Provenance struct {
	// SourceMetricCodes is every distinct MetricValue.Code this KPI's
	// formula (transitively, through any KPI dependency) referenced for
	// this specific (period, dimension) evaluation, sorted.
	SourceMetricCodes []string `json:"source_metric_codes,omitempty"`
	// DependencyKPICodes is every distinct KPI Code this KPI's formula
	// referenced (transitively), sorted.
	DependencyKPICodes []string `json:"dependency_kpi_codes,omitempty"`
	// SourceRefs is every distinct, non-empty MetricValue.SourceRef
	// actually resolved for this evaluation, sorted.
	SourceRefs []string `json:"source_refs,omitempty"`
}

// KPIResult is one KPI's evaluated outcome for one (period, dimension)
// combination — task section 38.
type KPIResult struct {
	Code       string       `json:"code"`
	Period     string       `json:"period"`
	Dimensions DimensionKey `json:"dimensions,omitempty"`

	Value Value `json:"value"`
	Unit  Unit  `json:"unit"`

	TargetEvaluation TargetEvaluation `json:"target_evaluation"`
	// Band is the caller-defined ThresholdBand this Value.Amount falls
	// into, if Definition.ThresholdBands was configured and Value is
	// available. BandAvailable distinguishes "no band configured"/"value
	// unavailable" from a genuinely computed band match.
	Band          ThresholdBand `json:"band,omitempty"`
	BandAvailable bool          `json:"band_available"`

	Change Change `json:"change"`
	Trend  *Trend `json:"trend,omitempty"`

	Provenance Provenance `json:"provenance"`
	Trace      *Trace     `json:"trace,omitempty"`
}

// Result is the output of Calculate — task section 38.
type Result struct {
	SchemaVersion             string `json:"schema_version"`
	FormulaVersion            string `json:"formula_version"`
	ExpressionLanguageVersion string `json:"expression_language_version"`

	KPIResults []KPIResult `json:"kpi_results,omitempty"`

	Coverage Coverage `json:"coverage"`

	DefinitionIssues []DefinitionIssue `json:"definition_issues,omitempty"`
	EvaluationIssues []EvaluationIssue `json:"evaluation_issues,omitempty"`
}
