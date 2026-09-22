package review

import (
	"fmt"
	"math"
	"sort"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/classification"
	"github.com/themurtez/go-valuate/financial/reconciliation"
	"github.com/themurtez/go-valuate/ingestion"
)

// Policy configures the materiality/threshold decisions Build makes when
// deciding whether a given upstream fact deserves a ReviewItem, and at what
// Severity. Every field has a conservative, documented default — see
// DefaultPolicy — matching the DefaultLimits/DefaultRules/DefaultTolerance
// pattern used throughout this repository.
type Policy struct {
	// ClassificationConfidenceThreshold is the classification.Confidence
	// (a [0, 1] heuristic, NOT a statistical probability — see
	// classification.Confidence's own doc comment) below which a
	// classification.Result gets a review item. Defaults to
	// classification.DefaultReviewThreshold (0.90) — the same threshold
	// classification itself uses for Result.ReviewRequired, reused here
	// rather than re-declared, so the two packages' notion of "needs
	// review" never silently drifts apart.
	ClassificationConfidenceThreshold float64
	// OCRConfidenceThreshold is the OCR label-text confidence below which a
	// KindOCRText review item is created.
	//
	// SCALE WARNING: this is on Tesseract's native 0-100 confidence scale
	// (ingestion.OCRProvenance.Confidence), NOT the [0, 1] scale
	// classification.Confidence uses. Passing a value like 0.9 here (a
	// reasonable-looking number if you're used to classification's scale)
	// would treat every OCR cell as low-confidence, since Tesseract virtually
	// always reports well above 0.9 on its own 0-100 scale. This mismatch is
	// a real footgun — double-check which scale you're setting before
	// changing this field. Defaults to 70.0, matching
	// ingestion/pdf/ocr_provenance.go's own lowConfidenceThreshold.
	OCRConfidenceThreshold float64
	// NumericOCRConfidenceThreshold is the OCR numeric-value confidence
	// below which a KindOCRNumeric review item is created. Same 0-100
	// Tesseract scale and same footgun as OCRConfidenceThreshold above.
	// Defaults to 70.0.
	NumericOCRConfidenceThreshold float64
	// RequireReviewForAllOCR, when true, creates a review item for EVERY
	// OCR-derived cell (label or numeric), regardless of confidence —
	// "strict mode" for a caller that wants a human to look at every
	// OCR-sourced value at least once. Defaults to false.
	RequireReviewForAllOCR bool
	// ReconciliationFailureBlocks, when true, assigns SeverityBlocking to a
	// reconciliation.Check with Status == StatusFail (rather than
	// SeverityError). Defaults to false: this repository's general
	// "opt-in strictness" convention (see orchestrator.FilterPolicy's
	// default PolicyIncludeAllEnabled) means a reconciliation failure is
	// surfaced but does not by itself prevent valuation unless a caller
	// explicitly opts into that behavior.
	ReconciliationFailureBlocks bool
	// MaterialAmountThreshold is the absolute-dollar floor below which
	// IsMaterial always returns true regardless of MaterialPercentOfRevenue
	// (see IsMaterial). 0 (the default) means "not applied" — i.e.
	// materiality gating is OFF by default, so every candidate item is
	// treated as material and gets a review item. A nonzero value is an
	// opt-in floor a caller sets to suppress review noise on very small
	// dollar amounts.
	MaterialAmountThreshold float64
	// MaterialPercentOfRevenue is the fraction of revenue (e.g. 0.01 = 1%)
	// below which an amount is considered immaterial, when revenue is
	// available and this field is > 0. 0 (the default) means "not
	// applied."
	MaterialPercentOfRevenue float64
	// ReviewAllClassifications, when true, creates a review item for every
	// classification.Result regardless of confidence/UNKNOWN/alternatives
	// (the caller-opt-in case from the classification review rules).
	// Defaults to false.
	ReviewAllClassifications bool
	// EnableAssumptionReview, when true, causes Build to create
	// KindValuationAssumption items from a supplied settings.Resolution
	// (see BuildInput.Assumptions). Defaults to false: not every caller
	// needs assumption review, and a caller not supplying a Resolution at
	// all gets no assumption items either way.
	EnableAssumptionReview bool
	// AlternativeConfidenceGap is how close (in classification.Confidence
	// units) an alternative candidate's confidence must be to the primary
	// result's confidence to count as "materially close," triggering a
	// review item even when the primary result is itself above
	// ClassificationConfidenceThreshold. Defaults to 0.05.
	AlternativeConfidenceGap float64
}

