package closequality

// MaterialityPolicy mirrors statements.MaterialityPolicy's shape (an
// intentionally-narrow local copy, per repo convention — see
// journaldiagnostics's Issue-taxonomy doc comment for the "no shared
// cross-package type" rationale). A figure is material if abs(amount) is
// at least AbsoluteAmount, or at least PercentOfAssets/PercentOfRevenue
// of the corresponding base when that base is supplied. Reuses the same
// OR-across-configured-thresholds semantics as review.IsMaterial: when
// every threshold is zero, materiality gating is off (everything is
// material) rather than nothing being material — see isMaterial.
type MaterialityPolicy struct {
	AbsoluteAmount   float64 `json:"absolute_amount,omitempty"`
	PercentOfAssets  float64 `json:"percent_of_assets,omitempty"`
	PercentOfRevenue float64 `json:"percent_of_revenue,omitempty"`
}

// isMaterial reports whether amount is material under p, given optional
// total-assets/revenue bases. When p has no configured threshold at all,
// materiality gating is off and every amount is treated as material —
// matching review.IsMaterial's documented "OFF by default" semantics
// (this package's own local copy of the same OR-logic, per repo
// convention; see financial/adjustments.HasErrors's doc comment for why
// this is duplicated rather than imported).
func isMaterial(amount float64, totalAssets, revenue *float64, p MaterialityPolicy) bool {
	a := abs(amount)
	if p.AbsoluteAmount > 0 && a >= p.AbsoluteAmount {
		return true
	}
	if p.PercentOfAssets > 0 && totalAssets != nil && a >= p.PercentOfAssets*abs(*totalAssets) {
		return true
	}
	if p.PercentOfRevenue > 0 && revenue != nil && a >= p.PercentOfRevenue*abs(*revenue) {
		return true
	}
	if p.AbsoluteAmount <= 0 && p.PercentOfAssets <= 0 && p.PercentOfRevenue <= 0 {
		return true
	}
	return false
}

// JournalFindingDisposition is how a journaldiagnostics.FindingCode
// should be treated when translated into a close-quality Finding.
type JournalFindingDisposition string

const (
	JournalFindingBlocking JournalFindingDisposition = "BLOCKING"
	JournalFindingWarning  JournalFindingDisposition = "WARNING"
	JournalFindingInfo     JournalFindingDisposition = "INFO"
	JournalFindingIgnore   JournalFindingDisposition = "IGNORE"
)

// Applicability declares which optional upstream modules are required
// for this period's close. A missing module that is not required does
// not affect readiness; a missing module that is required does (see
// Policy.TreatMissingRequiredInputAsBlocker and Dimension
// DataCompleteness).
type Applicability struct {
	ARRequired                 bool `json:"ar_required"`
	APRequired                 bool `json:"ap_required"`
	StatementsRequired         bool `json:"statements_required"`
	JournalDiagnosticsRequired bool `json:"journal_diagnostics_required"`
	ReconciliationsRequired    bool `json:"reconciliations_required"`
	CloseTasksRequired         bool `json:"close_tasks_required"`
}

// CloseTaskRules controls how incomplete/blocked required close tasks
// affect readiness.
type CloseTaskRules struct {
	// RequiredIncompleteBlocks: a required task in NOT_STARTED or
	// IN_PROGRESS status produces a BLOCKING finding rather than WARNING.
	RequiredIncompleteBlocks bool `json:"required_incomplete_blocks"`
}

