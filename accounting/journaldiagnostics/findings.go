package journaldiagnostics

import "sort"

// FindingCode is a stable identifier for one kind of unusual-pattern/
// control-review indicator this package detects. Message text is a fixed
// template per code (see messages.go) — a machine consumer relies on Code
// and Evidence, never on parsing Message.
type FindingCode string

const (
	FindingMaterialManualEntry             FindingCode = "MATERIAL_MANUAL_ENTRY"
	FindingMaterialPeriodEndEntry          FindingCode = "MATERIAL_PERIOD_END_ENTRY"
	FindingPostCloseEntry                  FindingCode = "POST_CLOSE_ENTRY"
	FindingWeekendEntry                    FindingCode = "WEEKEND_ENTRY"
	FindingOutsideBusinessHours            FindingCode = "OUTSIDE_BUSINESS_HOURS"
	FindingRoundDollarEntry                FindingCode = "ROUND_DOLLAR_ENTRY"
	FindingLargeEntry                      FindingCode = "LARGE_ENTRY"
	FindingAccountRelativeLargeEntry       FindingCode = "ACCOUNT_RELATIVE_LARGE_ENTRY"
	FindingRareAccountActivity             FindingCode = "RARE_ACCOUNT_ACTIVITY"
	FindingNewAccountActivity              FindingCode = "NEW_ACCOUNT_ACTIVITY"
	FindingOppositeNormalBalanceMovement   FindingCode = "OPPOSITE_NORMAL_BALANCE_MOVEMENT"
	FindingManualRevenueEntry              FindingCode = "MANUAL_REVENUE_ENTRY"
	FindingManualEquityEntry               FindingCode = "MANUAL_EQUITY_ENTRY"
	FindingSensitiveAccountEntry           FindingCode = "SENSITIVE_ACCOUNT_ENTRY"
	FindingExactDuplicateEntry             FindingCode = "EXACT_DUPLICATE_ENTRY"
	FindingPossibleDuplicateEntry          FindingCode = "POSSIBLE_DUPLICATE_ENTRY"
	FindingRepeatedIdenticalAmount         FindingCode = "REPEATED_IDENTICAL_AMOUNT"
	FindingRapidReversal                   FindingCode = "RAPID_REVERSAL"
	FindingCrossPeriodReversal             FindingCode = "CROSS_PERIOD_REVERSAL"
	FindingPeriodEndEntryWithEarlyReversal FindingCode = "PERIOD_END_ENTRY_WITH_EARLY_REVERSAL"
	FindingThresholdCluster                FindingCode = "THRESHOLD_CLUSTER"
	FindingSplitEntryCluster               FindingCode = "SPLIT_ENTRY_CLUSTER"
	FindingBlankDescription                FindingCode = "BLANK_DESCRIPTION"
	FindingGenericDescription              FindingCode = "GENERIC_DESCRIPTION"
	FindingMissingReference                FindingCode = "MISSING_REFERENCE"
	FindingSamePreparerApprover            FindingCode = "SAME_PREPARER_APPROVER"
	FindingMissingApprover                 FindingCode = "MISSING_APPROVER"
	FindingHighVolumeByPreparer            FindingCode = "HIGH_VOLUME_BY_PREPARER"
	FindingRareAccountCombination          FindingCode = "RARE_ACCOUNT_COMBINATION"
)

// findingCodeOrder fixes FindingCode declaration order for deterministic
// Findings sorting (see sortFindings) — mirrors accounting/ap's identical
// flagCodeOrder/flagRank convention.
var findingCodeOrder = []FindingCode{
	FindingMaterialManualEntry,
	FindingMaterialPeriodEndEntry,
	FindingPostCloseEntry,
	FindingWeekendEntry,
	FindingOutsideBusinessHours,
	FindingRoundDollarEntry,
	FindingLargeEntry,
	FindingAccountRelativeLargeEntry,
	FindingRareAccountActivity,
	FindingNewAccountActivity,
	FindingOppositeNormalBalanceMovement,
	FindingManualRevenueEntry,
	FindingManualEquityEntry,
	FindingSensitiveAccountEntry,
	FindingExactDuplicateEntry,
	FindingPossibleDuplicateEntry,
	FindingRepeatedIdenticalAmount,
	FindingRapidReversal,
	FindingCrossPeriodReversal,
	FindingPeriodEndEntryWithEarlyReversal,
	FindingThresholdCluster,
	FindingSplitEntryCluster,
	FindingBlankDescription,
	FindingGenericDescription,
	FindingMissingReference,
	FindingSamePreparerApprover,
	FindingMissingApprover,
	FindingHighVolumeByPreparer,
	FindingRareAccountCombination,
}

