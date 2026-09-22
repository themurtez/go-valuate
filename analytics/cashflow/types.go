// Package cashflow produces a deterministic cash-flow and cash-conversion
// analysis from a normalized financial.FinancialDataset, suitable for SMB
// advisory and transaction (M&A) use: a period-by-period bridge from EBITDA
// to free cash flow, EBITDA-to-cash conversion ratios, debt-service and
// owner-distribution coverage, and (for loss-making businesses) a cash
// burn/runway estimate.
//
// This repository's canonical financial.Code taxonomy has no dedicated
// codes for a cash-flow statement (financial.StatementCashFlow exists as a
// StatementType but no canonical code is tagged with it), for capital
// expenditures, for debt-service principal/interest, or for owner
// distributions — those figures vary too much in how source systems report
// them (a single "cash flow statement" export, a loan amortization
// schedule, a distributions ledger) to force into one flat code list. So
// this package takes EBITDA and working-capital changes from
// financial/metrics (recomputed internally from Input.Dataset, exactly as
// analytics/qoe, analytics/ratios, and analytics/workingcapital already
// do), and takes operating cash flow, capex, debt service, owner
// distributions, and cash taxes paid as caller-supplied, per-period,
// explicitly-available CashFlowValue figures — this package never invents
// a canonical source for them.
//
// Nothing here is inferred from EBITDA unless a caller explicitly opts in
// via Options.AllowEBITDAEstimate. When it does infer a figure, that
// specific CashFlowValue carries IsEstimate == true and a non-empty
// EstimateBasis explaining exactly how it was derived — this package never
// silently blends a reported figure and an estimated one into one
// undifferentiated number.
//
// Every function here is pure: no I/O, no mutation of caller-owned input, no
// package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package cashflow

