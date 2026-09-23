// Package forecast generates deterministic financial projections and
// scenario sets from caller-supplied assumptions.
//
// This package does not predict assumptions — it applies them. Every growth
// rate, margin target, expense change, capex figure, and debt-service term
// is supplied by the caller (a human analyst's judgment, an FP&A model, a
// board-approved plan); Calculate only compounds those assumptions forward
// from a historical base and reports the resulting P&L, EBITDA, SDE,
// margins, working capital, cash flow, and coverage figures. This mirrors
// valuation/dcf's identical "the caller forecasts, this package only
// computes" boundary — dcf discounts a caller-supplied cash-flow series;
// this package is the natural upstream complement that can produce that
// series (via Result.ScenarioResults[i].CashFlow.FreeCashFlow) from
// assumptions, but the two remain independent: this package never calls
// into valuation/dcf, and a caller is free to feed this package's projected
// free cash flow into dcf.Input.ForecastPeriods itself.
//
// The historical base comes from a normalized financial.FinancialDataset
// (reusing this repository's canonical taxonomy and PeriodInfo-based
// chronological-ordering convention — see financial/metrics,
// analytics/workingcapital, analytics/cashflow), read-only: Calculate never
// mutates Input.Dataset. Every forecast period's projected P&L line is keyed
// by financial.Code, so EBITDA/SDE/margins reuse this repository's one
// canonical set of code buckets (financial.CodesByCategory) rather than a
// second, forecast-specific taxonomy.
//
// A caller defines one or more named Scenario values (base, downside,
// upside, or a custom label — see ScenarioType) each carrying its own
// Assumptions; Calculate produces one ScenarioResult per Scenario, all
// compounding forward from the identical historical base, so scenarios
// differ only in the assumptions applied, never in the starting point.
//
// Every function here is pure: no I/O, no mutation of caller-owned input, no
// package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package forecast

import (
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
)

// FormulaVersion identifies this package's fixed formula set: the
// historical-base derivation, the per-period compounding rule (revenue,
// COGS/gross margin, opex, D&A, EBITDA, SDE, margins), the working-capital
// and cash-flow bridge, the debt-service-coverage formula, and every
// scenario-transformation helper's exact arithmetic. Bump this whenever any
// of that changes in a way that could make a historical Result not
// reproduce identically under new code — see the repository README's
// versioning-strategy section, which this constant follows exactly
// (financial.TaxonomyVersion, metrics.FormulaVersion,
// workingcapital.FormulaVersion, cashflow.FormulaVersion,
// variance.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// PeriodType mirrors metrics.PeriodType/workingcapital.PeriodType/
// cashflow.PeriodType/variance.PeriodType (fiscal year, YTD, quarter,
// month), duplicated here rather than aliased so this package's own doc
// comments apply directly at the call site — the same choice every
// analytics sibling package already made for its own PeriodType.
type PeriodType string

const (
	PeriodTypeFiscalYear PeriodType = "fiscal_year"
	PeriodTypeYTD        PeriodType = "ytd"
	PeriodTypeQuarter    PeriodType = "quarter"
	PeriodTypeMonth      PeriodType = "month"
)

// PeriodInfo is caller-supplied metadata about a single financial.Period in
// Input.Dataset, used to determine chronological order so this package can
// identify the most recent historical period as the base the first forecast
// period compounds from. financial.Period is intentionally just a string
// with no guaranteed sort order; this package requires PeriodInfo rather
// than inferring order or granularity from the string, mirroring every
// analytics sibling package's identical no-guessing rule.
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

// ForecastValue represents a single projected or historical figure that may
// or may not be calculable, mirroring metrics.MetricValue/
// cashflow.CashFlowValue's identical availability convention: Available
// distinguishes "computed to be exactly $0" from "cannot be computed
// because a required input is absent."
type ForecastValue struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

// Unavailable is the canonical zero-information ForecastValue.
func Unavailable() ForecastValue { return ForecastValue{} }

// AvailableValue reports a ForecastValue for a successfully computed figure.
func AvailableValue(v float64) ForecastValue { return ForecastValue{Available: true, Value: v} }

// revenueCodes mirrors financial/metrics' own total-revenue definition (see
// metrics' unexported revenueCodes). Duplicated here — rather than
// depending on financial/metrics for the list — since this package needs
// only the code list to drive per-code projection, not metrics' full
// Snapshot machinery, mirroring analytics/workingcapital's identical
// "duplicate the list, don't force the dependency" choice.
var revenueCodes = []financial.Code{
	financial.CodeRevProduct,
	financial.CodeRevService,
	financial.CodeRevRecurring,
	financial.CodeRevOther,
}

// cogsCodes mirrors financial/metrics' own total-COGS code list.
var cogsCodes = []financial.Code{
	financial.CodeCogsMaterial,
	financial.CodeCogsDirectLabor,
	financial.CodeCogsFreight,
	financial.CodeCogsOther,
}

// opexCodes mirrors financial/metrics' own total-opex code list (including
// owner compensation, exactly as metrics.totalOpex does — see that
// function's doc comment for why owner comp is a real operating expense
// even though SDE separately adds it back).
var opexCodes = []financial.Code{
	financial.CodeOpexPayroll,
	financial.CodeOpexOwnerComp,
	financial.CodeOpexRent,
	financial.CodeOpexMarketing,
	financial.CodeOpexInsurance,
	financial.CodeOpexUtilities,
	financial.CodeOpexSoftware,
	financial.CodeOpexProfessionalFees,
	financial.CodeOpexRepairs,
	financial.CodeOpexVehicle,
	financial.CodeOpexTravel,
	financial.CodeOpexOffice,
	financial.CodeOpexOther,
}

// RevenueMethod selects how a forecast period's total revenue is derived
// from the prior period's total revenue.
type RevenueMethod string

