package closechecklist

// ApplicabilityRuleType is the fixed, small set of applicability rule
// kinds this package supports. Deliberately not a generic expression
// engine — see the package's task spec section 6.
type ApplicabilityRuleType string

const (
	// ApplicabilityAlways always resolves APPLICABLE.
	ApplicabilityAlways ApplicabilityRuleType = "ALWAYS"
	// ApplicabilityCallerFlag resolves APPLICABLE/NOT_APPLICABLE based on
	// a caller-supplied boolean flag (Instance.ApplicabilityFlags, keyed
	// by ApplicabilityRule.FlagKey). A missing flag key resolves
	// UNDETERMINED.
	ApplicabilityCallerFlag ApplicabilityRuleType = "CALLER_FLAG"
	// ApplicabilityExternalGatePresent resolves APPLICABLE when a named
	// gate fact (ApplicabilityRule.GateCode) is present in
	// Instance.Gates at all (any Status, including FAIL — presence, not
	// pass/fail, drives applicability here), NOT_APPLICABLE when it is
	// absent.
	ApplicabilityExternalGatePresent ApplicabilityRuleType = "EXTERNAL_GATE_PRESENT"
)

// Applicability is the resolved applicability state for one task.
type Applicability string

const (
	ApplicableYes          Applicability = "APPLICABLE"
	ApplicableNo           Applicability = "NOT_APPLICABLE"
	ApplicableUndetermined Applicability = "UNDETERMINED"
)

// ApplicabilityRule declares how one TaskDefinition's applicability is
// resolved. The zero value (empty Type) is treated as ALWAYS.
type ApplicabilityRule struct {
	Type     ApplicabilityRuleType `json:"type,omitempty"`
	FlagKey  string                `json:"flag_key,omitempty"`
	GateCode string                `json:"gate_code,omitempty"`
}

func isRecognizedApplicabilityRuleType(t ApplicabilityRuleType) bool {
	switch t {
	case "", ApplicabilityAlways, ApplicabilityCallerFlag, ApplicabilityExternalGatePresent:
		return true
	default:
		return false
	}
}

// ApplicabilityOverride is an explicit caller override for one task's
// applicability, taking precedence over the task's own ApplicabilityRule
// evaluation entirely (section 6: "Support explicit caller overrides").
type ApplicabilityOverride struct {
	TaskCode      string        `json:"task_code"`
	Applicability Applicability `json:"applicability"`
	Reason        string        `json:"reason,omitempty"`
}

// resolveApplicability evaluates one task's applicability given an
// override index, caller flags, and the indexed gate facts. Overrides
// win unconditionally.
func resolveApplicability(taskCode string, rule ApplicabilityRule, overrides map[string]Applicability, flags map[string]bool, gates map[string]GateFact) Applicability {
	if a, ok := overrides[taskCode]; ok {
		return a
	}
	switch rule.Type {
	case "", ApplicabilityAlways:
		return ApplicableYes
	case ApplicabilityCallerFlag:
		v, ok := flags[rule.FlagKey]
		if !ok {
			return ApplicableUndetermined
		}
		if v {
			return ApplicableYes
		}
		return ApplicableNo
	case ApplicabilityExternalGatePresent:
		if _, ok := gates[rule.GateCode]; ok {
			return ApplicableYes
		}
		return ApplicableNo
	default:
		return ApplicableUndetermined
	}
}
