package review

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	adjustmentsai "github.com/themurtez/go-valuate/financial/adjustments/ai"
	"github.com/themurtez/go-valuate/financial/classification"
	classificationai "github.com/themurtez/go-valuate/financial/classification/ai"
	"github.com/themurtez/go-valuate/financial/earnings"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/financial/reconciliation"
	"github.com/themurtez/go-valuate/ingestion"
	"github.com/themurtez/go-valuate/ingestion/csv"
	"github.com/themurtez/go-valuate/settings"
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/applicability"
	"github.com/themurtez/go-valuate/valuation/capitalization"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/ebitda"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
	"github.com/themurtez/go-valuate/valuation/profile"
	"github.com/themurtez/go-valuate/valuation/report"
	"github.com/themurtez/go-valuate/valuation/sde"
	"github.com/themurtez/go-valuate/valuation/sensitivity"
)

// TestV1Contract_CanonicalEndToEnd is this repository's canonical V1 smoke
// test: it exercises the complete library contract described in
// docs/V1_CONTRACTS.md and docs/INTEGRATION.md, front to back, using only
// deterministic/fake dependencies (no real OpenAI call, no real Tesseract,
// no network, no database):
//
//	synthetic CSV financial document (raw bytes)
//	  -> ingestion/csv.Parse -> ingestion.Result
//	  -> Result.ToRawLineItems()
//	  -> deterministic classification (financial/classification)
//	  -> optional fake AI fallback classification (one deliberately-unknown row)
//	  -> review.Build (review plan)
//	  -> caller decisions (accept the AI suggestion, confirm structural rows)
//	  -> review.Apply
//	  -> financial.Normalize
//	  -> financial/reconciliation.Run
//	  -> financial/metrics.Calculate
//	  -> fake AI adjustment suggestion (financial/adjustments/ai)
//	  -> adjustment review (review.Build/Apply again)
//	  -> deterministic adjustment application (financial/adjustments.Apply)
//	  -> maintainable earnings (financial/earnings)
//	  -> settings resolution (settings.Resolve)
//	  -> valuation methods (sde, ebitda, capitalization)
//	  -> valuation/orchestrator.Execute
//	  -> basis reconciliation (via valuation/consensus's Options.TargetBasis)
//	  -> valuation/consensus.Calculate
//	  -> valuation/sensitivity
//	  -> valuation/report.Build
//	  -> JSON serialization
//
// This is the library's own proof that every stage documented in
// docs/V1_CONTRACTS.md actually composes end to end, not merely in
// isolation. See examples/full_flow for a runnable, narrated version of
// this same flow intended for an integrating engineer to read.
func TestV1Contract_CanonicalEndToEnd(t *testing.T) {
	// ---------------------------------------------------------------
	// Step 1-2: main app receives file bytes, selects a parser.
	// A synthetic, non-proprietary two-fiscal-year income statement,
	// deliberately including one label ("Misc Reimbursements") that
	// deterministic classification cannot confidently resolve on its own,
	// to exercise the optional AI fallback path.
	// ---------------------------------------------------------------
	const csvDoc = `Income Statement,FY2024,FY2025
Product Sales,850000,940000
Materials,310000,340000
Total COGS,310000,340000
Gross Profit,540000,600000
Owner Compensation,120000,128000
Payroll,180000,196000
Misc Reimbursements,38000,41000
Total Operating Expenses,338000,365000
Net Income,202000,235000
`
	sourceBytes := []byte(csvDoc)
	sourceSnapshot := append([]byte(nil), sourceBytes...) // for the no-mutation assertion below

	// ---------------------------------------------------------------
	// Step 3-4: ingestion.
	// ---------------------------------------------------------------
	ingestResult, ingestErr := csv.Parse(strings.NewReader(string(sourceBytes)), ingestion.Options{})
	if ingestErr != nil {
		t.Fatalf("csv.Parse failed: %+v", ingestErr)
	}
	if string(sourceBytes) != string(sourceSnapshot) {
		t.Fatal("source bytes were mutated by ingestion")
	}
	raws := ingestResult.ToRawLineItems()
	if len(raws) == 0 {
		t.Fatal("test setup error: expected non-empty raw line items")
	}
	rawsSnapshot := marshalForSnapshot(t, raws)

	// ---------------------------------------------------------------
	// Step 5: deterministic classification.
	// ---------------------------------------------------------------
	cfg := classification.Config{
		AliasLayers: []classification.AliasLayer{{Name: "global", Aliases: []classification.Alias{
			{Label: "Product Sales", Code: financial.CodeRevProduct},
			{Label: "Materials", Code: financial.CodeCogsMaterial},
			{Label: "Owner Compensation", Code: financial.CodeOpexOwnerComp},
			{Label: "Payroll", Code: financial.CodeOpexPayroll},
			// "Misc Reimbursements" deliberately has NO alias and no strong
			// rule match — this is the AI-fallback row.
		}}},
		Rules: classification.DefaultRules(),
	}
	deterministicResults := classification.ClassifyBatch(raws, cfg)
	if string(marshalForSnapshot(t, raws)) != string(rawsSnapshot) {
		t.Fatal("ClassifyBatch mutated its input raws")
	}

	// ---------------------------------------------------------------
	// Step 6: optional fake AI fallback classification.
	// AIBelowConfidence so the deterministic UNKNOWN "Misc Reimbursements"
	// row (and only that row) is eligible.
	// ---------------------------------------------------------------
	var miscRowID string
	for i, r := range deterministicResults {
		if r.IsUnknown() && raws[i].Label == "Misc Reimbursements" {
			miscRowID = raws[i].ID
		}
	}
	if miscRowID == "" {
		t.Fatal("test setup error: expected 'Misc Reimbursements' to be deterministically UNKNOWN")
	}

	fakeClassifier := classificationai.NewFakeClassifier()
	rawConfidence := 0.62
	fakeClassifier.Responses["Misc Reimbursements"] = classificationai.Response{
		Code:          financial.CodeOpexOther,
		RawConfidence: &rawConfidence,
		Reason:        "reimbursed miscellaneous expenses, closest fit is other operating expense",
	}
	aiPolicy := classificationai.DefaultPolicy()
	aiPolicy.Mode = classificationai.AIBelowConfidence
	aiPolicy.ConfidenceThreshold = 0.90

	batchOutcome := classificationai.ClassifyBatchWithFallback(context.Background(), raws, cfg, fakeClassifier, aiPolicy, nil)
	if len(batchOutcome.Outcomes) != len(raws) {
		t.Fatalf("expected %d fallback outcomes, got %d", len(raws), len(batchOutcome.Outcomes))
	}
	finalResults := make([]classification.Result, len(batchOutcome.Outcomes))
	for i, o := range batchOutcome.Outcomes {
		finalResults[i] = o.Result
	}
	var aiSourcedFound bool
	for _, r := range finalResults {
		if r.Source == classification.SourceAI {
			aiSourcedFound = true
			if !r.ReviewRequired {
				t.Error("expected an AI-sourced classification.Result to always have ReviewRequired == true")
			}
		}
	}
	if !aiSourcedFound {
		t.Fatal("expected at least one AI-sourced classification result in this fixture")
	}

	// ---------------------------------------------------------------
	// Step 7: review.Build (classification plan).
	// ---------------------------------------------------------------
	plan := Build(BuildInput{
		Classifications: finalResults,
		Raws:            raws,
	}, DefaultPolicy())

	// An AI-sourced classification item is always Required but only
	// SeverityWarning (never SeverityBlocking — "AI is not presumed wrong,
	// only unconfirmed"), so readiness before any decisions is
	// READY_WITH_WARNINGS here, not NOT_READY — see the README's "Mandatory
	// human review" section. This fixture deliberately has no genuinely
	// BLOCKING item (e.g. an unparsed period) to keep the happy path
	// focused on the AI-fallback/adjustment-suggestion contract; see
	// review/e2e_test.go for the sibling test that exercises an actual
	// NOT_READY-until-resolved blocking item.
	readinessBeforeDecisions := EvaluateReadiness(plan.Items)
	if readinessBeforeDecisions.State != ReadinessReadyWithWarnings {
		t.Fatalf("expected READY_WITH_WARNINGS before any review decisions (an unresolved AI-sourced item, never blocking by itself), got %s: %+v", readinessBeforeDecisions.State, readinessBeforeDecisions.Reasons)
	}

	// ---------------------------------------------------------------
	// Step 8: UI/main app collects decisions from a human.
	// Accept the AI suggestion; accept every structural-row confirmation;
	// accept every other classification item Build proposed review for.
	// ---------------------------------------------------------------
	var decisions []Decision
	for _, item := range plan.Items {
		switch item.Kind {
		case KindClassification, KindStructure:
			decisions = append(decisions, Decision{ItemID: item.ID, Action: ActionAccept})
		}
	}

	// ---------------------------------------------------------------
	// Step 9: review.Apply.
	// ---------------------------------------------------------------
	planSnapshot := marshalForSnapshot(t, plan)
	decisionsSnapshot := marshalForSnapshot(t, decisions)
	source := Source{MappedLineItems: toMappedLineItems(t, raws, finalResults)}
	sourceSnapshotJSON := marshalForSnapshot(t, source)

	applyResult := Apply(source, plan, decisions)
	if len(applyResult.Invalid) != 0 {
		t.Fatalf("expected every decision to apply cleanly, got Invalid=%+v", applyResult.Invalid)
	}
	if string(marshalForSnapshot(t, plan)) != string(planSnapshot) {
		t.Fatal("Apply mutated plan")
	}
	if string(marshalForSnapshot(t, decisions)) != string(decisionsSnapshot) {
		t.Fatal("Apply mutated decisions")
	}
	if string(marshalForSnapshot(t, source)) != string(sourceSnapshotJSON) {
		t.Fatal("Apply mutated source")
	}

	readinessAfterDecisions := EvaluateReadiness(applyForReadiness(plan, decisions))
	if readinessAfterDecisions.State == ReadinessNotReady {
		t.Fatalf("expected readiness to clear NOT_READY once every blocking item is resolved, got reasons: %+v", readinessAfterDecisions.Reasons)
	}

	// ---------------------------------------------------------------
	// Step 10: financial.Normalize.
	// ---------------------------------------------------------------
	mappedSnapshot := marshalForSnapshot(t, applyResult.MappedLineItems)
	dataset, normErr := financial.Normalize(applyResult.MappedLineItems, financial.NormalizeOptions{Currency: "USD", IncludeProvenance: true})
	if normErr != nil {
		t.Fatalf("Normalize failed: %v", normErr)
	}
	if string(marshalForSnapshot(t, applyResult.MappedLineItems)) != string(mappedSnapshot) {
		t.Fatal("Normalize mutated its input MappedLineItems")
	}
	assertMarshalsCleanly(t, dataset)

	// ---------------------------------------------------------------
	// Step 11: reconciliation + metrics.
	// ---------------------------------------------------------------
	reconResult := reconciliation.Run(dataset, reconciliation.Options{})
	if reconResult.HasFailures() {
		t.Fatalf("test setup error: expected a clean reconciliation, got failures: %+v", reconResult.Checks)
	}

	metricsResult := metrics.Calculate(dataset, metrics.Options{})
	snap2025, ok := metricsResult.SnapshotFor("FY2025")
	if !ok || !snap2025.SDE.Available || !snap2025.EBITDA.Available {
		t.Fatalf("test setup error: expected an available 2025 SDE/EBITDA snapshot, got %+v", snap2025)
	}
	assertMarshalsCleanly(t, metricsResult)

	// ---------------------------------------------------------------
	// Step 12a: fake AI adjustment suggestion, over the already-confirmed
	// dataset (source-bound — the amount must exactly match the row it
	// references).
	// ---------------------------------------------------------------
	var miscAmount2025 float64
	for _, item := range applyResult.MappedLineItems {
		if item.SourceID == miscRowID {
			miscAmount2025 = item.Values["FY2025"]
		}
	}
	if miscAmount2025 == 0 {
		t.Fatal("test setup error: expected a nonzero 2025 amount for the misc-reimbursements row")
	}

	candidateRows := []adjustmentsai.SourceRow{
		{RowID: miscRowID, Period: "FY2025", Label: "Misc Reimbursements", Code: financial.CodeOpexOther, StatementType: financial.StatementIncomeStatement, Amount: miscAmount2025},
	}
	fakeSuggester := &adjustmentsai.FakeSuggester{Response: adjustmentsai.Response{
		Provider: "fake", Model: "fake-1",
		Suggestions: []adjustmentsai.Suggestion{
			{SourceRowID: miscRowID, Period: "FY2025", AdjustmentType: "one_time_expense", Amount: miscAmount2025, Direction: adjustmentsai.DirectionIncreaseEarnings, Reason: "one-time relocation reimbursement, non-recurring"},
		},
	}}
	suggestReq := adjustmentsai.Request{Candidates: candidateRows, AllowedTypes: adjustmentsai.BuildAllowedTypes(adjustments.AllTypes())}
	suggestOutcome := adjustmentsai.SuggestAdjustments(context.Background(), suggestReq, fakeSuggester, adjustmentsai.Policy{Mode: adjustmentsai.ModeEnabled})
	if len(suggestOutcome.Valid) != 1 {
		t.Fatalf("expected 1 valid AI adjustment suggestion, got %d (rejected: %+v)", len(suggestOutcome.Valid), suggestOutcome.Rejected)
	}
	aiAdjustments := adjustmentsai.ToAdjustments(suggestOutcome.Valid, "")
	if aiAdjustments[0].Included {
		t.Fatal("expected the AI-suggested adjustment to start with Included == false")
	}

	// ---------------------------------------------------------------
	// Step 12b: adjustment review (review.Build/Apply again, this time
	// scoped to the adjustment).
	// ---------------------------------------------------------------
	adjPlan := Build(BuildInput{
		Adjustments:           aiAdjustments,
		RequiredAdjustmentIDs: adjustmentsai.RequiredAdjustmentIDs(aiAdjustments),
	}, DefaultPolicy())
	adjItem := mustFindItem(t, adjPlan, KindAdjustment, buildAdjustmentID(string(aiAdjustments[0].ID)))
	if !adjItem.Required {
		t.Fatal("expected the AI-suggested adjustment's review item to be Required")
	}

	adjSource := Source{Adjustments: aiAdjustments}
	beforeAcceptResult := Apply(adjSource, adjPlan, nil)
	beforeAcceptBridge := adjustments.Apply(snap2025, beforeAcceptResult.Adjustments)
	if beforeAcceptBridge.SDEBridge.NormalizedValue != snap2025.SDE.Value {
		t.Fatalf("expected normalized SDE to equal unadjusted SDE before review acceptance, got %v want %v", beforeAcceptBridge.SDEBridge.NormalizedValue, snap2025.SDE.Value)
	}

	adjDecisions := []Decision{{ItemID: adjItem.ID, Action: ActionAccept, Adjustment: &AdjustmentDecision{Included: true}}}
	adjApplyResult := Apply(adjSource, adjPlan, adjDecisions)
	if len(adjApplyResult.Invalid) != 0 {
		t.Fatalf("expected the adjustment accept decision to apply cleanly, got Invalid=%+v", adjApplyResult.Invalid)
	}

	// ---------------------------------------------------------------
	// Step 12c/13: deterministic adjustment application + readiness gate.
	// ---------------------------------------------------------------
	adjustmentResult := adjustments.Apply(snap2025, adjApplyResult.Adjustments)
	if len(adjustmentResult.Errors) != 0 {
		t.Fatalf("expected the confirmed adjustment set to apply cleanly, got Errors=%+v", adjustmentResult.Errors)
	}
	wantNormalizedSDE := snap2025.SDE.Value + miscAmount2025
	if adjustmentResult.SDEBridge.NormalizedValue != wantNormalizedSDE {
		t.Fatalf("expected normalized SDE to reflect the accepted add-back, got %v want %v", adjustmentResult.SDEBridge.NormalizedValue, wantNormalizedSDE)
	}
	assertMarshalsCleanly(t, adjustmentResult)

	finalAdjReadiness := EvaluateReadiness(adjPlan.Items)
	if finalAdjReadiness.State == ReadinessNotReady {
		t.Fatalf("expected an accepted (no longer unresolved-Required) AI adjustment to never force NOT_READY once decided, got: %+v", finalAdjReadiness.Reasons)
	}

	// ---------------------------------------------------------------
	// Step 14: maintainable earnings (using only the single available
	// normalized-SDE period this fixture has — latest_period).
	// ---------------------------------------------------------------
	earningsResult := earnings.Calculate([]earnings.Observation{
		{Period: "FY2025", Value: adjustmentResult.SDEBridge.NormalizedValue, PeriodType: earnings.PeriodTypeFiscalYear, Available: true},
	}, earnings.Options{Strategy: earnings.StrategyLatestPeriod})
	if !earningsResult.Available {
		t.Fatalf("test setup error: expected an available maintainable-earnings result, got %+v", earningsResult)
	}
	maintainableSDE := earningsResult.Value

	// ---------------------------------------------------------------
	// Step 15: settings resolution -> resolved valuation settings snapshot.
	// ---------------------------------------------------------------
	systemSettings := settings.Settings{SDEMultiple: settings.Float64(2.2), EBITDAMultiple: settings.Float64(3.0)}
	valuationSettings := settings.Settings{SDEMultiple: settings.Float64(2.5)} // valuation-scope override wins
	resolution := settings.Resolve(systemSettings, settings.Settings{}, settings.Settings{}, valuationSettings)
	resolvedSDEMultiple, ok := resolution.Values[settings.FieldKeySDEMultiple].(float64)
	if !ok {
		t.Fatalf("test setup error: expected a resolved SDE multiple, got Values=%+v", resolution.Values)
	}
	resolvedEBITDAMultiple, ok := resolution.Values[settings.FieldKeyEBITDAMultiple].(float64)
	if !ok {
		t.Fatalf("test setup error: expected a resolved EBITDA multiple, got Values=%+v", resolution.Values)
	}
	if resolvedSDEMultiple != 2.5 {
		t.Fatalf("expected the valuation-scope SDE multiple (2.5) to win over the system default (2.2), got %v", resolvedSDEMultiple)
	}

	// ---------------------------------------------------------------
	// Step 16: valuation methods via valuation/orchestrator.Execute.
	// ---------------------------------------------------------------
	prof := profile.Profile{
		OwnerOperated:  boolPtrE2E(true),
		AnnualRevenue:  floatPtrE2E(940000),
		AssetIntensity: floatPtrE2E(0.1),
	}
	applicabilityResults := applicability.Calculate(prof)

	run := orchestrator.Execute(orchestrator.Request{
		Resolution:    resolution,
		Applicability: &applicabilityResults,
		FilterPolicy:  applicability.PolicyIncludeAllEnabled,
		SDE:           &sde.Input{MaintainableSDE: maintainableSDE, Multiple: resolvedSDEMultiple},
		EBITDA: &ebitda.Input{
			MaintainableEBITDA: snap2025.EBITDA.Value, Multiple: resolvedEBITDAMultiple,
			EquityBridge: ebitda.EquityBridgeInput{Requested: true, ExcessCash: 40000, ShortTermDebt: 5000},
		},
		Capitalization: &capitalization.Input{MaintainableEarnings: maintainableSDE, CapitalizationRate: 0.32},
	})
	if len(run.Successful()) != 3 {
		t.Fatalf("expected all 3 supplied methods to succeed, got: %+v", run.Methods)
	}

	// ---------------------------------------------------------------
	// Step 16b/17: basis reconciliation + consensus.
	// ---------------------------------------------------------------
	consensusInputs := report.BuildConsensusInputs(run, map[valuation.Code]float64{
		valuation.CodeSDEMultiple:              1,
		valuation.CodeEBITDAMultiple:           1,
		valuation.CodeCapitalizationOfEarnings: 1,
	})
	consensusResult := consensus.Calculate(consensusInputs, consensus.Options{TargetBasis: valuation.ValueTypeEquity})
	if !consensusResult.Available {
		t.Fatalf("expected an available consensus result, got Errors=%+v", consensusResult.Errors)
	}
	if consensusResult.Basis != valuation.ValueTypeEquity {
		t.Fatalf("expected every included method to share the equity-value consensus basis, got %s", consensusResult.Basis)
	}
	for _, incl := range consensusResult.Included {
		if incl.ValueType != valuation.ValueTypeEquity {
			t.Errorf("expected every consensus.Result.Included entry to be on the equity basis after conversion, got method=%s valueType=%s", incl.Method, incl.ValueType)
		}
	}
	assertMarshalsCleanly(t, consensusResult)

	// ---------------------------------------------------------------
	// Step 17b: sensitivity (independent, caller-supplied scenarios).
	// ---------------------------------------------------------------
	multipleSensitivity := sensitivity.MultipleSensitivity(maintainableSDE, []float64{2.0, 2.5, 3.0})

	// ---------------------------------------------------------------
	// Step 17c: report.Build.
	// ---------------------------------------------------------------
	rpt := report.Build(report.BuildInput{
		ValuationDate:       "2026-09-22",
		Snapshots:           []metrics.Snapshot{snap2025},
		NormalizedEBITDA:    report.FinancialFigure{Available: true, Value: snap2025.EBITDA.Value},
		NormalizedSDE:       report.FinancialFigure{Available: true, Value: adjustmentResult.SDEBridge.NormalizedValue},
		AdjustmentsSDE:      &adjustmentResult,
		Run:                 &run,
		Consensus:           &consensusResult,
		MultipleSensitivity: &multipleSensitivity,
	})

	// ---------------------------------------------------------------
	// Step 18: JSON serialization ("main app persists result").
	// ---------------------------------------------------------------
	reportJSON, err := json.Marshal(rpt)
	if err != nil {
		t.Fatalf("json.Marshal(report.Report) failed: %v", err)
	}
	if !json.Valid(reportJSON) {
		t.Fatal("expected valid final report JSON")
	}
	var roundTripped report.Report
	if err := json.Unmarshal(reportJSON, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal(report.Report) failed: %v", err)
	}
	reportJSON2, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(reportJSON) != string(reportJSON2) {
		t.Fatal("report.Report did not round-trip byte-for-byte")
	}

	// ---------------------------------------------------------------
	// Final assertions (section 20's explicit checklist).
	// ---------------------------------------------------------------
	if rpt.SchemaVersion == "" {
		t.Error("expected a populated report.SchemaVersion")
	}
	if plan.Version == "" {
		t.Error("expected a populated review.Plan.Version")
	}
	if metricsResult.FormulaVersion == "" {
		t.Error("expected a populated metrics.Result.FormulaVersion")
	}
	if adjustmentResult.SemanticsVersion == "" {
		t.Error("expected a populated adjustments.Result.SemanticsVersion")
	}
	if applicabilityResults.RulesVersion == "" {
		t.Error("expected a populated applicability.Results.RulesVersion")
	}
	if consensusResult.FormulaVersion == "" {
		t.Error("expected a populated consensus.Result.FormulaVersion")
	}
	if resolution.SchemaVersion == "" {
		t.Error("expected a populated settings.Resolution.SchemaVersion")
	}
	for _, mo := range run.Methods {
		switch {
		case mo.SDE != nil && mo.SDE.MethodVersion == "":
			t.Errorf("expected a populated MethodVersion for method %s", mo.Method)
		case mo.EBITDA != nil && mo.EBITDA.MethodVersion == "":
			t.Errorf("expected a populated MethodVersion for method %s", mo.Method)
		case mo.Capitalization != nil && mo.Capitalization.MethodVersion == "":
			t.Errorf("expected a populated MethodVersion for method %s", mo.Method)
		}
	}
	var aiProvenanceFound bool
	for _, o := range batchOutcome.Outcomes {
		if o.Result.Source == classification.SourceAI {
			if o.Provenance == nil {
				t.Error("expected a non-nil ai.Provenance for the AI-sourced classification outcome")
			} else {
				aiProvenanceFound = true
				if o.Provenance.OrchestrationVersion == "" || o.Provenance.RequestSchemaVersion == "" {
					t.Error("expected populated AI provenance version fields")
				}
			}
		}
	}
	if !aiProvenanceFound {
		t.Error("expected AI classification provenance to survive into the final outcome")
	}

	finalReadiness := EvaluateReadiness(applyForReadiness(plan, decisions))
	if finalReadiness.State == ReadinessNotReady {
		t.Fatalf("expected no unresolved blocking review item to remain, got reasons: %+v", finalReadiness.Reasons)
	}

	if string(sourceBytes) != string(sourceSnapshot) {
		t.Fatal("source document bytes were mutated somewhere in the pipeline")
	}
}

