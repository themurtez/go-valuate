// Package anomalies detects deterministic, explainable anomalies and
// unusual changes across a normalized financial.FinancialDataset's accounts
// and periods: spikes, expense growth outpacing revenue, margin
// deterioration, new or disappearing expense categories, repeated
// suspiciously-identical values, sign flips, duplicate-like amounts across
// unrelated accounts, an outsized owner/discretionary expense share, and
// unexpected negative amounts on accounts that should not go negative.
//
// This package contains no AI/ML and no fraud determination of any kind.
// Every Anomaly is the output of one fixed, documented, caller-thresholded
// rule (see Thresholds) — never a model score, never an accusation. Findings
// use neutral language ("anomaly," "variance," "unusual pattern," "review
// recommended") throughout, per this package's task-level instruction, and
// Anomaly.Explanation is generated text describing exactly which numbers
// crossed which threshold, never free-form prose from an external source.
//
// Every function here is pure: no I/O, no mutation of caller-owned input, no
// package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package anomalies

import "github.com/themurtez/go-valuate/financial"

// FormulaVersion identifies this package's fixed detection-rule set: every
// rule's exact comparison method (see each RuleCode's doc comment) and the
// DefaultThresholds this package ships. Bump this whenever any of that
// changes in a way that could make a historical Result not reproduce
// identically under new code — see the repository README's
// versioning-strategy section, which this constant follows exactly
// (financial.TaxonomyVersion, metrics.FormulaVersion, qoe.FormulaVersion,
// workingcapital.FormulaVersion, cashflow.FormulaVersion,
// revenuequality.FormulaVersion, concentration.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// PeriodType mirrors metrics.PeriodType/workingcapital.PeriodType/
// revenuequality.PeriodType/concentration.PeriodType (fiscal year, YTD,
// quarter, month), duplicated here rather than aliased so this package's own
// doc comments apply directly at the call site — the same choice every
// analytics sibling package already made for its own PeriodType.
type PeriodType string

const (
	PeriodTypeFiscalYear PeriodType = "fiscal_year"
	PeriodTypeYTD        PeriodType = "ytd"
	PeriodTypeQuarter    PeriodType = "quarter"
	PeriodTypeMonth      PeriodType = "month"
)

// PeriodInfo is caller-supplied metadata about a single financial.Period,
// used to determine chronological order for every period-over-period rule
// (spike, percentage change, sign flip, margin deterioration,
// disappearing/reappearing accounts) and for "most recent period" rules
// (owner/discretionary share). financial.Period is intentionally just a
// string with no guaranteed sort order; this package requires PeriodInfo
// rather than inferring order or granularity from the string, mirroring
// every analytics sibling package's identical no-guessing rule.
type PeriodInfo struct {
	// Type is this period's granularity.
	Type PeriodType `json:"type"`
	// FiscalYear is the fiscal year this period falls within. Required for
	// chronological ordering.
	FiscalYear int `json:"fiscal_year"`
	// SequenceInYear orders periods that share the same FiscalYear and Type
	// (e.g. Q1=1..Q4=4, or month 1-12). Ignored for PeriodTypeFiscalYear and
	// PeriodTypeYTD.
	SequenceInYear int `json:"sequence_in_year,omitempty"`
}

// AccountGroup is a caller-defined, named collection of financial.Code
// values, used only to label Anomaly.Group for display/filtering (e.g. "G&A"
// grouping several OPEX_* codes together, or "Marketing" isolating
// CodeOpexMarketing alone). Entirely optional: this package runs every
// account-level rule at the individual financial.Code granularity regardless
// of whether groups are supplied — groups never change which anomalies are
// detected, only how they can be labeled/filtered afterward. A financial.Code
// not covered by any AccountGroup simply has an empty Anomaly.Group.
type AccountGroup struct {
	// Name is the caller-chosen display label for this group (e.g. "G&A",
	// "Marketing & Advertising"). Must be non-empty for the group to be
	// used; a group with an empty Name is skipped.
	Name string `json:"name"`
	// Codes lists every financial.Code that belongs to this group. A code
	// listed in more than one AccountGroup is labeled with whichever group
	// appears first in Input.AccountGroups (deterministic, caller-controlled
	// precedence — never Go map order).
	Codes []financial.Code `json:"codes"`
}

