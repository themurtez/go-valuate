package review

import (
	"math"
	"math/rand"
	"reflect"
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

// structuralClassificationPlanFixture builds a Plan+Source with a single
// KindClassification item whose underlying row is structural (kind), the
// shape that arises when Policy.ReviewAllClassifications creates a
// classification item even for a heading/subtotal/total row (see
// buildClassificationItems, which does not skip structural rows). Mirrors
// classificationPlanFixture but for the structural-row regression suite
// below (see the "Known gap" note this fixes).
func structuralClassificationPlanFixture(kind financial.RowKind) (Plan, Source) {
	status := rowKindToStatus(kind)
	plan := Plan{Items: []ReviewItem{{
		ID: "classification:row-s1", Kind: KindClassification, SourceRowID: "row-s1", Status: StatusPending,
		ProposedValue: "",
		Classification: &ClassificationPayload{
			OriginalLabel: "Gross Profit", ProposedCode: "", Kind: kind,
		},
	}}}
	source := Source{MappedLineItems: []financial.MappedLineItem{
		{SourceID: "row-s1", Label: "Gross Profit", Code: "", Status: status, Kind: kind,
			Values: map[financial.Period]float64{"2025": 100000}},
	}}
	return plan, source
}

// TestApply_ClassificationOverride_StructuralRowRejected proves an
// ACTION_OVERRIDE classification decision against a row whose effective
// financial.RowKind is structural (HEADING, SUBTOTAL, or TOTAL) is rejected
// rather than silently turning the row back into a RowStatusNormal row —
// see IssueStructuralRowOverride. Covers all three structural kinds plus a
// same-kind confirmation via ActionAccept, which must still succeed since
// it never proposes a code or changes structural semantics.
func TestApply_ClassificationOverride_StructuralRowRejected(t *testing.T) {
	for _, kind := range []financial.RowKind{financial.RowKindHeading, financial.RowKindSubtotal, financial.RowKindTotal} {
		t.Run(string(kind), func(t *testing.T) {
			plan, source := structuralClassificationPlanFixture(kind)
			originalStatus := source.MappedLineItems[0].Status
			originalKind := source.MappedLineItems[0].Kind

			result := Apply(source, plan, []Decision{{
				ItemID: "classification:row-s1", Action: ActionOverride,
				Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing},
			}})

			if len(result.Invalid) != 1 {
				t.Fatalf("expected the override to be rejected, got Invalid=%+v Applied=%+v", result.Invalid, result.Applied)
			}
			if result.Invalid[0].Issues[0].Code != IssueStructuralRowOverride {
				t.Errorf("expected IssueStructuralRowOverride, got %s", result.Invalid[0].Issues[0].Code)
			}
			assertItemStatus(t, result, "classification:row-s1", StatusInvalidDecision)

			got := result.MappedLineItems[0]
			if got.Status != originalStatus {
				t.Errorf("kind %s: row must not become RowStatusNormal; expected Status %s unchanged, got %s", kind, originalStatus, got.Status)
			}
			if got.Kind != originalKind {
				t.Errorf("kind %s: expected Kind unchanged at %s, got %s", kind, originalKind, got.Kind)
			}
			if got.Code != "" {
				t.Errorf("kind %s: expected Code to remain empty, got %q", kind, got.Code)
			}
		})
	}
}

// TestApply_ClassificationAccept_StructuralRowAllowed proves ActionAccept
// (confirming the current classification state) remains valid on a
// structural row, since it never assigns a code or changes structural
// semantics — only ActionOverride's code-assignment is unsafe.
func TestApply_ClassificationAccept_StructuralRowAllowed(t *testing.T) {
	plan, source := structuralClassificationPlanFixture(financial.RowKindSubtotal)
	result := Apply(source, plan, []Decision{{ItemID: "classification:row-s1", Action: ActionAccept}})

	if len(result.Invalid) != 0 {
		t.Fatalf("expected ActionAccept on a structural row to succeed, got Invalid=%+v", result.Invalid)
	}
	assertItemStatus(t, result, "classification:row-s1", StatusResolved)
	got := result.MappedLineItems[0]
	if got.Status != financial.RowStatusSubtotal {
		t.Errorf("expected Status to remain RowStatusSubtotal, got %s", got.Status)
	}
	if got.Code != "" {
		t.Errorf("expected Code to remain empty, got %q", got.Code)
	}
}

