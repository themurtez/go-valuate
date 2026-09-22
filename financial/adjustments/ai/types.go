// Package ai defines a provider-neutral boundary for optional AI-assisted
// normalization/add-back SUGGESTIONS over already-confirmed financial rows.
//
// This is a second, independent AI capability alongside
// financial/classification/ai — it answers a different question ("does this
// row look like it needs a normalization adjustment, and if so, which
// closed-set adjustments.Type?") and returns a different, source-bound
// contract. Nothing here knows about OpenAI, Anthropic, or any other
// provider's own types — see Suggester. A concrete provider lives in its own
// adapter subpackage (financial/adjustments/ai/openai) and depends on this
// package, never the other way around.
//
// Hard safety rules enforced throughout this package (see the repository
// README's "AI adjustment suggestions" section for the full rationale):
//
//   - AI never invents a financial amount: every accepted Suggestion's
//     Amount must equal the amount already present on the SourceRow it
//     references (see ValidateSuggestion). There is no free-floating amount
//     anywhere in this contract.
//   - AI never applies an adjustment. This package only ever produces
//     adjustments.Adjustment values with Included == false; only a human
//     review decision (via the review package) can flip that to true, and
//     only financial/adjustments.Apply changes normalized earnings.
//   - AI never invents a replacement salary, market rent, or any other
//     externally-benchmarked value — see Suggestion.RequiresUserInput.
//   - Every Suggestion is source-bound: it must name an existing
//     SourceRow.RowID/Period from the Request that produced it, or it is
//     rejected outright.
package ai

import (
	"context"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
)

// RequestSchemaVersion identifies the exact shape of Request this package
// sends to a Suggester and the exact shape of Response it expects back.
// Echoed on Provenance.RequestSchemaVersion so a persisted suggestion can
// always be traced back to the request/response contract that produced it.
const RequestSchemaVersion = "1.0.0"

// AllowedType is one closed-set adjustments.Type the model is permitted to
// choose, plus enough descriptive metadata to choose correctly. Built from
// adjustments.TypeMeta (see BuildAllowedTypes) rather than handing the
// provider bare enum strings.
type AllowedType struct {
	// Type is the canonical adjustment type (a member of
	// adjustments.AllTypes()).
	Type adjustments.Type `json:"type"`
	// Label is the human-readable display label (adjustments.TypeMeta.Label).
	Label string `json:"label"`
}

// BuildAllowedTypes converts adjustments.AllTypes() into the closed set a
// Suggester is told it may choose from.
func BuildAllowedTypes(metas []adjustments.TypeMeta) []AllowedType {
	out := make([]AllowedType, len(metas))
	for i, m := range metas {
		out[i] = AllowedType{Type: m.Type, Label: m.Label}
	}
	return out
}

// Direction states which way a suggested adjustment would move normalized
// earnings, mirroring adjustments.Effect under a name specific to this
// package's own wire contract (never re-exporting adjustments.Effect
// directly, per this repository's established pattern of every upstream-
// derived payload type owning a stable copy of its own shape).
type Direction string

const (
	// DirectionIncreaseEarnings means applying the suggested adjustment
	// would increase normalized earnings (adjustments.EffectIncrease).
	DirectionIncreaseEarnings Direction = "INCREASE_EARNINGS"
	// DirectionDecreaseEarnings means applying the suggested adjustment
	// would decrease normalized earnings (adjustments.EffectDecrease).
	DirectionDecreaseEarnings Direction = "DECREASE_EARNINGS"
)

// toEffect converts a Direction into the adjustments.Effect it corresponds
// to. ok is false for an unrecognized Direction.
func (d Direction) toEffect() (adjustments.Effect, bool) {
	switch d {
	case DirectionIncreaseEarnings:
		return adjustments.EffectIncrease, true
	case DirectionDecreaseEarnings:
		return adjustments.EffectDecrease, true
	default:
		return "", false
	}
}

