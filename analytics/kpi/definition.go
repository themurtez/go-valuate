package kpi

// Definition is one portable KPI formula — task section 2. Code is this
// KPI's stable machine identity, used by KPIRef.Code and everywhere a
// dependency graph or Result needs to name it; Name/Description are
// display text only and never participate in identity, matching, or
// evaluation.
type Definition struct {
	// Code uniquely identifies this KPI within one Calculate call.
	// Required, non-empty. Also the namespace that must never collide
	// with a MetricValue.Code a Definition's own expression (or any
	// other Definition's) references as a KPIRef — see
	// IssueNamespaceCollision.
	Code string `json:"code"`
	// Name/Description are display text, never used for identity or
	// matching.
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// Formula is this KPI's typed expression tree — see expression.go.
	Formula Expression `json:"formula"`
	// Unit is this KPI's expected/declared output unit. Definition-time
	// validation only requires Unit to be structurally valid (Unit.Valid())
	// — see IssueInvalidUnit. Whether Formula's root actually PRODUCES a
	// value in this Unit can only be checked once real MetricValues are
	// supplied (a METRIC leaf's unit is a runtime fact, not a definition-
	// time one), so that check happens per (period, dimension) instance
	// during evaluation instead: a mismatch there is reported as
	// AvailabilityUnitMismatch/AvailabilityCurrencyMismatch on that one
	// KPIResult.Value, not as a top-level DefinitionIssue — see
	// checkExpectedOutputUnit in evaluate.go.
	Unit Unit `json:"unit"`
	// Category is a caller-assigned grouping label (e.g. "Profitability",
	// "Operations"), carried through for display/filtering only.
	Category string `json:"category,omitempty"`
	// Tags is a caller-assigned free-form label set, carried through for
	// display/filtering only.
	Tags []string `json:"tags,omitempty"`
	// Target/ThresholdBands are optional caller-supplied evaluation
	// policies — see target.go/band.go. Nil Target means no target
	// configured (TargetEvaluation.TargetAvailable is then always false);
	// empty ThresholdBands means no band configured.
	Target         *TargetPolicy   `json:"target,omitempty"`
	ThresholdBands []ThresholdBand `json:"threshold_bands,omitempty"`
	// DefinitionVersion is a caller-assigned version label for THIS
	// specific Definition (e.g. "2" after a controller edits the
	// formula), independent of this package's own
	// SchemaVersion/FormulaVersion/ExpressionLanguageVersion constants —
	// carried through for a caller's own change-tracking, never
	// interpreted by this package.
	DefinitionVersion string `json:"definition_version,omitempty"`
}
