// Package review turns the outputs of ingestion, classification, and
// reconciliation into a structured, human-reviewable ReviewPlan, accepts a
// caller's Decisions against that plan, and Applies those decisions to
// produce corrected financial-domain data:
//
//	ingestion / classification / reconciliation results
//	        -> review.Build
//	        -> Plan (a deterministic, ordered []ReviewItem)
//	        -> caller/user Decisions
//	        -> review.Apply
//	        -> corrected financial-domain data (ApplyResult)
//
// This package contains no UI, no persistence, no HTTP, no database, no
// AI/LLM. It has no idea whether the "caller" resolving a Decision is a
// script, a future Vue app, or a human clicking a button — it only defines
// the domain shape a review workflow needs and applies it deterministically.
// A future UI is expected to RENDER this model (Plan, ReviewItem, Decision,
// ApplyResult, Readiness), not duplicate its rules — see the repository
// README's "review" section for the full architecture diagram and the
// non-goals this package deliberately leaves out.
//
// Build never auto-confirms anything it proposes: every ReviewItem starts
// unresolved (StatusPending) regardless of how confident the upstream data
// was, and only an explicit Decision passed to Apply can resolve one. This
// mirrors financial/classification's own "Classify only proposes; it never
// auto-confirms a mapping" rule one layer up — review.Build inherits that
// discipline rather than relaxing it.
//
// Every exported function here is pure: Build and Apply never mutate their
// inputs and perform no I/O. Determinism is a hard requirement throughout:
// item IDs, item ordering, and every collection this package returns are
// documented and independent of Go map iteration order (see each type's own
// doc comment for its specific ordering/ID rule).
package review

import "github.com/themurtez/go-valuate/financial"

// SchemaVersion identifies this package's fixed ReviewItem/Plan/Decision/
// ApplyResult shapes and the deterministic rules that produce them (ID
// formats, severity assignment, ordering, readiness state transitions).
// Echoed on Plan.Version. Bump this whenever any of that changes in a way
// that could make a historical Plan or ApplyResult not reproduce
// identically under new code — see the repository README's
// versioning-strategy section for the general rule this follows.
const SchemaVersion = "1.0.0"

// Kind identifies which stage of the pipeline a ReviewItem originated from.
// A stable, string-based enum (like classification.Source and
// reconciliation.CheckCode) so a future UI can group/filter/route items by
// kind without parsing free text.
type Kind string

const (
	// KindClassification covers a raw row whose classification.Result was
	// UNKNOWN, below the confidence threshold, materially close to an
	// alternative, or explicitly requested for review. See BuildClassification.
	KindClassification Kind = "CLASSIFICATION"
	// KindOCRText covers a row/cell whose label text was reconstructed from
	// low-confidence OCR. See ingestion.OCRProvenance.
	KindOCRText Kind = "OCR_TEXT"
	// KindOCRNumeric covers a numeric OCR value that is low-confidence,
	// ambiguous (OCR provenance present but Cell.Parsed == false), or was
	// heuristically corrected. See ingestion.OCRProvenance.NumericCorrected.
	KindOCRNumeric Kind = "OCR_NUMERIC"
	// KindPeriod covers an ambiguous, unparsed, or caller-override-required
	// reporting period. See ingestion.DetectedPeriod/PeriodType.
	KindPeriod Kind = "PERIOD"
	// KindStructure covers a row's structural role (heading/subtotal/total)
	// requiring confirmation. See financial.RowKind.
	KindStructure Kind = "STRUCTURE"
	// KindReconciliation covers a reconciliation.Check that failed or
	// warrants review under the caller's Policy. See reconciliation.Check.
	KindReconciliation Kind = "RECONCILIATION"
	// KindAdjustment covers an explicit, caller-supplied normalization
	// adjustment awaiting confirmation. See adjustments.Adjustment. Build
	// never invents an adjustment; it only creates a review item to confirm
	// one the caller already constructed and passed in.
	KindAdjustment Kind = "ADJUSTMENT"
	// KindValuationAssumption covers a resolved valuation setting (a
	// multiple, a discount rate, a method-inclusion flag, ...) exposed for
	// confirmation. Opt-in via Policy.EnableAssumptionReview. See
	// settings.Resolution.
	KindValuationAssumption Kind = "VALUATION_ASSUMPTION"
)

