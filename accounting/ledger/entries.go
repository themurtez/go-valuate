package ledger

// JournalLine is a single debit or credit posting within a JournalEntry.
type JournalLine struct {
	// ID is a caller-assigned identifier for this line, unique within its
	// JournalEntry. Optional — many callers never need to address an
	// individual line — but required if the caller wants provenance to
	// trace a Balance back to a specific line (see BalanceProvenance) or
	// wants duplicate-line detection (see IssueDuplicateLine, which only
	// fires for non-empty IDs).
	ID string `json:"id,omitempty"`
	// AccountID is the Account.ID this line posts to. Required.
	AccountID string `json:"account_id"`
	// Debit is the debit amount for this line. Must be >= 0. Exactly one of
	// Debit/Credit should be nonzero on a valid line — see
	// IssueInvalidDebitCredit.
	Debit float64 `json:"debit"`
	// Credit is the credit amount for this line. Must be >= 0.
	Credit float64 `json:"credit"`
	// Memo is an optional free-text note on this specific line.
	Memo string `json:"memo,omitempty"`
	// Dimensions is zero or more optional analysis tags on this line (e.g.
	// department, location, class, project, customer, vendor). Preserved in
	// input order; this package never reorders or deduplicates it.
	Dimensions []Dimension `json:"dimensions,omitempty"`
	// SourceRef is an opaque caller-defined reference to where this line
	// originated (e.g. an external system's line/transaction ID, an
	// imported-file row identifier). This package never interprets it —
	// see the package doc comment's provenance section.
	SourceRef string `json:"source_ref,omitempty"`
}

// JournalEntry is one double-entry accounting transaction: a set of
// balanced JournalLine postings sharing a date/period, description, and
// status.
type JournalEntry struct {
	// ID is a caller-assigned identifier unique within a Ledger. Required.
	ID string `json:"id"`
	// Date is the entry's posting date, in "YYYY-MM-DD" form (ISO 8601
	// calendar date), matching financial.Period's plain-string convention —
	// this package does not parse it into a time.Time, so callers are free
	// to use any consistent lexically-sortable date format. At least one of
	// Date or Period is required.
	Date string `json:"date,omitempty"`
	// Period is the caller's reporting period this entry belongs to (e.g.
	// "2025", "2025-Q1", "2025-03"), reusing financial.Period's semantics —
	// see periods.go. At least one of Date or Period is required.
	Period string `json:"period,omitempty"`
	// Description is a human-readable summary of the transaction.
	Description string `json:"description,omitempty"`
	// Source identifies what produced this entry (e.g. "manual",
	// "accounts_payable", "payroll_import"). Open string, caller-defined.
	Source string `json:"source,omitempty"`
	// Reference is a caller-defined reference/number for this entry (e.g. a
	// check number, invoice number, or internal journal number).
	Reference string `json:"reference,omitempty"`
	// ExternalReference is an opaque pointer to this entry's origin in an
	// external system, kept distinct from Reference (a human-facing
	// document number) — see the package doc comment's provenance section.
	// This package never interprets it.
	ExternalReference string `json:"external_reference,omitempty"`
	// Status is this entry's posting lifecycle state. The zero value ("")
	// is treated as StatusDraft by every function in this package — an
	// entry a caller forgets to set Status on does not silently affect
	// balances.
	Status EntryStatus `json:"status,omitempty"`
	// Reversal declares an explicit reversal relationship with another
	// entry, if any — see Reversal's doc comment. Never inferred.
	Reversal Reversal `json:"reversal,omitempty"`
	// Lines is this entry's debit/credit postings, in caller-supplied
	// order. A StatusPosted or StatusReversed entry must have at least 2
	// lines — see IssueTooFewLines.
	Lines []JournalLine `json:"lines,omitempty"`
}

// EffectiveStatus returns e.Status, treating the zero value as StatusDraft
// — see JournalEntry.Status's doc comment.
func (e JournalEntry) EffectiveStatus() EntryStatus {
	if e.Status == "" {
		return StatusDraft
	}
	return e.Status
}

// AffectsBalances reports whether an entry with status s should be
// included in balance/trial-balance calculation under the default policy:
// POSTED and REVERSED entries affect balances; DRAFT and VOIDED do not.
func AffectsBalances(s EntryStatus) bool {
	switch s {
	case StatusPosted, StatusReversed:
		return true
	default:
		return false
	}
}

// TotalDebits returns the sum of e.Lines' Debit amounts, skipping any
// non-finite value (callers should run ValidateEntries first to be told
// about those explicitly via IssueNonFiniteAmount).
func (e JournalEntry) TotalDebits() float64 {
	var total float64
	for _, l := range e.Lines {
		if !isNonFinite(l.Debit) {
			total += l.Debit
		}
	}
	return total
}

// TotalCredits returns the sum of e.Lines' Credit amounts, skipping any
// non-finite value.
func (e JournalEntry) TotalCredits() float64 {
	var total float64
	for _, l := range e.Lines {
		if !isNonFinite(l.Credit) {
			total += l.Credit
		}
	}
	return total
}

