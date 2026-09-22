package review

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/classification"
	"github.com/themurtez/go-valuate/financial/reconciliation"
	"github.com/themurtez/go-valuate/ingestion"
)

// buildFullInputFixture returns a BuildInput exercising every Kind at once,
// shared by the determinism and immutability tests so they all exercise the
// exact same realistic-shaped input.
func buildFullInputFixture() BuildInput {
	return BuildInput{
		Classifications: []classification.Result{
			{RowID: "row-1", Label: "Misc", Source: classification.SourceUnknown, Confidence: 0, ReviewRequired: true},
			{RowID: "row-2", Label: "Advertising", Code: financial.CodeOpexMarketing, Confidence: 0.98, Source: classification.SourceAlias},
			{RowID: "row-3", Label: "Gross Profit", Source: classification.SourceStructural, Status: financial.RowStatusSubtotal, Kind: financial.RowKindSubtotal},
		},
		Raws: []financial.RawLineItem{
			{ID: "row-1", Label: "Misc"},
			{ID: "row-2", Label: "Advertising"},
			{ID: "row-3", Label: "Gross Profit", Kind: financial.RowKindSubtotal},
		},
		Rows: []RowContext{
			{
				RowID: "row-4", Label: "Revenue", PageIndex: 0,
				Cells: []ingestion.Cell{
					{ColumnIndex: 1, Raw: "l2,34O", Parsed: false, OCR: &ingestion.OCRProvenance{OriginalText: "l2,34O", Confidence: 80}},
				},
				ColumnPeriods: map[int]financial.Period{1: "2025"},
			},
		},
		Periods: []ingestion.DetectedPeriod{
			{ColumnIndex: 2, Period: "current_year", PeriodType: ingestion.PeriodTypeUnknown, OriginalLabel: "Current Year", Confidence: 0},
		},
		Reconciliation: &reconciliation.Result{
			Checks: []reconciliation.Check{
				{Code: reconciliation.CheckBalanceSheetBalances, Status: reconciliation.StatusFail, Period: "2025", Explanation: "does not balance"},
			},
		},
		Adjustments: []adjustments.Adjustment{
			{ID: "adj-1", Period: "2025", Type: adjustments.TypePersonalVehicle, Amount: 7200, Reason: "personal vehicle", Included: true},
		},
	}
}

// TestBuild_StableIDs proves review item IDs are stable across repeated
// Build calls against identical input, and follow the documented format
// per kind.
func TestBuild_StableIDs(t *testing.T) {
	in := buildFullInputFixture()
	plan1 := Build(in, DefaultPolicy())
	plan2 := Build(in, DefaultPolicy())

	ids1 := idSet(plan1)
	ids2 := idSet(plan2)
	if len(ids1) != len(ids2) {
		t.Fatalf("expected same item count across repeated Build calls, got %d vs %d", len(ids1), len(ids2))
	}
	for id := range ids1 {
		if !ids2[id] {
			t.Errorf("ID %q present in first Build but not second", id)
		}
	}

	wantIDs := []string{
		"classification:row-1",
		"structure:row-3",
		"ocr-numeric:row-4:2025",
		"period:header:2",
		"reconciliation:BALANCE_SHEET_BALANCES:2025",
		"adjustment:adj-1",
	}
	for _, id := range wantIDs {
		if !ids1[id] {
			t.Errorf("expected ID %q in plan, not found (got %v)", id, ids1)
		}
	}
}

// TestBuild_IDStableAcrossConfidenceChange proves a classification item's
// ID depends only on the row ID: re-running Build with the same row ID but
// a different Code/Confidence keeps the SAME ID (see buildClassificationID).
func TestBuild_IDStableAcrossConfidenceChange(t *testing.T) {
	in1 := BuildInput{
		Classifications: []classification.Result{{RowID: "row-9", Label: "X", Source: classification.SourceUnknown, Confidence: 0, ReviewRequired: true}},
		Raws:            []financial.RawLineItem{{ID: "row-9", Label: "X"}},
	}
	in2 := BuildInput{
		Classifications: []classification.Result{{RowID: "row-9", Label: "X", Code: financial.CodeOpexOther, Source: classification.SourcePhraseRule, Confidence: 0.5, ReviewRequired: true}},
		Raws:            []financial.RawLineItem{{ID: "row-9", Label: "X"}},
	}
	plan1 := Build(in1, DefaultPolicy())
	plan2 := Build(in2, DefaultPolicy())

	item1 := mustFindItem(t, plan1, KindClassification, "classification:row-9")
	item2 := mustFindItem(t, plan2, KindClassification, "classification:row-9")
	if item1.ID != item2.ID {
		t.Errorf("expected stable ID across a confidence/code change, got %q vs %q", item1.ID, item2.ID)
	}
	if item1.Classification.Confidence == item2.Classification.Confidence {
		t.Fatal("test setup error: expected differing confidence between the two runs")
	}
}

