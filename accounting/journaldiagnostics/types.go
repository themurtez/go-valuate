// Package journaldiagnostics implements deterministic journal-entry
// diagnostics for accountant/controller review: unusual patterns,
// control-review indicators, duplicate-like activity, and period-end/
// post-close/timing behavior, mined from an accounting/ledger.Ledger.
//
// # Not a fraud-detection engine
//
// This package identifies patterns worth an accountant or controller's
// attention. It never states or implies fraud, theft, embezzlement,
// intentional manipulation, or misconduct — see docs/JOURNAL_DIAGNOSTICS.md's
// "Purpose and non-fraud boundary" section. Every Finding uses neutral
// language: "anomaly," "unusual pattern," "unusual activity," "review
// recommended," "control-review indicator." A caller may attach an external
// fraud/risk label to a Finding as its own data (see Finding's doc comment),
// but this package never independently concludes one. See
// safety_no_fraud_language_test.go for the regression test enforcing this
// across every generated message.
//
// # No composite score
//
// This package does not produce a fraud probability, fraud score,
// misconduct score, or any other single composite review-priority number.
// It returns Findings with a Severity (review priority, not probability of
// wrongdoing — see Severity's doc comment), typed Evidence, and summary
// counts; the calling application decides presentation and prioritization.
//
// # Ledger integration
//
// This package works directly from ledger.Ledger (ledger.Account +
// ledger.JournalEntry) rather than defining a second journal-entry
// accounting model. It reuses ledger.ValidateEntries for structural
// validation, ledger.NormalBalance for opposite-normal-balance detection,
// and the ledger package's own Reversal/EntryStatus semantics for
// reversal-aware analysis — it never infers a reversal from equal-and-
// opposite amounts. Nothing in this package mutates ledger.Ledger, any
// Account, or any JournalEntry it is given — see immutability_test.go.
//
// # Optional metadata
//
// ledger.JournalEntry intentionally does not carry every audit/control
// field (preparer, approver, timestamps, batch, external reference). Rather
// than pollute accounting/ledger to satisfy this package, diagnostics
// accept optional EntryMetadata keyed by EntryID (see EntryMetadata's doc
// comment). When metadata for a given field is absent, every diagnostic
// that depends on it reports itself unavailable (see RuleAvailability) —
// never false, and never silently omitted without being made visible in
// Result.RuleAvailability and Result.Coverage.
//
// # No hidden current-date dependency
//
// Every diagnostic takes an explicit PeriodWindow from caller input.
// Nothing in this package calls time.Now() — see determinism_test.go.
//
// # Determinism and immutability
//
// Every exported function is pure: no I/O, no mutation of caller-owned
// input, no package-global mutable state. Calculate can be called
// concurrently and repeatedly against identical input and always returns
// byte-for-byte identical JSON — see determinism_test.go and
// concurrency_test.go.
//
// # Explicit non-goals
//
// This package does not implement fraud determination, audit opinion,
// transaction approval, journal posting/editing, segregation-of-duties
// workflow, user identity management, access-control testing, bank
// reconciliation, invoice/vendor/customer matching, ML anomaly detection,
// AI narrative, or tax diagnostics. It has no persistence, HTTP/API, UI, or
// background jobs.
//
// # Boundary to future modules
//
//	accounting/ledger
//	      ↓
//	accounting/journaldiagnostics
//	      ↓
//	future bookkeeping/close-quality module
//
// A future close-quality/bookkeeping-quality module (not implemented here)
// is expected to aggregate this package's Result alongside other close
// checks. This package does not orchestrate that future module and knows
// nothing about it.
package journaldiagnostics

import (
	"math"
	"time"
)

