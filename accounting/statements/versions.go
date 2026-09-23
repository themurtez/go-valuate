package statements

// SchemaVersion identifies this package's public result/schema contract:
// the shape of AccountMapping, MappingResult, MappingCoverage, Statement,
// Section, Row, Result, and Issue. Bump whenever any of that shape changes
// in a way that could make a historical persisted Result not reproduce
// identically under new code — see the repository README's
// versioning-strategy section, which every sibling package's SchemaVersion
// (ledger.SchemaVersion, review.SchemaVersion, ...) already follows.
const SchemaVersion = "1.0.0"

// StatementFormulaVersion identifies this package's fixed statement-
// building semantics: sign normalization (signs.go), canonical calculated
// subtotals (income.go/balance.go — which delegate to
// financial/metrics.FormulaVersion for the underlying arithmetic, but this
// constant additionally versions THIS package's own structural-row
// assembly and section templates on top of that), contra-account handling,
// and hierarchy leaf-posting policy. Bump whenever any of that changes.
const StatementFormulaVersion = "1.0.0"

// MappingContractVersion identifies this package's account-mapping
// contract specifically: AccountMapping's shape, mapping precedence
// (explicit > deterministic suggestion > unmapped — see mapping.go),
// account-type safety rules (accounttype.go), and MappingTemplate rule
// precedence (templates.go). Versioned independently of
// StatementFormulaVersion per the task's instruction to version "the
// mapping contract if independently evolvable" — a mapping-precedence
// change does not necessarily imply a statement-formula change, and vice
// versa.
const MappingContractVersion = "1.0.0"
