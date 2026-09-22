package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
)

func TestSuggestAdjustments_Disabled_NoCallMade(t *testing.T) {
	fake := &FakeSuggester{}
	req := Request{Candidates: []SourceRow{{RowID: "row-1", Period: "2025", Amount: 100}}}
	out := SuggestAdjustments(context.Background(), req, fake, DefaultPolicy())
	if len(fake.Calls) != 0 {
		t.Fatal("expected no provider call when Policy.Mode is ModeDisabled (the default)")
	}
	if len(out.Issues) != 1 || out.Issues[0].Code != IssueProviderUnavailable {
		t.Fatalf("expected a single IssueProviderUnavailable issue, got %+v", out.Issues)
	}
}

func TestSuggestAdjustments_NilSuggester_NoPanic(t *testing.T) {
	req := Request{Candidates: []SourceRow{{RowID: "row-1", Period: "2025", Amount: 100}}}
	out := SuggestAdjustments(context.Background(), req, nil, Policy{Mode: ModeEnabled})
	if len(out.Issues) != 1 || out.Issues[0].Code != IssueProviderUnavailable {
		t.Fatalf("expected a single IssueProviderUnavailable issue, got %+v", out.Issues)
	}
	if out.Err != nil {
		t.Fatalf("expected no error return, got %v", out.Err)
	}
}

func TestSuggestAdjustments_NoCandidates_EmptyOutcome(t *testing.T) {
	fake := &FakeSuggester{}
	out := SuggestAdjustments(context.Background(), Request{}, fake, Policy{Mode: ModeEnabled})
	if len(fake.Calls) != 0 {
		t.Fatal("expected no provider call for an empty candidate set")
	}
	if len(out.Valid) != 0 || len(out.Rejected) != 0 || out.Provenance != nil {
		t.Fatalf("expected a fully empty Outcome, got %+v", out)
	}
}

func TestSuggestAdjustments_ProviderFailure_NeverBlocksDeterministicPipeline(t *testing.T) {
	fake := &FakeSuggester{Err: errors.New("boom")}
	req := Request{Candidates: []SourceRow{{RowID: "row-1", Period: "2025", Amount: 100}}, AllowedTypes: BuildAllowedTypes(adjustments.AllTypes())}
	out := SuggestAdjustments(context.Background(), req, fake, Policy{Mode: ModeEnabled})
	if out.Err == nil {
		t.Fatal("expected Err to carry the wrapped provider error")
	}
	if len(out.Issues) != 1 || out.Issues[0].Code != IssueProviderError {
		t.Fatalf("expected IssueProviderError, got %+v", out.Issues)
	}
	if len(out.Valid) != 0 {
		t.Fatalf("expected no valid suggestions on provider failure, got %+v", out.Valid)
	}
}

func TestSuggestAdjustments_Timeout_ReportsIssueTimeout(t *testing.T) {
	fake := &FakeSuggester{Err: context.DeadlineExceeded}
	req := Request{Candidates: []SourceRow{{RowID: "row-1", Period: "2025", Amount: 100}}}
	out := SuggestAdjustments(context.Background(), req, fake, Policy{Mode: ModeEnabled})
	if len(out.Issues) != 1 || out.Issues[0].Code != IssueTimeout {
		t.Fatalf("expected IssueTimeout, got %+v", out.Issues)
	}
}

func TestSuggestAdjustments_ValidAndRejectedSuggestionsSeparated(t *testing.T) {
	req := Request{
		Candidates: []SourceRow{
			{RowID: "row-1", Period: "2025", Label: "One-Time Legal Fee", Code: financial.CodeOpexOther, Amount: 42000},
		},
		AllowedTypes: BuildAllowedTypes(adjustments.AllTypes()),
	}
	fake := &FakeSuggester{Response: Response{
		Provider: "fake", Model: "fake-1",
		Suggestions: []Suggestion{
			{SourceRowID: "row-1", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense, Amount: 42000, Direction: DirectionIncreaseEarnings, Reason: "legal settlement"},
			{SourceRowID: "row-1", Period: "2025", AdjustmentType: adjustments.TypeOneTimeExpense, Amount: 99999, Direction: DirectionIncreaseEarnings, Reason: "invented"},
		},
	}}
	out := SuggestAdjustments(context.Background(), req, fake, Policy{Mode: ModeEnabled})
	if len(out.Valid) != 1 {
		t.Fatalf("expected 1 valid suggestion, got %d", len(out.Valid))
	}
	if len(out.Rejected) != 1 || out.Rejected[0].Issue.Code != IssueInventedAmount {
		t.Fatalf("expected 1 rejected suggestion with IssueInventedAmount, got %+v", out.Rejected)
	}
	if out.Provenance == nil {
		t.Fatal("expected non-nil Provenance on a successful provider call")
	}
	if out.Provenance.RawSuggestionCount != 2 || out.Provenance.ValidSuggestionCount != 1 || out.Provenance.RejectedCount != 1 {
		t.Fatalf("unexpected provenance counts: %+v", out.Provenance)
	}
	if out.Provenance.RequestSchemaVersion != RequestSchemaVersion || out.Provenance.OrchestrationVersion != OrchestrationVersion {
		t.Fatalf("expected provenance to echo current schema/orchestration versions, got %+v", out.Provenance)
	}
}

