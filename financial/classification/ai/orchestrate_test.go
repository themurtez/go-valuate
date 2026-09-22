package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
)

func floatPtr(f float64) *float64 { return &f }

// TestClassifyWithFallback_DisabledByDefault proves DefaultPolicy() never
// consults the Classifier at all, and a nil Classifier is safe (no panic)
// under the default policy.
func TestClassifyWithFallback_DisabledByDefault(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Totally Unrecognizable Label"}
	cfg := classification.Config{}

	out := ClassifyWithFallback(context.Background(), raw, cfg, nil, DefaultPolicy())

	if !out.Result.IsUnknown() {
		t.Fatalf("expected deterministic UNKNOWN result unchanged, got %+v", out.Result)
	}
	if out.Provenance != nil {
		t.Errorf("expected no Provenance when AI is disabled, got %+v", out.Provenance)
	}
	if len(out.Issues) != 0 {
		t.Errorf("expected no issues when AI is disabled, got %+v", out.Issues)
	}
}

// TestClassifyWithFallback_UnknownOnly_SkipsAlreadyClassifiedRows proves
// AIUnknownOnly never calls AI for a row the deterministic pipeline already
// classified, even at low confidence.
func TestClassifyWithFallback_UnknownOnly_SkipsAlreadyClassifiedRows(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Advertising"}
	cfg := classification.Config{
		AliasLayers: []classification.AliasLayer{{Name: "global", Aliases: []classification.Alias{
			{Label: "Advertising", Code: financial.CodeOpexMarketing},
		}}},
	}
	fake := NewFakeClassifier()
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyWithFallback(context.Background(), raw, cfg, fake, policy)

	if len(fake.Calls) != 0 {
		t.Fatalf("expected AIUnknownOnly to skip an already-classified row, got %d AI calls", len(fake.Calls))
	}
	if out.Result.Code != financial.CodeOpexMarketing {
		t.Errorf("expected the deterministic alias match preserved, got %s", out.Result.Code)
	}
}

// TestClassifyWithFallback_UnknownOnly_CallsAIForUnknown proves AIUnknownOnly
// calls AI for a genuinely UNKNOWN row and, given a valid response, promotes
// it to the row's usable Result with SourceAI and ReviewRequired.
func TestClassifyWithFallback_UnknownOnly_CallsAIForUnknown(t *testing.T) {
	raw := financial.RawLineItem{
		ID: "row-1", Label: "Field Labor", ParentLabel: "Cost of Sales",
		StatementType: financial.StatementIncomeStatement,
	}
	cfg := classification.Config{}
	fake := NewFakeClassifier()
	fake.Responses["Field Labor"] = Response{
		Code: financial.CodeCogsDirectLabor, Reason: "Field labor appears under Cost of Sales.",
		RawConfidence: floatPtr(0.82), Provider: "fake", Model: "fake-model-1",
	}
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyWithFallback(context.Background(), raw, cfg, fake, policy)

	if len(fake.Calls) != 1 {
		t.Fatalf("expected exactly 1 AI call, got %d", len(fake.Calls))
	}
	if out.Result.Code != financial.CodeCogsDirectLabor {
		t.Fatalf("expected AI-proposed code accepted, got %s", out.Result.Code)
	}
	if out.Result.Source != classification.SourceAI {
		t.Errorf("expected Source SourceAI, got %s", out.Result.Source)
	}
	if !out.Result.ReviewRequired {
		t.Error("expected ReviewRequired true on an AI-sourced result")
	}
	if out.Provenance == nil {
		t.Fatal("expected non-nil Provenance")
	}
	if !out.Provenance.ReviewRequired {
		t.Error("expected Provenance.ReviewRequired true")
	}
	if out.Provenance.Provider != "fake" || out.Provenance.Model != "fake-model-1" {
		t.Errorf("expected provider/model metadata preserved, got %+v", out.Provenance)
	}
	if out.Provenance.ModelConfidence == nil || *out.Provenance.ModelConfidence != 0.82 {
		t.Errorf("expected raw model confidence 0.82 preserved separately, got %v", out.Provenance.ModelConfidence)
	}
}

