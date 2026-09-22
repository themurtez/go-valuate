// Command full_flow is a runnable, narrated demonstration of the complete
// V1 integration flow documented in docs/V1_CONTRACTS.md and
// docs/INTEGRATION.md — from a source document's raw bytes through a
// finished, JSON-serializable valuation report.
//
// This program uses only synthetic fixture data (a hand-written CSV
// income statement) and fake providers for both optional AI capabilities
// (financial/classification/ai.FakeClassifier,
// financial/adjustments/ai.FakeSuggester) — it makes no network calls,
// requires no OpenAI API key, and requires no Tesseract installation. It
// exists to be READ by an engineer wiring this library into a larger
// application, not just run: each stage below is commented with what a
// real application would typically do differently (persist the
// intermediate result, collect a real human decision instead of
// auto-accepting, etc.) — see the inline "In a real application" notes.
//
// Run it with:
//
//	go run ./examples/full_flow
//
// See review/e2e_v1_contract_test.go for the same flow expressed as a Go
// test with explicit assertions (no-mutation checks, version-population
// checks, etc.) at every stage — this program demonstrates the identical
// sequence of real library calls, but is written to be read top to bottom
// by a human, with narration, rather than executed by `go test`.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

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
	"github.com/themurtez/go-valuate/review"
	"github.com/themurtez/go-valuate/settings"
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/applicability"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/ebitda"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
	"github.com/themurtez/go-valuate/valuation/profile"
	"github.com/themurtez/go-valuate/valuation/report"
	"github.com/themurtez/go-valuate/valuation/sde"
	"github.com/themurtez/go-valuate/valuation/sensitivity"
)

