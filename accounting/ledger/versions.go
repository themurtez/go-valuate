package ledger

// SchemaVersion identifies this package's public contract: the shape of
// Account, JournalEntry, JournalLine, Ledger, Balance, TrialBalance,
// TrialBalanceInput, NormalizedTrialBalance, and Issue, plus the
// validation/balance/trial-balance/rollup rules that produce them. Bump
// this whenever any of that changes in a way that could make a historical
// persisted result not reproduce identically under new code — see the
// repository README's versioning-strategy section, which this constant
// follows exactly (financial.TaxonomyVersion, ingestion.SchemaVersion,
// debt.FormulaVersion, etc.).
//
// A separate FormulaVersion is deliberately not defined for this package:
// every calculation here (balance movement, trial-balance totals, hierarchy
// rollups) is a direct, non-optional consequence of the double-entry rules
// already encoded in SchemaVersion itself — there is no independently
// versionable formula choice the way e.g. debt.FormulaVersion (which
// amortization/coverage formulas) or qoe.ScoreVersion (how already-computed
// figures are weighted into one heuristic number) have. If a future change
// introduces a genuinely separate calculation semantic (e.g. an alternate
// rollup strategy or an FX conversion mode), it should get its own
// versioned constant at that time rather than overloading SchemaVersion.
const SchemaVersion = "1.0.0"