// TestClassifyWithFallback_RequestContainsClosedSet proves the Request sent
// to the Classifier carries an explicit closed set of allowed codes.
func TestClassifyWithFallback_RequestContainsClosedSet(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Misc", StatementType: financial.StatementIncomeStatement}
	fake := NewFakeClassifier()
	fake.DefaultResponse = Response{Code: CodeUnknown}
	policy := Policy{Mode: AIUnknownOnly}

	ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

	if len(fake.Calls) != 1 {
		t.Fatalf("expected 1 AI call, got %d", len(fake.Calls))
	}
	req := fake.Calls[0]
	if len(req.AllowedCodes) == 0 {
		t.Fatal("expected a non-empty closed set of allowed codes")
	}
	for _, c := range req.AllowedCodes {
		if c.StatementType != string(financial.StatementIncomeStatement) {
			t.Errorf("expected only income-statement codes for an income-statement row, got %+v", c)
		}
		if !financial.IsValidCode(c.Code) {
			t.Errorf("allowed code %q is not a recognized canonical code", c.Code)
		}
	}
}

// TestClassifyWithFallback_InvalidCodeRejected proves an AI response
// proposing a code outside the closed set is rejected outright: UNKNOWN is
// preserved, and a structured IssueInvalidCode is reported — the model must
// never be allowed to invent a new account code.
func TestClassifyWithFallback_InvalidCodeRejected(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Something Weird"}
	fake := NewFakeClassifier()
	fake.Responses["Something Weird"] = Response{Code: "TOTALLY_MADE_UP_CODE"}
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

	if !out.Result.IsUnknown() {
		t.Fatalf("expected UNKNOWN preserved after an invalid AI code, got %+v", out.Result)
	}
	if out.Provenance != nil {
		t.Errorf("expected no Provenance for a rejected response, got %+v", out.Provenance)
	}
	if len(out.Issues) != 1 || out.Issues[0].Code != IssueInvalidCode {
		t.Fatalf("expected exactly 1 IssueInvalidCode, got %+v", out.Issues)
	}
}

// TestClassifyWithFallback_InvalidAlternativeCodeRejected proves the
// closed-set check also applies to Response.Alternatives, not just Code.
func TestClassifyWithFallback_InvalidAlternativeCodeRejected(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Something Weird", StatementType: financial.StatementIncomeStatement}
	fake := NewFakeClassifier()
	fake.Responses["Something Weird"] = Response{
		Code:         financial.CodeOpexOther,
		Alternatives: []financial.Code{"NOT_A_REAL_CODE"},
	}
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

	if !out.Result.IsUnknown() {
		t.Fatalf("expected UNKNOWN preserved, got %+v", out.Result)
	}
	if len(out.Issues) != 1 || out.Issues[0].Code != IssueInvalidCode {
		t.Fatalf("expected IssueInvalidCode, got %+v", out.Issues)
	}
}

// TestClassifyWithFallback_UnknownResponseAllowed proves AI may answer
// CodeUnknown without being forced to choose a code, and this is not
// treated as an error — the attempt is still recorded in Provenance (AI WAS
// consulted and declined, distinct from AI never being consulted at all),
// but the row's usable Result stays the deterministic UNKNOWN.
func TestClassifyWithFallback_UnknownResponseAllowed(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Ambiguous Item"}
	fake := NewFakeClassifier()
	fake.Responses["Ambiguous Item"] = Response{Code: CodeUnknown, Reason: "insufficient context"}
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

	if !out.Result.IsUnknown() {
		t.Fatalf("expected UNKNOWN preserved, got %+v", out.Result)
	}
	if len(out.Issues) != 0 {
		t.Errorf("expected no issues for a valid UNKNOWN response, got %+v", out.Issues)
	}
	if out.Provenance == nil {
		t.Fatal("expected Provenance recorded: AI was consulted and explicitly declined, which is itself audit-worthy")
	}
	if out.Provenance.ProposedCode != CodeUnknown {
		t.Errorf("expected Provenance.ProposedCode to record AI's own UNKNOWN answer, got %s", out.Provenance.ProposedCode)
	}
}

