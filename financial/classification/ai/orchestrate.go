package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
)

// OrchestrationVersion identifies the exact trigger/fallback/safety rules
// ClassifyWithFallback and ClassifyBatchWithFallback implement (which
// FallbackMode runs AI when, structural-row skipping, budget enforcement,
// disagreement handling). See the repository README's versioning-strategy
// section; bump only when this decision logic changes in a way that could
// make a historical FallbackOutcome not reproduce identically under new
// code.
const OrchestrationVersion = "1.0.0"

// isStructuralKind reports whether kind is one of the three structural
// financial.RowKind values (HEADING, SUBTOTAL, TOTAL) that AI fallback must
// never be allowed to silently fold back into an ordinary financial row —
// mirrors review's identical isStructuralRowKind guard (see
// review/apply.go), applied one layer earlier here so a structural row
// mostly never even reaches a provider call in the first place.
func isStructuralKind(kind financial.RowKind) bool {
	switch kind {
	case financial.RowKindHeading, financial.RowKindSubtotal, financial.RowKindTotal:
		return true
	default:
		return false
	}
}

// Provenance is the complete audit record for one row's AI-fallback
// attempt, whether or not it ultimately produced a usable suggestion — see
// the README's AI-provenance section (section 14 of the task brief) for
// exactly which fields this satisfies and why. Always populated once
// ClassifyWithFallback decides to attempt AI at all; a row AI was never
// consulted for (disabled, structural, over budget, already-confident under
// AIUnknownOnly/AIBelowConfidence) carries no Provenance.
type Provenance struct {
	// Provider is the adapter-supplied provider name (e.g. "openai"),
	// copied from Response metadata the Classifier implementation attaches
	// — see the openai adapter's own doc comment for how it sets this.
	Provider string `json:"provider,omitempty"`
	// Model is the specific model identifier the provider used (e.g.
	// "gpt-4o-mini"), when the adapter supplies one.
	Model string `json:"model,omitempty"`
	// AdapterVersion identifies the provider adapter package's own version
	// (e.g. openai.AdapterVersion), distinct from OrchestrationVersion and
	// RequestSchemaVersion — three independently-versioned concerns per the
	// task's explicit "add version identifiers for orchestration, request
	// schema, and provider adapter" instruction.
	AdapterVersion string `json:"adapter_version,omitempty"`
	// RequestSchemaVersion echoes RequestSchemaVersion at the time this
	// Provenance was produced.
	RequestSchemaVersion string `json:"request_schema_version"`
	// OrchestrationVersion echoes OrchestrationVersion at the time this
	// Provenance was produced.
	OrchestrationVersion string `json:"orchestration_version"`
	// TriggerMode is the Policy.Mode value that caused this row to be sent
	// to AI.
	TriggerMode FallbackMode `json:"trigger_mode"`
	// DeterministicResult is the deterministic pipeline's result that
	// triggered (or, under AIForce, accompanied) the fallback attempt — see
	// DeterministicSummary.
	DeterministicResult DeterministicSummary `json:"deterministic_result"`
	// ProposedCode is the AI-proposed code actually accepted after
	// validation (CodeUnknown if the AI result was invalid/failed/uncertain
	// — see FallbackOutcome.Result for what the row's usable result is).
	ProposedCode financial.Code `json:"proposed_code,omitempty"`
	// Alternatives mirrors Response.Alternatives, after validation.
	Alternatives []financial.Code `json:"alternatives,omitempty"`
	// Reason mirrors Response.Reason.
	Reason string `json:"reason,omitempty"`
	// ModelConfidence carries Response.RawConfidence forward under a name
	// that makes its non-calibrated nature explicit at every downstream
	// call site — see Response.RawConfidence's doc comment. Never combined
	// with classification.Confidence into a single blended number (section
	// 22 of the task brief) — the two stay separately identifiable here and
	// on DeterministicResult.Confidence.
	ModelConfidence *float64 `json:"model_confidence,omitempty"`
	// ReviewRequired is always true for any row that reached this far (a
	// genuine AI attempt was made) — a hard product rule, not a
	// model-confidence-dependent decision. Kept as an explicit field
	// (rather than leaving the caller to infer it) so a persisted
	// Provenance is self-describing.
	ReviewRequired bool `json:"review_required"`
	// Disagreement is populated when the deterministic pipeline already
	// proposed a DIFFERENT non-UNKNOWN code than AI (only possible under
	// AIForce or AIBelowConfidence — AIUnknownOnly only ever calls AI when
	// the deterministic side has no code to disagree with). See
	// Disagreement's own doc comment.
	Disagreement *Disagreement `json:"disagreement,omitempty"`
	// Issues carries every non-fatal Issue surfaced while producing this
	// Provenance (e.g. IssueContextTooLarge trimming context rows). Fatal
	// problems (provider error, invalid response) are NOT here — they
	// short-circuit Provenance entirely and are reported on
	// FallbackOutcome.Issues instead, since at that point there is no
	// successful AI attempt left to attach them to.
	Issues []Issue `json:"issues,omitempty"`
}

