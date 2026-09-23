// Package dealstructure models an acquisition's financing structure —
// sources and uses, debt tranches, a seller note, an earnout, and
// transaction fees — independently from valuation. Given a purchase price
// and a caller's chosen financing assumptions, this package derives how
// the deal is funded (sources vs. uses, required equity, a funding gap or
// surplus) and how each financing instrument pays out over time (a full
// period-by-period amortization schedule, including any interest-only
// period and balloon payment). It answers "how is this deal financed and
// what does the resulting payment schedule look like," never "what is
// this business worth" — see transactions/acquisition and the valuation/*
// packages for that question. A caller who has both an asking price from
// this package's SourcesAndUses and a target's earnings feeds
// acquisition.Input.Financing.DebtTranches-equivalent figures onward into
// that package for coverage/return screening; this package computes no
// DSCR, no leverage ratio, and no buyer return of its own.
//
// This package is deliberately independent of financial.FinancialDataset
// and of analytics/debt, the same design transactions/acquisition,
// analytics/covenants, and analytics/concentration already established
// for a domain whose inputs are deal facts (a purchase price, a lender's
// term sheet, a negotiated seller-note rate, an earnout schedule) with no
// financial-statement-traversal representation. Financing here also does
// not reuse analytics/debt.Amortize: that function derives only a
// first-year and steady-state annual debt service figure, never a full
// period-by-period schedule, and analytics/debt.LoanTerms has no balloon
// field — both of which this package's brief explicitly requires
// ("payment schedule," "interest/principal by period," "balloon if
// supplied"). Rather than force a balloon-capable, full-schedule
// amortizer into an awkward shared dependency with a package whose
// LoanTerms/Value types were not designed for it, this package implements
// its own amortization engine using the same standard level-payment
// formula analytics/debt already uses, extended with full-schedule and
// balloon support.
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state. Build can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package dealstructure

// FormulaVersion identifies this package's fixed formula set: the
// sources-and-uses/required-equity/funding-gap arithmetic, the
// per-tranche amortization formula (including interest-only handling and
// balloon-payment sizing), the seller-note and earnout schedule
// derivations, the financing-percentage formula, and the annual-
// debt-service aggregation across tranches. Bump this whenever any of
// that changes in a way that could make a historical Result not
// reproduce identically under new code — see the repository README's
// versioning-strategy section, which this constant follows exactly
// (financial.TaxonomyVersion, metrics.FormulaVersion, debt.FormulaVersion,
// acquisition.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown
// because a required input was absent" — the same availability
// convention debt.Value/acquisition.Value/covenants.Value all use,
// duplicated here as its own type per this repository's established
// convention (see cashflow.CashFlowValue's doc comment) rather than
// importing another package's Value into this one solely for this type.
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

// PaymentFrequency is how often a debt tranche's or seller note's
// scheduled payments occur — the same three frequencies
// analytics/debt.PaymentFrequency recognizes, duplicated here since this
// package does not import analytics/debt (see the package doc comment).
type PaymentFrequency string

const (
	FrequencyMonthly   PaymentFrequency = "monthly"
	FrequencyQuarterly PaymentFrequency = "quarterly"
	FrequencyAnnual    PaymentFrequency = "annual"
)

// paymentsPerYear returns the number of scheduled payments per year for
// f, and false if f is not one of the recognized PaymentFrequency
// values.
func paymentsPerYear(f PaymentFrequency) (int, bool) {
	switch f {
	case FrequencyMonthly:
		return 12, true
	case FrequencyQuarterly:
		return 4, true
	case FrequencyAnnual:
		return 1, true
	default:
		return 0, false
	}
}