import (
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// FormulaVersion identifies this package's fixed formula set: the
// EBITDA-to-FCF bridge, every conversion ratio, the coverage formulas, the
// burn/runway calculation, and the EBITDA-based estimate method (when
// Options.AllowEBITDAEstimate is set). Bump this whenever any of that
// changes in a way that could make a historical Result not reproduce
// identically under new code — see the repository README's
// versioning-strategy section, which this constant follows exactly
// (financial.TaxonomyVersion, metrics.FormulaVersion,
// workingcapital.FormulaVersion, qoe.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// PeriodType mirrors metrics.PeriodType (fiscal year, YTD, quarter, month),
// duplicated here rather than aliased so this package's own doc comments
// apply directly at the call site — the same choice
// analytics/workingcapital and analytics/ratios already made for their own
// PeriodType (see those packages' identical doc-comment rationale).
type PeriodType string

const (
	PeriodTypeFiscalYear PeriodType = "fiscal_year"
	PeriodTypeYTD        PeriodType = "ytd"
	PeriodTypeQuarter    PeriodType = "quarter"
	PeriodTypeMonth      PeriodType = "month"
)

// PeriodInfo is caller-supplied metadata about a single financial.Period,
// used to determine chronological order and comparability for Trend and
// the burn/runway calculation. financial.Period is intentionally just a
// string with no guaranteed sort order (see
// financial.FinancialDataset.Periods' doc comment); this package requires
// PeriodInfo rather than inferring order or granularity from the string,
// mirroring financial/metrics', analytics/qoe's, and
// analytics/workingcapital's identical no-guessing rule.
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

// CashFlowValue represents a single cash-flow-statement-shaped figure
// (operating cash flow, capex, a debt-service component, owner
// distributions, cash taxes paid) that may or may not be available, and
// that may be either reported (caller-supplied, exactly as recorded) or
// estimated (derived by this package — see Options.AllowEBITDAEstimate).
//
// This mirrors metrics.MetricValue/workingcapital.NWCValue's availability
// convention (Available distinguishes "reported/computed to be exactly $0"
// from "unknown because the input was not supplied"), plus an explicit
// reported-vs-estimate distinction this package's sibling packages don't
// need, since every figure here can plausibly come from a human-entered
// cash-flow statement instead of being derived by this package.
type CashFlowValue struct {
	// Available is true if Value is meaningful — either reported by the
	// caller or (only when IsEstimate is also true) derived by this
	// package. False means this figure could not be determined at all.
	Available bool `json:"available"`
	// Value is the figure itself. Meaningful only when Available is true.
	Value float64 `json:"value"`
	// IsEstimate is true when this package derived Value itself (always
	// under Options.AllowEBITDAEstimate — see EstimateBasis) rather than
	// the caller supplying a reported figure directly. A caller filtering
	// for "reported data only" checks this field, never Message text.
	IsEstimate bool `json:"is_estimate"`
	// EstimateBasis is a short, fixed, human-readable description of
	// exactly how Value was derived when IsEstimate is true (e.g. "EBITDA -
	// change in net working capital; no reported operating cash flow
	// supplied"). Empty when IsEstimate is false.
	EstimateBasis string `json:"estimate_basis,omitempty"`
}

// Unavailable is the canonical zero-information CashFlowValue.
func Unavailable() CashFlowValue { return CashFlowValue{} }

// Reported reports a CashFlowValue for a caller-supplied, as-recorded
// figure (IsEstimate == false).
func Reported(v float64) CashFlowValue { return CashFlowValue{Available: true, Value: v} }

// Estimated reports a CashFlowValue this package derived itself, with basis
// explaining exactly how (IsEstimate == true).
func Estimated(v float64, basis string) CashFlowValue {
	return CashFlowValue{Available: true, Value: v, IsEstimate: true, EstimateBasis: basis}
}

// DebtServiceFigure splits one period's debt service into its principal and
// interest components, since debt-service coverage conventions differ on
// whether interest (already deducted in EBITDA-derived earnings measures
// upstream of this package) should be included. Bridge.FreeCashFlowToFirm
// and Bridge.FreeCashFlowToOwner report both variants rather than picking
// one — see their doc comments.
type DebtServiceFigure struct {
	// Principal is the cash principal repaid in the period. Available only
	// if the caller supplied it.
	Principal CashFlowValue `json:"principal"`
	// Interest is the cash interest paid in the period. Available only if
	// the caller supplied it.
	Interest CashFlowValue `json:"interest"`
}

// Total returns Principal + Interest, available only if both are available.
func (d DebtServiceFigure) Total() CashFlowValue {
	if !d.Principal.Available || !d.Interest.Available {
		return Unavailable()
	}
	return Reported(d.Principal.Value + d.Interest.Value)
}

// Input bundles everything Calculate needs.
type Input struct {
	// Dataset is the normalized financial history to analyze. Required;
	// Calculate returns a Result with Available == false and an Errors
	// entry if Dataset has no periods.
	Dataset financial.FinancialDataset
	// PeriodMeta supplies chronological ordering for Dataset's periods.
	// Required for Trend and the burn/runway calculation; if nil, or a
	// period present in Dataset has no entry, those specific outputs are
	// left unavailable rather than the whole Result failing — mirroring
	// analytics/workingcapital.Input.PeriodMeta's identical convention.
	PeriodMeta map[financial.Period]metrics.PeriodInfo

	// OperatingCashFlow is the caller-supplied, as-reported cash from
	// operations for each period (e.g. from an actual cash-flow
	// statement), keyed by financial.Period. Optional per period: a period
	// missing from this map, or present with Available == false, has no
	// reported operating cash flow — see Options.AllowEBITDAEstimate for
	// the only way this package will fill that gap itself.
	OperatingCashFlow map[financial.Period]CashFlowValue
	// Capex is the caller-supplied capital expenditure for each period
	// (as a positive outflow magnitude, in the same sign convention as
	// financial.NormalizedItem.Amount for expenses — see FreeCashFlow's
	// doc comment for exactly how it is subtracted).
	Capex map[financial.Period]CashFlowValue
	// DebtService is the caller-supplied debt-service figure for each
	// period. Optional per period.
	DebtService map[financial.Period]DebtServiceFigure
	// OwnerDistributions is the caller-supplied cash distributions/dividends
	// paid to owners/shareholders for each period. Distinct from
	// financial.CodeOpexOwnerComp (W-2/payroll owner compensation, already
	// inside EBITDA) — a distribution is an equity draw made from cash
	// already generated, not an operating expense.
	OwnerDistributions map[financial.Period]CashFlowValue
	// CashTaxesPaid is the caller-supplied cash income tax actually paid
	// for each period, when known to differ from financial.CodeIncomeTax's
	// book tax expense (e.g. due to timing differences). Optional; this
	// package does not require it for any calculation here — it is
	// surfaced on Bridge purely for context alongside the other cash-flow
	// components a reader would expect to see.
	CashTaxesPaid map[financial.Period]CashFlowValue
	// CashBalance is the caller-supplied ending cash balance for each
	// period, used only for the burn/runway calculation (see
	// CashRunway). Optional; if absent, CashRunway.MonthsOfRunway is
	// unavailable even when burn rate itself is computable.
	CashBalance map[financial.Period]CashFlowValue

	// Policy configures which financial.Code values this package sums into
	// the working-capital component of the bridge. If the zero value,
	// analytics/workingcapital.Calculate itself substitutes its own
	// DefaultInclusionPolicy() (see that package's resolveInclusionPolicy)
	// — this package reuses analytics/workingcapital.InclusionPolicy
	// directly rather than inventing a second, subtly different
	// working-capital policy type, since "which balance-sheet codes count
	// as operating working capital" is exactly the question that package
	// already answers.
	Policy workingcapital.InclusionPolicy
}

// Options controls Calculate's optional behavior. The zero Options is
// valid: no EBITDA-based estimation is performed, and DefaultThresholds is
// used for conversion-ratio flags.
type Options struct {
	// AllowEBITDAEstimate opts into this package estimating operating cash
	// flow (EBITDA - change in net working capital) for a period whose
	// Input.OperatingCashFlow entry is missing or unavailable, and
	// transitively estimating FreeCashFlow (estimated OCF - Capex, when
	// Capex is available) for that period. Off by default: this package
	// never infers a cash-flow figure from EBITDA unless a caller
	// explicitly asks for it, per the package doc comment. Every estimated
	// figure carries CashFlowValue.IsEstimate == true and a populated
	// EstimateBasis, and Result.Warnings notes every period an estimate was
	// used for.
	AllowEBITDAEstimate bool
	// Thresholds configures every deterministic flag trigger point (see
	// Thresholds). If the zero value, DefaultThresholds() is used —
	// mirroring qoe.Options.Thresholds/review.Policy's identical
	// zero-value-means-defaults convention.
	Thresholds Thresholds
}

// Thresholds configures the trigger points for Result.Flags. All fields are
// decimals (a ratio of 0.5 means 50%) unless noted otherwise.
type Thresholds struct {
	// WeakConversionRatio is the EBITDAToOperatingCashFlow (or
	// EBITDAToFreeCashFlow) ratio at or below which FlagWeakCashConversion
	// triggers — conversion meaningfully below 1.0 (EBITDA not turning into
	// cash).
	WeakConversionRatio float64
	// HighCapexBurdenRatio is the Capex-to-EBITDA ratio at or above which
	// FlagHighCapexBurden triggers.
	HighCapexBurdenRatio float64
	// HighWorkingCapitalBurdenRatio is the |change in NWC|-to-EBITDA ratio
	// at or above which FlagHighWorkingCapitalBurden triggers.
	HighWorkingCapitalBurdenRatio float64
	// LowDebtServiceCoverageRatio is the DSCR (operating cash flow /
	// total debt service) at or below which FlagLowDebtServiceCoverage
	// triggers. A DSCR of 1.0 means cash from operations exactly covers
	// debt service with nothing left over, so this is typically set above
	// 1.0 (e.g. 1.25, a common lender covenant floor).
	LowDebtServiceCoverageRatio float64
	// UncoveredDistributionsRatio is the OwnerDistributions-to-free-cash-
	// flow ratio at or above which FlagDistributionsExceedFreeCashFlow
	// triggers (distributions consuming most or all of free cash flow,
	// leaving little cushion).
	UncoveredDistributionsRatio float64
	// LowRunwayMonths is the number of months of CashRunway at or below
	// which FlagLowCashRunway triggers.
	LowRunwayMonths float64
}

// DefaultThresholds returns this package's baseline SMB-advisory trigger
// points. A caller with different risk tolerance (e.g. a lender's own DSCR
// covenant) supplies its own Thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		WeakConversionRatio:           0.6,
		HighCapexBurdenRatio:          0.25,
		HighWorkingCapitalBurdenRatio: 0.15,
		LowDebtServiceCoverageRatio:   1.25,
		UncoveredDistributionsRatio:   0.75,
		LowRunwayMonths:               6,
	}
}

