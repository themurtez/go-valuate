package journaldiagnostics

// SourceBucket is one EntrySource's count/amount/percentage within the
// analyzed population — see the package doc's "Source-system mix" section.
// Useful even when it produces no findings.
type SourceBucket struct {
	Source         EntrySource `json:"source"`
	Count          int         `json:"count"`
	Amount         float64     `json:"amount"`
	PercentOfTotal float64     `json:"percent_of_total"`
}

// sourceOrder fixes EntrySource display order for deterministic
// SourceSummary output.
var sourceOrder = []EntrySource{SourceManual, SourceImport, SourceSystem, SourceRecurring, SourceAdjusting, SourceClosing, SourceUnknown}

// SourceSummary summarizes analyzed-entry activity by EntrySource. Entries
// with no metadata (or metadata with an empty Source) are not included in
// any bucket — see UnknownSourceCount/UnknownSourceAmount, kept distinct
// from the explicit SourceUnknown bucket (a caller explicitly saying
// "unknown" is different from supplying no metadata at all).
type SourceSummary struct {
	Buckets []SourceBucket `json:"buckets,omitempty"`
	// UnknownSourceCount and UnknownSourceAmount cover analyzed entries with
	// no metadata, or metadata with an unset Source.
	UnknownSourceCount  int     `json:"unknown_source_count"`
	UnknownSourceAmount float64 `json:"unknown_source_amount"`
}

// AccountActivityBucket is one account's analyzed-entry activity —
// supporting data for rare/new-account and account-relative diagnostics,
// also useful as a plain inventory.
type AccountActivityBucket struct {
	AccountID    string  `json:"account_id"`
	EntryCount   int     `json:"entry_count"`
	TotalDebits  float64 `json:"total_debits"`
	TotalCredits float64 `json:"total_credits"`
}

// AccountActivitySummary lists per-account analyzed activity, sorted by
// AccountID.
type AccountActivitySummary struct {
	Accounts []AccountActivityBucket `json:"accounts,omitempty"`
}

// DuplicateGroup is one set of two or more entries sharing a normalized
// economic-content signature — see duplicates.go. Exact ==true means every
// member matched the exact-duplicate rule (same normalized signature, same
// effective date); Exact == false means the group matched only the
// possible-duplicate (near-duplicate, within DuplicateWindowDays) rule.
type DuplicateGroup struct {
	Exact               bool     `json:"exact"`
	NormalizedSignature string   `json:"normalized_signature"`
	EntryIDs            []string `json:"entry_ids"`
	EarliestDate        string   `json:"earliest_date"`
	LatestDate          string   `json:"latest_date"`
	Amount              float64  `json:"amount"`
}

// ReversalPair is one explicit original/reversal relationship analyzed for
// timing.
type ReversalPair struct {
	OriginalEntryID  string  `json:"original_entry_id"`
	ReversingEntryID string  `json:"reversing_entry_id"`
	OriginalDate     string  `json:"original_date"`
	ReversingDate    string  `json:"reversing_date"`
	DaysBetween      Value   `json:"days_between"`
	Amount           float64 `json:"amount"`
	CrossPeriod      bool    `json:"cross_period"`
	Rapid            bool    `json:"rapid"`
}

// ReversalSummary reports explicit reversal activity within the analyzed
// population — see reversals.go. Never infers a reversal from equal-and-
// opposite amounts; every pair here comes from an explicit
// ledger.Reversal relationship.
type ReversalSummary struct {
	Available     bool           `json:"available"`
	ReversalCount int            `json:"reversal_count"`
	Pairs         []ReversalPair `json:"pairs,omitempty"`
}

// PeriodEndSummary reports activity within Policy.PeriodEndDays of
// PeriodWindow.EndDate — see periodend.go.
type PeriodEndSummary struct {
	Available           bool    `json:"available"`
	PeriodEndEntryCount int     `json:"period_end_entry_count"`
	PeriodEndAmount     float64 `json:"period_end_amount"`
	PercentOfTotal      Value   `json:"percent_of_total"`
}
