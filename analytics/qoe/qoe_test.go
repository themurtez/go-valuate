package qoe

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// TestHasErrors reflects both branches of HasErrors: an all-warning Issue
// slice reports no errors, and a slice carrying at least one SeverityError
// entry does.
func TestHasErrors(t *testing.T) {
	if HasErrors(nil) {
		t.Error("HasErrors(nil) should be false")
	}
	warningsOnly := []Issue{{Code: IssueNoPeriodMeta, Severity: SeverityWarning, Message: "advisory only"}}
	if HasErrors(warningsOnly) {
		t.Error("HasErrors should be false when every Issue is SeverityWarning")
	}
	withError := append(warningsOnly, Issue{Code: IssueNoPeriods, Severity: SeverityError, Message: "fatal"})
	if !HasErrors(withError) {
		t.Error("HasErrors should be true when at least one Issue is SeverityError")
	}
}

// TestCalculate_NoPeriods_Unavailable proves an empty dataset produces
// Available == false with a structured error, not a panic or a silently
// empty-but-Available Result.
func TestCalculate_NoPeriods_Unavailable(t *testing.T) {
	res := Calculate(Input{Dataset: financial.FinancialDataset{Currency: "USD"}}, Options{})
	if res.Available {
		t.Fatal("expected Available == false for an empty dataset")
	}
	if !HasErrors(res.Errors) {
		t.Errorf("expected an error Issue, got %+v", res.Errors)
	}
	if res.Errors[0].Code != IssueNoPeriods {
		t.Errorf("Errors[0].Code = %q, want %q", res.Errors[0].Code, IssueNoPeriods)
	}
	if res.FormulaVersion != FormulaVersion {
		t.Errorf("FormulaVersion = %q, want %q", res.FormulaVersion, FormulaVersion)
	}
}

// TestCalculate_CleanStableBusiness exercises the manufacturer fixture
// (steady ~6% revenue growth, no owner-operator so SDE == EBITDA, minimal
// adjustments) end to end: History is populated per period, reported ==
// normalized when no adjustments are supplied, and no critical flags fire.
func TestCalculate_CleanStableBusiness(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_manufacturer_multi_year.json")
	meta := threeYearMeta()

	res := runQoE(t, ds, meta, nil, Options{ComputeScore: true})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.History) != 3 {
		t.Fatalf("expected 3 periods of history, got %d", len(res.History))
	}
	wantOrder := []financial.Period{"2023", "2024", "2025"}
	for i, pf := range res.History {
		if pf.Period != wantOrder[i] {
			t.Errorf("History[%d].Period = %q, want %q (chronological order)", i, pf.Period, wantOrder[i])
		}
		if pf.ReportedEBITDA.Available && pf.NormalizedEBITDA.Value != pf.ReportedEBITDA.Value {
			t.Errorf("period %s: with no adjustments, normalized EBITDA (%v) should equal reported (%v)", pf.Period, pf.NormalizedEBITDA.Value, pf.ReportedEBITDA.Value)
		}
	}

	if res.Adjustments.TotalEBITDAAdjustment != 0 || res.Adjustments.TotalSDEAdjustment != 0 {
		t.Errorf("expected zero adjustment totals with no adjustments supplied, got %+v", res.Adjustments)
	}
	if len(res.Recurrence) != 0 {
		t.Errorf("expected no recurrence patterns with no adjustments, got %+v", res.Recurrence)
	}

	for _, f := range res.Flags {
		if f.Severity == FlagSeverityCritical {
			t.Errorf("clean stable business should not trigger a critical flag, got %+v", f)
		}
	}
	if res.Score == nil {
		t.Fatal("expected Score to be populated (ComputeScore: true)")
	}
	if res.Score.Value < 65 {
		t.Errorf("clean stable business should score at least 'moderate quality' (>=65), got %v (%s)", res.Score.Value, res.Score.Label)
	}
}

// TestCalculate_HighAdjustmentBurden builds a dataset where confirmed
// adjustments are a large fraction of reported EBITDA, and asserts
// FlagLargeNormalizationBurden fires with the exact computed ratio.
func TestCalculate_HighAdjustmentBurden(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")
	meta := threeYearMeta()

	// 2025 reported EBITDA for the HVAC fixture: confirm via a first pass,
	// then build an adjustment set intentionally sized to exceed the
	// default 30% burden threshold.
	base := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	last := base.History[len(base.History)-1]
	if !last.ReportedEBITDA.Available {
		t.Fatal("expected reported EBITDA to be available for the HVAC fixture's last period")
	}
	bigAmount := last.ReportedEBITDA.Value * 0.5 // 50% of reported EBITDA

	adjs := []adjustments.Adjustment{
		{
			ID: "big-one-time", Period: last.Period, Type: adjustments.TypeOneTimeExpense,
			Amount: bigAmount, Reason: "large one-time write-off", Included: true,
		},
	}

	res := runQoE(t, ds, meta, adjs, Options{})
	if !res.Ratios.AdjustmentToEBITDA.Available {
		t.Fatal("expected AdjustmentToEBITDA to be available")
	}
	wantRatio := bigAmount / last.ReportedEBITDA.Value
	if diff := res.Ratios.AdjustmentToEBITDA.Value - wantRatio; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("AdjustmentToEBITDA = %v, want %v", res.Ratios.AdjustmentToEBITDA.Value, wantRatio)
	}

	found := false
	for _, f := range res.Flags {
		if f.Code == FlagLargeNormalizationBurden {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagLargeNormalizationBurden to fire, got flags: %+v", res.Flags)
	}
}

