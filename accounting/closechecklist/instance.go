package closechecklist

import "time"

// Instance pairs one Template (by TemplateID+Version) with one Period's
// caller-supplied task states, evidence-bearing state, sign-offs,
// exceptions, gate facts, and applicability flags/overrides. This is
// [Calculate]'s primary input, alongside [Policy].
type Instance struct {
	Template Template `json:"template"`
	Period   Period   `json:"period"`

	// PeriodState is the caller's own tracked lifecycle state for this
	// period — evaluated for consistency (section 23), never mutated or
	// enforced.
	PeriodState PeriodState `json:"period_state"`

	// EvaluationDate is the explicit "as of" date used for every due-date
	// and exception-expiry computation. Required for any DueStatus other
	// than UNAVAILABLE. Never defaulted to time.Now().
	EvaluationDate time.Time `json:"evaluation_date"`

	// TaskStates is keyed implicitly by TaskState.TaskCode; a task with
	// no entry here defaults to NOT_STARTED (see [defaultTaskState]).
	// Duplicate TaskCode entries are reported as an Issue and only the
	// first (in slice order) is used.
	TaskStates []TaskState `json:"task_states,omitempty"`

	// ApplicabilityFlags supplies caller booleans for
	// ApplicabilityCallerFlag rules, keyed by ApplicabilityRule.FlagKey.
	ApplicabilityFlags map[string]bool `json:"applicability_flags,omitempty"`
	// ApplicabilityOverrides are explicit per-task applicability
	// overrides — see section 6.
	ApplicabilityOverrides []ApplicabilityOverride `json:"applicability_overrides,omitempty"`

	// Gates is every external gate fact available for this period,
	// keyed implicitly by GateFact.GateCode. Duplicate GateCode entries
	// are reported as an Issue and only the first is used.
	Gates []GateFact `json:"gates,omitempty"`

	// Exceptions are caller-approved exceptions against specific tasks —
	// see section 22.
	Exceptions []Exception `json:"exceptions,omitempty"`
}

// PriorClose is an optional prior period's Instance + Result, supplied
// only for comparison purposes — see comparison.go. Calculate never
// changes its own Result based on PriorClose beyond populating
// Result.PriorCloseComparison.
type PriorClose struct {
	Instance Instance `json:"instance"`
	Result   Result   `json:"result"`
}
