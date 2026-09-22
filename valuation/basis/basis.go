// Package basis implements explicit, auditable conversion of a single
// valuation method result from its native valuation.ValueType to a
// caller-chosen target basis, so a consensus/comparison across several
// methods never silently averages an Enterprise Value against an Equity
// Value against a net asset value.
//
// This package invents no new arithmetic: every conversion it performs
// reuses a figure a method itself already computed (its own
// valuation.Bridge, when the method's Input requested one) or is a
// documented identity (asset value and equity value are both an
// "assets minus liabilities" figure — see Convert's doc comment on the
// asset_value -> equity_value case). Where no deterministic path exists
// between two bases in this repository (e.g. equity value -> enterprise
// value has no bridge formula defined anywhere), Convert says so
// explicitly rather than guessing at one.
//
// Every function here is pure: no I/O, no mutation of its inputs.
package basis

import "github.com/themurtez/go-valuate/valuation"

// ConversionInput is one method result to attempt to express on a target
// basis: its native value/basis, plus whichever valuation.Bridge the
// method itself computed (the zero Bridge — Available == false — if the
// method's Input never requested one, or the method has no such concept,
// e.g. valuation/capitalization).
type ConversionInput struct {
	// Method identifies which valuation method Value came from.
	Method valuation.Code
	// ValueType is the basis Value is natively expressed on.
	ValueType valuation.ValueType
	// Value is the method's headline figure, on ValueType's basis.
	Value float64
	// Bridge is the method's own already-computed Enterprise-Value-to-
	// Equity-Value bridge (valuation.Bridge), if any. Convert never
	// recomputes bridge arithmetic itself — it only reads Bridge.EquityValue
	// when Bridge.Available is true. A zero Bridge is treated exactly like
	// "no bridge available," never like "a bridge computed to zero."
	Bridge valuation.Bridge
}

// Outcome classifies what Convert did with a single ConversionInput.
type Outcome string

const (
	// OutcomeDirect means Value was already expressed on the target basis
	// — no conversion was needed or performed.
	OutcomeDirect Outcome = "direct"
	// OutcomeConverted means Value was successfully converted to the
	// target basis using an already-computed Bridge (enterprise -> equity)
	// or a documented identity (asset -> equity). See Conversion.Inputs.
	OutcomeConverted Outcome = "converted"
	// OutcomeExcluded means Value could not be expressed on the target
	// basis at all — no deterministic conversion path exists in this
	// repository for the (original basis, target basis) pair, or one
	// exists in principle (enterprise -> equity) but the method's own
	// Bridge was not requested/available. See Conversion.ExclusionReason.
	OutcomeExcluded Outcome = "excluded"
)

// Conversion is the outcome of attempting to express one ConversionInput
// on a target basis — always returned, for every input, whether or not
// conversion succeeded, so a caller/report can show exactly what happened
// to every method rather than having failed conversions silently vanish.
type Conversion struct {
	// Method echoes ConversionInput.Method.
	Method valuation.Code `json:"method"`
	// TargetBasis is the basis Convert was asked to express this method's
	// value on.
	TargetBasis valuation.ValueType `json:"target_basis"`
	// Outcome classifies what happened — see the Outcome constants.
	Outcome Outcome `json:"outcome"`
	// OriginalValue is the method's native headline value, unconverted.
	OriginalValue float64 `json:"original_value"`
	// OriginalBasis is the method's native ValueType.
	OriginalBasis valuation.ValueType `json:"original_basis"`
	// ConvertedValue is OriginalValue expressed on TargetBasis. Meaningful
	// only when Outcome is OutcomeDirect (equal to OriginalValue) or
	// OutcomeConverted; zero when Outcome is OutcomeExcluded — an excluded
	// input must never be read as if it contributed a real (if wrong)
	// number.
	ConvertedValue float64 `json:"converted_value"`
	// Bridge is the valuation.Bridge actually used to produce
	// ConvertedValue from OriginalValue, when the conversion was a real
	// bridge (enterprise -> equity), so a caller can show the full
	// itemized cash/debt walk alongside the converted figure — never
	// folded invisibly into ConvertedValue alone. Available is false (the
	// zero Bridge) for an OutcomeDirect result (nothing was converted) and
	// for the asset -> equity identity case (see Convert's doc comment: no
	// cash/debt arithmetic is involved in that case, so there is no bridge
	// to show).
	Bridge valuation.Bridge `json:"bridge"`
	// ExclusionReason is a short, fixed, human-readable explanation of why
	// no conversion was possible. Set only when Outcome is OutcomeExcluded.
	ExclusionReason string `json:"exclusion_reason,omitempty"`
}