// DefaultPolicy returns the conservative default Policy every threshold
// documented on the Policy fields above uses, applied whenever a caller
// passes a zero-value Policy to Build (see resolvePolicy). Mirrors
// ingestion.DefaultLimits' role for ingestion.Limits.
func DefaultPolicy() Policy {
	return Policy{
		ClassificationConfidenceThreshold: float64(classification.DefaultReviewThreshold),
		OCRConfidenceThreshold:            70.0,
		NumericOCRConfidenceThreshold:     70.0,
		RequireReviewForAllOCR:            false,
		ReconciliationFailureBlocks:       false,
		MaterialAmountThreshold:           0,
		MaterialPercentOfRevenue:          0,
		ReviewAllClassifications:          false,
		EnableAssumptionReview:            false,
		AlternativeConfidenceGap:          0.05,
	}
}

// resolvePolicy returns p if any field differs from the zero value,
// otherwise DefaultPolicy() — so a caller passing Policy{} to Build gets
// sane defaults rather than every threshold effectively disabled.
func resolvePolicy(p Policy) Policy {
	if p == (Policy{}) {
		return DefaultPolicy()
	}
	return p
}

// RowContext supplies the row/period/provenance context Build needs
// alongside an ingestion.Row's Cells to produce OCR review items, without
// requiring a caller to hand over a whole ingestion.Result. A caller
// typically builds one RowContext per ingestion.Row it already has (Row.ID,
// Row.PageIndex, and the financial.Period each Cell.ColumnIndex resolved
// to, e.g. from ingestion.Metadata.Periods).
type RowContext struct {
	// RowID is the source ingestion.Row.ID (and, once carried through
	// ToRawLineItems, the same string as the corresponding
	// financial.RawLineItem.ID/MappedLineItem.SourceID) — the join key
	// between an OCR review item and its classification/structural review
	// item, when both exist for the same row.
	RowID string
	// Label is the row's label, for display.
	Label string
	// PageIndex is the row's source PDF page (ingestion.Row.PageIndex).
	PageIndex int
	// Cells is the row's cells, exactly as produced by ingestion (or
	// ingestion/pdf/ingestion/pdf's OCR path) — see ingestion.Cell.
	Cells []ingestion.Cell
	// ColumnPeriods maps a Cell.ColumnIndex to the financial.Period that
	// column represents, so a numeric OCR review item can carry Period
	// context. A column with no entry here is treated as having no known
	// period (Period left empty on the resulting item).
	ColumnPeriods map[int]financial.Period
}

// BuildInput bundles every optional source Build can consume. Every field is
// independently optional — Build simply produces no items for a category
// whose input is empty/nil, rather than requiring a caller to assemble a
// full pipeline result just to review, say, OCR cells alone.
type BuildInput struct {
	// Classifications is every classification.Result to consider for
	// KindClassification items, alongside the financial.RawLineItem each
	// one was produced from (Results[i] corresponds to Raws[i]) — Build
	// needs both: Result for the proposal/confidence/alternatives, RawLineItem
	// for Label/ParentLabel/Kind/StatementType display context that
	// classification.Result does not fully duplicate.
	Classifications []classification.Result
	Raws            []financial.RawLineItem

	// Rows supplies row/cell context for KindOCRText/KindOCRNumeric items.
	// See RowContext.
	Rows []RowContext

	// Periods is every ingestion.DetectedPeriod to consider for KindPeriod
	// items.
	Periods []ingestion.DetectedPeriod

	// Reconciliation, when non-nil, supplies reconciliation.Check values to
	// consider for KindReconciliation items.
	Reconciliation *reconciliation.Result

	// Adjustments is every adjustments.Adjustment to consider for
	// KindAdjustment confirmation items. Build never invents an adjustment;
	// every one supplied here simply gets a review item to confirm it.
	Adjustments []adjustments.Adjustment

	// Assumptions, when non-nil and Policy.EnableAssumptionReview is true,
	// supplies resolved valuation settings to expose as
	// KindValuationAssumption items. Carried as a small local interface
	// (see AssumptionSource) rather than importing settings.Resolution
	// directly, so this package's compile-time dependency surface stays
	// minimal; settings.Resolution already satisfies it structurally (see
	// build_test.go).
	Assumptions AssumptionSource

	// Revenue is the known total revenue for the dataset being reviewed,
	// used by IsMaterial when Policy.MaterialPercentOfRevenue > 0. A nil
	// pointer means revenue is unknown, in which case the
	// percent-of-revenue leg of materiality never applies (see IsMaterial).
	Revenue *float64
}