// DebtTranche describes one debt instrument funding the acquisition — a
// senior bank term loan, a mezzanine tranche, an equipment note, or any
// other amortizing loan. A seller note is modeled separately (see
// SellerNote) since it is called out as its own deal component in this
// package's brief, but shares an identical amortization engine.
type DebtTranche struct {
	// Label identifies this tranche for display (e.g. "Senior term
	// loan", "Mezzanine debt"). Optional.
	Label string `json:"label,omitempty"`
	// Amount is the original principal amount of this tranche. Must be
	// >= 0.
	Amount float64 `json:"amount"`
	// AnnualInterestRate is the nominal annual interest rate as a
	// decimal (0.08 for 8%). Must be >= 0; a rate above 1.0 (100%) is
	// accepted but flagged as an Issue since it is almost certainly a
	// units mistake (e.g. "8" instead of "0.08").
	AnnualInterestRate float64 `json:"annual_interest_rate"`
	// AmortizationYears is the number of years over which Amount
	// amortizes — to zero if BalloonAmount is unset, or down to
	// BalloonAmount if set (see BalloonAmount's doc comment) — excluding
	// any InterestOnlyYears. Must be > 0.
	AmortizationYears float64 `json:"amortization_years"`
	// TermYears is the actual life of the tranche: how many years of
	// payments actually occur before the loan is repaid in full (via
	// its final scheduled payment, or via BalloonAmount). Must be > 0
	// and <= AmortizationYears. When TermYears == AmortizationYears (the
	// common case), the loan simply amortizes to zero (or to
	// BalloonAmount) on its own final scheduled payment. When TermYears
	// < AmortizationYears, the loan is amortizing on an
	// AmortizationYears schedule but comes due — with its remaining
	// balance payable — at TermYears (a standard "10-year amortization,
	// 5-year term" structure), regardless of whether BalloonAmount is
	// also set; see Amortize's doc comment for exactly how the two
	// interact.
	TermYears float64 `json:"term_years"`
	// Frequency is how often payments occur. Must be one of the
	// PaymentFrequency constants.
	Frequency PaymentFrequency `json:"frequency"`
	// InterestOnlyYears is the number of years, at the start of the
	// tranche, during which only interest is paid (no principal
	// amortization). AmortizationYears still governs the amortization
	// schedule used to size the principal-and-interest payment once
	// amortization begins, mirroring analytics/debt.LoanTerms.
	// InterestOnlyYears's identical convention. Zero means no
	// interest-only period.
	InterestOnlyYears float64 `json:"interest_only_years,omitempty"`
	// BalloonAmount is the remaining principal balance due as a single
	// lump-sum payment at the end of the tranche's life, if supplied
	// (nonzero). When set, this tranche's periodic
	// principal-and-interest payment is sized so that amortizing
	// Amount over AmortizationYears leaves exactly BalloonAmount
	// outstanding at TermYears, rather than fully amortizing to zero —
	// a standard "$X balloon at year Y" structure. Zero (the default)
	// means no balloon: the tranche either fully amortizes to zero over
	// AmortizationYears, or, if TermYears < AmortizationYears, comes due
	// with whatever balance remains from the ordinary
	// (zero-balloon-sized) payment at that point.
	BalloonAmount float64 `json:"balloon_amount,omitempty"`
}

// SellerNote describes seller financing: a note the seller carries back
// as part of the purchase price, sharing DebtTranche's amortization
// engine but modeled as its own Input field since a caller's sources and
// uses, financing-percentage breakdown, and Result shape treat it as a
// distinct, separately labeled deal component (mirroring how brokers and
// buyers themselves always distinguish seller financing from bank debt).
type SellerNote struct {
	// Included is false when no seller note is part of this deal. When
	// false every other field is ignored.
	Included bool        `json:"included"`
	Terms    DebtTranche `json:"terms"`
}

// EarnoutPayment is one scheduled, deterministic earnout payment —
// already agreed at a specific amount and period, not a formula this
// package evaluates against future performance. This package accepts
// only a fully deterministic schedule (per the task brief: "earnout if
// deterministic schedule supplied"); a contingent, performance-based
// earnout (e.g. "10% of Year 2 revenue above $5M") is out of scope since
// evaluating it would require forecasting the target's future
// performance, a valuation/forecasting concern this package does not
// take on.
type EarnoutPayment struct {
	// PeriodNumber is which year (1-indexed, from close) this payment is
	// due. Must be >= 1.
	PeriodNumber int `json:"period_number"`
	// Amount is the payment amount due in this period. May be zero (a
	// scheduled period with no payment due) but not negative.
	Amount float64 `json:"amount"`
	// Label describes this payment for display (e.g. "Year 1 earnout —
	// revenue milestone"). Optional.
	Label string `json:"label,omitempty"`
}

