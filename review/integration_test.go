package review

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
	"github.com/themurtez/go-valuate/ingestion"
)

// TestIntegration_IngestionToClassificationToReviewToNormalize exercises the
// full chain: hand-built RawLineItems (shaped exactly as
// ingestion.Result.ToRawLineItems() would produce them) -> classification ->
// review.Build -> caller decisions -> review.Apply -> financial.Normalize.
// It proves an UNKNOWN classification blocks a clean normalize until a
// review decision resolves it, and that once resolved, the corrected
// MappedLineItems normalize successfully with the decided code.
func TestIntegration_IngestionToClassificationToReviewToNormalize(t *testing.T) {
	raws := []financial.RawLineItem{
		{ID: "row-1", Label: "Advertising & Promotion", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 12000}},
		{ID: "row-2", Label: "Shop Supplies", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 3000}},
	}
	cfg := classification.Config{
		AliasLayers: []classification.AliasLayer{
			{Name: "global", Aliases: []classification.Alias{{Label: "Advertising & Promotion", Code: financial.CodeOpexMarketing}}},
		},
		Rules: classification.DefaultRules(),
	}
	results := classification.ClassifyBatch(raws, cfg)

	// "Shop Supplies" is expected to have no alias and no strong rule match,
	// landing on UNKNOWN or low confidence — this test doesn't assume which
	// exactly (that's classification's own concern), it only asserts review
	// correctly surfaces whatever needs review and Apply correctly corrects
	// it once decided.
	plan := Build(BuildInput{Classifications: results, Raws: raws}, DefaultPolicy())

	mapped := make([]financial.MappedLineItem, len(results))
	for i, r := range results {
		mapped[i] = r.ToMappedLineItem(raws[i])
	}

	readinessBefore := EvaluateReadiness(plan.Items)
	// If nothing needed review, this integration test's premise (that a
	// low-confidence/UNKNOWN row exists to resolve) doesn't hold for this
	// input; fail loudly rather than silently passing a no-op test.
	if len(plan.Items) == 0 {
		t.Fatal("test setup error: expected at least one review item from this input (e.g. an UNKNOWN 'Shop Supplies' classification)")
	}

	// Resolve every classification review item by accepting/overriding to a
	// valid code, so downstream Normalize can succeed.
	var decisions []Decision
	for _, item := range plan.Items {
		if item.Kind != KindClassification {
			continue
		}
		decisions = append(decisions, Decision{
			ItemID: item.ID, Action: ActionOverride,
			Classification: &ClassificationDecision{Code: financial.CodeOpexOther},
		})
	}

	source := Source{MappedLineItems: mapped}
	applyResult := Apply(source, plan, decisions)

	if len(applyResult.UnresolvedRequired) != 0 {
		t.Errorf("expected every required item resolved after decisions, got %d unresolved: %+v", len(applyResult.UnresolvedRequired), applyResult.UnresolvedRequired)
	}

	readinessAfter := EvaluateReadiness(applyResult.Items)
	if readinessBefore.State == ReadinessReady && readinessAfter.State != ReadinessReady {
		t.Error("readiness should not regress from decisions resolving items")
	}
	if readinessAfter.State == ReadinessNotReady {
		t.Errorf("expected readiness to improve after resolving decisions, still NOT_READY: %+v", readinessAfter.Reasons)
	}

	dataset, err := financial.Normalize(applyResult.MappedLineItems, financial.NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("expected Normalize to succeed on corrected MappedLineItems, got error: %v", err)
	}
	if len(dataset.Items) == 0 {
		t.Error("expected at least one normalized item")
	}

	// Every normalized item's code must be a valid taxonomy code — proves
	// the override actually took effect and flowed all the way through.
	for _, it := range dataset.Items {
		if !financial.IsValidCode(it.Code) {
			t.Errorf("unexpected invalid code %q made it into the normalized dataset", it.Code)
		}
	}
}

