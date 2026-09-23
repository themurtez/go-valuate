// Package acquisition screens a prospective business acquisition using
// caller-supplied target financials, an asking price, and financing/deal-
// structure assumptions — the arithmetic a buyer or advisor runs to answer
// "what does this deal look like at this price and this financing
// structure," not "should I buy this business." Every figure Calculate
// returns is neutral arithmetic (a multiple, a ratio, a dollar amount) or a
// caller-defined threshold comparison; this package computes no score, no
// composite rating, and no buy/don't-buy recommendation. Decision-support,
// not investment advice.
//
// This package is deliberately independent of financial.FinancialDataset,
// the same design analytics/debt, analytics/covenants, and
// analytics/concentration already established for a domain whose inputs
// frequently do not come from a normalized financial statement traversal
// at all: a target's normalized EBITDA/SDE is typically already computed
// upstream (via financial/metrics and financial/adjustments, or supplied
// by a broker/CIM), a consensus valuation figure comes from
// valuation/consensus, and an asking price and financing terms are deal
// facts with no FinancialDataset representation. This package never
// recomputes EBITDA, SDE, or a consensus valuation itself — a caller
// supplies each already-calculated figure, and this package's job is the
// price-to-earnings, financing, and coverage arithmetic layered on top.
//
// Financing reuses analytics/debt directly. Input.Financing embeds
// []debt.LoanTerms for every tranche of acquisition debt (a bank term
// loan, a seller note, a combination of both — see Financing's doc
// comment), and this package calls debt.Amortize on each to derive annual
// debt service, exactly as a caller building the same analysis by hand
// would. An all-cash deal simply supplies no LoanTerms. This package
// invents no new amortization formula.
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state. Calculate can be called concurrently
// and repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package acquisition

import "github.com/themurtez/go-valuate/analytics/debt"