// Disagreement records that the deterministic pipeline and the AI fallback
// proposed two DIFFERENT non-UNKNOWN codes for the same row. Per the task's
// explicit instruction, this package never silently prefers one: both are
// preserved, side by side, so a future review UI can render exactly
//
//	Rule classifier: OPEX_PAYROLL
//	AI fallback:     COGS_DIRECT_LABOR
//
// and a human decides. FallbackOutcome.Result always stays the
// DETERMINISTIC result whenever Disagreement is non-nil — see
// ClassifyWithFallback.
type Disagreement struct {
	// DeterministicCode is the deterministic pipeline's proposed code.
	DeterministicCode financial.Code `json:"deterministic_code"`
	// AICode is the AI fallback's proposed code.
	AICode financial.Code `json:"ai_code"`
}

// FallbackOutcome is the result of attempting AI fallback for one row. It
// never modifies the deterministic classification.Result it was given —
// Result field below always starts as that exact same value and is only
// ever REPLACED (not mutated in place) with an AI-derived
// classification.Result when AI produced a validated, non-UNKNOWN, non-
// disagreeing improvement over an UNKNOWN/low-confidence deterministic
// result (see ClassifyWithFallback for the precise replacement rule).
type FallbackOutcome struct {
	// RowID identifies the financial.RawLineItem.ID this outcome concerns.
	RowID string `json:"row_id"`
	// Result is the classification.Result to use downstream: either the
	// unmodified deterministic result (AI disabled/skipped/failed/
	// disagreed/still-UNKNOWN), or a new Result reflecting the validated AI
	// suggestion (Source == classification.SourceAI, ReviewRequired ==
	// true) — see classify.go's buildAIResult.
	Result classification.Result `json:"result"`
	// Provenance is the full AI-attempt audit record, nil when AI was never
	// attempted for this row (see Provenance's own doc comment for exactly
	// when that is).
	Provenance *Provenance `json:"provenance,omitempty"`
	// Issues carries every Issue for this row — both non-fatal ones
	// (surfaced also on Provenance.Issues, duplicated here so a caller can
	// inspect every row's issues from one flat FallbackOutcome without
	// checking Provenance's presence first) and fatal ones (provider
	// error/timeout/invalid response) that left Provenance nil.
	Issues []Issue `json:"issues,omitempty"`
	// Err is the raw, wrapped error Classifier.Classify returned, if the
	// provider call itself failed — preserved for diagnostics per the
	// task's "preserve wrapped provider errors for diagnostics where safe"
	// instruction. Nil whenever the provider call succeeded (even if its
	// Response was then rejected as invalid, which is IssueInvalidCode/
	// IssueInvalidResponse on Issues, not Err).
	Err error `json:"-"`
}

