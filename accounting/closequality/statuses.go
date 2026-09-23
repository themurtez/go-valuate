package closequality

import "time"

// ReconciliationState is the status of an externally- or manually-
// computed account reconciliation. This package consumes reconciliation
// status only — it performs no matching itself; a dedicated
// reconciliation-matching engine is a later, separate package.
type ReconciliationState string

const (
	ReconciliationReconciled          ReconciliationState = "RECONCILED"
	ReconciliationReconciledWithItems ReconciliationState = "RECONCILED_WITH_ITEMS"
	ReconciliationUnreconciled        ReconciliationState = "UNRECONCILED"
	ReconciliationNotRequired         ReconciliationState = "NOT_REQUIRED"
	ReconciliationUnavailable         ReconciliationState = "UNAVAILABLE"
)

// ReconciliationStatus describes one account's externally-computed
// reconciliation outcome as of a point in time. Prompt 49 will build the
// actual matching engine; this type is a lightweight status carrier.
type ReconciliationStatus struct {
	AccountID         string              `json:"account_id"`
	Status            ReconciliationState `json:"status"`
	AsOfDate          string              `json:"as_of_date"`
	BookBalance       Value               `json:"book_balance,omitempty"`
	ReconciledBalance Value               `json:"reconciled_balance,omitempty"`
	Difference        Value               `json:"difference,omitempty"`
	UnresolvedCount   int                 `json:"unresolved_count,omitempty"`
	LastCompletedAt   *string             `json:"last_completed_at,omitempty"` // "YYYY-MM-DD"
	SourceRef         string              `json:"source_ref,omitempty"`
}

// CloseTaskState is the status of one external/manual close checklist
// item. This package tracks status only; it implements no workflow,
// assignment, or execution.
type CloseTaskState string

const (
	CloseTaskNotStarted    CloseTaskState = "NOT_STARTED"
	CloseTaskInProgress    CloseTaskState = "IN_PROGRESS"
	CloseTaskCompleted     CloseTaskState = "COMPLETED"
	CloseTaskNotApplicable CloseTaskState = "NOT_APPLICABLE"
	CloseTaskBlocked       CloseTaskState = "BLOCKED"
)

// CloseTaskStatus describes one external/manual close checklist item's
// current status. OwnerRef is opaque — this package performs no
// user/workflow management.
type CloseTaskStatus struct {
	Code        string         `json:"code"`
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required"`
	Status      CloseTaskState `json:"status"`
	OwnerRef    string         `json:"owner_ref,omitempty"`
	EvidenceRef string         `json:"evidence_ref,omitempty"`
}

// ExpectedSign is a caller's declaration of which balance sign an
// account is expected to carry. This package never infers expected sign
// from account name or even from ledger.AccountType's normal-balance
// convention by default — a caller must declare it explicitly per
// account via AccountExpectation, because opposite-sign balances (an
// overdraft, a credit memo, a contra balance, a reclass) are frequently
// legitimate. See docs/CLOSE_QUALITY.md's balance-sheet-plausibility
// section.
type ExpectedSign string

const (
	ExpectedSignNone     ExpectedSign = ""            // no sign expectation declared
	ExpectedSignPositive ExpectedSign = "POSITIVE"    // must be >= 0 (or > 0 if AllowZero is false)
	ExpectedSignNegative ExpectedSign = "NEGATIVE"    // must be <= 0 (or < 0 if AllowZero is false)
	ExpectedSignNormal   ExpectedSign = "NORMAL_ONLY" // must match ledger.NormalBalance(account.Type)'s display-sign convention (>= 0)
)

