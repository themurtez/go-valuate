package ai

import (
	"fmt"

	"github.com/themurtez/go-valuate/financial/classification"
)

// buildAIResult converts a validated, accepted AI Response into a
// classification.Result that slots into the exact same shape/handling as
// every deterministic Result — see classification.SourceAI's doc comment.
// Only called from resolveOutcome's one case where AI is actually allowed
// to become the row's usable result (deterministic was UNKNOWN/no
// competing code, AI proposed a validated non-UNKNOWN closed-set code, row
// is not structural).
func buildAIResult(det classification.Result, resp Response) classification.Result {
	alts := make([]classification.Candidate, 0, len(resp.Alternatives))
	for _, code := range resp.Alternatives {
		if code == CodeUnknown {
			continue
		}
		alts = append(alts, classification.Candidate{
			Code:       code,
			Confidence: aiCandidateConfidence(resp),
			Source:     classification.SourceAI,
			Reason:     "AI-proposed alternative",
		})
	}

	reason := resp.Reason
	if reason == "" {
		reason = "AI fallback classification"
	}

	return classification.Result{
		RowID:  det.RowID,
		Label:  det.Label,
		Code:   resp.Code,
		Status: det.Status,
		Kind:   det.Kind,
		// Confidence deliberately stays a classification.Confidence value
		// (this package's own deterministic heuristic scale) even though it
		// was populated from an AI RawConfidence — see aiCandidateConfidence
		// and section 22 of the task brief: this is NOT a blended
		// probability, it is the single explicit place a raw model
		// confidence is reinterpreted on this scale, purely so existing
		// Confidence-consuming code (review.Build's threshold checks,
		// display sorting) keeps working unchanged for an AI-sourced
		// Result. The ORIGINAL unconverted model confidence remains
		// separately available on Provenance.ModelConfidence for anything
		// that needs the true provider-reported number.
		Confidence:     aiCandidateConfidence(resp),
		Source:         classification.SourceAI,
		Reason:         reason,
		MatchedRule:    "ai_fallback",
		Alternatives:   alts,
		ReviewRequired: true,
	}
}

// aiCandidateConfidence maps an AI Response's optional RawConfidence onto
// classification.Confidence's [0, 1] scale, defaulting to
// classification.ConfidenceWeakRule when the provider supplied none — a
// deliberately conservative default (the same floor as the deterministic
// pipeline's weakest phrase-rule match) rather than assuming high
// confidence for a provider that did not report one.
func aiCandidateConfidence(resp Response) classification.Confidence {
	if resp.RawConfidence != nil {
		return classification.Confidence(*resp.RawConfidence)
	}
	return classification.ConfidenceWeakRule
}

// DescribeDisagreement renders a Disagreement as a short, stable,
// human-readable two-line string suitable for a future review UI's default
// display — see section 21 of the task brief's exact example format. Not
// used internally by this package; provided purely as a convenience so a
// caller does not need to re-derive this formatting.
func DescribeDisagreement(d Disagreement) string {
	return fmt.Sprintf("Rule classifier: %s\nAI fallback: %s", d.DeterministicCode, d.AICode)
}
