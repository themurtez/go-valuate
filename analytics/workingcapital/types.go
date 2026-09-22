// Package workingcapital analyzes historical operating working capital from
// a normalized financial.FinancialDataset and supports transaction-style
// working-capital peg analysis (comparing a current/target working-capital
// figure against a caller-selected historical benchmark).
//
// This package does not hard-code a single "deal definition" of working
// capital. Which balance-sheet codes count as operating current assets and
// operating current liabilities — and whether cash, debt, taxes payable,
// related-party balances, or other items are included or excluded — is
// entirely controlled by a caller-supplied InclusionPolicy. Two callers
// analyzing the same dataset under different deal conventions get different,
// independently correct answers; this package never picks one convention as
// "the" market definition.
//
// Every function here is pure: no I/O, no mutation of caller-owned input, no
// package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON.
package workingcapital

import "github.com/themurtez/go-valuate/financial"

// FormulaVersion identifies this package's fixed formula set (NWC
// definition given an InclusionPolicy, the statistics in Statistics, the
// peg methods in peg.go, and the seasonal-profile method in seasonal.go).
// Bump this whenever any of that changes in a way that could make a
// historical Result not reproduce identically under new code — see the
// repository README's versioning-strategy section, which this constant
// follows exactly (financial.TaxonomyVersion, metrics.FormulaVersion,
// adjustments.SemanticsVersion, qoe.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// PeriodType mirrors metrics.PeriodType (fiscal year, YTD, quarter, month)
// so this package can reason about seasonal grouping and comparability
// without requiring a caller to import financial/metrics solely for these
// values. See PeriodInfo.
type PeriodType string

const (
	PeriodTypeFiscalYear PeriodType = "fiscal_year"
	PeriodTypeYTD        PeriodType = "ytd"
	PeriodTypeQuarter    PeriodType = "quarter"
	PeriodTypeMonth      PeriodType = "month"
)