// TestClassifyWithFallback_EmptyResponseRejected proves a zero-value
// Response (no Code at all) is rejected as malformed.
func TestClassifyWithFallback_EmptyResponseRejected(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Misc"}
	fake := NewFakeClassifier()
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

	if len(out.Issues) != 1 || out.Issues[0].Code != IssueEmptyResponse {
		t.Fatalf("expected IssueEmptyResponse, got %+v", out.Issues)
	}
}

// TestClassifyWithFallback_OutOfRangeConfidenceRejected proves a
// RawConfidence outside [0, 1] is rejected.
func TestClassifyWithFallback_OutOfRangeConfidenceRejected(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Misc", StatementType: financial.StatementIncomeStatement}
	fake := NewFakeClassifier()
	fake.Responses["Misc"] = Response{Code: financial.CodeOpexOther, RawConfidence: floatPtr(1.5)}
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

	if len(out.Issues) != 1 || out.Issues[0].Code != IssueInvalidResponse {
		t.Fatalf("expected IssueInvalidResponse, got %+v", out.Issues)
	}
}

// TestClassifyWithFallback_StructuralRowSkippedByDefault proves a
// structural row (SUBTOTAL/TOTAL/HEADING) never reaches the Classifier at
// all under the default AllowStructuralRows=false, even under AIForce.
func TestClassifyWithFallback_StructuralRowSkippedByDefault(t *testing.T) {
	for _, kind := range []financial.RowKind{financial.RowKindHeading, financial.RowKindSubtotal, financial.RowKindTotal} {
		t.Run(string(kind), func(t *testing.T) {
			raw := financial.RawLineItem{ID: "row-1", Label: "Gross Profit", Kind: kind}
			fake := NewFakeClassifier()
			policy := Policy{Mode: AIForce}

			out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

			if len(fake.Calls) != 0 {
				t.Fatalf("expected 0 AI calls for a structural row, got %d", len(fake.Calls))
			}
			if out.Result.Kind != kind {
				t.Errorf("expected Kind preserved, got %s", out.Result.Kind)
			}
			foundSkipIssue := false
			for _, iss := range out.Issues {
				if iss.Code == IssueStructuralRowSkipped {
					foundSkipIssue = true
				}
			}
			if !foundSkipIssue {
				t.Errorf("expected IssueStructuralRowSkipped, got %+v", out.Issues)
			}
		})
	}
}

// TestClassifyWithFallback_ForcedStructuralRow_NeverAggregates proves that
// even when a caller explicitly forces AI on a structural row
// (AllowStructuralRows=true), the returned code can never cause the row to
// stop being structural — RowKind/Status must be preserved untouched
// regardless of what the model answers.
func TestClassifyWithFallback_ForcedStructuralRow_NeverAggregates(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Gross Profit", Kind: financial.RowKindSubtotal}
	det := classification.Classify(raw, classification.Config{})
	if det.Status != financial.RowStatusSubtotal {
		t.Fatalf("test setup error: expected the deterministic result to already be a subtotal, got %s", det.Status)
	}

	fake := NewFakeClassifier()
	fake.Responses["Gross Profit"] = Response{Code: financial.CodeRevProduct, Reason: "looks like revenue"}
	policy := Policy{Mode: AIForce, AllowStructuralRows: true}

	out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

	if len(fake.Calls) != 1 {
		t.Fatalf("expected exactly 1 AI call when explicitly forced, got %d", len(fake.Calls))
	}
	if out.Result.Status != financial.RowStatusSubtotal {
		t.Fatalf("structural row must never be converted to a normal row by AI: expected Status subtotal, got %s", out.Result.Status)
	}
	if out.Result.Kind != financial.RowKindSubtotal {
		t.Errorf("expected Kind preserved as subtotal, got %s", out.Result.Kind)
	}
	if out.Result.Code == financial.CodeRevProduct {
		t.Error("AI-proposed code must never become the structural row's usable Code")
	}
	// The AI's answer is still recorded for diagnostics, just not applied.
	if out.Provenance == nil || out.Provenance.ProposedCode != financial.CodeRevProduct {
		t.Errorf("expected the AI's answer preserved in Provenance for diagnostics, got %+v", out.Provenance)
	}
}

