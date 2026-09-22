package review

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/classification"
)

// classificationPlanFixture builds a minimal Plan with one KindClassification
// item plus the matching Source.MappedLineItems, for decision tests.
func classificationPlanFixture() (Plan, Source) {
	in := BuildInput{
		Classifications: []classification.Result{
			{RowID: "row-1", Label: "Misc", Source: classification.SourceUnknown, Confidence: 0, ReviewRequired: true},
		},
		Raws: []financial.RawLineItem{{ID: "row-1", Label: "Misc"}},
	}
	plan := Build(in, DefaultPolicy())
	source := Source{
		MappedLineItems: []financial.MappedLineItem{
			{SourceID: "row-1", Label: "Misc", Code: "", Status: financial.RowStatusNormal},
		},
	}
	return plan, source
}

// TestApply_AcceptClassification proves an accept decision leaves the
// mapped line item's already-proposed Code untouched but resolves the item.
func TestApply_AcceptClassification(t *testing.T) {
	plan, source := classificationPlanFixture()
	// Give the mapped item a proposed code as if classification had one
	// (not UNKNOWN this time) to exercise the accept path meaningfully.
	source.MappedLineItems[0].Code = financial.CodeOpexOther

	decisions := []Decision{{ItemID: "classification:row-1", Action: ActionAccept}}
	result := Apply(source, plan, decisions)

	if len(result.Applied) != 1 {
		t.Fatalf("expected 1 applied decision, got %d", len(result.Applied))
	}
	if result.MappedLineItems[0].Code != financial.CodeOpexOther {
		t.Errorf("expected code unchanged at OPEX_OTHER, got %s", result.MappedLineItems[0].Code)
	}
	assertItemStatus(t, result, "classification:row-1", StatusResolved)
}

// TestApply_OverrideClassification proves an override decision replaces the
// mapped line item's Code with the caller-supplied one.
func TestApply_OverrideClassification(t *testing.T) {
	plan, source := classificationPlanFixture()
	decisions := []Decision{{
		ItemID: "classification:row-1", Action: ActionOverride,
		Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing},
	}}
	result := Apply(source, plan, decisions)

	if result.MappedLineItems[0].Code != financial.CodeOpexMarketing {
		t.Errorf("expected overridden code OPEX_MARKETING, got %s", result.MappedLineItems[0].Code)
	}
	assertItemStatus(t, result, "classification:row-1", StatusResolved)
	if len(result.Applied) != 1 || result.Applied[0].FinalValue != string(financial.CodeOpexMarketing) {
		t.Errorf("expected AppliedDecision.FinalValue to be the overridden code, got %+v", result.Applied)
	}
}

// TestApply_IgnoreRow proves an ignore decision sets the mapped line item's
// Status to ignored.
func TestApply_IgnoreRow(t *testing.T) {
	plan, source := classificationPlanFixture()
	decisions := []Decision{{ItemID: "classification:row-1", Action: ActionIgnore}}
	result := Apply(source, plan, decisions)

	if result.MappedLineItems[0].Status != financial.RowStatusIgnored {
		t.Errorf("expected RowStatusIgnored, got %s", result.MappedLineItems[0].Status)
	}
	assertItemStatus(t, result, "classification:row-1", StatusResolved)
}

// TestApply_AcceptOCRNumeric proves accepting an OCR numeric item records
// its already-parsed value in CorrectedNumerics.
func TestApply_AcceptOCRNumeric(t *testing.T) {
	plan := Plan{
		Version: SchemaVersion,
		Items: []ReviewItem{
			{
				ID: "ocr-numeric:row-2:2025", Kind: KindOCRNumeric, Severity: SeverityWarning, Status: StatusPending,
				OCRNumeric: &OCRNumericPayload{ParsedAmount: ptrFloat(4200), Period: "2025"},
			},
		},
	}
	result := Apply(Source{}, plan, []Decision{{ItemID: "ocr-numeric:row-2:2025", Action: ActionAccept}})
	if v, ok := result.CorrectedNumerics["ocr-numeric:row-2:2025"]; !ok || v != 4200 {
		t.Errorf("expected corrected numeric 4200, got %v (ok=%v)", v, ok)
	}
	assertItemStatus(t, result, "ocr-numeric:row-2:2025", StatusResolved)
}

