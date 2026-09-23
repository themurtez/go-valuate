package closequality

// FindingCode is a stable identifier for one close-quality condition.
// Only codes this package actually emits are defined here — see each
// mine_*.go file for which code(s) it produces.
type FindingCode string

const (
	FindingLedgerValidationBlocker        FindingCode = "LEDGER_VALIDATION_BLOCKER"
	FindingTrialBalanceOutOfBalance       FindingCode = "TRIAL_BALANCE_OUT_OF_BALANCE"
	FindingMaterialUnmappedAccounts       FindingCode = "MATERIAL_UNMAPPED_ACCOUNTS"
	FindingBalanceSheetOutOfBalance       FindingCode = "BALANCE_SHEET_OUT_OF_BALANCE"
	FindingStatementBuildInvalid          FindingCode = "STATEMENT_BUILD_INVALID"
	FindingARAgingNotReconciled           FindingCode = "AR_AGING_NOT_RECONCILED"
	FindingARControlMismatch              FindingCode = "AR_CONTROL_MISMATCH"
	FindingAPAgingNotReconciled           FindingCode = "AP_AGING_NOT_RECONCILED"
	FindingAPControlMismatch              FindingCode = "AP_CONTROL_MISMATCH"
	FindingMaterialJournalReviewRequired  FindingCode = "MATERIAL_JOURNAL_REVIEW_REQUIRED"
	FindingPostCloseActivity              FindingCode = "POST_CLOSE_ACTIVITY"
	FindingUnexpectedAccountBalance       FindingCode = "UNEXPECTED_ACCOUNT_BALANCE"
	FindingClearingAccountNotCleared      FindingCode = "CLEARING_ACCOUNT_NOT_CLEARED"
	FindingStaleAccountBalance            FindingCode = "STALE_ACCOUNT_BALANCE"
	FindingBalanceRollforwardMismatch     FindingCode = "BALANCE_ROLLFORWARD_MISMATCH"
	FindingUnreconciledCriticalAccount    FindingCode = "UNRECONCILED_CRITICAL_ACCOUNT"
	FindingReconciliationItemsOutstanding FindingCode = "RECONCILIATION_ITEMS_OUTSTANDING"
	FindingRequiredCloseTaskIncomplete    FindingCode = "REQUIRED_CLOSE_TASK_INCOMPLETE"
	FindingRequiredCloseTaskBlocked       FindingCode = "REQUIRED_CLOSE_TASK_BLOCKED"
	FindingExpectedPeriodEntryMissing     FindingCode = "EXPECTED_PERIOD_ENTRY_MISSING"
	FindingMissingRequiredInput           FindingCode = "MISSING_REQUIRED_INPUT"
	FindingEquityRollforwardReview        FindingCode = "EQUITY_ROLLFORWARD_REVIEW"
)

// findingOrder is the fixed declaration/output order for FindingCode used
// as a sort tiebreak — never Go map order.
var findingOrder = []FindingCode{
	FindingLedgerValidationBlocker,
	FindingTrialBalanceOutOfBalance,
	FindingMaterialUnmappedAccounts,
	FindingBalanceSheetOutOfBalance,
	FindingStatementBuildInvalid,
	FindingARAgingNotReconciled,
	FindingARControlMismatch,
	FindingAPAgingNotReconciled,
	FindingAPControlMismatch,
	FindingMaterialJournalReviewRequired,
	FindingPostCloseActivity,
	FindingUnexpectedAccountBalance,
	FindingClearingAccountNotCleared,
	FindingStaleAccountBalance,
	FindingBalanceRollforwardMismatch,
	FindingUnreconciledCriticalAccount,
	FindingReconciliationItemsOutstanding,
	FindingRequiredCloseTaskIncomplete,
	FindingRequiredCloseTaskBlocked,
	FindingExpectedPeriodEntryMissing,
	FindingMissingRequiredInput,
	FindingEquityRollforwardReview,
}

