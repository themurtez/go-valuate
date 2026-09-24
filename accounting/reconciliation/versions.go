package reconciliation

// SchemaVersion identifies this package's public result/schema contract:
// the shape of Input, MatchingPolicy, BookItem, ExternalItem, MatchGroup,
// Candidate, ReconcilingItem, Balance, Coverage, MatchQualitySummary,
// Finding, Issue, and Result. Bump whenever any of that shape changes in
// a way that could make a historical persisted Result not reproduce
// identically under new code — see the repository README's
// versioning-strategy section, which every sibling package's
// SchemaVersion already follows.
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed reconciliation-equation
// and arithmetic semantics: the equation.go balance-adjustment/difference
// formula, the balances.go rollforward formula, the status.go decision
// table, and coverage.go's coverage/match-quality computations. Bump this
// whenever any of that changes in a way that could make a historical
// Result not reproduce identically under new code.
const FormulaVersion = "1.0.0"

// MatchingVersion identifies this package's matching-precedence and
// composite-search behavior specifically — see matcher.go's precedence
// order and composite.go's bounded search. Kept separate from
// FormulaVersion because persisted auto-match decisions may need to
// evolve (a new precedence rule, a different composite-search bound)
// independently of the reconciliation-equation arithmetic itself: two
// Results computed under different MatchingVersion values may legitimately
// classify the same input's matches differently even though the
// equation/status formulas (FormulaVersion) are unchanged.
const MatchingVersion = "1.0.0"

// Versions is the triple of version strings echoed on Result, grouped
// into one struct for JSON convenience.
type Versions struct {
	SchemaVersion   string `json:"schema_version"`
	FormulaVersion  string `json:"formula_version"`
	MatchingVersion string `json:"matching_version"`
}

func currentVersions() Versions {
	return Versions{
		SchemaVersion:   SchemaVersion,
		FormulaVersion:  FormulaVersion,
		MatchingVersion: MatchingVersion,
	}
}