// BatchOutcome is the result of ClassifyBatchWithFallback: one
// FallbackOutcome per input row (same order, same length — see
// classifyMany), plus batch-level bookkeeping.
type BatchOutcome struct {
	// Outcomes is exactly len(raws)-long; Outcomes[i] corresponds to
	// raws[i] passed to ClassifyBatchWithFallback.
	Outcomes []FallbackOutcome `json:"outcomes"`
	// AIRowsAttempted counts how many rows actually reached a Classifier
	// call (successful or not) — the number Policy.MaxAIRows bounds.
	AIRowsAttempted int `json:"ai_rows_attempted"`
	// BudgetExceeded is true when Policy.MaxAIRows was reached before every
	// eligible row could be attempted — see IssueBudgetExceeded on the
	// affected rows' Issues.
	BudgetExceeded bool `json:"budget_exceeded,omitempty"`
}

// ClassifyWithFallback classifies ONE raw row: it always runs the
// deterministic classification.Classify first, then consults classifier
// only if policy.Mode and the deterministic result together say to (see
// FallbackMode). Equivalent to calling ClassifyBatchWithFallback with a
// single-element slice; provided as a convenience for the common one-row
// case and to keep tests that only need to exercise one row simple.
func ClassifyWithFallback(ctx context.Context, raw financial.RawLineItem, cfg classification.Config, classifier Classifier, policy Policy) FallbackOutcome {
	out := ClassifyBatchWithFallback(ctx, []financial.RawLineItem{raw}, cfg, classifier, policy, nil)
	return out.Outcomes[0]
}

// ClassifyBatchWithFallback classifies every row in raws, preserving input
// order (result[i] always corresponds to raws[i] — see BatchOutcome), and
// applies AI fallback per policy for whichever rows are eligible.
//
// contextByRowID optionally supplies a caller-assembled context window per
// row ID (see ContextRow) — nil/absent entries simply mean no context rows
// are sent for that row. This package never assembles context automatically
// from raws itself (no implicit "nearby in the slice" heuristic), since
// slice order is not guaranteed to reflect the row's actual position in the
// source document; a caller that wants context rows decides which ones are
// relevant and supplies them explicitly.
//
// Failure isolation: a provider failure or invalid response for one row
// never affects any other row's result, and never makes that row's own
// result WORSE than what deterministic classification already produced
// (see the README's fallback-failure-behavior section) — UNLESS
// policy.Strict is true, in which case the first AI failure aborts the
// remaining batch (every not-yet-processed row is left on its
// deterministic result with IssueProviderError attached, and this function
// still returns normally with no Go error, consistent with review.Apply's
// "no bare Go error, everything via issues" convention elsewhere in this
// repository).
func ClassifyBatchWithFallback(ctx context.Context, raws []financial.RawLineItem, cfg classification.Config, classifier Classifier, policy Policy, contextByRowID map[string][]ContextRow) BatchOutcome {
	det := classification.ClassifyBatch(raws, cfg)

	outcomes := make([]FallbackOutcome, len(raws))
	var eligible []int
	for i, res := range det {
		outcomes[i] = FallbackOutcome{RowID: res.RowID, Result: res}
		if shouldAttemptAI(raws[i], res, policy) {
			eligible = append(eligible, i)
		} else if isStructuralKind(raws[i].Kind) && policy.Mode != AIDisabled && !policy.AllowStructuralRows {
			outcomes[i].Issues = append(outcomes[i].Issues, Issue{
				RowID: res.RowID, Code: IssueStructuralRowSkipped, Severity: SeverityWarning,
				Message: "AI fallback skipped: row is structural (HEADING/SUBTOTAL/TOTAL) and Policy.AllowStructuralRows is false",
			})
		}
	}

	batch := BatchOutcome{Outcomes: outcomes}
	if len(eligible) == 0 || policy.Mode == AIDisabled {
		return batch
	}
	if classifier == nil {
		for _, i := range eligible {
			outcomes[i].Issues = append(outcomes[i].Issues, Issue{
				RowID: det[i].RowID, Code: IssueProviderUnavailable, Severity: SeverityError,
				Message: "AI fallback requested but no Classifier was supplied",
			})
		}
		return batch
	}

	budget := len(eligible)
	if policy.MaxAIRows > 0 && policy.MaxAIRows < budget {
		budget = policy.MaxAIRows
	}
	attemptIdx := eligible[:budget]
	skippedIdx := eligible[budget:]
	for _, i := range skippedIdx {
		batch.BudgetExceeded = true
		outcomes[i].Issues = append(outcomes[i].Issues, Issue{
			RowID: det[i].RowID, Code: IssueBudgetExceeded, Severity: SeverityWarning,
			Message: fmt.Sprintf("AI fallback budget (MaxAIRows=%d) reached; row left on deterministic result", policy.MaxAIRows),
		})
	}

	reqs := make([]Request, len(attemptIdx))
	for k, i := range attemptIdx {
		reqs[k] = buildRequest(raws[i], det[i], cfg, policy, contextByRowID[det[i].RowID])
	}

	callCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		callCtx, cancel = context.WithTimeout(ctx, policy.timeout())
		defer cancel()
	}

	// Strict mode dispatches one request at a time so the FIRST failure
	// genuinely stops every further provider call (not merely suppresses
	// applying results already received) — classifyMany's normal path
	// dispatches the whole eligible set up front (sequentially or as
	// provider-side batches) for throughput, which would make "abandon on
	// first failure" purely cosmetic. Strict is the opt-in, deliberately
	// slower path; see Policy.Strict's doc comment.
	if policy.Strict {
		for k, i := range attemptIdx {
			resp, err := classifier.Classify(callCtx, reqs[k])
			batch.AIRowsAttempted++
			if err != nil {
				outcomes[i] = resolveOutcome(raws[i], det[i], reqs[k], resp, err, policy)
				for _, j := range attemptIdx[k+1:] {
					outcomes[j].Issues = append(outcomes[j].Issues, Issue{
						RowID: det[j].RowID, Code: IssueProviderError, Severity: SeverityWarning,
						Message: "batch abandoned after an earlier AI failure (Policy.Strict is true)",
					})
				}
				return batch
			}
			outcomes[i] = resolveOutcome(raws[i], det[i], reqs[k], resp, nil, policy)
		}
		return batch
	}

	results, errs := classifyMany(callCtx, classifier, reqs, policy.MaxBatchSize)
	batch.AIRowsAttempted = len(attemptIdx)

	for k, i := range attemptIdx {
		outcomes[i] = resolveOutcome(raws[i], det[i], reqs[k], results[k], errs[k], policy)
	}

	return batch
}

