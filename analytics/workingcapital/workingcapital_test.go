package workingcapital

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func TestCalculate_NoPeriods(t *testing.T) {
	res := Calculate(Input{}, Options{})
	if res.Available {
		t.Fatal("expected Available == false for empty dataset")
	}
	if !HasErrors(res.Errors) {
		t.Fatal("expected an error Issue for empty dataset")
	}
	if len(res.Errors) != 1 || res.Errors[0].Code != IssueNoPeriods {
		t.Fatalf("expected exactly one IssueNoPeriods error, got %+v", res.Errors)
	}
}

func TestCalculate_BasicPerPeriod(t *testing.T) {
	ds := newDataset().
		bsPeriod("2023", 38000, 15000, 0, 21000, 4000, 500000).
		bsPeriod("2024", 41000, 16500, 0, 23000, 4200, 540000).
		bsPeriod("2025", 44500, 17800, 0, 24500, 4400, 580000).
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: threeYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.History) != 3 {
		t.Fatalf("expected 3 periods, got %d", len(res.History))
	}

	p2025 := res.History[2]
	wantCA := 44500.0 + 17800.0
	wantCL := 24500.0 + 4400.0
	wantNWC := wantCA - wantCL
	if !p2025.OperatingCurrentAssets.Available || p2025.OperatingCurrentAssets.Value != wantCA {
		t.Errorf("2025 OperatingCurrentAssets = %+v, want %v", p2025.OperatingCurrentAssets, wantCA)
	}
	if !p2025.OperatingCurrentLiabilities.Available || p2025.OperatingCurrentLiabilities.Value != wantCL {
		t.Errorf("2025 OperatingCurrentLiabilities = %+v, want %v", p2025.OperatingCurrentLiabilities, wantCL)
	}
	if !p2025.NWC.Available || p2025.NWC.Value != wantNWC {
		t.Errorf("2025 NWC = %+v, want %v", p2025.NWC, wantNWC)
	}
	wantPct := wantNWC / 580000.0
	if !p2025.NWCPercentOfRevenue.Available || p2025.NWCPercentOfRevenue.Value != wantPct {
		t.Errorf("2025 NWCPercentOfRevenue = %+v, want %v", p2025.NWCPercentOfRevenue, wantPct)
	}
}

func TestCalculate_CashAndDebtExcludedByDefault(t *testing.T) {
	ds := newDataset().
		bsPeriod("2025", 40000, 15000, 0, 20000, 4000, 500000).
		add(financial.CodeBsCash, "2025", 100000).
		add(financial.CodeBsShortTermDebt, "2025", 50000).
		build()

	res := Calculate(Input{Dataset: ds}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	p := res.History[0]
	// Cash/short-term debt must not appear in the operating totals.
	wantCA := 40000.0 + 15000.0
	wantCL := 20000.0 + 4000.0
	if p.OperatingCurrentAssets.Value != wantCA {
		t.Errorf("OperatingCurrentAssets = %v, want %v (cash must be excluded by default)", p.OperatingCurrentAssets.Value, wantCA)
	}
	if p.OperatingCurrentLiabilities.Value != wantCL {
		t.Errorf("OperatingCurrentLiabilities = %v, want %v (short-term debt must be excluded by default)", p.OperatingCurrentLiabilities.Value, wantCL)
	}

	foundCash, foundDebt := false, false
	for _, c := range res.ExcludedCodes {
		if c == financial.CodeBsCash {
			foundCash = true
		}
		if c == financial.CodeBsShortTermDebt {
			foundDebt = true
		}
	}
	if !foundCash || !foundDebt {
		t.Errorf("expected ExcludedCodes to list cash and short-term debt, got %v", res.ExcludedCodes)
	}
}

func TestCalculate_CallerPolicyCanIncludeCashAndDebt(t *testing.T) {
	ds := newDataset().
		bsPeriod("2025", 40000, 15000, 0, 20000, 4000, 500000).
		add(financial.CodeBsCash, "2025", 100000).
		add(financial.CodeBsShortTermDebt, "2025", 50000).
		build()

	policy := DefaultInclusionPolicy()
	policy.AssetCodes = append(policy.AssetCodes, financial.CodeBsCash)
	policy.LiabilityCodes = append(policy.LiabilityCodes, financial.CodeBsShortTermDebt)

	res := Calculate(Input{Dataset: ds}, Options{InclusionPolicy: policy})
	p := res.History[0]
	wantCA := 40000.0 + 15000.0 + 100000.0
	wantCL := 20000.0 + 4000.0 + 50000.0
	if p.OperatingCurrentAssets.Value != wantCA {
		t.Errorf("OperatingCurrentAssets = %v, want %v", p.OperatingCurrentAssets.Value, wantCA)
	}
	if p.OperatingCurrentLiabilities.Value != wantCL {
		t.Errorf("OperatingCurrentLiabilities = %v, want %v", p.OperatingCurrentLiabilities.Value, wantCL)
	}
	for _, c := range res.ExcludedCodes {
		if c == financial.CodeBsCash || c == financial.CodeBsShortTermDebt {
			t.Errorf("expected cash/debt NOT excluded when caller policy includes them, got ExcludedCodes=%v", res.ExcludedCodes)
		}
	}
}

func TestCalculate_NegativeWorkingCapital(t *testing.T) {
	// Liabilities exceed assets: a common shape for subscription/services
	// businesses with deferred revenue-like current liabilities.
	ds := newDataset().
		bsPeriod("2025", 10000, 0, 0, 30000, 15000, 400000).
		build()

	res := Calculate(Input{Dataset: ds}, Options{})
	p := res.History[0]
	wantNWC := 10000.0 - (30000.0 + 15000.0)
	if !p.NWC.Available || p.NWC.Value != wantNWC {
		t.Fatalf("NWC = %+v, want %v", p.NWC, wantNWC)
	}
	if p.NWC.Value >= 0 {
		t.Fatal("expected negative NWC")
	}
	if !p.NWCPercentOfRevenue.Available || p.NWCPercentOfRevenue.Value >= 0 {
		t.Errorf("expected negative NWCPercentOfRevenue, got %+v", p.NWCPercentOfRevenue)
	}
}

func TestCalculate_MissingRevenue(t *testing.T) {
	ds := newDataset().
		add(financial.CodeBsAccountsReceivable, "2025", 40000).
		add(financial.CodeBsInventory, "2025", 15000).
		add(financial.CodeBsAccountsPayable, "2025", 20000).
		build()

	res := Calculate(Input{Dataset: ds}, Options{})
	if !res.Available {
		t.Fatalf("expected Available even without revenue, errors=%+v", res.Errors)
	}
	p := res.History[0]
	if p.Revenue.Available {
		t.Errorf("expected Revenue unavailable, got %+v", p.Revenue)
	}
	if p.NWCPercentOfRevenue.Available {
		t.Errorf("expected NWCPercentOfRevenue unavailable without revenue, got %+v", p.NWCPercentOfRevenue)
	}
	if !p.NWC.Available {
		t.Error("expected NWC still available despite missing revenue")
	}

	foundWarning := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoRevenueData {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected IssueNoRevenueData warning, got %+v", res.Warnings)
	}
}

func TestCalculate_StableNWC(t *testing.T) {
	ds := newDataset().
		bsPeriod("2023", 40000, 15000, 0, 24000, 4000, 500000).
		bsPeriod("2024", 40200, 15100, 0, 24100, 4050, 505000).
		bsPeriod("2025", 39900, 14950, 0, 23950, 3980, 498000).
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: threeYearMeta()}, Options{})
	if res.Trend.Direction != TrendStable {
		t.Errorf("expected TrendStable, got %v (percent change %+v)", res.Trend.Direction, res.Trend.PercentChange)
	}
}

