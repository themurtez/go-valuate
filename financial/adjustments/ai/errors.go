package ai

// IssueSeverity distinguishes a problem that leaves a suggestion rejected
// (SeverityError) from one worth surfacing but not fatal
// (SeverityWarning) — mirrors financial/classification/ai.IssueSeverity's
// identical two-severity model, kept as this package's own type per the
// repository's established error-taxonomy pattern (a provider/validation
// failure in this domain is not the same problem as a classification-AI
// failure or a review-decision validation problem).
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of adjustment-suggestion
// problem.
type IssueCode string

const (
	// IssueProviderUnavailable means the Suggester was nil, or a convenience
	// constructor could not build one — no provider call could even be
	// attempted.
	IssueProviderUnavailable IssueCode = "AI_PROVIDER_UNAVAILABLE"
	// IssueTimeout means the provider call did not complete before ctx's
	// deadline or Policy.Timeout elapsed.
	IssueTimeout IssueCode = "AI_TIMEOUT"
	// IssueProviderError means Suggester.Suggest returned a non-nil error
	// for a reason other than timeout/rate-limit.
	IssueProviderError IssueCode = "AI_PROVIDER_ERROR"
	// IssueRateLimited means the provider reported a rate-limit condition.
	IssueRateLimited IssueCode = "AI_RATE_LIMITED"
	// IssueMalformedResponse means the provider's Response itself could not
	// be trusted structurally (e.g. a non-finite Confidence) independent of
	// any one suggestion's own content.
	IssueMalformedResponse IssueCode = "AI_MALFORMED_RESPONSE"

	// IssueUnknownSourceRow means Suggestion.SourceRowID does not match any
	// candidate row in the Request that produced it.
	IssueUnknownSourceRow IssueCode = "UNKNOWN_SOURCE_ROW"
	// IssueWrongPeriod means Suggestion.Period does not match the
	// referenced SourceRow's own Period.
	IssueWrongPeriod IssueCode = "WRONG_PERIOD"
	// IssueInventedAmount means Suggestion.Amount does not exactly equal the
	// referenced SourceRow's own Amount — the AI proposed a dollar figure
	// that does not exist in the source data. Always rejected; never
	// silently repaired (section 9's explicit requirement).
	IssueInventedAmount IssueCode = "INVENTED_AMOUNT"
	// IssueInvalidAdjustmentType means Suggestion.AdjustmentType is not a
	// member of the Request's AllowedTypes closed set.
	IssueInvalidAdjustmentType IssueCode = "INVALID_ADJUSTMENT_TYPE"
	// IssueIncompatibleDirection means Suggestion.Direction conflicts with
	// AdjustmentType's own deterministic adjustments.Effect semantics (e.g.
	// suggesting DECREASE_EARNINGS for a type whose only legal effect is
	// EffectIncrease).
	IssueIncompatibleDirection IssueCode = "INCOMPATIBLE_DIRECTION"
	// IssueStructuralSourceRow means the referenced SourceRow is a
	// heading/subtotal/total row — never a valid adjustment target.
	IssueStructuralSourceRow IssueCode = "STRUCTURAL_SOURCE_ROW"
	// IssueAmbiguousSourceAmount means the referenced SourceRow is flagged
	// AmbiguousOCR — its own numeric value has not been confirmed, so no
	// adjustment may be suggested against it yet.
	IssueAmbiguousSourceAmount IssueCode = "AMBIGUOUS_SOURCE_AMOUNT"
	// IssueDuplicateSuggestion means another suggestion in the same Response
	// already targets the same (SourceRowID, Period, AdjustmentType) —
	// every subsequent duplicate is rejected, the first occurrence is kept.
	IssueDuplicateSuggestion IssueCode = "DUPLICATE_SUGGESTION"
	// IssueNonFiniteAmount means Suggestion.Amount (or Confidence) is NaN or
	// +/-Inf.
	IssueNonFiniteAmount IssueCode = "NON_FINITE_AMOUNT"
	// IssueMissingUserInput is an INFO-level (warning-severity) note
	// attached to a valid RequiresUserInput suggestion, so a caller can
	// distinguish "accepted but still needs a benchmark value" from a fully
	// resolved suggestion without inspecting every field.
	IssueMissingUserInput IssueCode = "REQUIRES_USER_INPUT"
)

// Issue is a single suggestion-related finding, mirroring
// financial/classification/ai.Issue's shape.
type Issue struct {
	// SourceRowID identifies the candidate row this issue concerns, when
	// applicable. Empty for a whole-Response-level issue (e.g.
	// IssueProviderError).
	SourceRowID string        `json:"source_row_id,omitempty"`
	Code        IssueCode     `json:"code"`
	Severity    IssueSeverity `json:"severity"`
	Message     string        `json:"message"`
}

// RateLimitError is an optional interface a Suggester's returned error may
// satisfy (via errors.As) to let Suggest report IssueRateLimited instead of
// the generic IssueProviderError.
type RateLimitError interface {
	error
	RateLimited() bool
}

// TimeoutError is an optional interface a Suggester's returned error may
// satisfy (via errors.As) to let Suggest report IssueTimeout instead of the
// generic IssueProviderError.
type TimeoutError interface {
	error
	Timeout() bool
}
