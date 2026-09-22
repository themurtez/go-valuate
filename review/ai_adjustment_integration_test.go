package review

import (
	"context"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	adjustmentsai "github.com/themurtez/go-valuate/financial/adjustments/ai"
	"github.com/themurtez/go-valuate/financial/metrics"
)

func allTypeMetas() []adjustments.TypeMeta { return adjustments.AllTypes() }

func applyDeterministicAdjustments(t *testing.T, snap metrics.Snapshot, adjs []adjustments.Adjustment) adjustments.Result {
	t.Helper()
	return adjustments.Apply(snap, adjs)
}

// TestIntegration_AIAdjustmentSuggestion_ProducesRequiredReviewItem proves
// section 11's core requirement: a validated AI adjustment suggestion,
// converted via adjustmentsai.ToAdjustments and fed into review.Build
// through BuildInput.RequiredAdjustmentIDs, always produces a KindAdjustment
// item with Required == true — unlike an ordinary caller-constructed
// adjustment, which defaults to Required == false (see
// TestBuild_AdjustmentConfirmation in build_test.go).
func TestIntegration_AIAdjustmentSuggestion_ProducesRequiredReviewItem(t *testing.T) {
	suggestion := adjustmentsai.Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: "one_time_expense",
		Amount: 42000, Direction: adjustmentsai.DirectionIncreaseEarnings, Reason: "one-time relocation expense",
	}
	adjs := adjustmentsai.ToAdjustments([]adjustmentsai.Suggestion{suggestion}, "")
	if len(adjs) != 1 {
		t.Fatalf("expected 1 adjustment, got %d", len(adjs))
	}
	if adjs[0].Included {
		t.Fatal("expected the AI-suggested adjustment to start with Included false")
	}

	plan := Build(BuildInput{
		Adjustments:           adjs,
		RequiredAdjustmentIDs: adjustmentsai.RequiredAdjustmentIDs(adjs),
	}, DefaultPolicy())

	item := mustFindItem(t, plan, KindAdjustment, buildAdjustmentID(string(adjs[0].ID)))
	if !item.Required {
		t.Error("expected the AI-suggested adjustment's review item to be Required")
	}
	if item.Severity != SeverityWarning {
		t.Errorf("expected SeverityWarning for a Required AI suggestion, got %s", item.Severity)
	}
}

// TestIntegration_AIAdjustmentSuggestion_DoesNotForceNotReady proves section
// 12: an unresolved AI adjustment suggestion, by itself, must not force
// NOT_READY — only READY_WITH_WARNINGS. A caller who wants stricter
// behavior enforces its own policy on top (e.g. refusing to proceed while
// ApplyResult.UnresolvedRequired is non-empty), but EvaluateReadiness itself
// never escalates a Required-but-non-blocking item to BLOCKING.
func TestIntegration_AIAdjustmentSuggestion_DoesNotForceNotReady(t *testing.T) {
	suggestion := adjustmentsai.Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: "one_time_expense",
		Amount: 42000, Direction: adjustmentsai.DirectionIncreaseEarnings, Reason: "one-time expense",
	}
	adjs := adjustmentsai.ToAdjustments([]adjustmentsai.Suggestion{suggestion}, "")
	plan := Build(BuildInput{
		Adjustments:           adjs,
		RequiredAdjustmentIDs: adjustmentsai.RequiredAdjustmentIDs(adjs),
	}, DefaultPolicy())

	readiness := EvaluateReadiness(plan.Items)
	if readiness.State == ReadinessNotReady {
		t.Fatalf("expected an unresolved AI adjustment suggestion to never by itself produce NOT_READY, got reasons: %+v", readiness.Reasons)
	}
	if readiness.State != ReadinessReadyWithWarnings {
		t.Errorf("expected READY_WITH_WARNINGS while the suggestion is unresolved, got %s", readiness.State)
	}
}