// AssumptionSource is the minimal shape Build needs from a resolved
// valuation-settings snapshot. settings.Resolution satisfies this directly
// (Values map[string]any, Sources map[string]settings.Scope — settings.Scope
// is a defined string type, so it satisfies the Sources method's string
// return via a simple conversion at the call site — see build_test.go for
// the exact adapter used against a real settings.Resolution). Defined
// locally, rather than importing the settings package, exactly like every
// other upstream payload in this file: review.go's own stable, minimal
// contract, not a re-export.
type AssumptionSource interface {
	// AssumptionValues returns the resolved values to expose for review,
	// keyed by setting key (e.g. "sde_multiple", "method_enabled.sde").
	AssumptionValues() map[string]any
	// AssumptionSource returns which scope supplied the value for key, or
	// "" if key is unknown.
	AssumptionSourceFor(key string) string
}

// Build converts a BuildInput into a deterministically ordered Plan under
// the given Policy. Build never mutates any part of in and performs no I/O.
// Every returned ReviewItem starts at StatusPending — see the package doc
// comment: Build only proposes, it never auto-confirms.
func Build(in BuildInput, policy Policy) Plan {
	policy = resolvePolicy(policy)

	var items []ReviewItem
	items = append(items, buildClassificationItems(in, policy)...)
	items = append(items, buildOCRItems(in, policy)...)
	items = append(items, buildPeriodItems(in, policy)...)
	items = append(items, buildStructureItems(in, policy)...)
	items = append(items, buildReconciliationItems(in, policy)...)
	items = append(items, buildAdjustmentItems(in, policy)...)
	if policy.EnableAssumptionReview && in.Assumptions != nil {
		items = append(items, buildAssumptionItems(in, policy)...)
	}

	sortItems(items)
	return Plan{Version: SchemaVersion, Items: items, Summary: summarize(items)}
}

// sortItems orders items deterministically: by Severity (BLOCKING, ERROR,
// WARNING, INFO), then by Kind (string order), then by SourceRowID (string
// order), then by ID (string order) — a real, tested sort.SliceStable over
// documented keys, never left to append/map order. See Plan.Items' doc
// comment.
func sortItems(items []ReviewItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if ra, rb := severityRank(a.Severity), severityRank(b.Severity); ra != rb {
			return ra < rb
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.SourceRowID != b.SourceRowID {
			return a.SourceRowID < b.SourceRowID
		}
		return a.ID < b.ID
	})
}

// --- Classification review items ---------------------------------------

// buildClassificationID returns the deterministic ID for a KindClassification
// item: "classification:<row-id>". Depends only on the row ID, by design —
// re-running Build with the same row ID but a different proposed
// Code/Confidence/Source (e.g. after an alias/rule change) keeps the SAME
// ID, so a caller can find "the same review item" across a rebuild after
// upstream classification data changed slightly. Changing the ROW ID
// (e.g. re-ingesting from a different source document) legitimately
// produces a different ID, since it is then a different row.
func buildClassificationID(rowID string) string {
	return fmt.Sprintf("classification:%s", rowID)
}

