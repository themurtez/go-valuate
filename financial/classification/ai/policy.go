package ai

import "time"

// FallbackMode is the caller-controlled trigger policy deciding WHEN
// ClassifyWithFallback consults a Classifier at all. The deterministic
// pipeline (financial/classification.Classify) always runs first and in
// full, regardless of FallbackMode — this only controls whether its result
// may additionally be handed to AI.
type FallbackMode string

const (
	// AIDisabled never calls the Classifier. This is the zero value and the
	// Policy default (see DefaultPolicy) — AI is opt-in, never on by
	// default.
	AIDisabled FallbackMode = "AI_DISABLED"
	// AIUnknownOnly calls the Classifier only for rows where the
	// deterministic result is SourceUnknown (classification.Result.IsUnknown()).
	// A row the deterministic pipeline already classified, at any
	// confidence, is left alone.
	AIUnknownOnly FallbackMode = "AI_UNKNOWN_ONLY"
	// AIBelowConfidence calls the Classifier for SourceUnknown rows, plus
	// any row whose deterministic Confidence is below
	// Policy.ConfidenceThreshold.
	AIBelowConfidence FallbackMode = "AI_BELOW_CONFIDENCE"
	// AIForce calls the Classifier for every non-structural row regardless
	// of the deterministic result (including an already-confident
	// explicit/alias/context-rule match), so a caller can compare AI
	// against the deterministic pipeline (see Disagreement). Structural-row
	// safety (below) still applies even in this mode.
	AIForce FallbackMode = "AI_FORCE"
)

// DefaultConfidenceThreshold is Policy.ConfidenceThreshold's default when
// unset and Mode == AIBelowConfidence, matching
// classification.DefaultReviewThreshold so "below confidence" means the
// same thing here as it already does for human review triage.
const DefaultConfidenceThreshold = 0.90

// DefaultMaxContextRows is Policy.MaxContextRows's default when zero.
const DefaultMaxContextRows = 4

// DefaultTimeout is Policy.Timeout's default when zero.
const DefaultTimeout = 20 * time.Second

// Policy bundles every caller-controlled AI-fallback decision:
// WHEN to call AI (Mode/ConfidenceThreshold), safety limits
// (AllowStructuralRows), and cost controls (MaxAIRows/MaxBatchSize/
// MaxContextRows/Timeout). A Policy is plain data, exactly like
// review.Policy and classification.Config — no I/O, no hidden defaults
// beyond what DefaultPolicy documents.
type Policy struct {
	// Mode selects the trigger rule — see FallbackMode's constants. Zero
	// value is AIDisabled.
	Mode FallbackMode
	// ConfidenceThreshold is the deterministic-confidence cutoff used by
	// AIBelowConfidence (see that constant's doc comment). Ignored for
	// every other Mode. Defaults to DefaultConfidenceThreshold when zero
	// and Mode == AIBelowConfidence.
	ConfidenceThreshold float64
	// AllowStructuralRows, when true, permits ClassifyWithFallback to send
	// a Request for a row whose RowKind is HEADING/SUBTOTAL/TOTAL — for
	// diagnostic use only (e.g. evaluating a model's structural-row
	// judgement). Even when true, the returned code can never cause that
	// row to normalize as if it were an ordinary account — see
	// StructuralRowResponse. Defaults to false: AI fallback normally never
	// runs at all for a structural row (see IssueStructuralRowSkipped).
	AllowStructuralRows bool
	// MaxAIRows caps how many rows in one ClassifyBatchWithFallback call
	// may be sent to AI in total. Zero means unlimited. Once reached,
	// every remaining eligible row is left on its deterministic/UNKNOWN
	// result and reported via IssueBudgetExceeded rather than silently
	// exceeding the caller's configured budget — see the README's
	// cost-control section.
	MaxAIRows int
	// MaxBatchSize caps how many Requests ClassifyWithFallback issues
	// concurrently/per logical batch when the Classifier supports batching
	// (see BatchClassifier). Zero means DefaultMaxBatchSize. Ignored for a
	// plain Classifier (no batching contract at that level — see
	// Classifier's own doc comment).
	MaxBatchSize int
	// MaxContextRows caps how many ContextRow entries a single Request may
	// carry. Zero means DefaultMaxContextRows. If a caller supplies more
	// context rows than this for one row, the excess is dropped (not sent)
	// rather than rejecting the row outright — a caller wanting a hard
	// failure on an oversized context window should check this before
	// calling.
	MaxContextRows int
	// Timeout bounds a single Classify call when the caller's ctx does not
	// already carry a tighter deadline. Zero means DefaultTimeout. Never
	// extends a deadline the caller's own ctx already sets — see
	// ClassifyWithFallback.
	Timeout time.Duration
	// Strict, when true, makes ClassifyBatchWithFallback return a non-nil
	// error and abandon the whole batch on the FIRST AI failure (provider
	// error, timeout, invalid response), instead of the default behavior of
	// leaving that one row on its deterministic/UNKNOWN result and
	// continuing with the rest of the batch (see the README's
	// fallback-failure-behavior section: "AI failure must never make
	// deterministic classification worse" is the default; Strict is the
	// opt-in exception for a caller that would rather fail loudly).
	Strict bool
}

// DefaultPolicy returns the zero-risk default: AI fallback disabled
// entirely. Every AI-fallback capability in this package is strictly
// opt-in — see the README's "AI is optional" statement.
func DefaultPolicy() Policy {
	return Policy{Mode: AIDisabled}
}

func (p Policy) confidenceThreshold() float64 {
	if p.ConfidenceThreshold == 0 {
		return DefaultConfidenceThreshold
	}
	return p.ConfidenceThreshold
}

func (p Policy) maxContextRows() int {
	if p.MaxContextRows == 0 {
		return DefaultMaxContextRows
	}
	if p.MaxContextRows < 0 {
		return 0
	}
	return p.MaxContextRows
}

func (p Policy) timeout() time.Duration {
	if p.Timeout == 0 {
		return DefaultTimeout
	}
	return p.Timeout
}
