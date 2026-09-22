// Package sde implements the Seller's Discretionary Earnings multiple
// valuation method: a single maintainable-SDE figure multiplied by a
// caller-selected multiple.
//
// This package does not compute SDE itself (see financial/metrics for
// baseline SDE and financial/adjustments for the normalized-SDE bridge) and
// does not select a multiple on the caller's behalf (see settings.Settings'
// SDEMultiple for one way a caller might resolve one) — it only knows how
// to combine an already-determined maintainable SDE and multiple into a
// single defensible value, with a fully explainable calculation trace, and
// to validate that combination for obviously invalid input (a non-positive
// multiple, a non-finite value). Every function here is pure: no I/O, no
// mutation of its inputs.
//
// Value type. The SDE multiple method conventionally prices a business for
// a single owner-operator buyer and is understood, by long-standing small-
// business valuation practice, to already reflect what that buyer would pay
// for the business's equity (SDE already includes the return to a working
// owner, which does not sit cleanly on either side of an enterprise-value/
// debt-free convention the way EBITDA does). Calculate therefore reports
// ValueType as valuation.ValueTypeEquity directly, not
// valuation.ValueTypeEnterprise — see Input's doc comment on the optional
// debt/cash bridge for the (unusual, but supported) case where a caller's
// chosen convention wants one anyway.
package sde

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/valuation"
)

// Code is this method's stable identifier. See valuation.CodeSDEMultiple.
const Code = valuation.CodeSDEMultiple

// Version is this method's calculation-logic version, incremented whenever
// Calculate's formula, validation rules, or output shape change in a way
// that could make a historical result computed under an older version not
// reproduce identically under a newer one. Callers persisting a Result are
// expected to persist Version alongside it.
const Version = "1.0.0"

// Validation issue codes specific to this method.
const (
	// IssueNonPositiveMultiple means Input.Multiple was <= 0. A multiple of
	// zero or less has no defensible interpretation as a pricing multiple.
	IssueNonPositiveMultiple valuation.IssueCode = "NON_POSITIVE_MULTIPLE"
	// IssueNonPositiveSDE is a warning (not an error — see Calculate's doc
	// comment) noting that Input.MaintainableSDE was zero or negative.
	IssueNonPositiveSDE valuation.IssueCode = "NON_POSITIVE_SDE"
)

// EquityBridgeInput optionally supplies a debt/cash bridge on top of the
// method's native equity-value result. This exists only for a caller whose
// chosen valuation convention wants to see an SDE-multiple-implied value
// further adjusted for a specific debt-free/cash-free deal structure; it is
// never required, and Calculate reports it as its own inspectable
// valuation.Bridge rather than folding it into MaintainableSDE * Multiple.
// Every field is a plain, non-negative magnitude; a field left at its zero
// value contributes zero, not "unavailable" (see Result.Bridge.Available
// for how a caller signals "no bridge was requested" instead).
type EquityBridgeInput struct {
	// Requested opts into computing Result.Bridge at all. false (the zero
	// value) means no bridge is computed and Result.Bridge.Available is
	// false — the common case, since the method's own result is already an
	// equity value.
	Requested bool `json:"requested"`
	// ExcessCash is cash/cash-equivalents to add.
	ExcessCash float64 `json:"excess_cash,omitempty"`
	// ShortTermDebt is short-term/current debt to subtract.
	ShortTermDebt float64 `json:"short_term_debt,omitempty"`
	// LongTermDebt is long-term debt to subtract.
	LongTermDebt float64 `json:"long_term_debt,omitempty"`
	// OtherDebt is any other caller-identified debt-like obligation to
	// subtract (e.g. a capital lease, a shareholder loan).
	OtherDebt float64 `json:"other_debt,omitempty"`
}

// Input is the SDE multiple method's strongly-typed input.
type Input struct {
	// MaintainableSDE is the single maintainable-SDE figure this method
	// prices (e.g. financial/earnings.Result.Value computed over a
	// financial/adjustments normalized-SDE series). This package does not
	// derive it — see the package doc comment.
	MaintainableSDE float64 `json:"maintainable_sde"`
	// Multiple is the SDE pricing multiple to apply. Must be > 0.
	Multiple float64 `json:"multiple"`
	// EquityBridge optionally requests a debt/cash bridge applied on top of
	// the native MaintainableSDE * Multiple equity result. See
	// EquityBridgeInput's doc comment; almost always left at its zero value.
	EquityBridge EquityBridgeInput `json:"equity_bridge,omitempty"`
}

