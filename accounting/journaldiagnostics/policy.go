package journaldiagnostics

import (
	"sort"

	"github.com/themurtez/go-valuate/accounting/ledger"
)

// Weekday mirrors time.Weekday's values (Sunday = 0 ... Saturday = 6) as a
// package-local type so Policy's JSON contract does not depend on stdlib's
// encoding of time.Weekday (which has none — it is not a
// json.Marshaler/Unmarshaler).
type Weekday int

const (
	Sunday    Weekday = 0
	Monday    Weekday = 1
	Tuesday   Weekday = 2
	Wednesday Weekday = 3
	Thursday  Weekday = 4
	Friday    Weekday = 5
	Saturday  Weekday = 6
)

// defaultWorkingDays is Monday-Friday, used when Policy.WorkingDays is
// empty.
func defaultWorkingDays() []Weekday {
	return []Weekday{Monday, Tuesday, Wednesday, Thursday, Friday}
}

// BusinessHours configures the outside-business-hours diagnostic (see
// timing.go). All three fields are required together for the diagnostic to
// be available — see RuleAvailability.OutsideHours. This package never
// infers a timezone; TimeZone must name an IANA zone loadable by
// time.LoadLocation (e.g. "America/New_York"), or the diagnostic is
// unavailable.
type BusinessHours struct {
	// StartHour and EndHour are the business day's boundaries in 24-hour
	// local time (e.g. StartHour: 9, EndHour: 17 for 9am-5pm). An entry
	// timestamped at or after StartHour and strictly before EndHour, in
	// TimeZone, is within business hours.
	StartHour int `json:"start_hour"`
	EndHour   int `json:"end_hour"`
	// TimeZone is the IANA zone name business hours are evaluated in.
	// Required (no default) — see the package doc comment's "No hidden
	// current-date dependency" section; a missing/invalid TimeZone makes the
	// diagnostic unavailable rather than assuming UTC or the host's local
	// zone.
	TimeZone string `json:"time_zone"`
}

func (h BusinessHours) configured() bool {
	return h.TimeZone != "" && h.EndHour > h.StartHour
}

