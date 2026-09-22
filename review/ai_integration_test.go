package review

import (
	"context"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
	"github.com/themurtez/go-valuate/financial/classification/ai"
)

// TestIntegration_AIFallback_AcceptedThroughReviewToNormalize is the
// AI-specific end-to-end chain (prompt 12, section 20): deterministic
// classifier proposes UNKNOWN -> ai.ClassifyWithFallback proposes a
// validated code -> review.Build creates a REQUIRED classification review
// item for it -> a user ACCEPT decision -> review.Apply -> financial.Normalize.
// Proves the AI-fallback boundary needs zero new review-package code: an
// ai-sourced classification.Result (Source == SourceAI) flows through
// review.Build exactly like any deterministic Result.
func TestIntegration_AIFallback_AcceptedThroughReviewToNormalize(t *testing.T) {
	raws := []financial.RawLineItem{
		{ID: "row-1", Label: "Product Sales", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 500000}},
		{ID: "row-2", Label: "Field Labor", ParentLabel: "Cost of Sales", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 120000}},
	}
	cfg := classification.Config{
		AliasLayers: []classification.AliasLayer{{Name: "global", Aliases: []classification.Alias{
			{Label: "Product Sales", Code: financial.CodeRevProduct},
			// "Field Labor" deliberately has no alias/rule match, so the
			// deterministic pipeline lands on UNKNOWN and AI fallback is
			// consulted.
		}}},
	}

	fake := ai.NewFakeClassifier()
	fake.Responses["Field Labor"] = ai.Response{
		Code: financial.CodeCogsDirectLabor, Reason: "Field labor appears under Cost of Sales.",
		Provider: "fake", Model: "fake-1",
	}
	policy := ai.Policy{Mode: ai.AIUnknownOnly}

	batch := ai.ClassifyBatchWithFallback(context.Background(), raws, cfg, fake, policy, nil)
	if len(batch.Outcomes) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(batch.Outcomes))
	}

	results := make([]classification.Result, len(batch.Outcomes))
	for i, out := range batch.Outcomes {
		results[i] = out.Result
	}

	aiResult := results[1]
	if aiResult.Source != classification.SourceAI {
		t.Fatalf("expected row-2 to be AI-sourced, got Source=%s Code=%s", aiResult.Source, aiResult.Code)
	}
	if !aiResult.ReviewRequired {
		t.Fatal("expected the AI-sourced result to have ReviewRequired true")
	}

	plan := Build(BuildInput{Classifications: results, Raws: raws}, DefaultPolicy())
	item := mustFindItem(t, plan, KindClassification, "classification:row-2")
	if !item.Required {
		t.Errorf("expected the AI-sourced classification item to be Required, got Required=%v Severity=%s", item.Required, item.Severity)
	}
	if item.Classification.ProposedCode != financial.CodeCogsDirectLabor {
		t.Errorf("expected the review item to carry the AI-proposed code, got %s", item.Classification.ProposedCode)
	}

	mapped := make([]financial.MappedLineItem, len(results))
	for i, r := range results {
		mapped[i] = r.ToMappedLineItem(raws[i])
	}
	source := Source{MappedLineItems: mapped}

	// Readiness must not be READY while the AI-sourced item is unresolved,
	// even though AI proposed a plausible code — mandatory human review,
	// not automatic trust (section 6).
	readinessBefore := EvaluateReadiness(plan.Items)
	if readinessBefore.State == ReadinessReady {
		t.Fatal("expected readiness to require review of the AI-sourced item before READY")
	}

	decisions := []Decision{{ItemID: "classification:row-2", Action: ActionAccept}}
	applyResult := Apply(source, plan, decisions)
	if len(applyResult.Invalid) != 0 {
		t.Fatalf("expected the accept decision to apply cleanly, got Invalid=%+v", applyResult.Invalid)
	}

	dataset, err := financial.Normalize(applyResult.MappedLineItems, financial.NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("expected Normalize to succeed after accepting the AI-sourced classification, got error: %v", err)
	}

	var foundLabor bool
	for _, it := range dataset.Items {
		if it.Code == financial.CodeCogsDirectLabor && it.Period == "2025" && it.Amount == 120000 {
			foundLabor = true
		}
	}
	if !foundLabor {
		t.Errorf("expected the AI-accepted COGS_DIRECT_LABOR amount to appear in the normalized dataset, got %+v", dataset.Items)
	}
}

// TestIntegration_AIFallback_InvalidCodeRejected_ReadinessStaysNotReady
// proves the other half of section 20: when AI proposes a code outside the
// closed set, the response is rejected, UNKNOWN remains, and readiness
// stays NOT_READY until a human resolves it (AI's malformed answer must
// never quietly become "good enough").
func TestIntegration_AIFallback_InvalidCodeRejected_ReadinessStaysNotReady(t *testing.T) {
	raws := []financial.RawLineItem{
		{ID: "row-1", Label: "Weird Line Item", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 5000}},
	}
	cfg := classification.Config{}

	fake := ai.NewFakeClassifier()
	fake.Responses["Weird Line Item"] = ai.Response{Code: "NOT_A_REAL_TAXONOMY_CODE"}
	policy := ai.Policy{Mode: ai.AIUnknownOnly}

	batch := ai.ClassifyBatchWithFallback(context.Background(), raws, cfg, fake, policy, nil)
	result := batch.Outcomes[0].Result
	if !result.IsUnknown() {
		t.Fatalf("expected UNKNOWN preserved after an invalid AI code, got %+v", result)
	}

	plan := Build(BuildInput{Classifications: []classification.Result{result}, Raws: raws}, DefaultPolicy())
	item := mustFindItem(t, plan, KindClassification, "classification:row-1")
	if item.Severity != SeverityBlocking {
		t.Fatalf("expected an UNKNOWN classification to remain BLOCKING, got %s", item.Severity)
	}

	readiness := EvaluateReadiness(plan.Items)
	if readiness.State != ReadinessNotReady {
		t.Fatalf("expected NOT_READY while the item remains unresolved, got %s", readiness.State)
	}

	// Apply with NO decisions: still unresolved, still NOT_READY.
	mapped := []financial.MappedLineItem{result.ToMappedLineItem(raws[0])}
	applyResult := Apply(Source{MappedLineItems: mapped}, plan, nil)
	readinessAfter := EvaluateReadiness(applyResult.Items)
	if readinessAfter.State != ReadinessNotReady {
		t.Fatalf("expected readiness to remain NOT_READY with no decisions supplied, got %s", readinessAfter.State)
	}
	if len(applyResult.UnresolvedRequired) != 1 {
		t.Fatalf("expected exactly 1 unresolved required item, got %d", len(applyResult.UnresolvedRequired))
	}
}