var findingRank = func() map[FindingCode]int {
	m := make(map[FindingCode]int, len(findingOrder))
	for i, c := range findingOrder {
		m[c] = i
	}
	return m
}()

// Evidence carries typed, finding-specific supporting figures so no
// finding's evidence lives only in Message text. Only the sub-fields
// relevant to a given Finding.Code are populated.
type Evidence struct {
	SubledgerBalance       Value  `json:"subledger_balance,omitempty"`
	ControlBalance         Value  `json:"control_balance,omitempty"`
	Difference             Value  `json:"difference,omitempty"`
	Tolerance              Value  `json:"tolerance,omitempty"`
	EndingBalance          Value  `json:"ending_balance,omitempty"`
	ExpectedCondition      string `json:"expected_condition,omitempty"`
	MaterialityThreshold   Value  `json:"materiality_threshold,omitempty"`
	TaskCode               string `json:"task_code,omitempty"`
	Required               bool   `json:"required,omitempty"`
	TaskStatus             string `json:"task_status,omitempty"`
	TotalAmount            Value  `json:"total_amount,omitempty"`
	CloseDate              string `json:"close_date,omitempty"`
	UnresolvedCount        Value  `json:"unresolved_count,omitempty"`
	OldestOpenDate         string `json:"oldest_open_date,omitempty"`
	AsOfDate               string `json:"as_of_date,omitempty"`
	OpeningBalance         Value  `json:"opening_balance,omitempty"`
	Movement               Value  `json:"movement,omitempty"`
	ClosingBalance         Value  `json:"closing_balance,omitempty"`
	ExpectedClosingBalance Value  `json:"expected_closing_balance,omitempty"`
	Label                  string `json:"label,omitempty"`
}

// Finding is a single close/bookkeeping condition needing review, with
// full provenance back to whichever upstream module (if any) it was
// translated from.
type Finding struct {
	Code      FindingCode `json:"code"`
	Dimension Dimension   `json:"dimension"`
	Severity  Severity    `json:"severity"`
	Message   string      `json:"message"`
	Evidence  Evidence    `json:"evidence"`

	// SourceModule/SourceCode preserve provenance: which upstream package
	// (if any) and which of its own issue/finding codes this Finding was
	// translated from. Both are empty when the Finding originates
	// entirely within this package (e.g. reconciliation coverage, close
	// task completion).
	SourceModule SourceModule `json:"source_module,omitempty"`
	SourceCode   string       `json:"source_code,omitempty"`

	// AdditionalSources lists any other (SourceModule, SourceCode)
	// references deduplicated into this same Finding — see dedupKey.
	// Empty unless deduplication actually merged more than one upstream
	// reference into this Finding.
	AdditionalSources []SourceRef `json:"additional_sources,omitempty"`

	// AccountIDs and EntryIDs are sorted ascending, identifying the
	// specific accounts/entries this Finding concerns, when applicable.
	AccountIDs []string `json:"account_ids,omitempty"`
	EntryIDs   []string `json:"entry_ids,omitempty"`
}

// SourceRef is one upstream provenance reference folded into a Finding's
// AdditionalSources during deduplication.
type SourceRef struct {
	SourceModule SourceModule `json:"source_module"`
	SourceCode   string       `json:"source_code,omitempty"`
}

// dedupKey is the stable key used to suppress duplicate Findings that
// clearly describe the same underlying condition surfaced through more
// than one upstream module (see docs/CLOSE_QUALITY.md's deduplication
// section). Two Findings with the same (Code, Dimension, first
// AccountID) are considered the same condition; the first-seen Finding
// (in mine* run order, itself deterministic) wins and later duplicates'
// extra SourceModule/SourceCode references are folded into
// AdditionalSources rather than discarded.
type dedupKey struct {
	code      FindingCode
	dimension Dimension
	accountID string
}

func keyFor(f Finding) dedupKey {
	acct := ""
	if len(f.AccountIDs) > 0 {
		acct = f.AccountIDs[0]
	}
	return dedupKey{code: f.Code, dimension: f.Dimension, accountID: acct}
}
