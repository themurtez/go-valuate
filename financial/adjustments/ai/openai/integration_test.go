package openai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/adjustments/ai"
)

// TestIntegration_RealProvider is the ONLY test in this package that touches
// the real OpenAI API. Entirely opt-in: it SKIPS (never fails) unless
// OPENAI_API_KEY is set, so normal `go test ./...` runs never require
// credentials, never touch the network, and never spend API cost.
//
// Kept deliberately tiny (two candidate rows, one call, a short model) to
// avoid spending meaningful API cost. It verifies exactly what a fake
// suggester cannot prove: the response actually parses as valid Structured
// Outputs JSON, and every returned suggestion is source-bound (passes
// ai.ValidateSuggestions against the exact request that was sent).
func TestIntegration_RealProvider(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("SKIPPED — provider credentials not configured (set OPENAI_API_KEY to run this test)")
	}

	suggester, err := New(Config{APIKey: apiKey, Model: os.Getenv("OPENAI_MODEL")})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	req := ai.Request{
		Candidates: []ai.SourceRow{
			{RowID: "row-1", Period: "2025", Label: "Owner Auto Lease", ParentLabel: "Operating Expenses", Code: financial.CodeOpexVehicle, StatementType: financial.StatementIncomeStatement, Amount: 9600},
			{RowID: "row-2", Period: "2025", Label: "Product Sales", StatementType: financial.StatementIncomeStatement, Amount: 500000},
		},
		AllowedTypes: ai.BuildAllowedTypes(adjustments.AllTypes()),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	outcome := ai.SuggestAdjustments(ctx, req, suggester, ai.Policy{Mode: ai.ModeEnabled})
	if outcome.Err != nil {
		t.Fatalf("real provider call failed: %v", outcome.Err)
	}
	for _, iss := range outcome.Rejected {
		t.Errorf("unexpected rejected suggestion from a live provider call: %+v (issue: %+v)", iss.Suggestion, iss.Issue)
	}
	t.Logf("real provider returned %d valid suggestion(s)", len(outcome.Valid))
	for _, s := range outcome.Valid {
		t.Logf("suggestion: row=%s type=%s amount=%v direction=%s requires_user_input=%v reason=%q", s.SourceRowID, s.AdjustmentType, s.Amount, s.Direction, s.RequiresUserInput, s.Reason)
	}
}