// TestApply_OverrideOCRNumeric proves overriding an OCR numeric item
// records the caller-supplied replacement amount.
func TestApply_OverrideOCRNumeric(t *testing.T) {
	plan := Plan{
		Items: []ReviewItem{
			{ID: "ocr-numeric:row-3:2025", Kind: KindOCRNumeric, Status: StatusPending, OCRNumeric: &OCRNumericPayload{Period: "2025"}},
		},
	}
	result := Apply(Source{}, plan, []Decision{{
		ItemID: "ocr-numeric:row-3:2025", Action: ActionOverride, OCRNumeric: &OCRNumericDecision{Amount: 9999},
	}})
	if v := result.CorrectedNumerics["ocr-numeric:row-3:2025"]; v != 9999 {
		t.Errorf("expected corrected numeric 9999, got %v", v)
	}
}

// TestApply_RejectOCRNumeric proves rejecting an OCR numeric item leaves it
// out of CorrectedNumerics and marks it StatusRejected.
func TestApply_RejectOCRNumeric(t *testing.T) {
	plan := Plan{
		Items: []ReviewItem{
			{ID: "ocr-numeric:row-4:2025", Kind: KindOCRNumeric, Status: StatusPending, OCRNumeric: &OCRNumericPayload{ParsedAmount: ptrFloat(1), Period: "2025"}},
		},
	}
	result := Apply(Source{}, plan, []Decision{{ItemID: "ocr-numeric:row-4:2025", Action: ActionReject}})
	if _, ok := result.CorrectedNumerics["ocr-numeric:row-4:2025"]; ok {
		t.Error("expected no entry in CorrectedNumerics for a rejected item")
	}
	assertItemStatus(t, result, "ocr-numeric:row-4:2025", StatusRejected)
}

// TestApply_OverrideRowKind proves overriding a KindStructure decision sets
// both Kind and the correct RowStatus on the mapped line item (per section
// 7's Kind->Status translation).
func TestApply_OverrideRowKind(t *testing.T) {
	plan := Plan{
		Items: []ReviewItem{
			{
				ID: "structure:row-5", Kind: KindStructure, SourceRowID: "row-5", Status: StatusPending,
				Structure: &StructurePayload{ProposedKind: financial.RowKindNormal},
			},
		},
	}
	source := Source{MappedLineItems: []financial.MappedLineItem{{SourceID: "row-5", Status: financial.RowStatusNormal}}}

	cases := []struct {
		kind           financial.RowKind
		expectedStatus financial.RowStatus
	}{
		{financial.RowKindHeading, financial.RowStatusIgnored},
		{financial.RowKindSubtotal, financial.RowStatusSubtotal},
		{financial.RowKindTotal, financial.RowStatusTotal},
		{financial.RowKindNormal, financial.RowStatusNormal},
	}
	for _, tc := range cases {
		result := Apply(source, plan, []Decision{{
			ItemID: "structure:row-5", Action: ActionOverride, Structure: &StructureDecision{RowKind: tc.kind},
		}})
		got := result.MappedLineItems[0]
		if got.Kind != tc.kind {
			t.Errorf("kind %s: expected Kind %s, got %s", tc.kind, tc.kind, got.Kind)
		}
		if got.Status != tc.expectedStatus {
			t.Errorf("kind %s: expected Status %s, got %s", tc.kind, tc.expectedStatus, got.Status)
		}
	}
}

