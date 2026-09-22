package qoe

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// buildDataset assembles a minimal financial.FinancialDataset from
// per-period (revenue, materialCOGS, payroll, ownerComp) figures — enough
// to drive metrics.Calculate's TotalRevenue/GrossProfit/EBIT/EBITDA/SDE
// formulas without needing a full realistic multi-statement fixture for
// every hand-built test scenario in this file. depreciation/amortization
// are omitted (metrics.ebitda treats them as 0 when absent, per that
// function's own doc comment), so EBITDA == EBIT here.
func buildDataset(periods []string, revenue, materialCOGS, payroll, ownerComp []float64) financial.FinancialDataset {
	var items []financial.NormalizedItem
	add := func(code financial.Code, values []float64) {
		for i, p := range periods {
			if values[i] == 0 {
				continue
			}
			items = append(items, financial.NormalizedItem{Code: code, Period: financial.Period(p), Amount: values[i]})
		}
	}
	add(financial.CodeRevProduct, revenue)
	add(financial.CodeCogsMaterial, materialCOGS)
	add(financial.CodeOpexPayroll, payroll)
	add(financial.CodeOpexOwnerComp, ownerComp)
	return financial.FinancialDataset{Currency: "USD", Items: items}
}

func metaFor(periods []string, startYear int) map[financial.Period]metrics.PeriodInfo {
	m := make(map[financial.Period]metrics.PeriodInfo, len(periods))
	for i, p := range periods {
		m[financial.Period(p)] = metrics.PeriodInfo{Type: metrics.PeriodTypeFiscalYear, FiscalYear: startYear + i}
	}
	return m
}

// TestRepeatedOneTimeAdjustments_FlagsAcrossYears builds a dataset with the
// same adjustments.Type (TypeOneTimeExpense) applied in three consecutive
// years and confirms buildRecurrence marks it LikelyNotNonRecurring and
// FlagRepeatedOneTimeAdjustments fires — the core "same category appears
// repeatedly" detection the task requires.
func TestRepeatedOneTimeAdjustments_FlagsAcrossYears(t *testing.T) {
	periods := []string{"2023", "2024", "2025"}
	ds := buildDataset(periods,
		[]float64{500000, 520000, 540000},
		[]float64{200000, 205000, 210000},
		[]float64{100000, 102000, 104000},
		[]float64{0, 0, 0},
	)
	meta := metaFor(periods, 2023)

	adjs := []adjustments.Adjustment{
		{ID: "ot-2023", Period: "2023", Type: adjustments.TypeOneTimeExpense, Amount: 8000, Reason: "flood damage repair", Included: true},
		{ID: "ot-2024", Period: "2024", Type: adjustments.TypeOneTimeExpense, Amount: 9000, Reason: "roof repair", Included: true},
		{ID: "ot-2025", Period: "2025", Type: adjustments.TypeOneTimeExpense, Amount: 7500, Reason: "equipment repair", Included: true},
	}

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta, Adjustments: adjs}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	var pattern *RecurrencePattern
	for i := range res.Recurrence {
		if res.Recurrence[i].Type == adjustments.TypeOneTimeExpense {
			pattern = &res.Recurrence[i]
		}
	}
	if pattern == nil {
		t.Fatalf("expected a RecurrencePattern for TypeOneTimeExpense, got %+v", res.Recurrence)
	}
	if pattern.Count != 3 {
		t.Errorf("Count = %d, want 3", pattern.Count)
	}
	if !pattern.LikelyNotNonRecurring {
		t.Error("expected LikelyNotNonRecurring == true when the same type appears in 3 separate periods")
	}
	wantTotal := 8000.0 + 9000.0 + 7500.0
	if pattern.TotalAmount != wantTotal {
		t.Errorf("TotalAmount = %v, want %v", pattern.TotalAmount, wantTotal)
	}
	wantPeriods := []financial.Period{"2023", "2024", "2025"}
	for i, p := range pattern.Periods {
		if p != wantPeriods[i] {
			t.Errorf("Periods[%d] = %q, want %q", i, p, wantPeriods[i])
		}
	}

	found := false
	for _, f := range res.Flags {
		if f.Code == FlagRepeatedOneTimeAdjustments {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagRepeatedOneTimeAdjustments to fire, got flags: %+v", res.Flags)
	}
}