func buildClassificationItems(in BuildInput, policy Policy) []ReviewItem {
	items := make([]ReviewItem, 0, len(in.Classifications))
	for i, res := range in.Classifications {
		var raw financial.RawLineItem
		if i < len(in.Raws) {
			raw = in.Raws[i]
		}
		if !classificationNeedsReview(res, policy) {
			continue
		}

		severity := SeverityWarning
		reason := fmt.Sprintf("confidence %.2f below threshold %.2f", float64(res.Confidence), policy.ClassificationConfidenceThreshold)
		if res.IsUnknown() {
			severity = SeverityBlocking
			reason = "classifier could not propose a canonical code (UNKNOWN)"
		} else if res.ReviewRequired {
			severity = SeverityWarning
			reason = "classifier flagged this mapping for review"
		}
		if materiallyCloseAlternative(res, policy) {
			if severity != SeverityBlocking {
				severity = SeverityWarning
			}
			reason = fmt.Sprintf("%s; a close alternative exists", reason)
		}
		if policy.ReviewAllClassifications && !res.IsUnknown() && res.Confidence >= classification.Confidence(policy.ClassificationConfidenceThreshold) && !materiallyCloseAlternative(res, policy) {
			severity = SeverityInfo
			reason = "review requested for all classifications"
		}

		alts := make([]ClassificationAlternative, 0, len(res.Alternatives))
		for _, c := range res.Alternatives {
			alts = append(alts, ClassificationAlternative{Code: c.Code, Confidence: float64(c.Confidence), Reason: c.Reason})
		}

		title := fmt.Sprintf("Classification review: %s", displayLabel(res.Label, raw.Label))
		items = append(items, ReviewItem{
			ID:            buildClassificationID(res.RowID),
			Kind:          KindClassification,
			Severity:      severity,
			Required:      severity == SeverityBlocking || severity == SeverityError,
			Title:         title,
			Reason:        reason,
			SourceRowID:   res.RowID,
			CurrentValue:  string(res.Code),
			ProposedValue: string(res.Code),
			RelatedCodes:  []string{string(res.Source)},
			Status:        StatusPending,
			Classification: &ClassificationPayload{
				OriginalLabel: firstNonEmpty(res.Label, raw.Label),
				ParentLabel:   raw.ParentLabel,
				ProposedCode:  res.Code,
				Confidence:    float64(res.Confidence),
				Source:        string(res.Source),
				Alternatives:  alts,
				Kind:          res.Kind,
				StatementType: raw.StatementType,
			},
		})
	}
	return items
}

// classificationNeedsReview implements section 4's trigger rules: UNKNOWN,
// ReviewRequired, below-threshold confidence, materially-close alternatives,
// or the caller opted into reviewing everything.
func classificationNeedsReview(res classification.Result, policy Policy) bool {
	if policy.ReviewAllClassifications {
		return true
	}
	if res.IsUnknown() {
		return true
	}
	if res.ReviewRequired {
		return true
	}
	if float64(res.Confidence) < policy.ClassificationConfidenceThreshold {
		return true
	}
	if materiallyCloseAlternative(res, policy) {
		return true
	}
	return false
}

// materiallyCloseAlternative reports whether the strongest alternative
// candidate's confidence is within policy.AlternativeConfidenceGap of the
// primary result's confidence — "materially close" per section 4.
// classification.Result.Alternatives is already ordered strongest-first, so
// only the first entry needs checking.
func materiallyCloseAlternative(res classification.Result, policy Policy) bool {
	if res.IsUnknown() || len(res.Alternatives) == 0 {
		return false
	}
	gap := policy.AlternativeConfidenceGap
	if gap <= 0 {
		gap = DefaultPolicy().AlternativeConfidenceGap
	}
	top := res.Alternatives[0]
	diff := float64(res.Confidence) - float64(top.Confidence)
	if diff < 0 {
		diff = -diff
	}
	return diff <= gap
}

// --- OCR review items -----------------------------------------------------

// buildOCRTextID returns the deterministic ID for a KindOCRText item:
// "ocr-text:<row-id>:<column-index>". Depends only on the row ID and column
// index; a re-run with the same cell position but different OCR
// confidence/text keeps the same ID.
func buildOCRTextID(rowID string, columnIndex int) string {
	return fmt.Sprintf("ocr-text:%s:%d", rowID, columnIndex)
}

// buildOCRNumericID returns the deterministic ID for a KindOCRNumeric item:
// "ocr-numeric:<row-id>:<period>". Falls back to the column index in place
// of period when no period is known for that column, so the ID is always
// well-formed. Depends only on row ID and period/column — a re-run with the
// same cell position but a different parsed amount/confidence/correction
// keeps the same ID, letting a caller track "the same numeric cell" across
// a rebuild.
func buildOCRNumericID(rowID string, period financial.Period, columnIndex int) string {
	if period != "" {
		return fmt.Sprintf("ocr-numeric:%s:%s", rowID, period)
	}
	return fmt.Sprintf("ocr-numeric:%s:col%d", rowID, columnIndex)
}