// Severity is a stable, structured signal for how urgently a ReviewItem
// needs attention. Readiness (see readiness.go) is driven ENTIRELY by
// Severity and Status — never by parsing a Reason or Message string — so
// every severity assignment in this package happens at Build time, from
// structured fields (a Policy threshold, a boolean flag, an enum value),
// never from free text.
type Severity string

const (
	// SeverityInfo is context only: nothing needs to change, but the item is
	// worth surfacing (e.g. a structural row the caller may want to confirm).
	SeverityInfo Severity = "INFO"
	// SeverityWarning means review is recommended but valuation can proceed
	// without resolving it.
	SeverityWarning Severity = "WARNING"
	// SeverityError means the input is suspicious or invalid and should
	// normally be corrected, but does not by itself prevent valuation.
	SeverityError Severity = "ERROR"
	// SeverityBlocking means valuation must not proceed until this item is
	// resolved. See readiness.go: NOT_READY occurs if and only if an
	// unresolved SeverityBlocking item remains.
	SeverityBlocking Severity = "BLOCKING"
)

// severityRank orders Severity from most to least urgent, for deterministic
// sorting (see Plan's ordering doc comment). Not exported: callers compare
// Severity by equality, never by an implied numeric rank of their own.
func severityRank(s Severity) int {
	switch s {
	case SeverityBlocking:
		return 0
	case SeverityError:
		return 1
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 3
	default:
		return 4
	}
}

// Status is a ReviewItem's current resolution state. Readiness reads Status
// directly (never a Reason string) to decide whether an item still counts
// as unresolved — see readiness.go.
type Status string

const (
	// StatusPending means no Decision has been applied to this item yet.
	// Every item produced by Build starts here — Build never auto-resolves
	// its own proposals (see the package doc comment).
	StatusPending Status = "PENDING"
	// StatusResolved means a valid Decision was applied and accepted.
	StatusResolved Status = "RESOLVED"
	// StatusRejected means a valid Decision explicitly rejected/excluded the
	// item's row or value (e.g. Action = ACTION_REJECT on an OCR numeric
	// item, marking the row unavailable).
	StatusRejected Status = "REJECTED"
	// StatusInvalidDecision means a Decision targeting this item was
	// supplied but failed validation (see Issue) and was not applied; the
	// item remains effectively unresolved for readiness purposes.
	StatusInvalidDecision Status = "INVALID_DECISION"
)

// IsUnresolved reports whether an item in this Status should still count as
// "not yet resolved" for readiness/summary purposes. Only StatusResolved and
// StatusRejected are resolutions a caller deliberately made; StatusPending
// and StatusInvalidDecision both mean "no accepted decision exists for this
// item."
func (s Status) IsUnresolved() bool {
	return s == StatusPending || s == StatusInvalidDecision
}

// ClassificationPayload carries classification-specific review context. See
// KindClassification.
type ClassificationPayload struct {
	// OriginalLabel is the source row's label exactly as it appeared.
	OriginalLabel string `json:"original_label"`
	// ParentLabel is the enclosing section label, if any.
	ParentLabel string `json:"parent_label,omitempty"`
	// ProposedCode is the classifier's primary proposed canonical code.
	// Empty when the row is SourceUnknown or structural.
	ProposedCode financial.Code `json:"proposed_code,omitempty"`
	// Confidence is classification.Result.Confidence, copied verbatim (a
	// heuristic strength in [0, 1], not a statistical probability — see
	// classification.Confidence's own doc comment).
	Confidence float64 `json:"confidence"`
	// Source is classification.Result.Source, copied verbatim.
	Source string `json:"source"`
	// Alternatives lists runner-up (code, confidence) pairs, strongest
	// first, mirroring classification.Result.Alternatives' own order.
	Alternatives []ClassificationAlternative `json:"alternatives,omitempty"`
	// Kind carries forward the row's upstream structural read (see
	// financial.RowKind), for display alongside the proposed code.
	Kind financial.RowKind `json:"kind,omitempty"`
	// StatementType is the row's originating statement, for display
	// context.
	StatementType financial.StatementType `json:"statement_type,omitempty"`
}