// Policy is every caller-configurable threshold and rule this package
// uses. Nothing that materially changes a Finding or the overall Status
// is a hidden code constant — see docs/CLOSE_QUALITY.md.
type Policy struct {
	Materiality MaterialityPolicy `json:"materiality"`

	// CriticalAccounts are account IDs (matched against
	// ReconciliationStatus.AccountID and AccountExpectation.AccountID)
	// whose unreconciled/uncleared status is treated as blocking
	// regardless of Materiality. Never inferred from account name.
	CriticalAccounts []string `json:"critical_accounts,omitempty"`

	// RequiredReconciliationAccounts are account IDs that must have a
	// ReconciliationStatus entry for ReconciliationCoverage to reach
	// full coverage. An account not listed here is simply not counted
	// in the coverage denominator (see Coverage.RequiredReconciliationAccounts).
	RequiredReconciliationAccounts []string `json:"required_reconciliation_accounts,omitempty"`

	// MaxAllowedUnreconciledItems: a RECONCILED_WITH_ITEMS status with
	// UnresolvedCount greater than this produces
	// FindingReconciliationItemsOutstanding. Zero means any unresolved
	// item at all triggers it.
	MaxAllowedUnreconciledItems int `json:"max_allowed_unreconciled_items,omitempty"`

	// JournalFindingRules maps a journaldiagnostics.FindingCode (by its
	// string value) to how it should be translated. A code with no entry
	// falls back to DefaultJournalFindingRules's disposition for that
	// code's severity, or JournalFindingInfo if wholly unlisted.
	JournalFindingRules map[string]JournalFindingDisposition `json:"journal_finding_rules,omitempty"`

	Applicability Applicability `json:"applicability"`

	// TreatMissingRequiredInputAsBlocker: when true (the default
	// recommendation — see DefaultPolicy), a required-but-missing module
	// produces a BLOCKING FindingMissingRequiredInput and forces
	// Status to NOT_READY. When false, it produces a WARNING instead.
	TreatMissingRequiredInputAsBlocker bool `json:"treat_missing_required_input_as_blocker"`

	// BalanceTolerance is the absolute tolerance used for trial-balance
	// and balance-sheet balancing checks when this package must judge
	// "balanced" itself (most such checks instead reuse an upstream
	// package's own Balanced/Reconciled bool, which already encodes that
	// package's own tolerance).
	BalanceTolerance float64 `json:"balance_tolerance,omitempty"`

	CloseTaskRules CloseTaskRules `json:"close_task_rules"`

	// AccountExpectations, keyed by AccountID for lookup; duplicates
	// (same AccountID twice) are reported as an Issue and only the first
	// is used.
	AccountExpectations []AccountExpectation `json:"account_expectations,omitempty"`

	// ExpectedActivityRules and ExpectedPeriodEntries are both entirely
	// caller-declared — never inferred from account names or history.
	ExpectedActivityRules []ExpectedActivityRule `json:"expected_activity_rules,omitempty"`
	ExpectedPeriodEntries []ExpectedPeriodEntry  `json:"expected_period_entries,omitempty"`

	// BalanceAges supplies staleness metadata per account (see BalanceAge).
	BalanceAges []BalanceAge `json:"balance_ages,omitempty"`

	// PriorPeriodBalances supplies rollforward data per account (see
	// PriorPeriodBalance). Optional — rollforward checks are skipped
	// entirely (not flagged unavailable) when not supplied, since a
	// rollforward check is itself opt-in.
	PriorPeriodBalances []PriorPeriodBalance `json:"prior_period_balances,omitempty"`

	// TreatLockedPostCloseAsBlocking: when the period's PeriodInfo.Lock
	// is CLOSED or LOCKED, post-close journal activity produces a
	// BLOCKING finding instead of WARNING.
	TreatLockedPostCloseAsBlocking bool `json:"treat_locked_post_close_as_blocking"`
}