// resolveThresholds returns t if it is non-zero, otherwise
// DefaultThresholds() — the same zero-value-means-defaults rule
// qoe.resolveThresholds/review.resolvePolicy use.
func resolveThresholds(t Thresholds) Thresholds {
	if t == (Thresholds{}) {
		return DefaultThresholds()
	}
	return t
}

// Bridge is one period's full walk from EBITDA to free cash flow, plus
// every intermediate line — the central per-period building block this
// package produces.
type Bridge struct {
	// Period is the reporting period this bridge applies to.
	Period financial.Period `json:"period"`

	// EBITDA is financial/metrics.Snapshot.EBITDA for this period,
	// recomputed by this package directly from Input.Dataset (see the
	// package doc comment).
	EBITDA metrics.MetricValue `json:"ebitda"`

	// ChangeInNWC is this period's operating-net-working-capital minus the
	// immediately preceding chronological period's, under Input.Policy —
	// an increase in NWC is a cash use (reported here as a positive
	// number) and a decrease is a cash source (negative). Available only
	// when both this period's and the preceding period's NWC are
	// available; unavailable for the earliest period in the series
	// (nothing precedes it) or when the resolved chronological order is
	// unknown (see Input.PeriodMeta).
	ChangeInNWC metrics.MetricValue `json:"change_in_nwc"`

	// OperatingCashFlow is Input.OperatingCashFlow for this period when
	// supplied and available, otherwise the EBITDA-based estimate (EBITDA
	// - ChangeInNWC) when Options.AllowEBITDAEstimate is set and both
	// EBITDA and ChangeInNWC are available, otherwise Unavailable().
	OperatingCashFlow CashFlowValue `json:"operating_cash_flow"`
	// Capex is Input.Capex for this period, echoed verbatim.
	Capex CashFlowValue `json:"capex"`
	// FreeCashFlow is OperatingCashFlow - Capex.Value (Capex treated as an
	// outflow magnitude). Available only when OperatingCashFlow is
	// available; if Capex is unavailable, FreeCashFlow equals
	// OperatingCashFlow verbatim and IsEstimate/EstimateBasis note that
	// capex was assumed zero because it was not supplied — this package
	// never silently drops the capex line without saying so.
	FreeCashFlow CashFlowValue `json:"free_cash_flow"`

	// DebtService is Input.DebtService for this period, echoed verbatim.
	DebtService DebtServiceFigure `json:"debt_service"`
	// FreeCashFlowToFirm is OperatingCashFlow - Capex.Value +
	// DebtService.Interest.Value (adding back after-financing-cost cash
	// flow to reach an all-capital-providers figure) — available only when
	// FreeCashFlow and DebtService.Interest are both available. See the
	// package README section for the exact formula and why interest
	// (rather than a tax-affected interest figure) is used: this package
	// has no tax-rate input to tax-affect interest with, so it reports the
	// pre-tax-shield approximation common in SMB/lower-middle-market
	// advisory work rather than fabricating a tax rate.
	FreeCashFlowToFirm metrics.MetricValue `json:"free_cash_flow_to_firm"`
	// FreeCashFlowToOwner is FreeCashFlow - DebtService.Total().Value (cash
	// remaining after both capex and debt service, the figure an owner can
	// actually draw without impairing the business or defaulting on debt)
	// — available only when FreeCashFlow and DebtService.Total() are both
	// available.
	FreeCashFlowToOwner metrics.MetricValue `json:"free_cash_flow_to_owner"`

	// OwnerDistributions is Input.OwnerDistributions for this period,
	// echoed verbatim.
	OwnerDistributions CashFlowValue `json:"owner_distributions"`
	// CashTaxesPaid is Input.CashTaxesPaid for this period, echoed
	// verbatim.
	CashTaxesPaid CashFlowValue `json:"cash_taxes_paid"`
}