// ClassificationAlternative is one runner-up classification candidate,
// mirroring classification.Candidate's shape without importing that
// package's exact type (this payload is this package's own stable contract
// — see the README's error-taxonomy reasoning for why review defines its
// own types rather than re-exporting upstream ones directly).
type ClassificationAlternative struct {
	Code       financial.Code `json:"code"`
	Confidence float64        `json:"confidence"`
	Reason     string         `json:"reason,omitempty"`
}

// OCRTextPayload carries OCR label-text review context. See KindOCRText.
type OCRTextPayload struct {
	// OriginalText is the OCR engine's recognized text exactly as reported
	// (ingestion.OCRProvenance.OriginalText).
	OriginalText string `json:"original_text"`
	// FinalText is the text actually used downstream (ingestion.Cell.Raw) —
	// identical to OriginalText for label cells, which this package never
	// corrects (only numeric cells get heuristic correction — see
	// OCRNumericPayload).
	FinalText string `json:"final_text"`
	// Confidence is the engine-reported confidence on the engine's own
	// scale (ingestion.OCRProvenance.Confidence) — see Policy's doc comment
	// for the scale mismatch between this and classification.Confidence.
	Confidence float64 `json:"confidence"`
	// Page is the 0-based source page, when known.
	Page int `json:"page"`
	// PixelBounds is the cell's bounding box on the source scanned image,
	// when available.
	PixelBounds *PixelBounds `json:"pixel_bounds,omitempty"`
}

// OCRNumericPayload carries OCR numeric-value review context. See
// KindOCRNumeric. Carries both the original OCR text and the final parsed
// value so a future UI can show the user exactly what the engine reported
// alongside whatever value (if any) was derived from it.
type OCRNumericPayload struct {
	// OriginalText is the OCR engine's recognized text exactly as reported,
	// before any numeric-safety correction
	// (ingestion.OCRProvenance.OriginalText).
	OriginalText string `json:"original_text"`
	// FinalText is the text actually parsed (ingestion.Cell.Raw) — may
	// differ from OriginalText when NumericCorrected is true.
	FinalText string `json:"final_text"`
	// ParsedAmount is the numeric value parsed from FinalText, when parsing
	// succeeded (ingestion.Cell.Numeric). Nil when the cell is ambiguous
	// (Parsed == false).
	ParsedAmount *float64 `json:"parsed_amount,omitempty"`
	// Ambiguous is true when this cell carries OCR provenance but did not
	// parse (Cell.OCR != nil && !Cell.Parsed) — the OCR-ambiguous case,
	// distinct from a clean, non-OCR parse failure, which this package does
	// not create a review item for.
	Ambiguous bool `json:"ambiguous"`
	// NumericCorrected is true when FinalText required a heuristic digit
	// correction before it would parse (ingestion.OCRProvenance.NumericCorrected).
	NumericCorrected bool `json:"numeric_corrected"`
	// Confidence is the engine-reported confidence on the engine's own
	// scale (ingestion.OCRProvenance.Confidence).
	Confidence float64 `json:"confidence"`
	// Page is the 0-based source page, when known.
	Page int `json:"page"`
	// PixelBounds is the cell's bounding box on the source scanned image,
	// when available.
	PixelBounds *PixelBounds `json:"pixel_bounds,omitempty"`
	// Period is the reporting period this numeric value belongs to, when
	// known.
	Period financial.Period `json:"period,omitempty"`
}

// PixelBounds mirrors ingestion.PixelBounds's shape (this package's own
// stable copy, not a re-export — see the README's error-taxonomy reasoning
// applied consistently across every upstream-derived payload here).
type PixelBounds struct {
	X, Y, Width, Height int
}