// Earnout is the deal's earnout component, if any.
type Earnout struct {
	// Included is false when no earnout is part of this deal. When
	// false, Payments is ignored.
	Included bool `json:"included"`
	// Payments is the deterministic, already-agreed payment schedule, in
	// whatever order supplied (Result.EarnoutSchedule sorts by
	// PeriodNumber — see that field's doc comment).
	Payments []EarnoutPayment `json:"payments,omitempty"`
}

// TransactionFees are caller-supplied one-time deal costs, each optional.
// When supplied, Total() is added to Uses.
type TransactionFees struct {
	// LegalAndAdvisory is legal, accounting, and M&A advisory fees.
	LegalAndAdvisory Value `json:"legal_and_advisory"`
	// DueDiligence is third-party diligence costs (quality of earnings,
	// environmental, technical, etc.).
	DueDiligence Value `json:"due_diligence"`
	// FinancingFees is lender origination/commitment fees on any debt
	// financing.
	FinancingFees Value `json:"financing_fees"`
	// Other is any additional one-time transaction cost not covered
	// above.
	Other Value `json:"other"`
}

// total sums every available TransactionFees field; zero (not
// Unavailable) when no field is supplied, mirroring
// acquisition.TransactionFees.total's identical "no fees supplied and
// fees supplied as zero are both a known total of 0" rationale.
func (f TransactionFees) total() float64 {
	var sum float64
	for _, v := range []Value{f.LegalAndAdvisory, f.DueDiligence, f.FinancingFees, f.Other} {
		if v.Available {
			sum += v.Amount
		}
	}
	return sum
}

// WorkingCapitalContribution is the incremental net working capital the
// buyer must fund at close, beyond the purchase price itself (e.g. a
// broker/CIM-stated NWC peg, or a caller's own analytics/workingcapital-
// derived requirement). Optional; folded into Uses when available.
type WorkingCapitalContribution struct {
	Amount Value `json:"amount"`
}

// ClosingAdjustments are optional caller-supplied adjustments to the
// purchase price at close, distinct from the deal's ordinary uses. Each
// is independently optional.
type ClosingAdjustments struct {
	// CashAcquired is cash on the target's balance sheet the buyer
	// receives at close, reducing the net amount the buyer must fund
	// (a cash-free/debt-free deal convention: the buyer nets out
	// acquired cash against the purchase price). Subtracted from Uses
	// when available.
	CashAcquired Value `json:"cash_acquired"`
	// AssumedDebt is existing target-company debt the buyer assumes as
	// part of the transaction (rather than the seller retiring it from
	// proceeds), added to Uses when available since it is additional
	// consideration the buyer is effectively funding.
	AssumedDebt Value `json:"assumed_debt"`
}

// Input bundles everything Build needs.
type Input struct {
	// PurchasePrice is the total agreed purchase price for the
	// transaction. Required for every sources-and-uses figure.
	PurchasePrice Value `json:"purchase_price"`
	// BuyerEquity is the buyer's own cash/equity contribution — the
	// down payment funded from the buyer's own capital, separate from
	// any debt tranche or seller note.
	BuyerEquity Value `json:"buyer_equity"`
	// DebtTranches is every debt instrument funding the acquisition
	// (excluding the seller note — see SellerNote), in the order
	// supplied. Empty means no third-party debt.
	DebtTranches []DebtTranche `json:"debt_tranches,omitempty"`
	// SellerNote is the deal's seller-financing component, if any.
	SellerNote SellerNote `json:"seller_note"`
	// Earnout is the deal's earnout component, if any.
	Earnout Earnout `json:"earnout"`
	// Fees are one-time transaction costs, if supplied. Optional.
	Fees TransactionFees `json:"fees"`
	// WorkingCapital is the incremental working capital contribution
	// required at close, if any.
	WorkingCapital WorkingCapitalContribution `json:"working_capital"`
	// ClosingAdjustments are optional cash-acquired/assumed-debt
	// adjustments to Uses.
	ClosingAdjustments ClosingAdjustments `json:"closing_adjustments"`
}