// --- test-local helpers ---

func marshalForSnapshot(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalForSnapshot: %v", err)
	}
	return b
}

// toMappedLineItems converts deterministic/AI-fallback classification
// results into financial.MappedLineItem values, exactly as a real caller's
// glue code would (Result.ToMappedLineItem), so review.Apply has a
// realistic Source to correct.
func toMappedLineItems(t *testing.T, raws []financial.RawLineItem, results []classification.Result) []financial.MappedLineItem {
	t.Helper()
	out := make([]financial.MappedLineItem, len(raws))
	for i, r := range results {
		out[i] = r.ToMappedLineItem(raws[i])
	}
	return out
}

// applyForReadiness re-evaluates readiness against a plan whose items have
// been marked resolved by the given decisions, mirroring how a caller
// would check readiness AFTER Apply — review.EvaluateReadiness operates on
// []ReviewItem, not on ApplyResult directly, so this rebuilds the
// post-decision item Status the same way Apply's own bookkeeping would.
func applyForReadiness(plan Plan, decisions []Decision) []ReviewItem {
	decided := make(map[string]bool, len(decisions))
	for _, d := range decisions {
		decided[d.ItemID] = true
	}
	items := make([]ReviewItem, len(plan.Items))
	for i, it := range plan.Items {
		if decided[it.ID] {
			it.Status = StatusResolved
		}
		items[i] = it
	}
	return items
}

func assertMarshalsCleanly(t *testing.T, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("assertMarshalsCleanly: json.Marshal failed (would indicate NaN/Inf or another encoding problem): %v", err)
	}
	if !json.Valid(b) {
		t.Fatal("assertMarshalsCleanly: produced invalid JSON")
	}
}

func boolPtrE2E(v bool) *bool        { return &v }
func floatPtrE2E(v float64) *float64 { return &v }