// TestApply_OverridePeriod proves overriding a period decision records the
// caller-supplied period in CorrectedPeriods.
func TestApply_OverridePeriod(t *testing.T) {
	plan := Plan{
		Items: []ReviewItem{
			{ID: "period:header:0", Kind: KindPeriod, Status: StatusPending, PeriodDetail: &PeriodPayload{ProposedPeriod: "unknown_label"}},
		},
	}
	result := Apply(Source{}, plan, []Decision{{
		ItemID: "period:header:0", Action: ActionOverride, PeriodOverride: &PeriodDecision{Period: "2025"},
	}})
	if got := result.CorrectedPeriods["period:header:0"]; got != "2025" {
		t.Errorf("expected corrected period 2025, got %q", got)
	}
}

// TestApply_AdjustmentIncludeExclude proves adjustment decisions correctly
// flip Included in both directions.
func TestApply_AdjustmentIncludeExclude(t *testing.T) {
	plan := Plan{
		Items: []ReviewItem{
			{ID: "adjustment:adj-1", Kind: KindAdjustment, Status: StatusPending, Adjustment: &AdjustmentPayload{AdjustmentID: "adj-1"}},
		},
	}
	source := Source{Adjustments: []adjustments.Adjustment{
		{ID: "adj-1", Period: "2025", Type: adjustments.TypePersonalVehicle, Amount: 1000, Included: false},
	}}

	included := Apply(source, plan, []Decision{{
		ItemID: "adjustment:adj-1", Action: ActionOverride, Adjustment: &AdjustmentDecision{Included: true},
	}})
	if !included.Adjustments[0].Included {
		t.Error("expected Included == true after include decision")
	}

	excluded := Apply(source, plan, []Decision{{ItemID: "adjustment:adj-1", Action: ActionIgnore}})
	if excluded.Adjustments[0].Included {
		t.Error("expected Included == false after ignore decision")
	}
}

// TestApply_AdjustmentModifyAmount proves an override decision with
// NewAmount replaces the adjustment's Amount.
func TestApply_AdjustmentModifyAmount(t *testing.T) {
	plan := Plan{
		Items: []ReviewItem{
			{ID: "adjustment:adj-2", Kind: KindAdjustment, Status: StatusPending, Adjustment: &AdjustmentPayload{AdjustmentID: "adj-2"}},
		},
	}
	source := Source{Adjustments: []adjustments.Adjustment{
		{ID: "adj-2", Period: "2025", Type: adjustments.TypeOneTimeExpense, Amount: 500, Included: true},
	}}
	result := Apply(source, plan, []Decision{{
		ItemID: "adjustment:adj-2", Action: ActionOverride,
		Adjustment: &AdjustmentDecision{Included: true, Amount: 750, NewAmount: true},
	}})
	if result.Adjustments[0].Amount != 750 {
		t.Errorf("expected amount overridden to 750, got %v", result.Adjustments[0].Amount)
	}
}

// TestApply_InvalidDecision_UnknownItemID proves a decision against a
// nonexistent item ID is reported in Invalid, not silently ignored or
// crashing.
func TestApply_InvalidDecision_UnknownItemID(t *testing.T) {
	plan := Plan{Items: []ReviewItem{{ID: "classification:row-1", Kind: KindClassification, Status: StatusPending}}}
	result := Apply(Source{}, plan, []Decision{{ItemID: "classification:does-not-exist", Action: ActionAccept}})

	if len(result.Applied) != 0 {
		t.Errorf("expected 0 applied decisions, got %d", len(result.Applied))
	}
	if len(result.Invalid) != 1 {
		t.Fatalf("expected 1 invalid decision, got %d", len(result.Invalid))
	}
	if result.Invalid[0].Issues[0].Code != IssueUnknownItemID {
		t.Errorf("expected IssueUnknownItemID, got %s", result.Invalid[0].Issues[0].Code)
	}
}

