// Package dcf implements a deterministic discounted-cash-flow valuation
// engine: an explicit, caller-supplied series of forecast free cash flows,
// discounted at a caller-supplied rate, plus a Gordon Growth terminal
// value.
//
// This package does not forecast anything. Every ForecastPeriod's cash flow
// is supplied by the caller; Calculate only discounts, sums, and computes
// the terminal value from the figures it is given. Nothing here reaches
// into financial/metrics, financial/adjustments, or financial/earnings —
// a caller assembles ForecastPeriod values from whatever combination of
// historical data and forward-looking judgment its own forecasting process
// produces, entirely outside this package. Every function here is pure: no
// I/O, no mutation of its inputs.
//
// Value type. A DCF discounts free cash flow to the firm (unlevered,
// pre-debt-service cash flow) at a weighted-average-cost-of-capital-style
// discount rate, which conventionally produces an Enterprise Value — the
// value of the operating business independent of how it happens to be
// financed. Calculate always reports the discounted sum as
// valuation.ValueTypeEnterprise, with Equity Value available only through
// the explicit valuation.Bridge (see Input.EquityBridge), matching
// valuation/ebitda's convention. This package does not support discounting
// levered free-cash-flow-to-equity at a cost-of-equity rate as an
// alternative mode: mixing the two conventions inside one Input would
// invite a caller to accidentally combine an equity-basis cash flow with an
// enterprise-basis discount rate (or vice versa) with no way for this
// package to detect the mismatch, so only the FCFF-to-Enterprise-Value
// convention is implemented.
package dcf

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/valuation"
)

// Code is this method's stable identifier. See valuation.CodeDCF.
const Code = valuation.CodeDCF

// Version is this method's calculation-logic version. See sde.Version's doc
// comment for what incrementing it means.
const Version = "1.0.0"

// Validation issue codes specific to this method.
const (
	// IssueNoForecastPeriods means Input.ForecastPeriods was empty. This
	// package generates no forecasts of its own — see the package doc
	// comment — so an empty forecast leaves nothing to discount.
	IssueNoForecastPeriods valuation.IssueCode = "NO_FORECAST_PERIODS"
	// IssueNonPositiveDiscountRate means Input.DiscountRate was <= 0.
	IssueNonPositiveDiscountRate valuation.IssueCode = "NON_POSITIVE_DISCOUNT_RATE"
	// IssueDiscountRateNotAboveTerminalGrowth means Input.DiscountRate was
	// <= Input.TerminalGrowthRate. Gordon Growth's denominator
	// (discount_rate - terminal_growth_rate) must be strictly positive: at
	// or below zero, the formula implies a business growing as fast as or
	// faster than it is discounted, which produces an infinite or negative
	// "value" with no economic meaning, not a large-but-real number.
	IssueDiscountRateNotAboveTerminalGrowth valuation.IssueCode = "DISCOUNT_RATE_NOT_ABOVE_TERMINAL_GROWTH"
	// IssueNegativeForecastCashFlow is a warning noting that at least one
	// ForecastPeriod.FreeCashFlow was zero or negative.
	IssueNegativeForecastCashFlow valuation.IssueCode = "NEGATIVE_FORECAST_CASH_FLOW"
	// IssueNegativeTerminalCashFlow is a warning noting that the terminal
	// year's cash flow used to seed the Gordon Growth formula
	// (ForecastPeriods' last entry) was zero or negative, which will
	// propagate through to a zero-or-negative terminal value.
	IssueNegativeTerminalCashFlow valuation.IssueCode = "NEGATIVE_TERMINAL_CASH_FLOW"
)

// ForecastPeriod is a single caller-supplied forecast period: a label and
// its free cash flow. Periods must be supplied in chronological order
// (period 1 first); Calculate discounts ForecastPeriods[i] using period
// number i+1, it does not parse or reorder Period.
type ForecastPeriod struct {
	// Period is a display label for this forecast period (e.g. "2026",
	// "Year 1"). Purely for explainability; not parsed.
	Period string `json:"period"`
	// FreeCashFlow is this period's forecast free cash flow to the firm,
	// caller-supplied. This package does not derive it.
	FreeCashFlow float64 `json:"free_cash_flow"`
}