// AccountExpectation is a caller-declared rule for one account's expected
// balance behavior, evaluated against ledger.Balance.DisplayBalance (or,
// when ledger is unavailable, against a caller-supplied Value the caller
// resolves themselves — see BalanceSheetQuality dimension notes). Every
// field is opt-in; declaring an AccountExpectation for an account is what
// turns on the checks it names, never account-name inference.
type AccountExpectation struct {
	AccountID     string       `json:"account_id"`
	ExpectedSign  ExpectedSign `json:"expected_sign,omitempty"`
	AllowZero     bool         `json:"allow_zero"`
	AllowNegative bool         `json:"allow_negative"`
	MinBalance    *float64     `json:"min_balance,omitempty"`
	MaxBalance    *float64     `json:"max_balance,omitempty"`
	// ShouldClear marks this a suspense/clearing account: a nonzero
	// (and, if Materiality is set, material) ending balance produces
	// FindingClearingAccountNotCleared.
	ShouldClear bool `json:"should_clear"`
	// MaxAgeDays, combined with a matching BalanceAge entry, flags a
	// stale ShouldClear balance (FindingStaleAccountBalance). Staleness
	// is unavailable without a matching BalanceAge — this package never
	// reconstructs age from the net ledger balance.
	MaxAgeDays  *int     `json:"max_age_days,omitempty"`
	Materiality *float64 `json:"materiality,omitempty"`
	Label       string   `json:"label,omitempty"`
}

// BalanceAge is caller-supplied metadata about how long an account's
// (typically suspense/clearing) balance has been outstanding, since this
// package cannot reconstruct age from a net ledger balance alone.
type BalanceAge struct {
	AccountID      string  `json:"account_id"`
	OldestOpenDate string  `json:"oldest_open_date"` // "YYYY-MM-DD"
	Amount         float64 `json:"amount"`
}

// PriorPeriodBalance is a caller-supplied opening/closing balance pair
// for period-over-period rollforward checks (opening + movement =
// closing). This package reuses ledger.Balance's own OpeningNet/Movement
// when a ledger.Ledger is available; PriorPeriodBalance exists for
// callers who only have externally-sourced balances (e.g. an imported
// trial balance with no ledger).
type PriorPeriodBalance struct {
	AccountID      string  `json:"account_id"`
	OpeningBalance float64 `json:"opening_balance"`
	PeriodMovement float64 `json:"period_movement"`
	ClosingBalance float64 `json:"closing_balance"`
	Tolerance      float64 `json:"tolerance,omitempty"`
}

// ExpectedActivityRule is a caller-declared expectation that a set of
// accounts should show at least some minimum activity during the period
// (e.g. "payroll accounts should show at least one entry"). Entirely
// caller-defined — this package never hard-codes which accounts should
// have activity.
type ExpectedActivityRule struct {
	Label           string   `json:"label"`
	AccountIDs      []string `json:"account_ids"`
	MinEntryCount   int      `json:"min_entry_count,omitempty"`
	MinAbsoluteMove float64  `json:"min_absolute_move,omitempty"`
}

// ExpectedPeriodEntry is a caller-declared expected recurring close
// entry (e.g. "monthly depreciation"), matched deterministically against
// actual ledger activity — never via fuzzy/NLP matching. If any
// declared criterion cannot be evaluated (e.g. no ledger supplied), the
// rule is unavailable rather than treated as unmet.
type ExpectedPeriodEntry struct {
	Label            string   `json:"label"`
	AccountIDs       []string `json:"account_ids,omitempty"`
	Source           string   `json:"source,omitempty"`            // matches ledger.JournalEntry.Source, if set
	DescriptionExact string   `json:"description_exact,omitempty"` // normalized (trimmed, case-folded) exact match against ledger.JournalEntry.Description
	MinAmount        *float64 `json:"min_amount,omitempty"`
	MaxAmount        *float64 `json:"max_amount,omitempty"`
}

func normalizeDescription(s string) string {
	return normalizeSpaceLower(s)
}

// balanceAgeAsOf resolves a BalanceAge's OldestOpenDate into an age in
// days relative to asOf, returning (0, false) if either date fails to
// parse.
func balanceAgeDays(oldestOpenDate, asOf string) (int, bool) {
	const layout = "2006-01-02"
	oldest, err := time.Parse(layout, oldestOpenDate)
	if err != nil {
		return 0, false
	}
	ref, err := time.Parse(layout, asOf)
	if err != nil {
		return 0, false
	}
	days := int(ref.Sub(oldest).Hours() / 24)
	if days < 0 {
		return 0, false
	}
	return days, true
}
