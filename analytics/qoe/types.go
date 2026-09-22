// Package qoe produces a deterministic quality-of-earnings (QoE) analysis
// from a normalized financial.FinancialDataset plus a caller's confirmed
// financial/adjustments.Adjustment set and a chosen financial/earnings
// maintainable-earnings strategy.
//
// This package computes nothing upstream of that: it does not classify raw
// rows, does not decide which adjustments are legitimate, and does not pick
// a maintainable-earnings strategy on the caller's behalf. It only
// (re)computes financial/metrics.Snapshot values across every period in the
// dataset, walks each period's normalized-EBITDA/SDE bridge via
// financial/adjustments.Apply, and turns that per-period history into a
// single explainable answer to "how good is this earnings number, and why."
//
// Every function here is pure: no I/O, no mutation of caller-owned input, no
// package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
//
// Quality flags (see Flag) are entirely rule-based against
// caller-configurable Thresholds — this package contains no AI/LLM and no
// opaque scoring. The optional 0-100 Score (see score.go) is a documented,
// clearly-labeled heuristic, never an accounting standard.
package qoe

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/earnings"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// FormulaVersion identifies this package's fixed formula/flag/scoring rule
// set (the exact ratio formulas, the Thresholds this package ships as
// DefaultThresholds, the flag-trigger rules in flags.go, and the Score
// formula in score.go). Bump this whenever any of that changes in a way
// that could make a historical Result not reproduce identically under new
// code — see the repository README's versioning-strategy section, which
// this constant follows exactly (metrics.FormulaVersion,
// adjustments.SemanticsVersion, etc.). Echoed on every Result.
const FormulaVersion = "1.0.0"

// Input bundles everything Calculate needs. This package does not require a
// customer/user/application object of any kind — only financial-domain
// data.
type Input struct {
	// Dataset is the normalized financial history to analyze. Required;
	// Calculate returns a Result with Available == false and an Errors
	// entry if Dataset has no periods.
	Dataset financial.FinancialDataset
	// PeriodMeta supplies chronological ordering for Dataset's periods,
	// exactly as financial/metrics.Options.PeriodMeta does. Required for
	// every trend-shaped output this package produces (RevenueGrowth,
	// EBITDAMarginTrend, EBITDAVolatility, SDEVolatility, and every
	// repeated-one-time/declining-margin flag) — Calculate never guesses a
	// period's chronological order from financial.Period's string value.
	// If nil, or if a period present in Dataset has no entry, those
	// specific outputs are left unavailable/empty rather than the whole
	// Result failing (mirroring financial/metrics.Trend's own
	// PeriodOrderError isolation).
	PeriodMeta map[financial.Period]metrics.PeriodInfo
	// Adjustments is every confirmed financial/adjustments.Adjustment to
	// apply, across every period they cover. Calculate does not filter
	// this to Included == true itself — that filtering already happens
	// inside adjustments.Apply per period, exactly as it would for any
	// other caller of that package. An Adjustment whose Period is not
	// present in Dataset simply never contributes to any period's bridge
	// (adjustments.Apply's own SkipWrongPeriod handles this per period).
	Adjustments []adjustments.Adjustment
	// MaintainableEBITDA is the caller's already-computed maintainable
	// EBITDA strategy result (financial/earnings.Calculate run over each
	// period's normalized-EBITDA bridge value). Optional: if
	// Available == false (the zero value), Result.MaintainableEBITDA is
	// simply unavailable and every ratio/flag that depends on it is
	// skipped rather than the whole Result failing.
	MaintainableEBITDA earnings.Result
	// MaintainableSDE is the caller's already-computed maintainable SDE
	// strategy result, the SDE-basis counterpart to MaintainableEBITDA.
	// Optional, same availability handling.
	MaintainableSDE earnings.Result
}

// Options controls Calculate's optional behavior. The zero Options is
// valid: it uses DefaultThresholds and does not compute Score.
type Options struct {
	// Thresholds configures every deterministic quality-flag trigger point
	// (see Thresholds). If the zero value, DefaultThresholds() is used —
	// mirroring review.Policy/resolvePolicy's identical
	// zero-value-means-defaults convention.
	Thresholds Thresholds
	// ComputeScore, when true, causes Calculate to populate Result.Score
	// with the deterministic heuristic 0-100 score (see score.go). Opt-in
	// because a caller that only wants raw measures + flags (per this
	// package's task-level "otherwise return flags + raw measures only"
	// requirement) should get exactly that, with no implied endorsement
	// that a single composite score is meaningful for their use case.
	ComputeScore bool
}

