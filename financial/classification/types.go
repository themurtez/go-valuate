// Package classification maps financial.RawLineItem values to
// financial.MappedLineItem values by proposing a canonical financial.Code
// for each row, deterministically.
//
// This package knows nothing about where its configuration (aliases, rules)
// comes from — no database, no accounts, no clients, no HTTP, no AI/LLM
// integration. Callers assemble a Config from whatever storage or process
// they like and pass it, along with raw rows, to Classify or ClassifyBatch.
// This keeps classification a pure function of its inputs, consistent with
// the rest of this module: see the root README for the full list of
// concerns this repository intentionally excludes.
//
// Classification here is entirely rule-based and deterministic: the same
// (row, config) pair always produces the same Result. It is designed so a
// future statistical or AI/LLM-based classifier could implement the same
// boundary (raw rows + config in, Result out) without any change to
// financial.Normalize or downstream valuation code — see the package README
// for more on that boundary.
package classification

import "github.com/themurtez/go-valuate/financial"

// Source identifies which stage of the classification pipeline produced a
// Result, in order of precedence from highest to lowest. Callers can use
// this to explain a classification to a human reviewer (e.g. "matched
// alias") or to decide how much to trust it.
type Source string

const (
	// SourceExplicit means the row's exact original label matched a
	// caller-supplied explicit mapping with no normalization applied.
	SourceExplicit Source = "explicit"
	// SourceAlias means the row's normalized label matched a caller-supplied
	// alias.
	SourceAlias Source = "alias"
	// SourceContextRule means a rule that inspected context beyond the
	// label alone (e.g. ParentLabel, StatementType) produced the match.
	SourceContextRule Source = "context_rule"
	// SourcePhraseRule means a rule matched based on a phrase or token
	// within the normalized label, without needing additional context.
	SourcePhraseRule Source = "phrase_rule"
	// SourceStructural means the row was recognized as structural (a
	// subtotal or total row) rather than an ordinary account, based on its
	// label and/or existing metadata.
	SourceStructural Source = "structural"
	// SourceUnknown means no stage of the pipeline could justify a mapping.
	SourceUnknown Source = "unknown"
)

// Confidence is a deterministic, heuristic strength score in [0, 1] for a
// proposed classification. It is NOT a statistical probability — nothing in
// this package is trained on data or measures real-world accuracy. It exists
// purely to give a consistent, orderable signal for how strongly the
// deterministic pipeline believes its own proposal, so callers can decide
// when to trust a mapping automatically versus route it to human review.
//
// Rough semantics used by the built-in pipeline (exact values may shift, but
// relative ordering will not):
//
//	1.00        explicit caller-supplied mapping for this exact row/label
//	~0.98       exact alias match on the normalized label
//	0.90 - 0.95 strong context-aware rule (label + parent/statement type)
//	0.70 - 0.89 weaker phrase/token rule (label alone, no context)
//	0            no rule matched: SourceUnknown
type Confidence float64

// Common confidence levels used by the built-in pipeline. Callers writing
// their own Rule implementations are free to return any value in [0, 1], but
// should stay consistent with the relative ordering these constants imply.
const (
	ConfidenceExplicit     Confidence = 1.00
	ConfidenceAlias        Confidence = 0.98
	ConfidenceStructural   Confidence = 0.97
	ConfidenceStrongRule   Confidence = 0.92
	ConfidenceWeakRule     Confidence = 0.75
	ConfidenceUnknown      Confidence = 0
	DefaultReviewThreshold Confidence = 0.90
)

// Candidate is one alternative (code, confidence) pairing considered during
// classification but not chosen as the primary result. Result.Alternatives
// carries these so a future review UI can offer a short list of runner-up
// codes instead of only the single winning proposal.
type Candidate struct {
	// Code is the alternative canonical taxonomy code.
	Code financial.Code `json:"code"`
	// Confidence is the heuristic strength of this alternative, using the
	// same scale as Result.Confidence.
	Confidence Confidence `json:"confidence"`
	// Source identifies which pipeline stage produced this alternative.
	Source Source `json:"source"`
	// Reason is a short, human-readable explanation of the alternative
	// match.
	Reason string `json:"reason,omitempty"`
}