// FormulaVersion identifies this package's fixed formula set: the
// price-to-revenue/EBITDA/SDE multiple formulas, the premium/discount-
// to-consensus formula, the required-equity-contribution and
// sources-and-uses arithmetic, the annual-debt-service/DSCR/post-debt-
// cash-flow formulas (via analytics/debt.Amortize), the cash-on-cash-
// return and simple-payback-period formulas, the leverage formula, the
// downside/upside scenario methodology, and the red-flag threshold rules.
// Bump this whenever any of that changes in a way that could make a
// historical Result not reproduce identically under new code — see the
// repository README's versioning-strategy section, which this constant
// follows exactly (financial.TaxonomyVersion, metrics.FormulaVersion,
// debt.FormulaVersion, covenants.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown
// because a required input was absent" — the same availability
// convention debt.Value/covenants.Value/metrics.MetricValue all use,
// duplicated here as its own type per this repository's established
// convention (see cashflow.CashFlowValue's doc comment) rather than
// importing another analytics package's Value into this one solely for
// this type (this package already imports analytics/debt for LoanTerms/
// Amortize, but keeps its own Value so its JSON shape and doc comments
// stand alone).
type Value struct {
	// Available is true if Amount is meaningful.
	Available bool `json:"available"`
	// Amount is the figure itself. Meaningful only when Available is
	// true; always 0 when Available is false.
	Amount float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may
// legitimately be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// TargetFinancials bundles the target business's headline figures a
// screening compares the asking price against. Every field is a plain
// caller-supplied figure — this package performs no financial-statement
// traversal and no normalization of its own; a caller who has a
// financial.FinancialDataset derives Revenue via financial/metrics
// upstream, and derives NormalizedEBITDA/NormalizedSDE via
// financial/metrics plus financial/adjustments upstream, exactly as
// analytics/debt.Input.EBITDA already expects.
type TargetFinancials struct {
	// Revenue is the target's trailing (or otherwise caller-defined)
	// annual revenue, used for the asking-price/revenue multiple.
	Revenue Value `json:"revenue"`
	// NormalizedEBITDA is the target's normalized/maintainable EBITDA —
	// after add-backs — used for the asking-price/EBITDA multiple and as
	// the default DSCR/coverage numerator's earnings base (see
	// resolveEarningsBase). Most acquisitions of an owner-operated
	// business use NormalizedSDE instead; a caller supplies whichever of
	// the two figures applies (or both).
	NormalizedEBITDA Value `json:"normalized_ebitda"`
	// NormalizedSDE is the target's normalized seller's discretionary
	// earnings, used for the asking-price/SDE multiple and, when
	// NormalizedEBITDA is unavailable, as the earnings base for coverage
	// figures.
	NormalizedSDE Value `json:"normalized_sde"`
}

// ConsensusValuation is the already-computed consensus valuation figure
// to compare the asking price against — typically
// valuation/consensus.Result.Statistics.WeightedMean or .SimpleMean, on
// whatever basis (enterprise or equity) the caller's AskingPrice is also
// expressed on. This package does not call valuation/consensus itself and
// draws no inference about which consensus statistic or basis was used;
// it only echoes Value/Basis alongside the arithmetic it derives from
// them.
type ConsensusValuation struct {
	// Value is the consensus figure (e.g. WeightedMean or SimpleMean from
	// valuation/consensus.Statistics).
	Value Value `json:"value"`
	// Basis labels which value basis Value is expressed on (e.g.
	// "enterprise_value", "equity_value") for display only — this package
	// performs no basis conversion (see valuation/basis for that) and
	// assumes the caller has already put AskingPrice and Value on a
	// comparable basis.
	Basis string `json:"basis,omitempty"`
}

// TransactionFees are caller-supplied one-time deal costs, each optional.
// When supplied, TotalFees is added to Financing's total sources-and-uses
// requirement (see SourcesAndUses).
type TransactionFees struct {
	// LegalAndAdvisory is legal, accounting, and M&A advisory fees.
	LegalAndAdvisory Value `json:"legal_and_advisory"`
	// DueDiligence is third-party diligence costs (quality of earnings,
	// environmental, technical, etc.).
	DueDiligence Value `json:"due_diligence"`
	// FinancingFees is lender origination/commitment fees on any debt
	// financing.
	FinancingFees Value `json:"financing_fees"`
	// Other is any additional one-time transaction cost not covered above.
	Other Value `json:"other"`
}

// total sums every available TransactionFees field; zero (not
// Unavailable) when no field is supplied, since "no fees supplied" and
// "fees supplied and equal to zero" are both correctly represented as a
// known total of 0 — TransactionFees is entirely optional per the package
// brief ("transaction fees if supplied"), so there is no distinct
// "unavailable" state to preserve here the way there is for e.g. EBITDA.
func (f TransactionFees) total() float64 {
	var sum float64
	for _, v := range []Value{f.LegalAndAdvisory, f.DueDiligence, f.FinancingFees, f.Other} {
		if v.Available {
			sum += v.Amount
		}
	}
	return sum
}

// Financing describes how the deal is funded: zero or more debt tranches
// (a bank acquisition term loan, a seller note, both, or neither for an
// all-cash deal) plus the buyer's own cash contribution.
type Financing struct {
	// DebtTranches is every debt instrument funding the acquisition —
	// e.g. a senior bank term loan and, separately, a seller note, each
	// as its own debt.LoanTerms entry so this package derives each
	// tranche's annual debt service via debt.Amortize independently, then
	// sums across tranches for total annual debt service (mirroring
	// analytics/debt.Input.ExistingDebt/ProposedLoans). Empty means an
	// all-cash deal.
	DebtTranches []debt.LoanTerms `json:"debt_tranches,omitempty"`
	// BuyerCashContribution is the buyer's own cash put into the deal,
	// separate from any debt tranche. When unavailable, RequiredEquity
	// (computed from SourcesAndUses) is used as the presumed cash
	// contribution for CashOnCashReturn/PaybackPeriod instead — see those
	// fields' doc comments.
	BuyerCashContribution Value `json:"buyer_cash_contribution"`
}

// WorkingCapitalRequirement is the incremental net working capital the
// buyer must fund at close, beyond the asking price itself (e.g. a
// broker/CIM-stated NWC peg, or a caller's own analytics/workingcapital-
// derived requirement). Optional; folded into SourcesAndUses.TotalUses
// when available.
type WorkingCapitalRequirement struct {
	Amount Value `json:"amount"`
}

// CapexAssumption is caller-supplied capital-expenditure assumptions used
// only for PostDebtCashFlow (annual maintenance/growth capex funded from
// operating cash flow after debt service, not part of the deal's sources
// and uses at close).
type CapexAssumption struct {
	// AnnualAmount is the assumed ongoing annual capex, deducted from
	// post-debt cash flow. Optional; treated as 0 when unavailable, since
	// an omitted capex assumption is a caller choice to exclude it, not a
	// missing input this package should refuse to compute around (capex
	// is supplementary to, not required for, the core screening figures).
	AnnualAmount Value `json:"annual_amount"`
}

// BuyerCompensationAssumption is an optional owner-operator compensation
// figure a caller wants deducted from earnings before computing
// post-debt cash flow — relevant when NormalizedSDE (which already
// excludes owner compensation as an add-back) is the earnings base and
// the buyer intends to draw a market-rate salary that was not part of the
// seller's addback, or when the buyer wants to model a specific
// replacement-owner salary distinct from the seller's. Left unavailable,
// no compensation deduction is applied (the earnings base is used as-is).
type BuyerCompensationAssumption struct {
	AnnualAmount Value `json:"annual_amount"`
}

// ScenarioAdjustment is one caller-defined stress or upside case applied
// to Revenue/NormalizedEBITDA/NormalizedSDE before recomputing coverage
// and cash-flow figures, mirroring analytics/debt.DownsideScenario's
// haircut convention. Financing terms and the asking price are held fixed
// under every scenario — a scenario stresses the target's earnings, not
// the deal structure.
type ScenarioAdjustment struct {
	// Label identifies this scenario (e.g. "Revenue -15%", "Upside case:
	// +10% EBITDA"). Required for a meaningful ScenarioResult.
	Label string `json:"label"`
	// EBITDAHaircutPercent is the fractional reduction applied to
	// TargetFinancials.NormalizedEBITDA (0.15 means a 15% reduction).
	// May be negative to model an upside case. Applied only when
	// NormalizedEBITDA is available.
	EBITDAHaircutPercent float64 `json:"ebitda_haircut_percent"`
	// SDEHaircutPercent is the fractional reduction applied to
	// TargetFinancials.NormalizedSDE, independent of
	// EBITDAHaircutPercent. Applied only when NormalizedSDE is available.
	SDEHaircutPercent float64 `json:"sde_haircut_percent"`
	// RevenueHaircutPercent is the fractional reduction applied to
	// TargetFinancials.Revenue, independent of the earnings haircuts
	// above (revenue and earnings do not necessarily move in lockstep —
	// a caller who wants them to models both explicitly). Applied only
	// when Revenue is available.
	RevenueHaircutPercent float64 `json:"revenue_haircut_percent"`
}

// RedFlagThresholds are caller-supplied trigger points for deterministic
// red-flag Metrics — this package invents no default thresholds and
// applies none of these checks unless the corresponding threshold is
// explicitly set to a nonzero value, mirroring
// analytics/debt.LenderPolicy and analytics/covenants' fully caller-
// driven threshold model. Every flag this produces is a mechanical
// threshold comparison, never a scored or weighted judgment.
type RedFlagThresholds struct {
	// MinimumDSCR is the lowest acceptable base-case debt-service
	// coverage ratio. Zero means not specified (the DSCR red flag is
	// skipped).
	MinimumDSCR float64 `json:"minimum_dscr,omitempty"`
	// MaximumPriceToEBITDA is the highest acceptable asking-price/EBITDA
	// multiple. Zero means not specified.
	MaximumPriceToEBITDA float64 `json:"maximum_price_to_ebitda,omitempty"`
	// MaximumPriceToSDE is the highest acceptable asking-price/SDE
	// multiple. Zero means not specified.
	MaximumPriceToSDE float64 `json:"maximum_price_to_sde,omitempty"`
	// MaximumPremiumToConsensusPercent is the highest acceptable asking-
	// price premium over consensus value, as a decimal (0.20 = 20% above
	// consensus). Zero means not specified. A discount (asking price
	// below consensus) never triggers this flag regardless of magnitude.
	MaximumPremiumToConsensusPercent float64 `json:"maximum_premium_to_consensus_percent,omitempty"`
	// MinimumCashOnCashReturn is the lowest acceptable first-year cash-
	// on-cash return, as a decimal (0.15 = 15%). Zero means not
	// specified.
	MinimumCashOnCashReturn float64 `json:"minimum_cash_on_cash_return,omitempty"`
	// MaximumPaybackYears is the longest acceptable simple payback
	// period, in years. Zero means not specified.
	MaximumPaybackYears float64 `json:"maximum_payback_years,omitempty"`
	// MaximumDebtToEBITDA is the highest acceptable total-debt-to-EBITDA
	// leverage multiple. Zero means not specified.
	MaximumDebtToEBITDA float64 `json:"maximum_debt_to_ebitda,omitempty"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// Target is the target business's headline financial figures.
	Target TargetFinancials `json:"target"`
	// Consensus is the already-computed consensus valuation to compare
	// AskingPrice against. Optional; when Consensus.Value is unavailable,
	// every consensus-relative figure (PremiumToConsensus, etc.) is left
	// unavailable.
	Consensus ConsensusValuation `json:"consensus"`
	// AskingPrice is the seller's asking price (or a caller-chosen
	// negotiated price) for the transaction, on the same basis as
	// Consensus.Value. Required for every price-multiple and premium/
	// discount figure.
	AskingPrice Value `json:"asking_price"`
	// Fees are one-time transaction costs, if supplied. Optional.
	Fees TransactionFees `json:"fees"`
	// Financing describes the deal's funding structure.
	Financing Financing `json:"financing"`
	// WorkingCapital is the incremental working capital requirement at
	// close, if any.
	WorkingCapital WorkingCapitalRequirement `json:"working_capital"`
	// Capex is the ongoing annual capex assumption used for post-debt
	// cash flow.
	Capex CapexAssumption `json:"capex"`
	// BuyerCompensation is an optional buyer owner-operator compensation
	// assumption deducted before computing post-debt cash flow.
	BuyerCompensation BuyerCompensationAssumption `json:"buyer_compensation"`
	// Scenarios is zero or more caller-defined stress/upside cases,
	// evaluated in the order supplied.
	Scenarios []ScenarioAdjustment `json:"scenarios,omitempty"`
	// RedFlags is the caller-supplied threshold set driving Result.Flags.
	// Zero-value RedFlagThresholds means every threshold check is
	// skipped (advisory-only IssueNoRedFlagThresholds is recorded).
	RedFlags RedFlagThresholds `json:"red_flags"`
}

// IssueSeverity distinguishes an input problem Calculate could not
// proceed past for some portion of the analysis (SeverityError) from one
// that is advisory only (SeverityWarning) — the same two-severity model
// every sibling analytics/transactions package uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent
// with every other package in this repository, rather than reusing
// another package's.
type IssueCode string

const (
	// IssueNoAskingPrice means Input.AskingPrice was unavailable, so
	// every price-multiple and premium/discount figure is unavailable.
	// Financing/coverage figures that do not depend on price are still
	// computed.
	IssueNoAskingPrice IssueCode = "NO_ASKING_PRICE"
	// IssueNoEarningsBase means neither Target.NormalizedEBITDA nor
	// Target.NormalizedSDE was available, so every earnings-based
	// multiple, coverage, and leverage figure is unavailable.
	IssueNoEarningsBase IssueCode = "NO_EARNINGS_BASE"
	// IssueNoConsensusValue means Input.Consensus.Value was unavailable,
	// so PremiumToConsensus/DiscountToConsensus are unavailable.
	IssueNoConsensusValue IssueCode = "NO_CONSENSUS_VALUE"
	// IssueInvalidLoanTerms means at least one Financing.DebtTranches
	// entry failed debt.LoanTerms validation (see debt.Amortize's
	// validation rules, applied identically here); that tranche is
	// excluded from every downstream calculation.
	IssueInvalidLoanTerms IssueCode = "INVALID_LOAN_TERMS"
	// IssueAllCashDeal means Financing.DebtTranches was empty, so annual
	// debt service is treated as zero and DSCR/leverage figures that
	// require nonzero debt service are unavailable rather than reported
	// as an infinite or undefined ratio. Advisory only.
	IssueAllCashDeal IssueCode = "ALL_CASH_DEAL"
	// IssueNoRedFlagThresholds means Input.RedFlags had no non-zero
	// threshold, so every red-flag check was skipped. Advisory only.
	IssueNoRedFlagThresholds IssueCode = "NO_RED_FLAG_THRESHOLDS"
	// IssueNoCashContribution means neither
	// Financing.BuyerCashContribution nor a computable RequiredEquity
	// was available, so CashOnCashReturn and PaybackPeriod are
	// unavailable.
	IssueNoCashContribution IssueCode = "NO_CASH_CONTRIBUTION"
)

// Issue is a single Calculate-time input finding.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// Tranche identifies, by index into Financing.DebtTranches (e.g.
	// "debt_tranches[0]"), which entry an Issue relates to. Empty when
	// the Issue is not tranche-specific.
	Tranche string `json:"tranche,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from debt.HasErrors/covenants.HasErrors and
// this repository's other sibling HasErrors functions rather than shared
// — see adjustments.HasErrors's doc comment for the full rationale (each
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

// EarningsBaseSource identifies which TargetFinancials figure a
// computation's earnings base was drawn from.
type EarningsBaseSource string

const (
	// EarningsSourceEBITDA means Target.NormalizedEBITDA was available
	// and used.
	EarningsSourceEBITDA EarningsBaseSource = "ebitda"
	// EarningsSourceSDE means Target.NormalizedEBITDA was unavailable
	// (or a scenario's EBITDA figure could not be derived), so
	// Target.NormalizedSDE was used instead.
	EarningsSourceSDE EarningsBaseSource = "sde"
	// EarningsSourceUnavailable means neither was available.
	EarningsSourceUnavailable EarningsBaseSource = "unavailable"
)

// PriceMultiples holds every asking-price-to-earnings multiple Calculate
// derives.
type PriceMultiples struct {
	// PriceToRevenue is AskingPrice / Target.Revenue. Available only when
	// both are available and Revenue is nonzero.
	PriceToRevenue Value `json:"price_to_revenue"`
	// PriceToEBITDA is AskingPrice / Target.NormalizedEBITDA. Available
	// only when both are available and NormalizedEBITDA is strictly
	// positive (a multiple against a zero or negative EBITDA is not a
	// meaningful ratio).
	PriceToEBITDA Value `json:"price_to_ebitda"`
	// PriceToSDE is AskingPrice / Target.NormalizedSDE. Available only
	// when both are available and NormalizedSDE is strictly positive.
	PriceToSDE Value `json:"price_to_sde"`
}

// ConsensusComparison holds AskingPrice's relationship to
// Input.Consensus.Value, as neutral arithmetic — see the package doc
// comment on avoiding evaluative verdicts. A positive Premium/
// PremiumPercent means AskingPrice is above consensus; negative means
// below (a discount). This package labels neither direction as favorable
// or unfavorable.
type ConsensusComparison struct {
	// ConsensusValue echoes Input.Consensus.Value for display alongside
	// the derived figures.
	ConsensusValue Value `json:"consensus_value"`
	// Premium is AskingPrice.Amount - ConsensusValue.Amount (signed).
	// Available only when both are available.
	Premium Value `json:"premium"`
	// PremiumPercent is Premium.Amount / ConsensusValue.Amount, as a
	// decimal (signed). Available only when both are available and
	// ConsensusValue is nonzero.
	PremiumPercent Value `json:"premium_percent"`
}

// SourcesAndUses is the deal's funding sources (debt tranches + buyer
// cash) against its total uses (asking price + fees + working capital),
// and the resulting required equity contribution.
type SourcesAndUses struct {
	// TotalUses is AskingPrice + Fees.total() + WorkingCapital.Amount
	// (each included only when available). Available only when
	// AskingPrice is available.
	TotalUses Value `json:"total_uses"`
	// TotalDebtFinancing is the sum of every valid Financing.DebtTranches
	// entry's Principal. Available (as a known, possibly-zero figure)
	// whenever TotalUses is available, since "no debt tranches" is a
	// known figure of zero debt, not a missing one.
	TotalDebtFinancing Value `json:"total_debt_financing"`
	// RequiredEquity is TotalUses.Amount - TotalDebtFinancing.Amount —
	// the cash the buyer must contribute (from BuyerCashContribution or
	// any other source) to close the deal at the resolved financing
	// structure. Available only when TotalUses is available.
	RequiredEquity Value `json:"required_equity"`
}

// CoverageResult holds one set of financing/coverage figures — used both
// for Result.BaseCase and for each ScenarioResult.
type CoverageResult struct {
	// EarningsBaseSource identifies which TargetFinancials figure
	// CoverageNumerator was computed from.
	EarningsBaseSource EarningsBaseSource `json:"earnings_base_source"`
	// CoverageNumerator is the actual earnings figure used (after any
	// scenario haircut).
	CoverageNumerator Value `json:"coverage_numerator"`

	// AnnualDebtService is the sum of every valid debt tranche's
	// debt.AmortizationSchedule.FirstYearAnnualDebtService. Available (as
	// a known, possibly-zero figure) whenever financing was resolved —
	// see SourcesAndUses.TotalDebtFinancing's doc comment for the same
	// "no debt is a known zero" convention.
	AnnualDebtService Value `json:"annual_debt_service"`
	// DSCR is CoverageNumerator.Amount / AnnualDebtService.Amount.
	// Available only when both are available and AnnualDebtService is
	// nonzero (an all-cash deal's zero debt service makes DSCR undefined
	// rather than infinite — see IssueAllCashDeal).
	DSCR Value `json:"dscr"`

	// PostDebtCashFlow is CoverageNumerator - AnnualDebtService -
	// Capex.AnnualAmount - BuyerCompensation.AnnualAmount (each
	// subtracted only when available). Available only when
	// CoverageNumerator and AnnualDebtService are both available.
	PostDebtCashFlow Value `json:"post_debt_cash_flow"`

	// Leverage is TotalDebtFinancing / earnings base (the scenario's
	// earnings figure, not CoverageNumerator specifically — leverage is
	// conventionally measured against EBITDA/SDE directly). Available
	// only when both are available and the earnings figure is strictly
	// positive.
	Leverage Value `json:"leverage"`
}

// ReturnMetrics holds the buyer-economics figures derived from
// PostDebtCashFlow and the resolved cash contribution.
type ReturnMetrics struct {
	// CashContribution is Financing.BuyerCashContribution if available,
	// otherwise SourcesAndUses.RequiredEquity — the denominator for
	// CashOnCashReturn and the basis for PaybackPeriod. Available only
	// when one of those two sources is.
	CashContribution Value `json:"cash_contribution"`
	// CashOnCashReturn is PostDebtCashFlow.Amount /
	// CashContribution.Amount, as a decimal. Available only when both are
	// available and CashContribution is strictly positive.
	CashOnCashReturn Value `json:"cash_on_cash_return"`
	// PaybackPeriodYears is CashContribution.Amount /
	// PostDebtCashFlow.Amount — the simple (undiscounted) number of years
	// of PostDebtCashFlow needed to recover CashContribution. Available
	// only when both are available and PostDebtCashFlow is strictly
	// positive (a zero or negative post-debt cash flow never pays back,
	// which this package reports as unavailable rather than an infinite
	// or negative period).
	PaybackPeriodYears Value `json:"payback_period_years"`
}

// ScenarioResult is one ScenarioAdjustment's full recomputation, applying
// that scenario's haircuts to Target.Revenue/NormalizedEBITDA/
// NormalizedSDE before recomputing PriceMultiples, CoverageResult, and
// ReturnMetrics, with AskingPrice, Financing, and Fees held fixed (a
// scenario stresses the target's financial performance, not the deal
// terms).
type ScenarioResult struct {
	// Scenario echoes the ScenarioAdjustment this result was computed
	// for.
	Scenario ScenarioAdjustment `json:"scenario"`
	// Multiples is PriceMultiples recomputed under this scenario's
	// stressed Revenue/EBITDA/SDE.
	Multiples PriceMultiples `json:"multiples"`
	// Coverage is CoverageResult recomputed under this scenario's
	// stressed earnings, with financing held fixed.
	Coverage CoverageResult `json:"coverage"`
	// Returns is ReturnMetrics recomputed from this scenario's Coverage.
	Returns ReturnMetrics `json:"returns"`
	// BreachesMinimumDSCR is true when Input.RedFlags.MinimumDSCR is set,
	// Coverage.DSCR is available, and Coverage.DSCR is below that
	// minimum. False (not just "unavailable") when
	// RedFlags.MinimumDSCR is unset, since there is no threshold to
	// breach.
	BreachesMinimumDSCR bool `json:"breaches_minimum_dscr"`
	// HasNegativeCashFlow is true when Coverage.PostDebtCashFlow is
	// available and below zero.
	HasNegativeCashFlow bool `json:"has_negative_cash_flow"`
}

// FlagCode is a stable identifier for one kind of deterministic
// acquisition-screening signal, analogous to debt.FlagCode/
// covenants.Operator-driven checks. Every Flag here is a mechanical
// threshold comparison against Input.RedFlags, never a scored or weighted
// judgment — see the package doc comment on avoiding buy/don't-buy
// verdicts.
type FlagCode string

const (
	// FlagBelowMinimumDSCR means Result.BaseCase.DSCR is available and
	// below Input.RedFlags.MinimumDSCR (when set).
	FlagBelowMinimumDSCR FlagCode = "BELOW_MINIMUM_DSCR"
	// FlagAboveMaximumPriceToEBITDA means Result.Multiples.PriceToEBITDA
	// exceeds Input.RedFlags.MaximumPriceToEBITDA (when set).
	FlagAboveMaximumPriceToEBITDA FlagCode = "ABOVE_MAXIMUM_PRICE_TO_EBITDA"
	// FlagAboveMaximumPriceToSDE means Result.Multiples.PriceToSDE
	// exceeds Input.RedFlags.MaximumPriceToSDE (when set).
	FlagAboveMaximumPriceToSDE FlagCode = "ABOVE_MAXIMUM_PRICE_TO_SDE"
	// FlagAboveMaximumPremiumToConsensus means
	// Result.Consensus.PremiumPercent exceeds
	// Input.RedFlags.MaximumPremiumToConsensusPercent (when set). Never
	// triggered by a discount (a negative premium), regardless of
	// magnitude — see RedFlagThresholds.MaximumPremiumToConsensusPercent.
	FlagAboveMaximumPremiumToConsensus FlagCode = "ABOVE_MAXIMUM_PREMIUM_TO_CONSENSUS"
	// FlagBelowMinimumCashOnCashReturn means
	// Result.Returns.CashOnCashReturn is available and below
	// Input.RedFlags.MinimumCashOnCashReturn (when set).
	FlagBelowMinimumCashOnCashReturn FlagCode = "BELOW_MINIMUM_CASH_ON_CASH_RETURN"
	// FlagAboveMaximumPayback means Result.Returns.PaybackPeriodYears is
	// available and above Input.RedFlags.MaximumPaybackYears (when set).
	FlagAboveMaximumPayback FlagCode = "ABOVE_MAXIMUM_PAYBACK"
	// FlagAboveMaximumLeverage means Result.BaseCase.Leverage exceeds
	// Input.RedFlags.MaximumDebtToEBITDA (when set).
	FlagAboveMaximumLeverage FlagCode = "ABOVE_MAXIMUM_LEVERAGE"
	// FlagNegativePostDebtCashFlow means Result.BaseCase.PostDebtCashFlow
	// is available and below zero — the deal does not cash-flow at the
	// base case, independent of any caller threshold.
	FlagNegativePostDebtCashFlow FlagCode = "NEGATIVE_POST_DEBT_CASH_FLOW"
	// FlagScenarioBreachesDSCR means at least one ScenarioResult has
	// BreachesMinimumDSCR == true.
	FlagScenarioBreachesDSCR FlagCode = "SCENARIO_BREACHES_DSCR"
	// FlagScenarioNegativeCashFlow means at least one ScenarioResult has
	// HasNegativeCashFlow == true.
	FlagScenarioNegativeCashFlow FlagCode = "SCENARIO_NEGATIVE_CASH_FLOW"
)

// FlagSeverity mirrors debt.FlagSeverity/covenants' severity role: a
// structured, matchable urgency signal, never inferred from Message text.
type FlagSeverity string

const (
	FlagSeverityInfo     FlagSeverity = "info"
	FlagSeverityWarning  FlagSeverity = "warning"
	FlagSeverityCritical FlagSeverity = "critical"
)

// Flag is a single deterministic, explainable acquisition-screening
// signal. Every Flag here is rule-based against Input.RedFlags or a fixed
// rule (FlagNegativePostDebtCashFlow), never AI-scored, and never an
// evaluative buy/don't-buy verdict — see the package doc comment.
type Flag struct {
	Code      FlagCode     `json:"code"`
	Severity  FlagSeverity `json:"severity"`
	Scenario  string       `json:"scenario,omitempty"`
	Message   string       `json:"message"`
	Value     float64      `json:"value"`
	Threshold float64      `json:"threshold"`
}

// Result is the output of Calculate: price multiples, consensus
// comparison, sources and uses, base-case financing/coverage/return
// figures, downside/upside scenarios, deterministic red flags, and
// formula version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (no
	// asking price, no earnings base, and no financing/consensus input
	// supplied at all) — every other field is then zero-value. In every
	// other case Calculate computes whatever subset of the analysis the
	// supplied input supports and reports gaps via Warnings/Errors and
	// per-field Value.Available.
	Available bool `json:"available"`

	// Multiples holds the base-case asking-price-to-earnings multiples.
	Multiples PriceMultiples `json:"multiples"`
	// Consensus holds AskingPrice's comparison to Input.Consensus.Value.
	Consensus ConsensusComparison `json:"consensus"`
	// SourcesAndUses holds the deal's funding sources/uses and required
	// equity contribution.
	SourcesAndUses SourcesAndUses `json:"sources_and_uses"`

	// DebtSchedules is one debt.AmortizationSchedule per valid entry in
	// Financing.DebtTranches, in the same order.
	DebtSchedules []debt.AmortizationSchedule `json:"debt_schedules,omitempty"`

	// BaseCase is the financing/coverage/leverage analysis at Input's
	// actual (non-haircut) earnings.
	BaseCase CoverageResult `json:"base_case"`
	// Returns is the buyer cash-on-cash-return/payback analysis derived
	// from BaseCase.
	Returns ReturnMetrics `json:"returns"`

	// Scenarios is one ScenarioResult per Input.Scenarios entry, in the
	// same order.
	Scenarios []ScenarioResult `json:"scenarios,omitempty"`

	// Flags is every deterministic red-flag signal Calculate triggered,
	// ordered by FlagCode's declaration order, then by Scenario.
	Flags []Flag `json:"flags,omitempty"`

	// RedFlags echoes Input.RedFlags this Result was screened against.
	RedFlags RedFlagThresholds `json:"red_flags"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