// TestBuild_StableOrdering proves Plan.Items is ordered deterministically:
// BLOCKING before ERROR before WARNING before INFO, and the same order is
// produced across repeated calls.
func TestBuild_StableOrdering(t *testing.T) {
	in := buildFullInputFixture()
	plan := Build(in, DefaultPolicy())

	if len(plan.Items) < 2 {
		t.Fatalf("expected multiple items to test ordering, got %d", len(plan.Items))
	}
	for i := 1; i < len(plan.Items); i++ {
		prevRank := severityRank(plan.Items[i-1].Severity)
		curRank := severityRank(plan.Items[i].Severity)
		if curRank < prevRank {
			t.Errorf("items not ordered by severity: index %d (%s) came after index %d (%s)",
				i, plan.Items[i].Severity, i-1, plan.Items[i-1].Severity)
		}
	}

	plan2 := Build(in, DefaultPolicy())
	for i := range plan.Items {
		if plan.Items[i].ID != plan2.Items[i].ID {
			t.Errorf("index %d: expected same item ID across repeated Build calls, got %q vs %q", i, plan.Items[i].ID, plan2.Items[i].ID)
		}
	}
}

// TestBuild_RepeatedCallsAreByteIdentical proves repeated Build calls
// against identical input produce byte-identical JSON output.
func TestBuild_RepeatedCallsAreByteIdentical(t *testing.T) {
	in := buildFullInputFixture()
	first, err := json.Marshal(Build(in, DefaultPolicy()))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Build(in, DefaultPolicy()))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Build output differs from the first run", i)
		}
	}
}

// TestApply_RepeatedCallsAreByteIdentical proves repeated Apply calls
// against identical input produce byte-identical JSON output.
func TestApply_RepeatedCallsAreByteIdentical(t *testing.T) {
	plan, source := classificationPlanFixture()
	decisions := []Decision{{ItemID: "classification:row-1", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}}}

	first, err := json.Marshal(Apply(source, plan, decisions))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Apply(source, plan, decisions))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Apply output differs from the first run", i)
		}
	}
}

// TestBuild_IDsIndependentOfInputOrder proves the same logical input,
// supplied in a different slice order, produces the same set of item IDs —
// i.e. IDs are never accidentally derived from position/iteration order.
func TestBuild_IDsIndependentOfInputOrder(t *testing.T) {
	adjsForward := []adjustmentsFixture{{id: "adj-a"}, {id: "adj-b"}, {id: "adj-c"}}
	reversed := []adjustmentsFixture{adjsForward[2], adjsForward[1], adjsForward[0]}

	plan1 := Build(BuildInput{Adjustments: toAdjustments(adjsForward)}, DefaultPolicy())
	plan2 := Build(BuildInput{Adjustments: toAdjustments(reversed)}, DefaultPolicy())

	if len(plan1.Items) != len(plan2.Items) {
		t.Fatalf("expected same item count, got %d vs %d", len(plan1.Items), len(plan2.Items))
	}
	for i := range plan1.Items {
		if plan1.Items[i].ID != plan2.Items[i].ID {
			t.Errorf("index %d: expected identical ID regardless of input order, got %q vs %q", i, plan1.Items[i].ID, plan2.Items[i].ID)
		}
	}
}

type adjustmentsFixture struct{ id string }

func toAdjustments(fs []adjustmentsFixture) []adjustments.Adjustment {
	out := make([]adjustments.Adjustment, 0, len(fs))
	for _, f := range fs {
		out = append(out, adjustments.Adjustment{ID: adjustments.ID(f.id), Period: "2025", Type: adjustments.TypeOneTimeExpense, Amount: 1, Reason: "x", Included: true})
	}
	return out
}

func idSet(plan Plan) map[string]bool {
	m := make(map[string]bool, len(plan.Items))
	for _, it := range plan.Items {
		m[it.ID] = true
	}
	return m
}