// EquityBridgeInput supplies the debt/cash figures needed to bridge this
// method's native Enterprise Value to an Equity Value, identical in shape
// and meaning to valuation/ebitda.EquityBridgeInput — see its doc comment.
type EquityBridgeInput struct {
	// Requested opts into computing Result.Bridge.
	Requested bool `json:"requested"`
	// ExcessCash is cash and cash-equivalents in excess of operating needs,
	// added to Enterprise Value.
	ExcessCash float64 `json:"excess_cash,omitempty"`
	// ShortTermDebt is short-term/current interest-bearing debt, subtracted.
	ShortTermDebt float64 `json:"short_term_debt,omitempty"`
	// LongTermDebt is long-term interest-bearing debt, subtracted.
	LongTermDebt float64 `json:"long_term_debt,omitempty"`
	// OtherDebt is any other caller-identified debt-like obligation to
	// subtract.
	OtherDebt float64 `json:"other_debt,omitempty"`
}

// Input is the DCF method's strongly-typed input.
type Input struct {
	// ForecastPeriods is the explicit, caller-supplied forecast: at least
	// one period, in chronological order. Calculate never generates a
	// forecast — see the package doc comment.
	ForecastPeriods []ForecastPeriod `json:"forecast_periods"`
	// DiscountRate is the rate used to discount each forecast period's cash
	// flow and the terminal value back to present value, expressed as a
	// decimal (0.15 = 15%). Must be > 0 and > TerminalGrowthRate.
	DiscountRate float64 `json:"discount_rate"`
	// TerminalGrowthRate is the long-run growth rate used in the Gordon
	// Growth terminal value formula, expressed as a decimal. Must be <
	// DiscountRate.
	TerminalGrowthRate float64 `json:"terminal_growth_rate"`
	// MidYearConvention, if true, discounts each forecast period's cash
	// flow (and the terminal value) as though received at the midpoint of
	// its period (discount exponent i-0.5 for period i) rather than at
	// period-end (exponent i) — a common refinement reflecting that cash
	// flow arrives throughout the year rather than in one lump sum on the
	// last day. false (the zero value, and this package's default) uses
	// plain period-end discounting. This package supports the convention
	// only because the caller can explicitly opt into it; it is never
	// applied implicitly.
	MidYearConvention bool `json:"mid_year_convention,omitempty"`
	// EquityBridge optionally requests the Enterprise-Value-to-Equity-Value
	// bridge. See EquityBridgeInput's doc comment.
	EquityBridge EquityBridgeInput `json:"equity_bridge,omitempty"`
}

// ProjectedPeriod is one forecast period's full calculation detail: its
// cash flow, discount factor, and present value, in the order Calculate
// discounted them.
type ProjectedPeriod struct {
	// Period echoes ForecastPeriod.Period.
	Period string `json:"period"`
	// PeriodNumber is this period's 1-based position in ForecastPeriods,
	// the exponent (or exponent basis, under MidYearConvention) used for
	// discounting.
	PeriodNumber int `json:"period_number"`
	// FreeCashFlow echoes ForecastPeriod.FreeCashFlow.
	FreeCashFlow float64 `json:"free_cash_flow"`
	// DiscountFactor is 1 / (1 + DiscountRate)^exponent, where exponent is
	// PeriodNumber, or PeriodNumber-0.5 under MidYearConvention.
	DiscountFactor float64 `json:"discount_factor"`
	// PresentValue is FreeCashFlow * DiscountFactor.
	PresentValue float64 `json:"present_value"`
}

