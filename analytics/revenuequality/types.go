// Package revenuequality evaluates the quality, stability, and composition
// of revenue from a normalized financial.FinancialDataset, optionally
// enriched with customer-level revenue detail, without requiring any
// app/database concept (no customer accounts, no subscription/billing
// records, no persisted history of any kind — see the top-level README's
// architectural rules).
//
// This repository's canonical financial.Code taxonomy carries revenue at
// only four codes (CodeRevProduct, CodeRevService, CodeRevRecurring,
// CodeRevOther — see financial.LookupCode), each a flat, mutually exclusive
// classification with no finer recurring/non-recurring sub-split within a
// single line item. So this package's dataset-level recurring/non-recurring
// composition is exactly as fine-grained as the taxonomy allows:
// CodeRevRecurring is "recurring revenue," and CodeRevProduct +
// CodeRevService + CodeRevOther together are "non-recurring revenue" for
// the purposes of this package. A caller whose source system tracks
// recurring/non-recurring at a finer grain than this taxonomy permits
// (e.g. a specific contract is recurring but happens to be coded
// CodeRevOther upstream) needs to reclassify it onto CodeRevRecurring
// before this package can see it as recurring — this package never guesses
// a line item's recurring status from its label, code metadata, or amount.
//
// Customer-level analysis (retention, new/lost/expansion/contraction
// revenue, concentration) is entirely optional and activates only when a
// caller supplies Input.CustomerRevenue. This package defines its own
// portable CustomerPeriodRevenue input type — a bare (customer key, period,
// amount) tuple plus two optional hints (RecurringFlag, Segment) — rather
// than requiring any application's customer/account/subscription object.
// CustomerPeriodRevenue.CustomerKey is an opaque caller-assigned string;
// this package never requires, stores, or infers any customer PII (name,
// email, address) beyond whatever opaque key the caller chooses to use.
//
// This package deliberately does not claim SaaS-style Net/Gross Revenue
// Retention (NRR/GRR) terminology anywhere in its exported API. NRR/GRR
// carry specific, widely-understood contractual assumptions (a defined
// subscription book, cohort-level tracking, a consistent renewal cadence)
// that this package's caller-agnostic customer-period-revenue input cannot
// guarantee holds for every business this package might be asked to
// analyze (a project-based consultancy's "customer revenue" history looks
// nothing like a SaaS company's). Instead this package reports the
// underlying arithmetic directly — RetainedRevenue, NewCustomerRevenue,
// LostCustomerRevenue, ExpansionRevenue, ContractionRevenue — and leaves
// any SaaS-specific ratio a caller wants (e.g. NRR = (Retained + Expansion
// - Contraction) / PriorPeriodRevenue) as arithmetic the caller performs
// over these figures once it has independently confirmed its business
// matches that model's assumptions.
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package revenuequality

import "github.com/themurtez/go-valuate/financial"

// FormulaVersion identifies this package's fixed formula set: the
// recurring/non-recurring revenue split, the Statistics/Trend/CAGR/
// Volatility formulas, the customer-transition (new/lost/expansion/
// contraction) formulas, the ConcentrationSummary/HHI formula, and every
// flag-trigger rule in Thresholds. Bump this whenever any of that changes
// in a way that could make a historical Result not reproduce identically
// under new code — see the repository README's versioning-strategy
// section, which this constant follows exactly (financial.TaxonomyVersion,
// metrics.FormulaVersion, workingcapital.FormulaVersion, cashflow.FormulaVersion,
// qoe.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// PeriodType mirrors metrics.PeriodType/workingcapital.PeriodType (fiscal
// year, YTD, quarter, month), duplicated here rather than aliased so this
// package's own doc comments apply directly at the call site — the same
// choice analytics/workingcapital, analytics/cashflow, and analytics/ratios
// already made for their own PeriodType.
type PeriodType string

const (
	PeriodTypeFiscalYear PeriodType = "fiscal_year"
	PeriodTypeYTD        PeriodType = "ytd"
	PeriodTypeQuarter    PeriodType = "quarter"
	PeriodTypeMonth      PeriodType = "month"
)

// PeriodInfo is caller-supplied metadata about a single financial.Period,
// used to determine chronological order for Trend/CAGR/Volatility and for
// the adjacent-period customer transitions in CustomerTransitions.
// financial.Period is intentionally just a string with no guaranteed sort
// order (see financial.FinancialDataset.Periods' doc comment); this package
// requires PeriodInfo rather than inferring order or granularity from the
// string, mirroring financial/metrics', analytics/qoe's,
// analytics/workingcapital's, and analytics/cashflow's identical
// no-guessing rule.
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

// RevenueValue represents a single revenue-quality figure that may or may
// not be calculable, mirroring metrics.MetricValue/workingcapital.NWCValue/
// cashflow.CashFlowValue's identical availability convention: Available
// distinguishes "computed to be exactly $0" from "cannot be computed
// because a required input is absent."
type RevenueValue struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

// Unavailable is the canonical zero-information RevenueValue.
func Unavailable() RevenueValue { return RevenueValue{} }