// IssueSeverity distinguishes an input problem Build could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model every
// sibling analytics/transactions package uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Build-time input
// problem. This package defines its own separate taxonomy, consistent
// with every other package in this repository, rather than reusing
// another package's.
type IssueCode string

const (
	// IssueNoPurchasePrice means Input.PurchasePrice was unavailable, so
	// Uses and every figure derived from it (RequiredEquity,
	// FundingGap, financing percentages) is unavailable. Per-tranche
	// amortization schedules are still computed.
	IssueNoPurchasePrice IssueCode = "NO_PURCHASE_PRICE"
	// IssueInvalidTrancheTerms means a DebtTranche (or the SellerNote's
	// Terms) failed structural validation (Amount/rate negative,
	// AmortizationYears/TermYears non-positive, TermYears exceeding
	// AmortizationYears, an unrecognized Frequency, InterestOnlyYears at
	// least as long as AmortizationYears, or a BalloonAmount exceeding
	// what Amount amortizing over AmortizationYears could leave
	// outstanding). That tranche is excluded from every downstream
	// calculation.
	IssueInvalidTrancheTerms IssueCode = "INVALID_TRANCHE_TERMS"
	// IssueNoFinancingSources means no BuyerEquity, DebtTranches,
	// SellerNote, or Earnout was supplied at all — Sources is a known
	// zero rather than a computed figure. Advisory only.
	IssueNoFinancingSources IssueCode = "NO_FINANCING_SOURCES"
	// IssueFundingGap means TotalSources is available and less than
	// TotalUses — the deal as structured does not fully fund its uses.
	// Advisory only (the gap itself is reported numerically via
	// Result.FundingGapOrSurplus).
	IssueFundingGap IssueCode = "FUNDING_GAP"
	// IssueInvalidEarnoutPayment means an EarnoutPayment had a
	// PeriodNumber < 1 or a negative Amount. That payment is excluded
	// from EarnoutSchedule.
	IssueInvalidEarnoutPayment IssueCode = "INVALID_EARNOUT_PAYMENT"
)