// TestIntegration_AIAdjustmentSuggestion_AcceptCreatesDeterministicInput
// proves section 11's ACCEPT path: a review.Decision with ActionAccept
// (Included=true) against the AI-suggested item produces an
// adjustments.Adjustment with Included == true, ready for
// financial/adjustments.Apply — and, per section 2's core safety rule,
// asserts NO earnings change occurred before this acceptance.
func TestIntegration_AIAdjustmentSuggestion_AcceptCreatesDeterministicInput(t *testing.T) {
	suggestion := adjustmentsai.Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: "one_time_expense",
		Amount: 42000, Direction: adjustmentsai.DirectionIncreaseEarnings, Reason: "one-time relocation expense",
	}
	adjs := adjustmentsai.ToAdjustments([]adjustmentsai.Suggestion{suggestion}, "")
	plan := Build(BuildInput{
		Adjustments:           adjs,
		RequiredAdjustmentIDs: adjustmentsai.RequiredAdjustmentIDs(adjs),
	}, DefaultPolicy())

	item := mustFindItem(t, plan, KindAdjustment, buildAdjustmentID(string(adjs[0].ID)))

	// Before acceptance: still excluded, exactly as ToAdjustments built it.
	source := Source{Adjustments: adjs}
	beforeResult := Apply(source, plan, nil)
	if beforeResult.Adjustments[0].Included {
		t.Fatal("expected the adjustment to remain excluded before any accept decision")
	}

	decisions := []Decision{{
		ItemID: item.ID, Action: ActionAccept,
		Adjustment: &AdjustmentDecision{Included: true},
	}}
	afterResult := Apply(source, plan, decisions)
	if len(afterResult.Invalid) != 0 {
		t.Fatalf("expected the accept decision to apply cleanly, got Invalid=%+v", afterResult.Invalid)
	}
	if !afterResult.Adjustments[0].Included {
		t.Fatal("expected Included true after an accept decision")
	}
	if afterResult.Adjustments[0].Amount != 42000 {
		t.Errorf("expected the accepted adjustment's amount to remain 42000, got %v", afterResult.Adjustments[0].Amount)
	}
}

// TestIntegration_AIAdjustmentSuggestion_RejectProducesNoAdjustment proves
// the REJECT/EXCLUDE path: an ActionIgnore decision against the AI-suggested
// item leaves Included false, so financial/adjustments.Apply never includes
// it in a bridge.
func TestIntegration_AIAdjustmentSuggestion_RejectProducesNoAdjustment(t *testing.T) {
	suggestion := adjustmentsai.Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: "unusual_loss",
		Amount: 18000, Direction: adjustmentsai.DirectionIncreaseEarnings, Reason: "fire damage",
	}
	adjs := adjustmentsai.ToAdjustments([]adjustmentsai.Suggestion{suggestion}, "")
	plan := Build(BuildInput{
		Adjustments:           adjs,
		RequiredAdjustmentIDs: adjustmentsai.RequiredAdjustmentIDs(adjs),
	}, DefaultPolicy())
	item := mustFindItem(t, plan, KindAdjustment, buildAdjustmentID(string(adjs[0].ID)))

	source := Source{Adjustments: adjs}
	decisions := []Decision{{ItemID: item.ID, Action: ActionIgnore}}
	result := Apply(source, plan, decisions)
	if len(result.Invalid) != 0 {
		t.Fatalf("expected the ignore decision to apply cleanly, got Invalid=%+v", result.Invalid)
	}
	if result.Adjustments[0].Included {
		t.Fatal("expected Included false after a reject/ignore decision")
	}
}