const (
	// RevenueMethodGrowthRate multiplies the prior period's total revenue by
	// (1 + growth rate). The default when a RevenuePeriodAssumption leaves
	// Method empty.
	RevenueMethodGrowthRate RevenueMethod = "growth_rate"
	// RevenueMethodFixedAmount replaces the prior period's total revenue
	// with an explicit caller-supplied dollar figure for this period,
	// ignoring compounding entirely — for a caller with an external
	// bottoms-up revenue forecast it wants applied verbatim.
	RevenueMethodFixedAmount RevenueMethod = "fixed_amount"
)

// RevenueCodeAssumption overrides RevenuePeriodAssumption's aggregate
// growth rate for one specific financial.Code, taking precedence over the
// aggregate rate for that code alone — mirroring variance.Policy.
// DirectionOverrides' identical "aggregate default + per-code override"
// precedence rule. Every revenue code without an override still grows at
// the aggregate RevenuePeriodAssumption.GrowthRate, so the total-revenue
// figure is always the sum of every code's own projection, never
// recomputed independently from the aggregate rate.
type RevenueCodeAssumption struct {
	Code financial.Code `json:"code"`
	// Method selects how this code's amount is derived; empty defaults to
	// RevenueMethodGrowthRate.
	Method RevenueMethod `json:"method,omitempty"`
	// GrowthRate is this code's period-over-period growth rate, as a
	// decimal (0.1 = 10%), applied when Method is RevenueMethodGrowthRate.
	GrowthRate float64 `json:"growth_rate,omitempty"`
	// FixedAmount is this code's exact projected amount for the period,
	// applied when Method is RevenueMethodFixedAmount.
	FixedAmount float64 `json:"fixed_amount,omitempty"`
}

// RevenuePeriodAssumption is one forecast period's revenue assumption: an
// aggregate growth rate (or fixed amount) applied to every revenue
// financial.Code without its own CodeOverrides entry, plus any per-code
// overrides.
type RevenuePeriodAssumption struct {
	// Method selects how the aggregate figure is derived; empty defaults to
	// RevenueMethodGrowthRate.
	Method RevenueMethod `json:"method,omitempty"`
	// GrowthRate is the aggregate period-over-period growth rate, as a
	// decimal, applied to every revenue code without its own CodeOverrides
	// entry, when Method is RevenueMethodGrowthRate.
	GrowthRate float64 `json:"growth_rate,omitempty"`
	// FixedAmount is the aggregate total-revenue figure for the period,
	// applied when Method is RevenueMethodFixedAmount and spread across
	// revenue codes proportionally to the base period's mix (see
	// projectRevenue) — codes with their own CodeOverrides entry are
	// excluded from that proportional split and use their override
	// instead.
	FixedAmount float64 `json:"fixed_amount,omitempty"`
	// CodeOverrides pins specific revenue financial.Code values' own
	// method/rate/amount, taking precedence over the aggregate figure above
	// for that code alone. A code listed more than once uses its first
	// entry — deterministic, caller-controlled precedence, never Go map
	// order.
	CodeOverrides []RevenueCodeAssumption `json:"code_overrides,omitempty"`
}

// COGSMethod selects how a forecast period's total cost of goods sold is
// derived.
type COGSMethod string

const (
	// COGSMethodGrossMarginPercent derives COGS as Revenue x (1 - gross
	// margin target), holding the gross margin percentage constant (or at a
	// caller-specified target) rather than growing COGS independently. The
	// default when a COGSPeriodAssumption leaves Method empty.
	COGSMethodGrossMarginPercent COGSMethod = "gross_margin_percent"
	// COGSMethodGrowthRate multiplies the prior period's total COGS by (1 +
	// growth rate), independent of the projected revenue figure.
	COGSMethodGrowthRate COGSMethod = "growth_rate"
	// COGSMethodFixedAmount replaces the prior period's total COGS with an
	// explicit caller-supplied dollar figure for this period.
	COGSMethodFixedAmount COGSMethod = "fixed_amount"
)

// COGSPeriodAssumption is one forecast period's cost-of-goods-sold
// assumption, expressed as one of three mutually exclusive methods (see
// COGSMethod). Unlike RevenuePeriodAssumption, this package does not offer
// a per-financial.Code override for COGS: gross margin is inherently an
// aggregate ratio against total revenue, and COGSMethodGrossMarginPercent's
// result is spread across COGS codes proportionally to the base period's
// mix exactly as RevenueMethodFixedAmount does for revenue (see
// projectCOGS) — a caller needing a genuinely different margin trajectory
// per COGS code supplies it as a distinct Scenario instead.
type COGSPeriodAssumption struct {
	// Method selects how this period's total COGS is derived; empty
	// defaults to COGSMethodGrossMarginPercent.
	Method COGSMethod `json:"method,omitempty"`
	// GrossMarginPercent is the target gross margin (Gross Profit / Total
	// Revenue) for this period, as a decimal (0.4 = 40%), used when Method
	// is COGSMethodGrossMarginPercent. COGS is derived as Revenue x (1 -
	// GrossMarginPercent).
	GrossMarginPercent float64 `json:"gross_margin_percent,omitempty"`
	// GrowthRate is COGS's own period-over-period growth rate, as a
	// decimal, used when Method is COGSMethodGrowthRate.
	GrowthRate float64 `json:"growth_rate,omitempty"`
	// FixedAmount is the exact total COGS figure for the period, used when
	// Method is COGSMethodFixedAmount.
	FixedAmount float64 `json:"fixed_amount,omitempty"`
}

// OpexMethod selects how a forecast period's operating-expense figure
// (aggregate or per-code) is derived.
type OpexMethod string

