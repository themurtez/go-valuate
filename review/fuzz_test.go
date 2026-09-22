package review

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
)

// fuzzTestPlan builds a small, fixed, realistic Plan with one
// KindClassification review item — the target every fuzzed Decision below
// is validated against. Kept deterministic and outside the fuzz function
// itself so every fuzz execution validates against the identical Plan,
// isolating ItemID/Action/Code as the only fuzzed variables.
func fuzzTestPlan() Plan {
	raw := financial.RawLineItem{ID: "row-1", Label: "Something Ambiguous", StatementType: financial.StatementIncomeStatement}
	result := classification.Result{Source: classification.SourceUnknown, ReviewRequired: true}
	return Build(BuildInput{
		Classifications: []classification.Result{result},
		Raws:            []financial.RawLineItem{raw},
	}, DefaultPolicy())
}

// FuzzApply_DecisionValidation proves review.Apply — the package's
// central validation surface — never panics on an arbitrary Decision,
// regardless of how malformed ItemID/Action/Code are, and never returns a
// Go error (its documented "no bare error, everything via issues"
// contract): every rejected decision must appear in ApplyResult.Invalid
// with a non-empty review.Issue.Code, and Apply must never mutate its
// plan/decisions inputs even under adversarial fuzzed input (checked via
// a snapshot comparison, mirroring apply_immutability_test.go's approach
// for the package's own hand-written tests).
func FuzzApply_DecisionValidation(f *testing.F) {
	plan := fuzzTestPlan()
	var realItemID string
	for _, it := range plan.Items {
		if it.Kind == KindClassification {
			realItemID = it.ID
		}
	}
	if realItemID == "" {
		f.Fatal("fuzz setup error: expected a KindClassification item in fuzzTestPlan")
	}

	seeds := []struct {
		itemID string
		action string
		code   string
	}{
		{realItemID, "ACCEPT", ""},
		{realItemID, "OVERRIDE", "OPEX_OTHER"},
		{realItemID, "OVERRIDE", ""},
		{realItemID, "OVERRIDE", "NOT_A_REAL_CODE"},
		{realItemID, "IGNORE", ""},
		{realItemID, "REJECT", ""},
		{realItemID, "CONFIRM", ""},
		{"", "ACCEPT", ""},
		{"unknown-item-id", "ACCEPT", ""},
		{realItemID, "", ""},
		{realItemID, "NOT_A_REAL_ACTION", ""},
		{realItemID, "OVERRIDE", "\x00\x01\x02"},
	}
	for _, s := range seeds {
		f.Add(s.itemID, s.action, s.code)
	}

	f.Fuzz(func(t *testing.T, itemID, action, code string) {
		decisions := []Decision{{
			ItemID:         itemID,
			Action:         Action(action),
			Classification: &ClassificationDecision{Code: financial.Code(code)},
		}}

		// Snapshot for the no-mutation check.
		planBefore := marshalForSnapshot(t, plan)
		decisionsBefore := marshalForSnapshot(t, decisions)

		result := Apply(Source{}, plan, decisions)

		if string(marshalForSnapshot(t, plan)) != string(planBefore) {
			t.Fatalf("Apply mutated plan for input itemID=%q action=%q code=%q", itemID, action, code)
		}
		if string(marshalForSnapshot(t, decisions)) != string(decisionsBefore) {
			t.Fatalf("Apply mutated decisions for input itemID=%q action=%q code=%q", itemID, action, code)
		}

		// Every entry in Invalid must carry at least one Issue with a
		// non-empty Code — Apply never rejects a decision without a
		// stable, matchable reason.
		for _, inv := range result.Invalid {
			if len(inv.Issues) == 0 {
				t.Fatalf("Apply produced an Invalid decision with zero Issues for input itemID=%q action=%q code=%q: %+v", itemID, action, code, inv)
			}
			for _, iss := range inv.Issues {
				if iss.Code == "" {
					t.Fatalf("Apply produced an Invalid decision with an empty Issue.Code for input itemID=%q action=%q code=%q: %+v", itemID, action, code, inv)
				}
			}
		}

		// A decision is either applied or invalid, never silently absent
		// from both — every fuzzed decision here targets exactly one
		// element, so exactly one of Applied/Invalid should account for it
		// (Applied may also be empty if the SAME item was already resolved
		// by an earlier decision in a multi-decision call, but this fuzz
		// target always sends exactly one decision, so that ambiguity does
		// not apply here).
		if len(result.Applied) == 0 && len(result.Invalid) == 0 {
			t.Fatalf("Apply accounted for neither Applied nor Invalid for a single supplied decision: itemID=%q action=%q code=%q", itemID, action, code)
		}
	})
}
