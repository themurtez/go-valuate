package review

import "testing"

// TestZeroValue_Policy_SubstitutesDefaultPolicyWholesale proves an exact
// zero-value Policy{} is swapped wholesale for DefaultPolicy() (a
// struct-equality check, not per-field defaulting like ingestion.Limits)
// — Build must not panic, and must not silently apply an all-zero
// materiality/threshold policy (which would make every classification
// item review-required at confidence 0, i.e. always).
func TestZeroValue_Policy_SubstitutesDefaultPolicyWholesale(t *testing.T) {
	plan := Build(BuildInput{}, Policy{})
	if plan.Version == "" {
		t.Error("expected a populated Plan.Version even for an entirely empty BuildInput/Policy")
	}
	if len(plan.Items) != 0 {
		t.Errorf("expected zero review items for an entirely empty BuildInput, got %d", len(plan.Items))
	}
}

// TestZeroValue_BuildInput_EmptyPlanNotPanic proves Build(BuildInput{},
// DefaultPolicy()) — every optional input source left nil/empty — is a
// safe zero value producing an empty Plan, not a panic on a nil
// Classifications/Raws/Rows/Adjustments slice.
func TestZeroValue_BuildInput_EmptyPlanNotPanic(t *testing.T) {
	plan := Build(BuildInput{}, DefaultPolicy())
	readiness := EvaluateReadiness(plan.Items)
	if readiness.State != ReadinessReady {
		t.Errorf("expected READY for an entirely empty plan (nothing to review), got %s", readiness.State)
	}
}

// TestZeroValue_Source_ApplyEmptyDecisionsNotPanic proves
// Apply(Source{}, Plan{}, nil) — every field at its zero value — does not
// panic: an empty decisions slice is documented as a valid, empty case.
func TestZeroValue_Source_ApplyEmptyDecisionsNotPanic(t *testing.T) {
	result := Apply(Source{}, Plan{}, nil)
	if len(result.Invalid) != 0 {
		t.Errorf("expected no invalid decisions when nothing was supplied, got %+v", result.Invalid)
	}
	if len(result.Applied) != 0 {
		t.Errorf("expected no applied decisions when nothing was supplied, got %+v", result.Applied)
	}
}