// DefaultJournalFindingRules is the recommended default disposition
// table (Prompt 42 section 11's suggested mapping), used for any
// journaldiagnostics.FindingCode not overridden in
// Policy.JournalFindingRules. Ledger-invalid / material period-end
// entries lean toward review warnings (never automatically blocking,
// since journaldiagnostics itself never asserts wrongdoing); post-close
// activity is handled separately by mine_journaldiagnostics.go via
// PeriodInfo.CloseDate/Lock rather than this table.
func DefaultJournalFindingRules() map[string]JournalFindingDisposition {
	return map[string]JournalFindingDisposition{
		"MATERIAL_MANUAL_ENTRY":                JournalFindingWarning,
		"MATERIAL_PERIOD_END_ENTRY":            JournalFindingWarning,
		"WEEKEND_ENTRY":                        JournalFindingInfo,
		"OUTSIDE_BUSINESS_HOURS":               JournalFindingInfo,
		"ROUND_DOLLAR_ENTRY":                   JournalFindingInfo,
		"LARGE_ENTRY":                          JournalFindingWarning,
		"ACCOUNT_RELATIVE_LARGE_ENTRY":         JournalFindingWarning,
		"RARE_ACCOUNT_ACTIVITY":                JournalFindingInfo,
		"NEW_ACCOUNT_ACTIVITY":                 JournalFindingInfo,
		"OPPOSITE_NORMAL_BALANCE_MOVEMENT":     JournalFindingWarning,
		"MANUAL_REVENUE_ENTRY":                 JournalFindingWarning,
		"MANUAL_EQUITY_ENTRY":                  JournalFindingWarning,
		"SENSITIVE_ACCOUNT_ENTRY":              JournalFindingWarning,
		"EXACT_DUPLICATE_ENTRY":                JournalFindingWarning,
		"POSSIBLE_DUPLICATE_ENTRY":             JournalFindingInfo,
		"REPEATED_IDENTICAL_AMOUNT":            JournalFindingInfo,
		"RAPID_REVERSAL":                       JournalFindingWarning,
		"CROSS_PERIOD_REVERSAL":                JournalFindingWarning,
		"PERIOD_END_ENTRY_WITH_EARLY_REVERSAL": JournalFindingWarning,
		"THRESHOLD_CLUSTER":                    JournalFindingInfo,
		"SPLIT_ENTRY_CLUSTER":                  JournalFindingWarning,
		"BLANK_DESCRIPTION":                    JournalFindingInfo,
		"GENERIC_DESCRIPTION":                  JournalFindingInfo,
		"MISSING_REFERENCE":                    JournalFindingInfo,
		"SAME_PREPARER_APPROVER":               JournalFindingWarning,
		"MISSING_APPROVER":                     JournalFindingInfo,
		"HIGH_VOLUME_BY_PREPARER":              JournalFindingInfo,
		"RARE_ACCOUNT_COMBINATION":             JournalFindingInfo,
	}
}

// DefaultPolicy returns a safe, broadly-applicable Policy: materiality
// gating off (everything material — see isMaterial), no critical
// accounts or required-reconciliation accounts declared (caller must opt
// in), the recommended journal-finding dispositions, every module
// non-required (so a missing module never blocks by default), missing-
// required-input treated as blocking once a caller does mark something
// required, and post-close activity in a locked period treated as
// blocking.
func DefaultPolicy() Policy {
	return Policy{
		JournalFindingRules:                DefaultJournalFindingRules(),
		TreatMissingRequiredInputAsBlocker: true,
		TreatLockedPostCloseAsBlocking:     true,
		CloseTaskRules:                     CloseTaskRules{RequiredIncompleteBlocks: false},
	}
}

func (p Policy) journalDisposition(code string) JournalFindingDisposition {
	if d, ok := p.JournalFindingRules[code]; ok {
		return d
	}
	if d, ok := DefaultJournalFindingRules()[code]; ok {
		return d
	}
	return JournalFindingInfo
}

// validatePolicy checks Policy for structurally invalid configuration:
// non-finite thresholds and invalid expected-activity rules. It does not
// duplicate expectationsByAccount's/closeTaskIssues's own per-entry
// validation — those run separately against their own input slices and
// contribute their own Issues.
func validatePolicy(p Policy) []Issue {
	var issues []Issue
	if isNonFinite(p.Materiality.AbsoluteAmount) || isNonFinite(p.Materiality.PercentOfAssets) || isNonFinite(p.Materiality.PercentOfRevenue) {
		issues = append(issues, Issue{
			Code:     IssueNonFiniteValue,
			Severity: IssueSeverityError,
			Message:  "policy materiality has a non-finite threshold",
		})
	}
	if isNonFinite(p.BalanceTolerance) {
		issues = append(issues, Issue{
			Code:     IssueNonFiniteValue,
			Severity: IssueSeverityError,
			Message:  "policy balance tolerance is non-finite",
		})
	}
	for _, r := range p.ExpectedActivityRules {
		if len(r.AccountIDs) == 0 {
			issues = append(issues, Issue{
				Code:     IssueInvalidExpectedActivityRule,
				Severity: IssueSeverityWarning,
				Message:  "expected activity rule has no account_ids and can never be evaluated",
				Ref:      r.Label,
			})
			continue
		}
		if isNonFinite(r.MinAbsoluteMove) {
			issues = append(issues, Issue{
				Code:     IssueNonFiniteValue,
				Severity: IssueSeverityError,
				Message:  "expected activity rule has a non-finite min_absolute_move",
				Ref:      r.Label,
			})
		}
	}
	return issues
}

func (p Policy) isCriticalAccount(accountID string) bool {
	for _, id := range p.CriticalAccounts {
		if id == accountID {
			return true
		}
	}
	return false
}
