package kpi

// TargetKind is the shape of a caller-supplied target — task section 22.
// This package never invents a target; TargetPolicy is always exactly
// what the caller supplied (or absent).
type TargetKind string

const (
	TargetMinimum TargetKind = "MINIMUM"
	TargetMaximum TargetKind = "MAXIMUM"
	TargetRange   TargetKind = "RANGE"
	TargetExact   TargetKind = "EXACT"
)

func isRecognizedTargetKind(k TargetKind) bool {
	switch k {
	case TargetMinimum, TargetMaximum, TargetRange, TargetExact:
		return true
	default:
		return false
	}
}

// TargetPolicy configures one KPI's target — task section 22. Exactly one
// shape applies per Kind:
//
//	MINIMUM: Min is the floor;  met when value >= Min
//	MAXIMUM: Max is the ceiling; met when value <= Max
//	RANGE:   met when Min <= value <= Max (both inclusive)
//	EXACT:   met when value == Exact (within Tolerance, default 0)
type TargetPolicy struct {
	Kind TargetKind `json:"kind"`
	Min  float64    `json:"min,omitempty"`
	Max  float64    `json:"max,omitempty"`
	// Exact is the target value for TargetExact.
	Exact float64 `json:"exact,omitempty"`
	// Tolerance is the absolute allowance around Exact within which the
	// target still counts as met, for TargetExact only. Zero means exact
	// equality (within amountTolerance floating-point slack — see
	// evaluate.go).
	Tolerance float64 `json:"tolerance,omitempty"`
}

// valid reports whether t is structurally usable: Kind is recognized, and
// RANGE additionally requires Min <= Max.
func (t TargetPolicy) valid() bool {
	if !isRecognizedTargetKind(t.Kind) {
		return false
	}
	if t.Kind == TargetRange && t.Min > t.Max {
		return false
	}
	if t.Tolerance < 0 {
		return false
	}
	return true
}

// TargetEvaluation is the result of comparing a KPI's Value against its
// TargetPolicy — task section 22's requested output fields.
type TargetEvaluation struct {
	// TargetAvailable is false when no TargetPolicy was configured, or the
	// KPI's own Value was unavailable, or the TargetPolicy itself was
	// structurally invalid (see IssueInvalidTarget) — every other field is
	// then zero-value.
	TargetAvailable bool `json:"target_available"`
	// TargetMet is only meaningful when TargetAvailable is true.
	TargetMet bool `json:"target_met"`
	// Difference is Value - the nearest target boundary that was violated
	// (0 if met, or for RANGE, the signed distance to whichever bound was
	// crossed). Meaningful only when TargetAvailable.
	Difference Value `json:"difference"`
	// DifferencePercent is Difference / the relevant target boundary * 100
	// — "where meaningful" (task section 22): unavailable when the
	// relevant boundary is 0.
	DifferencePercent Value `json:"difference_percent"`
}

// evaluateTarget compares value against policy. amount/available come
// from the KPI's already-evaluated Value.
func evaluateTarget(policy TargetPolicy, configured bool, value Value) TargetEvaluation {
	if !configured || !policy.valid() || !value.Available {
		return TargetEvaluation{}
	}
	switch policy.Kind {
	case TargetMinimum:
		diff := value.Amount - policy.Min
		return targetEvalResult(value.Amount >= policy.Min, diff, policy.Min)
	case TargetMaximum:
		diff := value.Amount - policy.Max
		return targetEvalResult(value.Amount <= policy.Max, diff, policy.Max)
	case TargetExact:
		diff := value.Amount - policy.Exact
		met := absFloat(diff) <= policy.Tolerance+amountTolerance
		return targetEvalResult(met, diff, policy.Exact)
	case TargetRange:
		switch {
		case value.Amount < policy.Min:
			return targetEvalResult(false, value.Amount-policy.Min, policy.Min)
		case value.Amount > policy.Max:
			return targetEvalResult(false, value.Amount-policy.Max, policy.Max)
		default:
			return targetEvalResult(true, 0, 0)
		}
	default:
		return TargetEvaluation{}
	}
}

func targetEvalResult(met bool, diff, boundary float64) TargetEvaluation {
	te := TargetEvaluation{
		TargetAvailable: true,
		TargetMet:       met,
		Difference:      availableValue(diff),
	}
	if boundary != 0 {
		te.DifferencePercent = availableValue(diff / boundary * 100)
	}
	return te
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
