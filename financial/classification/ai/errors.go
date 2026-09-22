package ai

// IssueSeverity distinguishes an AI-fallback problem that leaves the row at
// UNKNOWN (SeverityError — the provider call or its response could not be
// trusted at all) from one worth surfacing but not fatal to this row's
// classification (SeverityWarning) — mirroring review.IssueSeverity and
// adjustments.IssueSeverity's identical two-severity model, renamed here
// only to avoid collisions, per this repository's established
// error-taxonomy pattern (see the root README's "Error taxonomy" section).
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of AI-fallback problem,
// analogous to review.IssueCode and adjustments.IssueCode. This package
// defines its own system rather than reusing either: an AI provider-call
// failure (timeout, rate limit, invalid response) is a fundamentally
// different problem domain from a review-decision validation problem or an
// adjustment-set consistency problem, and forcing one type to serve all
// three would leak AI-specific concerns into packages that must remain
// buildable/testable without ever importing this one.
type IssueCode string

const (
	// IssueAIDisabled means Policy.Mode was AIDisabled — no provider call
	// was attempted. Never itself a reason a row stays unresolved; carried
	// only so provenance can record why no AI attempt happened.
	IssueAIDisabled IssueCode = "AI_DISABLED"
	// IssueProviderUnavailable means the Classifier was nil, or a
	// convenience constructor could not build one (e.g. missing
	// configuration) — no provider call could even be attempted.
	IssueProviderUnavailable IssueCode = "AI_PROVIDER_UNAVAILABLE"
	// IssueTimeout means the provider call did not complete before ctx's
	// deadline or Policy.Timeout elapsed.
	IssueTimeout IssueCode = "AI_TIMEOUT"
	// IssueProviderError means Classifier.Classify returned a non-nil error
	// for a reason other than timeout/rate-limit (a transport failure, an
	// authentication failure, an unexpected provider-side error, ...). The
	// underlying error is preserved for diagnostics — see
	// FallbackOutcome.Err.
	IssueProviderError IssueCode = "AI_PROVIDER_ERROR"
	// IssueInvalidResponse means the provider responded, but the Response
	// failed structural validation for a reason other than an out-of-set
	// code (missing Code entirely, a RawConfidence outside [0, 1] or
	// non-finite, ...). See ValidateResponse.
	IssueInvalidResponse IssueCode = "AI_INVALID_RESPONSE"
	// IssueInvalidCode means Response.Code (or an entry in
	// Response.Alternatives) was not CodeUnknown and not a member of the
	// Request's AllowedCodes — the model proposed a code outside the closed
	// set. Per the task's closed-set requirement, this always downgrades
	// the row to UNKNOWN; it never lets an invented code through.
	IssueInvalidCode IssueCode = "AI_INVALID_CODE"
	// IssueEmptyResponse means the provider returned a zero-value Response
	// with no Code set at all.
	IssueEmptyResponse IssueCode = "AI_EMPTY_RESPONSE"
	// IssueRateLimited means the provider reported a rate-limit condition.
	// Adapters that can distinguish this from a generic provider error
	// should return an error satisfying RateLimitError (see errors.go's
	// classifyProviderError) so ClassifyWithFallback can report this code
	// specifically rather than the generic IssueProviderError.
	IssueRateLimited IssueCode = "AI_RATE_LIMITED"
	// IssueContextTooLarge means Policy.MaxContextRows (or a provider-side
	// equivalent) was exceeded and the request was not sent.
	IssueContextTooLarge IssueCode = "AI_CONTEXT_TOO_LARGE"
	// IssueBudgetExceeded means a cost-control limit (Policy.MaxAIRows) was
	// reached before this row could be sent to AI — the row is left on its
	// deterministic/UNKNOWN result rather than silently exceeding the
	// caller's configured budget. See Policy.MaxAIRows.
	IssueBudgetExceeded IssueCode = "AI_BUDGET_EXCEEDED"
	// IssueStructuralRowSkipped is an INFO-level (warning-severity) note
	// that AI fallback did not run for a structural row (HEADING/SUBTOTAL/
	// TOTAL) because Policy did not explicitly force it — see
	// ClassifyWithFallback's structural-row handling.
	IssueStructuralRowSkipped IssueCode = "AI_STRUCTURAL_ROW_SKIPPED"
)

// Issue is a single AI-fallback finding for one row, mirroring
// review.Issue's shape.
type Issue struct {
	// RowID identifies the financial.RawLineItem.ID this issue concerns.
	RowID    string        `json:"row_id,omitempty"`
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// RateLimitError is an optional interface a Classifier's returned error may
// satisfy (via errors.As) to let ClassifyWithFallback report
// IssueRateLimited instead of the generic IssueProviderError. Adapters are
// not required to implement this; a plain error is always treated as
// IssueProviderError.
type RateLimitError interface {
	error
	RateLimited() bool
}

// TimeoutError is an optional interface a Classifier's returned error may
// satisfy (via errors.As) to let ClassifyWithFallback report IssueTimeout
// instead of the generic IssueProviderError, for adapters whose underlying
// transport does not return context.DeadlineExceeded directly (e.g. a
// provider SDK that wraps timeouts in its own error type).
type TimeoutError interface {
	error
	Timeout() bool
}