// PeriodPayload carries reporting-period review context. See KindPeriod.
type PeriodPayload struct {
	// OriginalLabel is the period header text exactly as it appeared
	// (ingestion.DetectedPeriod.OriginalLabel).
	OriginalLabel string `json:"original_label"`
	// ProposedPeriod is the canonical period ingestion assigned (which may
	// simply be a normalized form of OriginalLabel when unparsed — see
	// ingestion.PeriodTypeUnknown).
	ProposedPeriod financial.Period `json:"proposed_period"`
	// ProposedPeriodType is ingestion.DetectedPeriod.PeriodType, copied
	// verbatim (as a plain string so this package never imports ingestion's
	// PeriodType enum into its own public API).
	ProposedPeriodType string `json:"proposed_period_type"`
	// ColumnIndex is the 0-based source column this period was detected in.
	ColumnIndex int `json:"column_index"`
	// Confidence is ingestion.DetectedPeriod.Confidence, copied verbatim.
	Confidence float64 `json:"confidence"`
	// Evidence is ingestion.DetectedPeriod.Evidence, copied verbatim.
	Evidence string `json:"evidence,omitempty"`
}

// StructurePayload carries structural row-kind review context. See
// KindStructure. Reuses financial.RowKind directly rather than defining a
// parallel enum, per the task's explicit instruction.
type StructurePayload struct {
	// Label is the row's label as it appeared.
	Label string `json:"label"`
	// ProposedKind is the upstream structural read for this row.
	ProposedKind financial.RowKind `json:"proposed_kind"`
	// StatementType is the row's originating statement, for display
	// context.
	StatementType financial.StatementType `json:"statement_type,omitempty"`
}

// ReconciliationPayload carries reconciliation-check review context. See
// KindReconciliation.
type ReconciliationPayload struct {
	// CheckCode is reconciliation.Check.Code, copied verbatim (as a plain
	// string so this package never imports reconciliation's CheckCode type
	// into its own public API).
	CheckCode string `json:"check_code"`
	// CheckStatus is reconciliation.Check.Status, copied verbatim.
	CheckStatus string `json:"check_status"`
	// Expected mirrors reconciliation.Check.Expected.
	Expected *float64 `json:"expected,omitempty"`
	// Actual mirrors reconciliation.Check.Actual.
	Actual *float64 `json:"actual,omitempty"`
	// Difference mirrors reconciliation.Check.Difference.
	Difference *float64 `json:"difference,omitempty"`
	// ToleranceAbsolute mirrors reconciliation.Check.Tolerance.Absolute.
	ToleranceAbsolute float64 `json:"tolerance_absolute,omitempty"`
	// ToleranceRelativePercent mirrors
	// reconciliation.Check.Tolerance.RelativePercent.
	ToleranceRelativePercent float64 `json:"tolerance_relative_percent,omitempty"`
	// Explanation mirrors reconciliation.Check.Explanation.
	Explanation string `json:"explanation"`
}

// AdjustmentPayload carries explicit-adjustment review context. See
// KindAdjustment. Build only creates this to CONFIRM an adjustment the
// caller already constructed; it never invents one.
type AdjustmentPayload struct {
	// AdjustmentID is the source adjustments.Adjustment.ID.
	AdjustmentID string `json:"adjustment_id"`
	// Type is adjustments.Adjustment.Type, copied verbatim.
	Type string `json:"type"`
	// Amount is adjustments.Adjustment.Amount, copied verbatim (a
	// non-negative magnitude — see adjustments' own sign-convention doc
	// comment; this package does not reinterpret it).
	Amount float64 `json:"amount"`
	// Effect is adjustments.Adjustment.Effect, copied verbatim.
	Effect string `json:"effect,omitempty"`
	// Reason is adjustments.Adjustment.Reason, copied verbatim.
	Reason string `json:"reason,omitempty"`
	// Period is the reporting period this adjustment applies to.
	Period financial.Period `json:"period,omitempty"`
	// Included is adjustments.Adjustment.Included, copied verbatim — the
	// caller's own current include/exclude state before any review
	// decision.
	Included bool `json:"included"`
}

