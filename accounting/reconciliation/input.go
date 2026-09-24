package reconciliation

// Input is everything Calculate needs for one reconciliation period.
type Input struct {
	// AccountID is the book-side account being reconciled (opaque
	// caller-assigned identifier). Required — see IssueInvalidReconciliationKey.
	AccountID string `json:"account_id"`
	// ExternalAccountID is the external/control-side account or reference
	// (e.g. a bank account number, a lender loan number, a GL control
	// account ID). Recommended but not strictly required for every
	// ReconciliationType — see validReconciliationKey.
	ExternalAccountID string `json:"external_account_id,omitempty"`
	// Period is the caller's own period label (e.g. "2025-06"),
	// carried through for display/provenance only.
	Period string `json:"period,omitempty"`
	// AsOfDate is the date this reconciliation is performed as of,
	// "YYYY-MM-DD". Required — every aging/staleness calculation is
	// relative to this explicit date; this package never calls
	// time.Now().
	AsOfDate string `json:"as_of_date"`

	Type ReconciliationType `json:"type,omitempty"`

	BookItems     []BookItem     `json:"book_items,omitempty"`
	ExternalItems []ExternalItem `json:"external_items,omitempty"`

	BookBalance     BalanceInput `json:"book_balance,omitempty"`
	ExternalBalance BalanceInput `json:"external_balance,omitempty"`

	ConfirmedMatches []ConfirmedMatch  `json:"confirmed_matches,omitempty"`
	ReconcilingItems []ReconcilingItem `json:"reconciling_items,omitempty"`

	AgingBuckets []AgingBucket `json:"aging_buckets,omitempty"`

	Policy MatchingPolicy `json:"policy"`
}

func (in Input) effectiveType() ReconciliationType {
	if in.Type == "" {
		return TypeGeneric
	}
	return in.Type
}
