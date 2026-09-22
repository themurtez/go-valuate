// Package ebitda implements the EBITDA multiple valuation method: a single
// maintainable-EBITDA figure multiplied by a caller-selected multiple,
// producing an Enterprise Value, with an explicit bridge to Equity Value.
//
// This package does not compute EBITDA itself (see financial/metrics for
// baseline EBITDA and financial/adjustments for the normalized-EBITDA
// bridge) and does not select a multiple on the caller's behalf (see
// settings.Settings' EBITDAMultiple for one way a caller might resolve
// one). Every function here is pure: no I/O, no mutation of its inputs.
//
// Value type. EBITDA is a capital-structure-neutral, pre-debt-service
// figure, and an EBITDA multiple is conventionally understood by valuation
// practice to price the whole enterprise — what an acquirer would pay for
// the operating business on a cash-free, debt-free basis — not the specific
// equity stake of a specific capital structure. Calculate therefore always
// reports the direct MaintainableEBITDA * Multiple result as
// valuation.ValueTypeEnterprise, and separately computes an Equity Value
// only via the explicit valuation.Bridge (see Input.EquityBridge). This is
// the "strong documented reason otherwise" default the package brief
// calls for: no code path in this package produces an equity value except
// through that bridge.
package ebitda

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/valuation"
)

// Code is this method's stable identifier. See valuation.CodeEBITDAMultiple.
const Code = valuation.CodeEBITDAMultiple

// Version is this method's calculation-logic version. See sde.Version's doc
// comment for what incrementing it means.
const Version = "1.0.0"

// Validation issue codes specific to this method.
const (
	// IssueNonPositiveMultiple means Input.Multiple was <= 0.
	IssueNonPositiveMultiple valuation.IssueCode = "NON_POSITIVE_MULTIPLE"
	// IssueNonPositiveEBITDA is a warning noting that
	// Input.MaintainableEBITDA was zero or negative.
	IssueNonPositiveEBITDA valuation.IssueCode = "NON_POSITIVE_EBITDA"
)

// EquityBridgeInput supplies the debt/cash figures needed to bridge this
// method's native Enterprise Value to an Equity Value:
//
//	Equity Value = Enterprise Value + Excess Cash - Total Debt
//
// where Total Debt = ShortTermDebt + LongTermDebt + OtherDebt. Every field
// is a plain, non-negative magnitude contributed in the direction its name
// implies (cash is added, every debt field is subtracted) — there is no
// signed "net debt" field here, mirroring financial/adjustments' explicit
// magnitude-plus-direction convention, so a caller and a reviewer never
// have to remember which sign convention this particular bridge used.
//
// A zero-value EquityBridgeInput is a real, deliberate input (all bridge
// components are zero, e.g. a debt-free cash-free business) and is
// distinguished from "no bridge was requested at all" by Requested — see
// its doc comment.
type EquityBridgeInput struct {
	// Requested opts into computing Result.Bridge. false (the zero value)
	// means Result.Bridge.Available is false and no equity value is
	// computed at all — this method's own result remains a fully valid
	// Enterprise Value either way.
	Requested bool `json:"requested"`
	// ExcessCash is cash and cash-equivalents in excess of operating needs,
	// added to Enterprise Value.
	ExcessCash float64 `json:"excess_cash,omitempty"`
	// ShortTermDebt is short-term/current interest-bearing debt, subtracted.
	ShortTermDebt float64 `json:"short_term_debt,omitempty"`
	// LongTermDebt is long-term interest-bearing debt, subtracted.
	LongTermDebt float64 `json:"long_term_debt,omitempty"`
	// OtherDebt is any other caller-identified debt-like obligation the
	// caller's model wants subtracted (e.g. a capital lease, a shareholder
	// loan, preferred equity treated as a debt-like claim).
	OtherDebt float64 `json:"other_debt,omitempty"`
}