// Result is the DCF method's output.
type Result struct {
	// Method is this method's stable Code.
	Method valuation.Code `json:"method"`
	// MethodVersion is the Version Calculate ran under.
	MethodVersion string `json:"method_version"`
	// ValueType is always valuation.ValueTypeEnterprise for this method's
	// direct result — see the package doc comment.
	ValueType valuation.ValueType `json:"value_type"`
	// Input echoes the exact Input Calculate was given.
	Input Input `json:"input"`
	// Available is false if EnterpriseValue could not be computed at all
	// (a blocking validation error: no forecast periods, non-finite rates,
	// a non-positive discount rate, or discount rate not above terminal
	// growth rate). Available is true even when one or more forecast cash
	// flows are zero or negative, or the terminal cash flow is negative —
	// see Calculate's doc comment.
	Available bool `json:"available"`
	// ProjectedPeriods carries every forecast period's cash flow, discount
	// factor, and present value, in forecast order.
	ProjectedPeriods []ProjectedPeriod `json:"projected_periods,omitempty"`
	// SumOfPresentValues is the sum of every ProjectedPeriods[i].PresentValue.
	SumOfPresentValues float64 `json:"sum_of_present_values"`
	// TerminalYearCashFlow is ForecastPeriods' last entry's FreeCashFlow —
	// the FCF(n) the Gordon Growth formula grows forward from.
	TerminalYearCashFlow float64 `json:"terminal_year_cash_flow"`
	// TerminalCashFlow is FCF(n+1) = TerminalYearCashFlow x
	// (1 + TerminalGrowthRate), the numerator of the Gordon Growth formula.
	TerminalCashFlow float64 `json:"terminal_cash_flow"`
	// TerminalValue is TerminalCashFlow / (DiscountRate -
	// TerminalGrowthRate), valued as of the end of the final forecast
	// period (before discounting back to present value).
	TerminalValue float64 `json:"terminal_value"`
	// TerminalValueDiscountFactor is the discount factor applied to
	// TerminalValue — identical to the final forecast period's
	// DiscountFactor, since the terminal value is valued as of that same
	// point in time.
	TerminalValueDiscountFactor float64 `json:"terminal_value_discount_factor"`
	// TerminalValuePresentValue is TerminalValue * TerminalValueDiscountFactor.
	TerminalValuePresentValue float64 `json:"terminal_value_present_value"`
	// EnterpriseValue is SumOfPresentValues + TerminalValuePresentValue.
	// Meaningful only when Available is true.
	EnterpriseValue float64 `json:"enterprise_value"`
	// Steps is the full calculation trace, in the order computed.
	Steps []valuation.Step `json:"steps,omitempty"`
	// Bridge is populated only when Input.EquityBridge.Requested is true.
	Bridge valuation.Bridge `json:"bridge"`
	// Warnings carries non-blocking Issues.
	Warnings []valuation.Issue `json:"warnings,omitempty"`
	// Errors carries blocking Issues that made Available false.
	Errors []valuation.Issue `json:"errors,omitempty"`
}

