package review

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/classification"
	"github.com/themurtez/go-valuate/financial/reconciliation"
	"github.com/themurtez/go-valuate/ingestion"
)

func ptrFloat(v float64) *float64 { return &v }

// TestBuild_HighConfidenceClassification_NoReviewItem proves a clean,
// high-confidence classification produces no KindClassification item.
func TestBuild_HighConfidenceClassification_NoReviewItem(t *testing.T) {
	in := BuildInput{
		Classifications: []classification.Result{
			{RowID: "row-1", Label: "Advertising", Code: financial.CodeOpexMarketing, Confidence: 0.98, Source: classification.SourceAlias, ReviewRequired: false},
		},
		Raws: []financial.RawLineItem{{ID: "row-1", Label: "Advertising"}},
	}
	plan := Build(in, DefaultPolicy())
	for _, it := range plan.Items {
		if it.Kind == KindClassification {
			t.Fatalf("expected no classification review item for a high-confidence result, got %+v", it)
		}
	}
}

// TestBuild_UnknownClassification_ProducesBlockingItem proves an UNKNOWN
// classification.Result always produces a BLOCKING, Required item.
func TestBuild_UnknownClassification_ProducesBlockingItem(t *testing.T) {
	in := BuildInput{
		Classifications: []classification.Result{
			{RowID: "row-2", Label: "Misc", Source: classification.SourceUnknown, Confidence: 0, ReviewRequired: true},
		},
		Raws: []financial.RawLineItem{{ID: "row-2", Label: "Misc"}},
	}
	plan := Build(in, DefaultPolicy())
	item := mustFindItem(t, plan, KindClassification, "classification:row-2")
	if item.Severity != SeverityBlocking {
		t.Errorf("expected SeverityBlocking, got %s", item.Severity)
	}
	if !item.Required {
		t.Error("expected Required == true for an UNKNOWN classification")
	}
	if item.Classification == nil {
		t.Fatal("expected a Classification payload")
	}
}

// TestBuild_LowConfidenceClassification_ProducesWarningItem proves a
// below-threshold-but-not-UNKNOWN classification produces a review item at
// WARNING (not BLOCKING).
func TestBuild_LowConfidenceClassification_ProducesWarningItem(t *testing.T) {
	in := BuildInput{
		Classifications: []classification.Result{
			{RowID: "row-3", Label: "Shop Supplies", Code: financial.CodeOpexOther, Confidence: 0.75, Source: classification.SourcePhraseRule, ReviewRequired: true},
		},
		Raws: []financial.RawLineItem{{ID: "row-3", Label: "Shop Supplies"}},
	}
	plan := Build(in, DefaultPolicy())
	item := mustFindItem(t, plan, KindClassification, "classification:row-3")
	if item.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning, got %s", item.Severity)
	}
	if item.Classification.Confidence != 0.75 {
		t.Errorf("expected confidence 0.75 preserved on payload, got %v", item.Classification.Confidence)
	}
}

// TestBuild_ClassificationWithAlternatives_MateriallyClose proves a
// materially-close alternative triggers review even above threshold.
func TestBuild_ClassificationWithAlternatives_MateriallyClose(t *testing.T) {
	in := BuildInput{
		Classifications: []classification.Result{
			{
				RowID: "row-4", Label: "Repairs", Code: financial.CodeOpexOther, Confidence: 0.92, Source: classification.SourceContextRule,
				Alternatives: []classification.Candidate{{Code: financial.CodeCogsOther, Confidence: 0.90, Reason: "close alt"}},
			},
		},
		Raws: []financial.RawLineItem{{ID: "row-4", Label: "Repairs"}},
	}
	plan := Build(in, DefaultPolicy())
	item := mustFindItem(t, plan, KindClassification, "classification:row-4")
	if len(item.Classification.Alternatives) != 1 {
		t.Fatalf("expected 1 alternative, got %d", len(item.Classification.Alternatives))
	}
	if item.Classification.Alternatives[0].Code != financial.CodeCogsOther {
		t.Errorf("expected alternative code preserved, got %s", item.Classification.Alternatives[0].Code)
	}
}