func findingRank(c FindingCode) int {
	for i, fc := range findingCodeOrder {
		if fc == c {
			return i
		}
	}
	return len(findingCodeOrder)
}

// Severity is a Finding's review priority — explicitly NOT a probability of
// wrongdoing. A HIGH severity means "an accountant/controller should look
// at this soon," never "this is likely fraudulent." See the package doc
// comment's non-fraud boundary.
type Severity string

const (
	SeverityInfo    Severity = "INFO"
	SeverityWarning Severity = "WARNING"
	SeverityHigh    Severity = "HIGH"
)

var severityOrder = map[Severity]int{SeverityHigh: 0, SeverityWarning: 1, SeverityInfo: 2}

// Evidence carries the typed reasoning behind one Finding. Only the
// sub-fields relevant to the Finding's Code are populated; a machine
// consumer must not assume every field is set — see the per-field doc
// comments for exactly which FindingCode(s) populate each one. This mirrors
// accounting/ap.Flag's flat-struct convention rather than a Go interface
// per finding type, since every field here is JSON-serializable data with
// no behavior.
type Evidence struct {
	// EntryAmount is this entry's EntryMagnitude (total debits) — populated
	// on nearly every amount-related finding.
	EntryAmount Value `json:"entry_amount,omitempty"`

	// RoundBase is the divisor EntryAmount was evenly divisible by —
	// FindingRoundDollarEntry only.
	RoundBase Value `json:"round_base,omitempty"`

	// HistoricalMedian, MAD, and ObservationCount describe the account's
	// historical baseline — FindingAccountRelativeLargeEntry only.
	HistoricalMedian Value `json:"historical_median,omitempty"`
	MAD              Value `json:"mad,omitempty"`
	ObservationCount Value `json:"observation_count,omitempty"`
	Threshold        Value `json:"threshold,omitempty"`

	// HistoricalUseCount is an account's prior entry count —
	// FindingRareAccountActivity and FindingNewAccountActivity only (0 for
	// the latter).
	HistoricalUseCount Value `json:"historical_use_count,omitempty"`

	// EntryDate, PeriodEnd, and DaysFromPeriodEnd describe an entry's
	// position relative to the analysis window —
	// FindingMaterialPeriodEndEntry, FindingPostCloseEntry,
	// FindingWeekendEntry, FindingOutsideBusinessHours,
	// FindingPeriodEndEntryWithEarlyReversal.
	EntryDate         string `json:"entry_date,omitempty"`
	PeriodEnd         string `json:"period_end,omitempty"`
	DaysFromPeriodEnd Value  `json:"days_from_period_end,omitempty"`
	CloseDate         string `json:"close_date,omitempty"`

	// NormalizedSignature is the deterministic economic-content signature
	// two or more entries shared — FindingExactDuplicateEntry and
	// FindingPossibleDuplicateEntry only.
	NormalizedSignature string `json:"normalized_signature,omitempty"`
	// EntryIDs lists every entry participating in a group finding
	// (duplicate group, repeated-amount group, threshold cluster,
	// split-entry cluster).
	EntryIDs []string `json:"entry_ids,omitempty"`

	// OriginalEntryID, ReversingEntryID, and DaysBetween describe a
	// reversal relationship — FindingRapidReversal,
	// FindingCrossPeriodReversal,
	// FindingPeriodEndEntryWithEarlyReversal.
	OriginalEntryID  string `json:"original_entry_id,omitempty"`
	ReversingEntryID string `json:"reversing_entry_id,omitempty"`
	DaysBetween      Value  `json:"days_between,omitempty"`
	OriginalPeriod   string `json:"original_period,omitempty"`
	ReversingPeriod  string `json:"reversing_period,omitempty"`

	// IndividualAmounts and CombinedAmount describe a threshold/split
	// cluster — FindingThresholdCluster and FindingSplitEntryCluster only.
	IndividualAmounts []float64 `json:"individual_amounts,omitempty"`
	CombinedAmount    Value     `json:"combined_amount,omitempty"`

	// Description is the entry's (or generic-match's) description text —
	// FindingBlankDescription and FindingGenericDescription only.
	Description string `json:"description,omitempty"`

	// PreparerID and ApproverID are the opaque caller-supplied identifiers
	// involved — FindingSamePreparerApprover, FindingMissingApprover,
	// FindingHighVolumeByPreparer.
	PreparerID string `json:"preparer_id,omitempty"`
	ApproverID string `json:"approver_id,omitempty"`
	// PreparerEntryCount is the preparer's entry count within the analysis
	// period — FindingHighVolumeByPreparer only.
	PreparerEntryCount Value `json:"preparer_entry_count,omitempty"`

	// DebitAccountIDs and CreditAccountIDs (sorted) are an entry's account
	// combination — FindingRareAccountCombination only.
	DebitAccountIDs  []string `json:"debit_account_ids,omitempty"`
	CreditAccountIDs []string `json:"credit_account_ids,omitempty"`

	// SensitiveAccountLabel is the AccountReviewPolicy.Label an account
	// matched — FindingSensitiveAccountEntry only.
	SensitiveAccountLabel string `json:"sensitive_account_label,omitempty"`

	// Source is the EntryMetadata.Source that made a manual-entry finding
	// possible — every Finding* related to manual entries.
	Source EntrySource `json:"source,omitempty"`

	// ExternalLabel is an optional caller-supplied external classification
	// attached to this specific entry/finding as external data (e.g. from a
	// caller's own fraud/risk system). This package never sets it and never
	// interprets it — see the package doc comment's non-fraud boundary. A
	// caller that wants to carry such a label through its own downstream
	// presentation can do so via this field without this package itself
	// concluding anything from it. Always empty unless a caller-side
	// post-processing step chooses to populate it after this package
	// returns its Result — this package's own Calculate never writes it.
	ExternalLabel string `json:"external_label,omitempty"`
}