func TestSuggestAdjustments_RespectsContextCancellation(t *testing.T) {
	fake := &FakeSuggester{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := Request{Candidates: []SourceRow{{RowID: "row-1", Period: "2025", Amount: 100}}}
	out := SuggestAdjustments(ctx, req, fake, Policy{Mode: ModeEnabled})
	if out.Err == nil {
		t.Fatal("expected a context-cancellation error")
	}
}

func TestSuggestAdjustments_TimeoutAppliedWhenCtxHasNoDeadline(t *testing.T) {
	fake := &FakeSuggester{}
	req := Request{Candidates: []SourceRow{{RowID: "row-1", Period: "2025", Amount: 100}}}
	_ = SuggestAdjustments(context.Background(), req, fake, Policy{Mode: ModeEnabled, Timeout: 5 * time.Second})
	if len(fake.Calls) != 1 {
		t.Fatalf("expected exactly one call, got %d", len(fake.Calls))
	}
}

func TestSuggestAdjustmentsBatch_IsolatesPerRequestFailure(t *testing.T) {
	callCount := 0
	suggester := funcSuggester(func(ctx context.Context, req Request) (Response, error) {
		callCount++
		if req.Candidates[0].RowID == "row-bad" {
			return Response{}, errors.New("boom")
		}
		return Response{Suggestions: []Suggestion{
			{SourceRowID: req.Candidates[0].RowID, Period: req.Candidates[0].Period, AdjustmentType: adjustments.TypeOneTimeExpense, Amount: req.Candidates[0].Amount, Direction: DirectionIncreaseEarnings, Reason: "ok"},
		}}, nil
	})

	reqs := []Request{
		{Candidates: []SourceRow{{RowID: "row-good-1", Period: "2025", Amount: 100}}, AllowedTypes: BuildAllowedTypes(adjustments.AllTypes())},
		{Candidates: []SourceRow{{RowID: "row-bad", Period: "2025", Amount: 100}}, AllowedTypes: BuildAllowedTypes(adjustments.AllTypes())},
		{Candidates: []SourceRow{{RowID: "row-good-2", Period: "2025", Amount: 100}}, AllowedTypes: BuildAllowedTypes(adjustments.AllTypes())},
	}

	batch := SuggestAdjustmentsBatch(context.Background(), reqs, suggester, Policy{Mode: ModeEnabled})
	if callCount != 3 {
		t.Fatalf("expected all 3 requests attempted despite the middle one failing, got %d calls", callCount)
	}
	if len(batch.Outcomes) != 3 {
		t.Fatalf("expected 3 outcomes, got %d", len(batch.Outcomes))
	}
	if batch.Outcomes[0].Err != nil || batch.Outcomes[2].Err != nil {
		t.Fatalf("expected the two good requests to succeed, got errs: %v / %v", batch.Outcomes[0].Err, batch.Outcomes[2].Err)
	}
	if batch.Outcomes[1].Err == nil {
		t.Fatal("expected the bad request's own outcome to carry the error")
	}
	all := batch.AllValid()
	if len(all) != 2 {
		t.Fatalf("expected 2 flattened valid suggestions (from the two good requests), got %d", len(all))
	}
}

type funcSuggester func(ctx context.Context, req Request) (Response, error)

func (f funcSuggester) Suggest(ctx context.Context, req Request) (Response, error) {
	return f(ctx, req)
}

var _ Suggester = funcSuggester(nil)
