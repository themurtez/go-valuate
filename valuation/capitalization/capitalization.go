// Package capitalization implements the capitalization-of-earnings
// valuation method: a single maintainable-earnings figure divided by a
// caller-selected capitalization rate.
//
// This package does not compute maintainable earnings itself (see
// financial/earnings for selecting one across historical periods) and does
// not invent a capitalization rate on the caller's behalf — a
// capitalization rate is normally derived from a build-up method (risk-free
// rate + risk premiums - expected long-run growth), which is a distinct
// piece of domain judgment this package deliberately leaves to the caller
// rather than guessing at. Every function here is pure: no I/O, no
// mutation of its inputs.
//
// Value type. Capitalization of earnings, in conventional valuation
// practice, is most often applied directly to an earnings stream already
// understood to belong to equity holders (e.g. maintainable SDE or
// maintainable net income available to owners) and produces a value of
// that same claim — an Equity Value, not an Enterprise Value. This package
// does not attempt to guess which earnings base the caller supplied
// (SDE-like vs. debt-free-EBIT-like); Calculate always reports
// valuation.ValueTypeEquity, and Input's doc comment states this
// expectation explicitly so a caller using a debt-free earnings base
// knows to interpret (or bridge) the result accordingly rather than being
// misled by an unlabeled number.
package capitalization

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/valuation"
)

// Code is this method's stable identifier. See
// valuation.CodeCapitalizationOfEarnings.
const Code = valuation.CodeCapitalizationOfEarnings

// Version is this method's calculation-logic version. See sde.Version's doc
// comment for what incrementing it means.
const Version = "1.0.0"

// Validation issue codes specific to this method.
const (
	// IssueNonPositiveCapRate means Input.CapitalizationRate was <= 0.
	// Division by a non-positive rate has no defensible interpretation
	// here: a zero rate is an undefined division, and a negative rate would
	// invert the direction of the formula (dividing by a negative number
	// flips sign), which this package refuses to auto-invent a meaning for
	// rather than silently producing a value under an unstated alternate
	// convention.
	IssueNonPositiveCapRate valuation.IssueCode = "NON_POSITIVE_CAPITALIZATION_RATE"
	// IssueNonPositiveEarnings is a warning noting that
	// Input.MaintainableEarnings was zero or negative.
	IssueNonPositiveEarnings valuation.IssueCode = "NON_POSITIVE_EARNINGS"
)

// Input is the capitalization-of-earnings method's strongly-typed input.
type Input struct {
	// MaintainableEarnings is the single maintainable-earnings figure this
	// method capitalizes (e.g. financial/earnings.Result.Value). This
	// package does not derive it, and does not care whether the caller's
	// earnings base is SDE-like or EBIT-like — see the package doc comment
	// on why the result is always reported as an Equity Value regardless.
	MaintainableEarnings float64 `json:"maintainable_earnings"`
	// CapitalizationRate is the caller-supplied capitalization rate,
	// expressed as a decimal (0.20 = 20%). Must be > 0. This package never
	// derives one itself (e.g. via a build-up method) — see the package doc
	// comment.
	CapitalizationRate float64 `json:"capitalization_rate"`
}

// Result is the capitalization-of-earnings method's output.
type Result struct {
	// Method is this method's stable Code.
	Method valuation.Code `json:"method"`
	// MethodVersion is the Version Calculate ran under.
	MethodVersion string `json:"method_version"`
	// ValueType is always valuation.ValueTypeEquity for this method — see
	// the package doc comment.
	ValueType valuation.ValueType `json:"value_type"`
	// Input echoes the exact Input Calculate was given.
	Input Input `json:"input"`
	// Available is false only if EquityValue could not be computed at all
	// (a blocking validation error — non-finite input or a non-positive
	// capitalization rate). A zero or negative MaintainableEarnings still
	// produces Available == true — see Calculate's doc comment.
	Available bool `json:"available"`
	// EquityValue is MaintainableEarnings / CapitalizationRate. Meaningful
	// only when Available is true.
	EquityValue float64 `json:"equity_value"`
	// Steps is the full calculation trace.
	Steps []valuation.Step `json:"steps,omitempty"`
	// Warnings carries non-blocking Issues.
	Warnings []valuation.Issue `json:"warnings,omitempty"`
	// Errors carries blocking Issues that made Available false.
	Errors []valuation.Issue `json:"errors,omitempty"`
}

// Calculate computes the capitalization-of-earnings method's equity value:
//
//	Equity Value = Maintainable Earnings / Capitalization Rate
//
// Calculate never panics on bad financial input. A non-finite
// MaintainableEarnings/CapitalizationRate, or a CapitalizationRate <= 0, is
// a blocking SeverityError (Result.Available == false,
// Result.EquityValue == 0) — see IssueNonPositiveCapRate's doc comment for
// why a negative rate is rejected outright rather than accepted under some
// unstated alternate convention. A zero or negative MaintainableEarnings is
// NOT a blocking error: dividing a non-positive earnings figure by a valid
// positive rate is a well-defined (if unappealing) calculation, reported as
// a SeverityWarning, and Calculate does not clamp the resulting
// non-positive value up to zero.
func Calculate(input Input) Result {
	result := Result{
		Method:        Code,
		MethodVersion: Version,
		ValueType:     valuation.ValueTypeEquity,
		Input:         input,
	}

	var issues []valuation.Issue

	if !isFinite(input.MaintainableEarnings) {
		issues = append(issues, valuation.Issue{
			Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
			Message: "maintainable earnings is not a finite number",
		})
	}
	if !isFinite(input.CapitalizationRate) {
		issues = append(issues, valuation.Issue{
			Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
			Message: "capitalization rate is not a finite number",
		})
	} else if input.CapitalizationRate <= 0 {
		issues = append(issues, valuation.Issue{
			Code: IssueNonPositiveCapRate, Severity: valuation.SeverityError,
			Message: "capitalization rate must be greater than zero",
		})
	}

	if valuation.HasErrors(issues) {
		result.Errors = valuation.Errors(issues)
		result.Warnings = valuation.Warnings(issues)
		return result
	}

	if input.MaintainableEarnings <= 0 {
		issues = append(issues, valuation.Issue{
			Code: IssueNonPositiveEarnings, Severity: valuation.SeverityWarning,
			Message: "maintainable earnings is zero or negative; the resulting equity value will be zero or negative",
		})
	}

	equityValue := input.MaintainableEarnings / input.CapitalizationRate

	result.Available = true
	result.EquityValue = equityValue
	result.Steps = []valuation.Step{
		{
			Label:  "Equity Value = Maintainable Earnings / Capitalization Rate",
			Value:  equityValue,
			Detail: fmt.Sprintf("%v / %v", input.MaintainableEarnings, input.CapitalizationRate),
		},
	}
	result.Errors = valuation.Errors(issues)
	result.Warnings = valuation.Warnings(issues)
	return result
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