// TestBuild_LowConfidenceOCRLabel proves a low-confidence OCR label cell
// produces a KindOCRText item carrying both original and final text.
func TestBuild_LowConfidenceOCRLabel(t *testing.T) {
	in := BuildInput{
		Rows: []RowContext{
			{
				RowID: "row-5", Label: "Adverising", PageIndex: 1,
				Cells: []ingestion.Cell{
					{ColumnIndex: 0, Raw: "Adverising", OCR: &ingestion.OCRProvenance{OriginalText: "Adverising", Confidence: 55}},
				},
			},
		},
	}
	plan := Build(in, DefaultPolicy())
	item := mustFindItem(t, plan, KindOCRText, "ocr-text:row-5:0")
	if item.OCRText.OriginalText != "Adverising" || item.OCRText.FinalText != "Adverising" {
		t.Errorf("expected original/final text preserved, got %+v", item.OCRText)
	}
	if item.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning, got %s", item.Severity)
	}
}

// TestBuild_AmbiguousOCRNumeric proves a cell with OCR provenance but
// Cell.Parsed == false produces a BLOCKING, Required KindOCRNumeric item —
// the OCR-ambiguous case distinct from a clean parse failure.
func TestBuild_AmbiguousOCRNumeric(t *testing.T) {
	in := BuildInput{
		Rows: []RowContext{
			{
				RowID: "row-6", Label: "Revenue", PageIndex: 0,
				Cells: []ingestion.Cell{
					{ColumnIndex: 1, Raw: "l2,34O", Parsed: false, OCR: &ingestion.OCRProvenance{OriginalText: "l2,34O", Confidence: 80}},
				},
				ColumnPeriods: map[int]financial.Period{1: "2025"},
			},
		},
	}
	plan := Build(in, DefaultPolicy())
	item := mustFindItem(t, plan, KindOCRNumeric, "ocr-numeric:row-6:2025")
	if item.Severity != SeverityBlocking {
		t.Errorf("expected SeverityBlocking for ambiguous OCR numeric, got %s", item.Severity)
	}
	if !item.Required {
		t.Error("expected Required == true")
	}
	if !item.OCRNumeric.Ambiguous {
		t.Error("expected Ambiguous == true")
	}
	if item.OCRNumeric.ParsedAmount != nil {
		t.Error("expected nil ParsedAmount for an ambiguous cell")
	}
}

// TestBuild_CorrectedOCRNumeric proves a heuristically-corrected numeric
// cell produces a review item preserving both the original OCR text and
// the finally-parsed value.
func TestBuild_CorrectedOCRNumeric(t *testing.T) {
	in := BuildInput{
		Rows: []RowContext{
			{
				RowID: "row-7", Label: "COGS", PageIndex: 0,
				Cells: []ingestion.Cell{
					{
						ColumnIndex: 1, Raw: "12340", Parsed: true, Numeric: ptrFloat(12340),
						OCR: &ingestion.OCRProvenance{OriginalText: "l234O", Confidence: 88, NumericCorrected: true},
					},
				},
				ColumnPeriods: map[int]financial.Period{1: "2025"},
			},
		},
	}
	plan := Build(in, DefaultPolicy())
	item := mustFindItem(t, plan, KindOCRNumeric, "ocr-numeric:row-7:2025")
	if item.OCRNumeric.OriginalText != "l234O" {
		t.Errorf("expected original OCR text preserved, got %q", item.OCRNumeric.OriginalText)
	}
	if item.OCRNumeric.FinalText != "12340" {
		t.Errorf("expected final text preserved, got %q", item.OCRNumeric.FinalText)
	}
	if item.OCRNumeric.ParsedAmount == nil || *item.OCRNumeric.ParsedAmount != 12340 {
		t.Errorf("expected parsed amount 12340, got %v", item.OCRNumeric.ParsedAmount)
	}
	if !item.OCRNumeric.NumericCorrected {
		t.Error("expected NumericCorrected == true")
	}
}