// PeriodFigures is one period's reported vs. normalized EBITDA and SDE, plus
// the full adjustments.Result bridge each normalized figure was walked
// through — the per-period building block Result.History is built from.
type PeriodFigures struct {
	// Period is the reporting period these figures apply to.
	Period financial.Period `json:"period"`
	// ReportedEBITDA is financial/metrics.Snapshot.EBITDA for this period,
	// copied verbatim — "reported" here means "as reconstructed from the
	// dataset's own reported line items," matching financial/metrics' own
	// use of the word (see metrics.ebitda's doc comment): never externally
	// supplied, never adjustment-normalized.
	ReportedEBITDA metrics.MetricValue `json:"reported_ebitda"`
	// NormalizedEBITDA is EBITDABridge.NormalizedValue from applying
	// Input.Adjustments to this period's snapshot, when the bridge's base
	// was available. Available mirrors EBITDABridge.BaseAvailable.
	NormalizedEBITDA metrics.MetricValue `json:"normalized_ebitda"`
	// ReportedSDE is financial/metrics.Snapshot.SDE for this period.
	ReportedSDE metrics.MetricValue `json:"reported_sde"`
	// NormalizedSDE is SDEBridge.NormalizedValue for this period.
	NormalizedSDE metrics.MetricValue `json:"normalized_sde"`
	// Snapshot is the full financial/metrics.Snapshot this period's figures
	// were derived from, carried for callers that want revenue, margins, or
	// any other per-period metric this package does not surface directly by
	// name.
	Snapshot metrics.Snapshot `json:"snapshot"`
	// Adjustments is the full financial/adjustments.Result (both bridges,
	// applied/skipped lines, validation warnings/errors) for this period —
	// the complete auditable trail NormalizedEBITDA/NormalizedSDE were
	// read from.
	Adjustments adjustments.Result `json:"adjustments"`
}

// TypeBreakdown is the total confirmed-and-included adjustment contribution
// for one adjustments.Type, summed across every period and both bridges'
// applied lines independently (see AdjustmentBreakdown's doc comment for why
// EBITDA and SDE are kept separate here).
type TypeBreakdown struct {
	// Type is the adjustments.Type this breakdown covers.
	Type adjustments.Type `json:"type"`
	// EBITDATotal is the sum of every AppliedLine.SignedAmount of this Type
	// across every period's EBITDABridge.
	EBITDATotal float64 `json:"ebitda_total"`
	// EBITDACount is how many applied EBITDA-bridge lines of this Type were
	// summed into EBITDATotal.
	EBITDACount int `json:"ebitda_count"`
	// SDETotal is the sum of every AppliedLine.SignedAmount of this Type
	// across every period's SDEBridge.
	SDETotal float64 `json:"sde_total"`
	// SDECount is how many applied SDE-bridge lines of this Type were
	// summed into SDETotal.
	SDECount int `json:"sde_count"`
}

// AdjustmentBreakdown is the full confirmed-adjustment picture across every
// period in History: a grand total plus a per-type breakdown.
//
// EBITDA and SDE totals are kept separate throughout (never summed
// together) because a single Adjustment can target one bridge, the other,
// or both (adjustments.Target/appliesTo), and its EBITDA-bridge
// SignedAmount and SDE-bridge SignedAmount are not always equal — most
// visibly for TypeOwnerCompensationNormalization, which by default targets
// SDE only (see adjustments.buildTypeRegistry) and therefore contributes
// $0/0 lines to every EBITDATotal while contributing its full amount to
// SDETotal. Adding the two together would silently double-count or
// misrepresent that split.
type AdjustmentBreakdown struct {
	// TotalEBITDAAdjustment is the sum of every applied EBITDA-bridge
	// AppliedLine.SignedAmount across every period in History.
	TotalEBITDAAdjustment float64 `json:"total_ebitda_adjustment"`
	// TotalSDEAdjustment is the sum of every applied SDE-bridge
	// AppliedLine.SignedAmount across every period in History.
	TotalSDEAdjustment float64 `json:"total_sde_adjustment"`
	// ByType breaks TotalEBITDAAdjustment/TotalSDEAdjustment down per
	// adjustments.Type, sorted by Type string for determinism (never Go
	// map iteration order).
	ByType []TypeBreakdown `json:"by_type,omitempty"`
}