// AssumptionPayload carries a resolved valuation assumption exposed for
// confirmation. See KindValuationAssumption. This package does not decide
// values; it only exposes currently-resolved settings.Resolution entries
// and records overrides.
type AssumptionPayload struct {
	// SettingKey is the settings.Resolution.Values key this item exposes
	// (e.g. settings.FieldKeySDEMultiple or
	// settings.MethodFieldKey(settings.MethodSDE)).
	SettingKey string `json:"setting_key"`
	// CurrentValue is the currently-resolved value for SettingKey
	// (settings.Resolution.Values[SettingKey]), carried as `any` because
	// settings.Resolution.Values itself is `map[string]any` (numeric
	// fields resolve to float64, method-enable flags to bool) — this is
	// the one place this package accepts that upstream shape rather than
	// forcing a narrower type, since narrowing it here would require
	// duplicating settings' own field-type table.
	CurrentValue any `json:"current_value"`
	// SourceScope is the settings.Scope that supplied CurrentValue
	// (settings.Resolution.Sources[SettingKey]), copied verbatim as a
	// plain string.
	SourceScope string `json:"source_scope,omitempty"`
}

// ReviewItem is a single, stable unit of review: something upstream
// processing could not, or should not, resolve automatically. See the Kind
// constants for the eight review kinds and each Payload type's doc comment
// for kind-specific detail.
//
// Exactly one Payload field is populated, matching ReviewItem.Kind — this
// mirrors orchestrator.MethodOutcome's five separate typed pointers rather
// than one generic `any`/`map[string]any` field, so a caller consuming a
// ReviewItem of a known Kind never needs a type assertion to get fully
// typed data back out.
type ReviewItem struct {
	// ID is a deterministic identifier, stable across repeated Build calls
	// against identical source input and independent of Go map iteration
	// order. See each Kind's ID format, documented in build.go's ID-building
	// functions (buildClassificationID, buildOCRNumericID, etc.).
	ID string `json:"id"`
	// Kind identifies which review area this item belongs to.
	Kind Kind `json:"kind"`
	// Severity is this item's structured urgency signal — see Severity's
	// doc comment. Assigned entirely from structured Policy
	// thresholds/flags and upstream enum/boolean fields, never from
	// parsing Reason/Title text.
	Severity Severity `json:"severity"`
	// Required is true when this item must be resolved (accepted, overridden,
	// or otherwise decided) before the affected data should be trusted for
	// valuation. Distinct from Severity: a WARNING item can still be
	// Required (worth insisting on review) without being BLOCKING (gating
	// readiness) — see readiness.go for how Severity alone gates NOT_READY.
	Required bool `json:"required"`
	// Title is a short, stable, human-readable summary (e.g. "Unclassified
	// row: Misc Supplies").
	Title string `json:"title"`
	// Reason is a short human-readable explanation of why this item exists
	// (e.g. "classifier confidence 0.62 below threshold 0.90"). Display
	// only — never parsed by this package's own logic (see Severity's doc
	// comment).
	Reason string `json:"reason"`
	// SourceRowID is the originating row/check/adjustment/setting
	// identifier, when applicable (a financial.RawLineItem.ID,
	// ingestion.Row.ID, adjustments.Adjustment.ID, or similar). Empty for
	// dataset-wide items (e.g. some reconciliation checks).
	SourceRowID string `json:"source_row_id,omitempty"`
	// Provenance is a short, free-form description of where this item's
	// source data came from (e.g. "sheet-0-row-12", "page 2"), when
	// available beyond what SourceRowID/Period already convey.
	Provenance string `json:"provenance,omitempty"`
	// Period is the reporting period this item concerns, when applicable.
	Period financial.Period `json:"period,omitempty"`
	// CurrentValue is a short display string for the value currently in
	// effect before any decision (e.g. a proposed code, a parsed amount, a
	// detected period label). Empty when not applicable to this Kind.
	CurrentValue string `json:"current_value,omitempty"`
	// ProposedValue is a short display string for what upstream processing
	// proposed, when distinct from CurrentValue (most kinds set these
	// identically; kept as two fields for kinds where they could
	// legitimately differ in a future revision).
	ProposedValue string `json:"proposed_value,omitempty"`
	// RelatedCodes lists related warning/issue/check codes from upstream
	// packages (an ingestion.WarningCode, a reconciliation.CheckCode, etc.),
	// as plain strings, for traceability.
	RelatedCodes []string `json:"related_codes,omitempty"`
	// Status is this item's current resolution state. Always StatusPending
	// immediately after Build (see the package doc comment); Apply returns
	// a corrected view with Status updated per item.
	Status Status `json:"status"`

	// Exactly one of the following is non-nil, selected by Kind.
	Classification *ClassificationPayload `json:"classification,omitempty"`
	OCRText        *OCRTextPayload        `json:"ocr_text,omitempty"`
	OCRNumeric     *OCRNumericPayload     `json:"ocr_numeric,omitempty"`
	PeriodDetail   *PeriodPayload         `json:"period_detail,omitempty"`
	Structure      *StructurePayload      `json:"structure,omitempty"`
	Reconciliation *ReconciliationPayload `json:"reconciliation,omitempty"`
	Adjustment     *AdjustmentPayload     `json:"adjustment,omitempty"`
	Assumption     *AssumptionPayload     `json:"assumption,omitempty"`
}

