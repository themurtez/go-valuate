// Package ledger implements a reusable, application-independent general
// ledger / trial balance domain engine: a portable chart of accounts,
// journal entries and lines, account balance calculation, trial-balance
// construction (from journal entries or from an imported trial balance with
// no journal detail at all), account hierarchy/rollups, and structured
// validation.
//
// This package knows nothing about persistence, HTTP, auth, background
// jobs, QuickBooks/Xero or any other external accounting system, AI, or tax
// law. It consumes and produces plain Go structs that are JSON-compatible,
// so callers can serialize/deserialize freely at their own boundaries. It
// also does not map accounts to financial.Code or produce a
// financial.FinancialDataset — that mapping is a separate, later concern
// (see the package doc comment's "Boundary to the existing financial
// model" section below).
//
// Every exported function in this package is pure: no I/O, no mutation of
// caller-owned input (Account, JournalEntry, JournalLine, and everything
// they contain are never modified in place — see mutation_test.go), no
// package-global mutable state. Every function can be called concurrently
// and repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
//
// # Debit/credit and sign convention
//
// Every JournalLine carries explicit, separate Debit and Credit fields
// (never a single signed amount) — this is the same explicit-field
// convention double-entry bookkeeping itself uses, and it lets validation
// catch a line that mistakenly has both set (see IssueInvalidDebitCredit)
// rather than silently collapsing them into one number first.
//
// Once amounts are aggregated into a Balance, this package uses a single
// canonical raw sign convention: debit-positive, credit-negative. A debit
// movement is positive, a credit movement is negative, and
// Balance.RawBalance is opening net + period debits - period credits. This
// is a deliberate choice (not the only valid one) documented once here so
// every field that follows it can simply say "raw convention" instead of
// repeating the rule. Separately, Balance.DisplayBalance flips the sign for
// natural-credit account types (LIABILITY, EQUITY, REVENUE) so it always
// reads as a positive number when an account is in its normal position —
// see NormalBalance and Balance's doc comment.
//
// # Boundary to the existing financial model
//
//	accounting/ledger
//	      ↓
//	Prompt 38 statement mapper/builder
//	      ↓
//	financial.FinancialDataset
//
// This package is the accounting-operations layer beneath
// financial.FinancialDataset: it models the general ledger and trial
// balance a bookkeeping system produces, before that data has been mapped
// to the canonical financial.Code taxonomy or aggregated into
// financial.NormalizedItem values. A future statement mapper/builder
// (Prompt 38, not implemented here) is expected to consume a
// ledger.TrialBalance (or ledger.Balance slice) and produce
// financial.RawLineItem/financial.MappedLineItem values by mapping each
// Account to a financial.Code — the same role ingestion/csv,
// ingestion/xlsx, and ingestion/pdf already play for their respective
// source formats. Nothing in this package imports the financial package,
// and nothing in this package guesses at that mapping.
package ledger

import "math"

// AccountType is the stable high-level classification of an Account,
// determining its NormalBalance. These five values are fixed and are not
// expected to grow — every account in a double-entry system is one of
// these.
type AccountType string

const (
	AccountAsset     AccountType = "ASSET"
	AccountLiability AccountType = "LIABILITY"
	AccountEquity    AccountType = "EQUITY"
	AccountRevenue   AccountType = "REVENUE"
	AccountExpense   AccountType = "EXPENSE"
)

// DebitCredit identifies one side of a double-entry posting.
type DebitCredit string

const (
	Debit  DebitCredit = "DEBIT"
	Credit DebitCredit = "CREDIT"
)

// NormalBalance returns the side (DEBIT or CREDIT) on which AccountType t
// naturally increases, per fixed double-entry rules:
//
//	ASSET      -> DEBIT
//	EXPENSE    -> DEBIT
//	LIABILITY  -> CREDIT
//	EQUITY     -> CREDIT
//	REVENUE    -> CREDIT
//
// This is a fixed table, not a heuristic — it never inspects an account's
// Name, Number, or any other field. An unrecognized AccountType returns
// ("", false).
func NormalBalance(t AccountType) (DebitCredit, bool) {
	switch t {
	case AccountAsset, AccountExpense:
		return Debit, true
	case AccountLiability, AccountEquity, AccountRevenue:
		return Credit, true
	default:
		return "", false
	}
}

// EntryStatus is the posting lifecycle state of a JournalEntry.
type EntryStatus string

const (
	// StatusDraft means the entry has not been posted and does not affect
	// any Balance or TrialBalance by default.
	StatusDraft EntryStatus = "DRAFT"
	// StatusPosted means the entry is final and affects balances.
	StatusPosted EntryStatus = "POSTED"
	// StatusVoided means the entry was voided and never affects balances,
	// regardless of whether it was posted before being voided.
	StatusVoided EntryStatus = "VOIDED"
	// StatusReversed marks an entry that has been superseded by an explicit
	// reversing entry (see Reversal). A StatusReversed entry still affects
	// balances itself (the reversal is a separate JournalEntry that nets it
	// out) — this status is informational, distinguishing "this posting was
	// deliberately undone via a reversal" from a plain StatusPosted entry.
	StatusReversed EntryStatus = "REVERSED"
)

// PostedStatuses returns the EntryStatus values that affect balances by
// default: StatusPosted and StatusReversed. StatusDraft never affects
// balances (not yet final); StatusVoided never affects balances (explicitly
// nullified) — see IncludeStatuses in balances.go for how a caller can
// widen or narrow this set.
func PostedStatuses() []EntryStatus {
	return []EntryStatus{StatusPosted, StatusReversed}
}

// Reversal records an explicit reversal relationship between two
// JournalEntry values. Reversal is never inferred from equal-and-opposite
// amounts — a caller must state it explicitly by setting this field on
// both the original and the reversing entry.
type Reversal struct {
	// ReversalOfEntryID is the JournalEntry.ID this entry reverses. Set only
	// on the reversing entry.
	ReversalOfEntryID string `json:"reversal_of_entry_id,omitempty"`
	// ReversedByEntryID is the JournalEntry.ID of the entry that reverses
	// this one. Set only on the original (now-reversed) entry.
	ReversedByEntryID string `json:"reversed_by_entry_id,omitempty"`
}

// IsReversal reports whether r declares any reversal relationship.
func (r Reversal) IsReversal() bool {
	return r.ReversalOfEntryID != "" || r.ReversedByEntryID != ""
}

// Dimension is one lightweight, optional analysis tag on a JournalLine —
// department, location, class, project/job, customer, or vendor. Key is an
// open string rather than a closed enum so a caller can use whatever
// dimension taxonomy its own chart of accounts needs; the Dimension* key
// constants below list the conventional keys this package's own
// validation/tests use, but nothing in this package rejects an
// unrecognized key.
type Dimension struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Conventional Dimension.Key values. These are suggestions, not a closed
// set — see Dimension's doc comment.
const (
	DimensionDepartment string = "department"
	DimensionLocation   string = "location"
	DimensionClass      string = "class"
	DimensionProject    string = "project"
	DimensionCustomer   string = "customer"
	DimensionVendor     string = "vendor"
)

// isNonFinite reports whether v is NaN or +/-Inf — the shared guard every
// money-bearing field in this package is checked against, per the "reject
// NaN/Inf" money-safety rule.
func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}
