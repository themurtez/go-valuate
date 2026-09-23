package ledger

import "strconv"

// IssueSeverity distinguishes a problem that blocks some portion of the
// analysis (SeverityError) from one that is advisory only
// (SeverityWarning) — the same two-severity model every analytics package
// in this repository uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of validation/integrity
// finding. This package defines its own separate taxonomy, consistent with
// every other package in this repository, rather than reusing one of
// theirs. Only codes this package actually emits are defined — see each
// constant's doc comment for exactly when it fires.
type IssueCode string

const (
	// IssueUnbalancedEntry means a JournalEntry's total debits do not equal
	// its total credits within the configured Tolerance.
	IssueUnbalancedEntry IssueCode = "UNBALANCED_ENTRY"
	// IssueUnbalancedTrialBalance means a TrialBalance's (or
	// NormalizedTrialBalance's) total debits do not equal its total credits
	// within the configured Tolerance. Never auto-corrected — see
	// TrialBalance.Balanced's doc comment.
	IssueUnbalancedTrialBalance IssueCode = "UNBALANCED_TRIAL_BALANCE"
	// IssueUnknownAccount means a JournalLine, TrialBalanceInput line, or
	// OpeningBalance referenced an account ID not present in the supplied
	// chart of accounts (or, for ValidateAccounts, an Account had an empty
	// ID).
	IssueUnknownAccount IssueCode = "UNKNOWN_ACCOUNT"
	// IssueDuplicateAccount means two or more Account values in the same
	// chart share the same ID.
	IssueDuplicateAccount IssueCode = "DUPLICATE_ACCOUNT"
	// IssueDuplicateEntry means two or more JournalEntry values in the same
	// Ledger share the same ID.
	IssueDuplicateEntry IssueCode = "DUPLICATE_ENTRY"
	// IssueDuplicateLine means two or more JournalLine values within the
	// same JournalEntry share the same non-empty ID.
	IssueDuplicateLine IssueCode = "DUPLICATE_LINE"
	// IssueInvalidDebitCredit means a JournalLine or TrialBalanceInput line
	// has both Debit and Credit nonzero, or has neither nonzero on a
	// StatusPosted entry line.
	IssueInvalidDebitCredit IssueCode = "INVALID_DEBIT_CREDIT"
	// IssueNegativeAmount means a Debit, Credit, or opening-balance amount
	// is negative. This package never accepts a negative debit/credit —
	// direction is expressed by which field (Debit vs Credit) is nonzero,
	// not by sign.
	IssueNegativeAmount IssueCode = "NEGATIVE_AMOUNT"
	// IssueNonFiniteAmount means a money-bearing field was NaN or +/-Inf.
	// The offending value is treated as if it were absent/zero for
	// downstream calculation, never propagated.
	IssueNonFiniteAmount IssueCode = "NON_FINITE_AMOUNT"
	// IssueInactiveAccountPosting means a JournalLine posts to an Account
	// with Active == false. Whether this is an error or a warning depends
	// on the caller's InactivePostingPolicy — see validate.go.
	IssueInactiveAccountPosting IssueCode = "INACTIVE_ACCOUNT_POSTING"
	// IssueAccountHierarchyCycle means following Account.ParentID links from
	// some account eventually leads back to itself.
	IssueAccountHierarchyCycle IssueCode = "ACCOUNT_HIERARCHY_CYCLE"
	// IssueMissingParentAccount means an Account.ParentID refers to an
	// account ID not present in the chart.
	IssueMissingParentAccount IssueCode = "MISSING_PARENT_ACCOUNT"
	// IssueMixedCurrency means an aggregation (trial balance, rollup, or
	// balance range) was asked to combine accounts/lines with more than one
	// Currency without the caller supplying already-converted values or
	// explicit rates. The mismatched items are excluded from the
	// aggregate total, never silently summed across currencies.
	IssueMixedCurrency IssueCode = "MIXED_CURRENCY"
	// IssueInvalidPeriod means a requested period/date range was invalid
	// (e.g. an end date before a start date) or a JournalEntry had neither
	// Date nor Period set.
	IssueInvalidPeriod IssueCode = "INVALID_PERIOD"
	// IssueInvalidReversal means a Reversal reference did not resolve: the
	// referenced entry ID does not exist in the Ledger, or the two ends of
	// a reversal relationship do not agree with each other (e.g. A claims
	// to be reversed by B, but B does not claim to reverse A).
	IssueInvalidReversal IssueCode = "INVALID_REVERSAL"
	// IssueTooFewLines means a StatusPosted (or StatusReversed)
	// JournalEntry has fewer than 2 lines. A DRAFT entry with fewer than 2
	// lines is allowed (still being built) but is still reported as a
	// warning.
	IssueTooFewLines IssueCode = "TOO_FEW_LINES"
	// IssueMissingEntryID means a JournalEntry.ID is empty.
	IssueMissingEntryID IssueCode = "MISSING_ENTRY_ID"
	// IssueInvalidAccount means an Account failed a structural check other
	// than duplication/hierarchy (e.g. unrecognized Type, empty Name).
	IssueInvalidAccount IssueCode = "INVALID_ACCOUNT"
	// IssueInvalidOpeningBalance means an OpeningBalance referenced an
	// unknown account, had a non-finite Amount, or (for
	// NormalizeTrialBalance) an opening balance's implied sign disagreed
	// with its account's currency/consistency checks.
	IssueInvalidOpeningBalance IssueCode = "INVALID_OPENING_BALANCE"
)

// Issue is a single structured validation/integrity finding, returned
// instead of an error so callers can inspect every problem found in one
// pass rather than stopping at the first one.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// Entry identifies the JournalEntry.ID this Issue relates to, if any.
	Entry string `json:"entry,omitempty"`
	// Line identifies the JournalLine.ID (or, if the line has no ID, its
	// zero-based index within Entry formatted as "line[N]") this Issue
	// relates to, if any.
	Line string `json:"line,omitempty"`
	// Account identifies the Account.ID (or, when the account itself is the
	// problem, its Number/best-available identifier) this Issue relates to,
	// if any.
	Account string `json:"account,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from analytics/debt.HasErrors and this
// repository's other sibling HasErrors functions rather than shared — see
// financial/adjustments.HasErrors's doc comment for the full rationale
// (each package's Issue is a distinct Go type with no common interface
// worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// lineRef formats a JournalLine's identity for Issue.Line: its own ID if
// set, otherwise "line[N]" using its zero-based index within the entry.
func lineRef(line JournalLine, index int) string {
	if line.ID != "" {
		return line.ID
	}
	return indexRef(index)
}

func indexRef(index int) string {
	return "line[" + strconv.Itoa(index) + "]"
}