func buildOCRItems(in BuildInput, policy Policy) []ReviewItem {
	var items []ReviewItem
	// Sort RowContext by RowID first so iteration order (and therefore any
	// tie in later stable sort) never depends on caller-supplied slice
	// order alone being deterministic — belt-and-suspenders, since Build's
	// caller is expected to supply a stable slice already, but this
	// package never relies on that assumption silently.
	rows := make([]RowContext, len(in.Rows))
	copy(rows, in.Rows)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].RowID < rows[j].RowID })

	for _, row := range rows {
		cells := make([]ingestion.Cell, len(row.Cells))
		copy(cells, row.Cells)
		sort.SliceStable(cells, func(i, j int) bool { return cells[i].ColumnIndex < cells[j].ColumnIndex })

		for _, cell := range cells {
			if cell.OCR == nil {
				continue
			}
			period := row.ColumnPeriods[cell.ColumnIndex]
			if item, ok := ocrNumericItem(row, cell, period, policy); ok {
				items = append(items, item)
				continue
			}
			if item, ok := ocrTextItem(row, cell, policy); ok {
				items = append(items, item)
			}
		}
	}
	return items
}

// ocrNumericItem builds a KindOCRNumeric item for cell if it looks like a
// numeric cell (has a parsed value, or is the OCR-ambiguous case: OCR
// provenance present but Cell.Parsed == false) and warrants review under
// policy. The second return value is false for a cell this function
// declines to treat as numeric (e.g. an OCR label cell that never had a
// numeric value attempted), in which case the caller falls back to
// ocrTextItem.
//
// ingestion.Cell.Parsed == false is genuinely ambiguous on its own: it
// covers BOTH "this numeric-shaped cell failed to parse" (the case section
// 5 wants a review item for) AND "this is an ordinary label cell that was
// never attempted as numeric at all" (ingestion.Cell's own doc comment
// states both). Cell has no field marking "this is a period/value column"
// (that context is row-level, not cell-level), so this function uses
// row.ColumnPeriods — the same column-to-period map Build's caller already
// supplies for every recognized value column — as the structural signal: a
// cell only counts as an unparsed-numeric candidate when its column is a
// known period column. A label column (never present in ColumnPeriods)
// therefore always falls through to ocrTextItem instead, exactly as a
// clean, non-OCR label cell would.
func ocrNumericItem(row RowContext, cell ingestion.Cell, period financial.Period, policy Policy) (ReviewItem, bool) {
	_, isValueColumn := row.ColumnPeriods[cell.ColumnIndex]
	ambiguous := cell.OCR != nil && !cell.Parsed && isValueColumn
	looksNumeric := cell.Numeric != nil || ambiguous
	if !looksNumeric {
		return ReviewItem{}, false
	}

	prov := cell.OCR
	needsReview := policy.RequireReviewForAllOCR ||
		ambiguous ||
		prov.NumericCorrected ||
		(prov.Confidence >= 0 && prov.Confidence < policy.NumericOCRConfidenceThreshold)
	if !needsReview {
		return ReviewItem{}, false
	}

	severity := SeverityWarning
	reason := fmt.Sprintf("OCR numeric confidence %.1f below threshold %.1f", prov.Confidence, policy.NumericOCRConfidenceThreshold)
	required := false
	switch {
	case ambiguous:
		severity = SeverityBlocking
		required = true
		reason = "OCR could not confidently parse this numeric value"
	case prov.NumericCorrected:
		severity = SeverityError
		required = true
		reason = "numeric value required a heuristic OCR digit correction before it would parse"
	case policy.RequireReviewForAllOCR && prov.Confidence >= policy.NumericOCRConfidenceThreshold:
		severity = SeverityInfo
		reason = "review requested for all OCR-derived values (strict mode)"
	}

	current := ""
	if cell.Numeric != nil {
		current = fmt.Sprintf("%v", *cell.Numeric)
	}

	item := ReviewItem{
		ID:            buildOCRNumericID(row.RowID, period, cell.ColumnIndex),
		Kind:          KindOCRNumeric,
		Severity:      severity,
		Required:      required,
		Title:         fmt.Sprintf("OCR numeric review: %s", displayLabel(row.Label, "")),
		Reason:        reason,
		SourceRowID:   row.RowID,
		Provenance:    fmt.Sprintf("page %d, column %d", row.PageIndex, cell.ColumnIndex),
		Period:        period,
		CurrentValue:  current,
		ProposedValue: current,
		Status:        StatusPending,
		OCRNumeric: &OCRNumericPayload{
			OriginalText:     prov.OriginalText,
			FinalText:        cell.Raw,
			ParsedAmount:     cell.Numeric,
			Ambiguous:        ambiguous,
			NumericCorrected: prov.NumericCorrected,
			Confidence:       prov.Confidence,
			Page:             row.PageIndex,
			PixelBounds:      toPixelBounds(prov.PixelBounds),
			Period:           period,
		},
	}
	return item, true
}

