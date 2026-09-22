package review

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
)

// jsonSnapshot marshals v to a JSON string, for a robust deep-equality
// check that doesn't require the caller to hand-write a DeepEqual-friendly
// comparator for every type under test — mirroring this repository's own
// determinism-test convention (see adjustments/determinism_test.go) applied
// here for pre/post structural comparison instead of repeated-call
// comparison.
func jsonSnapshot(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return string(b)
}

// TestApply_DoesNotMutate_Source proves Apply does not mutate any field of
// the Source it is given: a deep JSON snapshot of source taken before
// calling Apply matches an identical snapshot taken after.
func TestApply_DoesNotMutate_Source(t *testing.T) {
	source := Source{
		MappedLineItems: []financial.MappedLineItem{
			{SourceID: "row-1", Label: "Misc", Code: financial.CodeOpexOther, Status: financial.RowStatusNormal, Values: map[financial.Period]float64{"2025": 100}},
		},
		Rows: []RowContext{
			{RowID: "row-1", Label: "Misc", PageIndex: 0, ColumnPeriods: map[int]financial.Period{1: "2025"}},
		},
		Adjustments: []adjustments.Adjustment{
			{ID: "adj-1", Period: "2025", Type: adjustments.TypePersonalVehicle, Amount: 500, Reason: "x", Included: true},
		},
	}
	before := jsonSnapshot(t, source)

	plan := Plan{Items: []ReviewItem{
		{ID: "classification:row-1", Kind: KindClassification, Status: StatusPending},
		{ID: "adjustment:adj-1", Kind: KindAdjustment, Status: StatusPending, Adjustment: &AdjustmentPayload{AdjustmentID: "adj-1"}},
	}}
	decisions := []Decision{
		{ItemID: "classification:row-1", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}},
		{ItemID: "adjustment:adj-1", Action: ActionOverride, Adjustment: &AdjustmentDecision{Included: false, Amount: 999, NewAmount: true}},
	}

	_ = Apply(source, plan, decisions)

	after := jsonSnapshot(t, source)
	if before != after {
		t.Errorf("Apply mutated its Source argument:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestApply_DoesNotMutate_Plan proves Apply does not mutate the Plan it is
// given.
func TestApply_DoesNotMutate_Plan(t *testing.T) {
	plan := Plan{
		Version: SchemaVersion,
		Items: []ReviewItem{
			{
				ID: "classification:row-2", Kind: KindClassification, Status: StatusPending,
				Classification: &ClassificationPayload{ProposedCode: financial.CodeOpexOther, Alternatives: []ClassificationAlternative{{Code: financial.CodeCogsOther, Confidence: 0.5}}},
			},
		},
	}
	before := jsonSnapshot(t, plan)

	source := Source{MappedLineItems: []financial.MappedLineItem{{SourceID: "row-2", Code: financial.CodeOpexOther}}}
	decisions := []Decision{{ItemID: "classification:row-2", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}}}

	_ = Apply(source, plan, decisions)

	after := jsonSnapshot(t, plan)
	if before != after {
		t.Errorf("Apply mutated its Plan argument:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestApply_DoesNotMutate_Decisions proves Apply does not mutate the
// decisions slice (or its pointed-to payloads) it is given.
func TestApply_DoesNotMutate_Decisions(t *testing.T) {
	decisions := []Decision{
		{ItemID: "classification:row-3", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}},
	}
	before := jsonSnapshot(t, decisions)

	plan := Plan{Items: []ReviewItem{{ID: "classification:row-3", Kind: KindClassification, Status: StatusPending}}}
	source := Source{MappedLineItems: []financial.MappedLineItem{{SourceID: "row-3"}}}

	_ = Apply(source, plan, decisions)

	after := jsonSnapshot(t, decisions)
	if before != after {
		t.Errorf("Apply mutated its decisions argument:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestApply_ResultIsIndependentCopy proves that mutating ApplyResult's
// returned MappedLineItems/Adjustments after the call does not alias back
// to the original Source slices (a stronger guarantee than "Source wasn't
// mutated": the returned corrected data must be a genuinely independent
// copy, not a shared backing array).
func TestApply_ResultIsIndependentCopy(t *testing.T) {
	source := Source{
		MappedLineItems: []financial.MappedLineItem{{SourceID: "row-4", Code: financial.CodeOpexOther, Values: map[financial.Period]float64{"2025": 1}}},
		Adjustments:     []adjustments.Adjustment{{ID: "adj-4", Period: "2025", Type: adjustments.TypePersonalVehicle, Amount: 1, Included: true}},
	}
	plan := Plan{Items: []ReviewItem{
		{ID: "classification:row-4", Kind: KindClassification, Status: StatusPending},
		{ID: "adjustment:adj-4", Kind: KindAdjustment, Status: StatusPending, Adjustment: &AdjustmentPayload{AdjustmentID: "adj-4"}},
	}}
	result := Apply(source, plan, nil)

	// Mutate the RETURNED slices/maps directly.
	result.MappedLineItems[0].Code = "MUTATED"
	result.MappedLineItems[0].Values["2025"] = 999999
	result.Adjustments[0].Amount = 888888

	if source.MappedLineItems[0].Code != financial.CodeOpexOther {
		t.Error("mutating ApplyResult.MappedLineItems leaked back into source.MappedLineItems")
	}
	if source.MappedLineItems[0].Values["2025"] != 1 {
		t.Error("mutating ApplyResult.MappedLineItems[i].Values leaked back into source's Values map")
	}
	if source.Adjustments[0].Amount != 1 {
		t.Error("mutating ApplyResult.Adjustments leaked back into source.Adjustments")
	}
}

// TestBuild_DoesNotMutate_Input proves Build does not mutate any part of
// the BuildInput it is given (classification results, raw items, rows,
// periods, reconciliation result, adjustments).
func TestBuild_DoesNotMutate_Input(t *testing.T) {
	in := buildFullInputFixture()
	before := jsonSnapshot(t, in)

	_ = Build(in, DefaultPolicy())

	after := jsonSnapshot(t, in)
	if before != after {
		t.Errorf("Build mutated its BuildInput argument:\nbefore: %s\nafter:  %s", before, after)
	}
}