// IsBalanced reports whether e's total debits equal its total credits
// within tolerance (absolute difference). tolerance must be >= 0; a
// negative tolerance is treated as 0.
func (e JournalEntry) IsBalanced(tolerance float64) bool {
	if tolerance < 0 {
		tolerance = 0
	}
	diff := e.TotalDebits() - e.TotalCredits()
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}

// InactivePostingPolicy controls whether ValidateEntries treats a posting
// to an inactive Account as an error or a warning.
type InactivePostingPolicy string

const (
	// InactivePostingWarning (the default, used when the zero value is
	// supplied) reports IssueInactiveAccountPosting as SeverityWarning.
	InactivePostingWarning InactivePostingPolicy = "warning"
	// InactivePostingError reports IssueInactiveAccountPosting as
	// SeverityError.
	InactivePostingError InactivePostingPolicy = "error"
)

// ValidateOptions configures ValidateEntries.
type ValidateOptions struct {
	// Tolerance is the maximum allowed absolute difference between an
	// entry's total debits and total credits before IssueUnbalancedEntry
	// fires. Must be >= 0; a negative value is treated as 0. The zero value
	// means exact equality is required — callers working with
	// floating-point money that has accumulated rounding error should set
	// an explicit small tolerance (e.g. 0.01) rather than relying on the
	// zero-value default silently being forgiving, since it is not.
	Tolerance float64
	// InactivePostingPolicy controls the severity of
	// IssueInactiveAccountPosting. The zero value is treated as
	// InactivePostingWarning.
	InactivePostingPolicy InactivePostingPolicy
}

func (o ValidateOptions) tolerance() float64 {
	if o.Tolerance < 0 {
		return 0
	}
	return o.Tolerance
}

func (o ValidateOptions) inactiveSeverity() IssueSeverity {
	if o.InactivePostingPolicy == InactivePostingError {
		return SeverityError
	}
	return SeverityWarning
}

// ValidateEntries checks entries against chart for every structural and
// balance rule this package enforces: entry ID presence and uniqueness,
// required date/period, minimum line count for posted entries, unknown
// accounts, invalid debit/credit shape (both set, negative, non-finite),
// debit==credit within tolerance, inactive-account posting policy,
// duplicate line IDs within an entry, and reversal-reference consistency
// (see reversals.go). It does not mutate entries or chart. Issues are
// returned in a deterministic order: one pass per entry in input order,
// each entry's own issues in a fixed check order, followed by
// cross-entry reversal-consistency issues.
func ValidateEntries(entries []JournalEntry, chart ChartOfAccounts, opts ValidateOptions) []Issue {
	var issues []Issue

	seenEntryIDs := make(map[string]bool, len(entries))
	for _, e := range entries {
		issues = append(issues, validateSingleEntry(e, chart, opts, seenEntryIDs)...)
	}

	issues = append(issues, validateReversals(entries)...)
	return issues
}