// ConversionRatios holds one period's EBITDA-to-cash conversion figures.
type ConversionRatios struct {
	// Period is the period these ratios were computed for.
	Period financial.Period `json:"period"`
	// EBITDAToOperatingCashFlow is OperatingCashFlow.Value / EBITDA.Value —
	// how much of reported/estimated EBITDA actually became operating cash.
	// Available only if both are available and EBITDA.Value != 0.
	EBITDAToOperatingCashFlow metrics.MetricValue `json:"ebitda_to_operating_cash_flow"`
	// EBITDAToFreeCashFlow is FreeCashFlow.Value / EBITDA.Value. Available
	// only if both are available and EBITDA.Value != 0.
	EBITDAToFreeCashFlow metrics.MetricValue `json:"ebitda_to_free_cash_flow"`
}

// TrendDirection is a coarse, deterministic characterization of a series'
// overall direction, computed from its first vs. last available
// observation — never inferred from a Message string. Mirrors
// workingcapital.TrendDirection/ratios.RatioTrendDirection exactly.
type TrendDirection string

const (
	TrendIncreasing  TrendDirection = "increasing"
	TrendDeclining   TrendDirection = "declining"
	TrendStable      TrendDirection = "stable"
	TrendUnavailable TrendDirection = "unavailable"
)

