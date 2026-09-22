package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/themurtez/go-valuate/financial/adjustments"
)

// OrchestrationVersion identifies the exact batching/validation/provenance
// rules SuggestAdjustments implements. See the repository README's
// versioning-strategy section; bump only when this decision logic changes
// in a way that could make a historical Outcome not reproduce identically
// under new code.
const OrchestrationVersion = "1.0.0"

// Provenance is the complete audit record for one Suggest attempt over a
// bounded candidate batch, whether or not it ultimately produced any usable
// suggestions — see section 10 of the task brief for exactly which fields
// this satisfies.
type Provenance struct {
	// Provider is the adapter-supplied provider name (e.g. "openai").
	Provider string `json:"provider,omitempty"`
	// Model is the specific model identifier the provider used, when
	// supplied.
	Model string `json:"model,omitempty"`
	// AdapterVersion identifies the provider adapter package's own version.
	AdapterVersion string `json:"adapter_version,omitempty"`
	// RequestSchemaVersion echoes RequestSchemaVersion at the time this
	// Provenance was produced.
	RequestSchemaVersion string `json:"request_schema_version"`
	// OrchestrationVersion echoes OrchestrationVersion at the time this
	// Provenance was produced.
	OrchestrationVersion string `json:"orchestration_version"`
	// CandidateRowCount is len(Request.Candidates) for the batch this
	// Provenance concerns.
	CandidateRowCount int `json:"candidate_row_count"`
	// RawSuggestionCount is how many suggestions the provider returned
	// before validation.
	RawSuggestionCount int `json:"raw_suggestion_count"`
	// ValidSuggestionCount is how many of those passed ValidateSuggestions
	// (including RequiresUserInput ones, which are valid-but-flagged).
	ValidSuggestionCount int `json:"valid_suggestion_count"`
	// RejectedCount is how many were rejected by validation.
	RejectedCount int `json:"rejected_count"`
}

// Outcome is the result of SuggestAdjustments for one caller-supplied
// candidate set.
type Outcome struct {
	// Valid is every suggestion that passed ValidateSuggestions, in the
	// provider's own returned order (including RequiresUserInput ones —
	// check each entry's Suggestion.RequiresUserInput).
	Valid []Suggestion
	// Rejected is every suggestion the provider returned that failed
	// validation, paired with why.
	Rejected []RejectedSuggestion
	// Provenance is the full audit record for this attempt, nil when AI was
	// never attempted (provider unavailable, disabled) or the provider call
	// itself failed (see Issues/Err in that case).
	Provenance *Provenance `json:"provenance,omitempty"`
	// Issues carries every non-fatal finding (e.g. IssueMissingUserInput for
	// each RequiresUserInput suggestion) plus, when the provider call itself
	// failed or was never attempted, the fatal Issue explaining why.
	Issues []Issue
	// Err is the raw, wrapped error Suggester.Suggest returned, if the
	// provider call itself failed. Nil whenever the provider call succeeded
	// (even if every suggestion it returned was then rejected).
	Err error `json:"-"`
}

// RejectedSuggestion pairs a raw provider suggestion with the Issue that got
// it rejected — kept for diagnostics/audit, never trusted for anything
// downstream.
type RejectedSuggestion struct {
	Suggestion Suggestion
	Issue      Issue
}

// SuggestAdjustments calls suggester with req and validates every returned
// suggestion via ValidateSuggestions. Provider failure never blocks the
// deterministic pipeline (section 13): a nil suggester, a disabled Policy,
// or a Suggest error all produce a normal Outcome (no panic, no Go error
// returned from this function) carrying a structured Issue explaining what
// happened — the caller decides whether/how to surface that.
//
// Respects ctx cancellation/deadlines; applies Policy.Timeout when ctx does
// not already carry a tighter deadline.
func SuggestAdjustments(ctx context.Context, req Request, suggester Suggester, policy Policy) Outcome {
	if policy.Mode == ModeDisabled {
		return Outcome{Issues: []Issue{{Code: IssueProviderUnavailable, Severity: SeverityWarning,
			Message: "AI adjustment suggestions disabled (Policy.Mode == ModeDisabled)"}}}
	}
	if suggester == nil {
		return Outcome{Issues: []Issue{{Code: IssueProviderUnavailable, Severity: SeverityWarning,
			Message: "AI adjustment suggestions requested but no Suggester was supplied"}}}
	}
	if len(req.Candidates) == 0 {
		return Outcome{}
	}

	callCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		callCtx, cancel = context.WithTimeout(ctx, policy.timeout())
		defer cancel()
	}

	resp, err := suggester.Suggest(callCtx, req)
	if err != nil {
		return Outcome{Issues: []Issue{classifyProviderError(err)}, Err: err}
	}

	validated := ValidateSuggestions(req, resp.Suggestions)

	out := Outcome{
		Provenance: &Provenance{
			Provider:             resp.Provider,
			Model:                resp.Model,
			AdapterVersion:       resp.AdapterVersion,
			RequestSchemaVersion: RequestSchemaVersion,
			OrchestrationVersion: OrchestrationVersion,
			CandidateRowCount:    len(req.Candidates),
			RawSuggestionCount:   len(resp.Suggestions),
		},
	}
	for i, v := range validated {
		if v.Rejected() {
			out.Rejected = append(out.Rejected, RejectedSuggestion{Suggestion: resp.Suggestions[i], Issue: *v.Issue})
			out.Issues = append(out.Issues, *v.Issue)
			continue
		}
		out.Valid = append(out.Valid, v.Suggestion)
		if v.Issue != nil {
			out.Issues = append(out.Issues, *v.Issue)
		}
	}
	out.Provenance.ValidSuggestionCount = len(out.Valid)
	out.Provenance.RejectedCount = len(out.Rejected)

	return out
}