// Result is the SDE multiple method's output, conforming to the shared
// valuation result envelope (method code/version, value type, calculated
// value, inputs, steps, warnings, errors — see the package doc comment on
// Calculate).
type Result struct {
	// Method is this method's stable Code, echoed for a caller that only
	// has a Result in hand (e.g. after deserializing a persisted one).
	Method valuation.Code `json:"method"`
	// MethodVersion is the Version Calculate ran under.
	MethodVersion string `json:"method_version"`
	// ValueType is always valuation.ValueTypeEquity for this method — see
	// the package doc comment.
	ValueType valuation.ValueType `json:"value_type"`
	// Input echoes the exact Input Calculate was given, so a Result is
	// self-contained and reproducible without the caller separately
	// retaining its own copy of Input.
	Input Input `json:"input"`
	// Available is false if EquityValue could not be computed at all (a
	// blocking validation error — see Errors). Available is true even when
	// MaintainableSDE itself was zero or negative and the resulting
	// EquityValue is consequently zero or negative: a negative
	// maintainable-SDE business is not an invalid calculation, it is a
	// business that a pure SDE multiple values at or below zero, which
	// Warnings flags rather than Errors blocking it — see Calculate's doc
	// comment.
	Available bool `json:"available"`
	// EquityValue is MaintainableSDE * Multiple. Meaningful only when
	// Available is true; zero when Available is false.
	EquityValue float64 `json:"equity_value"`
	// Steps is the full calculation trace, in the order computed.
	Steps []valuation.Step `json:"steps,omitempty"`
	// Bridge is populated only when Input.EquityBridge.Requested is true.
	// See EquityBridgeInput's doc comment.
	Bridge valuation.Bridge `json:"bridge"`
	// Warnings carries non-blocking Issues (e.g. IssueNonPositiveSDE).
	Warnings []valuation.Issue `json:"warnings,omitempty"`
	// Errors carries blocking Issues that made Available false.
	Errors []valuation.Issue `json:"errors,omitempty"`
}

// Calculate computes the SDE multiple method's equity value:
//
//	Equity Value = Maintainable SDE x Multiple
//
// Calculate never panics on bad financial input. A non-finite
// MaintainableSDE/Multiple or a non-positive Multiple is a blocking
// SeverityError (Result.Available == false, Result.EquityValue == 0) since
// no defensible number can be produced. A zero or negative MaintainableSDE
// is deliberately NOT a blocking error: it is a real, calculable outcome
// (a business with no discretionary earnings, or negative earnings, is
// validly priced at zero-or-below by a pure multiple method) and is instead
// reported as a SeverityWarning so a reviewer is alerted without the method
// refusing to produce a number. Calculate does not clamp a negative result
// up to zero — silently producing a "normal-looking" positive valuation
// from negative SDE would misrepresent the calculation.
func Calculate(input Input) Result {
	result := Result{
		Method:        Code,
		MethodVersion: Version,
		ValueType:     valuation.ValueTypeEquity,
		Input:         input,
	}

	var issues []valuation.Issue

	if !isFinite(input.MaintainableSDE) {
		issues = append(issues, valuation.Issue{
			Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
			Message: "maintainable SDE is not a finite number",
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

	if valuation.HasErrors(issues) {
		result.Errors = valuation.Errors(issues)
		result.Warnings = valuation.Warnings(issues)
		return result
	}

	if input.MaintainableSDE <= 0 {
		issues = append(issues, valuation.Issue{
			Code: IssueNonPositiveSDE, Severity: valuation.SeverityWarning,
			Message: "maintainable SDE is zero or negative; the resulting equity value will be zero or negative",
		})
	}

	equityValue := input.MaintainableSDE * input.Multiple

	result.Available = true
	result.EquityValue = equityValue
	result.Steps = []valuation.Step{
		{
			Label:  "Equity Value = Maintainable SDE x Multiple",
			Value:  equityValue,
			Detail: formatMultiplication(input.MaintainableSDE, input.Multiple),
		},
	}
	result.Errors = valuation.Errors(issues)
	result.Warnings = valuation.Warnings(issues)

	if input.EquityBridge.Requested {
		result.Bridge = buildBridge(equityValue, input.EquityBridge)
	}

	finalizeEnvelope(&result)
	return result
}

// finalizeEnvelope runs the shared valuation.ValidateResultEnvelope/
// ValidateFiniteSteps invariant checks (see their doc comments) against
// an already-computed Result, appending any finding to Errors and
// flipping Available to false — a successful Result must never carry an
// unknown method/version/basis or a non-finite calculation step. In
// today's code these checks never fire (Method/MethodVersion/ValueType
// are hard-coded correctly above, and every Step is built from
// already-finite validated inputs); they exist as a regression guard.
func finalizeEnvelope(result *Result) {
	issues := valuation.ValidateResultEnvelope(result.Method, result.MethodVersion, result.ValueType)
	issues = append(issues, valuation.ValidateFiniteSteps(result.Steps)...)
	if len(issues) == 0 {
		return
	}
	result.Errors = append(result.Errors, issues...)
	result.Available = false
	result.EquityValue = 0
}

// buildBridge applies an optional caller-requested debt/cash adjustment on
// top of the method's native equity value. See EquityBridgeInput's doc
// comment: this is not the standard EV->equity bridge (this method's native
// result is already an equity value), so the "EnterpriseValue" field on the
// returned valuation.Bridge is set to the pre-adjustment equity value for
// transparency about what the bridge started from, while ExcessCash/
// TotalDebt/EquityValue keep their usual meaning.
func buildBridge(baseEquityValue float64, in EquityBridgeInput) valuation.Bridge {
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
		EnterpriseValue: baseEquityValue,
		ExcessCash:      in.ExcessCash,
		TotalDebt:       totalDebt,
		DebtComponents:  components,
		EquityValue:     baseEquityValue + in.ExcessCash - totalDebt,
	}
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func formatMultiplication(a, b float64) string {
	return fmt.Sprintf("%v x %v", a, b)
}