const (
	// OpexMethodGrowthRate multiplies the prior period's amount by (1 +
	// growth rate). The default when an OpexPeriodAssumption or
	// OpexCodeAssumption leaves Method empty.
	OpexMethodGrowthRate OpexMethod = "growth_rate"
	// OpexMethodFixedAmount replaces the prior period's amount with an
	// explicit caller-supplied dollar figure for this period.
	OpexMethodFixedAmount OpexMethod = "fixed_amount"
	// OpexMethodExcludeAmount grows this code at GrowthRate from the prior
	// period's reported amount minus ExcludeFromBase, rather than from the
	// prior period's full reported amount — used to carry forward the
	// underlying recurring trend of a code whose prior-period figure
	// included a one-time item, without that one-time item permanently
	// inflating every subsequent period's growth base (see
	// ApplyOneTimeCostShock, the transformation helper that sets this
	// method automatically on the period immediately after the one-time
	// item). Only meaningful for a code whose *own* prior-period
	// OpexCodeAssumption used OpexMethodFixedAmount to record the one-time
	// spike; using it in any other circumstance simply subtracts
	// ExcludeFromBase from whatever the prior period actually reported for
	// this code before growing.
	OpexMethodExcludeAmount OpexMethod = "exclude_amount"
)

// OpexCodeAssumption overrides OpexPeriodAssumption's aggregate growth rate
// for one specific operating-expense financial.Code, taking precedence over
// the aggregate rate for that code alone — the same precedence rule
// RevenueCodeAssumption uses for revenue.
type OpexCodeAssumption struct {
	Code financial.Code `json:"code"`
	// Method selects how this code's amount is derived; empty defaults to
	// OpexMethodGrowthRate.
	Method OpexMethod `json:"method,omitempty"`
	// GrowthRate is this code's period-over-period growth rate, as a
	// decimal, applied when Method is OpexMethodGrowthRate or
	// OpexMethodExcludeAmount.
	GrowthRate float64 `json:"growth_rate,omitempty"`
	// FixedAmount is this code's exact projected amount for the period,
	// applied when Method is OpexMethodFixedAmount.
	FixedAmount float64 `json:"fixed_amount,omitempty"`
	// ExcludeFromBase is the dollar amount subtracted from the prior
	// period's reported amount for this code before applying GrowthRate,
	// applied when Method is OpexMethodExcludeAmount. See that constant's
	// doc comment.
	ExcludeFromBase float64 `json:"exclude_from_base,omitempty"`
}

// OpexPeriodAssumption is one forecast period's operating-expense
// assumption: an aggregate growth rate (or fixed amount) applied to every
// opex financial.Code without its own CodeOverrides entry, plus any
// per-code overrides — the same aggregate-plus-override shape
// RevenuePeriodAssumption uses for revenue.
type OpexPeriodAssumption struct {
	// Method selects how the aggregate figure is derived; empty defaults to
	// OpexMethodGrowthRate.
	Method OpexMethod `json:"method,omitempty"`
	// GrowthRate is the aggregate period-over-period growth rate, as a
	// decimal, applied to every opex code without its own CodeOverrides
	// entry, when Method is OpexMethodGrowthRate.
	GrowthRate float64 `json:"growth_rate,omitempty"`
	// FixedAmount is the aggregate total-opex figure for the period,
	// applied when Method is OpexMethodFixedAmount and spread across opex
	// codes proportionally to the base period's mix (mirroring
	// RevenuePeriodAssumption.FixedAmount's identical proportional-split
	// rule) — codes with their own CodeOverrides entry are excluded from
	// that split and use their override instead.
	FixedAmount float64 `json:"fixed_amount,omitempty"`
	// CodeOverrides pins specific opex financial.Code values' own
	// method/rate/amount. A code listed more than once uses its first
	// entry — deterministic, caller-controlled precedence.
	CodeOverrides []OpexCodeAssumption `json:"code_overrides,omitempty"`
}

// DepreciationAmortizationAssumption is one forecast period's depreciation
// and amortization figures, supplied directly by the caller (this package
// never derives D&A from a capex schedule or asset-life assumption — that
// is a distinct, more detailed modeling exercise outside this package's
// scope, per the prompt's "depreciation/amortization where supplied"
// requirement).
type DepreciationAmortizationAssumption struct {
	// Depreciation is this period's depreciation expense. Unavailable
	// (Available == false) means no assumption was supplied; this package
	// leaves the corresponding line unavailable rather than assuming $0 —
	// see projectDA.
	Depreciation ForecastValue `json:"depreciation"`
	// Amortization is this period's amortization expense, under the same
	// availability convention as Depreciation.
	Amortization ForecastValue `json:"amortization"`
}

// CapexAssumption is one forecast period's capital expenditure, supplied
// directly by the caller as a positive outflow magnitude — the same sign
// convention cashflow.Input.Capex uses.
type CapexAssumption struct {
	Capex ForecastValue `json:"capex"`
}

// WorkingCapitalMethod selects how a forecast period's operating net
// working capital is derived.
type WorkingCapitalMethod string

const (
	// WorkingCapitalMethodPercentOfRevenue derives this period's NWC as
	// Revenue x target percent — the common "NWC scales with revenue"
	// forecasting convention, mirroring analytics/workingcapital's own
	// NWCPercentOfRevenue figure. The default when a
	// WorkingCapitalPeriodAssumption leaves Method empty.
	WorkingCapitalMethodPercentOfRevenue WorkingCapitalMethod = "percent_of_revenue"
	// WorkingCapitalMethodFixedAmount replaces the period's NWC with an
	// explicit caller-supplied dollar figure.
	WorkingCapitalMethodFixedAmount WorkingCapitalMethod = "fixed_amount"
	// WorkingCapitalMethodHeldFlat carries the prior period's NWC forward
	// unchanged (a zero change in NWC, and therefore no cash-flow impact
	// from working capital for this period).
	WorkingCapitalMethodHeldFlat WorkingCapitalMethod = "held_flat"
)

