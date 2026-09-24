package closechecklist

// GateStatus is the fixed set of statuses a portable external gate fact
// may carry.
type GateStatus string

const (
	GatePass          GateStatus = "PASS"
	GateWarning       GateStatus = "WARNING"
	GateFail          GateStatus = "FAIL"
	GateUnavailable   GateStatus = "UNAVAILABLE"
	GateNotApplicable GateStatus = "NOT_APPLICABLE"
)

func isRecognizedGateStatus(s GateStatus) bool {
	switch s {
	case GatePass, GateWarning, GateFail, GateUnavailable, GateNotApplicable:
		return true
	default:
		return false
	}
}

// GateFact is a portable, typed fact about one external condition (a
// closequality dimension, a reconciliation result, any other sibling
// module's status) that a task's GateRules can reference by GateCode.
// This package never fetches or computes a GateFact itself — every one
// is supplied by the caller, typically via one of the adapters in
// adapters.go.
type GateFact struct {
	GateCode     string     `json:"gate_code"`
	Status       GateStatus `json:"status"`
	SourceModule string     `json:"source_module,omitempty"`
	SourceCode   string     `json:"source_code,omitempty"`
	SourceRef    string     `json:"source_ref,omitempty"`
}

// GateRequirement is the fixed set of gate-satisfaction rules a task may
// declare.
type GateRequirement string

const (
	GateRequirePass          GateRequirement = "PASS"
	GateRequirePassOrWarning GateRequirement = "PASS_OR_WARNING"
	GateRequireAvailable     GateRequirement = "AVAILABLE"
)

func isRecognizedGateRequirement(r GateRequirement) bool {
	switch r {
	case GateRequirePass, GateRequirePassOrWarning, GateRequireAvailable:
		return true
	default:
		return false
	}
}

// GateRule attaches one gate requirement to a task. Required, when
// false, means a missing/failed gate is reported as informational only
// and never contributes to Satisfied=false or an effective blocker —
// but the underlying gate status is still surfaced via TaskResult.Gates
// either way.
type GateRule struct {
	GateCode string          `json:"gate_code"`
	Require  GateRequirement `json:"require"`
	Required bool            `json:"required"`
}

// gateSatisfied reports whether fact (which may be absent — ok=false)
// satisfies rule. A missing required gate is never implicitly PASS
// (section 11).
func gateSatisfied(rule GateRule, fact GateFact, ok bool) bool {
	if !ok {
		return false
	}
	switch rule.Require {
	case GateRequirePass:
		return fact.Status == GatePass || fact.Status == GateNotApplicable
	case GateRequirePassOrWarning:
		return fact.Status == GatePass || fact.Status == GateWarning || fact.Status == GateNotApplicable
	case GateRequireAvailable:
		return fact.Status != GateUnavailable
	default:
		return false
	}
}