// Finding is one deterministic, explainable unusual-pattern or
// control-review indicator — never a fraud conclusion (see the package doc
// comment). Message is a fixed template for Code; machine consumers rely on
// Code and Evidence, never on parsing Message text.
type Finding struct {
	Code     FindingCode `json:"code"`
	Severity Severity    `json:"severity"`
	// Period is the analysis PeriodWindow.Period this Finding was detected
	// in.
	Period string `json:"period"`
	// EntryIDs are the JournalEntry.ID values this Finding concerns. A
	// single-entry finding has exactly one; a group finding (duplicate,
	// cluster, repeated-amount) has two or more, sorted ascending.
	EntryIDs []string `json:"entry_ids"`
	// AccountIDs are the Account.ID values this Finding concerns, sorted
	// ascending. May be empty for a finding that is not account-specific
	// (e.g. a threshold cluster spanning several accounts already captured
	// in Evidence).
	AccountIDs []string `json:"account_ids,omitempty"`
	// Amount is this Finding's characteristic dollar amount (the single
	// entry's EntryMagnitude, or a group's CombinedAmount) — see each
	// FindingCode's Evidence documentation for exactly which figure this
	// mirrors.
	Amount   Value    `json:"amount"`
	Evidence Evidence `json:"evidence"`
	Message  string   `json:"message"`
}

// firstEntryID returns f's first EntryIDs entry, or "" if empty — used as a
// stable tiebreaker key for sorting.
func (f Finding) firstEntryID() string {
	if len(f.EntryIDs) == 0 {
		return ""
	}
	return f.EntryIDs[0]
}

// sortFindings orders findings deterministically: Severity (HIGH, WARNING,
// INFO), then FindingCode declaration order, then effective date (Evidence.
// EntryDate, "" sorts first), then first EntryID — see the package doc
// comment's determinism section. Does not mutate the input slice's
// ownership (returns a new sorted slice; the caller-visible Finding values
// themselves are never shared mutable state).
func sortFindings(findings []Finding) []Finding {
	out := make([]Finding, len(findings))
	copy(out, findings)
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := severityOrder[out[i].Severity], severityOrder[out[j].Severity]
		if si != sj {
			return si < sj
		}
		ri, rj := findingRank(out[i].Code), findingRank(out[j].Code)
		if ri != rj {
			return ri < rj
		}
		if out[i].Evidence.EntryDate != out[j].Evidence.EntryDate {
			return out[i].Evidence.EntryDate < out[j].Evidence.EntryDate
		}
		return out[i].firstEntryID() < out[j].firstEntryID()
	})
	return out
}

// sortStrings returns a sorted copy of ss.
func sortStrings(ss []string) []string {
	out := make([]string, len(ss))
	copy(out, ss)
	sort.Strings(out)
	return out
}
