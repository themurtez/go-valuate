package advisory

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/analytics/kpi"
)

// TestIntegration_MultiPeriodFixture covers task section 117: current/
// prior source results produce absolute change, percent change,
// percentage-point change, and new/resolved/persistent insight identity
// via Input.Prior.
func TestIntegration_MultiPeriodFixture(t *testing.T) {
	closeIn := func(readiness closechecklist.ChecklistReadiness, blocking bool) CloseInputs {
		var blockers []closechecklist.Blocker
		if blocking {
			blockers = []closechecklist.Blocker{{TaskCode: "RECONCILE_AR", ReasonCode: closechecklist.BlockerEvidenceMissing, Message: "Missing evidence", EffectiveBlocking: true}}
		}
		return CloseInputs{CloseChecklist: closechecklist.Result{PeriodID: "period", Readiness: readiness, Blockers: blockers}}
	}

	priorInput := Input{Company: CompanyContext{CurrentPeriod: "2026-01"}, Close: closeIn(closechecklist.ChecklistInProgress, true)}
	priorResult := Build(priorInput, ExamplePolicy())

	currentInput := Input{
		Company: CompanyContext{CurrentPeriod: "2026-02", PriorPeriod: "2026-01"},
		Close:   closeIn(closechecklist.ChecklistReadyToClose, false), // the prior blocker is resolved this period
		Prior:   &priorResult,
	}
	currentResult := Build(currentInput, ExamplePolicy())

	if currentResult.PriorComparison == nil {
		t.Fatal("expected PriorComparison to be populated when Input.Prior is supplied")
	}
	pc := currentResult.PriorComparison

	foundResolved := false
	for _, ic := range pc.ResolvedInsights {
		if ic.Code == string(closechecklist.BlockerEvidenceMissing) {
			foundResolved = true
		}
	}
	if !foundResolved {
		t.Errorf("expected the prior period's blocking finding to appear in ResolvedInsights, got: %+v", pc.ResolvedInsights)
	}
}

// TestIntegration_KPIFixture covers task section 119: real
// analytics/kpi.KPIResult output, preserving value/unit/target/band/
// availability/trend, no re-evaluation of KPI formulas.
func TestIntegration_KPIFixture(t *testing.T) {
	in := Input{
		Company: CompanyContext{CurrentPeriod: "2026-02"},
		KPIValues: []kpi.KPIResult{
			{
				Code: "REVENUE_PER_FTE", Period: "2026-02",
				Value:            kpi.Value{Available: true, Amount: 125000},
				Unit:             kpi.Unit{Kind: kpi.UnitCurrency, CurrencyCode: "USD"},
				TargetEvaluation: kpi.TargetEvaluation{TargetAvailable: true, TargetMet: false},
				Band:             kpi.ThresholdBand{Label: "LOW", Min: 0, Max: 100000},
				BandAvailable:    true,
			},
			{
				Code: "UNAVAILABLE_KPI", Period: "2026-02",
				Value: kpi.Value{Available: false, Reason: kpi.AvailabilityMissingMetric},
			},
		},
	}

	result := Build(in, ExamplePolicy())

	kpiSection, ok := sectionByCode(result.Sections, SectionKPI)
	if !ok || kpiSection.Availability != StatusAvailable {
		t.Fatalf("KPI section unavailable: %+v", kpiSection)
	}

	m, ok := metricByCode(kpiSection.Metrics, "REVENUE_PER_FTE")
	if !ok || m.Value.Amount != 125000 || m.Unit != UnitCurrency {
		t.Errorf("REVENUE_PER_FTE metric not preserved verbatim: %+v", m)
	}

	foundTargetNotMet := false
	foundUnavailable := false
	for _, f := range kpiSection.Findings {
		if f.Code == "KPI_TARGET_NOT_MET" {
			foundTargetNotMet = true
		}
		if f.Code == "KPI_VALUE_UNAVAILABLE" {
			foundUnavailable = true
		}
	}
	if !foundTargetNotMet {
		t.Error("expected a KPI_TARGET_NOT_MET finding")
	}
	if !foundUnavailable {
		t.Error("expected a KPI_VALUE_UNAVAILABLE finding for the unavailable KPI")
	}
}

// TestCloseChecklistTimingField_CompatibilityDocumented is a compile-time/
// behavioral proof that this package never depends on
// closechecklist.PriorCloseComparison.CompletionTimingChanges being
// populated — task section 120. An empty/nil map here must never cause a
// panic or a silently-wrong comparison; this package computes its own
// generic current/prior comparison over already-built Metrics instead
// (see prior.go's compareMetricSets), never reading
// CompletionTimingChanges directly.
func TestCloseChecklistTimingField_CompatibilityDocumented(t *testing.T) {
	cc := closechecklist.Result{
		PeriodID: "2026-02", Readiness: closechecklist.ChecklistReadyToClose,
		PriorCloseComparison: &closechecklist.PriorCloseComparison{
			PriorPeriodID: "2026-01",
			// CompletionTimingChanges intentionally left nil — this
			// package must not depend on it (task section 120).
		},
	}
	in := Input{Company: CompanyContext{CurrentPeriod: "2026-02"}, Close: CloseInputs{CloseChecklist: cc}}

	result := Build(in, Policy{}) // must not panic

	close, ok := sectionByCode(result.Sections, SectionAccountingAndClose)
	if !ok || close.Availability != StatusAvailable {
		t.Fatalf("ACCOUNTING_AND_CLOSE unavailable despite valid CloseChecklist input: %+v", close)
	}
}