// RecurrencePattern is one (Type, effective effect direction) combination's
// history across every period in History — the unit RepeatedOneTime
// detection (see repeated.go) groups and flags on.
type RecurrencePattern struct {
	// Type is the adjustments.Type this pattern covers.
	Type adjustments.Type `json:"type"`
	// Periods lists, in chronological order (when PeriodMeta was supplied;
	// otherwise dataset order), every period in which at least one
	// applied, confirmed adjustment of Type contributed to either bridge.
	Periods []financial.Period `json:"periods"`
	// Count is len(Periods) — the number of distinct periods this Type
	// appeared in, the exact figure RepeatedOneTimeThreshold compares
	// against.
	Count int `json:"count"`
	// TotalAmount is the sum of every contributing AppliedLine's Amount
	// (the unsigned magnitude, not SignedAmount — see
	// adjustments.Adjustment.Amount's own sign-convention doc comment)
	// across Periods, for display alongside Count.
	TotalAmount float64 `json:"total_amount"`
	// LikelyNotNonRecurring is true when Count meets or exceeds
	// Thresholds.RepeatedOneTimeMinPeriods — this package's deterministic
	// signal that a Type nominally meant to be non-recurring (one_time_
	// expense, non_recurring_professional_fees, unusual_gain, unusual_loss)
	// has in fact recurred often enough to doubt that characterization. See
	// the package doc comment: this is a flag only, never an automatic
	// removal.
	LikelyNotNonRecurring bool `json:"likely_not_non_recurring"`
}

// RecurringSummary splits every confirmed, applied adjustment line across
// History into three buckets by its Type's inherent nature — distinct from
// RecurrencePattern, which asks "did this specific Type actually repeat
// across periods." RecurringSummary instead asks "is this Type the kind of
// thing that is expected to recur at all," independent of whether it
// happened to appear once or many times in this particular dataset. See
// recurringTypes/nonRecurringSummaryTypes in qoe.go for the exact
// classification of each built-in adjustments.Type.
//
// EBITDA-bridge and SDE-bridge contributions are kept separate for the
// same reason AdjustmentBreakdown keeps them separate (see that type's doc
// comment): a single Adjustment's EBITDA-bridge and SDE-bridge
// SignedAmount are not always equal.
type RecurringSummary struct {
	// RecurringEBITDATotal/RecurringSDETotal sum every applied AppliedLine
	// whose Adjustment.Type is inherently recurring in nature (owner
	// compensation normalization, personal vehicle/travel, owner-
	// discretionary expense, related-party rent) — items a caller should
	// generally expect to see again in a future period.
	RecurringEBITDATotal float64 `json:"recurring_ebitda_total"`
	RecurringSDETotal    float64 `json:"recurring_sde_total"`
	// RecurringCount is the number of applied lines (across both bridges,
	// counted once per bridge they applied to) summed into the two totals
	// above.
	RecurringCount int `json:"recurring_count"`

	// NonRecurringEBITDATotal/NonRecurringSDETotal sum every applied
	// AppliedLine whose Adjustment.Type is inherently non-recurring in
	// nature (one-time expense, non-recurring professional fees, unusual
	// gain/loss) — the same nonRecurringTypes set RecurrencePattern
	// restricts itself to.
	NonRecurringEBITDATotal float64 `json:"non_recurring_ebitda_total"`
	NonRecurringSDETotal    float64 `json:"non_recurring_sde_total"`
	// NonRecurringCount mirrors RecurringCount for the non-recurring
	// bucket.
	NonRecurringCount int `json:"non_recurring_count"`

	// UnclassifiedEBITDATotal/UnclassifiedSDETotal sum every applied
	// AppliedLine whose Type this package does not confidently place in
	// either bucket: TypeNonOperatingIncome (could be a recurring
	// investment income stream or a one-off gain — its own definition
	// doesn't say which) and TypeCustom (caller-defined, no inherent
	// nature this package can infer). Surfaced explicitly rather than
	// guessed into one of the two buckets above, consistent with this
	// package's "explicit availability instead of treating missing data as
	// zero" design rule applied to classification, not just numeric
	// availability.
	UnclassifiedEBITDATotal float64 `json:"unclassified_ebitda_total"`
	UnclassifiedSDETotal    float64 `json:"unclassified_sde_total"`
	// UnclassifiedCount mirrors RecurringCount for the unclassified
	// bucket.
	UnclassifiedCount int `json:"unclassified_count"`
}