// TestCalculate_NoAdjustments proves the zero-adjustments path: normalized
// figures equal reported figures at every period, AdjustmentBreakdown is
// zero, and no adjustment-driven flags fire.
func TestCalculate_NoAdjustments(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_saas_multi_year.json")
	meta := threeYearMeta()

	res := runQoE(t, ds, meta, nil, Options{})
	for _, pf := range res.History {
		if pf.ReportedEBITDA.Available != pf.NormalizedEBITDA.Available {
			t.Errorf("period %s: reported/normalized EBITDA availability mismatch", pf.Period)
		}
		if pf.ReportedEBITDA.Available && pf.ReportedEBITDA.Value != pf.NormalizedEBITDA.Value {
			t.Errorf("period %s: reported EBITDA %v != normalized EBITDA %v with no adjustments", pf.Period, pf.ReportedEBITDA.Value, pf.NormalizedEBITDA.Value)
		}
	}
	for _, f := range res.Flags {
		switch f.Code {
		case FlagLargeNormalizationBurden, FlagLargeOwnerDiscretionaryComponent, FlagRepeatedOneTimeAdjustments, FlagNonOperatingIncomeSupportingEarnings:
			t.Errorf("did not expect adjustment-driven flag %s with zero adjustments", f.Code)
		}
	}
}

// TestCalculate_MissingPeriods proves Calculate does not fail or panic when
// a period referenced by an Adjustment does not exist in Dataset (it is
// silently skipped per-period by adjustments.Apply's own SkipWrongPeriod,
// exactly as it would for any other caller of that package) and when
// PeriodMeta omits some of Dataset's periods (order falls back to dataset
// order for those periods rather than guessing).
func TestCalculate_MissingPeriods(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_agency_multi_year.json")

	adjs := []adjustments.Adjustment{
		{ID: "orphan", Period: "2099", Type: adjustments.TypeOneTimeExpense, Amount: 1000, Reason: "period not in dataset", Included: true},
	}

	// No PeriodMeta at all: every trend-dependent output should be empty,
	// not erroring.
	res := Calculate(Input{Dataset: ds, Adjustments: adjs}, Options{})
	if !res.Available {
		t.Fatalf("expected Available even with no PeriodMeta, errors=%+v", res.Errors)
	}
	if len(res.History) != 3 {
		t.Fatalf("expected 3 periods of history regardless of PeriodMeta, got %d", len(res.History))
	}
	if len(res.RevenueGrowth) != 0 {
		t.Errorf("expected no RevenueGrowth without PeriodMeta, got %+v", res.RevenueGrowth)
	}
	foundWarning := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoPeriodMeta {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected IssueNoPeriodMeta warning, got %+v", res.Warnings)
	}

	// Partial PeriodMeta (missing "2024"): chronologicalPeriods should
	// fall back to dataset order rather than a partial/guessed sort, and
	// trend calculations (which go through financial/metrics' own
	// resolvePeriodOrder) should report their own PeriodOrderError instead
	// of silently using a partial order.
	full := threeYearMeta()
	partial := map[financial.Period]metrics.PeriodInfo{
		"2023": full["2023"],
		"2025": full["2025"],
	}
	resPartial := Calculate(Input{Dataset: ds, PeriodMeta: partial}, Options{})
	if !resPartial.Available {
		t.Fatalf("expected Available even with partial PeriodMeta, errors=%+v", resPartial.Errors)
	}
	if len(resPartial.History) != 3 {
		t.Fatalf("expected 3 periods of history with partial PeriodMeta, got %d", len(resPartial.History))
	}
	// History order falls back to Dataset.Periods()'s lexical order
	// ("2023", "2024", "2025") since chronologicalPeriods refuses to
	// guess a partial sort — which happens to already be chronological
	// for this fixture's period labels, so this also confirms no panic
	// or corrupted ordering occurred.
	wantFallbackOrder := []financial.Period{"2023", "2024", "2025"}
	for i, pf := range resPartial.History {
		if pf.Period != wantFallbackOrder[i] {
			t.Errorf("History[%d].Period = %q, want %q", i, pf.Period, wantFallbackOrder[i])
		}
	}
	if len(resPartial.RevenueGrowth) != 0 {
		t.Errorf("expected no RevenueGrowth when PeriodMeta is missing an entry (metrics.Trend.Error), got %+v", resPartial.RevenueGrowth)
	}
}