// Calculate runs the DCF engine:
//
//  1. Discounts each ForecastPeriods[i] at DiscountRate using period number
//     i+1 (or i+0.5 under MidYearConvention).
//
//  2. Computes the Gordon Growth terminal value as of the end of the final
//     forecast period:
//
//     FCF(n+1) = FCF(n) x (1 + terminal_growth_rate)
//     Terminal Value = FCF(n+1) / (discount_rate - terminal_growth_rate)
//
//  3. Discounts the terminal value back to present value using the same
//     discount factor as the final forecast period.
//
//  4. Sums every forecast period's present value plus the terminal value's
//     present value into Enterprise Value.
//
//  5. If requested, bridges Enterprise Value to Equity Value.
//
// Calculate never panics on bad financial input. Blocking SeverityErrors
// (Result.Available == false, every numeric result field zero): no
// ForecastPeriods, a non-finite DiscountRate/TerminalGrowthRate/cash flow,
// DiscountRate <= 0, or DiscountRate <= TerminalGrowthRate (see
// IssueDiscountRateNotAboveTerminalGrowth's doc comment for why this must
// be a strict, blocking inequality rather than a warned edge case). A zero
// or negative forecast cash flow — including a negative terminal-year cash
// flow, which will produce a zero-or-negative terminal value — is NOT
// blocking: these are real, calculable (if concerning) scenarios reported
// as SeverityWarnings, never silently clamped or hidden.
func Calculate(input Input) Result {
	result := Result{
		Method:        Code,
		MethodVersion: Version,
		ValueType:     valuation.ValueTypeEnterprise,
		Input:         input,
	}

	var issues []valuation.Issue

	if len(input.ForecastPeriods) == 0 {
		issues = append(issues, valuation.Issue{
			Code: IssueNoForecastPeriods, Severity: valuation.SeverityError,
			Message: "at least one forecast period is required; this package does not generate forecasts",
		})
	}
	for i, fp := range input.ForecastPeriods {
		if !isFinite(fp.FreeCashFlow) {
			issues = append(issues, valuation.Issue{
				Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
				Message: fmt.Sprintf("forecast period %d (%q) free cash flow is not a finite number", i+1, fp.Period),
			})
		}
	}
	if !isFinite(input.DiscountRate) {
		issues = append(issues, valuation.Issue{
			Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
			Message: "discount rate is not a finite number",
		})
	} else if input.DiscountRate <= 0 {
		issues = append(issues, valuation.Issue{
			Code: IssueNonPositiveDiscountRate, Severity: valuation.SeverityError,
			Message: "discount rate must be greater than zero",
		})
	}
	if !isFinite(input.TerminalGrowthRate) {
		issues = append(issues, valuation.Issue{
			Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
			Message: "terminal growth rate is not a finite number",
		})
	}
	if input.EquityBridge.Requested {
		issues = append(issues, validateBridgeInputs(input.EquityBridge)...)
	}

	// Only compare DiscountRate/TerminalGrowthRate once both are confirmed
	// finite — an unordered comparison against NaN would be meaningless and
	// is already reported above.
	if isFinite(input.DiscountRate) && isFinite(input.TerminalGrowthRate) && input.DiscountRate > 0 && input.DiscountRate <= input.TerminalGrowthRate {
		issues = append(issues, valuation.Issue{
			Code: IssueDiscountRateNotAboveTerminalGrowth, Severity: valuation.SeverityError,
			Message: fmt.Sprintf("discount rate (%v) must be greater than terminal growth rate (%v)", input.DiscountRate, input.TerminalGrowthRate),
		})
	}

	if valuation.HasErrors(issues) {
		result.Errors = valuation.Errors(issues)
		result.Warnings = valuation.Warnings(issues)
		return result
	}

	for i, fp := range input.ForecastPeriods {
		if fp.FreeCashFlow <= 0 {
			issues = append(issues, valuation.Issue{
				Code: IssueNegativeForecastCashFlow, Severity: valuation.SeverityWarning,
				Message: fmt.Sprintf("forecast period %d (%q) free cash flow is zero or negative", i+1, fp.Period),
			})
		}
	}

	projected := make([]ProjectedPeriod, 0, len(input.ForecastPeriods))
	sumPV := 0.0
	var lastDiscountFactor float64
	for i, fp := range input.ForecastPeriods {
		periodNumber := i + 1
		exponent := float64(periodNumber)
		if input.MidYearConvention {
			exponent -= 0.5
		}
		discountFactor := 1.0 / math.Pow(1+input.DiscountRate, exponent)
		pv := fp.FreeCashFlow * discountFactor
		projected = append(projected, ProjectedPeriod{
			Period:         fp.Period,
			PeriodNumber:   periodNumber,
			FreeCashFlow:   fp.FreeCashFlow,
			DiscountFactor: discountFactor,
			PresentValue:   pv,
		})
		sumPV += pv
		lastDiscountFactor = discountFactor
	}

	terminalYearCF := input.ForecastPeriods[len(input.ForecastPeriods)-1].FreeCashFlow
	if terminalYearCF <= 0 {
		issues = append(issues, valuation.Issue{
			Code: IssueNegativeTerminalCashFlow, Severity: valuation.SeverityWarning,
			Message: "terminal year cash flow is zero or negative; the terminal value will be zero or negative",
		})
	}
	terminalCF := terminalYearCF * (1 + input.TerminalGrowthRate)
	terminalValue := terminalCF / (input.DiscountRate - input.TerminalGrowthRate)
	terminalValuePV := terminalValue * lastDiscountFactor

	enterpriseValue := sumPV + terminalValuePV

	result.Available = true
	result.ProjectedPeriods = projected
	result.SumOfPresentValues = sumPV
	result.TerminalYearCashFlow = terminalYearCF
	result.TerminalCashFlow = terminalCF
	result.TerminalValue = terminalValue
	result.TerminalValueDiscountFactor = lastDiscountFactor
	result.TerminalValuePresentValue = terminalValuePV
	result.EnterpriseValue = enterpriseValue

	steps := make([]valuation.Step, 0, len(projected)+6)
	for _, p := range projected {
		steps = append(steps, valuation.Step{
			Label:  fmt.Sprintf("PV of forecast period %d (%s)", p.PeriodNumber, p.Period),
			Value:  p.PresentValue,
			Detail: fmt.Sprintf("%v x %v", p.FreeCashFlow, p.DiscountFactor),
		})
	}
	steps = append(steps,
		valuation.Step{Label: "Sum of Present Values of Forecast Cash Flows", Value: sumPV},
		valuation.Step{Label: "Terminal Year Cash Flow = FCF(n)", Value: terminalYearCF},
		valuation.Step{
			Label:  "FCF(n+1) = FCF(n) x (1 + terminal growth rate)",
			Value:  terminalCF,
			Detail: fmt.Sprintf("%v x (1 + %v)", terminalYearCF, input.TerminalGrowthRate),
		},
		valuation.Step{
			Label:  "Terminal Value = FCF(n+1) / (discount rate - terminal growth rate)",
			Value:  terminalValue,
			Detail: fmt.Sprintf("%v / (%v - %v)", terminalCF, input.DiscountRate, input.TerminalGrowthRate),
		},
		valuation.Step{Label: "Present Value of Terminal Value", Value: terminalValuePV, Detail: fmt.Sprintf("%v x %v", terminalValue, lastDiscountFactor)},
		valuation.Step{Label: "Enterprise Value = Sum of PVs + PV of Terminal Value", Value: enterpriseValue},
	)
	result.Steps = steps

	if input.EquityBridge.Requested {
		bridge := buildBridge(enterpriseValue, input.EquityBridge)
		result.Bridge = bridge
		result.Steps = append(result.Steps,
			valuation.Step{Label: "+ Excess Cash", Value: bridge.ExcessCash},
			valuation.Step{Label: "- Total Debt", Value: bridge.TotalDebt},
			valuation.Step{Label: "Equity Value = Enterprise Value + Excess Cash - Total Debt", Value: bridge.EquityValue},
		)
	}

	result.Errors = valuation.Errors(issues)
	result.Warnings = valuation.Warnings(issues)
	return result
}

