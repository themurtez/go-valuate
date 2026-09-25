package advisory

import "testing"

// TestDedupeActions_SameIdentityMergesProvenance covers task section 118's
// action-dedup fixture: the same AR reconciliation problem arriving from
// reconciliation, close_quality, and close_checklist, one identity ->
// one action with three provenance sources.
func TestDedupeActions_SameIdentityMergesProvenance(t *testing.T) {
	items := []ActionItem{
		newGeneratedAction(actionTemplates[ActionResolveReconciliationBlocker], PriorityHigh, "reconciliation", "BALANCE_MISMATCH", []SourceRef{{Module: "reconciliation", Code: "BALANCE_MISMATCH", Period: "2026-01"}}, "AR-CONTROL", "2026-01"),
		newGeneratedAction(actionTemplates[ActionResolveReconciliationBlocker], PriorityCritical, "close_quality", "FindingUnreconciledCriticalAccount", []SourceRef{{Module: "close_quality", Code: "FindingUnreconciledCriticalAccount", Period: "2026-01"}}, "AR-CONTROL", "2026-01"),
		newGeneratedAction(actionTemplates[ActionResolveReconciliationBlocker], PriorityHigh, "close_checklist", "FindingTaskBlockedByGate", []SourceRef{{Module: "close_checklist", Code: "FindingTaskBlockedByGate", Period: "2026-01"}}, "AR-CONTROL", "2026-01"),
	}
	// Force identical ActionCode so identity (ActionCode+EntityRef+Period)
	// actually matches across all three — newGeneratedAction already uses
	// the same tmpl.Code for all three above since they share the same
	// actionTemplate.

	out := dedupeActions(items)

	if len(out) != 1 {
		t.Fatalf("expected exactly 1 deduplicated action, got %d: %+v", len(out), out)
	}
	got := out[0]
	if len(got.SourceRefs) < 3 {
		t.Errorf("expected at least 3 SourceRefs preserved (one per contributing module), got %d: %+v", len(got.SourceRefs), got.SourceRefs)
	}
	// Highest urgency among contributors must win (PriorityCritical).
	if got.Priority != PriorityCritical {
		t.Errorf("Priority = %v, want PriorityCritical (the most urgent contributor)", got.Priority)
	}
}

func TestDedupeActions_DifferentEntityRefNeverMerged(t *testing.T) {
	items := []ActionItem{
		newGeneratedAction(actionTemplates[ActionReviewOverdueAR], PriorityMedium, "ar", "over_90", nil, "CUST-1", "2026-01"),
		newGeneratedAction(actionTemplates[ActionReviewOverdueAR], PriorityMedium, "ar", "over_90", nil, "CUST-2", "2026-01"),
	}

	out := dedupeActions(items)

	if len(out) != 2 {
		t.Fatalf("different EntityRef must never merge, got %d actions, want 2", len(out))
	}
}

func TestDedupeActions_NeverFuzzyMatchesOnTitleSimilarity(t *testing.T) {
	// Two actions with completely different ActionCode but coincidentally
	// similar-looking Title/Description text must NOT merge — task
	// section 35's "do not fuzzy-match titles/descriptions" rule.
	a := newGeneratedAction(actionTemplates[ActionReviewOverdueAR], PriorityMedium, "ar", "code_a", nil, "CUST-1", "2026-01")
	b := newGeneratedAction(actionTemplates[ActionReviewOverdueAP], PriorityMedium, "ap", "code_b", nil, "CUST-1", "2026-01")

	out := dedupeActions([]ActionItem{a, b})

	if len(out) != 2 {
		t.Fatalf("distinct ActionCode must never merge regardless of text similarity, got %d actions, want 2", len(out))
	}
}

func TestDedupeActions_BlockingNeverDowngraded(t *testing.T) {
	nonBlocking := ActionItem{ActionCode: "X", EntityRef: "E", Period: "P", Blocking: false, Priority: PriorityLow}
	blocking := ActionItem{ActionCode: "X", EntityRef: "E", Period: "P", Blocking: true, Priority: PriorityLow}

	out := dedupeActions([]ActionItem{nonBlocking, blocking})

	if len(out) != 1 || !out[0].Blocking {
		t.Fatalf("expected the merged action to remain Blocking (OR semantics), got %+v", out)
	}
}

func TestDedupeActions_EmptyInput(t *testing.T) {
	if out := dedupeActions(nil); out != nil {
		t.Fatalf("dedupeActions(nil) = %+v, want nil", out)
	}
}