// Result is the outcome of classifying a single financial.RawLineItem. It
// carries enough information for a human-review UI to show why a code was
// proposed, how confident the pipeline is, and what else it considered.
//
// A Result never mutates the RawLineItem it was produced from; RowID and
// Label are copied out for convenience and provenance.
type Result struct {
	// RowID is the source row's financial.RawLineItem.ID, copied for
	// provenance and lookups.
	RowID string `json:"row_id"`
	// Label is the source row's original, unmodified label.
	Label string `json:"label"`
	// Code is the proposed canonical taxonomy code. Empty when Status is
	// RowStatusIgnored, RowStatusSubtotal, or RowStatusTotal (no code is
	// proposed for structural rows) or when the row is SourceUnknown.
	Code financial.Code `json:"code,omitempty"`
	// Status is the row status this result recommends for the eventual
	// financial.MappedLineItem: normal, subtotal, total, or ignored.
	// Classify produces RowStatusIgnored only for a heading row
	// (raw.Kind == financial.RowKindHeading) — a heading is not a subtotal
	// or total, it simply carries no financial amount at all, so
	// RowStatusIgnored is the correct normalize-time directive for it (see
	// financial.Normalize, which already skips RowStatusIgnored rows). For
	// every other row, a caller wanting to ignore it does so after
	// inspecting the Result.
	Status financial.RowStatus `json:"status"`
	// Kind carries forward raw.Kind (the upstream structural read), for
	// provenance/display. Classification does not branch on this field for
	// any row other than the one it was copied from; see RawLineItem.Kind.
	Kind financial.RowKind `json:"kind,omitempty"`
	// Confidence is the heuristic strength of Code, in [0, 1]. See
	// Confidence's doc comment: this is not a statistical probability.
	Confidence Confidence `json:"confidence"`
	// Source identifies which pipeline stage produced Code.
	Source Source `json:"source"`
	// Reason is a short, human-readable explanation of the match (e.g.
	// "matched alias: advertising and promotion" or "parent-aware rule:
	// labor under Cost of Sales").
	Reason string `json:"reason,omitempty"`
	// MatchedRule is the name of the rule or alias key that produced this
	// result, if applicable. Useful for debugging and for a review UI to
	// deep-link to the rule/alias that fired.
	MatchedRule string `json:"matched_rule,omitempty"`
	// Alternatives lists other candidate codes the pipeline considered,
	// ordered from strongest to weakest. May be empty.
	Alternatives []Candidate `json:"alternatives,omitempty"`
	// ReviewRequired is true when this classification should be surfaced to
	// a human before being trusted automatically. Classify sets this based
	// on Config.ReviewThreshold; it is always true for SourceUnknown.
	ReviewRequired bool `json:"review_required"`
}

// IsUnknown reports whether the classifier could not justify any mapping for
// this row.
func (r Result) IsUnknown() bool {
	return r.Source == SourceUnknown
}

// ToMappedLineItem converts a Result plus its originating raw row into a
// financial.MappedLineItem, ready to pass to financial.Normalize. It copies
// Values from raw rather than aliasing the caller's map.
//
// Callers with an UNKNOWN result must decide for themselves how to handle
// it: ToMappedLineItem does not substitute a fallback code (see the package
// README's UNKNOWN section). If Code is empty and Status is
// RowStatusNormal, the resulting MappedLineItem will fail
// financial.Normalize's validation, which is intentional: an unresolved
// mapping should not silently aggregate as if it were classified.
func (r Result) ToMappedLineItem(raw financial.RawLineItem) financial.MappedLineItem {
	values := make(map[financial.Period]float64, len(raw.Values))
	for period, amount := range raw.Values {
		values[period] = amount
	}
	return financial.MappedLineItem{
		SourceID:      raw.ID,
		Label:         raw.Label,
		StatementType: raw.StatementType,
		Code:          r.Code,
		Status:        r.Status,
		Kind:          r.Kind,
		Values:        values,
	}
}
