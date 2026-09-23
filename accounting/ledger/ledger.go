package ledger

// Ledger bundles a chart of accounts and its journal entries — the
// top-level input to trial-balance construction and multi-entry balance
// calculation. A caller working from an already-summarized trial balance
// instead of journal-entry detail uses TrialBalanceInput (see tbimport.go)
// rather than this type.
type Ledger struct {
	Accounts []Account      `json:"accounts"`
	Entries  []JournalEntry `json:"entries"`
}

// Validate runs ValidateAccounts and ValidateEntries against l's own
// Accounts/Entries, in that order, returning every Issue found. It does
// not mutate l.
func (l Ledger) Validate(opts ValidateOptions) []Issue {
	chart := BuildChartOfAccounts(l.Accounts)
	var issues []Issue
	issues = append(issues, ValidateAccounts(l.Accounts)...)
	issues = append(issues, ValidateEntries(l.Entries, chart, opts)...)
	return issues
}

// Chart builds and returns a ChartOfAccounts from l.Accounts.
func (l Ledger) Chart() ChartOfAccounts {
	return BuildChartOfAccounts(l.Accounts)
}
