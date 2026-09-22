package openai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
	"github.com/themurtez/go-valuate/financial/classification/ai"
)

// TestIntegration_RealProvider is the ONLY test in this repository that
// touches the real OpenAI API. It is entirely opt-in: it SKIPS (never
// fails) unless OPENAI_API_KEY is set in the environment, so normal
// `go test ./...` runs — including CI — never require credentials, never
// touch the network, and never spend API cost. See the package doc
// comment and the repository README's "test behavior without credentials"
// section.
//
// Kept deliberately tiny (one row, one call, a short model) to avoid
// spending meaningful API cost per the task's explicit instruction. It
// verifies exactly three things a real provider call must satisfy that a
// fake classifier cannot prove: the response actually parses as valid
// Structured Outputs JSON, the returned code is a member of the allowed
// closed set (or UNKNOWN), and review-required semantics still hold once
// the response reaches ai.ClassifyWithFallback.
func TestIntegration_RealProvider(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("SKIPPED — provider credentials not configured (set OPENAI_API_KEY to run this test)")
	}

	classifier, err := New(Config{APIKey: apiKey, Model: os.Getenv("OPENAI_MODEL")})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	raw := financial.RawLineItem{
		ID: "row-1", Label: "Field Labor", ParentLabel: "Cost of Sales",
		StatementType: financial.StatementIncomeStatement,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out := ai.ClassifyWithFallback(ctx, raw, classification.Config{}, classifier, ai.Policy{Mode: ai.AIUnknownOnly})

	if out.Err != nil {
		t.Fatalf("real provider call failed: %v", out.Err)
	}
	for _, iss := range out.Issues {
		t.Errorf("unexpected issue from a live provider call: %+v", iss)
	}

	if out.Result.Code != "" && !financial.IsValidCode(out.Result.Code) {
		t.Fatalf("provider proposed a code outside the closed set: %q", out.Result.Code)
	}
	if out.Provenance != nil {
		if !out.Provenance.ReviewRequired {
			t.Error("expected ReviewRequired true on any AI-sourced provenance, even from the real provider")
		}
		t.Logf("real provider result: code=%q reason=%q model=%q", out.Result.Code, out.Provenance.Reason, out.Provenance.Model)
	}
}

// TestIntegration_RealProvider_Batch is the batching counterpart to
// TestIntegration_RealProvider: the ONLY test in this repository that sends
// a real multi-row batch request to OpenAI. Entirely opt-in — SKIPS (never
// fails) unless OPENAI_API_KEY is set, exactly like the single-row test
// above.
//
// Kept to exactly two tiny rows (the smallest size that still proves
// batching, as opposed to a single-row call) to keep cost negligible. It
// verifies the three things section 14 requires that only a real batch
// response can prove: both requested row ids come back (none dropped, none
// duplicated), both returned codes are members of the closed set (or
// UNKNOWN — this adapter itself does not pre-filter, so an out-of-set code
// here would mean the provider ignored "strict" Structured Outputs, which
// this test would catch), and mandatory-review metadata still holds once
// the batch reaches ai.ClassifyBatchWithFallback.
func TestIntegration_RealProvider_Batch(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("SKIPPED — provider credentials not configured (set OPENAI_API_KEY to run this test)")
	}

	classifier, err := New(Config{APIKey: apiKey, Model: os.Getenv("OPENAI_MODEL")})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	raws := []financial.RawLineItem{
		{ID: "row-1", Label: "Field Labor", ParentLabel: "Cost of Sales", StatementType: financial.StatementIncomeStatement},
		{ID: "row-2", Label: "Office Supplies", ParentLabel: "Operating Expenses", StatementType: financial.StatementIncomeStatement},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out := ai.ClassifyBatchWithFallback(ctx, raws, classification.Config{}, classifier, ai.Policy{Mode: ai.AIUnknownOnly}, nil)

	if len(out.Outcomes) != 2 {
		t.Fatalf("expected 2 outcomes for 2 input rows, got %d", len(out.Outcomes))
	}
	if out.Outcomes[0].RowID != "row-1" || out.Outcomes[1].RowID != "row-2" {
		t.Fatalf("expected input order/row ids preserved, got %+v", []string{out.Outcomes[0].RowID, out.Outcomes[1].RowID})
	}

	for i, outcome := range out.Outcomes {
		if outcome.Err != nil {
			t.Fatalf("row %d (%s): real provider batch call failed: %v", i, outcome.RowID, outcome.Err)
		}
		for _, iss := range outcome.Issues {
			t.Errorf("row %d (%s): unexpected issue from a live provider batch call: %+v", i, outcome.RowID, iss)
		}
		if outcome.Result.Code != "" && !financial.IsValidCode(outcome.Result.Code) {
			t.Fatalf("row %d (%s): provider proposed a code outside the closed set: %q", i, outcome.RowID, outcome.Result.Code)
		}
		if outcome.Provenance != nil {
			if !outcome.Provenance.ReviewRequired {
				t.Errorf("row %d (%s): expected ReviewRequired true on any AI-sourced provenance, even from the real provider", i, outcome.RowID)
			}
			t.Logf("real provider batch result row=%s: code=%q reason=%q model=%q", outcome.RowID, outcome.Result.Code, outcome.Provenance.Reason, outcome.Provenance.Model)
		}
	}
}