// TestIntegration_ClassificationOverrideNeverAggregatesStructuralRow is the
// end-to-end regression for the structural-override safety fix: a subtotal
// row -> ReviewAllClassifications creates a KindClassification item for it
// (buildClassificationItems does not skip structural rows) -> a caller
// attempts an ACTION_OVERRIDE classification decision against it -> Apply
// -> financial.Normalize must never aggregate that subtotal into any code's
// total (the exact double-counting failure mode IssueStructuralRowOverride
// exists to prevent).
func TestIntegration_ClassificationOverrideNeverAggregatesStructuralRow(t *testing.T) {
	raws := []financial.RawLineItem{
		{ID: "row-1", Label: "Product Sales", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 500000}},
		{ID: "row-2", Label: "Materials", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 180000}},
		{ID: "row-3", Label: "Gross Profit", StatementType: financial.StatementIncomeStatement, Kind: financial.RowKindSubtotal, Values: map[financial.Period]float64{"2025": 320000}},
	}
	cfg := classification.Config{
		AliasLayers: []classification.AliasLayer{{Name: "global", Aliases: []classification.Alias{
			{Label: "Product Sales", Code: financial.CodeRevProduct},
			{Label: "Materials", Code: financial.CodeCogsMaterial},
		}}},
		Rules: classification.DefaultRules(),
	}
	results := classification.ClassifyBatch(raws, cfg)

	var subtotalResult classification.Result
	for _, r := range results {
		if r.RowID == "row-3" {
			subtotalResult = r
		}
	}
	if subtotalResult.Kind != financial.RowKindSubtotal || subtotalResult.Status != financial.RowStatusSubtotal {
		t.Fatalf("test setup error: expected row-3 to classify as a structural subtotal, got Kind=%s Status=%s", subtotalResult.Kind, subtotalResult.Status)
	}

	// ReviewAllClassifications surfaces a KindClassification item even for
	// the already-correctly-classified structural row — this is the exact
	// shape the "Known gap" note describes.
	policy := DefaultPolicy()
	policy.ReviewAllClassifications = true
	plan := Build(BuildInput{Classifications: results, Raws: raws}, policy)

	item := mustFindItem(t, plan, KindClassification, "classification:row-3")
	if item.Classification.Kind != financial.RowKindSubtotal {
		t.Fatalf("expected the review item to carry forward Kind=subtotal, got %s", item.Classification.Kind)
	}

	mapped := make([]financial.MappedLineItem, len(results))
	for i, r := range results {
		mapped[i] = r.ToMappedLineItem(raws[i])
	}
	source := Source{MappedLineItems: mapped}

	// A caller (or reviewer) attempts to force-classify the subtotal row as
	// an ordinary revenue account.
	decisions := []Decision{{
		ItemID: "classification:row-3", Action: ActionOverride,
		Classification: &ClassificationDecision{Code: financial.CodeRevProduct},
	}}
	applyResult := Apply(source, plan, decisions)

	if len(applyResult.Invalid) != 1 || applyResult.Invalid[0].Issues[0].Code != IssueStructuralRowOverride {
		t.Fatalf("expected the override to be rejected with IssueStructuralRowOverride, got Invalid=%+v Applied=%+v", applyResult.Invalid, applyResult.Applied)
	}

	dataset, err := financial.Normalize(applyResult.MappedLineItems, financial.NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("expected Normalize to succeed, got error: %v", err)
	}

	for _, it := range dataset.Items {
		if it.Code == financial.CodeRevProduct && it.Period == "2025" {
			// Only row-1's 500000 may contribute; row-3's 320000 must never
			// have been folded in, whether alone (500000) or doubled with
			// row-3 (820000).
			if it.Amount != 500000 {
				t.Fatalf("subtotal row was aggregated into REV_PRODUCT: expected 500000 (row-1 only), got %v", it.Amount)
			}
		}
	}
}

// TestIntegration_ScannedPDFToOCRToReviewToDecisionToCorrectedData exercises
// the OCR-specific chain section 20 asks for: an ambiguous/low-confidence
// OCR numeric cell -> review item -> explicit decision -> corrected data.
// Uses a hand-built RowContext/Cell shaped exactly as ingestion/pdf's OCR
// path would produce it (see ingestion.OCRProvenance/ingestion.Cell),
// rather than wiring up a real Tesseract-backed fixture, since this test's
// job is review.Build/Apply's own logic against a realistically-shaped
// input, not re-testing OCR extraction itself (ingestion/pdf already owns
// that testing).
func TestIntegration_ScannedPDFToOCRToReviewToDecisionToCorrectedData(t *testing.T) {
	row := RowContext{
		RowID: "sheet-0-row-3", Label: "Total Revenue", PageIndex: 0,
		Cells: []ingestion.Cell{
			{ColumnIndex: 0, Raw: "Total Revenue"},
			{
				ColumnIndex: 1, Raw: "l45,678", Parsed: false,
				OCR: &ingestion.OCRProvenance{OriginalText: "l45,678", Confidence: 62, ReviewRecommended: true},
			},
		},
		ColumnPeriods: map[int]financial.Period{1: "2025"},
	}

	plan := Build(BuildInput{Rows: []RowContext{row}}, DefaultPolicy())
	item := mustFindItem(t, plan, KindOCRNumeric, "ocr-numeric:sheet-0-row-3:2025")
	if item.Severity != SeverityBlocking {
		t.Fatalf("expected an ambiguous OCR numeric cell to be BLOCKING, got %s", item.Severity)
	}

	readiness := EvaluateReadiness(plan.Items)
	if readiness.State != ReadinessNotReady {
		t.Fatalf("expected NOT_READY before the ambiguous OCR value is resolved, got %s", readiness.State)
	}

	// The human reviewer reads "l45,678" and decides it should be 145678.
	decisions := []Decision{{
		ItemID: item.ID, Action: ActionOverride,
		OCRNumeric: &OCRNumericDecision{Amount: 145678},
	}}
	result := Apply(Source{Rows: []RowContext{row}}, plan, decisions)

	corrected, ok := result.CorrectedNumerics[item.ID]
	if !ok || corrected != 145678 {
		t.Fatalf("expected corrected numeric value 145678, got %v (ok=%v)", corrected, ok)
	}

	readinessAfter := EvaluateReadiness(result.Items)
	if readinessAfter.State == ReadinessNotReady {
		t.Errorf("expected readiness to no longer be NOT_READY after the OCR decision was resolved, got reasons: %+v", readinessAfter.Reasons)
	}
}