// TestIntegration_AIAdjustmentSuggestion_ModifiedAllowedValue proves the
// "modify amount only where deterministic domain rules allow it" path: an
// ActionOverride decision with AdjustmentDecision.NewAmount replaces the
// adjustment's amount (this is the existing review-domain lever a human
// uses to supply, e.g., a corrected figure or the missing benchmark value
// for a RequiresUserInput suggestion).
func TestIntegration_AIAdjustmentSuggestion_ModifiedAllowedValue(t *testing.T) {
	suggestion := adjustmentsai.Suggestion{
		SourceRowID: "row-5", Period: "2025", AdjustmentType: "owner_compensation_normalization",
		Amount: 220000, Direction: adjustmentsai.DirectionDecreaseEarnings, Reason: "officer comp appears above market rate",
		RequiresUserInput: true,
	}
	adjs := adjustmentsai.ToAdjustments([]adjustmentsai.Suggestion{suggestion}, "")
	if adjs[0].Amount != 0 {
		t.Fatalf("expected AI to leave amount at 0 pending user input, got %v", adjs[0].Amount)
	}
	plan := Build(BuildInput{
		Adjustments:           adjs,
		RequiredAdjustmentIDs: adjustmentsai.RequiredAdjustmentIDs(adjs),
	}, DefaultPolicy())
	item := mustFindItem(t, plan, KindAdjustment, buildAdjustmentID(string(adjs[0].ID)))

	source := Source{Adjustments: adjs}
	// The accountant supplies the real replacement-salary-derived
	// normalization amount (e.g. $30,000 difference between actual and
	// market-rate compensation) via ActionOverride.
	decisions := []Decision{{
		ItemID: item.ID, Action: ActionOverride,
		Adjustment: &AdjustmentDecision{Included: true, NewAmount: true, Amount: 30000, Reason: "market-rate GM salary comparison supplied by accountant"},
	}}
	result := Apply(source, plan, decisions)
	if len(result.Invalid) != 0 {
		t.Fatalf("expected the override decision to apply cleanly, got Invalid=%+v", result.Invalid)
	}
	if !result.Adjustments[0].Included || result.Adjustments[0].Amount != 30000 {
		t.Fatalf("expected Included=true amount=30000 after user-supplied override, got %+v", result.Adjustments[0])
	}
}

// TestIntegration_AIAdjustmentSuggestion_InvalidModificationRejected proves
// a non-finite override amount is rejected by Apply's existing validation
// (IssueNonFiniteAmount) — this package's decision-validation rules apply
// identically to an AI-sourced adjustment as to any other.
func TestIntegration_AIAdjustmentSuggestion_InvalidModificationRejected(t *testing.T) {
	suggestion := adjustmentsai.Suggestion{
		SourceRowID: "row-1", Period: "2025", AdjustmentType: "one_time_expense",
		Amount: 42000, Direction: adjustmentsai.DirectionIncreaseEarnings, Reason: "one-time expense",
	}
	adjs := adjustmentsai.ToAdjustments([]adjustmentsai.Suggestion{suggestion}, "")
	plan := Build(BuildInput{
		Adjustments:           adjs,
		RequiredAdjustmentIDs: adjustmentsai.RequiredAdjustmentIDs(adjs),
	}, DefaultPolicy())
	item := mustFindItem(t, plan, KindAdjustment, buildAdjustmentID(string(adjs[0].ID)))

	source := Source{Adjustments: adjs}
	nan := nanForTest()
	decisions := []Decision{{
		ItemID: item.ID, Action: ActionOverride,
		Adjustment: &AdjustmentDecision{Included: true, NewAmount: true, Amount: nan},
	}}
	result := Apply(source, plan, decisions)
	if len(result.Invalid) != 1 {
		t.Fatalf("expected 1 invalid decision for a non-finite override amount, got %d: %+v", len(result.Invalid), result.Invalid)
	}
	if result.Adjustments[0].Included {
		t.Fatal("expected the adjustment to remain excluded after an invalid override")
	}
}

func nanForTest() float64 {
	var zero float64
	return zero / zero
}