// TrendFlatBandPercent is the |change| / |first value| band, as a decimal,
// within which a series is characterized TrendStable rather than
// TrendIncreasing/TrendDeclining. Fixed (not caller-configurable) because it
// is part of this package's versioned FormulaVersion — the same fixed ±5%
// band analytics/workingcapital.TrendFlatBandPercent and
// analytics/ratios.TrendFlatBandPercent use.
const TrendFlatBandPercent = 0.05

// Trend is a first-vs-last-observation direction characterization of a
// series (see TrendDirection), plus the underlying comparison values for
// display.
type Trend struct {
	Direction   TrendDirection      `json:"direction"`
	FirstPeriod financial.Period    `json:"first_period,omitempty"`
	LastPeriod  financial.Period    `json:"last_period,omitempty"`
	FirstValue  metrics.MetricValue `json:"first_value"`
	LastValue   metrics.MetricValue `json:"last_value"`
	// PercentChange is (LastValue - FirstValue) / |FirstValue|. Available
	// only if both values are available and FirstValue.Value != 0.
	PercentChange metrics.MetricValue `json:"percent_change"`
}

// RecurringDrain is one recurring cash outflow category, aggregated across
// History, that structurally reduces cash available to an owner even when
// EBITDA looks healthy — the specific items this package's task requires
// surfacing as "recurring cash drains" distinct from the raw Bridge lines.
type RecurringDrain struct {
	// Category names the drain (see the RecurringDrainCategory constants).
	Category RecurringDrainCategory `json:"category"`
	// TotalAmount is the sum of this category's per-period magnitude across
	// every period in History with an available figure.
	TotalAmount float64 `json:"total_amount"`
	// PeriodsPresent is how many periods in History had an available,
	// nonzero figure for this category.
	PeriodsPresent int `json:"periods_present"`
	// AverageOfEBITDA is TotalAmount / (sum of EBITDA across the same
	// periods), when that sum is available and nonzero — how large this
	// drain is relative to the earnings it draws down.
	AverageOfEBITDA metrics.MetricValue `json:"average_of_ebitda"`
}

// RecurringDrainCategory identifies one kind of recurring cash outflow.
// Plain string (mirroring financial.Code/adjustments.Type) so new
// categories can be added later without breaking existing callers.
type RecurringDrainCategory string

const (
	RecurringDrainCapex              RecurringDrainCategory = "capex"
	RecurringDrainWorkingCapital     RecurringDrainCategory = "working_capital_build"
	RecurringDrainDebtService        RecurringDrainCategory = "debt_service"
	RecurringDrainOwnerDistributions RecurringDrainCategory = "owner_distributions"
)

