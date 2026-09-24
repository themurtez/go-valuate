package kpi

// IssueSeverity mirrors every sibling package's two-severity model
// (accounting/vendorspend.IssueSeverity, accounting/ap.IssueSeverity,
// ...): SeverityError excludes the affected KPI/definition from further
// evaluation; SeverityWarning is advisory only.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of problem. Task section
// 26 asks this package to keep Definition issues (a formula is
// structurally broken — a cycle, a bad operator arity, an invalid unit,
// ...) separate from runtime Evaluation issues (a specific metric was
// missing, a specific period had no data, a specific denominator was
// zero, ...); this package keeps that separation as two distinct Go
// types, DefinitionIssue and EvaluationIssue (see below), each with its
// own IssueCode subset, rather than one flat Issue list a caller has to
// filter by convention. Per-KPIResult unavailability already carries its
// own AvailabilityReason (value.go); EvaluationIssue exists only for
// evaluation-time problems severe enough to warrant a standalone,
// filterable, position-addressable record (e.g. "this metric was
// completely missing for this period/dimension combination" beyond what
// a single KPIResult's own Reason communicates) — task section 26's "do
// not flood top-level issues when a per-result availability reason is
// sufficient" instruction, applied by keeping EvaluationIssue emission
// deliberately sparse (see evaluate.go for exactly which cases emit one).
type IssueCode string

const (
	// --- Definition issues (structural problems with a Definition
	// itself, independent of any specific period/dimension) ---

	IssueDuplicateKPICode                     IssueCode = "DUPLICATE_KPI_CODE"
	IssueInvalidKPICode                       IssueCode = "INVALID_KPI_CODE"
	IssueInvalidExpression                    IssueCode = "INVALID_EXPRESSION"
	IssueUnknownOperator                      IssueCode = "UNKNOWN_OPERATOR"
	IssueInvalidArgumentCount                 IssueCode = "INVALID_ARGUMENT_COUNT"
	IssueUnknownKPI                           IssueCode = "UNKNOWN_KPI"
	IssueDependencyCycle                      IssueCode = "DEPENDENCY_CYCLE"
	IssueNamespaceCollision                   IssueCode = "NAMESPACE_COLLISION"
	IssueInvalidUnit                          IssueCode = "INVALID_UNIT"
	IssueInvalidTarget                        IssueCode = "INVALID_TARGET"
	IssueInvalidThresholdBand                 IssueCode = "INVALID_THRESHOLD_BAND"
	IssueOverlappingThresholdBands            IssueCode = "OVERLAPPING_THRESHOLD_BANDS"
	IssueUnsupportedExpressionLanguageVersion IssueCode = "UNSUPPORTED_EXPRESSION_LANGUAGE_VERSION"
	IssueExpressionTooComplex                 IssueCode = "EXPRESSION_TOO_COMPLEX"
	IssueDependencyTooDeep                    IssueCode = "DEPENDENCY_TOO_DEEP"

	// --- Evaluation issues (runtime problems tied to specific input
	// data) ---

	IssueUnknownMetric        IssueCode = "UNKNOWN_METRIC"
	IssueDuplicateMetricValue IssueCode = "DUPLICATE_METRIC_VALUE"
	IssueUnitMismatch         IssueCode = "UNIT_MISMATCH"
	IssueCurrencyMismatch     IssueCode = "CURRENCY_MISMATCH"
	IssueInvalidPeriod        IssueCode = "INVALID_PERIOD"
	IssueInvalidDimension     IssueCode = "INVALID_DIMENSION"
	IssueNonFiniteInput       IssueCode = "NON_FINITE_INPUT"
	IssueNonFiniteResult      IssueCode = "NON_FINITE_RESULT"
	IssueInvalidPolicy        IssueCode = "INVALID_POLICY"
)

// DefinitionIssue is one structural problem with a Definition — task
// section 26. KPICode identifies the affected Definition (empty only for
// an issue that cannot be attributed to one, e.g. IssueDuplicateKPICode
// names the duplicated code itself in KPICode regardless of which
// occurrence).
type DefinitionIssue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	KPICode  string        `json:"kpi_code,omitempty"`
}

// EvaluationIssue is one runtime problem tied to specific input data —
// task section 26. Deliberately sparse (see IssueCode's doc comment);
// most runtime unavailability is communicated via a KPIResult's own
// Value.Reason instead.
type EvaluationIssue struct {
	Code       IssueCode     `json:"code"`
	Severity   IssueSeverity `json:"severity"`
	Message    string        `json:"message"`
	KPICode    string        `json:"kpi_code,omitempty"`
	MetricCode string        `json:"metric_code,omitempty"`
	Period     string        `json:"period,omitempty"`
	Dimensions DimensionKey  `json:"dimensions,omitempty"`
}

// HasDefinitionErrors reports whether any DefinitionIssue has
// SeverityError.
//
// Intentionally duplicated from every sibling package's own HasErrors
// rather than shared — see financial/adjustments.HasErrors's doc comment
// for the full rationale (each package's Issue is a distinct Go type with
// no common interface worth introducing for one boolean function).
func HasDefinitionErrors(issues []DefinitionIssue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasEvaluationErrors reports whether any EvaluationIssue has
// SeverityError.
func HasEvaluationErrors(issues []EvaluationIssue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// definitionIssueCodeOrder/evaluationIssueCodeOrder fix declaration order
// for deterministic sorting — task section 42's "issues by
// code/KPI/period/dimension" rule.
var definitionIssueCodeOrder = []IssueCode{
	IssueDuplicateKPICode,
	IssueInvalidKPICode,
	IssueInvalidExpression,
	IssueUnknownOperator,
	IssueInvalidArgumentCount,
	IssueUnknownKPI,
	IssueDependencyCycle,
	IssueNamespaceCollision,
	IssueInvalidUnit,
	IssueInvalidTarget,
	IssueInvalidThresholdBand,
	IssueOverlappingThresholdBands,
	IssueUnsupportedExpressionLanguageVersion,
	IssueExpressionTooComplex,
	IssueDependencyTooDeep,
}

var evaluationIssueCodeOrder = []IssueCode{
	IssueUnknownMetric,
	IssueDuplicateMetricValue,
	// IssueInvalidUnit is shared with definitionIssueCodeOrder (a
	// Definition.Unit check) but is also reachable as an EvaluationIssue
	// (a MetricValue.Unit check, in buildMetricIndex) — the same
	// IssueCode can legitimately appear in either taxonomy since IssueCode
	// itself carries no severity/scope tag of its own; DefinitionIssue vs
	// EvaluationIssue (the containing Go type) is what actually
	// distinguishes them.
	IssueInvalidUnit,
	IssueUnitMismatch,
	IssueCurrencyMismatch,
	IssueInvalidPeriod,
	IssueInvalidDimension,
	IssueNonFiniteInput,
	IssueNonFiniteResult,
	IssueInvalidPolicy,
}

func definitionIssueRank(c IssueCode) int {
	for i, ic := range definitionIssueCodeOrder {
		if ic == c {
			return i
		}
	}
	return len(definitionIssueCodeOrder)
}

func evaluationIssueRank(c IssueCode) int {
	for i, ic := range evaluationIssueCodeOrder {
		if ic == c {
			return i
		}
	}
	return len(evaluationIssueCodeOrder)
}