// structureThenClassificationFixture builds a Plan+Source for a single row
// that starts structural (kind), with both a KindStructure item (proposing
// a change to RowKindNormal) and a KindClassification item targeting it —
// the shape TestApply_StructureOverrideToNormal_ThenClassificationOverride
// and its order-independence siblings below all share, so every variant
// exercises byte-identical starting state and only the decision slice order
// (or row suffix, for the multi-row test) differs.
func structureThenClassificationFixture(rowSuffix string, kind financial.RowKind) (Plan, Source) {
	rowID := "row-" + rowSuffix
	plan := Plan{Items: []ReviewItem{
		{
			ID: "structure:" + rowID, Kind: KindStructure, SourceRowID: rowID, Status: StatusPending,
			Structure: &StructurePayload{ProposedKind: kind},
		},
		{
			ID: "classification:" + rowID, Kind: KindClassification, SourceRowID: rowID, Status: StatusPending,
			Classification: &ClassificationPayload{OriginalLabel: "Gross Profit", Kind: kind},
		},
	}}
	source := Source{MappedLineItems: []financial.MappedLineItem{
		{SourceID: rowID, Label: "Gross Profit", Status: rowKindToStatus(kind), Kind: kind},
	}}
	return plan, source
}

// assertStructureThenClassificationApplied asserts the one shared outcome
// every ordering variant of the structure+classification decision pair must
// produce for rowID: both the structure and classification items for that
// row resolved cleanly, the row's Kind/Status corrected to NORMAL, and the
// classification code assigned. Does not assert result.Invalid/Applied
// counts overall — callers with more than one row's decisions in the same
// Apply call check those themselves.
func assertStructureThenClassificationApplied(t *testing.T, result ApplyResult, rowID, structureItemID, classificationItemID string, wantCode financial.Code) {
	t.Helper()
	if len(result.Invalid) != 0 {
		t.Fatalf("expected every decision to apply cleanly, got Invalid=%+v", result.Invalid)
	}
	idx, ok := indexMappedLineItemsBySourceID(result.MappedLineItems)[rowID]
	if !ok {
		t.Fatalf("expected a MappedLineItem for %q in the result", rowID)
	}
	got := result.MappedLineItems[idx]
	if got.Kind != financial.RowKindNormal {
		t.Errorf("row %s: expected Kind RowKindNormal after structure override, got %s", rowID, got.Kind)
	}
	if got.Status != financial.RowStatusNormal {
		t.Errorf("row %s: expected Status RowStatusNormal, got %s", rowID, got.Status)
	}
	if got.Code != wantCode {
		t.Errorf("row %s: expected Code %s after the classification override, got %s", rowID, wantCode, got.Code)
	}
	assertItemStatus(t, result, structureItemID, StatusResolved)
	assertItemStatus(t, result, classificationItemID, StatusResolved)
}

// TestApply_StructureOverrideToNormal_ThenClassificationOverride proves the
// intended pathway: an explicit KindStructure decision that changes a row
// to RowKindNormal, followed by a classification override in the SAME
// Apply call, succeeds — once structure has explicitly cleared the row's
// structural status, assigning it a financial code is legitimate again.
// This is the FORWARD order (structure decision first in the slice).
func TestApply_StructureOverrideToNormal_ThenClassificationOverride(t *testing.T) {
	plan, source := structureThenClassificationFixture("s2", financial.RowKindSubtotal)
	decisions := []Decision{
		{ItemID: "structure:row-s2", Action: ActionOverride, Structure: &StructureDecision{RowKind: financial.RowKindNormal}},
		{ItemID: "classification:row-s2", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}},
	}
	result := Apply(source, plan, decisions)
	if len(result.Applied) != 2 {
		t.Fatalf("expected 2 applied decisions, got %d: %+v", len(result.Applied), result.Applied)
	}
	assertStructureThenClassificationApplied(t, result, "row-s2", "structure:row-s2", "classification:row-s2", financial.CodeOpexMarketing)
}