// Policy is this package's single explicit, caller-configured set of
// thresholds and rule toggles. Every diagnostic rule that needs a numeric
// or structural threshold reads it from here; this package never invents a
// business-specific threshold (e.g. an approval limit) on its own — see
// DefaultPolicy's doc comment for exactly which fields have a documented
// default and which are intentionally left at zero (disabled) until a
// caller supplies them.
type Policy struct {
	// MaterialAmount is the minimum absolute entry magnitude (see
	// EntryMagnitude's doc comment) most amount-based rules require before
	// considering an entry at all — e.g. round-dollar, rare/new-account
	// activity, opposite-normal-balance movement, manual+period-end
	// combinations. Rules that need a materially different threshold define
	// their own field below instead of overloading this one. Zero disables
	// the material-amount gate for rules that read it (they still fire on
	// non-amount conditions alone where documented).
	MaterialAmount float64 `json:"material_amount"`

	// PeriodEndDays is how many calendar days before PeriodWindow.EndDate
	// (inclusive) counts as "period-end" — see periodend.go. Default 3 (see
	// DefaultPolicy).
	PeriodEndDays int `json:"period_end_days"`

	// RoundDollarBases are the divisors an entry's magnitude is checked
	// against for round-dollar detection (e.g. 100, 1000, 10000) — see
	// amounts.go. An entry matches the largest base it is evenly divisible
	// by. Default {100, 1000, 10000} (see DefaultPolicy).
	RoundDollarBases []float64 `json:"round_dollar_bases,omitempty"`
	// RoundDollarMinAmount is the minimum magnitude before round-dollar
	// detection applies — see the task's "do not flag tiny amounts merely
	// because they are round" instruction. Zero disables round-dollar
	// detection entirely (not "flag everything").
	RoundDollarMinAmount float64 `json:"round_dollar_min_amount"`

	// LargeEntryAbsoluteThreshold triggers FindingLargeEntry when an entry's
	// EntryMagnitude exceeds this fixed dollar amount. Zero disables the
	// absolute large-entry rule.
	LargeEntryAbsoluteThreshold float64 `json:"large_entry_absolute_threshold"`
	// MADMultiplier is K in the median + K*MAD account-relative large-entry
	// rule (see amounts.go) — how many scaled-MAD units above an account's
	// historical median magnitude counts as unusual. Default 3.5 (see
	// DefaultPolicy).
	MADMultiplier float64 `json:"mad_multiplier"`
	// MinBaselineObservations is the minimum number of an account's prior
	// historical entries required before the account-relative large-entry
	// rule (or the rare/new-account rules) is considered available for that
	// account — see IssueInsufficientBaseline and RuleAvailability. Default
	// 5 (see DefaultPolicy).
	MinBaselineObservations int `json:"min_baseline_observations"`

	// RareAccountMaxHistoricalEntries is the maximum number of an account's
	// prior historical entries under which a new material posting to it
	// counts as FindingRareAccountActivity — see accounts.go. Default 2 (see
	// DefaultPolicy).
	RareAccountMaxHistoricalEntries int `json:"rare_account_max_historical_entries"`

	// DuplicateWindowDays bounds FindingPossibleDuplicateEntry: two entries
	// with a matching normalized signature count as a possible duplicate
	// only when their effective dates are within this many days of each
	// other — see duplicates.go. Default 3 (see DefaultPolicy).
	DuplicateWindowDays int `json:"duplicate_window_days"`
	// MinRepeatedAmountCount is the minimum number of distinct entries
	// sharing the same EntryMagnitude (>= RepeatedAmountMinAmount) before
	// FindingRepeatedIdenticalAmount fires — see duplicates.go. Default 3
	// (see DefaultPolicy).
	MinRepeatedAmountCount int `json:"min_repeated_amount_count"`
	// RepeatedAmountMinAmount is the minimum magnitude an amount must have
	// before repeated-amount detection considers it. Zero disables the
	// repeated-amount rule.
	RepeatedAmountMinAmount float64 `json:"repeated_amount_min_amount"`

	// RapidReversalDays is the maximum number of days between an entry's
	// effective date and its explicit reversal's effective date before
	// FindingRapidReversal fires — see reversals.go. Default 3 (see
	// DefaultPolicy).
	RapidReversalDays int `json:"rapid_reversal_days"`

	// ApprovalThreshold is the caller's approval/review dollar threshold,
	// used by threshold-clustering diagnostics (see clustering.go). Zero
	// disables both threshold-clustering rules — this package never invents
	// a business-specific approval limit.
	ApprovalThreshold float64 `json:"approval_threshold"`
	// ThresholdClusterLowerPercent is the lower bound (as a fraction of
	// ApprovalThreshold, e.g. 0.9 for 90%) of the band checked for
	// FindingThresholdCluster. The upper bound is always 1.0 (at or just
	// under the threshold itself). Default 0.9 (see DefaultPolicy).
	ThresholdClusterLowerPercent float64 `json:"threshold_cluster_lower_percent"`
	// ThresholdClusterMinCount is the minimum number of entries within the
	// band (for FindingThresholdCluster) or the minimum number of same-date
	// same-account-pattern entries combining to meet ApprovalThreshold (for
	// split-entry clustering) before either finding fires. Default 2 (see
	// DefaultPolicy).
	ThresholdClusterMinCount int `json:"threshold_cluster_min_count"`

	// GenericDescriptions is a caller-supplied set of exact, normalized
	// (case-insensitive, trimmed) description strings treated as
	// uninformative — see controls.go. Empty means no generic-description
	// matching is performed (a blank description is still detected
	// independently — see FindingBlankDescription).
	GenericDescriptions []string `json:"generic_descriptions,omitempty"`
	// RequiredReferenceFields, if non-empty, is the set of reference-like
	// fields ("external_reference", "reference", "external_ref",
	// "batch_id") a material entry is expected to have at least one of —
	// see controls.go. Empty means the missing-reference diagnostic is not
	// opted into (see RuleAvailability.MissingReference).
	RequiredReferenceFields []string `json:"required_reference_fields,omitempty"`

	// HighVolumePreparerCount is the minimum number of entries by one
	// PreparerID within the analysis period before
	// FindingHighVolumeByPreparer fires — see controls.go. Zero disables
	// this rule.
	HighVolumePreparerCount int `json:"high_volume_preparer_count"`

	// WorkingDays is the set of weekdays considered "business days" for
	// weekend detection (see timing.go). Empty defaults to Monday-Friday
	// (see DefaultPolicy / defaultWorkingDays).
	WorkingDays []Weekday `json:"working_days,omitempty"`
	// BusinessHours configures the outside-business-hours diagnostic. The
	// zero value leaves that diagnostic unavailable — see BusinessHours'
	// doc comment.
	BusinessHours BusinessHours `json:"business_hours"`

	// SensitiveAccounts is zero or more caller-supplied account sets for the
	// sensitive-account-entry diagnostic — see AccountReviewPolicy and
	// FindingSensitiveAccountEntry. Never inferred from account names.
	SensitiveAccounts []AccountReviewPolicy `json:"sensitive_accounts,omitempty"`

	// IncludeStatuses restricts which ledger.EntryStatus values are analyzed
	// — see the package doc's "Posted-entry scope" section. The zero value
	// (nil) defaults to ledger.PostedStatuses() (POSTED and REVERSED).
	// VOIDED is never included, even if explicitly listed — mirrors
	// ledger.BalanceOptions.IncludeStatuses' identical rule.
	IncludeStatuses []ledger.EntryStatus `json:"include_statuses,omitempty"`

	// Tolerance is the entry-balance tolerance passed to
	// ledger.ValidateEntries — see ledger.ValidateOptions.Tolerance's doc
	// comment. Zero means exact equality is required.
	Tolerance float64 `json:"tolerance"`
}