// shouldAttemptAI implements FallbackMode's trigger semantics (section 4)
// plus structural-row safety (section 3): a structural row is never
// eligible unless policy.AllowStructuralRows is explicitly set.
func shouldAttemptAI(raw financial.RawLineItem, res classification.Result, policy Policy) bool {
	if policy.Mode == AIDisabled {
		return false
	}
	if isStructuralKind(raw.Kind) && !policy.AllowStructuralRows {
		return false
	}
	switch policy.Mode {
	case AIUnknownOnly:
		return res.IsUnknown()
	case AIBelowConfidence:
		return res.IsUnknown() || float64(res.Confidence) < policy.confidenceThreshold()
	case AIForce:
		return true
	default:
		return false
	}
}

// buildRequest assembles the minimal, deterministic Request for one row —
// see Request's own doc comment for exactly what is and is not included.
func buildRequest(raw financial.RawLineItem, res classification.Result, cfg classification.Config, policy Policy, context []ContextRow) Request {
	normalized := classification.NormalizeLabel(raw.Label)

	maxCtx := policy.maxContextRows()
	if len(context) > maxCtx {
		context = context[:maxCtx]
	}

	allowed := codesForStatement(raw.StatementType)

	return Request{
		RawLabel:        raw.Label,
		NormalizedLabel: normalized.Comparable,
		ParentLabel:     raw.ParentLabel,
		StatementType:   string(raw.StatementType),
		RowKind:         string(raw.Kind),
		AllowedCodes:    BuildAllowedCodes(allowed),
		ContextRows:     context,
		DeterministicResult: DeterministicSummary{
			Code:       res.Code,
			Source:     string(res.Source),
			Confidence: float64(res.Confidence),
		},
	}
}