// Input bundles everything Calculate needs. This package does not require a
// customer/user/application object of any kind — only financial-domain data.
type Input struct {
	// Dataset is the normalized financial history to analyze. Required;
	// Calculate returns a Result with Available == false and an Errors
	// entry if Dataset has no periods.
	Dataset financial.FinancialDataset
	// PeriodMeta supplies chronological ordering for Dataset's periods. See
	// PeriodInfo. Required for every period-over-period and
	// most-recent-period rule; if nil, or a period present in Dataset has no
	// entry, those specific rules are skipped (reported via
	// IssueNoPeriodMeta/IssuePeriodMissingFromMeta) rather than the whole
	// Result failing — mirroring analytics/qoe.Input.PeriodMeta's identical
	// convention. Rules that need no chronological order (repeated value,
	// duplicate-like amounts, unexpected negative amounts) still run without
	// it.
	PeriodMeta map[financial.Period]PeriodInfo
	// AccountGroups optionally labels Anomaly.Group for display/filtering —
	// see AccountGroup. Entirely optional and never changes detection
	// behavior.
	AccountGroups []AccountGroup
	// DiscretionaryCodes lists additional financial.Code values the caller
	// considers owner/discretionary, beyond financial.CodeOpexOwnerComp
	// (which this package always includes — see
	// RuleHighOwnerDiscretionaryShare's doc comment). For example, a caller
	// whose classification pipeline routes personal vehicle or travel
	// expenses onto CodeOpexVehicle/CodeOpexTravel supplies those here to
	// include them in the discretionary-share calculation. Duplicates of
	// CodeOpexOwnerComp are harmless.
	DiscretionaryCodes []financial.Code `json:"discretionary_codes,omitempty"`
}