func main() {
	// =====================================================================
	// STEP 1-2: main app receives file bytes, selects a parser.
	//
	// In a real application: the file arrives as an HTTP upload or from
	// object storage; the parser is chosen from the file's content type or
	// extension (.csv -> ingestion/csv, .xlsx -> ingestion/xlsx, .pdf ->
	// ingestion/pdf). This library never reads a file from disk itself —
	// it only ever accepts an io.Reader/[]byte the caller already has.
	// =====================================================================
	sourceDocument := []byte(`Income Statement,FY2024,FY2025
Product Sales,850000,940000
Materials,310000,340000
Total COGS,310000,340000
Gross Profit,540000,600000
Owner Compensation,120000,128000
Payroll,180000,196000
Misc Reimbursements,38000,41000
Total Operating Expenses,338000,365000
Net Income,202000,235000
`)
	fmt.Println("=== Step 1-2: source document received ===")
	fmt.Printf("%d bytes, selected parser: ingestion/csv\n\n", len(sourceDocument))

	// =====================================================================
	// STEP 3-4: ingestion.
	//
	// In a real application: this stage is stateless/pure. The main app
	// typically persists the raw ingestion.Result (or at least its
	// Warnings) alongside the source document metadata, for audit purposes
	// — see docs/INTEGRATION.md's Persistence snapshot guidance.
	// =====================================================================
	fmt.Println("=== Step 3-4: ingestion ===")
	ingestResult, ingestErr := csv.Parse(strings.NewReader(string(sourceDocument)), ingestion.Options{})
	if ingestErr != nil {
		log.Fatalf("ingestion failed: %+v", ingestErr)
	}
	for _, w := range ingestResult.Warnings {
		fmt.Printf("  warning: %s — %s\n", w.Code, w.Message)
	}
	rawLineItems := ingestResult.ToRawLineItems()
	fmt.Printf("parsed %d raw line items\n\n", len(rawLineItems))

	// =====================================================================
	// STEP 5: deterministic classification.
	//
	// In a real application: Config.AliasLayers is typically assembled
	// from the main app's own database (a global default layer, plus
	// account/client/valuation-specific overrides the user has previously
	// confirmed) — this library has no concept of where aliases come from.
	// =====================================================================
	fmt.Println("=== Step 5: deterministic classification ===")
	classifyConfig := classification.Config{
		AliasLayers: []classification.AliasLayer{{Name: "global", Aliases: []classification.Alias{
			{Label: "Product Sales", Code: financial.CodeRevProduct},
			{Label: "Materials", Code: financial.CodeCogsMaterial},
			{Label: "Owner Compensation", Code: financial.CodeOpexOwnerComp},
			{Label: "Payroll", Code: financial.CodeOpexPayroll},
			// "Misc Reimbursements" deliberately has no alias and no
			// strong rule match, so the deterministic pipeline leaves it
			// UNKNOWN — this is the row the AI fallback below resolves.
		}}},
		Rules: classification.DefaultRules(),
	}
	deterministicResults := classification.ClassifyBatch(rawLineItems, classifyConfig)
	var unknownLabel, unknownRowID string
	for i, r := range deterministicResults {
		if r.IsUnknown() {
			unknownLabel, unknownRowID = rawLineItems[i].Label, rawLineItems[i].ID
			fmt.Printf("  UNKNOWN: %q (row %s) — deterministic classification found no confident match\n", unknownLabel, unknownRowID)
		}
	}
	fmt.Println()

	// =====================================================================
	// STEP 6: optional AI fallback classification.
	//
	// In a real application: replace FakeClassifier with
	// financial/classification/ai/openai.New(openai.Config{APIKey: ...}).
	// The API key belongs to the main app's own config/secrets layer,
	// never hardcoded here — see docs/INTEGRATION.md's security notes.
	// This whole step is entirely optional: a caller that never wants AI
	// assistance simply never calls ClassifyBatchWithFallback and treats
	// every UNKNOWN row as a manual review item instead.
	// =====================================================================
	fmt.Println("=== Step 6: optional AI fallback classification (fake provider) ===")
	fakeClassifier := classificationai.NewFakeClassifier()
	rawConfidence := 0.62
	fakeClassifier.Responses[unknownLabel] = classificationai.Response{
		Code:          financial.CodeOpexOther,
		RawConfidence: &rawConfidence,
		Reason:        "reimbursed miscellaneous business expenses; closest fit is other operating expense",
	}
	aiPolicy := classificationai.DefaultPolicy()
	aiPolicy.Mode = classificationai.AIBelowConfidence // AI is opt-in; AIDisabled is the true default
	aiPolicy.ConfidenceThreshold = 0.90

	fallbackOutcome := classificationai.ClassifyBatchWithFallback(
		context.Background(), rawLineItems, classifyConfig, fakeClassifier, aiPolicy, nil,
	)
	classificationResults := make([]classification.Result, len(fallbackOutcome.Outcomes))
	for i, o := range fallbackOutcome.Outcomes {
		classificationResults[i] = o.Result
		if o.Result.Source == classification.SourceAI {
			fmt.Printf("  AI suggested %s for row %s (ReviewRequired=%v, never auto-trusted)\n", o.Result.Code, unknownRowID, o.Result.ReviewRequired)
		}
	}
	fmt.Println()

	// =====================================================================
	// STEP 7: review.Build.
	//
	// In a real application: this Plan is what a review UI renders — every
	// field needed for a review screen (title, reason, severity, current/
	// proposed value) is already here. Nothing about severity thresholds
	// or ID schemes should be re-derived in a frontend.
	// =====================================================================
	fmt.Println("=== Step 7: review.Build ===")
	mappedLineItems := make([]financial.MappedLineItem, len(rawLineItems))
	for i, r := range classificationResults {
		mappedLineItems[i] = r.ToMappedLineItem(rawLineItems[i])
	}
	reviewPlan := review.Build(review.BuildInput{
		Classifications: classificationResults,
		Raws:            rawLineItems,
	}, review.DefaultPolicy())
	fmt.Printf("plan has %d review items; readiness before decisions: %s\n\n",
		len(reviewPlan.Items), review.EvaluateReadiness(reviewPlan.Items).State)

	// =====================================================================
	// STEP 8: UI/main app collects decisions from a human.
	//
	// In a real application: THIS is where a human is actually in the
	// loop — a reviewer looks at the Plan rendered by step 7 and submits
	// Decision values back (e.g. via an HTTP request from the review UI).
	// This program auto-accepts every item to stay non-interactive and
	// demonstrate the full flow end to end; a real integration must not
	// skip this step.
	// =====================================================================
	fmt.Println("=== Step 8: human review decisions (auto-accepted here for demonstration) ===")
	var decisions []review.Decision
	for _, item := range reviewPlan.Items {
		switch item.Kind {
		case review.KindClassification, review.KindStructure:
			decisions = append(decisions, review.Decision{ItemID: item.ID, Action: review.ActionAccept})
		}
	}
	fmt.Printf("collected %d decisions\n\n", len(decisions))

	// =====================================================================
	// STEP 9: review.Apply.
	// =====================================================================
	fmt.Println("=== Step 9: review.Apply ===")
	applyResult := review.Apply(review.Source{MappedLineItems: mappedLineItems}, reviewPlan, decisions)
	if len(applyResult.Invalid) != 0 {
		log.Fatalf("unexpected invalid decisions: %+v", applyResult.Invalid)
	}
	fmt.Printf("applied %d decisions cleanly\n\n", len(applyResult.Applied))

	// =====================================================================
	// STEP 10: financial.Normalize.
	// =====================================================================
	fmt.Println("=== Step 10: financial.Normalize ===")
	dataset, normErr := financial.Normalize(applyResult.MappedLineItems, financial.NormalizeOptions{Currency: "USD"})
	if normErr != nil {
		log.Fatalf("normalize failed: %v", normErr)
	}
	fmt.Printf("normalized dataset: %d items across %d periods\n\n", len(dataset.Items), len(dataset.Periods()))

	// =====================================================================
	// STEP 11: reconciliation + metrics.
	// =====================================================================
	fmt.Println("=== Step 11: reconciliation + metrics ===")
	reconResult := reconciliation.Run(dataset, reconciliation.Options{})
	fmt.Printf("reconciliation: %d checks, failures=%v\n", len(reconResult.Checks), reconResult.HasFailures())

	metricsResult := metrics.Calculate(dataset, metrics.Options{})
	snapshot, ok := metricsResult.SnapshotFor("FY2025")
	if !ok {
		log.Fatal("expected an FY2025 snapshot")
	}
	fmt.Printf("FY2025 EBITDA=%.2f (available=%v) SDE=%.2f (available=%v)\n\n",
		snapshot.EBITDA.Value, snapshot.EBITDA.Available, snapshot.SDE.Value, snapshot.SDE.Available)

	// =====================================================================
	// STEP 12: adjustments, including an optional AI-suggested add-back.
	//
	// In a real application: candidateRows would be built from the
	// caller's own confirmed dataset (post review.Apply), and
	// FakeSuggester would be replaced with
	// financial/adjustments/ai/openai.New(...) exactly as in step 6. The
	// AI suggestion again goes through review — it never applies itself.
	// =====================================================================
	fmt.Println("=== Step 12: adjustments (with an optional AI-suggested add-back) ===")
	var miscRowAmount float64
	for _, item := range applyResult.MappedLineItems {
		if item.SourceID == unknownRowID {
			miscRowAmount = item.Values["FY2025"]
		}
	}
	candidateRows := []adjustmentsai.SourceRow{
		{RowID: unknownRowID, Period: "FY2025", Label: unknownLabel, Code: financial.CodeOpexOther, StatementType: financial.StatementIncomeStatement, Amount: miscRowAmount},
	}
	fakeSuggester := &adjustmentsai.FakeSuggester{Response: adjustmentsai.Response{
		Provider: "fake", Model: "fake-1",
		Suggestions: []adjustmentsai.Suggestion{
			{SourceRowID: unknownRowID, Period: "FY2025", AdjustmentType: "one_time_expense", Amount: miscRowAmount, Direction: adjustmentsai.DirectionIncreaseEarnings, Reason: "one-time relocation reimbursement, non-recurring"},
		},
	}}
	suggestOutcome := adjustmentsai.SuggestAdjustments(
		context.Background(),
		adjustmentsai.Request{Candidates: candidateRows, AllowedTypes: adjustmentsai.BuildAllowedTypes(adjustments.AllTypes())},
		fakeSuggester,
		adjustmentsai.Policy{Mode: adjustmentsai.ModeEnabled},
	)
	aiAdjustments := adjustmentsai.ToAdjustments(suggestOutcome.Valid, "")
	fmt.Printf("AI proposed %d adjustment(s) (Included=false until reviewed)\n", len(aiAdjustments))

	// AI adjustment suggestions go through review exactly like AI
	// classification did in step 7-9 — a second, small Build/Apply pass
	// scoped to the adjustment.
	adjustmentPlan := review.Build(review.BuildInput{
		Adjustments:           aiAdjustments,
		RequiredAdjustmentIDs: adjustmentsai.RequiredAdjustmentIDs(aiAdjustments),
	}, review.DefaultPolicy())
	var adjustmentDecisions []review.Decision
	for _, item := range adjustmentPlan.Items {
		if item.Kind == review.KindAdjustment {
			adjustmentDecisions = append(adjustmentDecisions, review.Decision{
				ItemID: item.ID, Action: review.ActionAccept,
				Adjustment: &review.AdjustmentDecision{Included: true},
			})
		}
	}
	adjustmentApplyResult := review.Apply(review.Source{Adjustments: aiAdjustments}, adjustmentPlan, adjustmentDecisions)

	adjustmentResult := adjustments.Apply(snapshot, adjustmentApplyResult.Adjustments)
	fmt.Printf("normalized SDE after accepted add-back: %.2f (was %.2f)\n\n", adjustmentResult.SDEBridge.NormalizedValue, snapshot.SDE.Value)

	// =====================================================================
	// STEP 13: readiness gate.
	//
	// In a real application: a main app should not proceed to valuation
	// while State == NOT_READY. This fixture has no genuinely blocking
	// item, so this only demonstrates the check itself.
	// =====================================================================
	fmt.Println("=== Step 13: readiness gate ===")
	readiness := review.EvaluateReadiness(reviewPlan.Items)
	fmt.Printf("readiness: %s\n\n", readiness.State)

	// =====================================================================
	// STEP 14: maintainable earnings.
	// =====================================================================
	fmt.Println("=== Step 14: maintainable earnings ===")
	earningsResult := earnings.Calculate([]earnings.Observation{
		{Period: "FY2025", Value: adjustmentResult.SDEBridge.NormalizedValue, PeriodType: earnings.PeriodTypeFiscalYear, Available: true},
	}, earnings.Options{Strategy: earnings.StrategyLatestPeriod})
	maintainableSDE := earningsResult.Value
	fmt.Printf("maintainable SDE (latest_period strategy): %.2f\n\n", maintainableSDE)

	// =====================================================================
	// STEP 15: resolved valuation settings snapshot.
	//
	// In a real application: the four Settings layers come from the main
	// app's own settings store (system defaults, account overrides, client
	// overrides, this specific valuation's overrides). Resolve is called
	// ONCE per valuation run and the resulting Resolution is what gets
	// persisted and passed to the orchestrator — never re-resolved
	// mid-calculation. See docs/V1_CONTRACTS.md § 6.
	// =====================================================================
	fmt.Println("=== Step 15: settings resolution ===")
	resolution := settings.Resolve(
		settings.Settings{SDEMultiple: settings.Float64(2.2), EBITDAMultiple: settings.Float64(3.0)}, // system
		settings.Settings{}, // account
		settings.Settings{}, // client
		settings.Settings{SDEMultiple: settings.Float64(2.5)}, // valuation-specific override
	)
	resolvedSDEMultiple := resolution.Values[settings.FieldKeySDEMultiple].(float64)
	resolvedEBITDAMultiple := resolution.Values[settings.FieldKeyEBITDAMultiple].(float64)
	fmt.Printf("resolved SDE multiple: %.2fx (source: %s)\n", resolvedSDEMultiple, resolution.Sources[settings.FieldKeySDEMultiple])
	fmt.Printf("resolved EBITDA multiple: %.2fx (source: %s)\n\n", resolvedEBITDAMultiple, resolution.Sources[settings.FieldKeyEBITDAMultiple])

	// =====================================================================
	// STEP 16: valuation methods / orchestrator.
	// =====================================================================
	fmt.Println("=== Step 16: valuation methods via orchestrator.Execute ===")
	applicabilityResults := applicability.Calculate(profile.Profile{
		OwnerOperated:  ptr(true),
		AnnualRevenue:  ptr(940000.0),
		AssetIntensity: ptr(0.1),
	})
	run := orchestrator.Execute(orchestrator.Request{
		Resolution:    resolution,
		Applicability: &applicabilityResults,
		FilterPolicy:  applicability.PolicyIncludeAllEnabled,
		SDE:           &sde.Input{MaintainableSDE: maintainableSDE, Multiple: resolvedSDEMultiple},
		EBITDA: &ebitda.Input{
			MaintainableEBITDA: snapshot.EBITDA.Value, Multiple: resolvedEBITDAMultiple,
			EquityBridge: ebitda.EquityBridgeInput{Requested: true, ExcessCash: 40000, ShortTermDebt: 5000},
		},
	})
	for _, mo := range run.Methods {
		fmt.Printf("  %s: %s\n", mo.Method, mo.Outcome)
	}
	fmt.Println()

	// =====================================================================
	// STEP 16b: basis conversion + consensus.
	//
	// report.BuildConsensusInputs pre-populates each method's own already-
	// computed Bridge, so consensus.Calculate can convert the EBITDA
	// method's enterprise-value result onto the same equity basis as SDE's
	// native equity-value result, without the caller re-supplying cash/debt
	// figures a second time. See docs/V1_CONTRACTS.md's "Value basis and
	// conversion" reference.
	// =====================================================================
	fmt.Println("=== Step 16b: consensus (equity-value basis) ===")
	consensusInputs := report.BuildConsensusInputs(run, map[valuation.Code]float64{
		valuation.CodeSDEMultiple:    1,
		valuation.CodeEBITDAMultiple: 1,
	})
	consensusResult := consensus.Calculate(consensusInputs, consensus.Options{TargetBasis: valuation.ValueTypeEquity})
	fmt.Printf("consensus basis: %s, simple mean: %.2f, weighted mean: %.2f\n\n",
		consensusResult.Basis, consensusResult.Statistics.SimpleMean, consensusResult.Statistics.WeightedMean)

	// =====================================================================
	// STEP 17: sensitivity + report.
	// =====================================================================
	fmt.Println("=== Step 17: sensitivity + report.Build ===")
	multipleSensitivity := sensitivity.MultipleSensitivity(maintainableSDE, []float64{2.0, 2.5, 3.0})

	finalReport := report.Build(report.BuildInput{
		ValuationDate:       "2026-09-22",
		Snapshots:           []metrics.Snapshot{snapshot},
		NormalizedEBITDA:    report.FinancialFigure{Available: true, Value: snapshot.EBITDA.Value},
		NormalizedSDE:       report.FinancialFigure{Available: true, Value: adjustmentResult.SDEBridge.NormalizedValue},
		AdjustmentsSDE:      &adjustmentResult,
		Run:                 &run,
		Consensus:           &consensusResult,
		MultipleSensitivity: &multipleSensitivity,
	})
	fmt.Printf("report schema version: %s\n\n", finalReport.SchemaVersion)

	// =====================================================================
	// STEP 18: main app persists result.
	//
	// This library never persists anything itself — report.Report (and
	// every intermediate stage's output above) is a plain, JSON-
	// serializable Go value the main app writes to its own database. See
	// docs/INTEGRATION.md's Persistence snapshot guidance for the fuller
	// list of what a finalized valuation run should generally capture
	// beyond just the final report (raw line items, review decisions,
	// resolved settings snapshot, AI provenance, etc.).
	// =====================================================================
	fmt.Println("=== Step 18: persistence (main app's own responsibility) ===")
	reportJSON, err := json.MarshalIndent(finalReport, "", "  ")
	if err != nil {
		log.Fatalf("marshaling final report: %v", err)
	}
	fmt.Printf("final report: %d bytes of JSON, ready for the main app to persist\n", len(reportJSON))
	fmt.Printf("summary.simple_consensus=%.2f summary.weighted_consensus=%.2f methods=%d\n",
		finalReport.Summary.SimpleConsensus, finalReport.Summary.WeightedConsensus, len(finalReport.Methods))
}

func ptr[T any](v T) *T { return &v }
