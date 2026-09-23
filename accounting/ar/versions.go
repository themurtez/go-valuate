package ar

// SchemaVersion identifies this package's public result/schema contract:
// the shape of Receivable, Payment, BucketDefinition, CustomerSummary,
// Result, and Issue. Bump whenever any of that shape changes in a way that
// could make a historical persisted Result not reproduce identically under
// new code — see the repository README's versioning-strategy section,
// which every sibling package's SchemaVersion (ledger.SchemaVersion,
// statements.SchemaVersion, ...) already follows.
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed calculation semantics:
// aging-basis/bucket assignment (buckets.go), DSO (dso.go), historical
// trend and migration (trends.go), collection metrics (collections.go),
// concentration (concentration.go), and every flag-trigger rule
// (flags.go). Bump this whenever any of that changes in a way that could
// make a historical Result not reproduce identically under new code.
const FormulaVersion = "1.0.0"