// ConvertAll attempts to express every ConversionInput on target, and
// always returns exactly one Conversion per input, in the same order —
// see Convert for the per-input rules.
func ConvertAll(inputs []ConversionInput, target valuation.ValueType) []Conversion {
	out := make([]Conversion, 0, len(inputs))
	for _, in := range inputs {
		out = append(out, Convert(in, target))
	}
	return out
}

// Convert attempts to express a single ConversionInput on target.
//
// Rules, in order:
//
//  1. in.ValueType == target: OutcomeDirect. Nothing is converted;
//     ConvertedValue == OriginalValue exactly.
//  2. in.ValueType == ValueTypeEnterprise, target == ValueTypeEquity, and
//     in.Bridge.Available: OutcomeConverted, using in.Bridge.EquityValue
//     exactly as the method itself computed it (see
//     valuation.Bridge's doc comment: Equity Value = Enterprise Value +
//     Excess Cash - Total Debt). Convert never recomputes this formula
//     itself — it only reads the method's own result.
//  3. in.ValueType == ValueTypeAsset, target == ValueTypeEquity:
//     OutcomeConverted via an explicit identity, ConvertedValue ==
//     OriginalValue. An adjusted net asset value (Adjusted Assets -
//     Adjusted Liabilities, see valuation/netassets' package doc comment)
//     and a balance-sheet-basis equity value are the same subtraction;
//     this is a basis relabel, not a cash/debt bridge, so Conversion.Bridge
//     stays unavailable — do not confuse this identity with rule 2's real
//     bridge arithmetic. This does NOT mean adjusted net asset value and a
//     going-concern earnings-based equity value are interchangeable in
//     what they represent (see valuation/netassets' doc comment on why
//     its ValueType stays ValueTypeAsset rather than ValueTypeEquity) —
//     only that, once a caller has explicitly chosen to compare on an
//     equity basis, this is the one defensible number to use.
//  4. Anything else (equity -> enterprise in either direction beyond rule
//     2's reverse, enterprise -> equity with no bridge available, asset ->
//     enterprise, or any basis to itself already covered by rule 1):
//     OutcomeExcluded with a structured ExclusionReason. No other
//     conversion path is invented — see the package doc comment.
func Convert(in ConversionInput, target valuation.ValueType) Conversion {
	c := Conversion{
		Method:        in.Method,
		TargetBasis:   target,
		OriginalValue: in.Value,
		OriginalBasis: in.ValueType,
	}

	if in.ValueType == target {
		c.Outcome = OutcomeDirect
		c.ConvertedValue = in.Value
		return c
	}

	if in.ValueType == valuation.ValueTypeEnterprise && target == valuation.ValueTypeEquity {
		if in.Bridge.Available {
			c.Outcome = OutcomeConverted
			c.ConvertedValue = in.Bridge.EquityValue
			c.Bridge = in.Bridge
			return c
		}
		c.Outcome = OutcomeExcluded
		c.ExclusionReason = "enterprise value cannot be converted to equity value: no equity bridge (excess cash/debt) was computed for this result"
		return c
	}

	if in.ValueType == valuation.ValueTypeAsset && target == valuation.ValueTypeEquity {
		c.Outcome = OutcomeConverted
		c.ConvertedValue = in.Value
		return c
	}

	c.Outcome = OutcomeExcluded
	c.ExclusionReason = "no deterministic conversion from " + string(in.ValueType) + " to " + string(target) + " is defined"
	return c
}