// TestClassifyWithFallback_Disagreement proves that when the deterministic
// pipeline and AI propose two DIFFERENT non-UNKNOWN codes, both are
// preserved (Disagreement) and the DETERMINISTIC result remains the row's
// usable Result — AI is never silently preferred.
func TestClassifyWithFallback_Disagreement(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Advertising", StatementType: financial.StatementIncomeStatement}
	cfg := classification.Config{
		AliasLayers: []classification.AliasLayer{{Name: "global", Aliases: []classification.Alias{
			{Label: "Advertising", Code: financial.CodeOpexMarketing},
		}}},
	}
	fake := NewFakeClassifier()
	fake.Responses["Advertising"] = Response{Code: financial.CodeCogsDirectLabor, Reason: "AI disagrees"}
	policy := Policy{Mode: AIForce}

	out := ClassifyWithFallback(context.Background(), raw, cfg, fake, policy)

	if out.Result.Code != financial.CodeOpexMarketing {
		t.Fatalf("expected the deterministic result preserved as the usable Result, got %s", out.Result.Code)
	}
	if out.Result.Source == classification.SourceAI {
		t.Error("expected Source to remain the deterministic source, not SourceAI")
	}
	if out.Provenance == nil || out.Provenance.Disagreement == nil {
		t.Fatal("expected a recorded Disagreement")
	}
	if out.Provenance.Disagreement.DeterministicCode != financial.CodeOpexMarketing {
		t.Errorf("expected DeterministicCode OPEX_MARKETING, got %s", out.Provenance.Disagreement.DeterministicCode)
	}
	if out.Provenance.Disagreement.AICode != financial.CodeCogsDirectLabor {
		t.Errorf("expected AICode COGS_DIRECT_LABOR, got %s", out.Provenance.Disagreement.AICode)
	}

	desc := DescribeDisagreement(*out.Provenance.Disagreement)
	if desc == "" {
		t.Error("expected a non-empty human-readable disagreement description")
	}
}

// TestClassifyWithFallback_ProviderError_LeavesDeterministicResultIntact
// proves a provider failure never makes the row's result WORSE than the
// deterministic pipeline already produced.
func TestClassifyWithFallback_ProviderError_LeavesDeterministicResultIntact(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Misc"}
	fake := NewFakeClassifier()
	fake.Errs["Misc"] = errors.New("provider is down")
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

	if !out.Result.IsUnknown() {
		t.Fatalf("expected UNKNOWN preserved after a provider error, got %+v", out.Result)
	}
	if out.Err == nil {
		t.Error("expected the wrapped provider error to be preserved on Err")
	}
	if len(out.Issues) != 1 || out.Issues[0].Code != IssueProviderError {
		t.Fatalf("expected IssueProviderError, got %+v", out.Issues)
	}
}

// TestClassifyWithFallback_Timeout proves a context-deadline failure is
// reported as IssueTimeout specifically, not the generic IssueProviderError.
func TestClassifyWithFallback_Timeout(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Misc"}
	fake := NewFakeClassifier()
	fake.Errs["Misc"] = context.DeadlineExceeded
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

	if len(out.Issues) != 1 || out.Issues[0].Code != IssueTimeout {
		t.Fatalf("expected IssueTimeout, got %+v", out.Issues)
	}
}