// BatchOutcome is the result of SuggestAdjustmentsBatch: one Outcome per
// Request (same order, same length as reqs), plus flattened convenience
// accessors.
type BatchOutcome struct {
	// Outcomes is exactly len(reqs)-long; Outcomes[i] corresponds to reqs[i].
	Outcomes []Outcome
}

// AllValid flattens every Outcome's Valid suggestions, in request order then
// provider-returned order within each request — the typical input to
// ToAdjustments for a caller that split candidates across several Requests
// via BuildRequests.
func (b BatchOutcome) AllValid() []Suggestion {
	var out []Suggestion
	for _, o := range b.Outcomes {
		out = append(out, o.Valid...)
	}
	return out
}

// AllIssues flattens every Outcome's Issues, in request order.
func (b BatchOutcome) AllIssues() []Issue {
	var out []Issue
	for _, o := range b.Outcomes {
		out = append(out, o.Issues...)
	}
	return out
}

// SuggestAdjustmentsBatch runs SuggestAdjustments for every entry in reqs
// (typically produced by BuildRequests), preserving request order. One
// request's provider failure is isolated to that request's own Outcome —
// see SuggestAdjustments' identical per-call failure-isolation guarantee —
// and never prevents any other request in reqs from being attempted.
func SuggestAdjustmentsBatch(ctx context.Context, reqs []Request, suggester Suggester, policy Policy) BatchOutcome {
	outcomes := make([]Outcome, len(reqs))
	for i, req := range reqs {
		outcomes[i] = SuggestAdjustments(ctx, req, suggester, policy)
	}
	return BatchOutcome{Outcomes: outcomes}
}

// classifyProviderError maps a Suggester-returned error to the most
// specific Issue this package can report — mirrors
// financial/classification/ai's identical classifyProviderError.
func classifyProviderError(err error) Issue {
	code := IssueProviderError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		code = IssueTimeout
	default:
		var te TimeoutError
		var re RateLimitError
		switch {
		case errors.As(err, &te):
			code = IssueTimeout
		case errors.As(err, &re):
			code = IssueRateLimited
		}
	}
	return Issue{Code: code, Severity: SeverityError, Message: err.Error()}
}

// ToAdjustments converts every valid suggestion in valid into an
// adjustments.Adjustment, ALWAYS with Included == false (section 2/11: an
// AI suggestion never applies itself; only an explicit review.Decision can
// flip Included to true — see review/convert.go's BuildReviewInput, which is
// the only sanctioned path from here into the review domain).
//
// idPrefix namespaces the generated adjustments.ID
// ("ai-adjustment:<prefix>:<row-id>:<period>:<type>") so a caller combining
// AI suggestions from multiple batches/runs never collides IDs purely by
// chance; pass "" for no extra namespacing.
//
// A RequiresUserInput suggestion (section 5) is converted with Amount == 0
// and Notes explicitly stating a benchmark value is still needed — never
// the flagged CURRENT amount, since using that as the adjustment's own
// Amount would silently normalize earnings by the WRONG (unbenchmarked)
// figure the moment a caller accepted it. The flagged current amount is
// still visible via Notes; a human/accountant must supply the real
// normalization amount and construct a proper adjustments.Adjustment (or
// override this one's Amount through review, which
// AdjustmentDecision.NewAmount already supports) before it can be included.
func ToAdjustments(valid []Suggestion, idPrefix string) []adjustments.Adjustment {
	out := make([]adjustments.Adjustment, 0, len(valid))
	for _, s := range valid {
		id := adjustmentID(idPrefix, s)
		adj := adjustments.Adjustment{
			ID:        id,
			Period:    s.Period,
			Type:      s.AdjustmentType,
			Reason:    s.Reason,
			SourceRef: s.SourceRowID,
			Included:  false,
			Notes:     "AI-suggested adjustment; requires human review before inclusion",
		}
		if s.RequiresUserInput {
			adj.Amount = 0
			adj.Notes = fmt.Sprintf("AI-suggested %s: flagged current amount %v requires a user-supplied replacement/benchmark value before this adjustment can be applied (requires_user_input)", s.AdjustmentType, s.Amount)
		} else {
			adj.Amount = s.Amount
			effect, ok := s.Direction.toEffect()
			if ok {
				adj.Effect = effect
			}
		}
		out = append(out, adj)
	}
	return out
}

// RequiredAdjustmentIDs returns the set of adjustments.ID values (as plain
// strings) for adjs — intended to be passed straight to
// review.BuildInput.RequiredAdjustmentIDs so every AI-suggested adjustment
// reaches review.Build as a Required item (section 11's hard requirement),
// never silently defaulting to the ordinary caller-adjustment
// Required == false review.Build otherwise applies. Typically called with
// ToAdjustments' own return value: review.BuildInput{Adjustments: adjs,
// RequiredAdjustmentIDs: ai.RequiredAdjustmentIDs(adjs)}.
func RequiredAdjustmentIDs(adjs []adjustments.Adjustment) map[string]bool {
	out := make(map[string]bool, len(adjs))
	for _, a := range adjs {
		out[string(a.ID)] = true
	}
	return out
}

func adjustmentID(prefix string, s Suggestion) adjustments.ID {
	if prefix == "" {
		return adjustments.ID(fmt.Sprintf("ai-adjustment:%s:%s:%s", s.SourceRowID, s.Period, s.AdjustmentType))
	}
	return adjustments.ID(fmt.Sprintf("ai-adjustment:%s:%s:%s:%s", prefix, s.SourceRowID, s.Period, s.AdjustmentType))
}
