package ledger

// IntegrityReport bundles every structural/integrity Issue found across a
// Ledger's accounts and entries in one pass — the single entry point a
// caller wanting a full "is this ledger sound" check should use, rather
// than calling ValidateAccounts/ValidateEntries separately. It performs no
// calculation of its own; it is a thin, deterministic composition of
// ValidateAccounts and ValidateEntries (which itself includes
// validateReversals).
type IntegrityReport struct {
	SchemaVersion string  `json:"schema_version"`
	Issues        []Issue `json:"issues,omitempty"`
	// Balanced is true only if no entry carries IssueUnbalancedEntry — see
	// TrialBalance.Balanced for the equivalent trial-balance-level signal,
	// which this report does not itself compute (it validates entries, not
	// balances).
	HasErrors bool `json:"has_errors"`
}

// CheckLedgerIntegrity runs every structural integrity check this package
// defines against l: account validity/duplication/hierarchy (cycles,
// missing parents), and entry validity/balance/reversal-consistency. It
// does not mutate l.
func CheckLedgerIntegrity(l Ledger, opts ValidateOptions) IntegrityReport {
	issues := l.Validate(opts)
	return IntegrityReport{
		SchemaVersion: SchemaVersion,
		Issues:        issues,
		HasErrors:     HasErrors(issues),
	}
}