// validateBridgeInputs checks the bridge's own numeric fields for
// finiteness.
func validateBridgeInputs(in EquityBridgeInput) []valuation.Issue {
	var issues []valuation.Issue
	fields := map[string]float64{
		"excess cash":     in.ExcessCash,
		"short-term debt": in.ShortTermDebt,
		"long-term debt":  in.LongTermDebt,
		"other debt":      in.OtherDebt,
	}
	for name, v := range fields {
		if !isFinite(v) {
			issues = append(issues, valuation.Issue{
				Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
				Message: fmt.Sprintf("equity bridge %s is not a finite number", name),
			})
		}
	}
	return issues
}

// buildBridge applies the Enterprise-Value-to-Equity-Value bridge.
func buildBridge(enterpriseValue float64, in EquityBridgeInput) valuation.Bridge {
	var components []valuation.Component
	totalDebt := 0.0
	if in.ShortTermDebt != 0 {
		components = append(components, valuation.Component{Label: "Short-Term Debt", Amount: in.ShortTermDebt})
		totalDebt += in.ShortTermDebt
	}
	if in.LongTermDebt != 0 {
		components = append(components, valuation.Component{Label: "Long-Term Debt", Amount: in.LongTermDebt})
		totalDebt += in.LongTermDebt
	}
	if in.OtherDebt != 0 {
		components = append(components, valuation.Component{Label: "Other Debt", Amount: in.OtherDebt})
		totalDebt += in.OtherDebt
	}

	return valuation.Bridge{
		Available:       true,
		EnterpriseValue: enterpriseValue,
		ExcessCash:      in.ExcessCash,
		TotalDebt:       totalDebt,
		DebtComponents:  components,
		EquityValue:     enterpriseValue + in.ExcessCash - totalDebt,
	}
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