// TestApply_ReverseOrder_ClassificationThenStructure_SameResult is prompt
// 11B's core regression: the SAME logical decision set as
// TestApply_StructureOverrideToNormal_ThenClassificationOverride, but with
// the classification override listed BEFORE the structure override in the
// input slice. Apply must resolve KindStructure decisions before
// KindClassification decisions regardless of caller slice order (see
// decisionPhase/decisionApplicationOrder), so this must produce the exact
// same final mapped row as the forward-order test above — proving Apply's
// semantic result is independent of caller array order, not merely that
// this particular reversed case happens to also succeed.
func TestApply_ReverseOrder_ClassificationThenStructure_SameResult(t *testing.T) {
	plan, source := structureThenClassificationFixture("s2", financial.RowKindSubtotal)
	decisions := []Decision{
		{ItemID: "classification:row-s2", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}},
		{ItemID: "structure:row-s2", Action: ActionOverride, Structure: &StructureDecision{RowKind: financial.RowKindNormal}},
	}
	result := Apply(source, plan, decisions)
	if len(result.Applied) != 2 {
		t.Fatalf("expected 2 applied decisions, got %d: %+v", len(result.Applied), result.Applied)
	}
	assertStructureThenClassificationApplied(t, result, "row-s2", "structure:row-s2", "classification:row-s2", financial.CodeOpexMarketing)

	// Reverse order must produce the exact same final row as forward order.
	forward := Apply(source, plan, []Decision{
		{ItemID: "structure:row-s2", Action: ActionOverride, Structure: &StructureDecision{RowKind: financial.RowKindNormal}},
		{ItemID: "classification:row-s2", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}},
	})
	if !reflect.DeepEqual(result.MappedLineItems, forward.MappedLineItems) {
		t.Fatalf("reverse-order result diverged from forward-order result: reverse=%+v forward=%+v", result.MappedLineItems, forward.MappedLineItems)
	}
}

// TestApply_StructuralRow_NoStructureDecision_ClassificationStillRejected
// proves that WITHOUT an accompanying structure decision in the same Apply
// call, a classification override against a structural row is still
// rejected — order-independence must never be mistaken for "classification
// can force structural rows to NORMAL on its own." This is the same
// guarantee 11A's TestApply_ClassificationOverride_StructuralRowRejected
// covers; kept here too as an explicit sibling of the ordering tests above,
// using the identical fixture, so a future reader sees all three outcomes
// (forward, reverse, structure-omitted) side by side.
func TestApply_StructuralRow_NoStructureDecision_ClassificationStillRejected(t *testing.T) {
	plan, source := structureThenClassificationFixture("s2", financial.RowKindSubtotal)
	decisions := []Decision{
		{ItemID: "classification:row-s2", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}},
	}
	result := Apply(source, plan, decisions)

	if len(result.Applied) != 0 {
		t.Fatalf("expected the override to be rejected, got Applied=%+v", result.Applied)
	}
	if len(result.Invalid) != 1 || result.Invalid[0].Issues[0].Code != IssueStructuralRowOverride {
		t.Fatalf("expected IssueStructuralRowOverride, got Invalid=%+v", result.Invalid)
	}
	got := result.MappedLineItems[0]
	if got.Kind != financial.RowKindSubtotal || got.Status != financial.RowStatusSubtotal {
		t.Errorf("expected the row to remain structural, got Kind=%s Status=%s", got.Kind, got.Status)
	}
}

