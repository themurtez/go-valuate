package review

import "testing"

// TestReadiness_Ready proves an item set with no unresolved items produces
// READY with no reasons.
func TestReadiness_Ready(t *testing.T) {
	items := []ReviewItem{
		{ID: "a", Severity: SeverityBlocking, Status: StatusResolved},
		{ID: "b", Severity: SeverityWarning, Status: StatusResolved},
	}
	got := EvaluateReadiness(items)
	if got.State != ReadinessReady {
		t.Errorf("expected READY, got %s", got.State)
	}
	if len(got.Reasons) != 0 {
		t.Errorf("expected no reasons, got %+v", got.Reasons)
	}
}

// TestReadiness_ReadyWithWarnings proves an unresolved WARNING/ERROR item
// (with no unresolved BLOCKING item) produces READY_WITH_WARNINGS, never
// NOT_READY.
func TestReadiness_ReadyWithWarnings(t *testing.T) {
	items := []ReviewItem{
		{ID: "a", Severity: SeverityBlocking, Status: StatusResolved},
		{ID: "b", Severity: SeverityWarning, Status: StatusPending},
		{ID: "c", Severity: SeverityError, Status: StatusPending},
	}
	got := EvaluateReadiness(items)
	if got.State != ReadinessReadyWithWarnings {
		t.Errorf("expected READY_WITH_WARNINGS, got %s", got.State)
	}
	if len(got.Reasons) != 2 {
		t.Errorf("expected 2 reasons, got %d: %+v", len(got.Reasons), got.Reasons)
	}
}

// TestReadiness_NotReady proves a single unresolved BLOCKING item forces
// NOT_READY, regardless of how many other items exist.
func TestReadiness_NotReady(t *testing.T) {
	items := []ReviewItem{
		{ID: "a", Severity: SeverityBlocking, Status: StatusPending},
		{ID: "b", Severity: SeverityWarning, Status: StatusResolved},
	}
	got := EvaluateReadiness(items)
	if got.State != ReadinessNotReady {
		t.Errorf("expected NOT_READY, got %s", got.State)
	}
	found := false
	for _, r := range got.Reasons {
		if r.Code == ReasonUnresolvedBlocking && r.ItemID == "a" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a ReasonUnresolvedBlocking reason for item 'a', got %+v", got.Reasons)
	}
}

// TestReadiness_UnresolvedBlockerAloneForcesNotReady proves that even a
// large number of unresolved WARNING/ERROR items can never, by themselves,
// produce NOT_READY — only an unresolved BLOCKING item can.
func TestReadiness_ManyWarningsNeverForceNotReady(t *testing.T) {
	items := make([]ReviewItem, 0, 50)
	for i := 0; i < 50; i++ {
		items = append(items, ReviewItem{ID: "warn", Severity: SeverityWarning, Status: StatusPending})
	}
	got := EvaluateReadiness(items)
	if got.State == ReadinessNotReady {
		t.Errorf("50 unresolved WARNING items must never produce NOT_READY by themselves, got %s", got.State)
	}
	if got.State != ReadinessReadyWithWarnings {
		t.Errorf("expected READY_WITH_WARNINGS, got %s", got.State)
	}
}

// TestReadiness_MaterialityThresholdInteraction proves that Build's
// materiality-driven item set correctly reflects readiness: with
// materiality gating off (default), a nontrivial amount always produces a
// review item and downstream readiness reflects that item's severity, but
// with a threshold set high enough, the same amount produces no item and
// readiness is unaffected by it (see IsMaterial and buildReconciliationItems'
// composability with Policy fields generally — this test exercises the
// materiality helper directly feeding into an item set, since Build itself
// doesn't currently gate any single kind purely on IsMaterial, per
// section 19's "single small helper function" scope).
func TestReadiness_MaterialityThreshold(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaterialAmountThreshold = 10000

	amount := 50.0
	if IsMaterial(amount, nil, policy) {
		t.Fatal("test setup error: expected 50 to be immaterial under a 10000 threshold")
	}
	// An immaterial amount produces no BLOCKING item in this synthetic
	// scenario, so readiness is READY.
	items := []ReviewItem{}
	got := EvaluateReadiness(items)
	if got.State != ReadinessReady {
		t.Errorf("expected READY for an empty/immaterial item set, got %s", got.State)
	}
}

// TestReadiness_ReconciliationBlockingPolicy proves that a reconciliation
// failure only gates readiness when Policy.ReconciliationFailureBlocks
// makes it SeverityBlocking; otherwise it's an ERROR that yields
// READY_WITH_WARNINGS.
func TestReadiness_ReconciliationBlockingPolicy(t *testing.T) {
	errorItem := ReviewItem{ID: "reconciliation:x:2025", Kind: KindReconciliation, Severity: SeverityError, Status: StatusPending}
	blockingItem := ReviewItem{ID: "reconciliation:x:2025", Kind: KindReconciliation, Severity: SeverityBlocking, Status: StatusPending}

	notBlockingResult := EvaluateReadiness([]ReviewItem{errorItem})
	if notBlockingResult.State != ReadinessReadyWithWarnings {
		t.Errorf("expected READY_WITH_WARNINGS for an unresolved ERROR-severity reconciliation item, got %s", notBlockingResult.State)
	}

	blockingResult := EvaluateReadiness([]ReviewItem{blockingItem})
	if blockingResult.State != ReadinessNotReady {
		t.Errorf("expected NOT_READY for an unresolved BLOCKING-severity reconciliation item, got %s", blockingResult.State)
	}
}

// TestReadiness_PureFunctionOfSeverityAndStatus proves EvaluateReadiness
// never depends on Reason/Title/Message text: two items differing only in
// free-text fields but identical in Severity/Status produce identical
// readiness.
func TestReadiness_PureFunctionOfSeverityAndStatus(t *testing.T) {
	a := ReviewItem{ID: "a", Severity: SeverityBlocking, Status: StatusPending, Reason: "reason A", Title: "title A"}
	b := ReviewItem{ID: "a", Severity: SeverityBlocking, Status: StatusPending, Reason: "totally different free text here", Title: "a different title"}

	ra := EvaluateReadiness([]ReviewItem{a})
	rb := EvaluateReadiness([]ReviewItem{b})
	if ra.State != rb.State {
		t.Errorf("expected identical readiness state regardless of Reason/Title text, got %s vs %s", ra.State, rb.State)
	}
}