// ocrTextItem builds a KindOCRText item for a non-numeric OCR cell (a
// label) that warrants review under policy.
func ocrTextItem(row RowContext, cell ingestion.Cell, policy Policy) (ReviewItem, bool) {
	prov := cell.OCR
	needsReview := policy.RequireReviewForAllOCR ||
		(prov.Confidence >= 0 && prov.Confidence < policy.OCRConfidenceThreshold)
	if !needsReview {
		return ReviewItem{}, false
	}

	severity := SeverityWarning
	reason := fmt.Sprintf("OCR label confidence %.1f below threshold %.1f", prov.Confidence, policy.OCRConfidenceThreshold)
	if policy.RequireReviewForAllOCR && prov.Confidence >= policy.OCRConfidenceThreshold {
		severity = SeverityInfo
		reason = "review requested for all OCR-derived values (strict mode)"
	}

	item := ReviewItem{
		ID:            buildOCRTextID(row.RowID, cell.ColumnIndex),
		Kind:          KindOCRText,
		Severity:      severity,
		Required:      false,
		Title:         fmt.Sprintf("OCR label review: %s", displayLabel(row.Label, cell.Raw)),
		Reason:        reason,
		SourceRowID:   row.RowID,
		Provenance:    fmt.Sprintf("page %d, column %d", row.PageIndex, cell.ColumnIndex),
		CurrentValue:  cell.Raw,
		ProposedValue: cell.Raw,
		Status:        StatusPending,
		OCRText: &OCRTextPayload{
			OriginalText: prov.OriginalText,
			FinalText:    cell.Raw,
			Confidence:   prov.Confidence,
			Page:         row.PageIndex,
			PixelBounds:  toPixelBounds(prov.PixelBounds),
		},
	}
	return item, true
}

func toPixelBounds(pb *ingestion.PixelBounds) *PixelBounds {
	if pb == nil {
		return nil
	}
	return &PixelBounds{X: pb.X, Y: pb.Y, Width: pb.Width, Height: pb.Height}
}

// --- Period review items ---------------------------------------------------

// buildPeriodID returns the deterministic ID for a KindPeriod item:
// "period:<source-row>:<column>". Since a DetectedPeriod is column-scoped
// rather than row-scoped, "source-row" here is the literal string "column"
// combined with the column index is sufficient on its own — this package
// uses the fixed sentinel row token "header" (no single RawLineItem owns a
// period column) so the ID still reads as the documented
// "period:<source-row>:<column>" shape while being unambiguous. Depends
// only on ColumnIndex; a re-run with the same column but a different
// resolved Period/PeriodType/Confidence keeps the same ID.
func buildPeriodID(columnIndex int) string {
	return fmt.Sprintf("period:header:%d", columnIndex)
}

func buildPeriodItems(in BuildInput, policy Policy) []ReviewItem {
	periods := make([]ingestion.DetectedPeriod, len(in.Periods))
	copy(periods, in.Periods)
	sort.SliceStable(periods, func(i, j int) bool { return periods[i].ColumnIndex < periods[j].ColumnIndex })

	items := make([]ReviewItem, 0, len(periods))
	for _, p := range periods {
		unparsed := p.PeriodType == ingestion.PeriodTypeUnknown
		ambiguous := !unparsed && p.Confidence < 1.0
		if !unparsed && !ambiguous {
			continue
		}

		severity := SeverityWarning
		reason := fmt.Sprintf("period label confidence %.2f", p.Confidence)
		required := false
		if unparsed {
			severity = SeverityBlocking
			required = true
			reason = fmt.Sprintf("period label %q could not be parsed into a known period type", p.OriginalLabel)
		}

		items = append(items, ReviewItem{
			ID:            buildPeriodID(p.ColumnIndex),
			Kind:          KindPeriod,
			Severity:      severity,
			Required:      required,
			Title:         fmt.Sprintf("Period review: %s", p.OriginalLabel),
			Reason:        reason,
			Period:        p.Period,
			CurrentValue:  p.OriginalLabel,
			ProposedValue: string(p.Period),
			Status:        StatusPending,
			PeriodDetail: &PeriodPayload{
				OriginalLabel:      p.OriginalLabel,
				ProposedPeriod:     p.Period,
				ProposedPeriodType: string(p.PeriodType),
				ColumnIndex:        p.ColumnIndex,
				Confidence:         p.Confidence,
				Evidence:           p.Evidence,
			},
		})
	}
	return items
}