// DefaultPolicy returns this package's baseline thresholds. Every field
// with a fixed, business-neutral default (e.g. PeriodEndDays,
// MADMultiplier, MinBaselineObservations, RapidReversalDays,
// ThresholdClusterLowerPercent/MinCount, RoundDollarBases,
// DuplicateWindowDays, MinRepeatedAmountCount,
// RareAccountMaxHistoricalEntries, WorkingDays) is set here. Fields that are
// inherently business-specific and have no sensible universal default —
// MaterialAmount, LargeEntryAbsoluteThreshold, RoundDollarMinAmount,
// RepeatedAmountMinAmount, ApprovalThreshold, GenericDescriptions,
// RequiredReferenceFields, HighVolumePreparerCount, BusinessHours,
// SensitiveAccounts — are left at their zero value (disabling the rules
// that depend on them) until a caller supplies one. This mirrors
// accounting/ap.DefaultThresholds' identical "fixed default vs.
// materiality-scaled/caller-only" split.
func DefaultPolicy() Policy {
	return Policy{
		PeriodEndDays:                   3,
		RoundDollarBases:                []float64{100, 1000, 10000},
		MADMultiplier:                   3.5,
		MinBaselineObservations:         5,
		RareAccountMaxHistoricalEntries: 2,
		DuplicateWindowDays:             3,
		MinRepeatedAmountCount:          3,
		RapidReversalDays:               3,
		ThresholdClusterLowerPercent:    0.9,
		ThresholdClusterMinCount:        2,
		WorkingDays:                     defaultWorkingDays(),
	}
}

// resolve merges p over DefaultPolicy field by field: a zero-valued field
// (for fields with a fixed default) takes the default; fields with no fixed
// default are passed through unchanged (zero means "disabled," not
// "unset").
func (p Policy) resolve() Policy {
	d := DefaultPolicy()
	if p.PeriodEndDays == 0 {
		p.PeriodEndDays = d.PeriodEndDays
	}
	if len(p.RoundDollarBases) == 0 {
		p.RoundDollarBases = d.RoundDollarBases
	}
	if p.MADMultiplier == 0 {
		p.MADMultiplier = d.MADMultiplier
	}
	if p.MinBaselineObservations == 0 {
		p.MinBaselineObservations = d.MinBaselineObservations
	}
	if p.RareAccountMaxHistoricalEntries == 0 {
		p.RareAccountMaxHistoricalEntries = d.RareAccountMaxHistoricalEntries
	}
	if p.DuplicateWindowDays == 0 {
		p.DuplicateWindowDays = d.DuplicateWindowDays
	}
	if p.MinRepeatedAmountCount == 0 {
		p.MinRepeatedAmountCount = d.MinRepeatedAmountCount
	}
	if p.RapidReversalDays == 0 {
		p.RapidReversalDays = d.RapidReversalDays
	}
	if p.ThresholdClusterLowerPercent == 0 {
		p.ThresholdClusterLowerPercent = d.ThresholdClusterLowerPercent
	}
	if p.ThresholdClusterMinCount == 0 {
		p.ThresholdClusterMinCount = d.ThresholdClusterMinCount
	}
	if len(p.WorkingDays) == 0 {
		p.WorkingDays = d.WorkingDays
	}
	return p
}

// isWorkingDay reports whether d is a member of workingDays.
func isWorkingDay(d Weekday, workingDays []Weekday) bool {
	for _, w := range workingDays {
		if w == d {
			return true
		}
	}
	return false
}

// includeStatusSet returns p's IncludeStatuses as a lookup set, defaulting
// to ledger.PostedStatuses() (POSTED, REVERSED) when empty, and always
// excluding VOIDED — mirrors ledger.BalanceOptions.includeStatuses'
// identical rule.
func (p Policy) includeStatusSet() map[ledger.EntryStatus]bool {
	statuses := p.IncludeStatuses
	if len(statuses) == 0 {
		statuses = ledger.PostedStatuses()
	}
	out := make(map[ledger.EntryStatus]bool, len(statuses))
	for _, s := range statuses {
		if s != ledger.StatusVoided {
			out[s] = true
		}
	}
	return out
}

// sortedFloat64s returns a sorted copy of vs, ascending.
func sortedFloat64s(vs []float64) []float64 {
	out := make([]float64, len(vs))
	copy(out, vs)
	sort.Float64s(out)
	return out
}
