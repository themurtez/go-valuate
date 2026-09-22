package review

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/classification"
	"github.com/themurtez/go-valuate/financial/earnings"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/valuation/sde"
)

// TestEndToEnd_RawInputToValuation_BlockedUntilReviewResolved is this
// package's complete end-to-end test, per section 21 of the task brief:
//
//	raw financial input (hand-built RawLineItems, ingestion-shaped)
//	  -> classification.ClassifyBatch
//	  -> review.Build
//	  -> review.Apply is attempted with an UNRESOLVED blocking item
//	     (valuation must not proceed — asserted directly)
//	  -> caller decisions resolve the blocking item
//	  -> review.Apply again, now fully resolved
//	  -> financial.Normalize
//	  -> financial/metrics.Calculate
//	  -> financial/adjustments.Apply (normalized SDE bridge)
//	  -> financial/earnings.Calculate (maintainable SDE across periods)
//	  -> valuation/sde.Calculate
//
// This lives in review's own package (not a second e2e-style package)
// since review is now itself one more pipeline stage most packages don't
// need to know about, and this is the one test allowed to reach all the
// way from raw rows to a priced valuation.
func TestEndToEnd_RawInputToValuation_BlockedUntilReviewResolved(t *testing.T) {
	// --- Raw financial input (two fiscal years, deliberately including one
	// row with a genuinely ambiguous label that classification cannot
	// confidently resolve on its own).
	raws := []financial.RawLineItem{
		{ID: "r1-2024", Label: "Product Sales", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2024": 500000}},
		{ID: "r1-2025", Label: "Product Sales", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 560000}},
		{ID: "r2-2024", Label: "Materials", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2024": 180000}},
		{ID: "r2-2025", Label: "Materials", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 195000}},
		{ID: "r3-2024", Label: "Owner Compensation", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2024": 90000}},
		{ID: "r3-2025", Label: "Owner Compensation", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 95000}},
		{ID: "r4-2024", Label: "Miscellaneous Bank Charges", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2024": 40000}},
		{ID: "r4-2025", Label: "Miscellaneous Bank Charges", StatementType: financial.StatementIncomeStatement, Values: map[financial.Period]float64{"2025": 42000}},
	}

	cfg := classification.Config{
		AliasLayers: []classification.AliasLayer{{Name: "global", Aliases: []classification.Alias{
			{Label: "Product Sales", Code: financial.CodeRevProduct},
			{Label: "Materials", Code: financial.CodeCogsMaterial},
			{Label: "Owner Compensation", Code: financial.CodeOpexOwnerComp},
			// "Miscellaneous Bank Charges" deliberately has NO alias and no strong rule
			// match, so it becomes this test's UNKNOWN/low-confidence
			// blocking item.
		}}},
		Rules: classification.DefaultRules(),
	}
	results := classification.ClassifyBatch(raws, cfg)

	var unknownExists bool
	for _, r := range results {
		if r.IsUnknown() {
			unknownExists = true
		}
	}
	if !unknownExists {
		t.Fatal("test setup error: expected at least one UNKNOWN classification (the 'Miscellaneous Bank Charges' rows) to exercise the blocking-readiness assertion")
	}

	plan := Build(BuildInput{Classifications: results, Raws: raws}, DefaultPolicy())

	mapped := make([]financial.MappedLineItem, len(results))
	for i, r := range results {
		mapped[i] = r.ToMappedLineItem(raws[i])
	}
	source := Source{MappedLineItems: mapped}

	// --- Step 1: attempt to Apply with NO decisions at all. The blocking
	// item(s) remain unresolved, and readiness must report NOT_READY.
	applyBeforeDecisions := Apply(source, plan, nil)
	readinessBeforeDecisions := EvaluateReadiness(applyBeforeDecisions.Items)
	if readinessBeforeDecisions.State != ReadinessNotReady {
		t.Fatalf("expected NOT_READY before the UNKNOWN classification is resolved, got %s (reasons: %+v)",
			readinessBeforeDecisions.State, readinessBeforeDecisions.Reasons)
	}
	if len(applyBeforeDecisions.UnresolvedRequired) == 0 {
		t.Fatal("expected at least one UnresolvedRequired item before decisions are supplied")
	}

	// Assert the pipeline is genuinely blocked: a caller respecting this
	// gate does not proceed to Normalize/valuation at this point. We
	// enforce that assertion here directly rather than merely documenting
	// it, by checking the gate function itself, which is exactly what a
	// real caller is expected to check before continuing.
	if readinessBeforeDecisions.State == ReadinessReady || readinessBeforeDecisions.State == ReadinessReadyWithWarnings {
		t.Fatal("valuation must not be considered proceedable while a blocking item is unresolved")
	}

	// --- Step 2: the reviewer resolves every classification item (override
	// the UNKNOWN "Miscellaneous Bank Charges" rows to a real code; accept everything
	// else already correctly classified).
	var decisions []Decision
	for _, item := range plan.Items {
		if item.Kind != KindClassification {
			continue
		}
		if item.Classification.ProposedCode == "" {
			decisions = append(decisions, Decision{
				ItemID: item.ID, Action: ActionOverride,
				Classification: &ClassificationDecision{Code: financial.CodeOpexOther},
			})
		} else {
			decisions = append(decisions, Decision{ItemID: item.ID, Action: ActionAccept})
		}
	}

	applyResult := Apply(source, plan, decisions)
	if len(applyResult.UnresolvedRequired) != 0 {
		t.Fatalf("expected zero unresolved required items after decisions, got %d: %+v", len(applyResult.UnresolvedRequired), applyResult.UnresolvedRequired)
	}
	readinessAfterDecisions := EvaluateReadiness(applyResult.Items)
	if readinessAfterDecisions.State == ReadinessNotReady {
		t.Fatalf("expected readiness to clear NOT_READY once every blocking item is resolved, got reasons: %+v", readinessAfterDecisions.Reasons)
	}

	// --- Step 3: Normalize the corrected MappedLineItems.
	dataset, err := financial.Normalize(applyResult.MappedLineItems, financial.NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("Normalize failed on corrected data: %v", err)
	}

	// --- Step 4: financial/metrics.Calculate per period.
	metricsResult := metrics.Calculate(dataset, metrics.Options{})
	snap2024, ok2024 := metricsResult.SnapshotFor("2024")
	if !ok2024 {
		t.Fatal("expected a metrics snapshot for period 2024")
	}
	snap2025, ok2025 := metricsResult.SnapshotFor("2025")
	if !ok2025 {
		t.Fatal("expected a metrics snapshot for period 2025")
	}
	if !snap2025.SDE.Available {
		t.Fatal("expected SDE to be available for 2025 after normalization")
	}

	// --- Step 5: financial/adjustments.Apply — a normalized SDE bridge
	// confirmed via this test's own hand-built adjustment (kept simple:
	// one owner-compensation normalization for 2025).
	adjs := []adjustments.Adjustment{
		{
			ID: "adj-1", Period: "2025", Type: adjustments.TypeOwnerCompensationNormalization,
			Amount: 20000, Effect: adjustments.EffectDecrease, Targets: []adjustments.Target{adjustments.TargetSDE},
			Reason: "normalize actual owner draw to market-rate replacement", Included: true,
		},
	}
	adjResult2025 := adjustments.Apply(snap2025, adjs)
	if !adjResult2025.SDEBridge.BaseAvailable {
		t.Fatal("expected SDE bridge base to be available for 2025")
	}

	adjResult2024 := adjustments.Apply(snap2024, nil)

	// --- Step 6: financial/earnings.Calculate — maintainable SDE across
	// both periods.
	observations := []earnings.Observation{
		{Period: "2024", PeriodType: earnings.PeriodTypeFiscalYear, Value: adjResult2024.SDEBridge.NormalizedValue, Available: adjResult2024.SDEBridge.BaseAvailable},
		{Period: "2025", PeriodType: earnings.PeriodTypeFiscalYear, Value: adjResult2025.SDEBridge.NormalizedValue, Available: adjResult2025.SDEBridge.BaseAvailable},
	}
	earningsResult := earnings.Calculate(observations, earnings.Options{Strategy: earnings.StrategySimpleAverage})
	if !earningsResult.Available {
		t.Fatalf("expected maintainable SDE to be available, got errors: %+v", earningsResult.Errors)
	}

	// --- Step 7: valuation/sde.Calculate — the priced result.
	sdeResult := sde.Calculate(sde.Input{MaintainableSDE: earningsResult.Value, Multiple: 2.5})
	if !sdeResult.Available {
		t.Fatalf("expected the SDE valuation to be Available, got errors: %+v", sdeResult.Errors)
	}
	if sdeResult.EquityValue <= 0 {
		t.Errorf("expected a positive equity value for this profitable business, got %v", sdeResult.EquityValue)
	}

	t.Logf("end-to-end result: maintainable SDE=%.2f, equity value=%.2f", earningsResult.Value, sdeResult.EquityValue)
}