// TestApply_ConflictingStructureDecisions proves two DIFFERENT structure
// decisions for the same item ID within one Apply call are still rejected
// as conflicting (IssueConflictingDecision) — phase-based reordering must
// never be mistaken for a way to "resolve" a genuine conflict by picking
// whichever one happens to sort first.
func TestApply_ConflictingStructureDecisions(t *testing.T) {
	plan := Plan{Items: []ReviewItem{{
		ID: "structure:row-s3", Kind: KindStructure, SourceRowID: "row-s3", Status: StatusPending,
		Structure: &StructurePayload{ProposedKind: financial.RowKindSubtotal},
	}}}
	source := Source{MappedLineItems: []financial.MappedLineItem{
		{SourceID: "row-s3", Status: financial.RowStatusSubtotal, Kind: financial.RowKindSubtotal},
	}}
	decisions := []Decision{
		{ItemID: "structure:row-s3", Action: ActionOverride, Structure: &StructureDecision{RowKind: financial.RowKindNormal}},
		{ItemID: "structure:row-s3", Action: ActionOverride, Structure: &StructureDecision{RowKind: financial.RowKindTotal}},
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
	// The row must remain exactly as it started: neither conflicting
	// decision may have partially applied.
	got := result.MappedLineItems[0]
	if got.Kind != financial.RowKindSubtotal || got.Status != financial.RowStatusSubtotal {
		t.Errorf("expected the row untouched by the conflicting pair, got Kind=%s Status=%s", got.Kind, got.Status)
	}
}

// TestApply_MultipleRows_OrderIndependent proves that when TWO unrelated
// rows each have their own structure+classification decision pair, the
// semantic result for each row is identical regardless of how the four
// decisions are interleaved/ordered in the input slice — ordering changes
// affecting one row's decisions must never influence another row's outcome,
// and the phase reordering must apply uniformly across every row present in
// one Apply call, not just a single hard-coded row.
func TestApply_MultipleRows_OrderIndependent(t *testing.T) {
	planA, sourceA := structureThenClassificationFixture("m1", financial.RowKindSubtotal)
	planB, sourceB := structureThenClassificationFixture("m2", financial.RowKindTotal)
	plan := Plan{Items: append(append([]ReviewItem{}, planA.Items...), planB.Items...)}
	source := Source{MappedLineItems: append(append([]financial.MappedLineItem{}, sourceA.MappedLineItems...), sourceB.MappedLineItems...)}

	// Deliberately interleaved: row-m2's classification decision comes
	// before row-m1's structure decision, and row-m1's classification
	// decision comes before row-m2's structure decision.
	decisions := []Decision{
		{ItemID: "classification:row-m2", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexOther}},
		{ItemID: "classification:row-m1", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}},
		{ItemID: "structure:row-m1", Action: ActionOverride, Structure: &StructureDecision{RowKind: financial.RowKindNormal}},
		{ItemID: "structure:row-m2", Action: ActionOverride, Structure: &StructureDecision{RowKind: financial.RowKindNormal}},
	}
	result := Apply(source, plan, decisions)

	if len(result.Applied) != 4 {
		t.Fatalf("expected 4 applied decisions (2 rows x structure+classification), got %d: %+v", len(result.Applied), result.Applied)
	}
	assertStructureThenClassificationApplied(t, result, "row-m1", "structure:row-m1", "classification:row-m1", financial.CodeOpexMarketing)
	assertStructureThenClassificationApplied(t, result, "row-m2", "structure:row-m2", "classification:row-m2", financial.CodeOpexOther)
}

// TestApply_PermutedOrder_DeterministicResult reorders a small valid
// decision set (two rows, each with a structure+classification pair)
// through several random permutations and asserts every permutation
// produces the exact same final MappedLineItems — the strongest form of
// prompt 11B's determinism requirement: "same Source+Plan+decision SET
// yields the same semantic output regardless of input slice order,"
// checked across arbitrary shuffles rather than just the two hand-picked
// forward/reverse cases above. Uses a seeded math/rand generator (never Go
// map iteration order) so this test is itself reproducible.
func TestApply_PermutedOrder_DeterministicResult(t *testing.T) {
	planA, sourceA := structureThenClassificationFixture("p1", financial.RowKindSubtotal)
	planB, sourceB := structureThenClassificationFixture("p2", financial.RowKindTotal)
	plan := Plan{Items: append(append([]ReviewItem{}, planA.Items...), planB.Items...)}
	source := Source{MappedLineItems: append(append([]financial.MappedLineItem{}, sourceA.MappedLineItems...), sourceB.MappedLineItems...)}

	baseDecisions := []Decision{
		{ItemID: "structure:row-p1", Action: ActionOverride, Structure: &StructureDecision{RowKind: financial.RowKindNormal}},
		{ItemID: "classification:row-p1", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexMarketing}},
		{ItemID: "structure:row-p2", Action: ActionOverride, Structure: &StructureDecision{RowKind: financial.RowKindNormal}},
		{ItemID: "classification:row-p2", Action: ActionOverride, Classification: &ClassificationDecision{Code: financial.CodeOpexOther}},
	}

	reference := Apply(source, plan, baseDecisions)
	assertStructureThenClassificationApplied(t, reference, "row-p1", "structure:row-p1", "classification:row-p1", financial.CodeOpexMarketing)
	assertStructureThenClassificationApplied(t, reference, "row-p2", "structure:row-p2", "classification:row-p2", financial.CodeOpexOther)

	rng := rand.New(rand.NewSource(42))
	for perm := 0; perm < 12; perm++ {
		shuffled := append([]Decision(nil), baseDecisions...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		result := Apply(source, plan, shuffled)
		if len(result.Invalid) != 0 {
			t.Fatalf("permutation %d (%+v): expected all decisions to apply cleanly, got Invalid=%+v", perm, shuffled, result.Invalid)
		}
		for i := range result.MappedLineItems {
			if !reflect.DeepEqual(result.MappedLineItems[i], reference.MappedLineItems[i]) {
				t.Fatalf("permutation %d: MappedLineItems[%d] diverged from the reference order's result: got %+v, want %+v",
					perm, i, result.MappedLineItems[i], reference.MappedLineItems[i])
			}
		}
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