// TestEndToEnd_AIAdjustmentSuggestion_ToValuation is this package's
// end-to-end chain for the AI-adjustment-suggestion capability (section 17
// "End-to-end" / section 11's full architecture diagram):
//
//	financial rows (already classified/normalized)
//	  -> financial/adjustments/ai candidate selection + fake Suggester
//	  -> adjustmentsai.ToAdjustments (Included == false)
//	  -> review.Build (KindAdjustment, Required == true)
//	  -> human ACCEPT decision
//	  -> review.Apply
//	  -> financial/adjustments.Apply (deterministic engine)
//	  -> normalized SDE
//
// Asserts explicitly that normalized SDE before acceptance equals the
// unadjusted snapshot SDE (no earnings change occurs before review
// acceptance), and that only after acceptance does the adjustment move
// normalized SDE.
func TestEndToEnd_AIAdjustmentSuggestion_ToValuation(t *testing.T) {
	dataset := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeRevProduct, Period: "2025", Amount: 500000},
			{Code: financial.CodeCogsMaterial, Period: "2025", Amount: 150000},
			{Code: financial.CodeOpexPayroll, Period: "2025", Amount: 120000},
			{Code: financial.CodeOpexOwnerComp, Period: "2025", Amount: 90000},
			{Code: financial.CodeOpexOther, Period: "2025", Amount: 42000, Sources: []financial.SourceRef{{RowID: "row-1", Period: "2025", Amount: 42000}}},
		},
	}
	metricsResult := metrics.Calculate(dataset, metrics.Options{})
	snap, ok := metricsResult.SnapshotFor("2025")
	if !ok || !snap.SDE.Available {
		t.Fatalf("test setup error: expected an available 2025 SDE snapshot, got %+v", snap)
	}

	// --- AI candidate selection + suggestion over the confirmed dataset.
	candidates := []adjustmentsai.SourceRow{
		{RowID: "row-1", Period: "2025", Label: "One-Time Legal Settlement", Code: financial.CodeOpexOther, StatementType: financial.StatementIncomeStatement, Amount: 42000},
	}
	fake := &adjustmentsai.FakeSuggester{Response: adjustmentsai.Response{
		Provider: "fake", Model: "fake-1",
		Suggestions: []adjustmentsai.Suggestion{
			{SourceRowID: "row-1", Period: "2025", AdjustmentType: "one_time_expense", Amount: 42000, Direction: adjustmentsai.DirectionIncreaseEarnings, Reason: "one-time legal settlement, non-recurring"},
		},
	}}
	req := adjustmentsai.Request{Candidates: candidates, AllowedTypes: adjustmentsai.BuildAllowedTypes(allTypeMetas())}
	outcome := adjustmentsai.SuggestAdjustments(context.Background(), req, fake, adjustmentsai.Policy{Mode: adjustmentsai.ModeEnabled})
	if len(outcome.Valid) != 1 {
		t.Fatalf("expected 1 valid AI suggestion, got %d (rejected: %+v)", len(outcome.Valid), outcome.Rejected)
	}

	adjs := adjustmentsai.ToAdjustments(outcome.Valid, "")

	plan := Build(BuildInput{
		Adjustments:           adjs,
		RequiredAdjustmentIDs: adjustmentsai.RequiredAdjustmentIDs(adjs),
	}, DefaultPolicy())
	item := mustFindItem(t, plan, KindAdjustment, buildAdjustmentID(string(adjs[0].ID)))
	if !item.Required {
		t.Fatal("expected the AI-suggested adjustment's review item to be Required")
	}

	source := Source{Adjustments: adjs}

	// --- Assert NO earnings change before acceptance.
	beforeApply := Apply(source, plan, nil)
	beforeAdjResult := applyDeterministicAdjustments(t, snap, beforeApply.Adjustments)
	if beforeAdjResult.SDEBridge.NormalizedValue != snap.SDE.Value {
		t.Fatalf("expected normalized SDE to equal unadjusted SDE before review acceptance, got %v want %v", beforeAdjResult.SDEBridge.NormalizedValue, snap.SDE.Value)
	}

	// --- Human accepts the suggestion.
	decisions := []Decision{{ItemID: item.ID, Action: ActionAccept, Adjustment: &AdjustmentDecision{Included: true}}}
	afterApply := Apply(source, plan, decisions)
	if len(afterApply.Invalid) != 0 {
		t.Fatalf("expected the accept decision to apply cleanly, got Invalid=%+v", afterApply.Invalid)
	}

	afterAdjResult := applyDeterministicAdjustments(t, snap, afterApply.Adjustments)
	wantSDE := snap.SDE.Value + 42000
	if afterAdjResult.SDEBridge.NormalizedValue != wantSDE {
		t.Fatalf("expected normalized SDE to reflect the accepted add-back, got %v want %v", afterAdjResult.SDEBridge.NormalizedValue, wantSDE)
	}
}