// Ratios holds the adjustment-burden ratios Calculate derives from the most
// recent available period in History — "most recent" meaning
// chronologically last when PeriodMeta orders History, else the last entry
// in Dataset/History order.
type Ratios struct {
	// Period is the period these ratios were computed from.
	Period financial.Period `json:"period"`
	// AdjustmentToEBITDA is |total EBITDA-bridge adjustment for Period| /
	// |reported EBITDA for Period| — how large the normalization burden is
	// relative to the reported base. Available only if reported EBITDA is
	// available and nonzero, and the EBITDA bridge's base was itself
	// available.
	AdjustmentToEBITDA metrics.MetricValue `json:"adjustment_to_ebitda"`
	// AdjustmentToSDE is the same ratio computed against reported SDE and
	// the SDE-bridge adjustment total.
	AdjustmentToSDE metrics.MetricValue `json:"adjustment_to_sde"`
}

// FlagCode is a stable identifier for one kind of deterministic
// earnings-quality flag, analogous to adjustments.IssueCode/review.IssueCode
// in every other package in this repository (see the README's error
// taxonomy). This package defines its own separate system rather than
// reusing adjustments.IssueCode: a QoE quality signal ("earnings are
// volatile") is a different problem domain from an adjustment-set
// consistency problem ("this ID is duplicated"), even though both
// ultimately read adjustments.Result — folding one into the other would
// leak qoe-specific codes into financial/adjustments or vice versa, exactly
// the reasoning the README already applies three times over for
// valuation/adjustments/review.
type FlagCode string

const (
	// FlagLargeNormalizationBurden means AdjustmentToEBITDA or
	// AdjustmentToSDE exceeds Thresholds.LargeNormalizationBurdenRatio.
	FlagLargeNormalizationBurden FlagCode = "LARGE_NORMALIZATION_BURDEN"
	// FlagDecliningEBITDADespiteRevenueGrowth means the most recent
	// year-over-year point in both RevenueGrowth and EBITDAMarginTrend's
	// underlying EBITDA series shows revenue grew while EBITDA declined.
	FlagDecliningEBITDADespiteRevenueGrowth FlagCode = "DECLINING_EBITDA_DESPITE_REVENUE_GROWTH"
	// FlagVolatileEarnings means EBITDAVolatility or SDEVolatility exceeds
	// Thresholds.VolatileEarningsRatio.
	FlagVolatileEarnings FlagCode = "VOLATILE_EARNINGS"
	// FlagInconsistentMargins means EBITDAMarginTrend's margin values swing
	// by more than Thresholds.InconsistentMarginSwing (an absolute
	// percentage-point range: max - min) across the fiscal years used.
	FlagInconsistentMargins FlagCode = "INCONSISTENT_MARGINS"
	// FlagLargeOwnerDiscretionaryComponent means the most recent period's
	// SDE-bridge owner-related adjustment total (owner compensation
	// normalization + personal vehicle + personal travel +
	// owner-discretionary-expense types) is at least
	// Thresholds.OwnerDiscretionaryShareOfSDE of normalized SDE for that
	// period.
	FlagLargeOwnerDiscretionaryComponent FlagCode = "LARGE_OWNER_DISCRETIONARY_COMPONENT"
	// FlagRepeatedOneTimeAdjustments means at least one RecurrencePattern
	// in Result.Recurrence has LikelyNotNonRecurring == true. See
	// RecurrencePattern.
	FlagRepeatedOneTimeAdjustments FlagCode = "REPEATED_ONE_TIME_ADJUSTMENTS"
	// FlagNonOperatingIncomeSupportingEarnings means the most recent
	// period's non-operating-income-removal adjustments (TypeNonOperatingIncome,
	// TypeUnusualGain) total at least Thresholds.NonOperatingIncomeShareOfEBITDA
	// of reported EBITDA for that period.
	FlagNonOperatingIncomeSupportingEarnings FlagCode = "NON_OPERATING_INCOME_SUPPORTING_EARNINGS"
	// FlagNegativeOrNearZeroMaintainableEarnings means MaintainableEBITDA
	// or MaintainableSDE is Available with a Value at or below
	// Thresholds.NearZeroMaintainableEarnings.
	FlagNegativeOrNearZeroMaintainableEarnings FlagCode = "NEGATIVE_OR_NEAR_ZERO_MAINTAINABLE_EARNINGS"
)

