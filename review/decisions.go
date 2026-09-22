package review

import "github.com/themurtez/go-valuate/financial"

// Action identifies what a caller decided to do about a single ReviewItem.
// A stable, string-based enum, like every other enum in this package.
type Action string

const (
	// ActionAccept accepts the item's current/proposed value as-is (e.g.
	// accept a classification mapping, accept a parsed OCR numeric value,
	// accept a structural RowKind, accept a detected period).
	ActionAccept Action = "ACCEPT"
	// ActionOverride replaces the item's proposed value with a
	// caller-supplied one (e.g. a different canonical code, a corrected
	// numeric amount, a different RowKind, a different period, a modified
	// adjustment amount, a different assumption value). Always requires a
	// typed payload.
	ActionOverride Action = "OVERRIDE"
	// ActionIgnore excludes the item's underlying row/value from downstream
	// processing without treating it as an error (e.g. ignore a row
	// entirely, exclude an adjustment).
	ActionIgnore Action = "IGNORE"
	// ActionReject marks the item's underlying value as unavailable/invalid
	// (e.g. an OCR numeric value that cannot be trusted at all — distinct
	// from ActionIgnore: a rejected numeric row is flagged unavailable
	// rather than silently dropped).
	ActionReject Action = "REJECT"
	// ActionConfirm confirms an item that exists purely for
	// acknowledgment (e.g. confirming an adjustment already marked
	// Included, or confirming a valuation assumption's currently-resolved
	// value) without changing or overriding anything.
	ActionConfirm Action = "CONFIRM"
)

// ClassificationDecision carries the payload for a Decision against a
// KindClassification item.
type ClassificationDecision struct {
	// Code is the canonical financial.Code to use. Required (and must pass
	// financial.IsValidCode) when Action == ActionOverride; ignored for
	// every other Action.
	Code financial.Code `json:"code,omitempty"`
}

// OCRNumericDecision carries the payload for a Decision against a
// KindOCRNumeric item.
type OCRNumericDecision struct {
	// Amount is the corrected numeric value to use. Required (and must be
	// finite) when Action == ActionOverride; ignored for every other
	// Action.
	Amount float64 `json:"amount,omitempty"`
}

// StructureDecision carries the payload for a Decision against a
// KindStructure item.
type StructureDecision struct {
	// RowKind is the financial.RowKind to use. Required (and must be one of
	// the four known RowKind constants) when Action == ActionOverride;
	// ignored for every other Action.
	RowKind financial.RowKind `json:"row_kind,omitempty"`
}

// PeriodDecision carries the payload for a Decision against a KindPeriod
// item.
type PeriodDecision struct {
	// Period is the corrected canonical period to use. Required (and must
	// be non-empty) when Action == ActionOverride; ignored for every other
	// Action.
	Period financial.Period `json:"period,omitempty"`
}

// AdjustmentDecision carries the payload for a Decision against a
// KindAdjustment item.
type AdjustmentDecision struct {
	// Included sets adjustments.Adjustment.Included when Action is
	// ActionAccept, ActionOverride, or ActionConfirm (true = include,
	// false = exclude) — used together with Action to express both
	// "include" and "exclude" as ActionOverride with a different Included
	// value, since inclusion is the only thing this decision can change
	// safely without re-deriving the whole Adjustment.
	Included bool `json:"included"`
	// Amount, when Action == ActionOverride and NewAmount is true, replaces
	// the adjustment's Amount. Must be finite.
	Amount float64 `json:"amount,omitempty"`
	// NewAmount is true when Amount should replace the adjustment's
	// original amount. false means Amount is ignored and only Included is
	// applied (a pure include/exclude decision) — kept as an explicit flag
	// rather than inferring "Amount == 0 means no change," since 0 is a
	// legitimate override value.
	NewAmount bool `json:"new_amount,omitempty"`
	// Reason, when non-empty, replaces the adjustment's Reason.
	Reason string `json:"reason,omitempty"`
}

// AssumptionDecision carries the payload for a Decision against a
// KindValuationAssumption item.
type AssumptionDecision struct {
	// Value is the overriding value for this valuation input, required when
	// Action == ActionOverride. Carried as `any` because the assumption it
	// overrides may itself be numeric or boolean (see
	// AssumptionPayload.CurrentValue's identical reasoning) — this package
	// validates it is one of those two supported kinds and, if numeric,
	// finite (see decision validation in apply.go).
	Value any `json:"value,omitempty"`
}

// Decision is a caller's resolution for a single ReviewItem, identified by
// ItemID. Exactly one of the typed payload fields is populated, matching
// the targeted ReviewItem's Kind — mirroring ReviewItem's own typed-payload
// design (see types.go) rather than a generic map[string]any value bag.
//
// A Decision with no payload field populated is valid for Action values
// that need none (ActionAccept, ActionIgnore, ActionReject, ActionConfirm
// against most Kinds) — see apply.go's per-Kind validation for exactly
// which (Kind, Action) pairs require a payload.
type Decision struct {
	// ItemID is the ReviewItem.ID this decision targets. Required; must
	// match an ID present in the Plan passed to Apply.
	ItemID string `json:"item_id"`
	// Action is what the caller decided to do. Required.
	Action Action `json:"action"`

	// Exactly one of the following may be non-nil, selected by the
	// targeted ReviewItem's Kind.
	Classification *ClassificationDecision `json:"classification,omitempty"`
	OCRNumeric     *OCRNumericDecision     `json:"ocr_numeric,omitempty"`
	Structure      *StructureDecision      `json:"structure,omitempty"`
	PeriodOverride *PeriodDecision         `json:"period_override,omitempty"`
	Adjustment     *AdjustmentDecision     `json:"adjustment,omitempty"`
	Assumption     *AssumptionDecision     `json:"assumption,omitempty"`
}