// --- Structural review items ------------------------------------------------

// buildStructureID returns the deterministic ID for a KindStructure item:
// "structure:<row-id>". Depends only on the row ID; a re-run with the same
// row but a different Kind read keeps the same ID.
func buildStructureID(rowID string) string {
	return fmt.Sprintf("structure:%s", rowID)
}

// buildStructureItems creates INFO-severity confirmation items for every
// row carrying a non-zero upstream financial.RowKind (heading/subtotal/
// total), so a caller can accept or override structure per section 7. Every
// such row already flows through classification into
// SourceStructural/RowStatusIgnored|Subtotal|Total, so these items are
// advisory confirmation, not a sign anything is wrong — hence INFO, never
// BLOCKING, and Required == false: nothing downstream depends on the
// caller resolving these before valuation can proceed.
func buildStructureItems(in BuildInput, _ Policy) []ReviewItem {
	items := make([]ReviewItem, 0)
	for i, res := range in.Classifications {
		var raw financial.RawLineItem
		if i < len(in.Raws) {
			raw = in.Raws[i]
		}
		kind := res.Kind
		if kind == financial.RowKindNormal {
			continue
		}
		items = append(items, ReviewItem{
			ID:            buildStructureID(res.RowID),
			Kind:          KindStructure,
			Severity:      SeverityInfo,
			Required:      false,
			Title:         fmt.Sprintf("Structure review: %s", displayLabel(res.Label, raw.Label)),
			Reason:        fmt.Sprintf("row structurally read as %q", string(kind)),
			SourceRowID:   res.RowID,
			CurrentValue:  string(kind),
			ProposedValue: string(kind),
			Status:        StatusPending,
			Structure: &StructurePayload{
				Label:         firstNonEmpty(res.Label, raw.Label),
				ProposedKind:  kind,
				StatementType: raw.StatementType,
			},
		})
	}
	return items
}

// --- Reconciliation review items --------------------------------------------

// buildReconciliationID returns the deterministic ID for a
// KindReconciliation item: "reconciliation:<check-code>:<period>". Depends
// only on the check code and period; a re-run with the same check on the
// same period but a different Status/Expected/Actual keeps the same ID.
func buildReconciliationID(code, period string) string {
	if period == "" {
		period = "dataset"
	}
	return fmt.Sprintf("reconciliation:%s:%s", code, period)
}

func buildReconciliationItems(in BuildInput, policy Policy) []ReviewItem {
	if in.Reconciliation == nil {
		return nil
	}
	checks := make([]reconciliation.Check, len(in.Reconciliation.Checks))
	copy(checks, in.Reconciliation.Checks)
	sort.SliceStable(checks, func(i, j int) bool {
		if checks[i].Period != checks[j].Period {
			return checks[i].Period < checks[j].Period
		}
		return checks[i].Code < checks[j].Code
	})

	items := make([]ReviewItem, 0)
	for _, c := range checks {
		if c.Status != reconciliation.StatusFail && c.Status != reconciliation.StatusWarning {
			continue
		}

		severity := SeverityWarning
		required := false
		if c.Status == reconciliation.StatusFail {
			severity = SeverityError
			required = true
			if policy.ReconciliationFailureBlocks {
				severity = SeverityBlocking
			}
		}

		items = append(items, ReviewItem{
			ID:           buildReconciliationID(string(c.Code), string(c.Period)),
			Kind:         KindReconciliation,
			Severity:     severity,
			Required:     required,
			Title:        fmt.Sprintf("Reconciliation review: %s", c.Code),
			Reason:       c.Explanation,
			Period:       c.Period,
			RelatedCodes: append([]string{string(c.Code)}, c.RelatedCodes...),
			Status:       StatusPending,
			Reconciliation: &ReconciliationPayload{
				CheckCode:                string(c.Code),
				CheckStatus:              string(c.Status),
				Expected:                 c.Expected,
				Actual:                   c.Actual,
				Difference:               c.Difference,
				ToleranceAbsolute:        c.Tolerance.Absolute,
				ToleranceRelativePercent: c.Tolerance.RelativePercent,
				Explanation:              c.Explanation,
			},
		})
	}
	return items
}

// --- Adjustment review items ------------------------------------------------

