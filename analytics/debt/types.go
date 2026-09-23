// Package debt analyzes debt service coverage, leverage, and caller-defined
// debt capacity for a single business/borrower — the analysis a lender or
// advisor performs to answer "how much debt can this cash flow support,"
// not "will a lender approve this loan." Nothing in this package's output
// is, or should be read as, a credit decision, a commitment to lend, or a
// substitute for a lender's own underwriting policy.
//
// This package is deliberately independent of financial.FinancialDataset
// and the financial.Code taxonomy, the same design analytics/concentration
// already established for a domain whose inputs frequently do not come
// from a normalized financial statement at all: a lender-supplied term
// sheet, a caller's own normalized/adjusted EBITDA figure (already run
// through financial/metrics and financial/adjustments upstream), or a
// standalone "what could this business support" what-if calculation with
// no financial.FinancialDataset in the loop at all. A caller who has a
// financial.FinancialDataset computes EBITDA via financial/metrics (and
// normalizes it via financial/adjustments) and passes the resulting figure
// into Input.EBITDA — this package never recomputes EBITDA itself and
// never requires a dataset to do so.
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package debt

// FormulaVersion identifies this package's fixed formula set: the
// amortization/payment formula, annual-debt-service aggregation, DSCR/
// fixed-charge-coverage/leverage/interest-coverage formulas, the
// maximum-debt-under-DSCR and maximum-debt-under-leverage solvers, the
// combined-capacity (most-restrictive-constraint) rule, and the downside
// scenario methodology. Bump this whenever any of that changes in a way
// that could make a historical Result not reproduce identically under new
// code — see the repository README's versioning-strategy section, which
// this constant follows exactly (financial.TaxonomyVersion,
// metrics.FormulaVersion, cashflow.FormulaVersion,
// concentration.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown because
// a required input was absent" — the same availability convention
// metrics.MetricValue/cashflow.CashFlowValue/concentration's sibling types
// all use, duplicated here as its own type per this repository's
// established convention (see cashflow.CashFlowValue's doc comment) rather
// than importing financial/metrics into a package that otherwise has no
// dependency on it.
type Value struct {
	// Available is true if Value is meaningful.
	Available bool `json:"available"`
	// Amount is the figure itself. Meaningful only when Available is true;
	// always 0 when Available is false.
	Amount float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// Available reports a Value for a known figure (which may legitimately be
// zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// PaymentFrequency is how often a loan's scheduled payments occur.
type PaymentFrequency string

const (
	FrequencyMonthly   PaymentFrequency = "monthly"
	FrequencyQuarterly PaymentFrequency = "quarterly"
	FrequencyAnnual    PaymentFrequency = "annual"
)

// paymentsPerYear returns the number of scheduled payments per year for f,
// and false if f is not one of the recognized PaymentFrequency values.
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

// LoanTerms describes one proposed or existing loan's amortization
// schedule, sufficient to derive its annual debt service without the
// caller pre-computing a payment schedule itself.
type LoanTerms struct {
	// Label identifies this loan for display (e.g. "Proposed acquisition
	// term loan", "Existing equipment note"). Optional.
	Label string `json:"label,omitempty"`
	// Principal is the original (or, for an existing loan being analyzed
	// mid-life, current outstanding) loan amount. Must be >= 0.
	Principal float64 `json:"principal"`
	// AnnualInterestRate is the nominal annual interest rate as a decimal
	// (0.08 for 8%). Must be >= 0; a rate above 1.0 (100%) is accepted but
	// flagged as an Issue since it is almost certainly a units mistake
	// (e.g. "8" instead of "0.08").
	AnnualInterestRate float64 `json:"annual_interest_rate"`
	// AmortizationYears is the number of years over which Principal fully
	// amortizes to zero (excluding any InterestOnlyYears — see that
	// field's doc comment for how the two combine). Must be > 0.
	AmortizationYears float64 `json:"amortization_years"`
	// Frequency is how often payments occur. Must be one of the
	// PaymentFrequency constants.
	Frequency PaymentFrequency `json:"frequency"`
	// InterestOnlyYears is the number of years, at the start of the loan,
	// during which only interest is paid (no principal amortization).
	// AmortizationYears still governs the amortization schedule used to
	// size the principal-and-interest payment once amortization begins —
	// i.e. a 10-year loan with a 2-year interest-only period amortizes the
	// full Principal over AmortizationYears (10) once amortization starts,
	// not over the remaining 8 years; this mirrors standard commercial
	// lending practice where the I/O period defers amortization without
	// shortening it. Zero means no interest-only period.
	InterestOnlyYears float64 `json:"interest_only_years,omitempty"`
}

// AmortizationSchedule is the derived per-payment and annualized debt
// service for one LoanTerms, computed by Amortize.
type AmortizationSchedule struct {
	// Terms echoes the LoanTerms this schedule was derived from.
	Terms LoanTerms `json:"terms"`
	// PaymentsPerYear is the resolved payment count implied by
	// Terms.Frequency.
	PaymentsPerYear int `json:"payments_per_year"`
	// PeriodicPrincipalAndInterestPayment is the level payment amount once
	// amortization begins (after any InterestOnlyYears), covering both
	// principal and interest each period.
	PeriodicPrincipalAndInterestPayment float64 `json:"periodic_principal_and_interest_payment"`
	// PeriodicInterestOnlyPayment is the payment amount during the
	// InterestOnlyYears period (Terms.Principal x periodic rate). Zero if
	// Terms.InterestOnlyYears is zero.
	PeriodicInterestOnlyPayment float64 `json:"periodic_interest_only_payment,omitempty"`
	// FirstYearAnnualDebtService is the total principal + interest paid in
	// the first 12 months, accounting for whether that first year falls
	// inside the interest-only period, straddles the transition, or is
	// already in full amortization — see BuildSchedule for the exact
	// year-by-year logic. This is the figure AnnualDebtService uses.
	FirstYearAnnualDebtService float64 `json:"first_year_annual_debt_service"`
	// SteadyStateAnnualDebtService is the total principal + interest paid
	// in a full year once amortization is underway (PeriodicPrincipalAndInterestPayment
	// x PaymentsPerYear) — the figure a caller would expect in a typical
	// year after any interest-only period ends.
	SteadyStateAnnualDebtService float64 `json:"steady_state_annual_debt_service"`
}

// LenderPolicy is an optional set of caller-supplied covenant thresholds a
// lender might apply. Purely descriptive here — this package never claims
// that meeting these thresholds means a lender will approve anything (see
// the package doc comment); it only echoes the policy alongside the
// computed figures so a caller can compare them side by side.
type LenderPolicy struct {
	// MinimumDSCR is the lowest debt-service coverage ratio the policy
	// requires. Zero means not specified.
	MinimumDSCR float64 `json:"minimum_dscr,omitempty"`
	// MaximumDebtToEBITDA is the highest total-debt-to-EBITDA multiple the
	// policy allows. Zero means not specified.
	MaximumDebtToEBITDA float64 `json:"maximum_debt_to_ebitda,omitempty"`
	// MaximumNetDebtToEBITDA is the highest net-debt-to-EBITDA multiple the
	// policy allows. Zero means not specified.
	MaximumNetDebtToEBITDA float64 `json:"maximum_net_debt_to_ebitda,omitempty"`
	// MinimumFixedChargeCoverage is the lowest fixed-charge coverage ratio
	// the policy requires. Zero means not specified.
	MinimumFixedChargeCoverage float64 `json:"minimum_fixed_charge_coverage,omitempty"`
}

// FixedChargeInputs supplies the additional obligations (beyond debt
// service itself) a fixed-charge coverage ratio conventionally includes.
// Fixed-charge coverage is computed only when at least one of these is
// supplied — see FixedChargeCoverage.Available.
type FixedChargeInputs struct {
	// LeasePayments is annual operating lease/rent payments treated as a
	// fixed charge.
	LeasePayments Value `json:"lease_payments"`
	// CurrentPortionLongTermDebt is annual scheduled principal payments on
	// debt not already counted in Input.ExistingDebt (e.g. a capital lease
	// obligation reported separately). Optional; most callers leave this
	// unavailable and let ExistingDebt/ProposedLoans cover all debt
	// principal.
	CurrentPortionLongTermDebt Value `json:"current_portion_long_term_debt"`
	// CashTaxes is annual cash income taxes paid, when a caller wants a
	// post-tax fixed-charge coverage figure. Optional.
	CashTaxes Value `json:"cash_taxes"`
	// CapitalExpenditures is annual maintenance/growth capex a caller wants
	// deducted before assessing fixed-charge coverage. Optional.
	CapitalExpenditures Value `json:"capital_expenditures"`
	// UnfinancedCapex, if true, means CapitalExpenditures (when available)
	// should be subtracted from the fixed-charge coverage numerator — a
	// common lender convention when capex is not itself debt-financed.
	UnfinancedCapex bool `json:"unfinanced_capex,omitempty"`
}

// DownsideScenario is one caller-defined stress case applied to
// Input.EBITDA (and, if supplied, Input.CashFlow) to see how coverage
// holds up under worse conditions than the base case.
type DownsideScenario struct {
	// Label identifies this scenario (e.g. "Revenue -15%", "Recession
	// case"). Required for a meaningful ScenarioResult.
	Label string `json:"label"`
	// EBITDAHaircutPercent is the fractional reduction applied to
	// Input.EBITDA (0.15 means EBITDA is reduced by 15%). May be negative
	// to model an upside case instead. Applied only if Input.EBITDA is
	// available.
	EBITDAHaircutPercent float64 `json:"ebitda_haircut_percent"`
	// CashFlowHaircutPercent is the fractional reduction applied to
	// Input.CashFlow, independent of EBITDAHaircutPercent (a caller may
	// stress EBITDA and cash flow by different amounts, e.g. if a working
	// capital build is assumed under the downside case). Applied only if
	// Input.CashFlow is available; if Input.CashFlow is not supplied,
	// DSCR under this scenario falls back to the EBITDA-based coverage
	// measure exactly as the base case does — see CoverageResult's doc
	// comment.
	CashFlowHaircutPercent float64 `json:"cash_flow_haircut_percent"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// EBITDA is the normalized/maintainable EBITDA (or SDE, for a
	// sole-proprietor-style business) to evaluate coverage and leverage
	// against — typically derived upstream via financial/metrics and
	// financial/adjustments, but supplied here as a plain figure since
	// this package has no financial.FinancialDataset dependency (see the
	// package doc comment). Required for every coverage/leverage
	// calculation; a Result with EBITDA.Available == false still computes
	// whatever loan-terms-only figures (annual debt service, amortization
	// schedules) don't need it, with everything else left unavailable and
	// an Issue recorded.
	EBITDA Value `json:"ebitda"`
	// CashFlow is an optional caller-supplied cash-flow figure (e.g.
	// operating cash flow or free cash flow from analytics/cashflow) to
	// use as DSCR's numerator instead of EBITDA, when a caller considers
	// cash flow the more accurate coverage measure. If unavailable, DSCR
	// and all other coverage/capacity figures fall back to EBITDA — see
	// CoverageResult.NumeratorSource.
	CashFlow Value `json:"cash_flow"`

	// ExistingDebt is the business's existing loans (already outstanding),
	// each described by LoanTerms so this package derives their annual
	// debt service directly, rather than requiring the caller to
	// pre-compute a payment schedule itself.
	ExistingDebt []LoanTerms `json:"existing_debt,omitempty"`
	// ProposedLoans is one or more new loans being evaluated for capacity
	// (e.g. a proposed acquisition term loan). Kept separate from
	// ExistingDebt so Result can report pre- and post-transaction coverage
	// distinctly.
	ProposedLoans []LoanTerms `json:"proposed_loans,omitempty"`

	// ExistingDebtBalance is the total outstanding principal balance across
	// all debt (existing and, if applicable, proposed), used for the
	// Debt/EBITDA and Net-Debt/EBITDA leverage ratios. If zero-value and
	// ExistingDebt/ProposedLoans is non-empty, this package sums
	// LoanTerms.Principal across both slices instead — see
	// resolveTotalDebtBalance. Supplying this explicitly is preferred when
	// the caller has a more precise current balance than the original
	// LoanTerms.Principal (e.g. a partially amortized existing loan).
	ExistingDebtBalance Value `json:"existing_debt_balance"`
	// CashAndEquivalents is unrestricted cash available to offset gross
	// debt for the Net-Debt/EBITDA ratio. Optional; if unavailable,
	// NetDebtToEBITDA is left unavailable.
	CashAndEquivalents Value `json:"cash_and_equivalents"`
	// InterestExpense is total annual interest expense across all debt,
	// used for InterestCoverage. If zero-value, this package sums the
	// interest component of every AmortizationSchedule's
	// FirstYearAnnualDebtService instead — see resolveInterestExpense.
	InterestExpense Value `json:"interest_expense"`

	// FixedCharges supplies the additional obligations a fixed-charge
	// coverage ratio includes beyond debt service. Optional; if every
	// field is unavailable, FixedChargeCoverage is left unavailable.
	FixedCharges FixedChargeInputs `json:"fixed_charges"`

	// Policy is an optional lender policy to compare computed figures
	// against and to drive the maximum-debt-under-minimum-DSCR /
	// maximum-debt-under-leverage-cap calculations. If Policy.MinimumDSCR
	// or Policy.MaximumDebtToEBITDA/MaximumNetDebtToEBITDA is zero, the
	// corresponding capacity calculation is skipped (left unavailable)
	// rather than computed against an assumed cap this package invented.
	Policy LenderPolicy `json:"policy"`

	// DownsideScenarios is zero or more caller-defined stress cases to
	// evaluate coverage under, in addition to the base case. Evaluated in
	// the order supplied.
	DownsideScenarios []DownsideScenario `json:"downside_scenarios,omitempty"`
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model every
// sibling analytics package uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository, rather than reusing one of
// theirs.
type IssueCode string

const (
	// IssueNoEBITDA means Input.EBITDA was unavailable, so every
	// EBITDA-based coverage, leverage, and capacity figure is unavailable.
	// Loan-terms-only figures (amortization, annual debt service) are
	// still computed.
	IssueNoEBITDA IssueCode = "NO_EBITDA"
	// IssueNegativeEBITDA means Input.EBITDA is available but <= 0.
	// Coverage/leverage ratios that divide by EBITDA are left unavailable
	// (dividing a positive debt-service figure by a non-positive earnings
	// figure produces a mathematically defined but practically meaningless
	// ratio) rather than returned as a negative or infinite DSCR.
	IssueNegativeEBITDA IssueCode = "NEGATIVE_EBITDA"
	// IssueInvalidLoanTerms means at least one LoanTerms had a
	// non-positive AmortizationYears, a negative Principal, a negative
	// AnnualInterestRate, an unrecognized Frequency, or
	// InterestOnlyYears >= AmortizationYears. That specific loan is
	// excluded from every downstream calculation.
	IssueInvalidLoanTerms IssueCode = "INVALID_LOAN_TERMS"
	// IssueSuspiciousInterestRate means a LoanTerms.AnnualInterestRate is
	// above 1.0 (100%), which almost always indicates the rate was
	// supplied as a whole-number percent (e.g. 8) rather than a decimal
	// (0.08). The loan is still processed using the rate as supplied.
	IssueSuspiciousInterestRate IssueCode = "SUSPICIOUS_INTEREST_RATE"
	// IssueNoDebt means neither ExistingDebt, ProposedLoans, nor
	// ExistingDebtBalance supplied any debt, so every debt-service,
	// coverage, and leverage figure that requires a debt amount reports a
	// zero/perfect-coverage result rather than being left unavailable —
	// see Result's field docs for exactly which figures this affects.
	// Advisory only.
	IssueNoDebt IssueCode = "NO_DEBT"
	// IssueNoLenderPolicy means Input.Policy had no non-zero threshold, so
	// every maximum-debt-under-constraint calculation was skipped.
	// Advisory only.
	IssueNoLenderPolicy IssueCode = "NO_LENDER_POLICY"
)

// Issue is a single Calculate-time input finding.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// Loan identifies, by index into the originating Input slice (e.g.
	// "existing_debt[0]" or "proposed_loans[1]"), which loan an
	// Issue relates to. Empty when the Issue is not loan-specific.
	Loan string `json:"loan,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from cashflow.HasErrors/concentration.HasErrors
// and this repository's other sibling HasErrors functions rather than
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

// CoverageNumeratorSource identifies which Input figure a CoverageResult's
// DSCR numerator was computed from.
type CoverageNumeratorSource string

const (
	// CoverageSourceCashFlow means Input.CashFlow was available and used.
	CoverageSourceCashFlow CoverageNumeratorSource = "cash_flow"
	// CoverageSourceEBITDA means Input.CashFlow was unavailable (or a
	// scenario's cash-flow figure could not be derived), so Input.EBITDA
	// was used instead.
	CoverageSourceEBITDA CoverageNumeratorSource = "ebitda"
	// CoverageSourceUnavailable means neither was available.
	CoverageSourceUnavailable CoverageNumeratorSource = "unavailable"
)

// CoverageResult holds one set of coverage/leverage figures — used both for
// Result.BaseCase and for each ScenarioResult.
type CoverageResult struct {
	// NumeratorSource identifies which figure DSCR/InterestCoverage were
	// computed from — see CoverageNumeratorSource.
	NumeratorSource CoverageNumeratorSource `json:"numerator_source"`
	// CoverageNumerator is the actual figure used (Input.CashFlow.Amount or
	// Input.EBITDA.Amount, after any scenario haircut), mirroring
	// NumeratorSource.
	CoverageNumerator Value `json:"coverage_numerator"`

	// AnnualDebtService is the sum of every included loan's
	// AmortizationSchedule.FirstYearAnnualDebtService.
	AnnualDebtService Value `json:"annual_debt_service"`
	// DSCR is CoverageNumerator.Amount / AnnualDebtService.Amount.
	// Available only when both are available; if AnnualDebtService.Amount
	// == 0 (IssueNoDebt), DSCR is left unavailable since coverage of zero
	// debt service is not a meaningful ratio — see Result.Warnings for
	// IssueNoDebt instead.
	DSCR Value `json:"dscr"`

	// FixedChargeCoverage is (CoverageNumerator - CashTaxes -
	// UnfinancedCapex) + LeasePayments, all divided by (AnnualDebtService +
	// CurrentPortionLongTermDebt + LeasePayments). Available only when
	// CoverageNumerator, AnnualDebtService, and at least one
	// FixedChargeInputs figure are available — see the package README
	// section for the exact formula.
	FixedChargeCoverage Value `json:"fixed_charge_coverage"`

	// InterestCoverage is CoverageNumerator.Amount /
	// Input.InterestExpense.Amount (or the derived interest total — see
	// Input.InterestExpense's doc comment). Available only when both are
	// available and the denominator is nonzero.
	InterestCoverage Value `json:"interest_coverage"`

	// TotalDebtBalance is the resolved total outstanding principal used for
	// leverage ratios — see Input.ExistingDebtBalance's doc comment.
	TotalDebtBalance Value `json:"total_debt_balance"`
	// DebtToEBITDA is TotalDebtBalance.Amount / EBITDA (the scenario's
	// EBITDA, not CoverageNumerator — leverage is conventionally measured
	// against EBITDA specifically, never a cash-flow figure). Available
	// only when both are available and EBITDA is strictly positive — a
	// leverage multiple against a zero or negative EBITDA is not a
	// meaningful ratio (mirroring IssueNegativeEBITDA's rationale).
	DebtToEBITDA Value `json:"debt_to_ebitda"`
	// NetDebtToEBITDA is (TotalDebtBalance.Amount -
	// Input.CashAndEquivalents.Amount) / EBITDA. Available only when
	// TotalDebtBalance, CashAndEquivalents, and EBITDA are all available
	// and EBITDA is strictly positive.
	NetDebtToEBITDA Value `json:"net_debt_to_ebitda"`
}

// LimitingConstraintCode identifies which single constraint most restricts
// MaximumCapacity.CombinedMaximumDebt.
type LimitingConstraintCode string

const (
	// LimitDSCR means the minimum-DSCR-implied maximum debt was the
	// smaller (more restrictive) of the two, or the leverage-cap figure was
	// unavailable.
	LimitDSCR LimitingConstraintCode = "DSCR"
	// LimitLeverage means the leverage-cap-implied maximum debt was the
	// smaller (more restrictive) of the two, or the DSCR-implied figure was
	// unavailable.
	LimitLeverage LimitingConstraintCode = "LEVERAGE"
	// LimitNone means neither constraint was available (no Policy
	// threshold supplied), so no combined capacity could be computed.
	LimitNone LimitingConstraintCode = "NONE"
)

// MaximumCapacity is how much additional/total debt the business could
// support under Input.Policy's thresholds, computed at Input's proposed
// loan pricing/terms (see capacity.go for the exact solve). This describes
// mathematical capacity under the supplied assumptions only — see the
// package doc comment: it is never a statement that a lender would approve
// this amount.
type MaximumCapacity struct {
	// PricingTerms is the LoanTerms (principal ignored) used to translate a
	// maximum annual debt service into a maximum principal — the interest
	// rate, amortization, frequency, and interest-only period a new loan
	// at this capacity is assumed to carry. Taken from the first entry in
	// Input.ProposedLoans, or Input.ExistingDebt if ProposedLoans is
	// empty; unavailable (zero-value) if neither is supplied.
	PricingTerms LoanTerms `json:"pricing_terms"`
	// HasPricingTerms is true if PricingTerms was resolved from Input.
	HasPricingTerms bool `json:"has_pricing_terms"`

	// MaxDebtUnderDSCR is the total debt balance (at PricingTerms pricing)
	// whose annual debt service exactly equals CoverageNumerator /
	// Policy.MinimumDSCR. Available only when CoverageNumerator,
	// Policy.MinimumDSCR (nonzero), and HasPricingTerms are all available.
	MaxDebtUnderDSCR Value `json:"max_debt_under_dscr"`
	// MaxDebtUnderLeverage is EBITDA x Policy.MaximumDebtToEBITDA (or, if
	// Policy.MaximumNetDebtToEBITDA is also set, the net-debt-cap-implied
	// gross debt figure after adding back CashAndEquivalents — see
	// capacity.go). Available only when EBITDA and at least one leverage
	// cap are available.
	MaxDebtUnderLeverage Value `json:"max_debt_under_leverage"`

	// CombinedMaximumDebt is the smaller (more restrictive) of
	// MaxDebtUnderDSCR and MaxDebtUnderLeverage when both are available;
	// whichever one is available when only one is; unavailable if neither
	// is.
	CombinedMaximumDebt Value `json:"combined_maximum_debt"`
	// LimitingConstraint identifies which constraint CombinedMaximumDebt
	// came from.
	LimitingConstraint LimitingConstraintCode `json:"limiting_constraint"`

	// Headroom is CombinedMaximumDebt.Amount - TotalDebtBalance (the
	// resolved current total debt, existing + proposed) — how much
	// additional debt capacity remains (negative means the business is
	// already over the combined cap). Available only when
	// CombinedMaximumDebt and the current total debt balance are both
	// available.
	Headroom Value `json:"headroom"`
}

// ScenarioResult is one DownsideScenario's coverage figures, computed by
// applying that scenario's haircuts to Input.EBITDA/Input.CashFlow before
// recomputing CoverageResult, with debt service and leverage held fixed
// (a downside scenario stresses earnings, not the debt structure itself).
type ScenarioResult struct {
	// Scenario echoes the DownsideScenario this result was computed for.
	Scenario DownsideScenario `json:"scenario"`
	// Coverage is the resulting coverage/leverage figures under this
	// scenario's stressed EBITDA/cash flow.
	Coverage CoverageResult `json:"coverage"`
	// BreachesMinimumDSCR is true when Input.Policy.MinimumDSCR is set,
	// Coverage.DSCR is available, and Coverage.DSCR is below that minimum.
	// False (not just "unavailable") when Policy.MinimumDSCR is unset,
	// since there is no threshold to breach.
	BreachesMinimumDSCR bool `json:"breaches_minimum_dscr"`
}

// FlagCode is a stable identifier for one kind of deterministic debt-
// capacity signal, analogous to cashflow.FlagCode/qoe.FlagCode.
type FlagCode string

const (
	// FlagBelowMinimumDSCR means Result.BaseCase.DSCR is available and
	// below Input.Policy.MinimumDSCR (when set).
	FlagBelowMinimumDSCR FlagCode = "BELOW_MINIMUM_DSCR"
	// FlagAboveLeverageCap means Result.BaseCase.DebtToEBITDA (or
	// NetDebtToEBITDA, when the corresponding cap is the one set) exceeds
	// Input.Policy's leverage cap.
	FlagAboveLeverageCap FlagCode = "ABOVE_LEVERAGE_CAP"
	// FlagBelowMinimumFixedChargeCoverage means
	// Result.BaseCase.FixedChargeCoverage is available and below
	// Input.Policy.MinimumFixedChargeCoverage (when set).
	FlagBelowMinimumFixedChargeCoverage FlagCode = "BELOW_MINIMUM_FIXED_CHARGE_COVERAGE"
	// FlagNegativeHeadroom means MaximumCapacity.Headroom is available and
	// negative — current debt already exceeds the combined capacity limit.
	FlagNegativeHeadroom FlagCode = "NEGATIVE_HEADROOM"
	// FlagScenarioBreachesDSCR means at least one ScenarioResult has
	// BreachesMinimumDSCR == true.
	FlagScenarioBreachesDSCR FlagCode = "SCENARIO_BREACHES_DSCR"
	// FlagNoDebtService means IssueNoDebt was recorded — every
	// coverage ratio trivially shows no debt burden, which is worth
	// surfacing as a flag (not just a Warnings entry) since a caller
	// scanning Flags alone should not miss it.
	FlagNoDebtService FlagCode = "NO_DEBT_SERVICE"
)

// FlagSeverity mirrors cashflow.FlagSeverity/qoe.FlagSeverity's role: a
// structured, matchable urgency signal, never inferred from Message text.
type FlagSeverity string

const (
	FlagSeverityInfo     FlagSeverity = "info"
	FlagSeverityWarning  FlagSeverity = "warning"
	FlagSeverityCritical FlagSeverity = "critical"
)

// Flag is a single deterministic, explainable debt-capacity signal. Every
// Flag here is rule-based against Input.Policy or fixed rules, never
// AI-scored.
type Flag struct {
	Code      FlagCode     `json:"code"`
	Severity  FlagSeverity `json:"severity"`
	Scenario  string       `json:"scenario,omitempty"`
	Message   string       `json:"message"`
	Value     float64      `json:"value"`
	Threshold float64      `json:"threshold"`
}

// Result is the output of Calculate: amortization schedules, base-case
// coverage/leverage, maximum capacity under Input.Policy, downside
// scenarios, deterministic flags, and formula version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (no
	// EBITDA, no cash flow, and no loans/debt balance supplied at all) —
	// every other field is then zero-value. In every other case Calculate
	// computes whatever subset of the analysis the supplied input
	// supports and reports gaps via Warnings/Errors and per-field
	// Value.Available.
	Available bool `json:"available"`

	// ExistingSchedules is one AmortizationSchedule per valid entry in
	// Input.ExistingDebt, in the same order.
	ExistingSchedules []AmortizationSchedule `json:"existing_schedules,omitempty"`
	// ProposedSchedules is one AmortizationSchedule per valid entry in
	// Input.ProposedLoans, in the same order.
	ProposedSchedules []AmortizationSchedule `json:"proposed_schedules,omitempty"`

	// BaseCase is the coverage/leverage analysis at Input's actual EBITDA/
	// cash flow and actual debt (existing + proposed), with no haircut
	// applied.
	BaseCase CoverageResult `json:"base_case"`
	// ExistingOnlyCase is the same analysis using only Input.ExistingDebt
	// (excluding Input.ProposedLoans) — the "before this transaction"
	// coverage picture, so a caller can see the incremental effect of the
	// proposed debt. Identical to BaseCase when Input.ProposedLoans is
	// empty.
	ExistingOnlyCase CoverageResult `json:"existing_only_case"`

	// Capacity is the maximum-debt-under-constraint analysis. Zero-value
	// (every field unavailable) if Input.Policy has no thresholds set.
	Capacity MaximumCapacity `json:"capacity"`

	// Scenarios is one ScenarioResult per Input.DownsideScenarios entry,
	// in the same order.
	Scenarios []ScenarioResult `json:"scenarios,omitempty"`

	// Flags is every deterministic signal Calculate triggered, ordered by
	// FlagCode's declaration order, then by Scenario.
	Flags []Flag `json:"flags,omitempty"`

	// Policy echoes Input.Policy this Result was computed under.
	Policy LenderPolicy `json:"policy"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