// TestRepeatedOneTimeAdjustments_SinglePeriodDoesNotFlag proves a
// TypeOneTimeExpense appearing in only one period is NOT flagged — the
// legitimate one-time case must not produce noise.
func TestRepeatedOneTimeAdjustments_SinglePeriodDoesNotFlag(t *testing.T) {
	periods := []string{"2023", "2024", "2025"}
	ds := buildDataset(periods,
		[]float64{500000, 520000, 540000},
		[]float64{200000, 205000, 210000},
		[]float64{100000, 102000, 104000},
		[]float64{0, 0, 0},
	)
	meta := metaFor(periods, 2023)
	adjs := []adjustments.Adjustment{
		{ID: "ot-2025", Period: "2025", Type: adjustments.TypeOneTimeExpense, Amount: 7500, Reason: "equipment repair", Included: true},
	}

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta, Adjustments: adjs}, Options{})
	for _, r := range res.Recurrence {
		if r.LikelyNotNonRecurring {
			t.Errorf("did not expect LikelyNotNonRecurring for a single-period occurrence: %+v", r)
		}
	}
	for _, f := range res.Flags {
		if f.Code == FlagRepeatedOneTimeAdjustments {
			t.Error("did not expect FlagRepeatedOneTimeAdjustments for a single-period occurrence")
		}
	}
}

// TestBuildRecurrence_DistinctSameTypeAdjustmentsInSamePeriodBothCounted is
// a regression test: two DIFFERENT adjustments of the same nominally
// non-recurring Type in the same period, one targeting EBITDA only and one
// targeting SDE only, must both contribute their full Amount to
// RecurrencePattern.TotalAmount — a dedup keyed on (period, Type) alone
// would incorrectly drop the second adjustment's Amount, since both
// AppliedLines share the same Type even though they come from different
// Adjustment IDs.
func TestBuildRecurrence_DistinctSameTypeAdjustmentsInSamePeriodBothCounted(t *testing.T) {
	periods := []string{"2024", "2025"}
	ds := buildDataset(periods,
		[]float64{600000, 620000},
		[]float64{200000, 205000},
		[]float64{150000, 155000},
		[]float64{0, 0},
	)
	meta := metaFor(periods, 2024)

	adjs := []adjustments.Adjustment{
		{
			ID: "ot-ebitda-only", Period: "2025", Type: adjustments.TypeOneTimeExpense,
			Amount: 4000, Targets: []adjustments.Target{adjustments.TargetEBITDA},
			Reason: "one-time repair, EBITDA only", Included: true,
		},
		{
			ID: "ot-sde-only", Period: "2025", Type: adjustments.TypeOneTimeExpense,
			Amount: 6000, Targets: []adjustments.Target{adjustments.TargetSDE},
			Reason: "one-time repair, SDE only", Included: true,
		},
		// A second period so this Type's Count reaches the
		// RepeatedOneTimeMinPeriods threshold and LikelyNotNonRecurring
		// is exercised alongside TotalAmount in the same test.
		{
			ID: "ot-2024", Period: "2024", Type: adjustments.TypeOneTimeExpense,
			Amount: 1000, Reason: "prior year repair", Included: true,
		},
	}

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta, Adjustments: adjs}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	var pattern *RecurrencePattern
	for i := range res.Recurrence {
		if res.Recurrence[i].Type == adjustments.TypeOneTimeExpense {
			pattern = &res.Recurrence[i]
		}
	}
	if pattern == nil {
		t.Fatalf("expected a RecurrencePattern for TypeOneTimeExpense, got %+v", res.Recurrence)
	}
	wantTotal := 4000.0 + 6000.0 + 1000.0
	if pattern.TotalAmount != wantTotal {
		t.Errorf("TotalAmount = %v, want %v (both same-Type, same-period adjustments must be counted independently)", pattern.TotalAmount, wantTotal)
	}
	if pattern.Count != 2 {
		t.Errorf("Count = %d, want 2 distinct periods", pattern.Count)
	}
	if !pattern.LikelyNotNonRecurring {
		t.Error("expected LikelyNotNonRecurring == true with 2 distinct periods")
	}

	// Also confirm AdjustmentBreakdown's per-type EBITDA/SDE totals are
	// each correct independently, exercising the same underlying
	// AppliedLine data from a different aggregation path.
	var tb *TypeBreakdown
	for i := range res.Adjustments.ByType {
		if res.Adjustments.ByType[i].Type == adjustments.TypeOneTimeExpense {
			tb = &res.Adjustments.ByType[i]
		}
	}
	if tb == nil {
		t.Fatal("expected a TypeBreakdown for TypeOneTimeExpense")
	}
	wantEBITDA := 4000.0 + 1000.0
	wantSDE := 6000.0 + 1000.0
	if tb.EBITDATotal != wantEBITDA {
		t.Errorf("EBITDATotal = %v, want %v", tb.EBITDATotal, wantEBITDA)
	}
	if tb.SDETotal != wantSDE {
		t.Errorf("SDETotal = %v, want %v", tb.SDETotal, wantSDE)
	}
}