// codesForStatement narrows the closed set to the row's own statement type
// when known, so the model is not asked to choose between (for example)
// balance-sheet codes for an income-statement row — a tighter closed set is
// both cheaper and less error-prone than the full taxonomy every time.
// Falls back to the full taxonomy when the row's StatementType is unset or
// matches no known code (defensive only; every financial.StatementType
// value currently has at least one code).
func codesForStatement(st financial.StatementType) []financial.CodeMeta {
	if st == "" {
		return financial.AllCodes()
	}
	var metas []financial.CodeMeta
	for _, m := range financial.AllCodes() {
		if m.StatementType == st {
			metas = append(metas, m)
		}
	}
	if len(metas) == 0 {
		return financial.AllCodes()
	}
	return metas
}

// resolveOutcome turns one provider call's (Response, error) pair into a
// FallbackOutcome: validates the response, decides whether it improves on
// the deterministic result, and builds Provenance/Disagreement per the
// rules documented on FallbackOutcome/Provenance/Disagreement.
func resolveOutcome(raw financial.RawLineItem, det classification.Result, req Request, resp Response, callErr error, policy Policy) FallbackOutcome {
	out := FallbackOutcome{RowID: det.RowID, Result: det}

	if callErr != nil {
		out.Err = callErr
		out.Issues = append(out.Issues, classifyProviderError(det.RowID, callErr))
		return out
	}

	if issue := ValidateResponse(req, resp); issue != nil {
		issue.RowID = det.RowID
		out.Issues = append(out.Issues, *issue)
		return out
	}

	prov := &Provenance{
		Provider:             resp.Provider,
		Model:                resp.Model,
		AdapterVersion:       resp.AdapterVersion,
		RequestSchemaVersion: RequestSchemaVersion,
		OrchestrationVersion: OrchestrationVersion,
		TriggerMode:          policy.Mode,
		DeterministicResult:  req.DeterministicResult,
		ProposedCode:         resp.Code,
		Alternatives:         resp.Alternatives,
		Reason:               resp.Reason,
		ModelConfidence:      resp.RawConfidence,
		ReviewRequired:       true,
	}

	structural := isStructuralKind(raw.Kind)

	switch {
	case resp.Code == CodeUnknown:
		// AI declined to propose anything — deterministic result stands
		// (see section 9: AI may always answer UNKNOWN).
	case structural:
		// Section 3: even a forced diagnostic AI call on a structural row
		// must never be allowed to cause normalization to include it —
		// preserve the deterministic (structural) result untouched and
		// record what AI said purely for provenance/diagnostics.
	case det.Code != "" && det.Code != resp.Code && !det.IsUnknown():
		// Deterministic already proposed a DIFFERENT real code: preserve
		// both, never silently prefer AI (section 21).
		prov.Disagreement = &Disagreement{DeterministicCode: det.Code, AICode: resp.Code}
	default:
		// Deterministic was UNKNOWN (or, under AIForce, agreed/had no
		// competing code) and AI proposed a validated closed-set code: this
		// is the one case AI actually becomes the row's usable Result.
		out.Result = buildAIResult(det, resp)
	}

	out.Provenance = prov
	out.Issues = append(out.Issues, prov.Issues...)
	return out
}

// classifyProviderError maps a Classifier-returned error to the most
// specific Issue this package can report, per section 16's error taxonomy:
// context.DeadlineExceeded or a TimeoutError -> IssueTimeout, a
// RateLimitError -> IssueRateLimited, everything else -> the generic
// IssueProviderError. The original error is never inspected for its string
// contents — only structural checks (errors.Is/As), so callers are never
// required to parse provider error strings (section 16's explicit
// requirement).
func classifyProviderError(rowID string, err error) Issue {
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
	return Issue{RowID: rowID, Code: code, Severity: SeverityError, Message: err.Error()}
}