// TestBuild_PeriodAmbiguity proves an unparsed period label produces a
// BLOCKING KindPeriod item.
func TestBuild_PeriodAmbiguity(t *testing.T) {
	in := BuildInput{
		Periods: []ingestion.DetectedPeriod{
			{ColumnIndex: 2, Period: "current_year", PeriodType: ingestion.PeriodTypeUnknown, OriginalLabel: "Current Year", Confidence: 0},
		},
	}
	plan := Build(in, DefaultPolicy())
	item := mustFindItem(t, plan, KindPeriod, "period:header:2")
	if item.Severity != SeverityBlocking {
		t.Errorf("expected SeverityBlocking for unparsed period, got %s", item.Severity)
	}
	if !item.Required {
		t.Error("expected Required == true")
	}
}

// TestBuild_StructuralUncertainty proves a heading/subtotal/total row
// produces an INFO-severity KindStructure confirmation item.
func TestBuild_StructuralUncertainty(t *testing.T) {
	in := BuildInput{
		Classifications: []classification.Result{
			{RowID: "row-8", Label: "Gross Profit", Source: classification.SourceStructural, Status: financial.RowStatusSubtotal, Kind: financial.RowKindSubtotal},
		},
		Raws: []financial.RawLineItem{{ID: "row-8", Label: "Gross Profit", Kind: financial.RowKindSubtotal}},
	}
	plan := Build(in, DefaultPolicy())
	item := mustFindItem(t, plan, KindStructure, "structure:row-8")
	if item.Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo, got %s", item.Severity)
	}
	if item.Structure.ProposedKind != financial.RowKindSubtotal {
		t.Errorf("expected RowKindSubtotal, got %s", item.Structure.ProposedKind)
	}
}

// TestBuild_ReconciliationFailure proves a FAIL check produces an ERROR
// item by default, and a BLOCKING item when Policy.ReconciliationFailureBlocks
// is true.
func TestBuild_ReconciliationFailure(t *testing.T) {
	recResult := &reconciliation.Result{
		Checks: []reconciliation.Check{
			{Code: reconciliation.CheckBalanceSheetBalances, Status: reconciliation.StatusFail, Period: "2025", Explanation: "assets != liabilities+equity"},
		},
	}
	in := BuildInput{Reconciliation: recResult}

	plan := Build(in, DefaultPolicy())
	item := mustFindItem(t, plan, KindReconciliation, "reconciliation:BALANCE_SHEET_BALANCES:2025")
	if item.Severity != SeverityError {
		t.Errorf("expected SeverityError by default, got %s", item.Severity)
	}

	blockingPolicy := DefaultPolicy()
	blockingPolicy.ReconciliationFailureBlocks = true
	plan2 := Build(in, blockingPolicy)
	item2 := mustFindItem(t, plan2, KindReconciliation, "reconciliation:BALANCE_SHEET_BALANCES:2025")
	if item2.Severity != SeverityBlocking {
		t.Errorf("expected SeverityBlocking under ReconciliationFailureBlocks, got %s", item2.Severity)
	}
}