// WorkingCapitalPeriodAssumption is one forecast period's operating
// net-working-capital assumption (see WorkingCapitalMethod). Working
// capital projection is optional: a ScenarioResult period with no
// assumption supplied simply leaves NWC/ChangeInNWC unavailable for that
// period rather than assuming zero — the same "explicit availability, not
// zero" rule as every other assumption in this package.
type WorkingCapitalPeriodAssumption struct {
	// Method selects how this period's NWC is derived; empty defaults to
	// WorkingCapitalMethodPercentOfRevenue.
	Method WorkingCapitalMethod `json:"method,omitempty"`
	// PercentOfRevenue is the target NWC-to-revenue ratio, as a decimal,
	// used when Method is WorkingCapitalMethodPercentOfRevenue.
	PercentOfRevenue float64 `json:"percent_of_revenue,omitempty"`
	// FixedAmount is the exact NWC figure for the period, used when Method
	// is WorkingCapitalMethodFixedAmount.
	FixedAmount float64 `json:"fixed_amount,omitempty"`
}

// TaxMethod selects how a forecast period's income tax expense is derived.
type TaxMethod string

const (
	// TaxMethodPercentOfPretaxIncome derives tax as pre-tax income (EBIT
	// minus interest expense plus interest income, when available — see
	// projectTax) x tax rate, floored at $0 (this package never reports a
	// negative tax expense / tax benefit from a projected loss — see
	// projectTax's doc comment). The default when a TaxPeriodAssumption
	// leaves Method empty.
	TaxMethodPercentOfPretaxIncome TaxMethod = "percent_of_pretax_income"
	// TaxMethodFixedAmount replaces the period's tax expense with an
	// explicit caller-supplied dollar figure.
	TaxMethodFixedAmount TaxMethod = "fixed_amount"
)

// TaxPeriodAssumption is one forecast period's income tax assumption (see
// TaxMethod). Optional: a period with no assumption leaves projected tax
// and NetIncome unavailable.
type TaxPeriodAssumption struct {
	// Method selects how this period's tax is derived; empty defaults to
	// TaxMethodPercentOfPretaxIncome.
	Method TaxMethod `json:"method,omitempty"`
	// TaxRate is the effective tax rate, as a decimal, used when Method is
	// TaxMethodPercentOfPretaxIncome.
	TaxRate float64 `json:"tax_rate,omitempty"`
	// FixedAmount is the exact tax expense for the period, used when Method
	// is TaxMethodFixedAmount.
	FixedAmount float64 `json:"fixed_amount,omitempty"`
}

// DebtServiceAssumption is one forecast period's debt-service terms,
// supplied directly by the caller — this package never derives an
// amortization schedule from a principal balance and rate; a caller with a
// full loan schedule supplies each period's principal/interest split
// directly (mirroring cashflow.DebtServiceFigure's identical
// caller-supplied-split shape). InterestRateShock (see ApplyDebtRateShock)
// operates on InterestRate, not on Interest directly, for a caller that
// wants a rate-shock scenario to actually recompute interest from a
// principal balance — see that function's doc comment for exactly when it
// applies.
type DebtServiceAssumption struct {
	// Principal is this period's scheduled cash principal repayment.
	Principal ForecastValue `json:"principal"`
	// Interest is this period's scheduled cash interest payment, used
	// directly in DebtServiceCoverage unless InterestRate and
	// BeginningBalance are both available, in which case Interest is
	// recomputed as BeginningBalance x InterestRate — see projectDebtService.
	Interest ForecastValue `json:"interest"`
	// InterestRate is the annual interest rate applied to BeginningBalance
	// to recompute Interest, as a decimal. Optional; only used when both
	// this and BeginningBalance are available (see the Interest doc
	// comment) — the mechanism ApplyDebtRateShock adjusts.
	InterestRate ForecastValue `json:"interest_rate,omitempty"`
	// BeginningBalance is the outstanding debt principal at the start of
	// this period, used with InterestRate to recompute Interest. Optional.
	BeginningBalance ForecastValue `json:"beginning_balance,omitempty"`
}

// Total returns Principal + Interest, available only if both are available.
func (d DebtServiceAssumption) Total() ForecastValue {
	if !d.Principal.Available || !d.Interest.Available {
		return Unavailable()
	}
	return AvailableValue(d.Principal.Value + d.Interest.Value)
}

// Assumptions bundles every period's assumption for one Scenario. Every
// slice is indexed by forecast period number (index 0 is forecast period 1,
// the first period after the historical base) and must have at least
// Input.Horizon entries for that period to be projected — a period beyond
// the end of a slice (or a zero-value entry within it) is treated as "no
// assumption supplied" for that specific line, not as "assume zero" (see
// each Method's zero-value default and the per-line Unavailable rules
// documented on ScenarioResult's fields).
type Assumptions struct {
	// Revenue is this scenario's revenue assumption for each forecast
	// period. Required for period i to have a projected revenue figure; a
	// period with the zero-value RevenuePeriodAssumption still projects
	// (RevenueMethodGrowthRate with a 0 growth rate — flat revenue), since
	// there is no ambiguity in a zero-value revenue assumption the way
	// there is for, say, a zero-value working-capital assumption.
	Revenue []RevenuePeriodAssumption `json:"revenue"`
	// COGS is this scenario's cost-of-goods-sold assumption for each
	// forecast period. A period with the zero-value COGSPeriodAssumption
	// projects COGSMethodGrossMarginPercent with a 0% target margin (100%
	// COGS-of-revenue) — since that is almost never the intent, callers
	// should always supply an explicit GrossMarginPercent; Calculate warns
	// (IssueImpliedZeroGrossMargin) when a non-empty COGS slice entry
	// leaves GrossMarginPercent at exactly 0 under the default method.
	COGS []COGSPeriodAssumption `json:"cogs"`
	// Opex is this scenario's operating-expense assumption for each
	// forecast period, under the same zero-value-is-flat-growth rule as
	// Revenue.
	Opex []OpexPeriodAssumption `json:"opex"`
	// DepreciationAmortization is this scenario's D&A assumption for each
	// forecast period. Optional per period (see
	// DepreciationAmortizationAssumption's field docs); a nil or
	// short slice simply leaves D&A unavailable for the missing periods.
	DepreciationAmortization []DepreciationAmortizationAssumption `json:"depreciation_amortization,omitempty"`
	// Capex is this scenario's capital-expenditure assumption for each
	// forecast period. Optional per period.
	Capex []CapexAssumption `json:"capex,omitempty"`
	// WorkingCapital is this scenario's operating-net-working-capital
	// assumption for each forecast period. Optional per period; see
	// WorkingCapitalPeriodAssumption's doc comment.
	WorkingCapital []WorkingCapitalPeriodAssumption `json:"working_capital,omitempty"`
	// Tax is this scenario's income-tax assumption for each forecast
	// period. Optional per period.
	Tax []TaxPeriodAssumption `json:"tax,omitempty"`
	// DebtService is this scenario's debt-service assumption for each
	// forecast period. Optional per period.
	DebtService []DebtServiceAssumption `json:"debt_service,omitempty"`
}