// buildAdjustmentID returns the deterministic ID for a KindAdjustment item:
// "adjustment:<adjustment-id>". Depends only on the adjustment's own caller-
// supplied ID; a re-run with the same ID but a different Amount/Effect/
// Reason keeps the same ID.
func buildAdjustmentID(adjID string) string {
	return fmt.Sprintf("adjustment:%s", adjID)
}

// buildAdjustmentItems creates one confirmation item per supplied
// adjustments.Adjustment, per section 9: Build never generates an
// adjustment itself, it only creates a review item to confirm one the
// caller already constructed.
func buildAdjustmentItems(in BuildInput, _ Policy) []ReviewItem {
	adjs := make([]adjustments.Adjustment, len(in.Adjustments))
	copy(adjs, in.Adjustments)
	sort.SliceStable(adjs, func(i, j int) bool { return adjs[i].ID < adjs[j].ID })

	items := make([]ReviewItem, 0, len(adjs))
	for _, adj := range adjs {
		items = append(items, ReviewItem{
			ID:            buildAdjustmentID(string(adj.ID)),
			Kind:          KindAdjustment,
			Severity:      SeverityInfo,
			Required:      false,
			Title:         fmt.Sprintf("Adjustment confirmation: %s", adj.Type),
			Reason:        adj.Reason,
			Period:        adj.Period,
			CurrentValue:  fmt.Sprintf("%v", adj.Amount),
			ProposedValue: fmt.Sprintf("%v", adj.Amount),
			Status:        StatusPending,
			Adjustment: &AdjustmentPayload{
				AdjustmentID: string(adj.ID),
				Type:         string(adj.Type),
				Amount:       adj.Amount,
				Effect:       string(adj.Effect),
				Reason:       adj.Reason,
				Period:       adj.Period,
				Included:     adj.Included,
			},
		})
	}
	return items
}

// --- Valuation assumption review items --------------------------------------

// buildAssumptionID returns the deterministic ID for a
// KindValuationAssumption item: "assumption:<setting-key>". Depends only on
// the setting key; a re-run with the same key but a different resolved
// value/source scope keeps the same ID.
func buildAssumptionID(key string) string {
	return fmt.Sprintf("assumption:%s", key)
}

func buildAssumptionItems(in BuildInput, _ Policy) []ReviewItem {
	values := in.Assumptions.AssumptionValues()
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	items := make([]ReviewItem, 0, len(keys))
	for _, key := range keys {
		v := values[key]
		items = append(items, ReviewItem{
			ID:            buildAssumptionID(key),
			Kind:          KindValuationAssumption,
			Severity:      SeverityInfo,
			Required:      false,
			Title:         fmt.Sprintf("Assumption confirmation: %s", key),
			Reason:        "resolved valuation input available for confirmation",
			CurrentValue:  fmt.Sprintf("%v", v),
			ProposedValue: fmt.Sprintf("%v", v),
			Status:        StatusPending,
			Assumption: &AssumptionPayload{
				SettingKey:   key,
				CurrentValue: v,
				SourceScope:  in.Assumptions.AssumptionSourceFor(key),
			},
		})
	}
	return items
}

// --- Materiality -------------------------------------------------------------

// IsMaterial implements section 19's single materiality helper: amount is
// material if abs(amount) >= Policy.MaterialAmountThreshold OR (revenue is
// known, Policy.MaterialPercentOfRevenue > 0, and abs(amount) >=
// MaterialPercentOfRevenue * revenue). Both Policy fields default to 0,
// which means "not applied" for that leg — with both at their zero-value
// default, IsMaterial always returns true (materiality gating is OFF by
// default; a 0 threshold never means "nothing is material").
func IsMaterial(amount float64, revenue *float64, policy Policy) bool {
	abs := math.Abs(amount)
	if policy.MaterialAmountThreshold > 0 && abs >= policy.MaterialAmountThreshold {
		return true
	}
	if policy.MaterialPercentOfRevenue > 0 && revenue != nil && abs >= policy.MaterialPercentOfRevenue*math.Abs(*revenue) {
		return true
	}
	if policy.MaterialAmountThreshold <= 0 && policy.MaterialPercentOfRevenue <= 0 {
		return true
	}
	return false
}

// --- small local helpers ----------------------------------------------------

func displayLabel(candidates ...string) string {
	return firstNonEmpty(candidates...)
}

func firstNonEmpty(candidates ...string) string {
	for _, c := range candidates {
		if c != "" {
			return c
		}
	}
	return ""
}