// AvailableValue reports a RevenueValue for a successfully computed figure.
func AvailableValue(v float64) RevenueValue { return RevenueValue{Available: true, Value: v} }

// CustomerPeriodRevenue is one customer's revenue in one period — the
// portable customer-revenue input type this package requires for every
// customer-level output (see the package doc comment). A caller assembles
// this slice from whatever source system it has (a billing export, an
// invoicing system, a CRM) — this package has no opinion on where it came
// from.
type CustomerPeriodRevenue struct {
	// CustomerKey is an opaque, caller-assigned identifier for the
	// customer. Any stable string the caller chooses (an internal customer
	// ID, a hash) — this package never requires it to be a real name, email,
	// or other PII, and treats it purely as an equality-comparable key.
	CustomerKey string `json:"customer_key"`
	// Period is the reporting period this revenue was earned in. Must be a
	// period present in Input.Dataset.Periods() for this row to be included
	// in period-aligned outputs (ConcentrationSummary, CustomerTransitions);
	// a row for a period absent from Dataset is carried in
	// Result.Issues as IssueCustomerPeriodNotInDataset but does not error
	// the whole calculation.
	Period financial.Period `json:"period"`
	// Amount is this customer's revenue for Period, in the dataset's native
	// currency. Not required to reconcile exactly to Dataset's total revenue
	// for Period — see ConcentrationSummary.UnallocatedRevenue for how a
	// mismatch is surfaced rather than silently rescaled.
	Amount float64 `json:"amount"`
	// RecurringFlag, when non-nil, states whether this specific customer
	// row is recurring revenue. Optional: a caller without contract-level
	// recurring detail for its customer data leaves this nil, and every
	// output that would otherwise split customer revenue by recurring
	// status is simply left unavailable for that row rather than guessed.
	// A *bool (not a plain bool) so "unknown" is representable distinctly
	// from "known to be non-recurring."
	RecurringFlag *bool `json:"recurring_flag,omitempty"`
	// Segment, when non-empty, is a caller-assigned grouping label (e.g. a
	// product line, industry vertical, or sales channel) used only for
	// ConcentrationSummary's optional by-segment breakdown. Plain string
	// (mirroring financial.Code/adjustments.Type) so this package never
	// hard-codes a segment taxonomy.
	Segment string `json:"segment,omitempty"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// Dataset is the normalized financial history to analyze. Required;
	// Calculate returns a Result with Available == false and an Errors
	// entry if Dataset has no periods. Dataset's own CodeRevProduct/
	// CodeRevService/CodeRevRecurring/CodeRevOther items are this package's
	// sole source for TotalRevenueHistory and the recurring/non-recurring
	// split — see the package doc comment.
	Dataset financial.FinancialDataset
	// PeriodMeta supplies chronological ordering for Dataset's periods.
	// Required for RevenueTrend, RevenueCAGR, RevenueVolatility, and every
	// CustomerTransitions entry; if nil, or a period present in Dataset has
	// no entry, those specific outputs are left unavailable rather than the
	// whole Result failing — mirroring analytics/workingcapital.Input.
	// PeriodMeta's identical convention.
	PeriodMeta map[financial.Period]PeriodInfo

	// CustomerRevenue is the optional customer-level revenue detail. When
	// empty, every customer-level output (CustomerHistory, retention,
	// new/lost/expansion/contraction, ConcentrationSummary) is left at its
	// zero value with Available == false, and Result.Issues notes
	// IssueNoCustomerData — this package proceeds using only Dataset's
	// period-level totals for every other output.
	CustomerRevenue []CustomerPeriodRevenue

	// Policy configures ConcentrationSummary's top-N cutoffs and which
	// segments (if any) get a by-segment breakdown. If the zero value,
	// DefaultPolicy() is used.
	Policy Policy
}

// Policy configures Calculate's optional, caller-adjustable behavior that
// is not a fixed part of FormulaVersion (unlike Thresholds, which governs
// flag triggers — see Thresholds' own doc comment for why the two are kept
// separate).
type Policy struct {
	// ConcentrationTopN lists each customer-count cutoff
	// ConcentrationSummary.TopNShares reports a share for (e.g. []int{1, 5,
	// 10} for "top 1 customer," "top 5 customers," "top 10 customers"). If
	// empty, DefaultPolicy's []int{1, 5, 10} is used.
	ConcentrationTopN []int `json:"concentration_top_n"`
}

// DefaultPolicy returns this package's baseline concentration cutoffs: top
// 1, top 5, and top 10 customers, the cutoffs most commonly requested in
// SMB/lower-middle-market advisory and transaction diligence.
func DefaultPolicy() Policy {
	return Policy{ConcentrationTopN: []int{1, 5, 10}}
}

// resolvePolicy returns p if ConcentrationTopN is non-empty, otherwise
// DefaultPolicy() — the same zero-value-means-defaults rule
// workingcapital.resolveInclusionPolicy/qoe.resolveThresholds/
// cashflow.resolveThresholds use.
func resolvePolicy(p Policy) Policy {
	if len(p.ConcentrationTopN) == 0 {
		return DefaultPolicy()
	}
	return p
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model
// adjustments.IssueSeverity/review.IssueSeverity/qoe.IssueSeverity/
// workingcapital.IssueSeverity/cashflow.IssueSeverity use.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository, rather than reusing one of
// theirs — a revenue-quality input problem is a distinct problem domain.
type IssueCode string

const (
	// IssueNoPeriods means Input.Dataset has no periods at all; Calculate
	// returns Available == false.
	IssueNoPeriods IssueCode = "NO_PERIODS"
	// IssueNoPeriodMeta means Input.PeriodMeta was nil or empty, so
	// chronological ordering, RevenueTrend, RevenueCAGR,
	// RevenueVolatility, and CustomerTransitions could not be computed.
	// Advisory only: per-period TotalRevenueHistory figures are still fully
	// computed in Dataset.Periods() lexical order.
	IssueNoPeriodMeta IssueCode = "NO_PERIOD_META"
	// IssuePeriodMissingFromMeta means at least one period present in
	// Dataset has no PeriodMeta entry, so TotalRevenueHistory falls back to
	// lexical order and every ordering-dependent output is unavailable —
	// mirroring workingcapital.IssuePeriodMissingFromMeta's identical rule.
	IssuePeriodMissingFromMeta IssueCode = "PERIOD_MISSING_FROM_META"
	// IssueNoRevenueData means Input.Dataset has no revenue codes present
	// for any period, so TotalRevenueHistory is entirely unavailable.
	IssueNoRevenueData IssueCode = "NO_REVENUE_DATA"
	// IssueNoCustomerData means Input.CustomerRevenue was empty, so every
	// customer-level output is left unavailable. Advisory only.
	IssueNoCustomerData IssueCode = "NO_CUSTOMER_DATA"
	// IssueCustomerPeriodNotInDataset means at least one
	// CustomerPeriodRevenue.Period was not present in Dataset.Periods(); that
	// row is excluded from every period-aligned customer output
	// (ConcentrationSummary, CustomerTransitions) but does not error the
	// whole calculation.
	IssueCustomerPeriodNotInDataset IssueCode = "CUSTOMER_PERIOD_NOT_IN_DATASET"
	// IssueCustomerRevenueUnreconciled means, for at least one period, the
	// sum of Input.CustomerRevenue rows for that period differs from
	// Dataset's total revenue for that period by more than a trivial
	// rounding amount — see ConcentrationSummary.UnallocatedRevenue. This
	// package does not require customer-level detail to fully reconcile to
	// the dataset total (a caller's customer export may cover only a subset
	// of revenue, e.g. product revenue but not service revenue), so this is
	// advisory only.
	IssueCustomerRevenueUnreconciled IssueCode = "CUSTOMER_REVENUE_UNRECONCILED"
)

// Issue is a single Calculate-time input finding, mirroring
// adjustments.Issue/review.Issue/qoe.Issue/workingcapital.Issue/
// cashflow.Issue's shape.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from adjustments.HasErrors/valuation.HasErrors/
// review.HasErrors/qoe.HasErrors/workingcapital.HasErrors/cashflow.HasErrors
// rather than shared — see adjustments.HasErrors's doc comment for the full
// rationale (each package's Issue is a distinct Go type with no common
// interface worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// PeriodRevenue is one period's total, recurring, and non-recurring revenue
// composition, read directly from Input.Dataset — see the package doc
// comment for exactly which financial.Code values contribute to each.
type PeriodRevenue struct {
	// Period is the reporting period this figure applies to.
	Period financial.Period `json:"period"`
	// TotalRevenue is CodeRevProduct + CodeRevService + CodeRevRecurring +
	// CodeRevOther for this period, when at least one is present — the same
	// total financial/metrics.Snapshot.TotalRevenue computes (duplicated
	// here rather than depending on financial/metrics, exactly as
	// workingcapital.PeriodNWC.Revenue duplicates it, since this package
	// needs only the total plus its recurring/non-recurring split, not
	// metrics' full Snapshot machinery).
	TotalRevenue RevenueValue `json:"total_revenue"`
	// RecurringRevenue is CodeRevRecurring alone for this period. Available
	// only if CodeRevRecurring is present in Dataset for this period —
	// distinct from "recurring revenue is exactly $0," which is also
	// representable via RevenueValue.Available == true, Value == 0.
	RecurringRevenue RevenueValue `json:"recurring_revenue"`
	// NonRecurringRevenue is CodeRevProduct + CodeRevService + CodeRevOther
	// for this period. Available only if at least one of those three is
	// present in Dataset for this period.
	NonRecurringRevenue RevenueValue `json:"non_recurring_revenue"`
	// RecurringPercent is RecurringRevenue.Value / TotalRevenue.Value.
	// Available only if RecurringRevenue, NonRecurringRevenue, and
	// TotalRevenue are all available and TotalRevenue.Value != 0 — requiring
	// NonRecurringRevenue too (not just Recurring/Total) so this percentage
	// is never computed from a partial revenue picture (e.g. a dataset that
	// has CodeRevRecurring but is missing CodeRevProduct for a period where
	// product revenue was actually earned would otherwise silently overstate
	// RecurringPercent).
	RecurringPercent RevenueValue `json:"recurring_percent"`
	// NonRecurringPercent is NonRecurringRevenue.Value / TotalRevenue.Value,
	// under the same availability rule as RecurringPercent.
	NonRecurringPercent RevenueValue `json:"non_recurring_percent"`
	// Components lists each of the four revenue codes present in Dataset for
	// this period, and its amount, for auditability.
	Components []Component `json:"components,omitempty"`
}

// Component is one named financial.Code that contributed to a computed
// revenue figure, mirroring metrics.Component/workingcapital.Component's
// role: carried so callers can explain how a figure was produced without
// this package implementing a generic expression engine.
type Component struct {
	// Code identifies the canonical financial.Code this component came
	// from.
	Code string `json:"code"`
	// Label is a short human-readable description of the component.
	Label string `json:"label,omitempty"`
	// Amount is the value contributed by this component.
	Amount float64 `json:"amount"`
}

// Statistics summarizes a series of RevenueValue observations across
// TotalRevenueHistory: central tendency, dispersion, and range. Mirrors
// workingcapital.Statistics's identical shape and method exactly.
type Statistics struct {
	// Average is the unweighted arithmetic mean across every available
	// observation.
	Average RevenueValue `json:"average"`
	// Median is the median across every available observation.
	Median RevenueValue `json:"median"`
	// Min is the smallest available observation.
	Min RevenueValue `json:"min"`
	// Max is the largest available observation.
	Max RevenueValue `json:"max"`
	// SampleSize is the number of available observations Average/Median/
	// Min/Max were computed from.
	SampleSize int `json:"sample_size"`
}

// TrendDirection is a coarse, deterministic characterization of a series'
// overall direction, computed from its first vs. last available
// observation — never inferred from a Message string. Mirrors
// workingcapital.TrendDirection/cashflow.TrendDirection exactly.
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
// band analytics/workingcapital.TrendFlatBandPercent,
// analytics/cashflow.TrendFlatBandPercent, and
// analytics/ratios.TrendFlatBandPercent use.
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
	FirstValue RevenueValue `json:"first_value"`
	LastValue  RevenueValue `json:"last_value"`
	// PercentChange is (LastValue - FirstValue) / |FirstValue|. Available
	// only if both values are available and FirstValue.Value != 0.
	PercentChange RevenueValue `json:"percent_change"`
}

// GrowthPoint is one chronologically-adjacent period-over-period growth
// observation, mirroring metrics.GrowthPoint/ratios.GrowthPoint's shape.
type GrowthPoint struct {
	FromPeriod financial.Period `json:"from_period"`
	ToPeriod   financial.Period `json:"to_period"`
	// Growth is (ToValue - FromValue) / |FromValue|. Available only if both
	// values are available and FromValue != 0.
	Growth RevenueValue `json:"growth"`
	// GrowthFromZeroBase is true when FromValue is available and exactly 0
	// (division by zero would otherwise occur) — Growth is left Unavailable
	// in that case rather than reporting an infinite or undefined
	// percentage, mirroring metrics.GrowthPoint.GrowthFromZeroBase exactly.
	GrowthFromZeroBase bool `json:"growth_from_zero_base,omitempty"`
}

// CAGRResult is a compound annual growth rate computed over
// TotalRevenueHistory's full available span, mirroring
// metrics.CAGRResult's shape and validity rules — duplicated here (rather
// than depending on financial/metrics) because metrics' own calculateCAGR
// is unexported and restricted to fiscal-year-to-fiscal-year comparisons
// only (see metrics.Trend's doc comment on that MVP restriction), while
// this package computes CAGR across whatever chronological order
// Input.PeriodMeta establishes — the same reasoning
// analytics/ratios.calculateGrowthSeries's doc comment gives for
// duplicating metrics.calculateYoYGrowth rather than being restricted by
// it.
type CAGRResult struct {
	// Value is (LastValue / FirstValue) ^ (1 / Years) - 1. Available only if
	// Invalid is empty and both endpoints are available.
	Value RevenueValue `json:"value"`
	// Years is the number of years the CAGR spans, derived from
	// FirstPeriod's and LastPeriod's PeriodInfo.FiscalYear.
	Years int `json:"years,omitempty"`
	// FirstPeriod/LastPeriod are the chronologically first/last periods with
	// an available TotalRevenue observation.
	FirstPeriod financial.Period `json:"first_period,omitempty"`
	LastPeriod  financial.Period `json:"last_period,omitempty"`
	// Invalid, when non-empty, is a short fixed reason CAGR could not be
	// computed even though endpoint values were available (e.g. a
	// non-positive starting value, or a zero year span) — mirroring
	// metrics.CAGRResult.Invalid exactly. A caller checks this field, never
	// Value.Available alone, to distinguish "no data" from "data present but
	// CAGR mathematically undefined for it."
	Invalid string `json:"invalid,omitempty"`
}

// VolatilityResult is the sample standard deviation of TotalRevenueHistory's
// period-over-period growth-rate series, mirroring metrics.VolatilityResult/
// workingcapital.Statistics.Volatility's identical method (see
// calculateVolatility in stats.go for the full rationale: this measures how
// much the period-over-period change itself swings, which is more
// meaningful for stability assessment than the standard deviation of raw
// dollar levels).
type VolatilityResult struct {
	// Value is the sample standard deviation, expressed as a decimal.
	// Requires at least 3 chronologically ordered observations (2 growth-rate
	// points) with no zero-base transition.
	Value RevenueValue `json:"value"`
	// SampleSize is the number of period-over-period growth observations
	// Value was computed from.
	SampleSize int `json:"sample_size"`
}

// CustomerPeriodTotal is one period's aggregated customer-level revenue
// picture: how many distinct customers contributed, their combined total,
// and (when RecurringFlag detail is present on the underlying
// CustomerPeriodRevenue rows) the recurring/non-recurring split of that
// customer total.
type CustomerPeriodTotal struct {
	// Period is the reporting period this figure applies to.
	Period financial.Period `json:"period"`
	// CustomerCount is the number of distinct CustomerKey values with at
	// least one CustomerPeriodRevenue row for this period.
	CustomerCount int `json:"customer_count"`
	// TotalCustomerRevenue is the sum of every CustomerPeriodRevenue.Amount
	// for this period.
	TotalCustomerRevenue RevenueValue `json:"total_customer_revenue"`
	// RecurringCustomerRevenue is the sum of every CustomerPeriodRevenue.Amount
	// for this period whose RecurringFlag is non-nil and true. Available
	// only if at least one row for this period has RecurringFlag set
	// (either true or false) — see RecurringFlagCoverage.
	RecurringCustomerRevenue RevenueValue `json:"recurring_customer_revenue"`
	// RecurringFlagCoverage is the fraction of TotalCustomerRevenue (by
	// dollar amount, not row count) contributed by rows with a non-nil
	// RecurringFlag — how much of this period's customer revenue has a known
	// recurring status. 0 when no row has RecurringFlag set.
	RecurringFlagCoverage float64 `json:"recurring_flag_coverage"`
}

// CustomerTransition classifies every customer active in either of two
// chronologically adjacent periods (From, To) into new, lost, retained, or
// changed, and sums each category's revenue impact. Computed once per
// adjacent pair in CustomerHistory's chronological order (see the package
// doc comment on why this package compares only chronologically adjacent
// periods, not arbitrary caller-specified pairs).
//
// This package reports the underlying dollar movements directly rather than
// a SaaS-style NRR/GRR ratio — see the package doc comment for why.
type CustomerTransition struct {
	// FromPeriod/ToPeriod are the two chronologically adjacent periods this
	// transition compares.
	FromPeriod financial.Period `json:"from_period"`
	ToPeriod   financial.Period `json:"to_period"`

	// FromPeriodTotalRevenue/ToPeriodTotalRevenue are the sum of every
	// customer's revenue in FromPeriod/ToPeriod respectively (i.e.
	// customerTotalsByKey's own grand total for each period, computed once
	// here rather than left for a caller — or a sibling function in this
	// package — to reconstruct from the categorized fields below. Every
	// ratio this package computes against "FromPeriod's total customer
	// revenue" (see Thresholds.HighLostCustomerRevenueRatio/
	// ShrinkingExistingBaseRatio) divides by this field directly, never by
	// an expression like RetainedRevenue + ContractionRevenue -
	// ExpansionRevenue, which is NOT algebraically equal to FromPeriod's
	// total whenever any retained customer expanded (RetainedRevenue is
	// already min(from,to) per customer, so ExpansionRevenue is additional
	// revenue on top of it, not a component to subtract back out).
	FromPeriodTotalRevenue RevenueValue `json:"from_period_total_revenue"`
	ToPeriodTotalRevenue   RevenueValue `json:"to_period_total_revenue"`
	// TotalRevenueGrowth is ToPeriodTotalRevenue - FromPeriodTotalRevenue —
	// this transition's own implied total-customer-revenue growth, stored
	// here (rather than recomputed at each call site that needs it, e.g.
	// FlagGrowthDependentOnNewCustomers) so every consumer reads the same
	// single derivation. Available only when both totals are available.
	TotalRevenueGrowth RevenueValue `json:"total_revenue_growth"`

	// NewCustomerRevenue is the sum of ToPeriod revenue from customers with
	// no revenue in FromPeriod (present in the customer data at all, with a
	// zero or absent row for FromPeriod).
	NewCustomerRevenue RevenueValue `json:"new_customer_revenue"`
	// NewCustomerCount is the number of distinct customers contributing to
	// NewCustomerRevenue.
	NewCustomerCount int `json:"new_customer_count"`

	// LostCustomerRevenue is the sum of FromPeriod revenue from customers
	// with no revenue in ToPeriod, reported as a positive magnitude (the
	// amount of revenue lost, not a negative delta).
	LostCustomerRevenue RevenueValue `json:"lost_customer_revenue"`
	// LostCustomerCount is the number of distinct customers contributing to
	// LostCustomerRevenue.
	LostCustomerCount int `json:"lost_customer_count"`

	// RetainedRevenue is, for customers present in both FromPeriod and
	// ToPeriod, the smaller of the two periods' revenue for that customer
	// summed across every such customer — the portion of FromPeriod revenue
	// that persisted into ToPeriod regardless of whether that customer's
	// spend also grew or shrank. ExpansionRevenue and ContractionRevenue
	// report the growth/shrink portions separately, so
	// RetainedRevenue + ExpansionRevenue - ContractionRevenue reconstructs
	// each retained customer's full ToPeriod revenue.
	RetainedRevenue RevenueValue `json:"retained_revenue"`
	// RetainedCustomerCount is the number of distinct customers present in
	// both FromPeriod and ToPeriod.
	RetainedCustomerCount int `json:"retained_customer_count"`

	// ExpansionRevenue is, for customers present in both periods whose
	// ToPeriod revenue exceeds their FromPeriod revenue, the sum of
	// (ToPeriod - FromPeriod) across those customers.
	ExpansionRevenue RevenueValue `json:"expansion_revenue"`
	// ExpandedCustomerCount is the number of distinct customers contributing
	// to ExpansionRevenue.
	ExpandedCustomerCount int `json:"expanded_customer_count"`

	// ContractionRevenue is, for customers present in both periods whose
	// ToPeriod revenue is less than their FromPeriod revenue (but still
	// greater than zero — a drop to zero is LostCustomerRevenue, not
	// contraction), the sum of (FromPeriod - ToPeriod) across those
	// customers, reported as a positive magnitude.
	ContractionRevenue RevenueValue `json:"contraction_revenue"`
	// ContractedCustomerCount is the number of distinct customers
	// contributing to ContractionRevenue.
	ContractedCustomerCount int `json:"contracted_customer_count"`

	// ExistingCustomerBaseChange is
	// RetainedRevenue + ExpansionRevenue - ContractionRevenue - FromPeriod's
	// total revenue from customers retained or contracted or lost (i.e. the
	// full FromPeriod customer base) — the net change in revenue from
	// customers who were already present in FromPeriod, excluding
	// NewCustomerRevenue entirely. Negative means the existing customer base
	// shrank even before counting new-customer growth. Available only when
	// RetainedRevenue, ExpansionRevenue, ContractionRevenue, and
	// LostCustomerRevenue are all available.
	ExistingCustomerBaseChange RevenueValue `json:"existing_customer_base_change"`
}

// TopNShare is the share of a period's total customer revenue contributed
// by its top N customers by revenue.
type TopNShare struct {
	// N is the customer-count cutoff (see Policy.ConcentrationTopN).
	N int `json:"n"`
	// Revenue is the summed revenue of the top N customers (or every
	// customer, if fewer than N were present).
	Revenue RevenueValue `json:"revenue"`
	// Percent is Revenue.Value / period total customer revenue. Available
	// only if both are available and the total is nonzero.
	Percent RevenueValue `json:"percent"`
}

// SegmentShare is one Segment's share of a period's total customer revenue,
// populated only for rows in Input.CustomerRevenue that supplied a
// non-empty Segment.
type SegmentShare struct {
	Segment string       `json:"segment"`
	Revenue RevenueValue `json:"revenue"`
	// Percent is Revenue.Value / period total customer revenue (across every
	// customer, not just segmented ones). Available only if both are
	// available and the total is nonzero.
	Percent RevenueValue `json:"percent"`
}

// ConcentrationSummary reports how concentrated a period's customer revenue
// is among its largest customers (and, optionally, its Segments), computed
// once for the most recent period present in Input.CustomerRevenue
// (chronologically last, when PeriodMeta covers it; otherwise the last
// period in Dataset.Periods() lexical order that has customer data) —
// mirroring cashflow.buildFlags/qoe.buildFlags's identical "read the most
// recent period" convention, since concentration risk is a
// point-in-time diligence question, not a trend.
type ConcentrationSummary struct {
	// Period is the period this summary was computed for.
	Period financial.Period `json:"period"`
	// CustomerCount is the number of distinct customers with revenue in
	// Period.
	CustomerCount int `json:"customer_count"`
	// TotalCustomerRevenue is the sum of every customer's revenue in Period.
	TotalCustomerRevenue RevenueValue `json:"total_customer_revenue"`
	// UnallocatedRevenue is Dataset's TotalRevenue for Period minus
	// TotalCustomerRevenue, when both are available — the portion of this
	// period's total revenue not represented in the customer-level detail
	// (e.g. because the caller's customer export covers only a subset of
	// revenue streams). Not an error; see IssueCustomerRevenueUnreconciled.
	// Can be negative if customer-level detail exceeds Dataset's total
	// (e.g. gross vs. net revenue conventions differ between the two
	// sources) — this package reports the signed difference without
	// guessing which source is correct.
	UnallocatedRevenue RevenueValue `json:"unallocated_revenue"`
	// TopNShares is one TopNShare per Policy.ConcentrationTopN cutoff,
	// sorted ascending by N.
	TopNShares []TopNShare `json:"top_n_shares,omitempty"`
	// HHI is the Herfindahl-Hirschman Index of customer revenue shares for
	// Period: the sum of each customer's (revenue share)^2, expressed on the
	// conventional 0-10,000 scale (a share of 1.0 contributes 10,000; a
	// share of 0.1 contributes 100). Higher means more concentrated among
	// fewer customers. Available only when CustomerCount > 0 and
	// TotalCustomerRevenue is available and nonzero.
	HHI RevenueValue `json:"hhi"`
	// Segments is one SegmentShare per distinct non-empty Segment value
	// present in Period's customer rows, sorted by Segment ascending. Empty
	// if no row for Period supplied a Segment.
	Segments []SegmentShare `json:"segments,omitempty"`
}

// Thresholds configures the trigger points for Result.Flags. All ratio
// fields are decimals (a ratio of 0.5 means 50%) unless noted otherwise.
// Kept separate from Policy (which configures ConcentrationSummary's
// reporting cutoffs): Policy changes what is reported, Thresholds changes
// only whether an already-computed figure crosses a caller-adjustable line
// into a Flag — mirroring cashflow's identical Options.Thresholds vs.
// Input.Policy separation.
type Thresholds struct {
	// DecliningRecurringMixPoints is the number of percentage points (raw
	// decimal, e.g. 0.05 for 5 points) PeriodRevenue.RecurringPercent is
	// allowed to fall from TotalRevenueHistory's first-vs-last available
	// observation before FlagDecliningRecurringMix triggers.
	DecliningRecurringMixPoints float64
	// NewCustomerGrowthDependenceRatio is the NewCustomerRevenue-to-total-
	// revenue-growth ratio at or above which FlagGrowthDependentOnNewCustomers
	// triggers (i.e. new-customer revenue accounts for most or all of the
	// period's total revenue growth, meaning the existing base is flat or
	// shrinking beneath it).
	NewCustomerGrowthDependenceRatio float64
	// HighLostCustomerRevenueRatio is the LostCustomerRevenue-to-FromPeriod-
	// total-customer-revenue ratio at or above which
	// FlagHighLostCustomerRevenue triggers.
	HighLostCustomerRevenueRatio float64
	// VolatileRevenueRatio is RevenueVolatility.Value at or above which
	// FlagVolatileRevenue triggers.
	VolatileRevenueRatio float64
	// OnePeriodSpikeRatio is the ratio by which a single period's
	// TotalRevenue exceeds the average of every other available period
	// before FlagOnePeriodSpike triggers (e.g. 0.5 means the spike period is
	// at least 50% above the average of all other periods).
	OnePeriodSpikeRatio float64
	// ShrinkingExistingBaseRatio is the |ExistingCustomerBaseChange|-to-
	// FromPeriod-total-customer-revenue ratio at or above which
	// FlagShrinkingExistingCustomerBase triggers, when
	// ExistingCustomerBaseChange is negative.
	ShrinkingExistingBaseRatio float64
}

// DefaultThresholds returns this package's baseline SMB-advisory trigger
// points. A caller with different risk tolerance supplies its own
// Thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		DecliningRecurringMixPoints:      0.05,
		NewCustomerGrowthDependenceRatio: 0.8,
		HighLostCustomerRevenueRatio:     0.15,
		VolatileRevenueRatio:             0.25,
		OnePeriodSpikeRatio:              0.5,
		ShrinkingExistingBaseRatio:       0.1,
	}
}

// resolveThresholds returns t if it is non-zero, otherwise
// DefaultThresholds() — the same zero-value-means-defaults rule
// qoe.resolveThresholds/cashflow.resolveThresholds use.
func resolveThresholds(t Thresholds) Thresholds {
	if t == (Thresholds{}) {
		return DefaultThresholds()
	}
	return t
}

// Options controls Calculate's optional flag-trigger behavior. The zero
// Options is valid: DefaultThresholds is used.
type Options struct {
	// Thresholds configures every deterministic flag trigger point (see
	// Thresholds). If the zero value, DefaultThresholds() is used —
	// mirroring qoe.Options.Thresholds/cashflow.Options.Thresholds's
	// identical zero-value-means-defaults convention.
	Thresholds Thresholds
}

// FlagCode is a stable identifier for one kind of deterministic
// revenue-quality signal, analogous to qoe.FlagCode/cashflow.FlagCode/
// ratios.SignalCode.
type FlagCode string

const (
	// FlagDecliningRecurringMix means TotalRevenueHistory's
	// RecurringPercent fell by at least Thresholds.DecliningRecurringMixPoints
	// (raw percentage points) from its first to its last available
	// observation.
	FlagDecliningRecurringMix FlagCode = "DECLINING_RECURRING_MIX"
	// FlagGrowthDependentOnNewCustomers means the most recent
	// CustomerTransition's NewCustomerRevenue is at or above
	// Thresholds.NewCustomerGrowthDependenceRatio of that period's total
	// revenue growth.
	FlagGrowthDependentOnNewCustomers FlagCode = "GROWTH_DEPENDENT_ON_NEW_CUSTOMERS"
	// FlagHighLostCustomerRevenue means the most recent CustomerTransition's
	// LostCustomerRevenue-to-FromPeriod-total-customer-revenue ratio is at
	// or above Thresholds.HighLostCustomerRevenueRatio.
	FlagHighLostCustomerRevenue FlagCode = "HIGH_LOST_CUSTOMER_REVENUE"
	// FlagVolatileRevenue means RevenueVolatility.Value is at or above
	// Thresholds.VolatileRevenueRatio.
	FlagVolatileRevenue FlagCode = "VOLATILE_REVENUE"
	// FlagOnePeriodSpike means exactly one period in TotalRevenueHistory
	// exceeds the average of every other available period by at least
	// Thresholds.OnePeriodSpikeRatio.
	FlagOnePeriodSpike FlagCode = "ONE_PERIOD_SPIKE"
	// FlagShrinkingExistingCustomerBase means the most recent
	// CustomerTransition's ExistingCustomerBaseChange is negative and its
	// magnitude relative to FromPeriod's total customer revenue is at or
	// above Thresholds.ShrinkingExistingBaseRatio.
	FlagShrinkingExistingCustomerBase FlagCode = "SHRINKING_EXISTING_CUSTOMER_BASE"
)

// FlagSeverity mirrors qoe.FlagSeverity/cashflow.FlagSeverity/
// ratios.SignalSeverity's role: a structured, matchable urgency signal,
// never inferred from Message text.
type FlagSeverity string

const (
	FlagSeverityInfo     FlagSeverity = "info"
	FlagSeverityWarning  FlagSeverity = "warning"
	FlagSeverityCritical FlagSeverity = "critical"
)

// Flag is a single deterministic, explainable revenue-quality signal. Every
// Flag here is rule-based against Thresholds, never AI-scored — mirroring
// qoe.Flag/cashflow.Flag's identical design.
type Flag struct {
	Code      FlagCode         `json:"code"`
	Severity  FlagSeverity     `json:"severity"`
	Period    financial.Period `json:"period,omitempty"`
	Message   string           `json:"message"`
	Value     float64          `json:"value"`
	Threshold float64          `json:"threshold"`
}

// Result is the output of Calculate: the full per-period revenue
// composition, growth/CAGR/volatility characterization, customer-level
// history and transitions (when Input.CustomerRevenue was supplied),
// concentration summary, deterministic flags, and formula version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoPeriods) — every other field is then zero-value.
	Available bool `json:"available"`

	// TotalRevenueHistory is one PeriodRevenue per period in Input.Dataset,
	// ordered chronologically when Input.PeriodMeta was supplied and covers
	// every period; otherwise in Dataset.Periods()'s lexical order.
	TotalRevenueHistory []PeriodRevenue `json:"total_revenue_history,omitempty"`

	// RevenueStatistics summarizes TotalRevenueHistory's TotalRevenue
	// series (see Statistics).
	RevenueStatistics Statistics `json:"revenue_statistics"`
	// RevenueTrend characterizes TotalRevenue's overall first-vs-last
	// direction across TotalRevenueHistory.
	RevenueTrend Trend `json:"revenue_trend"`
	// RevenueGrowth is one GrowthPoint per chronologically adjacent pair in
	// TotalRevenueHistory. Empty if chronological order was unavailable (see
	// IssueNoPeriodMeta/IssuePeriodMissingFromMeta).
	RevenueGrowth []GrowthPoint `json:"revenue_growth,omitempty"`
	// RevenueCAGR is the compound annual growth rate across
	// TotalRevenueHistory's full available span.
	RevenueCAGR CAGRResult `json:"revenue_cagr"`
	// RevenueVolatility is the period-over-period growth volatility of
	// TotalRevenueHistory.
	RevenueVolatility VolatilityResult `json:"revenue_volatility"`

	// CustomerHistory is one CustomerPeriodTotal per period present in
	// Input.CustomerRevenue and Dataset, in the same order as
	// TotalRevenueHistory. Empty if Input.CustomerRevenue was empty (see
	// IssueNoCustomerData).
	CustomerHistory []CustomerPeriodTotal `json:"customer_history,omitempty"`
	// CustomerTransitions is one CustomerTransition per chronologically
	// adjacent pair of periods present in CustomerHistory. Empty if
	// Input.CustomerRevenue was empty or chronological order was
	// unavailable.
	CustomerTransitions []CustomerTransition `json:"customer_transitions,omitempty"`
	// ConcentrationSummary summarizes customer concentration for the most
	// recent period with customer data. Zero value (CustomerCount == 0) if
	// Input.CustomerRevenue was empty.
	ConcentrationSummary ConcentrationSummary `json:"concentration_summary"`

	// Flags is every deterministic signal Calculate triggered, ordered by
	// FlagCode's declaration order, then by Period.
	Flags []Flag `json:"flags,omitempty"`

	// Thresholds echoes the resolved Options.Thresholds (after
	// DefaultThresholds substitution) this Result was computed under. Named
	// to match Input.Policy's own echo below, mirroring
	// cashflow.Result.Thresholds/workingcapital.Result.InclusionPolicy's
	// identical self-describing-output convention.
	Thresholds Thresholds `json:"thresholds"`
	// Policy echoes the resolved Input.Policy (after DefaultPolicy
	// substitution) this Result was computed under.
	Policy Policy `json:"policy"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