// TestRecurringSummary_ClassifiesThreeBuckets builds a dataset with one
// recurring-type adjustment (personal vehicle), one non-recurring-type
// adjustment (one-time expense), and one unclassified-type adjustment
// (non-operating income removal), and confirms buildRecurringSummary
// places each in its correct bucket with the correct signed total.
func TestRecurringSummary_ClassifiesThreeBuckets(t *testing.T) {
	periods := []string{"2025"}
	ds := buildDataset(periods,
		[]float64{500000},
		[]float64{200000},
		[]float64{150000},
		[]float64{0},
	)
	meta := metaFor(periods, 2025)

	adjs := []adjustments.Adjustment{
		{ID: "recurring", Period: "2025", Type: adjustments.TypePersonalVehicle, Amount: 6000, Reason: "owner's personal vehicle", Included: true},
		{ID: "non-recurring", Period: "2025", Type: adjustments.TypeOneTimeExpense, Amount: 9000, Reason: "one-time repair", Included: true},
		{ID: "unclassified", Period: "2025", Type: adjustments.TypeNonOperatingIncome, Amount: 4000, Reason: "investment income removal", Included: true},
	}

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta, Adjustments: adjs}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	summary := res.RecurringAdjustments

	if summary.RecurringEBITDATotal != 6000 || summary.RecurringSDETotal != 6000 {
		t.Errorf("Recurring totals = (%v, %v), want (6000, 6000)", summary.RecurringEBITDATotal, summary.RecurringSDETotal)
	}
	if summary.RecurringCount != 2 { // one applied line per bridge (EBITDA + SDE, both targeted by default)
		t.Errorf("RecurringCount = %d, want 2", summary.RecurringCount)
	}

	if summary.NonRecurringEBITDATotal != 9000 || summary.NonRecurringSDETotal != 9000 {
		t.Errorf("NonRecurring totals = (%v, %v), want (9000, 9000)", summary.NonRecurringEBITDATotal, summary.NonRecurringSDETotal)
	}
	if summary.NonRecurringCount != 2 {
		t.Errorf("NonRecurringCount = %d, want 2", summary.NonRecurringCount)
	}

	// TypeNonOperatingIncome defaults to EffectDecrease, so its
	// SignedAmount is negative.
	if summary.UnclassifiedEBITDATotal != -4000 || summary.UnclassifiedSDETotal != -4000 {
		t.Errorf("Unclassified totals = (%v, %v), want (-4000, -4000)", summary.UnclassifiedEBITDATotal, summary.UnclassifiedSDETotal)
	}
	if summary.UnclassifiedCount != 2 {
		t.Errorf("UnclassifiedCount = %d, want 2", summary.UnclassifiedCount)
	}
}

