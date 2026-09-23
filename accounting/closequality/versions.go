package closequality

// SchemaVersion identifies this package's public result/schema contract:
// the shape of Input, Policy, Dimension/DimensionResult, Finding,
// Evidence, Coverage, CloseTaskSummary, ComparisonResult, Result, and
// Issue. Bump whenever any of that shape changes in a way that could
// make a historical persisted Result not reproduce identically under new
// code — see the repository README's versioning-strategy section, which
// every sibling package's SchemaVersion already follows.
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed close-readiness
// semantics and defaults: every mine_*.go translation rule, the
// readiness-status decision table (readiness.go), deduplication keying
// (findings.go's dedupKey), and DefaultPolicy's/DefaultJournalFindingRules's
// values. Bump this whenever any of that changes in a way that could
// make a historical Result not reproduce identically under new code.
const FormulaVersion = "1.0.0"

// Versions is the pair of version strings echoed on Result, grouped into
// one struct for JSON convenience.
type Versions struct {
	SchemaVersion  string `json:"schema_version"`
	FormulaVersion string `json:"formula_version"`
}

func currentVersions() Versions {
	return Versions{SchemaVersion: SchemaVersion, FormulaVersion: FormulaVersion}
}
