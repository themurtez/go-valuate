package kpi

// SchemaVersion identifies this package's fixed data shapes: Definition,
// Expression, MetricValue, DimensionKey, Period, TargetPolicy,
// ThresholdBand, Options, KPIResult, Trace, Result, and every Issue type.
// Bump this whenever a field is added, removed, or its meaning changes in
// a way that could break a caller's stored JSON — see the repository
// README's versioning-strategy section, which this constant follows
// exactly (financial.SchemaVersion, vendorspend.SchemaVersion, etc.).
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed formula set: every
// Operator's semantics (evaluate.go), aggregation rule semantics
// (metric.go), target/band evaluation (target.go, band.go), trend/change
// computation (trend.go), and dependency-cycle/validation rules
// (dependency.go, definition.go). Bump this whenever any of that changes
// in a way that could make a historical Result not reproduce identically
// under new code. Adding a new Operator or AggregationRule that a
// Definition must opt into does not by itself change any *existing*
// Definition's result, so it does not require a bump; changing what an
// *existing* Operator/AggregationRule computes does.
const FormulaVersion = "1.0.0"

// ExpressionLanguageVersion identifies the Expression AST shape itself —
// task section 27 and the doc.go "No string DSL in V1" section. This is
// deliberately a distinct version axis from SchemaVersion/FormulaVersion:
// SchemaVersion covers every type's field shape, FormulaVersion covers
// operator semantics, and ExpressionLanguageVersion specifically covers
// whether a given Expression tree (as a value, independent of what it
// computes) is one this engine's parser/validator/evaluator can accept at
// all — the axis a future string-DSL-to-AST compiler would need to check
// against before emitting a tree for this engine to consume.
//
// A Definition does not carry its own ExpressionLanguageVersion field in
// V1 (there being only one version to declare), but a persistence layer
// storing Definitions is expected to record which
// ExpressionLanguageVersion each stored Definition was authored against,
// and Calculate/ValidateDefinition report
// IssueUnsupportedExpressionLanguageVersion if a future caller passes a
// version string other than this one via Options (Options.
// ExpressionLanguageVersion, left empty by default to mean "this
// version").
const ExpressionLanguageVersion = "1.0.0"