// Summary aggregates a []ReviewItem into display-ready counts. Used both by
// Plan (immediately after Build, when every item is still unresolved) and
// by ApplyResult (after decisions have been applied) — the same type in
// both places is deliberate, so a caller/UI can render a "before" and
// "after" summary with identical code.
type Summary struct {
	// Total is len(Items).
	Total int `json:"total"`
	// Required is the count of items with Required == true.
	Required int `json:"required"`
	// Unresolved is the count of items whose Status.IsUnresolved() is true.
	Unresolved int `json:"unresolved"`
	// Blocking is the count of unresolved items with Severity ==
	// SeverityBlocking — the exact population readiness.go's NOT_READY rule
	// checks (see EvaluateReadiness).
	Blocking int `json:"blocking"`
	// Warnings is the count of unresolved items with Severity ==
	// SeverityWarning.
	Warnings int `json:"warnings"`
	// ByKind maps each Kind present in Items to its count. Marshals with
	// sorted string keys (encoding/json's standard map behavior), so JSON
	// output is deterministic despite being a map — see the repository
	// README's "Deterministic ordering guarantees" section, which already
	// establishes this pattern for settings.Resolution.Values/Sources and
	// metrics.Snapshot.Results.
	ByKind map[Kind]int `json:"by_kind,omitempty"`
}

// summarize computes a Summary over items, in one place, so Plan and
// ApplyResult can never drift in how they define "Required"/"Unresolved"/
// "Blocking"/"Warnings".
func summarize(items []ReviewItem) Summary {
	s := Summary{Total: len(items)}
	for _, it := range items {
		if it.Required {
			s.Required++
		}
		unresolved := it.Status.IsUnresolved()
		if unresolved {
			s.Unresolved++
			switch it.Severity {
			case SeverityBlocking:
				s.Blocking++
			case SeverityWarning:
				s.Warnings++
			}
		}
		if s.ByKind == nil {
			s.ByKind = make(map[Kind]int)
		}
		s.ByKind[it.Kind]++
	}
	return s
}

// Plan is the top-level output of Build: a deterministically ordered list
// of ReviewItem values plus a Summary over them, ready for a caller to
// present, collect Decisions against, and pass (along with those Decisions)
// to Apply.
type Plan struct {
	// Version echoes SchemaVersion.
	Version string `json:"version"`
	// Items is every ReviewItem Build produced, deterministically ordered:
	// primarily by Severity (BLOCKING, ERROR, WARNING, INFO), then by Kind,
	// then by SourceRowID, then by ID — see build.go's sortItems, which
	// implements this with sort.SliceStable over exactly these keys, never
	// left to append/map order.
	Items []ReviewItem `json:"items"`
	// Summary aggregates Items — see Summary's doc comment.
	Summary Summary `json:"summary"`
}
