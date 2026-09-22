package ratios

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

func TestCalculate_NoPeriods(t *testing.T) {
	res := Calculate(Input{}, Options{})
	if res.Available {
		t.Fatal("expected Available == false for an empty dataset")
	}
	if !HasErrors(res.Errors) {
		t.Fatal("expected an error Issue for an empty dataset")
	}
	if res.Errors[0].Code != IssueNoPeriods {
		t.Fatalf("expected IssueNoPeriods, got %s", res.Errors[0].Code)
	}
}

// TestCalculate_NormalCase exercises a complete income statement + balance
// sheet for a single period and checks every ratio category is Available
// with the expected value.
func TestCalculate_NormalCase(t *testing.T) {
	ds := newDataset().
		incomeStatement("2025", 1000, 400, 300, 20, 10, 15). // revenue 1000, cogs 400, opex 300, dep 20, interest 10, tax 15
		balanceSheet("2025", 100, 200, 50, 80, 40, 60, 300). // cash 100, ar 200, inv 50, ap 80, std 40, ltd 60, equity 300
		build()

	res := Calculate(Input{Dataset: ds}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.History) != 1 {
		t.Fatalf("expected 1 period, got %d", len(res.History))
	}
	p := res.History[0]

	// Profitability: GrossProfit = 1000-400 = 600, GrossMargin = 0.6
	assertAvailable(t, "GrossMargin", p.GrossMargin.Value, 0.6)
	// EBIT = GrossProfit - Opex = 600 - 300 = 300, OperatingMargin = 0.3
	assertAvailable(t, "OperatingMargin", p.OperatingMargin.Value, 0.3)
	// EBITDA = EBIT + Dep = 300 + 20 = 320, EBITDAMargin = 0.32
	assertAvailable(t, "EBITDAMargin", p.EBITDAMargin.Value, 0.32)
	// NetIncome = EBIT - InterestExpense - Tax = 300 - 10 - 15 = 275, NetMargin = 0.275
	assertAvailable(t, "NetMargin", p.NetMargin.Value, 0.275)

	// Liquidity: CurrentAssets = cash+ar+inv = 100+200+50 = 350.
	// CurrentLiabilities = ap+std = 80+40 = 120. CurrentRatio = 350/120.
	assertAvailable(t, "CurrentRatio", p.CurrentRatio.Value, 350.0/120.0)
	// QuickAssets = cash+ar = 300. QuickRatio = 300/120.
	assertAvailable(t, "QuickRatio", p.QuickRatio.Value, 300.0/120.0)
	// CashRatio = 100/120.
	assertAvailable(t, "CashRatio", p.CashRatio.Value, 100.0/120.0)

	// Leverage: TotalDebt = 40+60=100. TotalEquity=300. DebtToEquity=100/300.
	assertAvailable(t, "DebtToEquity", p.DebtToEquity.Value, 100.0/300.0)
	// TotalAssets = cash+ar+inv = 100+200+50 = 350 (no fixed/intangible/goodwill in this fixture).
	assertAvailable(t, "DebtToAssets", p.DebtToAssets.Value, 100.0/350.0)
	// DebtToEBITDA = 100/320.
	assertAvailable(t, "DebtToEBITDA", p.DebtToEBITDA.Value, 100.0/320.0)
	// NetDebt = TotalDebt - Cash = 100-100=0. NetDebtToEBITDA = 0/320 = 0.
	assertAvailable(t, "NetDebtToEBITDA", p.NetDebtToEBITDA.Value, 0)
	// InterestCoverage = EBIT/InterestExpense = 300/10 = 30.
	assertAvailable(t, "InterestCoverage", p.InterestCoverage.Value, 30)

	// Efficiency: AssetTurnover = Revenue/TotalAssets = 1000/350.
	assertAvailable(t, "AssetTurnover", p.AssetTurnover.Value, 1000.0/350.0)
	// ReceivablesTurnover = Revenue/AR = 1000/200 = 5.
	assertAvailable(t, "ReceivablesTurnover", p.ReceivablesTurnover.Value, 5)
	// InventoryTurnover = COGS/Inventory = 400/50 = 8.
	assertAvailable(t, "InventoryTurnover", p.InventoryTurnover.Value, 8)
	// DSO = (AR/Revenue)*365 = (200/1000)*365 = 73.
	assertAvailable(t, "DaysSalesOutstanding", p.DaysSalesOutstanding.Value, 73)
	// DIO = (Inventory/COGS)*365 = (50/400)*365 = 45.625.
	assertAvailable(t, "DaysInventoryOutstanding", p.DaysInventoryOutstanding.Value, 45.625)
	// DPO = (AP/COGS)*365 = (80/400)*365 = 73.
	assertAvailable(t, "DaysPayableOutstanding", p.DaysPayableOutstanding.Value, 73)
	// CCC = DSO+DIO-DPO = 73+45.625-73 = 45.625.
	assertAvailable(t, "CashConversionCycle", p.CashConversionCycle.Value, 45.625)

	// ROA = NetIncome/TotalAssets = 275/350.
	assertAvailable(t, "ReturnOnAssets", p.ReturnOnAssets.Value, 275.0/350.0)
	// ROE = NetIncome/TotalEquity = 275/300.
	assertAvailable(t, "ReturnOnEquity", p.ReturnOnEquity.Value, 275.0/300.0)
}