func TestCalculate_DecliningNWC(t *testing.T) {
	ds := newDataset().
		bsPeriod("2023", 60000, 30000, 0, 20000, 4000, 500000).
		bsPeriod("2024", 45000, 22000, 0, 20000, 4000, 500000).
		bsPeriod("2025", 30000, 15000, 0, 20000, 4000, 500000).
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: threeYearMeta()}, Options{})
	if res.Trend.Direction != TrendDeclining {
		t.Errorf("expected TrendDeclining, got %v (percent change %+v)", res.Trend.Direction, res.Trend.PercentChange)
	}
	if res.Trend.FirstPeriod != "2023" || res.Trend.LastPeriod != "2025" {
		t.Errorf("expected FirstPeriod=2023 LastPeriod=2025, got %v/%v", res.Trend.FirstPeriod, res.Trend.LastPeriod)
	}
}

func TestCalculate_IncreasingNWC(t *testing.T) {
	ds := newDataset().
		bsPeriod("2023", 30000, 15000, 0, 20000, 4000, 500000).
		bsPeriod("2024", 45000, 22000, 0, 20000, 4000, 500000).
		bsPeriod("2025", 60000, 30000, 0, 20000, 4000, 500000).
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: threeYearMeta()}, Options{})
	if res.Trend.Direction != TrendIncreasing {
		t.Errorf("expected TrendIncreasing, got %v", res.Trend.Direction)
	}
}