// FlagSeverity mirrors adjustments.IssueSeverity/review.Severity's role: a
// structured, matchable urgency signal, never inferred from Message text.
type FlagSeverity string

const (
	// FlagSeverityInfo is a worth-noting observation that does not by
	// itself suggest a problem (e.g. a single moderate flag in an
	// otherwise clean history).
	FlagSeverityInfo FlagSeverity = "info"
	// FlagSeverityWarning means the underlying figure crossed a threshold
	// this package's DefaultThresholds treats as noteworthy.
	FlagSeverityWarning FlagSeverity = "warning"
	// FlagSeverityCritical means the underlying figure crossed a threshold
	// this package's DefaultThresholds treats as materially concerning
	// (e.g. negative maintainable earnings).
	FlagSeverityCritical FlagSeverity = "critical"
)

// Flag is a single deterministic, explainable earnings-quality signal. See
// the package doc comment: every Flag here is rule-based against
// Thresholds, never AI-scored.
type Flag struct {
	// Code identifies which quality signal this is. See the FlagCode
	// constants.
	Code FlagCode `json:"code"`
	// Severity is this flag's structured urgency — see FlagSeverity.
	Severity FlagSeverity `json:"severity"`
	// Period is the period this flag most directly concerns, when
	// applicable (empty for a flag computed across the whole history, e.g.
	// FlagVolatileEarnings).
	Period financial.Period `json:"period,omitempty"`
	// Message is a short, human-readable explanation, already filled in
	// with the specific figures that triggered this flag (e.g. "adjustment
	// total is 42.3% of reported EBITDA, exceeding the 30.0% threshold").
	// Display only — this package's own logic never parses Message; see
	// Code for the stable, matchable signal.
	Message string `json:"message"`
	// Value is the specific computed figure that triggered this flag (a
	// ratio, a margin swing, a volatility statistic, a dollar amount),
	// meaningful only in combination with Code (which states what kind of
	// figure it is).
	Value float64 `json:"value"`
	// Threshold is the configured Thresholds value Value was compared
	// against, echoed here so a caller can display "42.3% vs. a 30.0%
	// threshold" without separately re-reading Options.Thresholds.
	Threshold float64 `json:"threshold"`
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model
// adjustments.IssueSeverity/review.IssueSeverity use.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate system for the same
// reason FlagCode does (see FlagCode's doc comment): an input-validity
// problem ("no periods in dataset") is a different problem domain from a
// quality signal ("earnings are volatile"), so the two are never merged
// into one taxonomy despite living in the same package.
type IssueCode string

const (
	// IssueNoPeriods means Input.Dataset has no periods at all; Calculate
	// returns Available == false.
	IssueNoPeriods IssueCode = "NO_PERIODS"
	// IssueNoPeriodMeta means Input.PeriodMeta was nil or empty, so every
	// trend-shaped output (RevenueGrowth, EBITDAMarginTrend, volatility,
	// the declining-margin/declining-EBITDA flags) could not be computed.
	// Advisory only (SeverityWarning): per-period figures in History are
	// still fully computed without PeriodMeta.
	IssueNoPeriodMeta IssueCode = "NO_PERIOD_META"
	// IssueMaintainableEBITDAUnavailable means Input.MaintainableEBITDA.Available
	// was false, so Result.MaintainableEBITDA and every ratio/flag that
	// depends on it were skipped. Advisory only.
	IssueMaintainableEBITDAUnavailable IssueCode = "MAINTAINABLE_EBITDA_UNAVAILABLE"
	// IssueMaintainableSDEUnavailable is the SDE-basis counterpart to
	// IssueMaintainableEBITDAUnavailable.
	IssueMaintainableSDEUnavailable IssueCode = "MAINTAINABLE_SDE_UNAVAILABLE"
)

// Issue is a single Calculate-time input finding, mirroring
// adjustments.Issue/review.Issue's shape.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from adjustments.HasErrors/valuation.HasErrors/
// review.HasErrors rather than shared — see adjustments.HasErrors's doc
// comment for the full rationale (each package's Issue is a distinct Go
// type with no common interface worth introducing for one boolean
// function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Result is the output of Calculate: the full per-period history, every
// derived summary/ratio/trend figure, deterministic quality flags, and an
// optional heuristic score.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula/flag/scoring rule set produced this Result — see the
	// FormulaVersion constant's doc comment.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoPeriods) — every other field is then zero-value. This mirrors
	// metrics.MetricValue's availability convention at the whole-Result
	// level, the same pattern earnings.Result uses.
	Available bool `json:"available"`

	// History is one PeriodFigures per period in Input.Dataset, ordered
	// chronologically when Input.PeriodMeta was supplied and covers every
	// period; otherwise in Dataset.Periods()'s lexical order (see
	// financial.FinancialDataset.Periods).
	History []PeriodFigures `json:"history,omitempty"`

	// Adjustments is the full confirmed-adjustment breakdown across every
	// period in History. See AdjustmentBreakdown.
	Adjustments AdjustmentBreakdown `json:"adjustments"`
	// Recurrence lists every (Type) RecurrencePattern observed across
	// History, sorted by Type string for determinism. See
	// RecurrencePattern.
	Recurrence []RecurrencePattern `json:"recurrence,omitempty"`
	// RecurringAdjustments splits every confirmed, applied adjustment line
	// across History into recurring/non-recurring/unclassified buckets by
	// each line's Type — distinct from Recurrence, which tracks whether a
	// specific Type actually repeated across periods in THIS dataset. See
	// RecurringSummary.
	RecurringAdjustments RecurringSummary `json:"recurring_adjustments"`

	// MaintainableEBITDA echoes Input.MaintainableEBITDA when it was
	// Available, else the zero earnings.Result (Available == false).
	MaintainableEBITDA earnings.Result `json:"maintainable_ebitda"`
	// MaintainableSDE echoes Input.MaintainableSDE when it was Available.
	MaintainableSDE earnings.Result `json:"maintainable_sde"`

	// RevenueGrowth is financial/metrics.Trend.RevenueYoYGrowth,
	// recomputed by this package's own metrics.Calculate call over
	// Dataset/PeriodMeta (see qoe.go) — not re-derived by hand, so it
	// always matches exactly what financial/metrics itself would report
	// for this dataset. Empty if PeriodMeta was not supplied or fewer than
	// two comparable fiscal years exist.
	RevenueGrowth []metrics.GrowthPoint `json:"revenue_growth,omitempty"`
	// RevenueVolatility is financial/metrics.Trend.RevenueVolatility,
	// copied verbatim — "revenue stability" is this same statistic:
	// smaller means more stable.
	RevenueVolatility metrics.VolatilityResult `json:"revenue_volatility"`
	// EBITDAMarginTrend is financial/metrics.Trend.EBITDAMarginTrend,
	// copied verbatim.
	EBITDAMarginTrend []metrics.MarginPoint `json:"ebitda_margin_trend,omitempty"`
	// EBITDAVolatility is financial/metrics.Trend.EBITDAVolatility, copied
	// verbatim — this package's "earnings volatility" measure for the
	// EBITDA basis.
	EBITDAVolatility metrics.VolatilityResult `json:"ebitda_volatility"`
	// SDEVolatility is the same volatility statistic computed over the SDE
	// series (financial/metrics.Trend does not itself expose this one, so
	// this package computes it directly using the identical method — see
	// qoe.go's sdeVolatility/calculateVolatility).
	SDEVolatility metrics.VolatilityResult `json:"sde_volatility"`

	// Ratios holds the most-recent-period adjustment-burden ratios. See
	// Ratios.
	Ratios Ratios `json:"ratios"`

	// Flags is every deterministic quality flag Calculate triggered,
	// ordered by FlagCode's declaration order (LargeNormalizationBurden
	// first through NegativeOrNearZeroMaintainableEarnings last), then by
	// Period — never Go map order, and never by an implied "worst first"
	// rank the way review.Severity is ranked, since a QoE flag list is
	// read as a checklist, not a priority queue.
	Flags []Flag `json:"flags,omitempty"`

	// Score is the optional deterministic heuristic score, populated only
	// when Options.ComputeScore is true. Nil otherwise. See score.go.
	Score *Score `json:"score,omitempty"`

	// Thresholds echoes the resolved Options.Thresholds (after
	// DefaultThresholds substitution) this Result was computed under, so a
	// persisted Result remains self-describing about exactly which
	// trigger points produced its Flags.
	Thresholds Thresholds `json:"thresholds"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