// SourceRow is one explicitly-selected, already-confirmed financial row the
// caller supplies as adjustment-suggestion candidate/context. Every
// Suggestion the model returns must point back at a SourceRow's RowID+Period
// with an EXACTLY matching Amount — see ValidateSuggestion. This package
// never derives SourceRow itself from raw ingestion/classification output;
// the caller (typically after review.Apply has produced confirmed
// financial.MappedLineItem data) builds this list explicitly, which is also
// the enforcement point for "AI never sees structural or ambiguous-OCR
// rows" (see SelectCandidates).
type SourceRow struct {
	// RowID identifies the source row (financial.MappedLineItem.SourceID or
	// equivalent). Required.
	RowID string `json:"row_id"`
	// Period is the reporting period this row's Amount applies to. Required.
	Period financial.Period `json:"period"`
	// Label is the row's label exactly as it appeared in the source.
	Label string `json:"label"`
	// ParentLabel is the row's enclosing section label, if any.
	ParentLabel string `json:"parent_label,omitempty"`
	// Code is the row's canonical classification, when known
	// (financial.MappedLineItem.Code). May be empty for a caller-selected
	// row that has no settled classification yet — candidate selection by
	// code (see CandidateRules) simply never matches such a row.
	Code financial.Code `json:"code,omitempty"`
	// StatementType is the row's originating statement.
	StatementType financial.StatementType `json:"statement_type,omitempty"`
	// Amount is this row's reported value for Period — the ONLY amount a
	// Suggestion referencing this row may use (see ValidateSuggestion).
	// Required to be finite.
	Amount float64 `json:"amount"`
	// RowKind is the row's structural role, when known
	// (financial.MappedLineItem.Kind / financial.RowKind). A row with a
	// non-empty structural kind (heading/subtotal/total) is never eligible
	// for a suggestion — see isStructuralSourceRow.
	RowKind financial.RowKind `json:"row_kind,omitempty"`
	// AmbiguousOCR is true when this row's Amount was reconstructed from
	// low-confidence or heuristically-corrected OCR (or did not cleanly
	// parse at all) and has not yet been confirmed through review — see
	// review.OCRNumericPayload.Ambiguous. A row flagged here is never
	// eligible for a suggestion (see ValidateSuggestion): AI must not
	// normalize a number nobody has confirmed is even correct.
	AmbiguousOCR bool `json:"ambiguous_ocr,omitempty"`
}

// ContextRow is minimal, non-identifying context about a row NEAR a
// candidate row, supplied only to help the model disambiguate — mirrors
// financial/classification/ai.ContextRow, but deliberately WITHOUT an
// amount: nearby-row context is for label/section disambiguation only, never
// an extra number the model could confuse with the candidate's own Amount.
type ContextRow struct {
	Label       string `json:"label"`
	ParentLabel string `json:"parent_label,omitempty"`
}

// MultiYearValue is one other period's reported amount for the SAME
// source/account as a candidate row, supplied only when the caller judges it
// useful for the model to see a trend (e.g. "this expense tripled last
// year") — section 7's explicit allowance. Never used by ValidateSuggestion
// as an acceptable substitute for the candidate row's own Period+Amount.
type MultiYearValue struct {
	Period financial.Period `json:"period"`
	Amount float64          `json:"amount"`
}

// Request is the minimal, deterministic, source-bound payload sent to a
// Suggester for one bounded candidate set. Unlike
// financial/classification/ai.Request, amounts ARE included here — see the
// package doc comment's privacy note: an adjustment suggestion is
// inescapably about a specific dollar figure, so omitting it would make the
// capability useless; what stays excluded is everything NOT needed to judge
// whether a row is a normalization candidate (customer/user identity, bank
// details, tax IDs, unrelated rows, full statements).
type Request struct {
	// Candidates is the bounded set of rows the model may suggest
	// adjustments for. Every Suggestion returned must reference a RowID
	// present here (see ValidateSuggestion). Bounded by CandidatePolicy/
	// BatchPolicy — never "every row in the dataset."
	Candidates []SourceRow `json:"candidates"`
	// AllowedTypes is the CLOSED SET of adjustment types the model may
	// choose from. See BuildAllowedTypes.
	AllowedTypes []AllowedType `json:"allowed_types"`
	// IndustryContext is free-form business-context text the CALLER
	// explicitly supplies (e.g. "residential HVAC contractor"). This package
	// never infers or looks up industry information itself.
	IndustryContext string `json:"industry_context,omitempty"`
	// ContextRows optionally maps a candidate's RowID to a caller-bounded
	// window of nearby rows for disambiguation. Never populated
	// automatically.
	ContextRows map[string][]ContextRow `json:"context_rows,omitempty"`
	// MultiYearValues optionally maps a candidate's RowID to other periods'
	// values for the same source/account, when the caller judges the trend
	// useful (section 7). Never populated automatically.
	MultiYearValues map[string][]MultiYearValue `json:"multi_year_values,omitempty"`
}