// fakeRateLimitErr satisfies RateLimitError for testing error-taxonomy
// classification.
type fakeRateLimitErr struct{}

func (fakeRateLimitErr) Error() string     { return "rate limited" }
func (fakeRateLimitErr) RateLimited() bool { return true }

// TestClassifyWithFallback_RateLimited proves an error satisfying
// RateLimitError is reported as IssueRateLimited.
func TestClassifyWithFallback_RateLimited(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Misc"}
	fake := NewFakeClassifier()
	fake.Errs["Misc"] = fakeRateLimitErr{}
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, fake, policy)

	if len(out.Issues) != 1 || out.Issues[0].Code != IssueRateLimited {
		t.Fatalf("expected IssueRateLimited, got %+v", out.Issues)
	}
}

// TestClassifyBatchWithFallback_PreservesOrderAndIsolatesFailures proves
// ClassifyBatchWithFallback preserves input order and that one malformed
// response does not corrupt any other row's result.
func TestClassifyBatchWithFallback_PreservesOrderAndIsolatesFailures(t *testing.T) {
	raws := []financial.RawLineItem{
		{ID: "row-1", Label: "Unknown One", StatementType: financial.StatementIncomeStatement},
		{ID: "row-2", Label: "Unknown Two", StatementType: financial.StatementIncomeStatement},
		{ID: "row-3", Label: "Unknown Three", StatementType: financial.StatementIncomeStatement},
	}
	fake := NewFakeClassifier()
	fake.Responses["Unknown One"] = Response{Code: financial.CodeOpexOther}
	fake.Responses["Unknown Two"] = Response{Code: "INVALID_CODE_HERE"} // malformed
	fake.Responses["Unknown Three"] = Response{Code: financial.CodeOpexMarketing}
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyBatchWithFallback(context.Background(), raws, classification.Config{}, fake, policy, nil)

	if len(out.Outcomes) != 3 {
		t.Fatalf("expected 3 outcomes, got %d", len(out.Outcomes))
	}
	if out.Outcomes[0].RowID != "row-1" || out.Outcomes[1].RowID != "row-2" || out.Outcomes[2].RowID != "row-3" {
		t.Fatalf("expected input order preserved, got %+v", out.Outcomes)
	}
	if out.Outcomes[0].Result.Code != financial.CodeOpexOther {
		t.Errorf("row-1: expected OPEX_OTHER, got %s", out.Outcomes[0].Result.Code)
	}
	if !out.Outcomes[1].Result.IsUnknown() {
		t.Errorf("row-2: expected UNKNOWN preserved after malformed AI response, got %+v", out.Outcomes[1].Result)
	}
	if len(out.Outcomes[1].Issues) != 1 || out.Outcomes[1].Issues[0].Code != IssueInvalidCode {
		t.Errorf("row-2: expected IssueInvalidCode, got %+v", out.Outcomes[1].Issues)
	}
	if out.Outcomes[2].Result.Code != financial.CodeOpexMarketing {
		t.Errorf("row-3: expected OPEX_MARKETING (unaffected by row-2's failure), got %s", out.Outcomes[2].Result.Code)
	}
}

// TestClassifyBatchWithFallback_MaxAIRows proves the budget limit leaves
// remaining eligible rows on their deterministic result with
// IssueBudgetExceeded, rather than silently exceeding the configured cap.
func TestClassifyBatchWithFallback_MaxAIRows(t *testing.T) {
	raws := []financial.RawLineItem{
		{ID: "row-1", Label: "Unknown One"},
		{ID: "row-2", Label: "Unknown Two"},
		{ID: "row-3", Label: "Unknown Three"},
	}
	fake := NewFakeClassifier()
	fake.DefaultResponse = Response{Code: CodeUnknown}
	policy := Policy{Mode: AIUnknownOnly, MaxAIRows: 2}

	out := ClassifyBatchWithFallback(context.Background(), raws, classification.Config{}, fake, policy, nil)

	if out.AIRowsAttempted != 2 {
		t.Fatalf("expected exactly 2 AI attempts under MaxAIRows=2, got %d", out.AIRowsAttempted)
	}
	if !out.BudgetExceeded {
		t.Error("expected BudgetExceeded true")
	}
	if len(fake.Calls) != 2 {
		t.Fatalf("expected exactly 2 provider calls, got %d", len(fake.Calls))
	}
	lastOutcome := out.Outcomes[2]
	foundBudgetIssue := false
	for _, iss := range lastOutcome.Issues {
		if iss.Code == IssueBudgetExceeded {
			foundBudgetIssue = true
		}
	}
	if !foundBudgetIssue {
		t.Errorf("expected row-3 to carry IssueBudgetExceeded, got %+v", lastOutcome.Issues)
	}
}