// Input is the EBITDA multiple method's strongly-typed input.
type Input struct {
	// MaintainableEBITDA is the single maintainable-EBITDA figure this
	// method prices. This package does not derive it — see the package doc
	// comment.
	MaintainableEBITDA float64 `json:"maintainable_ebitda"`
	// Multiple is the EBITDA pricing multiple to apply. Must be > 0.
	Multiple float64 `json:"multiple"`
	// EquityBridge optionally requests the Enterprise-Value-to-Equity-Value
	// bridge. See EquityBridgeInput's doc comment.
	EquityBridge EquityBridgeInput `json:"equity_bridge,omitempty"`
}

// Result is the EBITDA multiple method's output.
type Result struct {
	// Method is this method's stable Code.
	Method valuation.Code `json:"method"`
	// MethodVersion is the Version Calculate ran under.
	MethodVersion string `json:"method_version"`
	// ValueType is always valuation.ValueTypeEnterprise for this method's
	// direct result — see the package doc comment. The bridged figure, when
	// requested, is reported separately via Bridge.EquityValue, never by
	// changing this field.
	ValueType valuation.ValueType `json:"value_type"`
	// Input echoes the exact Input Calculate was given.
	Input Input `json:"input"`
	// Available is false only if EnterpriseValue could not be computed at
	// all (a blocking validation error). A zero or negative
	// MaintainableEBITDA still produces Available == true, per the "do not
	// silently invent a positive number" rule — see Calculate's doc
	// comment.
	Available bool `json:"available"`
	// EnterpriseValue is MaintainableEBITDA * Multiple. Meaningful only when
	// Available is true.
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

// Calculate computes the EBITDA multiple method's enterprise value:
//
//	Enterprise Value = Maintainable EBITDA x Multiple
//
// and, if Input.EquityBridge.Requested, the bridge to Equity Value:
//
//	Equity Value = Enterprise Value + Excess Cash - Total Debt
//
// Calculate never panics on bad financial input. A non-finite
// MaintainableEBITDA/Multiple or a non-positive Multiple is a blocking
// SeverityError (Result.Available == false, Result.EnterpriseValue == 0).
// A zero or negative MaintainableEBITDA is NOT a blocking error — it is a
// real, calculable (if unappealing) outcome, reported as a
// SeverityWarning, and Calculate does not clamp the resulting negative
// enterprise value up to zero.
func Calculate(input Input) Result {
	result := Result{
		Method:        Code,
		MethodVersion: Version,
		ValueType:     valuation.ValueTypeEnterprise,
		Input:         input,
	}

	var issues []valuation.Issue

	if !isFinite(input.MaintainableEBITDA) {
		issues = append(issues, valuation.Issue{
			Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
			Message: "maintainable EBITDA is not a finite number",
		})
	}
	if !isFinite(input.Multiple) {
		issues = append(issues, valuation.Issue{
			Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
			Message: "multiple is not a finite number",
		})
	} else if input.Multiple <= 0 {
		issues = append(issues, valuation.Issue{
			Code: IssueNonPositiveMultiple, Severity: valuation.SeverityError,
			Message: "multiple must be greater than zero",
		})
	}
	if input.EquityBridge.Requested {
		issues = append(issues, validateBridgeInputs(input.EquityBridge)...)
	}

	if valuation.HasErrors(issues) {
		result.Errors = valuation.Errors(issues)
		result.Warnings = valuation.Warnings(issues)
		return result
	}

	if input.MaintainableEBITDA <= 0 {
		issues = append(issues, valuation.Issue{
			Code: IssueNonPositiveEBITDA, Severity: valuation.SeverityWarning,
			Message: "maintainable EBITDA is zero or negative; the resulting enterprise value will be zero or negative",
		})
	}

	enterpriseValue := input.MaintainableEBITDA * input.Multiple

	result.Available = true
	result.EnterpriseValue = enterpriseValue
	result.Steps = []valuation.Step{
		{
			Label:  "Enterprise Value = Maintainable EBITDA x Multiple",
			Value:  enterpriseValue,
			Detail: fmt.Sprintf("%v x %v", input.MaintainableEBITDA, input.Multiple),
		},
	}

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
// finiteness, independent of whether MaintainableEBITDA/Multiple are valid.
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

// buildBridge applies the Enterprise-Value-to-Equity-Value bridge. Callers
// only reach here after validateBridgeInputs has confirmed every field is
// finite.
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