// isNonFinite reports whether v is NaN or +/-Inf — the shared guard every
// money-bearing field in this package is checked against.
func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown because
// a required input was absent." This package's own local copy of the
// convention every sibling package in this repository duplicates rather
// than importing another package's Value — see
// transactions/salereadiness.Value's doc comment for the full rationale.
type Value struct {
	Available bool    `json:"available"`
	Amount    float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may legitimately
// be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// EntrySource identifies what produced a JournalEntry, for diagnostics
// keyed by EntryMetadata.Source (distinct from ledger.JournalEntry.Source,
// which is an open caller-defined string — see EntryMetadata.Source's doc
// comment for why this package defines its own closed set for this
// purpose).
type EntrySource string

const (
	SourceManual    EntrySource = "MANUAL"
	SourceImport    EntrySource = "IMPORT"
	SourceSystem    EntrySource = "SYSTEM"
	SourceRecurring EntrySource = "RECURRING"
	SourceAdjusting EntrySource = "ADJUSTING"
	SourceClosing   EntrySource = "CLOSING"
	SourceUnknown   EntrySource = "UNKNOWN"
)

// isRecognizedSource reports whether s is one of the fixed EntrySource
// values (SourceUnknown counts as recognized — a caller explicitly saying
// "unknown" is different from supplying no metadata at all).
func isRecognizedSource(s EntrySource) bool {
	switch s {
	case SourceManual, SourceImport, SourceSystem, SourceRecurring, SourceAdjusting, SourceClosing, SourceUnknown:
		return true
	default:
		return false
	}
}

// EntryMetadata is optional audit/control metadata for one JournalEntry,
// supplied separately from ledger.Ledger and joined by EntryID — see the
// package doc comment's "Optional metadata" section. A caller supplies
// zero, one, or many fields per entry; every field is independently
// optional, and this package never fabricates a value for a field the
// caller did not supply.
type EntryMetadata struct {
	// EntryID is the ledger.JournalEntry.ID this metadata describes.
	// Required for the metadata to be usable — see IssueUnknownMetadataEntry
	// and IssueDuplicateMetadataEntry.
	EntryID string `json:"entry_id"`
	// Source identifies what produced this entry. Distinct from
	// ledger.JournalEntry.Source (an open, caller-defined display string):
	// this field is this package's own closed EntrySource taxonomy, used for
	// manual-entry detection (see manual.go) and source-mix summaries (see
	// SourceSummary). The zero value ("") means "not supplied," distinct
	// from the explicit SourceUnknown value.
	Source EntrySource `json:"source,omitempty"`
	// CreatedAt is when this entry was created/drafted, if known. Optional;
	// a nil value means unknown, not "created at the zero time."
	CreatedAt *time.Time `json:"created_at,omitempty"`
	// PostedAt is when this entry was posted, if known. Optional. Used for
	// post-close (see periodend.go) and business-hours (see timing.go)
	// diagnostics; PostedAt is preferred over CreatedAt for those when both
	// are supplied, since posting — not drafting — is the control-relevant
	// event.
	PostedAt *time.Time `json:"posted_at,omitempty"`
	// PreparerID is an opaque, caller-assigned identifier for who prepared
	// this entry. Never a real name — see the package doc comment's
	// "identity" note in controls.go. Optional.
	PreparerID string `json:"preparer_id,omitempty"`
	// ApproverID is an opaque, caller-assigned identifier for who approved
	// this entry. Optional.
	ApproverID string `json:"approver_id,omitempty"`
	// BatchID is an opaque, caller-defined batch/import identifier this
	// entry belongs to. Optional.
	BatchID string `json:"batch_id,omitempty"`
	// ExternalRef is an opaque pointer to this entry's origin in an external
	// system, distinct from ledger.JournalEntry.ExternalReference (which
	// already exists on the entry itself) — a caller with a second,
	// diagnostics-specific reference source (e.g. a workflow/approval
	// system's own ID) can supply it here instead of forcing it onto the
	// ledger record. Optional.
	ExternalRef string `json:"external_ref,omitempty"`
}

// effectiveTimestamp returns PostedAt if set, otherwise CreatedAt, otherwise
// nil — the "best available timestamp" used by diagnostics that do not
// specifically need the posting/creation distinction (e.g. weekend
// detection). Callers needing the distinction (post-close, business-hours)
// read the specific field directly instead.
func (m EntryMetadata) effectiveTimestamp() *time.Time {
	if m.PostedAt != nil {
		return m.PostedAt
	}
	return m.CreatedAt
}

// PeriodWindow is the caller-supplied explicit analysis period/date
// boundary. This package never guesses a fiscal year, month boundary, close
// date, or reporting period, and never calls time.Now() — see the package
// doc comment's "No hidden current-date dependency" section.
type PeriodWindow struct {
	// Period is the caller's label for this analysis period (e.g. "2025-03",
	// "2025-Q1"), matching ledger.JournalEntry.Period's semantics. Not
	// required to be used for entry selection (see ledger.PeriodRange for
	// that); carried through to Result.Period for display.
	Period string `json:"period"`
	// StartDate and EndDate bound the analysis window, in "YYYY-MM-DD" form
	// (matching ledger.JournalEntry.Date's convention). Both required and
	// StartDate must be <= EndDate — see IssueInvalidPeriod.
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	// CloseDate is when this period was (or will be) closed, if known.
	// Optional; only supplied when the caller actually knows a close date —
	// this package never guesses one. Required for post-close diagnostics
	// (see PostCloseAvailable in RuleAvailability).
	CloseDate *string `json:"close_date,omitempty"`
}

// Valid reports whether w's StartDate/EndDate are both set and consistent
// (StartDate <= EndDate).
func (w PeriodWindow) Valid() bool {
	if w.StartDate == "" || w.EndDate == "" {
		return false
	}
	return w.StartDate <= w.EndDate
}

// AccountReviewPolicy is one caller-supplied set of accounts flagged for
// heightened review attention (e.g. cash accounts, clearing accounts,
// suspense accounts, related-party accounts, management-override accounts).
// This package never infers sensitive-account status from an Account's Name
// or Number — see FindingSensitiveAccountEntry.
type AccountReviewPolicy struct {
	// Label is a caller-defined display name for this account set (e.g.
	// "Cash accounts", "Related-party accounts").
	Label string `json:"label"`
	// AccountIDs are the ledger.Account.ID values in this set.
	AccountIDs []string `json:"account_ids"`
}
