package journaldiagnostics

// IssueSeverity distinguishes a problem that blocks some portion of the
// analysis (SeverityError) from one that is advisory only
// (SeverityWarning) — the same two-severity model every analytics package
// in this repository uses.
type IssueSeverity string

const (
	IssueSeverityError   IssueSeverity = "error"
	IssueSeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of analysis/input problem —
// distinct from FindingCode, which identifies a journal-entry anomaly
// itself (see the package doc comment and Finding's doc comment for the
// Issue-vs-Finding distinction: a structurally invalid entry or a
// misconfigured Policy is an Issue; an unusual-but-structurally-valid entry
// is a Finding). Only codes this package actually emits are defined.
type IssueCode string

const (
	// IssueInvalidPeriod means the supplied PeriodWindow is missing
	// StartDate/EndDate or has StartDate after EndDate.
	IssueInvalidPeriod IssueCode = "INVALID_PERIOD"
	// IssueLedgerValidationFailed means ledger.ValidateEntries (or
	// ledger.ValidateAccounts) reported at least one SeverityError issue
	// against the supplied Ledger. The underlying ledger.Issue values are
	// not duplicated here — see Result.LedgerIssues.
	IssueLedgerValidationFailed IssueCode = "LEDGER_VALIDATION_FAILED"
	// IssueDuplicateMetadataEntry means two or more EntryMetadata values
	// share the same EntryID. Only the first occurrence (input order) is
	// used; later ones are ignored.
	IssueDuplicateMetadataEntry IssueCode = "DUPLICATE_METADATA"
	// IssueUnknownMetadataEntry means an EntryMetadata.EntryID does not
	// match any JournalEntry.ID in the supplied Ledger.
	IssueUnknownMetadataEntry IssueCode = "UNKNOWN_METADATA_ENTRY"
	// IssueInvalidTimestamp means an EntryMetadata.CreatedAt or PostedAt
	// could not be evaluated against Policy.BusinessHours because
	// BusinessHours.TimeZone did not name a loadable IANA zone.
	IssueInvalidTimestamp IssueCode = "INVALID_TIMESTAMP"
	// IssueInvalidPolicy means a Policy field was structurally invalid (e.g.
	// negative threshold, ThresholdClusterLowerPercent outside [0,1]).
	IssueInvalidPolicy IssueCode = "INVALID_POLICY"
	// IssueNonFiniteThreshold means a Policy numeric field was NaN or +/-Inf.
	// The offending rule is treated as disabled, never propagated.
	IssueNonFiniteThreshold IssueCode = "NON_FINITE_THRESHOLD"
	// IssueInsufficientBaseline means an account had fewer than
	// Policy.MinBaselineObservations prior historical entries, so the
	// account-relative large-entry rule could not be evaluated for it this
	// run. This is expected/common, not necessarily a data problem — see
	// RuleAvailability for the per-rule, non-per-account summary of this
	// same condition.
	IssueInsufficientBaseline IssueCode = "INSUFFICIENT_BASELINE"
)

// Issue is a single structured analysis/input problem — see IssueCode for
// the Issue-vs-Finding distinction.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// EntryID identifies the JournalEntry.ID this Issue relates to, if any.
	EntryID string `json:"entry_id,omitempty"`
	// AccountID identifies the Account.ID this Issue relates to, if any.
	AccountID string `json:"account_id,omitempty"`
}

// HasErrors reports whether any Issue in issues has IssueSeverityError.
//
// Intentionally duplicated from ledger.HasErrors and this repository's
// other sibling HasErrors functions rather than shared — see
// financial/adjustments.HasErrors's doc comment for the full rationale
// (each package's Issue is a distinct Go type with no common interface
// worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == IssueSeverityError {
			return true
		}
	}
	return false
}