// TestApply_InvalidDecision_InvalidCode proves an override with a
// non-taxonomy code is rejected.
func TestApply_InvalidDecision_InvalidCode(t *testing.T) {
	plan, source := classificationPlanFixture()
	result := Apply(source, plan, []Decision{{
		ItemID: "classification:row-1", Action: ActionOverride,
		Classification: &ClassificationDecision{Code: "NOT_A_REAL_CODE"},
	}})
	if len(result.Invalid) != 1 {
		t.Fatalf("expected 1 invalid decision, got %d", len(result.Invalid))
	}
	if result.Invalid[0].Issues[0].Code != IssueInvalidCode {
		t.Errorf("expected IssueInvalidCode, got %s", result.Invalid[0].Issues[0].Code)
	}
	assertItemStatus(t, result, "classification:row-1", StatusInvalidDecision)
}

// TestApply_InvalidDecision_NonFiniteAmount proves a NaN/Inf numeric
// override is rejected.
func TestApply_InvalidDecision_NonFiniteAmount(t *testing.T) {
	plan := Plan{Items: []ReviewItem{{ID: "ocr-numeric:row-6:2025", Kind: KindOCRNumeric, Status: StatusPending, OCRNumeric: &OCRNumericPayload{}}}}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		result := Apply(Source{}, plan, []Decision{{
			ItemID: "ocr-numeric:row-6:2025", Action: ActionOverride, OCRNumeric: &OCRNumericDecision{Amount: bad},
		}})
		if len(result.Invalid) != 1 || result.Invalid[0].Issues[0].Code != IssueNonFiniteAmount {
			t.Errorf("value %v: expected IssueNonFiniteAmount, got %+v", bad, result.Invalid)
		}
	}
}

// TestApply_InvalidDecision_InvalidRowKind proves a structure override with
// an unrecognized RowKind is rejected.
func TestApply_InvalidDecision_InvalidRowKind(t *testing.T) {
	plan := Plan{Items: []ReviewItem{{ID: "structure:row-7", Kind: KindStructure, Status: StatusPending, Structure: &StructurePayload{}}}}
	result := Apply(Source{}, plan, []Decision{{
		ItemID: "structure:row-7", Action: ActionOverride, Structure: &StructureDecision{RowKind: "not-a-real-kind"},
	}})
	if len(result.Invalid) != 1 || result.Invalid[0].Issues[0].Code != IssueInvalidRowKind {
		t.Errorf("expected IssueInvalidRowKind, got %+v", result.Invalid)
	}
}

// TestApply_InvalidDecision_MalformedPeriod proves an empty period override
// is rejected.
func TestApply_InvalidDecision_MalformedPeriod(t *testing.T) {
	plan := Plan{Items: []ReviewItem{{ID: "period:header:1", Kind: KindPeriod, Status: StatusPending, PeriodDetail: &PeriodPayload{}}}}
	result := Apply(Source{}, plan, []Decision{{
		ItemID: "period:header:1", Action: ActionOverride, PeriodOverride: &PeriodDecision{Period: ""},
	}})
	if len(result.Invalid) != 1 || result.Invalid[0].Issues[0].Code != IssueMalformedPeriod {
		t.Errorf("expected IssueMalformedPeriod, got %+v", result.Invalid)
	}
}

// TestApply_InvalidDecision_WrongActionForKind proves an Action not on the
// allowed list for a Kind is rejected.
func TestApply_InvalidDecision_WrongActionForKind(t *testing.T) {
	plan := Plan{Items: []ReviewItem{{ID: "adjustment:adj-3", Kind: KindAdjustment, Status: StatusPending, Adjustment: &AdjustmentPayload{}}}}
	result := Apply(Source{}, plan, []Decision{{ItemID: "adjustment:adj-3", Action: ActionReject}})
	if len(result.Invalid) != 1 || result.Invalid[0].Issues[0].Code != IssueInvalidAction {
		t.Errorf("expected IssueInvalidAction for ActionReject on KindAdjustment, got %+v", result.Invalid)
	}
}