// TestVolatileEarnings_Flags builds a wildly swinging EBITDA series (via
// swinging owner-compensation-free payroll) across 4 fiscal years and
// confirms FlagVolatileEarnings fires.
func TestVolatileEarnings_Flags(t *testing.T) {
	periods := []string{"2022", "2023", "2024", "2025"}
	ds := buildDataset(periods,
		[]float64{1000000, 1000000, 1000000, 1000000},
		[]float64{400000, 400000, 400000, 400000},
		// Payroll swings wildly, driving EBITDA from 200k to 550k to 50k to 500k.
		[]float64{400000, 50000, 550000, 100000},
		[]float64{0, 0, 0, 0},
	)
	meta := metaFor(periods, 2022)

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if !res.EBITDAVolatility.Value.Available {
		t.Fatal("expected EBITDAVolatility to be available with 4 fiscal years")
	}

	found := false
	for _, f := range res.Flags {
		if f.Code == FlagVolatileEarnings {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagVolatileEarnings to fire for a wildly swinging EBITDA series (volatility=%v), got flags: %+v", res.EBITDAVolatility.Value.Value, res.Flags)
	}
}

// TestDecliningMargins_Flags builds an EBITDA-margin series that swings
// well beyond the default InconsistentMarginSwing threshold and confirms
// FlagInconsistentMargins fires, and separately confirms
// FlagDecliningEBITDADespiteRevenueGrowth fires when a period's revenue
// grows while EBITDA falls.
func TestDecliningMargins_Flags(t *testing.T) {
	periods := []string{"2023", "2024", "2025"}
	ds := buildDataset(periods,
		// Revenue grows every year...
		[]float64{1000000, 1100000, 1300000},
		[]float64{300000, 300000, 300000},
		// ...but payroll grows even faster in 2025, shrinking EBITDA
		// despite higher revenue: EBITDA = 1000000-300000-400000=300000 (30%),
		// 1100000-300000-420000=380000 (34.5%), 1300000-300000-900000=100000 (7.7%).
		[]float64{400000, 420000, 900000},
		[]float64{0, 0, 0},
	)
	meta := metaFor(periods, 2023)

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	foundInconsistent := false
	foundDeclining := false
	for _, f := range res.Flags {
		switch f.Code {
		case FlagInconsistentMargins:
			foundInconsistent = true
		case FlagDecliningEBITDADespiteRevenueGrowth:
			foundDeclining = true
			if f.Period != "2025" {
				t.Errorf("expected declining-EBITDA flag to be anchored on 2025, got %q", f.Period)
			}
		}
	}
	if !foundInconsistent {
		t.Errorf("expected FlagInconsistentMargins to fire, margin trend: %+v", res.EBITDAMarginTrend)
	}
	if !foundDeclining {
		t.Errorf("expected FlagDecliningEBITDADespiteRevenueGrowth to fire, got flags: %+v", res.Flags)
	}
}

// TestNegativeEBITDA_Result proves Calculate handles a period with negative
// reported EBITDA without panicking or corrupting availability, and that
// a maintainable-earnings figure at/below zero triggers
// FlagNegativeOrNearZeroMaintainableEarnings.
func TestNegativeEBITDA_Result(t *testing.T) {
	periods := []string{"2024", "2025"}
	ds := buildDataset(periods,
		[]float64{500000, 500000},
		[]float64{200000, 200000},
		// Payroll exceeds gross profit in both years: EBITDA negative both years.
		[]float64{400000, 420000},
		[]float64{0, 0},
	)
	meta := metaFor(periods, 2024)

	res := runQoE(t, ds, meta, nil, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	for _, pf := range res.History {
		if !pf.ReportedEBITDA.Available {
			t.Fatalf("expected reported EBITDA to be available (even if negative) for period %s", pf.Period)
		}
		if pf.ReportedEBITDA.Value >= 0 {
			t.Fatalf("expected negative reported EBITDA for period %s, got %v", pf.Period, pf.ReportedEBITDA.Value)
		}
	}

	if !res.MaintainableEBITDA.Available {
		t.Fatal("expected MaintainableEBITDA to be available (simple average of two negative figures is still a defined average)")
	}
	if res.MaintainableEBITDA.Value >= 0 {
		t.Fatalf("expected negative maintainable EBITDA, got %v", res.MaintainableEBITDA.Value)
	}

	found := false
	for _, f := range res.Flags {
		if f.Code == FlagNegativeOrNearZeroMaintainableEarnings && f.Severity == FlagSeverityCritical {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a critical FlagNegativeOrNearZeroMaintainableEarnings flag, got flags: %+v", res.Flags)
	}
}

// TestOwnerHeavySDE_Flags builds a dataset where owner compensation makes
// up a large share of normalized SDE (via a large owner-compensation
// normalization adjustment) and confirms
// FlagLargeOwnerDiscretionaryComponent fires.
func TestOwnerHeavySDE_Flags(t *testing.T) {
	periods := []string{"2024", "2025"}
	ds := buildDataset(periods,
		[]float64{600000, 620000},
		[]float64{200000, 205000},
		[]float64{150000, 155000},
		// Owner draws a large, above-market salary.
		[]float64{180000, 190000},
	)
	meta := metaFor(periods, 2024)

	adjs := []adjustments.Adjustment{
		// Normalize the owner's actual 190000 draw down to a 60000
		// market-rate replacement — a 130000 add-back to SDE, per
		// adjustments.Apply's documented "supply the difference" contract.
		{
			ID: "owner-norm-2025", Period: "2025", Type: adjustments.TypeOwnerCompensationNormalization,
			Amount: 130000, Effect: adjustments.EffectIncrease, Reason: "normalize to market-rate GM replacement salary", Included: true,
		},
	}

	res := runQoE(t, ds, meta, adjs, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	found := false
	for _, f := range res.Flags {
		if f.Code == FlagLargeOwnerDiscretionaryComponent {
			found = true
			if f.Period != "2025" {
				t.Errorf("expected owner-discretionary flag anchored on 2025, got %q", f.Period)
			}
		}
	}
	if !found {
		t.Errorf("expected FlagLargeOwnerDiscretionaryComponent to fire, got flags: %+v", res.Flags)
	}
}

// TestNonOperatingIncomeSupportingEarnings_Flags confirms
// FlagNonOperatingIncomeSupportingEarnings fires when a large fraction of
// reported EBITDA is explained away by non-operating-income-removal
// adjustments.
func TestNonOperatingIncomeSupportingEarnings_Flags(t *testing.T) {
	periods := []string{"2024", "2025"}
	ds := buildDataset(periods,
		[]float64{500000, 500000},
		[]float64{200000, 200000},
		[]float64{150000, 150000},
		[]float64{0, 0},
	)
	meta := metaFor(periods, 2024)

	// Reported EBITDA for 2025 = 500000-200000-150000 = 150000.
	adjs := []adjustments.Adjustment{
		{
			ID: "non-op-income", Period: "2025", Type: adjustments.TypeNonOperatingIncome,
			Amount: 60000, Reason: "one-time investment gain included in other income", Included: true,
		},
	}

	res := runQoE(t, ds, meta, adjs, Options{})
	found := false
	for _, f := range res.Flags {
		if f.Code == FlagNonOperatingIncomeSupportingEarnings {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagNonOperatingIncomeSupportingEarnings to fire, got flags: %+v", res.Flags)
	}
}
