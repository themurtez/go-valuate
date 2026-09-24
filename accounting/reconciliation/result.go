package reconciliation

// Result is this package's top-level output for one reconciliation
// period.
type Result struct {
	AccountID         string             `json:"account_id"`
	ExternalAccountID string             `json:"external_account_id,omitempty"`
	Period            string             `json:"period,omitempty"`
	AsOfDate          string             `json:"as_of_date"`
	Type              ReconciliationType `json:"type"`

	Status Status `json:"status"`

	Equation EquationResult `json:"equation"`

	BookBalance     Balance `json:"book_balance"`
	ExternalBalance Balance `json:"external_balance"`

	// TransactionModeAvailable is true when at least one BookItem or
	// ExternalItem was supplied — see task section 9/51's "balance-only
	// vs transaction coverage stay separate" instruction.
	TransactionModeAvailable bool `json:"transaction_mode_available"`

	MatchedGroups          []MatchGroup `json:"matched_groups,omitempty"`
	UnmatchedBookItems     []string     `json:"unmatched_book_items,omitempty"`
	UnmatchedExternalItems []string     `json:"unmatched_external_items,omitempty"`

	AmbiguousCandidates []Candidate `json:"ambiguous_candidates,omitempty"`

	ReconcilingItems []ReconcilingItem `json:"reconciling_items,omitempty"`

	AgedUnmatchedBookItems     []AgedItem `json:"aged_unmatched_book_items,omitempty"`
	AgedUnmatchedExternalItems []AgedItem `json:"aged_unmatched_external_items,omitempty"`
	AgedReconcilingItems       []AgedItem `json:"aged_reconciling_items,omitempty"`

	MatchSummaryBook     MatchSummary `json:"match_summary_book"`
	MatchSummaryExternal MatchSummary `json:"match_summary_external"`

	MatchQuality MatchQualitySummary `json:"match_quality"`
	Coverage     Coverage            `json:"coverage"`

	Findings []Finding `json:"findings,omitempty"`
	Issues   []Issue   `json:"issues,omitempty"`

	Versions Versions `json:"versions"`
}