// TestBuild_OptionalAssumptionConfirmation proves assumption review items
// are only created when Policy.EnableAssumptionReview is true and
// BuildInput.Assumptions is supplied.
func TestBuild_OptionalAssumptionConfirmation(t *testing.T) {
	src := fakeAssumptionSource{
		values:  map[string]any{"sde_multiple": 2.5},
		sources: map[string]string{"sde_multiple": "account"},
	}
	in := BuildInput{Assumptions: src}

	// Disabled by default.
	plan := Build(in, DefaultPolicy())
	for _, it := range plan.Items {
		if it.Kind == KindValuationAssumption {
			t.Fatalf("expected no assumption items when EnableAssumptionReview is false, got %+v", it)
		}
	}

	// Enabled.
	policy := DefaultPolicy()
	policy.EnableAssumptionReview = true
	plan2 := Build(in, policy)
	item := mustFindItem(t, plan2, KindValuationAssumption, "assumption:sde_multiple")
	if item.Assumption.CurrentValue != 2.5 {
		t.Errorf("expected current value 2.5, got %v", item.Assumption.CurrentValue)
	}
	if item.Assumption.SourceScope != "account" {
		t.Errorf("expected source scope 'account', got %q", item.Assumption.SourceScope)
	}
}

// TestBuild_AdjustmentConfirmation proves every supplied adjustment gets a
// confirmation item, never an invented one.
func TestBuild_AdjustmentConfirmation(t *testing.T) {
	in := BuildInput{
		Adjustments: []adjustments.Adjustment{
			{ID: "adj-1", Period: "2025", Type: adjustments.TypePersonalVehicle, Amount: 7200, Reason: "personal truck", Included: true},
		},
	}
	plan := Build(in, DefaultPolicy())
	item := mustFindItem(t, plan, KindAdjustment, "adjustment:adj-1")
	if item.Adjustment.Amount != 7200 {
		t.Errorf("expected amount 7200, got %v", item.Adjustment.Amount)
	}
	if item.Severity != SeverityInfo {
		t.Errorf("expected SeverityInfo for an adjustment confirmation, got %s", item.Severity)
	}
}

// TestBuild_MaterialityDefaultOff proves a zero-value Policy materiality
// threshold means "everything is material," never "nothing is material."
func TestBuild_MaterialityDefaultOff(t *testing.T) {
	policy := DefaultPolicy()
	if !IsMaterial(0.01, nil, policy) {
		t.Error("expected IsMaterial(0.01, nil, DefaultPolicy()) == true: a zero threshold must mean 'everything is material'")
	}
	if !IsMaterial(-500000, nil, policy) {
		t.Error("expected a large negative amount to be material with default policy")
	}
}

// TestBuild_MaterialityThreshold proves a nonzero MaterialAmountThreshold
// and MaterialPercentOfRevenue behave as documented.
func TestBuild_MaterialityThreshold(t *testing.T) {
	policy := DefaultPolicy()
	policy.MaterialAmountThreshold = 1000
	if IsMaterial(500, nil, policy) {
		t.Error("expected 500 < threshold 1000 to be immaterial")
	}
	if !IsMaterial(1500, nil, policy) {
		t.Error("expected 1500 >= threshold 1000 to be material")
	}

	policy2 := DefaultPolicy()
	policy2.MaterialAmountThreshold = 1_000_000 // deliberately high so the percent leg decides
	policy2.MaterialPercentOfRevenue = 0.01
	revenue := 100_000.0
	if IsMaterial(500, &revenue, policy2) {
		t.Error("expected 500 < 1% of 100,000 to be immaterial")
	}
	if !IsMaterial(2000, &revenue, policy2) {
		t.Error("expected 2000 >= 1% of 100,000 to be material")
	}
}

// --- test helpers -----------------------------------------------------------

func mustFindItem(t *testing.T, plan Plan, kind Kind, id string) ReviewItem {
	t.Helper()
	for _, it := range plan.Items {
		if it.Kind == kind && it.ID == id {
			return it
		}
	}
	t.Fatalf("expected to find item kind=%s id=%s in plan; got %d items", kind, id, len(plan.Items))
	return ReviewItem{}
}

type fakeAssumptionSource struct {
	values  map[string]any
	sources map[string]string
}

func (f fakeAssumptionSource) AssumptionValues() map[string]any { return f.values }
func (f fakeAssumptionSource) AssumptionSourceFor(key string) string {
	return f.sources[key]
}