func TestCalculate_NoPeriodMetaFallsBackToLexicalOrder(t *testing.T) {
	ds := newDataset().
		bsPeriod("2023", 40000, 15000, 0, 24000, 4000, 500000).
		bsPeriod("2024", 41000, 16500, 0, 23000, 4200, 540000).
		build()

	res := Calculate(Input{Dataset: ds}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
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
	// Trend/SeasonalProfile should still be safely zero-valued, not crash.
	if res.Trend.Direction != TrendUnavailable && res.Trend.Direction != "" {
		// Trend is still computed off History's order even without
		// PeriodMeta (lexical fallback), so a 2-point stable/declining/
		// increasing verdict is legitimate here — just confirm no panic
		// and a valid enum value.
		switch res.Trend.Direction {
		case TrendIncreasing, TrendDeclining, TrendStable:
		default:
			t.Errorf("unexpected TrendDirection %q", res.Trend.Direction)
		}
	}
}

func TestCalculate_PeriodMissingFromMeta(t *testing.T) {
	ds := newDataset().
		bsPeriod("2023", 40000, 15000, 0, 24000, 4000, 500000).
		bsPeriod("2024", 41000, 16500, 0, 23000, 4200, 540000).
		build()

	partialMeta := map[financial.Period]PeriodInfo{
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
	}
	res := Calculate(Input{Dataset: ds, PeriodMeta: partialMeta}, Options{})
	foundWarning := false
	for _, w := range res.Warnings {
		if w.Code == IssuePeriodMissingFromMeta {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected IssuePeriodMissingFromMeta warning, got %+v", res.Warnings)
	}
}

func TestCalculate_EmptyInclusionPolicyWarns(t *testing.T) {
	ds := newDataset().bsPeriod("2025", 40000, 15000, 0, 20000, 4000, 500000).build()

	res := Calculate(Input{Dataset: ds}, Options{
		InclusionPolicy: InclusionPolicy{AssetCodes: []financial.Code{financial.CodeBsAccountsReceivable}},
	})
	foundWarning := false
	for _, w := range res.Warnings {
		if w.Code == IssueEmptyInclusionPolicy {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected IssueEmptyInclusionPolicy warning, got %+v", res.Warnings)
	}
	if res.History[0].OperatingCurrentLiabilities.Available {
		t.Error("expected OperatingCurrentLiabilities unavailable with no liability codes configured")
	}
}

func TestCalculate_QuarterlyVsAnnualComparability(t *testing.T) {
	// A dataset mixing annual and quarterly periods: Trend/statistics use
	// whatever chronological order PeriodMeta supplies (fiscal year/YTD
	// ranked ahead of quarter/month within the same fiscal year, mirroring
	// financial/metrics' granularity ranking) without silently averaging
	// incompatible granularities together into one misleading number — this
	// package leaves that judgment to the caller via which periods they
	// include in Dataset, and SeasonalProfile explicitly separates out
	// same-granularity periods only.
	ds := newDataset().
		bsPeriod("2024", 40000, 15000, 0, 24000, 4000, 500000).
		bsPeriod("2025-Q1", 41000, 15500, 0, 24200, 4050, 130000).
		bsPeriod("2025-Q2", 42000, 16000, 0, 24400, 4100, 135000).
		build()

	meta := map[financial.Period]PeriodInfo{
		"2024":    {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025-Q1": {Type: PeriodTypeQuarter, FiscalYear: 2025, SequenceInYear: 1},
		"2025-Q2": {Type: PeriodTypeQuarter, FiscalYear: 2025, SequenceInYear: 2},
	}
	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.History) != 3 {
		t.Fatalf("expected 3 periods, got %d", len(res.History))
	}
	// Fiscal year 2024 must sort before both 2025 quarters.
	if res.History[0].Period != "2024" {
		t.Errorf("expected 2024 first chronologically, got %v", res.History[0].Period)
	}
	if res.History[1].Period != "2025-Q1" || res.History[2].Period != "2025-Q2" {
		t.Errorf("expected Q1 then Q2, got %v then %v", res.History[1].Period, res.History[2].Period)
	}
}

func TestCalculate_CurrentNWCFromAsOf(t *testing.T) {
	ds := newDataset().
		bsPeriod("2025", 44500, 17800, 0, 24500, 4400, 580000).
		build()

	res := Calculate(Input{Dataset: ds, AsOf: "2025", PeriodMeta: threeYearMeta()}, Options{PegMethod: PegMethodLatest})
	if !res.PegComparison.CurrentNWC.Available {
		t.Fatalf("expected CurrentNWC derived from AsOf, got %+v", res.PegComparison.CurrentNWC)
	}
	want := (44500.0 + 17800.0) - (24500.0 + 4400.0)
	if res.PegComparison.CurrentNWC.Value != want {
		t.Errorf("CurrentNWC = %v, want %v", res.PegComparison.CurrentNWC.Value, want)
	}
}

func TestCalculate_CurrentNWCOverridesAsOf(t *testing.T) {
	ds := newDataset().
		bsPeriod("2025", 44500, 17800, 0, 24500, 4400, 580000).
		build()

	res := Calculate(Input{
		Dataset:    ds,
		AsOf:       "2025",
		CurrentNWC: AvailableValue(999999),
	}, Options{PegMethod: PegMethodLatest})
	if res.PegComparison.CurrentNWC.Value != 999999 {
		t.Errorf("expected explicit CurrentNWC to override AsOf derivation, got %v", res.PegComparison.CurrentNWC.Value)
	}
}

func TestCalculate_CurrentNWCUnavailableWithoutAsOfOrOverride(t *testing.T) {
	ds := newDataset().bsPeriod("2025", 44500, 17800, 0, 24500, 4400, 580000).build()
	res := Calculate(Input{Dataset: ds}, Options{PegMethod: PegMethodLatest})
	if res.PegComparison.CurrentNWC.Available {
		t.Errorf("expected CurrentNWC unavailable, got %+v", res.PegComparison.CurrentNWC)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueCurrentNWCUnavailable {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueCurrentNWCUnavailable warning, got %+v", res.Warnings)
	}
}