// Suggestion is one model-proposed adjustment for one candidate row. Every
// field is validated by ValidateSuggestion before this package trusts it —
// see validate.go.
type Suggestion struct {
	// SourceRowID must match a Request.Candidates[i].RowID exactly. Required.
	SourceRowID string `json:"source_row_id"`
	// Period must match that candidate row's own Period exactly. Required.
	Period financial.Period `json:"period"`
	// AdjustmentType is the model's chosen closed-set type. Must be a member
	// of the Request's AllowedTypes. Required.
	AdjustmentType adjustments.Type `json:"adjustment_type"`
	// Amount must EXACTLY equal the referenced candidate row's own Amount
	// under MVP rules (see the package doc comment) — a bare "close enough"
	// number is rejected, never silently repaired. Required.
	Amount float64 `json:"amount"`
	// Direction states which way this suggestion would move normalized
	// earnings. Required, and must be compatible with AdjustmentType's
	// deterministic Effect semantics (see ValidateSuggestion).
	Direction Direction `json:"direction"`
	// Reason is the model's short natural-language explanation. Display
	// only, never parsed by this package's own logic.
	Reason string `json:"reason,omitempty"`
	// Confidence is a provider-supplied heuristic strength in [0, 1], if the
	// provider returns one. Never a calibrated statistical probability — see
	// financial/classification/ai.Response.RawConfidence's identical
	// warning.
	Confidence *float64 `json:"confidence,omitempty"`
	// RequiresUserInput is true when this suggestion identifies a
	// normalization need but the deterministic adjustment type requires a
	// caller/accountant-supplied benchmark value this package forbids AI
	// from inventing (a replacement market salary for
	// adjustments.TypeOwnerCompensationNormalization, a market rent for
	// adjustments.TypeRelatedPartyRentAdjustment) — see section 5. When
	// true, Amount still carries the CURRENT recorded amount being flagged
	// (never a proposed replacement), Direction is advisory only, and the
	// resulting adjustments.Adjustment is built with Included == false and
	// a zero/placeholder normalization amount pending explicit user input
	// (see ToAdjustments).
	RequiresUserInput bool `json:"requires_user_input,omitempty"`
}

// Response is what a Suggester returns for one Request.
type Response struct {
	// Suggestions is every proposed adjustment for this request's
	// candidates. May be empty (the model found nothing worth suggesting) —
	// an empty Response is not an error.
	Suggestions []Suggestion `json:"suggestions"`
	// Provider identifies which adapter produced this Response (e.g.
	// "openai"). Set by the Suggester implementation.
	Provider string `json:"provider,omitempty"`
	// Model identifies the specific model used, when the adapter supplies
	// one.
	Model string `json:"model,omitempty"`
	// AdapterVersion identifies the provider adapter package's own version.
	AdapterVersion string `json:"adapter_version,omitempty"`
}

// Suggester is the provider-neutral AI adjustment-suggestion boundary.
// Exactly one Request in, exactly one Response (or an error) out for a whole
// bounded candidate batch — unlike
// financial/classification/ai.Classifier (one row per call), a Suggester
// call is naturally multi-row because normalization candidates are
// meaningfully compared against each other and against the closed adjustment
// taxonomy in one pass (see section 15's batching requirement).
//
// Implementations must not depend on anything outside ctx/req to produce a
// Response — no hidden global client state, no reading files/env vars inside
// Suggest itself.
type Suggester interface {
	// Suggest proposes adjustments for req, or returns a non-nil error if
	// the provider call itself failed. Suggest must respect ctx
	// cancellation/deadlines.
	Suggest(ctx context.Context, req Request) (Response, error)
}