// Options controls Calculate's optional, caller-adjustable behavior. The
// zero Options is valid: DefaultThresholds is used.
type Options struct {
	// Thresholds configures every deterministic anomaly trigger point (see
	// Thresholds). If the zero value, DefaultThresholds() is used —
	// mirroring every analytics sibling package's identical
	// zero-value-means-defaults convention.
	Thresholds Thresholds
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model every
// analytics sibling package uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository, rather than reusing one of
// theirs — an anomaly-detection input problem is a distinct problem domain.
type IssueCode string

const (
	// IssueNoPeriods means Input.Dataset has no periods at all; Calculate
	// returns Available == false.
	IssueNoPeriods IssueCode = "NO_PERIODS"
	// IssueNoPeriodMeta means Input.PeriodMeta was nil or empty, so every
	// period-over-period and most-recent-period rule was skipped. Advisory
	// only: rules that need no chronological order still ran.
	IssueNoPeriodMeta IssueCode = "NO_PERIOD_META"
	// IssuePeriodMissingFromMeta means at least one period present in
	// Dataset has no PeriodMeta entry, so chronological ordering fell back
	// to Dataset.Periods()'s lexical order and every period-over-period rule
	// was skipped — mirroring workingcapital/qoe's identical
	// all-or-nothing rule (a partially-ordered series is not trustworthy for
	// "which period is the baseline").
	IssuePeriodMissingFromMeta IssueCode = "PERIOD_MISSING_FROM_META"
)

// Issue is a single Calculate-time input finding, mirroring every analytics
// sibling package's identical Issue shape.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from adjustments.HasErrors/qoe.HasErrors/
// workingcapital.HasErrors/cashflow.HasErrors/revenuequality.HasErrors/
// concentration.HasErrors rather than shared — see adjustments.HasErrors's
// doc comment for the full rationale (each package's Issue is a distinct Go
// type with no common interface worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// RuleCode identifies which deterministic detection rule produced an
// Anomaly. Every rule is documented here with its exact comparison method;
// Thresholds' fields are named to match.
type RuleCode string

const (
	// RuleAbsoluteAmountSpike means one account's amount changed, period
	// over chronologically-prior period, by at least
	// Thresholds.AbsoluteAmountSpike (in the dataset's currency, unsigned).
	// Fires independently of RulePercentageChangeSpike — a large dollar move
	// on a large base account may not clear a percentage threshold but is
	// still material in absolute terms.
	RuleAbsoluteAmountSpike RuleCode = "ABSOLUTE_AMOUNT_SPIKE"
	// RulePercentageChangeSpike means one account's amount changed, period
	// over chronologically-prior period, by at least
	// Thresholds.PercentageChangeSpike as a fraction of the prior period's
	// absolute value (0.5 = 50%). Requires the baseline (prior) amount to be
	// nonzero — a move from exactly $0 is reported instead as
	// RuleNewMaterialExpenseCategory or RuleAccountDisappearedReappeared,
	// which are the rules built for that boundary case.
	RulePercentageChangeSpike RuleCode = "PERCENTAGE_CHANGE_SPIKE"
	// RuleExpenseOutpacingRevenue means, for a chronologically adjacent
	// period pair where both TotalRevenue and a given expense account
	// (COGS or OPEX category) are available and revenue is positive in both
	// periods, the expense's period-over-period growth rate exceeds
	// revenue's period-over-period growth rate by at least
	// Thresholds.ExpenseOutpacingRevenueGap (a raw percentage-point gap,
	// e.g. 0.20 = 20 points), and the expense actually grew (its growth rate
	// is positive) — a shrinking expense growing "less negatively" than
	// revenue shrank is not this rule's concern.
	RuleExpenseOutpacingRevenue RuleCode = "EXPENSE_OUTPACING_REVENUE"
	// RuleMarginDeterioration means, for a chronologically adjacent period
	// pair, computed gross margin (Total Revenue minus COGS, divided by
	// Total Revenue) or operating margin (Total Revenue minus COGS minus
	// OPEX, divided by Total Revenue) declined by at least
	// Thresholds.MarginDeteriorationPoints (raw percentage points, e.g. 0.10
	// = 10 points) from the prior period. Both margins are checked
	// independently; either crossing produces its own Anomaly.
	RuleMarginDeterioration RuleCode = "MARGIN_DETERIORATION"
	// RuleNewMaterialExpenseCategory means an expense account (COGS or OPEX
	// category) has no reported amount (no NormalizedItem at all) in the
	// chronologically prior period but a materially large amount — at least
	// Thresholds.NewCategoryMaterialAmount, or at least
	// Thresholds.NewCategoryMaterialPercentOfRevenue of that period's Total
	// Revenue when revenue is available — in the current period.
	RuleNewMaterialExpenseCategory RuleCode = "NEW_MATERIAL_EXPENSE_CATEGORY"
	// RuleAccountDisappearedReappeared means an account that had a nonzero
	// reported amount in the chronologically prior period has no reported
	// amount at all (no NormalizedItem) in the current period
	// ("disappeared"), or vice versa: no amount in the period before that,
	// and a nonzero amount now after at least one gap period
	// ("reappeared"). Fires for any Code, income-statement or balance-sheet,
	// not only expenses — an account vanishing from a balance sheet is as
	// structurally notable as one vanishing from the income statement.
	RuleAccountDisappearedReappeared RuleCode = "ACCOUNT_DISAPPEARED_REAPPEARED"
	// RuleRepeatedUnusualValue means the same account reports the exact same
	// nonzero amount (within FloatEqualityTolerance) in at least
	// Thresholds.RepeatedValueMinOccurrences distinct periods — a signal
	// that a value may have been copy-pasted or left stale rather than
	// re-entered each period. One Anomaly per (account, repeated amount)
	// combination, referencing every period the value repeated in via
	// Anomaly.RelatedPeriods.
	RuleRepeatedUnusualValue RuleCode = "REPEATED_UNUSUAL_VALUE"
	// RuleSignFlip means one account's amount has the opposite sign
	// (strictly positive vs. strictly negative; a value of exactly zero
	// never triggers this rule on either side) in the current period versus
	// the chronologically prior period, and both magnitudes are at least
	// Thresholds.SignFlipMinMagnitude — filtering out sign changes on
	// near-zero noise.
	RuleSignFlip RuleCode = "SIGN_FLIP"
	// RuleDuplicateLikeAmounts means two or more DISTINCT (account, period)
	// observations across the whole dataset report the exact same nonzero
	// amount (within FloatEqualityTolerance), for accounts this package
	// considers "suspicious" to duplicate — see suspiciousDuplicateCodes in
	// duplicates.go for the exact set (COGS and OPEX categories, where an
	// identical dollar figure recurring across different accounts and/or
	// periods is a more notable coincidence than on, say, a balance-sheet
	// control total). One Anomaly per distinct repeated-amount cluster,
	// referencing every (account, period) pair via Anomaly.RelatedAccounts/
	// RelatedPeriods.
	RuleDuplicateLikeAmounts RuleCode = "DUPLICATE_LIKE_AMOUNTS"
	// RuleHighOwnerDiscretionaryShare means, for the chronologically most
	// recent period with an available Total Revenue figure, the sum of
	// financial.CodeOpexOwnerComp plus every Input.DiscretionaryCodes amount
	// is at least Thresholds.OwnerDiscretionaryShareOfRevenue of that
	// period's Total Revenue.
	RuleHighOwnerDiscretionaryShare RuleCode = "HIGH_OWNER_DISCRETIONARY_SHARE"
	// RuleUnexpectedNegativeAmount means a revenue account (financial
	// CategoryRevenue) or an expense account (CategoryCogs/CategoryOpex)
	// reports a negative amount at least Thresholds.UnexpectedNegativeMinMagnitude
	// in absolute value — revenue and expense lines are conventionally
	// non-negative in this repository's sign convention (see
	// financial/metrics' package doc comment), so a negative value on one of
	// these codes is a structural anomaly worth surfacing even before
	// considering period-over-period change. A single-period rule: needs no
	// PeriodMeta.
	RuleUnexpectedNegativeAmount RuleCode = "UNEXPECTED_NEGATIVE_AMOUNT"
)

// AnomalySeverity is a structured, matchable urgency signal for an Anomaly,
// never inferred from Explanation text — mirroring qoe.FlagSeverity/
// concentration.FlagSeverity's identical role. Named distinctly from
// IssueSeverity (which uses the plain Severity* names) for the same reason
// every analytics sibling package keeps its finding-severity type separate
// from its input-issue-severity type — see IssueCode's doc comment.
type AnomalySeverity string

const (
	AnomalySeverityInfo     AnomalySeverity = "info"
	AnomalySeverityWarning  AnomalySeverity = "warning"
	AnomalySeverityCritical AnomalySeverity = "critical"
)

// FloatEqualityTolerance documents the precision this package uses whenever
// it must decide "are these two float64 amounts the same value"
// (RuleRepeatedUnusualValue, RuleDuplicateLikeAmounts): amounts are
// considered equal when they round to the same value at cents precision
// (see roundKey in duplicates.go). Fixed (not caller-configurable) because
// floating-point amounts that originated from the same source figure can
// differ in the last bit or two after ingestion/normalization arithmetic,
// and this is a structural equality check, not a materiality judgment a
// caller should tune — every materiality/sensitivity knob in this package
// lives in Thresholds instead. Expressed as a fraction of one dollar
// (0.01 = one cent) so the precision this package actually applies is
// visible on Result without requiring a caller to read duplicates.go.
const FloatEqualityTolerance = 0.01

// Provenance carries the source-row trail for an Anomaly's observed (and,
// when applicable, baseline) figures, when Input.Dataset's underlying
// NormalizedItem values carried Sources — mirrors financial.NormalizedItem.Sources'
// identical "omitted when provenance was not requested or is unavailable"
// convention. Never populated by this package itself; only ever a verbatim
// copy of what Input.Dataset already carried.
type Provenance struct {
	// ObservedSources traces Anomaly.Observed back to its source rows.
	ObservedSources []financial.SourceRef `json:"observed_sources,omitempty"`
	// BaselineSources traces Anomaly.Baseline back to its source rows, when
	// Anomaly has a period-over-period Baseline.
	BaselineSources []financial.SourceRef `json:"baseline_sources,omitempty"`
}

// Anomaly is a single deterministic, explainable finding. Every field this
// package's task instructions require is present: code, severity, account,
// period, baseline, observed value, delta, threshold, explanation, and
// provenance where available. This package never calls a finding fraud —
// see the package doc comment.
type Anomaly struct {
	// Code identifies which detection rule produced this Anomaly. See the
	// RuleCode constants.
	Code RuleCode `json:"code"`
	// Severity is this anomaly's structured urgency — see the
	// AnomalySeverity constants.
	Severity AnomalySeverity `json:"severity"`
	// Account is the financial.Code this anomaly concerns. Empty only for
	// RuleExpenseOutpacingRevenue/RuleMarginDeterioration when the rule
	// fires against a synthetic subtotal (e.g. "operating margin") rather
	// than a single code — see MetricLabel in that case.
	Account financial.Code `json:"account,omitempty"`
	// MetricLabel names the synthetic figure this anomaly concerns when
	// Account is not a single financial.Code (e.g. "gross_margin",
	// "operating_margin" for RuleMarginDeterioration). Empty whenever
	// Account is set.
	MetricLabel string `json:"metric_label,omitempty"`
	// Group is Account's AccountGroup.Name, when Input.AccountGroups covers
	// Account. Empty if AccountGroups did not cover it or was not supplied.
	Group string `json:"group,omitempty"`
	// Period is the period this anomaly's Observed value applies to (the
	// current/later period for every period-over-period rule).
	Period financial.Period `json:"period"`
	// BaselinePeriod is the period Baseline was read from, for every
	// period-over-period rule (empty for single-period rules:
	// RuleUnexpectedNegativeAmount, RuleHighOwnerDiscretionaryShare,
	// RuleRepeatedUnusualValue, RuleDuplicateLikeAmounts — see each
	// RuleCode's doc comment for what it compares instead).
	BaselinePeriod financial.Period `json:"baseline_period,omitempty"`
	// Baseline is the prior/reference value this anomaly's Observed value
	// was compared against. Available is false when this rule has no single
	// baseline figure (e.g. RuleUnexpectedNegativeAmount, or
	// RuleNewMaterialExpenseCategory where the account had no prior amount
	// at all — Available stays false rather than a misleading $0 baseline,
	// distinct from a genuine reported $0 which would set Available true
	// with Value 0).
	Baseline AnomalyValue `json:"baseline"`
	// Observed is the current value that triggered this anomaly.
	Observed AnomalyValue `json:"observed"`
	// Delta is Observed.Value - Baseline.Value when both are Available,
	// else 0 and meaningless. For RulePercentageChangeSpike/
	// RuleExpenseOutpacingRevenue/RuleMarginDeterioration, this is the
	// raw dollar (or, for margin, percentage-point) change — see each
	// RuleCode's doc comment for the exact figure Threshold is compared
	// against, which is not always identical to Delta.
	Delta float64 `json:"delta"`
	// Threshold is the configured Thresholds value that was crossed, echoed
	// here so a caller can display "42.3% vs. a 30.0% threshold" without
	// separately re-reading Options.Thresholds.
	Threshold float64 `json:"threshold"`
	// Explanation is a short, human-readable, already-filled-in description
	// of exactly what crossed which threshold (e.g. "OPEX_MARKETING rose
	// from $10,000.00 to $45,000.00 (350.0%), exceeding the 50.0% spike
	// threshold"). Generated text only, using neutral terms (anomaly,
	// variance, unusual pattern, review recommended) — never an accusation,
	// per the package doc comment. Display only; this package's own logic
	// never parses Explanation — see Code for the stable, matchable signal.
	Explanation string `json:"explanation"`
	// RelatedPeriods lists every additional period this anomaly references
	// beyond Period/BaselinePeriod, populated only by
	// RuleRepeatedUnusualValue and RuleDuplicateLikeAmounts (every period a
	// repeated/duplicate amount appeared in), sorted chronologically when
	// PeriodMeta covers every listed period, else in Dataset lexical order.
	RelatedPeriods []financial.Period `json:"related_periods,omitempty"`
	// RelatedAccounts lists every additional financial.Code this anomaly
	// references beyond Account, populated only by
	// RuleDuplicateLikeAmounts (every other account sharing the duplicated
	// amount), sorted by Code string.
	RelatedAccounts []financial.Code `json:"related_accounts,omitempty"`
	// Provenance traces Observed/Baseline back to source rows, when
	// Input.Dataset's underlying NormalizedItem values carried Sources.
	Provenance Provenance `json:"provenance,omitempty"`
}

// AnomalyValue represents a single figure that may or may not be available,
// mirroring metrics.MetricValue/workingcapital.NWCValue/
// concentration.ConcentrationValue's identical availability convention:
// Available distinguishes "computed/reported to be exactly $0" from "cannot
// be computed because a required input is absent."
type AnomalyValue struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

// Unavailable is the canonical zero-information AnomalyValue.
func Unavailable() AnomalyValue { return AnomalyValue{} }

// AvailableValue reports an AnomalyValue for a successfully computed figure.
func AvailableValue(v float64) AnomalyValue { return AnomalyValue{Available: true, Value: v} }

// Thresholds configures every deterministic anomaly trigger point this
// package evaluates. Every field has a conservative, documented default —
// see DefaultThresholds — matching the qoe.Thresholds/
// concentration.Thresholds zero-value-means-defaults pattern used throughout
// this repository.
type Thresholds struct {
	// AbsoluteAmountSpike is the unsigned dollar change (in the dataset's
	// currency) at or above which RuleAbsoluteAmountSpike triggers. Defaults
	// to 10000.
	AbsoluteAmountSpike float64 `json:"absolute_amount_spike"`
	// PercentageChangeSpike is the |change| / |prior period value| ratio at
	// or above which RulePercentageChangeSpike triggers. Expressed as a
	// decimal (0.5 = 50%). Defaults to 0.5.
	PercentageChangeSpike float64 `json:"percentage_change_spike"`
	// ExpenseOutpacingRevenueGap is the raw percentage-point gap between an
	// expense account's growth rate and Total Revenue's growth rate, over
	// the same adjacent period pair, at or above which
	// RuleExpenseOutpacingRevenue triggers. Defaults to 0.20 (20 points).
	ExpenseOutpacingRevenueGap float64 `json:"expense_outpacing_revenue_gap"`
	// MarginDeteriorationPoints is the raw percentage-point decline in gross
	// or operating margin, period over prior period, at or above which
	// RuleMarginDeterioration triggers. Defaults to 0.10 (10 points).
	MarginDeteriorationPoints float64 `json:"margin_deterioration_points"`
	// NewCategoryMaterialAmount is the absolute-dollar floor a
	// previously-absent expense account's current-period amount must meet
	// or exceed for RuleNewMaterialExpenseCategory to trigger, combined via
	// OR with NewCategoryMaterialPercentOfRevenue (mirroring
	// review.IsMaterial's two-leg materiality test). Defaults to 5000.
	NewCategoryMaterialAmount float64 `json:"new_category_material_amount"`
	// NewCategoryMaterialPercentOfRevenue is a fraction of the current
	// period's Total Revenue (e.g. 0.02 = 2%) that, combined via OR with
	// NewCategoryMaterialAmount, is sufficient for
	// RuleNewMaterialExpenseCategory to trigger. Evaluated only when that
	// period's Total Revenue is available. Defaults to 0.02.
	NewCategoryMaterialPercentOfRevenue float64 `json:"new_category_material_percent_of_revenue"`
	// RepeatedValueMinOccurrences is the minimum number of distinct periods
	// the exact same nonzero amount must appear in, for the same account,
	// before RuleRepeatedUnusualValue triggers. Defaults to 3.
	RepeatedValueMinOccurrences int `json:"repeated_value_min_occurrences"`
	// SignFlipMinMagnitude is the minimum absolute value (in the dataset's
	// currency) both the prior and current period's amount must have,
	// before RuleSignFlip triggers on a sign change between them. Filters
	// out sign noise on near-zero balances. Defaults to 100.
	SignFlipMinMagnitude float64 `json:"sign_flip_min_magnitude"`
	// OwnerDiscretionaryShareOfRevenue is financial.CodeOpexOwnerComp plus
	// every Input.DiscretionaryCodes amount, as a fraction of the most
	// recent available period's Total Revenue, at or above which
	// RuleHighOwnerDiscretionaryShare triggers. Expressed as a decimal (0.15
	// = 15%). Defaults to 0.15.
	OwnerDiscretionaryShareOfRevenue float64 `json:"owner_discretionary_share_of_revenue"`
	// UnexpectedNegativeMinMagnitude is the minimum absolute value a
	// negative revenue or expense amount must have before
	// RuleUnexpectedNegativeAmount triggers. Filters out immaterial
	// rounding noise around zero. Defaults to 1.
	UnexpectedNegativeMinMagnitude float64 `json:"unexpected_negative_min_magnitude"`
}

// DefaultThresholds returns the conservative default Thresholds every field
// documented above uses, applied whenever a caller passes a zero-value
// Thresholds to Calculate (see resolveThresholds). Mirrors
// qoe.DefaultThresholds/concentration.DefaultThresholds's identical role.
func DefaultThresholds() Thresholds {
	return Thresholds{
		AbsoluteAmountSpike:                 10000,
		PercentageChangeSpike:               0.5,
		ExpenseOutpacingRevenueGap:          0.20,
		MarginDeteriorationPoints:           0.10,
		NewCategoryMaterialAmount:           5000,
		NewCategoryMaterialPercentOfRevenue: 0.02,
		RepeatedValueMinOccurrences:         3,
		SignFlipMinMagnitude:                100,
		OwnerDiscretionaryShareOfRevenue:    0.15,
		UnexpectedNegativeMinMagnitude:      1,
	}
}

// resolveThresholds returns t if any field differs from the zero value,
// otherwise DefaultThresholds() — the same zero-value-means-defaults rule
// qoe.resolveThresholds/concentration.resolveThresholds use.
func resolveThresholds(t Thresholds) Thresholds {
	if t == (Thresholds{}) {
		return DefaultThresholds()
	}
	return t
}

// Result is the output of Calculate: every deterministic anomaly found,
// ordered deterministically, plus the resolved configuration and any
// input-validity issues.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// detection-rule set produced this Result — see the FormulaVersion
	// constant's doc comment.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoPeriods) — every other field is then zero-value. Mirrors every
	// analytics sibling package's identical whole-Result availability
	// convention.
	Available bool `json:"available"`

	// Anomalies is every anomaly Calculate detected, ordered by Period
	// (chronologically when PeriodMeta covers it, else dataset lexical
	// order), then by RuleCode's declaration order, then by Account string —
	// never Go map order. See buildOrdering in anomalies.go for the exact
	// comparator.
	Anomalies []Anomaly `json:"anomalies,omitempty"`

	// Summary is a deterministic rollup of Anomalies by rule and severity —
	// see Summary.
	Summary Summary `json:"summary"`

	// Thresholds echoes the resolved Options.Thresholds (after
	// DefaultThresholds substitution) this Result was computed under, so a
	// persisted Result remains self-describing about exactly which trigger
	// points produced its Anomalies.
	Thresholds Thresholds `json:"thresholds"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}

// RuleCount is the number of Anomalies of one RuleCode within a Summary.
type RuleCount struct {
	Code  RuleCode `json:"code"`
	Count int      `json:"count"`
}

// SeverityCount is the number of Anomalies of one AnomalySeverity within a
// Summary.
type SeverityCount struct {
	Severity AnomalySeverity `json:"severity"`
	Count    int             `json:"count"`
}

// Summary is a deterministic rollup of Result.Anomalies, so a caller can
// answer "how many, and how bad" without re-scanning Anomalies itself.
type Summary struct {
	// Total is len(Result.Anomalies).
	Total int `json:"total"`
	// ByRule is one RuleCount per RuleCode with at least one Anomaly,
	// ordered by RuleCode's declaration order — never Go map order.
	ByRule []RuleCount `json:"by_rule,omitempty"`
	// BySeverity is one SeverityCount per AnomalySeverity with at least one
	// Anomaly, ordered info, warning, critical.
	BySeverity []SeverityCount `json:"by_severity,omitempty"`
	// ReviewRecommended is true when Total > 0. Named per this package's
	// neutral-language instruction: this package flags patterns worth a
	// human's attention, never a verdict — see the package doc comment.
	ReviewRecommended bool `json:"review_recommended"`
}