func validateSingleEntry(e JournalEntry, chart ChartOfAccounts, opts ValidateOptions, seenEntryIDs map[string]bool) []Issue {
	var issues []Issue

	if e.ID == "" {
		issues = append(issues, Issue{
			Code:     IssueMissingEntryID,
			Severity: SeverityError,
			Message:  "journal entry has an empty ID",
		})
	} else if seenEntryIDs[e.ID] {
		issues = append(issues, Issue{
			Code:     IssueDuplicateEntry,
			Severity: SeverityError,
			Message:  "duplicate entry ID: " + e.ID,
			Entry:    e.ID,
		})
	} else {
		seenEntryIDs[e.ID] = true
	}

	if e.Date == "" && e.Period == "" {
		issues = append(issues, Issue{
			Code:     IssueInvalidPeriod,
			Severity: SeverityError,
			Message:  "entry " + e.ID + " has neither Date nor Period set",
			Entry:    e.ID,
		})
	}

	status := e.EffectiveStatus()
	requiresBalance := status == StatusPosted || status == StatusReversed
	if requiresBalance && len(e.Lines) < 2 {
		issues = append(issues, Issue{
			Code:     IssueTooFewLines,
			Severity: SeverityError,
			Message:  "posted entry " + e.ID + " has fewer than 2 lines",
			Entry:    e.ID,
		})
	} else if len(e.Lines) < 2 {
		issues = append(issues, Issue{
			Code:     IssueTooFewLines,
			Severity: SeverityWarning,
			Message:  "draft entry " + e.ID + " has fewer than 2 lines",
			Entry:    e.ID,
		})
	}

	seenLineIDs := make(map[string]bool, len(e.Lines))
	for i, line := range e.Lines {
		ref := lineRef(line, i)

		if line.ID != "" {
			if seenLineIDs[line.ID] {
				issues = append(issues, Issue{
					Code:     IssueDuplicateLine,
					Severity: SeverityError,
					Message:  "duplicate line ID " + line.ID + " in entry " + e.ID,
					Entry:    e.ID,
					Line:     line.ID,
				})
			}
			seenLineIDs[line.ID] = true
		}

		if _, ok := chart.Lookup(line.AccountID); !ok {
			issues = append(issues, Issue{
				Code:     IssueUnknownAccount,
				Severity: SeverityError,
				Message:  "entry " + e.ID + " line " + ref + " references unknown account: " + line.AccountID,
				Entry:    e.ID,
				Line:     ref,
				Account:  line.AccountID,
			})
		} else if acct, _ := chart.Lookup(line.AccountID); !acct.Active {
			issues = append(issues, Issue{
				Code:     IssueInactiveAccountPosting,
				Severity: opts.inactiveSeverity(),
				Message:  "entry " + e.ID + " line " + ref + " posts to inactive account: " + line.AccountID,
				Entry:    e.ID,
				Line:     ref,
				Account:  line.AccountID,
			})
		}

		debitNonFinite := isNonFinite(line.Debit)
		creditNonFinite := isNonFinite(line.Credit)
		if debitNonFinite || creditNonFinite {
			issues = append(issues, Issue{
				Code:     IssueNonFiniteAmount,
				Severity: SeverityError,
				Message:  "entry " + e.ID + " line " + ref + " has a non-finite debit/credit amount",
				Entry:    e.ID,
				Line:     ref,
				Account:  line.AccountID,
			})
			continue
		}

		if line.Debit < 0 || line.Credit < 0 {
			issues = append(issues, Issue{
				Code:     IssueNegativeAmount,
				Severity: SeverityError,
				Message:  "entry " + e.ID + " line " + ref + " has a negative debit or credit",
				Entry:    e.ID,
				Line:     ref,
				Account:  line.AccountID,
			})
			continue
		}

		bothSet := line.Debit != 0 && line.Credit != 0
		neitherSet := line.Debit == 0 && line.Credit == 0
		if bothSet {
			issues = append(issues, Issue{
				Code:     IssueInvalidDebitCredit,
				Severity: SeverityError,
				Message:  "entry " + e.ID + " line " + ref + " has both debit and credit set",
				Entry:    e.ID,
				Line:     ref,
				Account:  line.AccountID,
			})
		} else if neitherSet && requiresBalance {
			issues = append(issues, Issue{
				Code:     IssueInvalidDebitCredit,
				Severity: SeverityWarning,
				Message:  "entry " + e.ID + " line " + ref + " has neither debit nor credit set",
				Entry:    e.ID,
				Line:     ref,
				Account:  line.AccountID,
			})
		}
	}

	if requiresBalance && len(e.Lines) >= 2 && !e.IsBalanced(opts.tolerance()) {
		issues = append(issues, Issue{
			Code:     IssueUnbalancedEntry,
			Severity: SeverityError,
			Message:  "entry " + e.ID + " total debits do not equal total credits within tolerance",
			Entry:    e.ID,
		})
	}

	return issues
}

// validateReversals checks that every Reversal reference declared in
// entries resolves to a real entry and that both ends of a reversal
// relationship agree with each other — see IssueInvalidReversal. Issues
// are returned in entries' input order.
func validateReversals(entries []JournalEntry) []Issue {
	byID := make(map[string]JournalEntry, len(entries))
	for _, e := range entries {
		if e.ID != "" {
			byID[e.ID] = e
		}
	}

	var issues []Issue
	for _, e := range entries {
		if e.Reversal.ReversalOfEntryID != "" {
			orig, ok := byID[e.Reversal.ReversalOfEntryID]
			switch {
			case !ok:
				issues = append(issues, Issue{
					Code:     IssueInvalidReversal,
					Severity: SeverityError,
					Message:  "entry " + e.ID + " claims to reverse unknown entry " + e.Reversal.ReversalOfEntryID,
					Entry:    e.ID,
				})
			case orig.Reversal.ReversedByEntryID != e.ID:
				issues = append(issues, Issue{
					Code:     IssueInvalidReversal,
					Severity: SeverityError,
					Message:  "entry " + e.ID + " claims to reverse " + orig.ID + ", but " + orig.ID + " does not claim to be reversed by it",
					Entry:    e.ID,
				})
			}
		}
		if e.Reversal.ReversedByEntryID != "" {
			rev, ok := byID[e.Reversal.ReversedByEntryID]
			switch {
			case !ok:
				issues = append(issues, Issue{
					Code:     IssueInvalidReversal,
					Severity: SeverityError,
					Message:  "entry " + e.ID + " claims to be reversed by unknown entry " + e.Reversal.ReversedByEntryID,
					Entry:    e.ID,
				})
			case rev.Reversal.ReversalOfEntryID != e.ID:
				issues = append(issues, Issue{
					Code:     IssueInvalidReversal,
					Severity: SeverityError,
					Message:  "entry " + e.ID + " claims to be reversed by " + rev.ID + ", but " + rev.ID + " does not claim to reverse it",
					Entry:    e.ID,
				})
			}
		}
	}
	return issues
}