// TestClassifyBatchWithFallback_ContextRowsTrimmedToMax proves
// Policy.MaxContextRows trims an oversized context window rather than
// rejecting the row.
func TestClassifyBatchWithFallback_ContextRowsTrimmedToMax(t *testing.T) {
	raws := []financial.RawLineItem{{ID: "row-1", Label: "Unknown"}}
	fake := NewFakeClassifier()
	fake.DefaultResponse = Response{Code: CodeUnknown}
	policy := Policy{Mode: AIUnknownOnly, MaxContextRows: 2}
	ctxRows := map[string][]ContextRow{
		"row-1": {{Label: "a"}, {Label: "b"}, {Label: "c"}, {Label: "d"}},
	}

	ClassifyBatchWithFallback(context.Background(), raws, classification.Config{}, fake, policy, ctxRows)

	if len(fake.Calls) != 1 {
		t.Fatalf("expected 1 AI call, got %d", len(fake.Calls))
	}
	if len(fake.Calls[0].ContextRows) != 2 {
		t.Fatalf("expected context rows trimmed to MaxContextRows=2, got %d", len(fake.Calls[0].ContextRows))
	}
}

// TestClassifyBatchWithFallback_NoClassifier proves AI-fallback rows are
// reported as IssueProviderUnavailable when Policy requests AI but no
// Classifier was supplied, without panicking.
func TestClassifyBatchWithFallback_NoClassifier(t *testing.T) {
	raws := []financial.RawLineItem{{ID: "row-1", Label: "Unknown"}}
	policy := Policy{Mode: AIUnknownOnly}

	out := ClassifyBatchWithFallback(context.Background(), raws, classification.Config{}, nil, policy, nil)

	if len(out.Outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(out.Outcomes))
	}
	if len(out.Outcomes[0].Issues) != 1 || out.Outcomes[0].Issues[0].Code != IssueProviderUnavailable {
		t.Fatalf("expected IssueProviderUnavailable, got %+v", out.Outcomes[0].Issues)
	}
}

// TestClassifyBatchWithFallback_BatchClassifierPath proves a Classifier
// that also implements BatchClassifier is used via its batch endpoint, and
// produces the exact same per-row results as the sequential path.
func TestClassifyBatchWithFallback_BatchClassifierPath(t *testing.T) {
	raws := []financial.RawLineItem{
		{ID: "row-1", Label: "Unknown One"},
		{ID: "row-2", Label: "Unknown Two"},
	}
	fake := NewFakeClassifier()
	fake.Responses["Unknown One"] = Response{Code: financial.CodeOpexOther}
	fake.Responses["Unknown Two"] = Response{Code: financial.CodeOpexMarketing}
	policy := Policy{Mode: AIUnknownOnly}

	batchFake := fake.AsBatchClassifier()
	out := ClassifyBatchWithFallback(context.Background(), raws, classification.Config{}, batchFake, policy, nil)

	if len(batchFake.BatchCalls) != 1 {
		t.Fatalf("expected the batch endpoint to be used exactly once, got %d calls", len(batchFake.BatchCalls))
	}
	if out.Outcomes[0].Result.Code != financial.CodeOpexOther || out.Outcomes[1].Result.Code != financial.CodeOpexMarketing {
		t.Fatalf("expected both rows correctly resolved via the batch path, got %+v", out.Outcomes)
	}
}