// TestApply_DuplicateIdenticalDecision proves two byte-identical decisions
// for the same item are treated as a harmless repeat (warning, single
// application) rather than a hard error.
func TestApply_DuplicateIdenticalDecision(t *testing.T) {
	plan, source := classificationPlanFixture()
	d := Decision{ItemID: "classification:row-1", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}}
	result := Apply(source, plan, []Decision{d, d})

	if len(result.Applied) != 1 {
		t.Errorf("expected exactly 1 applied decision for an identical duplicate pair, got %d", len(result.Applied))
	}
	foundDupWarning := false
	for _, w := range result.Warnings {
		if w.Code == IssueDuplicateItemID {
			foundDupWarning = true
		}
	}
	if !foundDupWarning {
		t.Error("expected an IssueDuplicateItemID warning for the identical repeat")
	}
}

// TestApply_ConflictingDuplicateDecision proves two DIFFERENT decisions for
// the same item ID within one Apply call are rejected as conflicting, and
// neither is applied.
func TestApply_ConflictingDuplicateDecision(t *testing.T) {
	plan, source := classificationPlanFixture()
	decisions := []Decision{
		{ItemID: "classification:row-1", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}},
		{ItemID: "classification:row-1", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexOther}},
	}
	result := Apply(source, plan, decisions)

	if len(result.Applied) != 0 {
		t.Errorf("expected 0 applied decisions for a conflicting pair, got %d", len(result.Applied))
	}
	if len(result.Invalid) != 2 {
		t.Fatalf("expected both conflicting decisions reported invalid, got %d", len(result.Invalid))
	}
	for _, inv := range result.Invalid {
		if inv.Issues[0].Code != IssueConflictingDecision {
			t.Errorf("expected IssueConflictingDecision, got %s", inv.Issues[0].Code)
		}
	}
}

// TestApply_UnresolvedRequiredItems proves an unresolved Required item
// (never decided) shows up in ApplyResult.UnresolvedRequired.
func TestApply_UnresolvedRequiredItems(t *testing.T) {
	plan := Plan{Items: []ReviewItem{
		{ID: "classification:row-8", Kind: KindClassification, Required: true, Severity: SeverityBlocking, Status: StatusPending},
	}}
	result := Apply(Source{}, plan, nil)
	if len(result.UnresolvedRequired) != 1 {
		t.Fatalf("expected 1 unresolved required item, got %d", len(result.UnresolvedRequired))
	}
	if result.UnresolvedRequired[0].ID != "classification:row-8" {
		t.Errorf("expected classification:row-8, got %s", result.UnresolvedRequired[0].ID)
	}
}

// TestApply_EmptyDecisionsIsNotAnError proves an empty decisions slice is a
// valid, empty case: nothing applied, every item stays as-is.
func TestApply_EmptyDecisionsIsNotAnError(t *testing.T) {
	plan := Plan{Items: []ReviewItem{{ID: "classification:row-9", Kind: KindClassification, Status: StatusPending}}}
	result := Apply(Source{}, plan, []Decision{})
	if len(result.Applied) != 0 || len(result.Invalid) != 0 {
		t.Errorf("expected no applied/invalid decisions for an empty decisions slice, got applied=%d invalid=%d", len(result.Applied), len(result.Invalid))
	}
	if result.Items[0].Status != StatusPending {
		t.Errorf("expected item to remain StatusPending, got %s", result.Items[0].Status)
	}
}

// --- helpers -----------------------------------------------------------------

func assertItemStatus(t *testing.T, result ApplyResult, id string, want Status) {
	t.Helper()
	for _, it := range result.Items {
		if it.ID == id {
			if it.Status != want {
				t.Errorf("item %s: expected status %s, got %s", id, want, it.Status)
			}
			return
		}
	}
	t.Fatalf("item %s not found in result.Items", id)
}