func TestCalculate_ZeroDenominators(t *testing.T) {
	ds := newDataset().
		incomeStatement("2025", 0, 0, 0, 0, 0, 0). // TotalRevenue is Available (anyPresent) but == 0
		balanceSheet("2025", 0, 0, 0, 0, 0, 0, 0).
		build()

	res := Calculate(Input{Dataset: ds}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	p := res.History[0]

	assertUnavailable(t, "GrossMargin", p.GrossMargin.Value)                   // revenue == 0
	assertUnavailable(t, "CurrentRatio", p.CurrentRatio.Value)                 // current liabilities == 0
	assertUnavailable(t, "DebtToEquity", p.DebtToEquity.Value)                 // equity == 0
	assertUnavailable(t, "DebtToEBITDA", p.DebtToEBITDA.Value)                 // EBITDA == 0
	assertUnavailable(t, "ReceivablesTurnover", p.ReceivablesTurnover.Value)   // AR == 0
	assertUnavailable(t, "InterestCoverage", p.InterestCoverage.Value)         // interest expense == 0
	assertUnavailable(t, "DaysSalesOutstanding", p.DaysSalesOutstanding.Value) // revenue == 0
}

func TestCalculate_NegativeEquity(t *testing.T) {
	ds := newDataset().
		incomeStatement("2025", 1000, 400, 300, 20, 10, 15).
		balanceSheet("2025", 100, 200, 50, 80, 40, 60, -300). // accumulated deficit
		build()

	res := Calculate(Input{Dataset: ds}, Options{})
	p := res.History[0]

	// ROE and DebtToEquity are Available and negative, not suppressed.
	assertAvailable(t, "ReturnOnEquity", p.ReturnOnEquity.Value, 275.0/-300.0)
	if p.ReturnOnEquity.Value.Value >= 0 {
		t.Fatalf("expected negative ReturnOnEquity against negative equity, got %v", p.ReturnOnEquity.Value.Value)
	}
	assertAvailable(t, "DebtToEquity", p.DebtToEquity.Value, 100.0/-300.0)
}

func TestCalculate_NegativeEarnings(t *testing.T) {
	ds := newDataset().
		incomeStatement("2025", 500, 400, 300, 20, 10, 0). // GrossProfit=100, EBIT=100-300=-200, EBITDA=-200+20=-180
		balanceSheet("2025", 100, 200, 50, 80, 40, 60, 300).
		build()

	res := Calculate(Input{Dataset: ds}, Options{})
	p := res.History[0]

	if !p.EBITDAMargin.Value.Available || p.EBITDAMargin.Value.Value >= 0 {
		t.Fatalf("expected negative available EBITDAMargin, got %+v", p.EBITDAMargin.Value)
	}
	// DebtToEBITDA against negative EBITDA is Available (allowed through, not suppressed).
	if !p.DebtToEBITDA.Value.Available {
		t.Fatal("expected DebtToEBITDA to be available against negative EBITDA")
	}
	// InterestCoverage = EBIT/InterestExpense = -200/10 = -20.
	assertAvailable(t, "InterestCoverage", p.InterestCoverage.Value, -20)
}

func TestCalculate_MissingBalanceSheet(t *testing.T) {
	ds := newDataset().
		incomeStatement("2025", 1000, 400, 300, 20, 10, 15).
		build()

	res := Calculate(Input{Dataset: ds}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	p := res.History[0]

	// Income-statement ratios remain available.
	assertAvailable(t, "GrossMargin", p.GrossMargin.Value, 0.6)
	assertAvailable(t, "EBITDAMargin", p.EBITDAMargin.Value, 0.32)

	// Every balance-sheet-derived ratio is unavailable, not zero.
	assertUnavailable(t, "CurrentRatio", p.CurrentRatio.Value)
	assertUnavailable(t, "QuickRatio", p.QuickRatio.Value)
	assertUnavailable(t, "CashRatio", p.CashRatio.Value)
	assertUnavailable(t, "DebtToEquity", p.DebtToEquity.Value)
	assertUnavailable(t, "DebtToAssets", p.DebtToAssets.Value)
	assertUnavailable(t, "ReturnOnAssets", p.ReturnOnAssets.Value)
	assertUnavailable(t, "ReturnOnEquity", p.ReturnOnEquity.Value)
	assertUnavailable(t, "AssetTurnover", p.AssetTurnover.Value)
	assertUnavailable(t, "ReceivablesTurnover", p.ReceivablesTurnover.Value)
	assertUnavailable(t, "InventoryTurnover", p.InventoryTurnover.Value)

	// InterestCoverage needs only EBIT + interest expense (both
	// income-statement figures), so it remains available even with no
	// balance sheet at all.
	assertAvailable(t, "InterestCoverage", p.InterestCoverage.Value, 300.0/10.0)

	assertUnavailable(t, "TotalAssets", p.TotalAssets)
}

// TestCalculate_MultiYearTrend exercises the realistic HVAC fixture across
// three fiscal years and checks Trends/Comparisons/Growth are populated and
// chronologically ordered.
func TestCalculate_MultiYearTrend(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")
	meta := threeYearMeta()

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.History) != 3 {
		t.Fatalf("expected 3 periods, got %d", len(res.History))
	}
	wantOrder := []financial.Period{"2023", "2024", "2025"}
	for i, p := range res.History {
		if p.Period != wantOrder[i] {
			t.Fatalf("History[%d].Period = %s, want %s", i, p.Period, wantOrder[i])
		}
	}

	if len(res.Trends) == 0 {
		t.Fatal("expected at least one RatioTrend")
	}
	if len(res.Comparisons) == 0 {
		t.Fatal("expected at least one Comparison")
	}
	if len(res.Growth.RevenueGrowth) != 2 {
		t.Fatalf("expected 2 RevenueGrowth points across 3 years, got %d", len(res.Growth.RevenueGrowth))
	}

	// Every GrowthPoint's FromPeriod/ToPeriod must be chronologically adjacent.
	for i, gp := range res.Growth.RevenueGrowth {
		if gp.FromPeriod != wantOrder[i] || gp.ToPeriod != wantOrder[i+1] {
			t.Fatalf("RevenueGrowth[%d] = %s->%s, want %s->%s", i, gp.FromPeriod, gp.ToPeriod, wantOrder[i], wantOrder[i+1])
		}
	}
}

// TestCalculate_NoPeriodMeta proves History is still fully computed without
// PeriodMeta, but Trends/Comparisons/Growth are left empty with an advisory
// warning — mirroring workingcapital's identical isolation rule.
func TestCalculate_NoPeriodMeta(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")

	res := Calculate(Input{Dataset: ds}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.History) != 3 {
		t.Fatalf("expected 3 periods, got %d", len(res.History))
	}
	if len(res.Trends) != 0 {
		t.Fatal("expected no Trends without PeriodMeta")
	}
	if len(res.Comparisons) != 0 {
		t.Fatal("expected no Comparisons without PeriodMeta")
	}
	if len(res.Growth.RevenueGrowth) != 0 {
		t.Fatal("expected no Growth without PeriodMeta")
	}

	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoPeriodMeta {
			found = true
		}
	}
	if !found {
		t.Fatal("expected IssueNoPeriodMeta warning")
	}
}

// TestCalculate_DeterministicOrdering proves History/Results ordering does
// not depend on dataset item order or map iteration.
func TestCalculate_DeterministicOrdering(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")
	meta := threeYearMeta()

	res1 := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	res2 := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})

	if len(res1.History) != len(res2.History) {
		t.Fatal("History length differs between runs")
	}
	for i := range res1.History {
		if res1.History[i].Period != res2.History[i].Period {
			t.Fatalf("History[%d].Period differs: %s vs %s", i, res1.History[i].Period, res2.History[i].Period)
		}
	}
}

const epsilon = 1e-9

func assertAvailable(t *testing.T, name string, got metrics.MetricValue, want float64) {
	t.Helper()
	if !got.Available {
		t.Fatalf("%s: expected Available, got Unavailable", name)
	}
	diff := got.Value - want
	if diff < 0 {
		diff = -diff
	}
	if diff > epsilon {
		t.Fatalf("%s: got %v, want %v", name, got.Value, want)
	}
}

func assertUnavailable(t *testing.T, name string, got metrics.MetricValue) {
	t.Helper()
	if got.Available {
		t.Fatalf("%s: expected Unavailable, got Available (%v)", name, got.Value)
	}
}