// Issue is a single Build-time input finding.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// Tranche identifies, by reference (e.g. "debt_tranches[0]",
	// "seller_note", "earnout.payments[2]"), which entry an Issue
	// relates to. Empty when the Issue is not entry-specific.
	Tranche string `json:"tranche,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from debt.HasErrors/acquisition.HasErrors and
// this repository's other sibling HasErrors functions rather than
// shared — see adjustments.HasErrors's doc comment for the full
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

// PeriodDebtService is one period's principal/interest/balance breakdown
// within an AmortizationSchedule.
type PeriodDebtService struct {
	// PeriodNumber is this payment's 1-indexed sequence number within
	// the schedule.
	PeriodNumber int `json:"period_number"`
	// Payment is the total cash paid this period (principal + interest,
	// or the interest-only payment, or, in the final period, the
	// regular payment plus any BalloonAmount).
	Payment float64 `json:"payment"`
	// Interest is this period's interest component.
	Interest float64 `json:"interest"`
	// Principal is this period's principal component (0 during the
	// interest-only period).
	Principal float64 `json:"principal"`
	// Balloon is any lump-sum balloon principal paid this period (only
	// ever nonzero in the schedule's final period).
	Balloon float64 `json:"balloon,omitempty"`
	// EndingBalance is the outstanding principal balance immediately
	// after this period's payment (including any Balloon).
	EndingBalance float64 `json:"ending_balance"`
}

// AmortizationSchedule is one DebtTranche's (or the SellerNote's) full
// derived payment schedule, computed by Amortize.
type AmortizationSchedule struct {
	// Label echoes the originating DebtTranche.Label (or "Seller note"
	// when this schedule was derived from Input.SellerNote).
	Label string `json:"label,omitempty"`
	// Terms echoes the DebtTranche this schedule was derived from.
	Terms DebtTranche `json:"terms"`
	// PaymentsPerYear is the resolved payment count implied by
	// Terms.Frequency.
	PaymentsPerYear int `json:"payments_per_year"`
	// PeriodicPrincipalAndInterestPayment is the level payment amount
	// once amortization begins (after any InterestOnlyYears), covering
	// both principal and interest each period (excluding the final
	// period's balloon, if any).
	PeriodicPrincipalAndInterestPayment float64 `json:"periodic_principal_and_interest_payment"`
	// PeriodicInterestOnlyPayment is the payment amount during the
	// InterestOnlyYears period (Terms.Amount x periodic rate). Zero if
	// Terms.InterestOnlyYears is zero.
	PeriodicInterestOnlyPayment float64 `json:"periodic_interest_only_payment,omitempty"`
	// Periods is the full period-by-period schedule from origination
	// through the tranche's final payment (at Terms.TermYears), in
	// ascending PeriodNumber order.
	Periods []PeriodDebtService `json:"periods"`
	// FirstYearAnnualDebtService is the total principal + interest paid
	// in the first 12 months.
	FirstYearAnnualDebtService float64 `json:"first_year_annual_debt_service"`
	// SteadyStateAnnualDebtService is the total principal + interest
	// paid in a full year once amortization is underway
	// (PeriodicPrincipalAndInterestPayment x PaymentsPerYear).
	SteadyStateAnnualDebtService float64 `json:"steady_state_annual_debt_service"`
	// BalloonPayment is Terms.BalloonAmount, echoed here for
	// convenience (0 when Terms.BalloonAmount is unset). Also present
	// as the final Periods entry's Balloon field.
	BalloonPayment float64 `json:"balloon_payment,omitempty"`
}

// AnnualDebtServicePeriod is one year's aggregated debt service across
// every tranche (DebtTranches + SellerNote, when included).
type AnnualDebtServicePeriod struct {
	// Year is the 1-indexed year number from close.
	Year int `json:"year"`
	// Payment is total cash paid across every tranche this year
	// (principal + interest + any balloon due this year).
	Payment float64 `json:"payment"`
	// Interest is the interest component across every tranche this
	// year.
	Interest float64 `json:"interest"`
	// Principal is the scheduled (non-balloon) principal component
	// across every tranche this year.
	Principal float64 `json:"principal"`
	// Balloon is any balloon principal due across every tranche this
	// year.
	Balloon float64 `json:"balloon,omitempty"`
}

// SourcesAndUses is the deal's funding sources against its total uses,
// and the resulting required equity/funding gap.
type SourcesAndUses struct {
	// TotalUses is PurchasePrice + Fees.total() + WorkingCapital.Amount
	// + ClosingAdjustments.AssumedDebt - ClosingAdjustments.CashAcquired
	// (each included only when available). Available only when
	// PurchasePrice is available.
	TotalUses Value `json:"total_uses"`

	// TotalDebtFinancing is the sum of every valid DebtTranches entry's
	// Amount (excluding the seller note). Available (as a known,
	// possibly-zero figure) once Build runs, since "no third-party
	// debt" is a known figure of zero, not a missing one.
	TotalDebtFinancing Value `json:"total_debt_financing"`
	// SellerNoteAmount is SellerNote.Terms.Amount when SellerNote.Included,
	// otherwise a known zero.
	SellerNoteAmount Value `json:"seller_note_amount"`
	// TotalEarnoutAmount is the sum of every valid EarnoutPayment.Amount
	// when Earnout.Included, otherwise a known zero. Included in
	// TotalSources on the view that a seller-financed earnout is deal
	// consideration the buyer does not fund at close (mirroring
	// SellerNoteAmount's treatment as a funding source rather than a
	// cash use), even though — unlike debt principal or a seller note —
	// it is paid out over time from future cash flow rather than drawn
	// at closing.
	TotalEarnoutAmount Value `json:"total_earnout_amount"`
	// BuyerEquity echoes Input.BuyerEquity.
	BuyerEquity Value `json:"buyer_equity"`

	// TotalSources is TotalDebtFinancing + SellerNoteAmount +
	// TotalEarnoutAmount + BuyerEquity (each included only when
	// available; unavailable BuyerEquity is treated as 0 in this sum
	// since the other three are always known-zero-or-more once Build
	// runs — see IssueNoFinancingSources for the all-unavailable case).
	TotalSources Value `json:"total_sources"`

	// RequiredEquity is TotalUses.Amount - (TotalDebtFinancing.Amount +
	// SellerNoteAmount.Amount + TotalEarnoutAmount.Amount) — the buyer
	// cash contribution the deal's non-equity financing does not cover.
	// Available only when TotalUses is available. This may differ from
	// BuyerEquity when the buyer's supplied equity does not match what
	// the deal actually requires — see FundingGapOrSurplus.
	RequiredEquity Value `json:"required_equity"`

	// FundingGapOrSurplus is TotalSources.Amount - TotalUses.Amount:
	// positive means the deal's sources (including BuyerEquity) exceed
	// its uses (a surplus), negative means sources fall short (a
	// funding gap that must be closed with additional equity, debt, or
	// a lower purchase price). Available only when both TotalSources
	// and TotalUses are available.
	FundingGapOrSurplus Value `json:"funding_gap_or_surplus"`
}

// FinancingPercentages expresses each funding source as a fraction of
// TotalSources, for display alongside SourcesAndUses.
type FinancingPercentages struct {
	// DebtPercent is TotalDebtFinancing / TotalSources.
	DebtPercent Value `json:"debt_percent"`
	// SellerNotePercent is SellerNoteAmount / TotalSources.
	SellerNotePercent Value `json:"seller_note_percent"`
	// EarnoutPercent is TotalEarnoutAmount / TotalSources.
	EarnoutPercent Value `json:"earnout_percent"`
	// EquityPercent is BuyerEquity / TotalSources.
	EquityPercent Value `json:"equity_percent"`
}

// Result is the output of Build: sources and uses, financing
// percentages, every tranche's amortization schedule (including the
// seller note), aggregated annual debt service, the earnout schedule,
// the funding gap/surplus, issues, and formula version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Build could not proceed at all (no
	// purchase price, no buyer equity, no debt tranches, no seller
	// note, and no earnout supplied at all) — every other field is then
	// zero-value. In every other case Build computes whatever subset of
	// the analysis the supplied input supports and reports gaps via
	// Warnings/Errors and per-field Value.Available.
	Available bool `json:"available"`

	// SourcesAndUses holds the deal's funding sources/uses, required
	// equity, and funding gap/surplus.
	SourcesAndUses SourcesAndUses `json:"sources_and_uses"`
	// FinancingPercentages holds each funding source's share of total
	// sources.
	FinancingPercentages FinancingPercentages `json:"financing_percentages"`

	// DebtSchedules is one AmortizationSchedule per valid entry in
	// Input.DebtTranches, in the same order.
	DebtSchedules []AmortizationSchedule `json:"debt_schedules,omitempty"`
	// SellerNoteSchedule is the seller note's AmortizationSchedule, only
	// present when Input.SellerNote.Included and its terms are valid.
	SellerNoteSchedule *AmortizationSchedule `json:"seller_note_schedule,omitempty"`

	// AnnualDebtService is one entry per year across the combined life
	// of every included tranche (DebtTranches + SellerNote), aggregating
	// principal/interest/balloon across tranches for that year, in
	// ascending Year order. The number of years spans the longest
	// tranche's TermYears.
	AnnualDebtService []AnnualDebtServicePeriod `json:"annual_debt_service,omitempty"`

	// EarnoutSchedule echoes every valid Input.Earnout.Payments entry,
	// sorted by PeriodNumber (ties broken by input order), only present
	// when Input.Earnout.Included.
	EarnoutSchedule []EarnoutPayment `json:"earnout_schedule,omitempty"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
