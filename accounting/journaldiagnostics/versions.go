package journaldiagnostics

// SchemaVersion identifies this package's public result/schema contract:
// the shape of EntryMetadata, PeriodWindow, AccountReviewPolicy, Policy,
// Finding, Evidence, DuplicateGroup, Result, and Issue. Bump whenever any of
// that shape changes in a way that could make a historical persisted Result
// not reproduce identically under new code — see the repository README's
// versioning-strategy section, which every sibling package's SchemaVersion
// (ledger.SchemaVersion, ar.SchemaVersion, ap.SchemaVersion, ...) already
// follows.
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed diagnostic-rule semantics
// and defaults: every finding-trigger rule (manual.go, periodend.go,
// timing.go, amounts.go, accounts.go, duplicates.go, reversals.go,
// clustering.go, controls.go), the median+MAD baseline method (amounts.go),
// duplicate-signature normalization (duplicates.go), and DefaultPolicy's
// threshold values. Bump this whenever any of that changes in a way that
// could make a historical Result not reproduce identically under new code.
const FormulaVersion = "1.0.0"