// TestClassifyBatchWithFallback_StrictModeAbandonsOnFirstFailure proves
// Policy.Strict stops the batch at the first AI failure: no request is even
// dispatched for rows after the failing one.
func TestClassifyBatchWithFallback_StrictModeAbandonsOnFirstFailure(t *testing.T) {
	raws := []financial.RawLineItem{
		{ID: "row-1", Label: "Fails"},
		{ID: "row-2", Label: "Never Reached"},
	}
	fake := NewFakeClassifier()
	fake.Errs["Fails"] = errors.New("boom")
	fake.DefaultResponse = Response{Code: financial.CodeOpexOther}
	policy := Policy{Mode: AIForce, Strict: true}

	out := ClassifyBatchWithFallback(context.Background(), raws, classification.Config{}, fake, policy, nil)

	if len(fake.Calls) != 1 {
		t.Fatalf("expected exactly 1 provider call before Strict mode aborts, got %d", len(fake.Calls))
	}
	if out.Outcomes[1].Result.Code == financial.CodeOpexOther {
		t.Error("expected row-2 to never have been classified by AI in Strict mode after row-1 failed")
	}
	foundAbandon := false
	for _, iss := range out.Outcomes[1].Issues {
		if iss.Code == IssueProviderError {
			foundAbandon = true
		}
	}
	if !foundAbandon {
		t.Errorf("expected row-2 to carry an abandonment issue, got %+v", out.Outcomes[1].Issues)
	}
}

// TestClassifyBatchWithFallback_BelowConfidenceTriggersOnLowConfidence
// proves AIBelowConfidence calls AI for a row under the caller's
// threshold, even though it is not UNKNOWN.
func TestClassifyBatchWithFallback_BelowConfidenceTriggersOnLowConfidence(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "vehicle upkeep stuff", StatementType: financial.StatementIncomeStatement}
	cfg := classification.Config{Rules: classification.DefaultRules()}
	det := classification.Classify(raw, cfg)
	if det.IsUnknown() {
		t.Skip("test setup requires a low-but-nonzero-confidence deterministic match; adjust fixture if DefaultRules changes")
	}

	fake := NewFakeClassifier()
	fake.DefaultResponse = Response{Code: CodeUnknown}
	policy := Policy{Mode: AIBelowConfidence, ConfidenceThreshold: 1.0} // force "below" to be true for any match

	out := ClassifyWithFallback(context.Background(), raw, cfg, fake, policy)
	_ = out

	if len(fake.Calls) != 1 {
		t.Fatalf("expected AIBelowConfidence to call AI for a below-threshold match, got %d calls", len(fake.Calls))
	}
}

// TestClassifyBatchWithFallback_ContextDeadlineRespected proves Classify is
// given a context that eventually expires when the caller's own ctx has no
// deadline — Policy.Timeout is honored.
func TestClassifyBatchWithFallback_ContextDeadlineRespected(t *testing.T) {
	raw := financial.RawLineItem{ID: "row-1", Label: "Unknown"}
	blocking := &blockingClassifier{unblock: make(chan struct{})}
	defer close(blocking.unblock)
	policy := Policy{Mode: AIUnknownOnly, Timeout: 10 * time.Millisecond}

	out := ClassifyWithFallback(context.Background(), raw, classification.Config{}, blocking, policy)

	if len(out.Issues) != 1 || out.Issues[0].Code != IssueTimeout {
		t.Fatalf("expected IssueTimeout from Policy.Timeout, got %+v", out.Issues)
	}
}

type blockingClassifier struct {
	unblock chan struct{}
}

func (b *blockingClassifier) Classify(ctx context.Context, req Request) (Response, error) {
	select {
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case <-b.unblock:
		return Response{Code: CodeUnknown}, nil
	}
}