// CashRunway estimates, for a loss-making or cash-burning business, how
// many months of operation remain at the current burn rate — computed only
// when enough input exists (see Available).
type CashRunway struct {
	// Available is true only if MonthlyBurnRate is available and negative
	// (i.e. the business is actually burning cash) — a profitable or
	// break-even business has no runway to report, so this is left
	// unavailable rather than reporting an infinite or negative-of-negative
	// number.
	Available bool `json:"available"`
	// MonthlyBurnRate is the average monthly OperatingCashFlow across
	// History's available observations (negative means burning cash,
	// positive means generating cash). Uses OperatingCashFlow (not free
	// cash flow) since runway is conventionally about operating burn;
	// capex-heavy one-time spend is visible separately via
	// Result.RecurringDrains' RecurringDrainCapex entry.
	MonthlyBurnRate metrics.MetricValue `json:"monthly_burn_rate"`
	// CurrentCashBalance is the most recent chronologically available
	// Input.CashBalance figure.
	CurrentCashBalance metrics.MetricValue `json:"current_cash_balance"`
	// MonthsOfRunway is CurrentCashBalance.Value /
	// |MonthlyBurnRate.Value|, available only when both
	// CurrentCashBalance and a negative MonthlyBurnRate are available.
	MonthsOfRunway metrics.MetricValue `json:"months_of_runway"`
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model
// adjustments.IssueSeverity/review.IssueSeverity/qoe.IssueSeverity/
// workingcapital.IssueSeverity use.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository, rather than reusing one of
// theirs — a cash-flow input problem is a distinct problem domain.
type IssueCode string

const (
	// IssueNoPeriods means Input.Dataset has no periods at all; Calculate
	// returns Available == false.
	IssueNoPeriods IssueCode = "NO_PERIODS"
	// IssueNoPeriodMeta means Input.PeriodMeta was nil or empty, so
	// chronological ordering, ChangeInNWC, Trend, and CashRunway could not
	// be computed. Advisory only: per-period Bridge figures that don't
	// depend on ordering are still fully computed in Dataset.Periods()
	// lexical order.
	IssueNoPeriodMeta IssueCode = "NO_PERIOD_META"
	// IssuePeriodMissingFromMeta means at least one period present in
	// Dataset has no PeriodMeta entry, so History falls back to lexical
	// order and ordering-dependent outputs are unavailable — mirroring
	// workingcapital.IssuePeriodMissingFromMeta's identical rule.
	IssuePeriodMissingFromMeta IssueCode = "PERIOD_MISSING_FROM_META"
	// IssueNoCashFlowStatement means Input.OperatingCashFlow had no
	// available entry for any period, so every Bridge.OperatingCashFlow
	// (and everything downstream of it) is unavailable unless
	// Options.AllowEBITDAEstimate is set. Advisory only.
	IssueNoCashFlowStatement IssueCode = "NO_CASH_FLOW_STATEMENT"
	// IssueEstimatedFromEBITDA means at least one period's
	// OperatingCashFlow (and/or FreeCashFlow) was filled in from EBITDA
	// under Options.AllowEBITDAEstimate rather than reported — see
	// Bridge.OperatingCashFlow.IsEstimate for exactly which periods.
	// Advisory only; this Issue exists so a caller scanning Warnings alone
	// (without inspecting every Bridge) still sees that estimation
	// occurred somewhere in the result.
	IssueEstimatedFromEBITDA IssueCode = "ESTIMATED_FROM_EBITDA"
)

// Issue is a single Calculate-time input finding, mirroring
// adjustments.Issue/review.Issue/qoe.Issue/workingcapital.Issue's shape.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from adjustments.HasErrors/valuation.HasErrors/
// review.HasErrors/qoe.HasErrors/workingcapital.HasErrors rather than
// shared — see adjustments.HasErrors's doc comment for the full rationale
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

// FlagCode is a stable identifier for one kind of deterministic cash-flow
// signal, analogous to qoe.FlagCode/ratios.SignalCode.
type FlagCode string

const (
	// FlagWeakCashConversion means the most recent period's
	// EBITDAToOperatingCashFlow or EBITDAToFreeCashFlow is at or below
	// Thresholds.WeakConversionRatio.
	FlagWeakCashConversion FlagCode = "WEAK_CASH_CONVERSION"
	// FlagHighCapexBurden means the most recent period's Capex-to-EBITDA
	// ratio is at or above Thresholds.HighCapexBurdenRatio.
	FlagHighCapexBurden FlagCode = "HIGH_CAPEX_BURDEN"
	// FlagHighWorkingCapitalBurden means the most recent period's
	// |ChangeInNWC|-to-EBITDA ratio is at or above
	// Thresholds.HighWorkingCapitalBurdenRatio.
	FlagHighWorkingCapitalBurden FlagCode = "HIGH_WORKING_CAPITAL_BURDEN"
	// FlagLowDebtServiceCoverage means the most recent period's DSCR is at
	// or below Thresholds.LowDebtServiceCoverageRatio.
	FlagLowDebtServiceCoverage FlagCode = "LOW_DEBT_SERVICE_COVERAGE"
	// FlagDistributionsExceedFreeCashFlow means the most recent period's
	// OwnerDistributions-to-FreeCashFlow ratio is at or above
	// Thresholds.UncoveredDistributionsRatio.
	FlagDistributionsExceedFreeCashFlow FlagCode = "DISTRIBUTIONS_EXCEED_FREE_CASH_FLOW"
	// FlagLowCashRunway means CashRunway.MonthsOfRunway is available and at
	// or below Thresholds.LowRunwayMonths.
	FlagLowCashRunway FlagCode = "LOW_CASH_RUNWAY"
	// FlagDecliningConversionTrend means ConversionTrend's direction is
	// TrendDeclining.
	FlagDecliningConversionTrend FlagCode = "DECLINING_CONVERSION_TREND"
)

// FlagSeverity mirrors qoe.FlagSeverity/ratios.SignalSeverity's role: a
// structured, matchable urgency signal, never inferred from Message text.
type FlagSeverity string

const (
	FlagSeverityInfo     FlagSeverity = "info"
	FlagSeverityWarning  FlagSeverity = "warning"
	FlagSeverityCritical FlagSeverity = "critical"
)

// Flag is a single deterministic, explainable cash-flow signal. Every Flag
// here is rule-based against Thresholds, never AI-scored — mirroring
// qoe.Flag's identical design.
type Flag struct {
	Code      FlagCode         `json:"code"`
	Severity  FlagSeverity     `json:"severity"`
	Period    financial.Period `json:"period,omitempty"`
	Message   string           `json:"message"`
	Value     float64          `json:"value"`
	Threshold float64          `json:"threshold"`
}

// Result is the output of Calculate: the full per-period cash-flow bridge,
// conversion ratios, trend, recurring drains, coverage burdens, cash
// runway, deterministic flags, and formula version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoPeriods) — every other field is then zero-value.
	Available bool `json:"available"`

	// History is one Bridge per period in Input.Dataset, ordered
	// chronologically when Input.PeriodMeta was supplied and covers every
	// period; otherwise in Dataset.Periods()'s lexical order.
	History []Bridge `json:"history,omitempty"`
	// Conversion is one ConversionRatios per period in History, in the same
	// order.
	Conversion []ConversionRatios `json:"conversion,omitempty"`

	// OperatingCashFlowTrend characterizes History's OperatingCashFlow
	// series first-vs-last.
	OperatingCashFlowTrend Trend `json:"operating_cash_flow_trend"`
	// FreeCashFlowTrend characterizes History's FreeCashFlow series
	// first-vs-last.
	FreeCashFlowTrend Trend `json:"free_cash_flow_trend"`
	// ConversionTrend characterizes Conversion's EBITDAToFreeCashFlow
	// series first-vs-last — the trend FlagDecliningConversionTrend reads.
	ConversionTrend Trend `json:"conversion_trend"`

	// RecurringDrains breaks down capex, working-capital build, debt
	// service, and owner distributions as recurring cash drains across
	// History, sorted by RecurringDrainCategory's declaration order
	// (capex, working_capital_build, debt_service, owner_distributions) —
	// never Go map order.
	RecurringDrains []RecurringDrain `json:"recurring_drains,omitempty"`

	// CashRunway is the burn/runway estimate for a loss-making business.
	// See CashRunway.Available for when this is meaningful.
	CashRunway CashRunway `json:"cash_runway"`

	// Flags is every deterministic signal Calculate triggered, ordered by
	// FlagCode's declaration order, then by Period.
	Flags []Flag `json:"flags,omitempty"`

	// Thresholds echoes the resolved Options.Thresholds (after
	// DefaultThresholds substitution) this Result was computed under.
	Thresholds Thresholds `json:"thresholds"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