// ScenarioType classifies a Scenario's relationship to the caller's base
// case — display/grouping metadata only, never used by Calculate to alter
// how a Scenario's own Assumptions are applied (a "downside" scenario is
// exactly as caller-defined as a "custom" one; this package attaches no
// automatic haircut or transformation based on ScenarioType alone — see the
// package doc comment's "does not predict assumptions" rule).
type ScenarioType string

const (
	ScenarioTypeBase     ScenarioType = "base"
	ScenarioTypeDownside ScenarioType = "downside"
	ScenarioTypeUpside   ScenarioType = "upside"
	ScenarioTypeCustom   ScenarioType = "custom"
)

// Scenario is one named, independently-assumed forecast run. Calculate
// projects every Scenario forward from the identical historical base (see
// Input.Dataset), so ScenarioResults differ only in the Assumptions
// supplied here.
type Scenario struct {
	// Name is a caller-chosen, unique-within-this-Input label (e.g. "Base
	// Case", "Downside — Customer Loss"). Required; Calculate reports
	// IssueDuplicateScenarioName if two Scenarios share a Name, and
	// IssueEmptyScenarioName if Name is empty.
	Name string `json:"name"`
	// Type classifies this Scenario for display/grouping (see ScenarioType).
	Type ScenarioType `json:"type"`
	// Assumptions is this Scenario's full assumption set.
	Assumptions Assumptions `json:"assumptions"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// Dataset is the normalized historical financial data every Scenario
	// projects forward from. Required; Calculate returns a Result with
	// Available == false and an Errors entry if Dataset has no periods.
	Dataset financial.FinancialDataset `json:"dataset"`
	// PeriodMeta supplies chronological ordering for Dataset's periods, so
	// this package can identify the most recent historical period as the
	// base. Required; if nil, or a period present in Dataset has no entry,
	// Calculate returns a Result with Available == false and an Errors
	// entry — unlike this package's analytics siblings (where missing
	// PeriodMeta merely disables trend/seasonality outputs), this package
	// cannot identify a base period to project from at all without
	// chronological order, so the whole Result fails rather than silently
	// picking a lexically-last period that might not be chronologically
	// last.
	PeriodMeta map[financial.Period]PeriodInfo `json:"period_meta"`
	// Horizon is the number of forecast periods to project, starting
	// immediately after the historical base period. Must be >= 1; Calculate
	// returns a Result with Available == false and an Errors entry
	// otherwise.
	Horizon int `json:"horizon"`
	// ForecastPeriodLabels optionally supplies a display label for each
	// forecast period (index 0 is forecast period 1), e.g. "FY2027" or
	// "Year 1". A period beyond the end of this slice (or when the slice is
	// nil) falls back to a generated "Period N" label — purely cosmetic,
	// never parsed.
	ForecastPeriodLabels []string `json:"forecast_period_labels,omitempty"`
	// Scenarios is the set of named scenarios to project. Required; at
	// least one. Calculate returns a Result with Available == false and an
	// Errors entry if empty.
	Scenarios []Scenario `json:"scenarios"`
	// WorkingCapitalPolicy configures which financial.Code values count as
	// operating current assets/liabilities when deriving the historical
	// base period's starting NWC (see BaseFinancials.WorkingCapital). A
	// projected period's own NWC always comes from that period's
	// WorkingCapitalPeriodAssumption (e.g. PercentOfRevenue is always
	// caller-supplied per period) — this policy is used only to derive
	// BaseFinancials.WorkingCapital itself. If the zero value,
	// workingcapital.DefaultInclusionPolicy() is used — this package reuses
	// analytics/workingcapital.InclusionPolicy directly rather than
	// inventing a second, subtly different working-capital policy type,
	// mirroring analytics/cashflow.Input.Policy's identical choice.
	WorkingCapitalPolicy workingcapital.InclusionPolicy `json:"working_capital_policy"`
	// DebtServiceCoverageSource selects which cash-flow figure
	// DebtServiceCoverage divides by. If empty, DSCSourceOperatingCashFlow
	// is used.
	DebtServiceCoverageSource DebtServiceCoverageSource `json:"debt_service_coverage_source,omitempty"`
}

// DebtServiceCoverageSource selects the numerator DSCR is computed from.
type DebtServiceCoverageSource string

const (
	// DSCSourceOperatingCashFlow divides OperatingCashFlow by total debt
	// service — the conventional lender DSCR definition. The default.
	DSCSourceOperatingCashFlow DebtServiceCoverageSource = "operating_cash_flow"
	// DSCSourceEBITDA divides EBITDA by total debt service — a simpler
	// variant some lenders use in lieu of a full cash-flow statement.
	DSCSourceEBITDA DebtServiceCoverageSource = "ebitda"
)

// resolveDSCSource returns s if non-empty, otherwise DSCSourceOperatingCashFlow.
func resolveDSCSource(s DebtServiceCoverageSource) DebtServiceCoverageSource {
	if s == "" {
		return DSCSourceOperatingCashFlow
	}
	return s
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
// theirs — a forecast input problem is a distinct problem domain.
type IssueCode string

const (
	// IssueNoPeriods means Input.Dataset had no periods at all; Calculate
	// returns Available == false.
	IssueNoPeriods IssueCode = "NO_PERIODS"
	// IssueNoPeriodMeta means Input.PeriodMeta was nil, empty, or missing
	// an entry for at least one period present in Input.Dataset, so the
	// historical base period could not be reliably identified; Calculate
	// returns Available == false — see Input.PeriodMeta's doc comment.
	IssueNoPeriodMeta IssueCode = "NO_PERIOD_META"
	// IssueInvalidHorizon means Input.Horizon was < 1; Calculate returns
	// Available == false.
	IssueInvalidHorizon IssueCode = "INVALID_HORIZON"
	// IssueNoScenarios means Input.Scenarios was empty; Calculate returns
	// Available == false.
	IssueNoScenarios IssueCode = "NO_SCENARIOS"
	// IssueEmptyScenarioName means a Scenario's Name was empty; that
	// Scenario is skipped entirely (no ScenarioResult is produced for it).
	IssueEmptyScenarioName IssueCode = "EMPTY_SCENARIO_NAME"
	// IssueDuplicateScenarioName means two or more Scenarios shared the
	// same Name; only the first is projected, later duplicates are skipped.
	IssueDuplicateScenarioName IssueCode = "DUPLICATE_SCENARIO_NAME"
	// IssueNoBaseRevenue means the historical base period had no available
	// total revenue (no revenue code present at all), so
	// RevenueMethodGrowthRate cannot compound from it for that Scenario —
	// every forecast period's revenue is left unavailable unless the
	// Scenario uses RevenueMethodFixedAmount throughout. Advisory only,
	// scoped to the affected Scenario(s).
	IssueNoBaseRevenue IssueCode = "NO_BASE_REVENUE"
	// IssueMissingRevenueAssumption means a Scenario's Assumptions.Revenue
	// slice had fewer than Input.Horizon entries, so at least one forecast
	// period has no revenue assumption and its revenue (and everything
	// depending on it) is left unavailable. Advisory only.
	IssueMissingRevenueAssumption IssueCode = "MISSING_REVENUE_ASSUMPTION"
	// IssueMissingCOGSAssumption mirrors IssueMissingRevenueAssumption for
	// Assumptions.COGS.
	IssueMissingCOGSAssumption IssueCode = "MISSING_COGS_ASSUMPTION"
	// IssueMissingOpexAssumption mirrors IssueMissingRevenueAssumption for
	// Assumptions.Opex.
	IssueMissingOpexAssumption IssueCode = "MISSING_OPEX_ASSUMPTION"
	// IssueImpliedZeroGrossMargin means a period's COGSPeriodAssumption used
	// (explicitly or via the zero-value default) COGSMethodGrossMarginPercent
	// with GrossMarginPercent == 0, implying COGS consumes 100% of revenue.
	// Advisory only — this is a legal, if unusual, assumption.
	IssueImpliedZeroGrossMargin IssueCode = "IMPLIED_ZERO_GROSS_MARGIN"
	// IssueNegativeProjectedValue means a projected figure that is normally
	// expected non-negative (total revenue, total COGS, total opex) came
	// out negative for at least one period/Scenario — most often the
	// result of a large negative growth rate or shock compounding past
	// zero. Advisory only; the negative figure is still reported verbatim,
	// never clamped, per this package's "apply the assumption, don't
	// second-guess it" design.
	IssueNegativeProjectedValue IssueCode = "NEGATIVE_PROJECTED_VALUE"
)

// Issue is a single Calculate-time input finding, mirroring every analytics
// sibling package's identical Issue shape.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	// Scenario is the Scenario.Name this Issue applies to, when scoped to
	// one Scenario. Empty for an Input-level Issue that applies to the
	// whole Result (e.g. IssueNoPeriods).
	Scenario string `json:"scenario,omitempty"`
	// Period is the forecast period label this Issue applies to, when
	// scoped to one period. Empty for a Scenario-level or Input-level
	// Issue.
	Period  string `json:"period,omitempty"`
	Message string `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from adjustments.HasErrors/qoe.HasErrors/
// workingcapital.HasErrors/cashflow.HasErrors/revenuequality.HasErrors/
// concentration.HasErrors/anomalies.HasErrors/variance.HasErrors rather
// than shared — see adjustments.HasErrors's doc comment for the full
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

// LineItem is one financial.Code's projected (or historical base) amount
// for one period, with enough metadata to explain how it was derived
// without requiring a caller to re-derive it from Assumptions itself —
// mirroring metrics.Component's identical explainability role, but carried
// per-period rather than only as a computation-trace component.
type LineItem struct {
	Code   financial.Code `json:"code"`
	Label  string         `json:"label,omitempty"`
	Amount float64        `json:"amount"`
}

// PeriodPL is one period's full projected (or historical base) profit and
// loss, plus every derived margin/earnings figure — the central per-period
// building block this package produces. The same shape is used for
// BaseFinancials (the historical starting point) and for each
// ScenarioResult.ProjectedPeriods entry, so a caller can display/diff them
// uniformly.
type PeriodPL struct {
	// Period is a display label: the historical financial.Period string for
	// BaseFinancials, or the resolved forecast-period label (see
	// Input.ForecastPeriodLabels) for a ScenarioResult entry.
	Period string `json:"period"`
	// PeriodNumber is 0 for BaseFinancials, or this period's 1-based
	// position within the forecast horizon for a ScenarioResult entry.
	PeriodNumber int `json:"period_number"`

	// RevenueLines is one LineItem per revenue financial.Code with an
	// available amount, sorted by Code ascending.
	RevenueLines []LineItem `json:"revenue_lines,omitempty"`
	// TotalRevenue is the sum of RevenueLines. Available only if at least
	// one revenue code has an available amount.
	TotalRevenue ForecastValue `json:"total_revenue"`

	// COGSLines mirrors RevenueLines for COGS codes.
	COGSLines []LineItem `json:"cogs_lines,omitempty"`
	// TotalCOGS mirrors TotalRevenue for COGS codes.
	TotalCOGS ForecastValue `json:"total_cogs"`

	// GrossProfit is TotalRevenue - TotalCOGS. Available only if both are
	// available.
	GrossProfit ForecastValue `json:"gross_profit"`
	// GrossMargin is GrossProfit / TotalRevenue. Available only if
	// GrossProfit is available and TotalRevenue is nonzero — mirroring
	// metrics.grossMargin's identical zero-revenue guard.
	GrossMargin ForecastValue `json:"gross_margin"`

	// OpexLines mirrors RevenueLines for operating-expense codes.
	OpexLines []LineItem `json:"opex_lines,omitempty"`
	// TotalOpex mirrors TotalRevenue for opex codes.
	TotalOpex ForecastValue `json:"total_opex"`
	// OwnerCompensation is the CodeOpexOwnerComp line specifically, echoed
	// separately for SDE's add-back — mirroring metrics.Snapshot.
	// OwnerCompensation's identical role.
	OwnerCompensation ForecastValue `json:"owner_compensation"`

	// EBIT is GrossProfit - TotalOpex. Available only if both are
	// available.
	EBIT ForecastValue `json:"ebit"`

	// Depreciation/Amortization are this period's D&A figures — the
	// historical base's actual reported amounts for BaseFinancials, or the
	// Scenario's DepreciationAmortizationAssumption for a projected period.
	Depreciation ForecastValue `json:"depreciation"`
	Amortization ForecastValue `json:"amortization"`

	// EBITDA is EBIT + Depreciation + Amortization, using the exact same
	// formula and availability rule as metrics.ebitda (available whenever
	// EBIT is available; missing D&A contributes $0 rather than making
	// EBITDA unavailable).
	EBITDA ForecastValue `json:"ebitda"`
	// EBITDAMargin is EBITDA / TotalRevenue, under the same zero-revenue
	// guard as GrossMargin.
	EBITDAMargin ForecastValue `json:"ebitda_margin"`

	// SDE is EBITDA + OwnerCompensation, mirroring metrics.sde's exact
	// formula and availability rule (available whenever EBITDA is
	// available; missing owner compensation contributes $0).
	SDE ForecastValue `json:"sde"`
	// SDEMargin is SDE / TotalRevenue, under the same zero-revenue guard.
	SDEMargin ForecastValue `json:"sde_margin"`

	// InterestExpense/InterestIncome are this period's figures, when
	// available — the historical base's actual reported amounts, or (for a
	// projected period) derived from the Scenario's DebtServiceAssumption
	// when it supplies InterestRate/BeginningBalance, otherwise
	// Assumptions.DebtService[i].Interest verbatim, otherwise unavailable.
	InterestExpense ForecastValue `json:"interest_expense"`
	InterestIncome  ForecastValue `json:"interest_income"`

	// PretaxIncome is EBIT - InterestExpense + InterestIncome. Available
	// only if EBIT is available (missing interest figures contribute $0,
	// mirroring EBITDA's D&A availability rule).
	PretaxIncome ForecastValue `json:"pretax_income"`
	// IncomeTax is this period's tax expense (see TaxPeriodAssumption).
	// Available only when a tax assumption was supplied (for a projected
	// period) or the historical base reported one.
	IncomeTax ForecastValue `json:"income_tax"`
	// NetIncome is PretaxIncome - IncomeTax. Available only if both are
	// available.
	NetIncome ForecastValue `json:"net_income"`
}

// WorkingCapitalPeriod is one period's operating net working capital and
// its period-over-period change, mirroring cashflow.Bridge's ChangeInNWC
// role but carried as its own type since this package computes NWC for
// projected periods directly from assumptions rather than recomputing it
// from a financial.FinancialDataset via analytics/workingcapital.Calculate.
type WorkingCapitalPeriod struct {
	// Period/PeriodNumber mirror PeriodPL's fields.
	Period       string `json:"period"`
	PeriodNumber int    `json:"period_number"`
	// NWC is this period's operating net working capital. Available only
	// when derivable: the historical base's actual figure (from
	// Input.Dataset under Input.WorkingCapitalPolicy), or a projected
	// period's WorkingCapitalPeriodAssumption result.
	NWC ForecastValue `json:"nwc"`
	// ChangeInNWC is this period's NWC minus the immediately preceding
	// period's NWC (an increase is a cash use, reported as positive; a
	// decrease is a cash source, reported as negative) — the same sign
	// convention cashflow.Bridge.ChangeInNWC uses. Available only when both
	// this period's and the preceding period's NWC are available.
	ChangeInNWC ForecastValue `json:"change_in_nwc"`
}

// CashFlowPeriod is one projected period's cash-flow figures, derived
// entirely from that period's PeriodPL, WorkingCapitalPeriod, and
// Assumptions.Capex/Assumptions.DebtService — computed only when the
// underlying inputs support it (see each field's availability rule),
// mirroring analytics/cashflow.Bridge's identical
// "no reported statement, only what the inputs allow" convention, adapted
// here to build forward from EBITDA rather than from a caller-reported
// operating-cash-flow figure (a forecast has no reported cash-flow
// statement to read from by definition).
type CashFlowPeriod struct {
	Period       string `json:"period"`
	PeriodNumber int    `json:"period_number"`

	// OperatingCashFlow is EBITDA - ChangeInNWC - IncomeTax (tax subtracted
	// only when available; a missing tax assumption contributes $0, exactly
	// mirroring EBITDA's D&A availability rule elsewhere in this package).
	// Available only when EBITDA and ChangeInNWC are both available.
	OperatingCashFlow ForecastValue `json:"operating_cash_flow"`
	// Capex echoes Assumptions.Capex for this period.
	Capex ForecastValue `json:"capex"`
	// FreeCashFlow is OperatingCashFlow - Capex.Value. Available only when
	// OperatingCashFlow is available; a missing Capex assumption
	// contributes $0 (mirroring cashflow.Bridge.FreeCashFlow's identical
	// "capex assumed zero, not unavailable" rule when Capex itself is
	// absent — this package does not separately flag that case since a
	// missing per-period Capex assumption is a common, unremarkable input
	// gap for an SMB forecast).
	FreeCashFlow ForecastValue `json:"free_cash_flow"`

	// DebtService echoes Assumptions.DebtService for this period (after
	// InterestRate/BeginningBalance recomputation — see projectDebtService).
	DebtService DebtServiceAssumption `json:"debt_service"`
	// FreeCashFlowToOwner is FreeCashFlow - DebtService.Total().Value.
	// Available only when both are available.
	FreeCashFlowToOwner ForecastValue `json:"free_cash_flow_to_owner"`

	// DebtServiceCoverage is this period's DSCR: the source selected by
	// Input.DebtServiceCoverageSource (OperatingCashFlow or EBITDA) divided
	// by DebtService.Total().Value. Available only when the numerator and
	// DebtService.Total() are both available and DebtService.Total().Value
	// != 0 — a zero-debt-service period has no meaningful coverage ratio
	// (division undefined, not "infinite coverage"), mirroring
	// metrics.grossMargin's identical zero-denominator guard.
	DebtServiceCoverage ForecastValue `json:"debt_service_coverage"`
}

// ScenarioResult is one Scenario's full projection: the P&L, working
// capital, and cash flow for every forecast period, plus the calculation
// trace and warnings specific to this Scenario.
type ScenarioResult struct {
	// Name/Type echo the source Scenario.
	Name string       `json:"name"`
	Type ScenarioType `json:"type"`

	// ProjectedPeriods is one PeriodPL per forecast period, in period-number
	// order (1..Input.Horizon).
	ProjectedPeriods []PeriodPL `json:"projected_periods"`
	// WorkingCapital is one WorkingCapitalPeriod per forecast period, in the
	// same order.
	WorkingCapital []WorkingCapitalPeriod `json:"working_capital"`
	// CashFlow is one CashFlowPeriod per forecast period, in the same
	// order.
	CashFlow []CashFlowPeriod `json:"cash_flow"`

	// Trace is the full step-by-step calculation trace for this Scenario,
	// across every period, in computation order — mirroring valuation.Step/
	// dcf.Result.Steps' identical explainability role, adapted to a
	// multi-period, multi-line forecast.
	Trace []TraceStep `json:"trace,omitempty"`

	// Warnings carries every Issue scoped to this Scenario with
	// SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
}

// TraceStep is a single labeled calculation step within a ScenarioResult's
// Trace, mirroring valuation.Step's identical shape (Label/Value/Detail) so
// a caller already familiar with that convention can read this package's
// trace the same way.
type TraceStep struct {
	// Period is the forecast period label this step applies to.
	Period string `json:"period"`
	// Label is a short, fixed, human-readable description of what this step
	// computed.
	Label string `json:"label"`
	// Value is the resulting figure.
	Value float64 `json:"value"`
	// Detail is a short human-readable description of the exact arithmetic
	// used, when non-trivial.
	Detail string `json:"detail,omitempty"`
}

// BaseFinancials is the historical base period's P&L and working capital —
// the fixed starting point every ScenarioResult compounds forward from,
// echoed once at the top level (rather than duplicated inside every
// ScenarioResult) since it is identical across all of them.
type BaseFinancials struct {
	// Period is the historical financial.Period Calculate identified as the
	// most recent chronologically (per Input.PeriodMeta).
	Period financial.Period `json:"period"`
	// PL is the base period's actual P&L, computed directly from
	// Input.Dataset using the exact same formulas as PeriodPL's projected
	// fields (see base.go) — so a caller can display period 0 (actual)
	// alongside periods 1..Horizon (projected) uniformly. InterestExpense/
	// InterestIncome/IncomeTax/NetIncome reflect the dataset's actual
	// reported figures for the base period, not a projection.
	PL PeriodPL `json:"pl"`
	// WorkingCapital is the base period's actual operating NWC, computed
	// from Input.Dataset under Input.WorkingCapitalPolicy.
	WorkingCapital ForecastValue `json:"working_capital"`
}

// Result is the output of Calculate: the historical base, one ScenarioResult
// per Scenario, and formula version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoPeriods/IssueNoPeriodMeta/IssueInvalidHorizon/
	// IssueNoScenarios) — every other field is then zero-value.
	Available bool `json:"available"`

	// Base is the historical starting point every scenario projects
	// forward from.
	Base BaseFinancials `json:"base"`

	// Horizon echoes Input.Horizon.
	Horizon int `json:"horizon"`
	// ForecastPeriods is the resolved display label for each forecast
	// period (index 0 is period 1), after Input.ForecastPeriodLabels
	// substitution/fallback.
	ForecastPeriods []string `json:"forecast_periods"`

	// ScenarioResults is one ScenarioResult per Input.Scenarios entry that
	// passed name validation (see IssueEmptyScenarioName/
	// IssueDuplicateScenarioName), in Input.Scenarios order.
	ScenarioResults []ScenarioResult `json:"scenario_results,omitempty"`

	// DebtServiceCoverageSource echoes the resolved
	// Input.DebtServiceCoverageSource (after default substitution).
	DebtServiceCoverageSource DebtServiceCoverageSource `json:"debt_service_coverage_source"`
	// WorkingCapitalPolicy echoes the resolved Input.WorkingCapitalPolicy
	// (after workingcapital.DefaultInclusionPolicy substitution).
	WorkingCapitalPolicy workingcapital.InclusionPolicy `json:"working_capital_policy"`

	// Warnings carries every Input-level Issue (Scenario == "") with
	// SeverityWarning. Scenario-scoped warnings are also duplicated onto
	// each ScenarioResult.Warnings for a caller working with one
	// ScenarioResult in isolation, and are NOT included here — see
	// ScenarioResult.Warnings.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