// PeriodInfo is caller-supplied metadata about a single financial.Period,
// used to determine chronological order, comparability, and (for
// quarter/month periods) seasonal position. financial.Period is
// intentionally just a string with no guaranteed sort order (see
// financial.FinancialDataset.Periods' doc comment); this package requires
// PeriodInfo rather than inferring order or granularity from the string,
// mirroring financial/metrics' and financial/earnings' identical
// no-guessing rule.
type PeriodInfo struct {
	// Type is this period's granularity.
	Type PeriodType `json:"type"`
	// FiscalYear is the fiscal year this period falls within. Required for
	// chronological ordering.
	FiscalYear int `json:"fiscal_year"`
	// SequenceInYear orders periods that share the same FiscalYear and Type
	// (e.g. Q1=1..Q4=4, or month 1-12), and — for PeriodTypeQuarter/
	// PeriodTypeMonth — identifies seasonal position (see SeasonalProfile).
	// Ignored for PeriodTypeFiscalYear and PeriodTypeYTD.
	SequenceInYear int `json:"sequence_in_year,omitempty"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// Dataset is the normalized financial history to analyze. Required;
	// Calculate returns a Result with Available == false and an Errors entry
	// if Dataset has no periods.
	Dataset financial.FinancialDataset
	// PeriodMeta supplies chronological ordering (and, for quarter/month
	// periods, seasonal position) for Dataset's periods. Required for every
	// trend/seasonal output this package produces; if nil, or a period
	// present in Dataset has no entry, those specific outputs are left
	// unavailable rather than the whole Result failing — mirroring
	// financial/metrics.Trend's PeriodOrderError isolation and
	// analytics/qoe.Input.PeriodMeta's identical convention.
	PeriodMeta map[financial.Period]PeriodInfo
	// AsOf is the current transaction date's period, when analyzing a live
	// deal. Optional: if empty, or not present in Dataset, CurrentNWC and
	// PegComparison are left unavailable and Calculate proceeds using only
	// Dataset's historical periods.
	AsOf financial.Period
	// CurrentNWC, when Available, overrides the current net working capital
	// figure used for PegComparison instead of deriving it from Dataset at
	// AsOf. This supports the common transaction scenario where a caller has
	// a more current (e.g. estimated/interim close-date) NWC figure than
	// what the normalized historical dataset contains. If not Available and
	// AsOf is set and present in Dataset, Calculate derives the current NWC
	// from Dataset at AsOf instead.
	CurrentNWC NWCValue
}

// NWCValue represents a net-working-capital figure that may or may not be
// calculable, mirroring metrics.MetricValue's availability convention:
// Available distinguishes "computed to be exactly $0" from "cannot be
// computed because a required input is absent."
type NWCValue struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

// Unavailable is the canonical zero-information NWCValue.
func Unavailable() NWCValue { return NWCValue{} }

// AvailableValue reports an NWCValue for a successfully computed figure.
func AvailableValue(v float64) NWCValue { return NWCValue{Available: true, Value: v} }

// Component is one named balance-sheet code (or synthetic subtotal) that
// contributed to a computed NWC figure, mirroring metrics.Component's role:
// carried so callers can explain how a figure was produced without this
// package implementing a generic expression engine.
type Component struct {
	// Code identifies the canonical financial.Code this component came
	// from, or a synthetic identifier ("OPERATING_CURRENT_ASSETS",
	// "OPERATING_CURRENT_LIABILITIES") for a component that is itself a
	// computed subtotal.
	Code string `json:"code"`
	// Label is a short human-readable description of the component.
	Label string `json:"label,omitempty"`
	// Amount is the value contributed by this component, in the dataset's
	// native sign convention (assets positive, liabilities positive — sign
	// flipping for the NWC subtraction happens only in Bridge).
	Amount float64 `json:"amount"`
}

// PeriodNWC is one period's operating net working capital, its component
// bridge, and NWC as a percentage of revenue for that period.
type PeriodNWC struct {
	// Period is the reporting period this figure applies to.
	Period financial.Period `json:"period"`
	// OperatingCurrentAssets is the sum of every financial.Code the
	// InclusionPolicy includes as an operating current asset for this
	// period. Available only if at least one included code is present in
	// Dataset for this period.
	OperatingCurrentAssets NWCValue `json:"operating_current_assets"`
	// OperatingCurrentLiabilities is the sum of every financial.Code the
	// InclusionPolicy includes as an operating current liability for this
	// period.
	OperatingCurrentLiabilities NWCValue `json:"operating_current_liabilities"`
	// NWC is OperatingCurrentAssets - OperatingCurrentLiabilities. Available
	// only if both inputs are available.
	NWC NWCValue `json:"nwc"`
	// Revenue is financial.CodeRevProduct + CodeRevService +
	// CodeRevRecurring + CodeRevOther for this period, when at least one is
	// present — this package's own revenue total, computed the same way
	// metrics.totalRevenue is, so this package does not require a caller to
	// separately run financial/metrics just to get NWCPercentOfRevenue.
	Revenue NWCValue `json:"revenue"`
	// NWCPercentOfRevenue is NWC.Value / Revenue.Value. Available only if
	// both NWC and Revenue are available and Revenue.Value != 0.
	NWCPercentOfRevenue NWCValue `json:"nwc_percent_of_revenue"`
	// AssetComponents lists each included operating-current-asset code
	// present in Dataset for this period, and its amount.
	AssetComponents []Component `json:"asset_components,omitempty"`
	// LiabilityComponents lists each included operating-current-liability
	// code present in Dataset for this period, and its amount.
	LiabilityComponents []Component `json:"liability_components,omitempty"`
}

// Statistics summarizes a series of NWC (or NWC-percent-of-revenue)
// observations across History: central tendency, dispersion, and range.
type Statistics struct {
	// Average is the unweighted arithmetic mean across every available
	// observation.
	Average NWCValue `json:"average"`
	// Median is the median across every available observation.
	Median NWCValue `json:"median"`
	// Min and Max are the smallest and largest available observations.
	Min NWCValue `json:"min"`
	// Max is the largest available observation.
	Max NWCValue `json:"max"`
	// Volatility is the sample standard deviation of the period-over-period
	// percentage-change series (the same method financial/metrics.Trend
	// uses for revenue/EBITDA volatility — see calculateVolatility),
	// expressed as a decimal. Requires at least 3 chronologically ordered
	// observations (2 growth-rate points) with no zero-base transition; see
	// VolatilitySampleSize.
	Volatility NWCValue `json:"volatility"`
	// VolatilitySampleSize is the number of period-over-period growth
	// observations Volatility was computed from.
	VolatilitySampleSize int `json:"volatility_sample_size,omitempty"`
	// SampleSize is the number of available observations Average/Median/
	// Min/Max were computed from.
	SampleSize int `json:"sample_size"`
}

// TrendDirection is a coarse, deterministic characterization of a series'
// overall direction, computed from its first vs. last available
// observation — never inferred from a Message string.
type TrendDirection string

const (
	// TrendIncreasing means the last available observation exceeds the
	// first by more than TrendFlatBandPercent (see Trend).
	TrendIncreasing TrendDirection = "increasing"
	// TrendDeclining means the last available observation is below the
	// first by more than TrendFlatBandPercent.
	TrendDeclining TrendDirection = "declining"
	// TrendStable means the first and last available observations are
	// within TrendFlatBandPercent of each other.
	TrendStable TrendDirection = "stable"
	// TrendUnavailable means fewer than two available observations exist,
	// so no direction could be determined.
	TrendUnavailable TrendDirection = "unavailable"
)

// TrendFlatBandPercent is the |change| / |first value| band, as a decimal,
// within which a series is characterized TrendStable rather than
// TrendIncreasing/TrendDeclining. Fixed (not caller-configurable) because it
// is part of this package's versioned FormulaVersion, exactly like
// analytics/qoe's fixed flag-trigger formulas are versioned rather than
// exposed as yet another threshold knob for a coarse three-way
// classification.
const TrendFlatBandPercent = 0.05

// Trend is a first-vs-last-observation direction characterization of a
// series (see TrendDirection), plus the underlying comparison values for
// display.
type Trend struct {
	Direction TrendDirection `json:"direction"`
	// FirstPeriod/LastPeriod are the chronologically first/last periods
	// with an available observation, when Direction is not
	// TrendUnavailable.
	FirstPeriod financial.Period `json:"first_period,omitempty"`
	LastPeriod  financial.Period `json:"last_period,omitempty"`
	// FirstValue/LastValue are the corresponding observation values.
	FirstValue NWCValue `json:"first_value"`
	LastValue  NWCValue `json:"last_value"`
	// PercentChange is (LastValue - FirstValue) / |FirstValue|. Available
	// only if both values are available and FirstValue.Value != 0.
	PercentChange NWCValue `json:"percent_change"`
}

// SeasonalPeriod is one calendar position's (e.g. "Q1", month 3) average
// NWC-as-percent-of-revenue across every fiscal year sharing that position
// in History, when monthly or quarterly detail is present in Dataset.
type SeasonalPeriod struct {
	// SequenceInYear is the PeriodInfo.SequenceInYear this entry
	// summarizes (1-4 for quarters, 1-12 for months).
	SequenceInYear int `json:"sequence_in_year"`
	// AverageNWCPercentOfRevenue is the unweighted mean of
	// PeriodNWC.NWCPercentOfRevenue across every period in History sharing
	// this SequenceInYear and PeriodType, when at least one is available.
	AverageNWCPercentOfRevenue NWCValue `json:"average_nwc_percent_of_revenue"`
	// SampleSize is the number of fiscal years contributing to the average.
	SampleSize int `json:"sample_size"`
}

// SeasonalProfile groups PeriodNWC observations by calendar position across
// fiscal years, revealing a recurring seasonal pattern (e.g. Q4 always
// carries higher working capital due to holiday inventory build). Populated
// only when Dataset contains PeriodTypeQuarter or PeriodTypeMonth periods
// per PeriodMeta — a purely annual dataset has no seasonal pattern to
// extract, so SeasonalProfile is left empty (not an error).
type SeasonalProfile struct {
	// PeriodType is PeriodTypeQuarter or PeriodTypeMonth — SeasonalProfile
	// never mixes the two granularities in one profile.
	PeriodType PeriodType `json:"period_type"`
	// Periods is one SeasonalPeriod per distinct SequenceInYear observed,
	// sorted by SequenceInYear ascending.
	Periods []SeasonalPeriod `json:"periods,omitempty"`
}

// PegMethod identifies which deterministic strategy SuggestedPeg uses to
// derive a suggested working-capital peg from History. PegMethod is a plain
// string (mirroring financial.Code/adjustments.Type/earnings.Strategy) so
// new methods can be added later without breaking existing callers. This
// package never invents a "market" peg method — every method here reduces
// to arithmetic over the caller's own historical NWC series or a
// caller-supplied fixed figure.
type PegMethod string

const (
	// PegMethodLatest uses the single most recent period's NWC in History.
	PegMethodLatest PegMethod = "latest"
	// PegMethodSimpleAverage is the unweighted arithmetic mean of every
	// available NWC observation in History.
	PegMethodSimpleAverage PegMethod = "simple_average"
	// PegMethodMedian is the median of every available NWC observation in
	// History.
	PegMethodMedian PegMethod = "median"
	// PegMethodTrailingAverage is the unweighted arithmetic mean of the
	// most recent TrailingPeriods available observations in History. See
	// Options.TrailingPeriods.
	PegMethodTrailingAverage PegMethod = "trailing_average"
	// PegMethodFixed uses Options.FixedPeg verbatim, for a caller that has
	// already negotiated a specific peg dollar figure outside this package
	// (e.g. from a letter of intent) and wants PegComparison computed
	// against it.
	PegMethodFixed PegMethod = "fixed"
)

// Options controls Calculate's optional behavior.
type Options struct {
	// InclusionPolicy configures which financial.Code values count as
	// operating current assets/liabilities. If the zero value (no codes
	// configured on either side), DefaultInclusionPolicy() is used — see
	// resolveInclusionPolicy.
	InclusionPolicy InclusionPolicy
	// PegMethod selects the strategy SuggestedPeg uses. If empty,
	// SuggestedPeg is left unavailable (this package never guesses a
	// default deal convention — see the package doc comment).
	PegMethod PegMethod
	// TrailingPeriods is the window size for PegMethodTrailingAverage.
	// Required (> 0) when PegMethod is PegMethodTrailingAverage; otherwise
	// ignored.
	TrailingPeriods int
	// FixedPeg is the caller-supplied peg figure for PegMethodFixed.
	// Ignored by every other PegMethod.
	FixedPeg NWCValue
}

// PegComparison compares the current NWC (from Input.CurrentNWC or derived
// at Input.AsOf) against SuggestedPeg.
type PegComparison struct {
	// CurrentNWC is the current NWC figure used for this comparison —
	// Input.CurrentNWC if Available, else the NWC derived from Dataset at
	// Input.AsOf.
	CurrentNWC NWCValue `json:"current_nwc"`
	// Peg is the peg figure compared against (SuggestedPeg.Value).
	Peg NWCValue `json:"peg"`
	// ExcessDeficit is CurrentNWC - Peg: positive means current NWC exceeds
	// the peg (a seller-favorable excess, under the common convention that
	// a seller is credited for NWC delivered above the peg), negative means
	// a deficit below the peg. Available only if both CurrentNWC and Peg
	// are available. This package does not decide who owes whom — it only
	// reports the signed arithmetic difference; the deal-specific
	// true-up/settlement mechanics are outside this package's scope.
	ExcessDeficit NWCValue `json:"excess_deficit"`
}

// SuggestedPeg is the peg figure derived under Options.PegMethod, plus
// enough context to explain how it was derived.
type SuggestedPeg struct {
	// Method is the Options.PegMethod this peg was derived under. Empty if
	// Options.PegMethod was empty (no peg requested).
	Method PegMethod `json:"method,omitempty"`
	// Value is the derived peg figure. Unavailable if Method is empty, or
	// if the method's required inputs were not available (e.g.
	// PegMethodTrailingAverage with TrailingPeriods exceeding the number of
	// available observations).
	Value NWCValue `json:"value"`
	// PeriodsUsed lists, in chronological order, the periods whose NWC
	// observations contributed to Value (empty for PegMethodFixed, which
	// uses no historical periods).
	PeriodsUsed []financial.Period `json:"periods_used,omitempty"`
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model
// adjustments.IssueSeverity/review.IssueSeverity/qoe.IssueSeverity use.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository (financial/adjustments.IssueCode,
// review.IssueCode, analytics/qoe.IssueCode) rather than reusing one of
// theirs — a working-capital input problem is a distinct problem domain.
type IssueCode string

const (
	// IssueNoPeriods means Input.Dataset has no periods at all; Calculate
	// returns Available == false.
	IssueNoPeriods IssueCode = "NO_PERIODS"
	// IssueNoPeriodMeta means Input.PeriodMeta was nil or empty, so
	// chronological ordering, Trend, and SeasonalProfile could not be
	// computed. Advisory only: per-period History figures are still fully
	// computed in Dataset.Periods() lexical order.
	IssueNoPeriodMeta IssueCode = "NO_PERIOD_META"
	// IssuePeriodMissingFromMeta means at least one period present in
	// Dataset has no PeriodMeta entry, so History falls back to lexical
	// order and Trend/SeasonalProfile are unavailable — mirroring
	// financial/metrics.PeriodOrderError's all-or-nothing ordering rule.
	IssuePeriodMissingFromMeta IssueCode = "PERIOD_MISSING_FROM_META"
	// IssueNoRevenueData means Input.Dataset has no revenue codes present
	// for any period, so every PeriodNWC.NWCPercentOfRevenue is
	// unavailable. Advisory only.
	IssueNoRevenueData IssueCode = "NO_REVENUE_DATA"
	// IssueEmptyInclusionPolicy means Options.InclusionPolicy resolved to
	// having no codes on one or both sides (only possible if a caller
	// explicitly sets an empty, non-zero InclusionPolicy — see
	// resolveInclusionPolicy), so OperatingCurrentAssets or
	// OperatingCurrentLiabilities is unavailable for every period.
	IssueEmptyInclusionPolicy IssueCode = "EMPTY_INCLUSION_POLICY"
	// IssuePegMethodUnavailable means Options.PegMethod was set but its
	// required inputs were not available (e.g. no History observations, or
	// TrailingPeriods exceeding the available observation count), so
	// SuggestedPeg.Value is unavailable.
	IssuePegMethodUnavailable IssueCode = "PEG_METHOD_UNAVAILABLE"
	// IssueCurrentNWCUnavailable means neither Input.CurrentNWC nor a
	// Dataset-derived figure at Input.AsOf was available, so PegComparison
	// is unavailable even though SuggestedPeg may still be populated.
	IssueCurrentNWCUnavailable IssueCode = "CURRENT_NWC_UNAVAILABLE"
)

// Issue is a single Calculate-time input finding, mirroring
// adjustments.Issue/review.Issue/qoe.Issue's shape.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from adjustments.HasErrors/valuation.HasErrors/
// review.HasErrors/qoe.HasErrors rather than shared — see
// adjustments.HasErrors's doc comment for the full rationale (each
// package's Issue is a distinct Go type with no common interface worth
// introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Result is the output of Calculate: the full per-period NWC history,
// summary statistics, trend/seasonal characterization, and (if requested) a
// suggested peg and current excess/deficit comparison.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoPeriods) — every other field is then zero-value.
	Available bool `json:"available"`

	// History is one PeriodNWC per period in Input.Dataset, ordered
	// chronologically when Input.PeriodMeta was supplied and covers every
	// period; otherwise in Dataset.Periods()'s lexical order.
	History []PeriodNWC `json:"history,omitempty"`

	// NWCStatistics summarizes History's NWC.Value series (see Statistics).
	NWCStatistics Statistics `json:"nwc_statistics"`
	// NWCPercentOfRevenueStatistics summarizes History's
	// NWCPercentOfRevenue series.
	NWCPercentOfRevenueStatistics Statistics `json:"nwc_percent_of_revenue_statistics"`
	// Trend characterizes NWC.Value's overall first-vs-last direction
	// across History. See Trend/TrendDirection.
	Trend Trend `json:"trend"`
	// SeasonalProfile groups History by calendar position when quarterly or
	// monthly detail is present. Zero value (empty Periods) when Dataset
	// contains only annual/YTD periods.
	SeasonalProfile SeasonalProfile `json:"seasonal_profile"`

	// SuggestedPeg is the peg derived under Options.PegMethod. Zero value
	// (Method == "") if Options.PegMethod was not set.
	SuggestedPeg SuggestedPeg `json:"suggested_peg"`
	// PegComparison compares the current NWC against SuggestedPeg.Value.
	// Zero value if either side is unavailable — see
	// IssueCurrentNWCUnavailable/IssuePegMethodUnavailable.
	PegComparison PegComparison `json:"peg_comparison"`

	// ExcludedCodes lists every financial.Code present in Dataset's balance
	// sheet items that InclusionPolicy did not assign to either side (asset
	// or liability) — the accounts this analysis deliberately left out, for
	// auditability. Sorted by Code for determinism.
	ExcludedCodes []financial.Code `json:"excluded_codes,omitempty"`

	// InclusionPolicy echoes the resolved Options.InclusionPolicy (after
	// DefaultInclusionPolicy substitution) this Result was computed under,
	// so a persisted Result remains self-describing about exactly which
	// codes were included/excluded.
	InclusionPolicy InclusionPolicy `json:"inclusion_policy"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
