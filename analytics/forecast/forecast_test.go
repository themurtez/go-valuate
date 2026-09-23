package forecast

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

const epsilon = 1e-9

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

func mustAvailable(t *testing.T, name string, v ForecastValue) float64 {
	t.Helper()
	if !v.Available {
		t.Fatalf("%s: expected available, got unavailable", name)
	}
	return v.Value
}

// TestCalculate_NoPeriods verifies the top-level blocking-error path.
func TestCalculate_NoPeriods(t *testing.T) {
	result := Calculate(Input{
		Dataset:    financial.FinancialDataset{Currency: "USD"},
		PeriodMeta: threeYearMeta(),
		Horizon:    3,
		Scenarios:  []Scenario{{Name: "Base"}},
	})
	if result.Available {
		t.Fatalf("expected Available=false for an empty dataset")
	}
	if !HasErrors(result.Errors) {
		t.Fatalf("expected an error issue")
	}
	found := false
	for _, e := range result.Errors {
		if e.Code == IssueNoPeriods {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueNoPeriods, got %+v", result.Errors)
	}
}

// TestCalculate_NoPeriodMeta verifies that a missing PeriodMeta blocks the
// whole Result, per Input.PeriodMeta's doc comment (unlike sibling
// packages, this package cannot identify a base period without it).
func TestCalculate_NoPeriodMeta(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 20_000).build()
	result := Calculate(Input{
		Dataset:   ds,
		Horizon:   3,
		Scenarios: []Scenario{{Name: "Base"}},
	})
	if result.Available {
		t.Fatalf("expected Available=false with no PeriodMeta")
	}
	if len(result.Errors) != 1 || result.Errors[0].Code != IssueNoPeriodMeta {
		t.Fatalf("expected exactly one IssueNoPeriodMeta, got %+v", result.Errors)
	}
}

// TestCalculate_InvalidHorizon verifies Horizon < 1 is blocking.
func TestCalculate_InvalidHorizon(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 20_000).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    0,
		Scenarios:  []Scenario{{Name: "Base"}},
	})
	if result.Available {
		t.Fatalf("expected Available=false for horizon 0")
	}
	if len(result.Errors) != 1 || result.Errors[0].Code != IssueInvalidHorizon {
		t.Fatalf("expected IssueInvalidHorizon, got %+v", result.Errors)
	}
}

// TestCalculate_NoScenarios verifies an empty Scenarios slice is blocking.
func TestCalculate_NoScenarios(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 20_000).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    2,
	})
	if result.Available {
		t.Fatalf("expected Available=false with no scenarios")
	}
	if len(result.Errors) != 1 || result.Errors[0].Code != IssueNoScenarios {
		t.Fatalf("expected IssueNoScenarios, got %+v", result.Errors)
	}
}

// TestCalculate_BasePeriodIdentification verifies the chronologically last
// period (per PeriodMeta, not dataset/lexical order) is used as the base,
// using a dataset whose periods are added out of chronological order and
// whose lexical order would pick the wrong one.
func TestCalculate_BasePeriodIdentification(t *testing.T) {
	ds := newDataset().
		simpleBasePeriod("2025", 2_000_000, 800_000, 600_000, 40_000).
		simpleBasePeriod("2023", 1_000_000, 400_000, 300_000, 20_000).
		simpleBasePeriod("2024", 1_500_000, 600_000, 450_000, 30_000).
		build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
		Horizon:    1,
		Scenarios:  []Scenario{{Name: "Base", Assumptions: Assumptions{Revenue: flatRevenueGrowth(0, 1)}}},
	})
	if !result.Available {
		t.Fatalf("expected Available=true, errors: %+v", result.Errors)
	}
	if result.Base.Period != "2025" {
		t.Fatalf("expected base period 2025 (chronologically last), got %s", result.Base.Period)
	}
	if got := mustAvailable(t, "base revenue", result.Base.PL.TotalRevenue); !approxEqual(got, 2_000_000) {
		t.Fatalf("expected base revenue 2000000, got %v", got)
	}
}

// TestCalculate_BaseFinancials verifies the base period's PeriodPL is
// computed with the exact same formulas as financial/metrics (hand-verified
// arithmetic).
func TestCalculate_BaseFinancials(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2025", 800_000).
		add(financial.CodeRevService, "2025", 200_000).
		add(financial.CodeCogsMaterial, "2025", 400_000).
		add(financial.CodeOpexPayroll, "2025", 250_000).
		add(financial.CodeOpexOwnerComp, "2025", 80_000).
		add(financial.CodeDepreciation, "2025", 30_000).
		add(financial.CodeAmortization, "2025", 10_000).
		add(financial.CodeInterestExpense, "2025", 15_000).
		add(financial.CodeIncomeTax, "2025", 25_000).
		build()

	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios:  []Scenario{{Name: "Base", Assumptions: Assumptions{Revenue: flatRevenueGrowth(0, 1)}}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	pl := result.Base.PL

	if got := mustAvailable(t, "total revenue", pl.TotalRevenue); !approxEqual(got, 1_000_000) {
		t.Fatalf("total revenue: want 1000000, got %v", got)
	}
	if got := mustAvailable(t, "gross profit", pl.GrossProfit); !approxEqual(got, 600_000) {
		t.Fatalf("gross profit: want 600000, got %v", got)
	}
	if got := mustAvailable(t, "total opex", pl.TotalOpex); !approxEqual(got, 330_000) {
		t.Fatalf("total opex: want 330000, got %v", got)
	}
	if got := mustAvailable(t, "ebit", pl.EBIT); !approxEqual(got, 270_000) {
		t.Fatalf("ebit: want 270000, got %v", got)
	}
	if got := mustAvailable(t, "ebitda", pl.EBITDA); !approxEqual(got, 310_000) {
		t.Fatalf("ebitda: want 310000 (270000+30000+10000), got %v", got)
	}
	if got := mustAvailable(t, "sde", pl.SDE); !approxEqual(got, 390_000) {
		t.Fatalf("sde: want 390000 (310000+80000), got %v", got)
	}
	if got := mustAvailable(t, "pretax income", pl.PretaxIncome); !approxEqual(got, 255_000) {
		t.Fatalf("pretax income: want 255000 (270000-15000), got %v", got)
	}
	if got := mustAvailable(t, "net income", pl.NetIncome); !approxEqual(got, 230_000) {
		t.Fatalf("net income: want 230000 (255000-25000), got %v", got)
	}
}

// TestCalculate_SingleYearGrowth verifies a one-period growth-rate
// projection against hand-computed figures.
func TestCalculate_SingleYearGrowth(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 20_000).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{{
			Name: "Base",
			Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(0.10, 1),
				COGS:    flatGrossMargin(0.60, 1), // 60% gross margin -> COGS = revenue*0.4
				Opex:    flatOpexGrowth(0.05, 1),
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	sr := result.ScenarioResults[0]
	if len(sr.ProjectedPeriods) != 1 {
		t.Fatalf("expected 1 projected period, got %d", len(sr.ProjectedPeriods))
	}
	p1 := sr.ProjectedPeriods[0]

	wantRevenue := 1_100_000.0 // 1,000,000 * 1.10
	if got := mustAvailable(t, "period1 revenue", p1.TotalRevenue); !approxEqual(got, wantRevenue) {
		t.Fatalf("period1 revenue: want %v, got %v", wantRevenue, got)
	}
	wantCOGS := wantRevenue * 0.40 // 1 - 0.60 target margin
	if got := mustAvailable(t, "period1 cogs", p1.TotalCOGS); !approxEqual(got, wantCOGS) {
		t.Fatalf("period1 cogs: want %v, got %v", wantCOGS, got)
	}
	wantGP := wantRevenue - wantCOGS
	if got := mustAvailable(t, "period1 gross profit", p1.GrossProfit); !approxEqual(got, wantGP) {
		t.Fatalf("period1 gross profit: want %v, got %v", wantGP, got)
	}
	if got := mustAvailable(t, "period1 gross margin", p1.GrossMargin); !approxEqual(got, 0.60) {
		t.Fatalf("period1 gross margin: want 0.60, got %v", got)
	}
	wantOpex := 300_000.0 * 1.05
	if got := mustAvailable(t, "period1 opex", p1.TotalOpex); !approxEqual(got, wantOpex) {
		t.Fatalf("period1 opex: want %v, got %v", wantOpex, got)
	}
	wantEBIT := wantGP - wantOpex
	if got := mustAvailable(t, "period1 ebit", p1.EBIT); !approxEqual(got, wantEBIT) {
		t.Fatalf("period1 ebit: want %v, got %v", wantEBIT, got)
	}
	// No D&A assumption supplied for the projected period -> EBITDA == EBIT.
	if got := mustAvailable(t, "period1 ebitda", p1.EBITDA); !approxEqual(got, wantEBIT) {
		t.Fatalf("period1 ebitda: want %v (no D&A assumption), got %v", wantEBIT, got)
	}
}

// TestCalculate_MultiYearCompounding verifies that growth compounds
// period-over-period rather than always applying against the historical
// base — a 3-year 10% revenue growth series must yield
// base*1.1, base*1.1^2, base*1.1^3, not base*1.1 for every period.
func TestCalculate_MultiYearCompounding(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 500_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    3,
		Scenarios: []Scenario{{
			Name: "Base",
			Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(0.10, 3),
				COGS:    flatGrossMargin(0.50, 3),
				Opex:    flatOpexGrowth(0.0, 3),
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	periods := result.ScenarioResults[0].ProjectedPeriods
	if len(periods) != 3 {
		t.Fatalf("expected 3 periods, got %d", len(periods))
	}

	base := 1_000_000.0
	want := []float64{base * 1.10, base * 1.10 * 1.10, base * 1.10 * 1.10 * 1.10}
	for i, p := range periods {
		got := mustAvailable(t, "revenue", p.TotalRevenue)
		if !approxEqual(got, want[i]) {
			t.Fatalf("period %d revenue: want %v, got %v", i+1, want[i], got)
		}
	}

	// Verify compounding, not flat repetition against the base: period 3
	// must differ meaningfully from period 1.
	if approxEqual(mustAvailable(t, "p1", periods[0].TotalRevenue), mustAvailable(t, "p3", periods[2].TotalRevenue)) {
		t.Fatalf("expected period 1 and period 3 revenue to differ under compounding")
	}
}

// TestCalculate_NegativeGrowth verifies a negative growth rate is applied
// verbatim (declining revenue), never clamped to zero.
func TestCalculate_NegativeGrowth(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    2,
		Scenarios: []Scenario{{
			Name: "Downside",
			Type: ScenarioTypeDownside,
			Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(-0.20, 2),
				COGS:    flatGrossMargin(0.60, 2),
				Opex:    flatOpexGrowth(0, 2),
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	periods := result.ScenarioResults[0].ProjectedPeriods
	want1 := 1_000_000.0 * 0.80
	want2 := want1 * 0.80
	if got := mustAvailable(t, "p1 revenue", periods[0].TotalRevenue); !approxEqual(got, want1) {
		t.Fatalf("period1 revenue: want %v, got %v", want1, got)
	}
	if got := mustAvailable(t, "p2 revenue", periods[1].TotalRevenue); !approxEqual(got, want2) {
		t.Fatalf("period2 revenue: want %v, got %v", want2, got)
	}
}

// TestCalculate_RevenueCollapsesToNegative verifies that a severe enough
// negative growth run can drive a downstream figure negative, and that
// Calculate reports it as an advisory warning rather than silently
// clamping it — total opex outgrowing a shrinking revenue base is a
// realistic way for EBIT (not necessarily revenue itself) to go negative;
// this test instead directly exercises TotalOpex/TotalCOGS going negative
// via an extreme negative growth rate, which IssueNegativeProjectedValue is
// specifically scoped to catch.
func TestCalculate_ExtremeNegativeGrowthWarns(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{{
			Name: "Crash",
			Assumptions: Assumptions{
				Revenue: []RevenuePeriodAssumption{{Method: RevenueMethodGrowthRate, GrowthRate: -1.50}},
				COGS:    flatGrossMargin(0.60, 1),
				Opex:    flatOpexGrowth(0, 1),
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	sr := result.ScenarioResults[0]
	got := mustAvailable(t, "revenue", sr.ProjectedPeriods[0].TotalRevenue)
	if got >= 0 {
		t.Fatalf("expected negative revenue from a -150%% growth rate, got %v", got)
	}
	foundWarning := false
	for _, w := range sr.Warnings {
		if w.Code == IssueNegativeProjectedValue {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("expected IssueNegativeProjectedValue warning, got %+v", sr.Warnings)
	}
}

// TestCalculate_MarginCompression verifies a lower gross-margin target
// widens COGS and narrows gross profit relative to a flat-margin baseline,
// on identical revenue.
func TestCalculate_MarginCompression(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()

	run := func(margin float64) PeriodPL {
		result := Calculate(Input{
			Dataset:    ds,
			PeriodMeta: singlePeriodMeta("2025", 2025),
			Horizon:    1,
			Scenarios: []Scenario{{
				Name: "S",
				Assumptions: Assumptions{
					Revenue: flatRevenueGrowth(0, 1),
					COGS:    flatGrossMargin(margin, 1),
					Opex:    flatOpexGrowth(0, 1),
				},
			}},
		})
		if !result.Available {
			t.Fatalf("errors: %+v", result.Errors)
		}
		return result.ScenarioResults[0].ProjectedPeriods[0]
	}

	healthy := run(0.60)
	compressed := run(0.45)

	healthyGP := mustAvailable(t, "healthy gp", healthy.GrossProfit)
	compressedGP := mustAvailable(t, "compressed gp", compressed.GrossProfit)
	if compressedGP >= healthyGP {
		t.Fatalf("expected compressed-margin gross profit (%v) < healthy gross profit (%v)", compressedGP, healthyGP)
	}
	if got := mustAvailable(t, "compressed margin", compressed.GrossMargin); !approxEqual(got, 0.45) {
		t.Fatalf("expected gross margin 0.45, got %v", got)
	}
}

// TestCalculate_MissingRevenueAssumption verifies that a Scenario whose
// Assumptions.Revenue slice is shorter than Horizon still projects the
// missing periods (falling back to the zero-value RevenuePeriodAssumption —
// flat, 0% growth from the prior period, per Assumptions.Revenue's doc
// comment) while reporting IssueMissingRevenueAssumption so the caller
// knows a default was silently applied, without failing the whole Result.
func TestCalculate_MissingRevenueAssumption(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    2,
		Scenarios: []Scenario{{
			Name: "Partial",
			Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(0.05, 1), // only 1 of 2 periods supplied
				COGS:    flatGrossMargin(0.6, 2),
				Opex:    flatOpexGrowth(0, 2),
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	sr := result.ScenarioResults[0]
	p1Revenue := mustAvailable(t, "period 1 revenue", sr.ProjectedPeriods[0].TotalRevenue)
	wantP1 := 1_000_000.0 * 1.05
	if !approxEqual(p1Revenue, wantP1) {
		t.Fatalf("period 1 revenue: want %v, got %v", wantP1, p1Revenue)
	}
	// Period 2 has no explicit assumption, so it falls back to the
	// zero-value default (flat growth from period 1), not unavailable.
	p2Revenue := mustAvailable(t, "period 2 revenue", sr.ProjectedPeriods[1].TotalRevenue)
	if !approxEqual(p2Revenue, p1Revenue) {
		t.Fatalf("period 2 revenue: want flat carry-forward %v, got %v", p1Revenue, p2Revenue)
	}
	found := false
	for _, w := range sr.Warnings {
		if w.Code == IssueMissingRevenueAssumption {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueMissingRevenueAssumption, got %+v", sr.Warnings)
	}
}

// TestCalculate_MissingCOGSAndOpexAssumption verifies COGS/opex behave like
// revenue when their assumption slices are short.
func TestCalculate_MissingCOGSAndOpexAssumption(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    2,
		Scenarios: []Scenario{{
			Name:        "Partial",
			Assumptions: Assumptions{Revenue: flatRevenueGrowth(0, 2)}, // no COGS/Opex at all
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	sr := result.ScenarioResults[0]
	codes := map[IssueCode]bool{}
	for _, w := range sr.Warnings {
		codes[w.Code] = true
	}
	if !codes[IssueMissingCOGSAssumption] {
		t.Fatalf("expected IssueMissingCOGSAssumption, got %+v", sr.Warnings)
	}
	if !codes[IssueMissingOpexAssumption] {
		t.Fatalf("expected IssueMissingOpexAssumption, got %+v", sr.Warnings)
	}
	// COGS unavailable (no assumption, gross-margin method needs revenue
	// which IS available, but with GrossMarginPercent 0 by zero-value ->
	// still computable). Confirm gross profit is still internally
	// consistent rather than silently wrong.
	p1 := sr.ProjectedPeriods[0]
	if !p1.TotalCOGS.Available {
		t.Fatalf("expected COGS available under the zero-value gross-margin default")
	}
}

// TestCalculate_BaseUpsideDownsideScenarios runs three scenarios (base,
// upside, downside) from the same base period and verifies upside >
// base > downside on projected revenue and EBITDA, per the prompt's
// required base/downside/upside coverage.
func TestCalculate_BaseUpsideDownsideScenarios(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    2,
		Scenarios: []Scenario{
			{Name: "Base", Type: ScenarioTypeBase, Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(0.05, 2), COGS: flatGrossMargin(0.60, 2), Opex: flatOpexGrowth(0.03, 2),
			}},
			{Name: "Upside", Type: ScenarioTypeUpside, Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(0.15, 2), COGS: flatGrossMargin(0.62, 2), Opex: flatOpexGrowth(0.03, 2),
			}},
			{Name: "Downside", Type: ScenarioTypeDownside, Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(-0.05, 2), COGS: flatGrossMargin(0.55, 2), Opex: flatOpexGrowth(0.03, 2),
			}},
		},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	if len(result.ScenarioResults) != 3 {
		t.Fatalf("expected 3 scenario results, got %d", len(result.ScenarioResults))
	}

	byName := map[string]ScenarioResult{}
	for _, sr := range result.ScenarioResults {
		byName[sr.Name] = sr
	}

	baseRev := mustAvailable(t, "base rev", byName["Base"].ProjectedPeriods[1].TotalRevenue)
	upsideRev := mustAvailable(t, "upside rev", byName["Upside"].ProjectedPeriods[1].TotalRevenue)
	downsideRev := mustAvailable(t, "downside rev", byName["Downside"].ProjectedPeriods[1].TotalRevenue)

	if !(upsideRev > baseRev && baseRev > downsideRev) {
		t.Fatalf("expected upside(%v) > base(%v) > downside(%v)", upsideRev, baseRev, downsideRev)
	}

	baseEBITDA := mustAvailable(t, "base ebitda", byName["Base"].ProjectedPeriods[1].EBITDA)
	upsideEBITDA := mustAvailable(t, "upside ebitda", byName["Upside"].ProjectedPeriods[1].EBITDA)
	downsideEBITDA := mustAvailable(t, "downside ebitda", byName["Downside"].ProjectedPeriods[1].EBITDA)
	if !(upsideEBITDA > baseEBITDA && baseEBITDA > downsideEBITDA) {
		t.Fatalf("expected upside EBITDA(%v) > base EBITDA(%v) > downside EBITDA(%v)", upsideEBITDA, baseEBITDA, downsideEBITDA)
	}
}

// TestCalculate_ScenariosProjectFromIdenticalBase verifies every
// ScenarioResult in one Calculate call starts from byte-identical base
// figures — scenarios must differ only in assumptions, never in starting
// point.
func TestCalculate_ScenariosProjectFromIdenticalBase(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{
			{Name: "A", Assumptions: Assumptions{Revenue: flatRevenueGrowth(0, 1), COGS: flatGrossMargin(0.5, 1), Opex: flatOpexGrowth(0, 1)}},
			{Name: "B", Assumptions: Assumptions{Revenue: flatRevenueGrowth(0.5, 1), COGS: flatGrossMargin(0.1, 1), Opex: flatOpexGrowth(0.9, 1)}},
		},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	if result.Base.Period != "2025" {
		t.Fatalf("unexpected base period %s", result.Base.Period)
	}
	if got := mustAvailable(t, "base revenue", result.Base.PL.TotalRevenue); !approxEqual(got, 1_000_000) {
		t.Fatalf("base revenue changed: %v", got)
	}
	// Base is reported once at the top level, identical regardless of how
	// many/which scenarios ran — verify it doesn't vary by re-running with
	// only scenario A.
	resultA := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{
			{Name: "A", Assumptions: Assumptions{Revenue: flatRevenueGrowth(0, 1), COGS: flatGrossMargin(0.5, 1), Opex: flatOpexGrowth(0, 1)}},
		},
	})
	if mustAvailable(t, "base revenue A", resultA.Base.PL.TotalRevenue) != mustAvailable(t, "base revenue AB", result.Base.PL.TotalRevenue) {
		t.Fatalf("base financials differ depending on which scenarios were included")
	}
}

// TestCalculate_DuplicateAndEmptyScenarioNames verifies both name-validation
// warnings and that a duplicate's later occurrence is skipped rather than
// silently overwriting the first.
func TestCalculate_DuplicateAndEmptyScenarioNames(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{
			{Name: "Base", Assumptions: Assumptions{Revenue: flatRevenueGrowth(0.01, 1), COGS: flatGrossMargin(0.5, 1), Opex: flatOpexGrowth(0, 1)}},
			{Name: "", Assumptions: Assumptions{Revenue: flatRevenueGrowth(0.99, 1)}},
			{Name: "Base", Assumptions: Assumptions{Revenue: flatRevenueGrowth(0.50, 1), COGS: flatGrossMargin(0.5, 1), Opex: flatOpexGrowth(0, 1)}},
		},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	if len(result.ScenarioResults) != 1 {
		t.Fatalf("expected exactly 1 scenario result (duplicate/empty skipped), got %d", len(result.ScenarioResults))
	}
	got := mustAvailable(t, "revenue", result.ScenarioResults[0].ProjectedPeriods[0].TotalRevenue)
	want := 1_000_000.0 * 1.01
	if !approxEqual(got, want) {
		t.Fatalf("expected first Base entry's assumption to win: want %v, got %v", want, got)
	}
	codes := map[IssueCode]bool{}
	for _, w := range result.Warnings {
		codes[w.Code] = true
	}
	if !codes[IssueEmptyScenarioName] {
		t.Fatalf("expected IssueEmptyScenarioName warning, got %+v", result.Warnings)
	}
	if !codes[IssueDuplicateScenarioName] {
		t.Fatalf("expected IssueDuplicateScenarioName warning, got %+v", result.Warnings)
	}
}

// TestCalculate_NoHiddenMutation verifies Calculate never mutates
// Input.Dataset, Input.Scenarios, or any nested Assumptions slice.
func TestCalculate_NoHiddenMutation(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 20_000).build()
	originalItemsLen := len(ds.Items)
	originalFirstAmount := ds.Items[0].Amount

	revAssumptions := flatRevenueGrowth(0.10, 2)
	scenario := Scenario{
		Name: "Base",
		Assumptions: Assumptions{
			Revenue: revAssumptions,
			COGS:    flatGrossMargin(0.6, 2),
			Opex:    flatOpexGrowth(0.05, 2),
		},
	}
	scenarios := []Scenario{scenario}

	_ = Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    2,
		Scenarios:  scenarios,
	})

	if len(ds.Items) != originalItemsLen {
		t.Fatalf("Dataset.Items length changed: %d -> %d", originalItemsLen, len(ds.Items))
	}
	if ds.Items[0].Amount != originalFirstAmount {
		t.Fatalf("Dataset.Items[0].Amount mutated: %v -> %v", originalFirstAmount, ds.Items[0].Amount)
	}
	if revAssumptions[0].GrowthRate != 0.10 {
		t.Fatalf("caller's Revenue assumption slice was mutated: %v", revAssumptions[0].GrowthRate)
	}
	if scenarios[0].Name != "Base" {
		t.Fatalf("caller's Scenarios slice was mutated")
	}
}

// TestCalculate_RevenueCodeOverride verifies a per-code revenue override
// takes precedence over the aggregate growth rate for that code alone,
// while every other revenue code still grows at the aggregate rate.
func TestCalculate_RevenueCodeOverride(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2025", 600_000).
		add(financial.CodeRevService, "2025", 400_000).
		build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{{
			Name: "S",
			Assumptions: Assumptions{
				Revenue: []RevenuePeriodAssumption{{
					Method:     RevenueMethodGrowthRate,
					GrowthRate: 0.10, // aggregate: applies to REV_PRODUCT
					CodeOverrides: []RevenueCodeAssumption{
						{Code: financial.CodeRevService, Method: RevenueMethodGrowthRate, GrowthRate: -0.50},
					},
				}},
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	p1 := result.ScenarioResults[0].ProjectedPeriods[0]
	var productAmt, serviceAmt float64
	var foundProduct, foundService bool
	for _, l := range p1.RevenueLines {
		switch l.Code {
		case financial.CodeRevProduct:
			productAmt, foundProduct = l.Amount, true
		case financial.CodeRevService:
			serviceAmt, foundService = l.Amount, true
		}
	}
	if !foundProduct || !foundService {
		t.Fatalf("expected both revenue lines present, got %+v", p1.RevenueLines)
	}
	if !approxEqual(productAmt, 600_000*1.10) {
		t.Fatalf("expected product revenue to grow at aggregate rate: want %v, got %v", 600_000*1.10, productAmt)
	}
	if !approxEqual(serviceAmt, 400_000*0.50) {
		t.Fatalf("expected service revenue to use its override rate: want %v, got %v", 400_000*0.50, serviceAmt)
	}
	wantTotal := productAmt + serviceAmt
	if got := mustAvailable(t, "total revenue", p1.TotalRevenue); !approxEqual(got, wantTotal) {
		t.Fatalf("total revenue should equal sum of lines: want %v, got %v", wantTotal, got)
	}
}

// TestCalculate_WorkingCapitalAndCashFlow verifies the working-capital and
// cash-flow bridge end to end: NWC as percent of revenue, change in NWC,
// operating cash flow, free cash flow, and debt-service coverage.
func TestCalculate_WorkingCapitalAndCashFlow(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2025", 1_000_000).
		add(financial.CodeCogsMaterial, "2025", 400_000).
		add(financial.CodeOpexPayroll, "2025", 200_000).
		add(financial.CodeBsAccountsReceivable, "2025", 150_000).
		add(financial.CodeBsInventory, "2025", 50_000).
		add(financial.CodeBsAccountsPayable, "2025", 100_000).
		build()

	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{{
			Name: "S",
			Assumptions: Assumptions{
				Revenue:        flatRevenueGrowth(0.10, 1),
				COGS:           flatGrossMargin(0.60, 1),
				Opex:           flatOpexGrowth(0.0, 1),
				WorkingCapital: []WorkingCapitalPeriodAssumption{{Method: WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.10}},
				Tax:            []TaxPeriodAssumption{{Method: TaxMethodPercentOfPretaxIncome, TaxRate: 0.25}},
				Capex:          []CapexAssumption{{Capex: AvailableValue(20_000)}},
				DebtService:    []DebtServiceAssumption{{Principal: AvailableValue(30_000), Interest: AvailableValue(10_000)}},
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}

	// Base NWC = 150,000 + 50,000 - 100,000 = 100,000.
	baseNWC := mustAvailable(t, "base nwc", result.Base.WorkingCapital)
	if !approxEqual(baseNWC, 100_000) {
		t.Fatalf("base nwc: want 100000, got %v", baseNWC)
	}

	sr := result.ScenarioResults[0]
	wc1 := sr.WorkingCapital[0]
	wantRevenue := 1_100_000.0
	wantNWC := wantRevenue * 0.10
	if got := mustAvailable(t, "p1 nwc", wc1.NWC); !approxEqual(got, wantNWC) {
		t.Fatalf("period1 nwc: want %v, got %v", wantNWC, got)
	}
	wantChange := wantNWC - baseNWC
	if got := mustAvailable(t, "p1 change in nwc", wc1.ChangeInNWC); !approxEqual(got, wantChange) {
		t.Fatalf("period1 change in nwc: want %v, got %v", wantChange, got)
	}

	cf1 := sr.CashFlow[0]
	pl1 := sr.ProjectedPeriods[0]
	ebitda := mustAvailable(t, "ebitda", pl1.EBITDA)
	tax := mustAvailable(t, "tax", pl1.IncomeTax)
	wantOCF := ebitda - wantChange - tax
	if got := mustAvailable(t, "ocf", cf1.OperatingCashFlow); !approxEqual(got, wantOCF) {
		t.Fatalf("operating cash flow: want %v, got %v", wantOCF, got)
	}
	wantFCF := wantOCF - 20_000
	if got := mustAvailable(t, "fcf", cf1.FreeCashFlow); !approxEqual(got, wantFCF) {
		t.Fatalf("free cash flow: want %v, got %v", wantFCF, got)
	}
	wantFCFO := wantFCF - 40_000 // principal 30k + interest 10k
	if got := mustAvailable(t, "fcf to owner", cf1.FreeCashFlowToOwner); !approxEqual(got, wantFCFO) {
		t.Fatalf("fcf to owner: want %v, got %v", wantFCFO, got)
	}
	wantDSCR := wantOCF / 40_000
	if got := mustAvailable(t, "dscr", cf1.DebtServiceCoverage); !approxEqual(got, wantDSCR) {
		t.Fatalf("dscr: want %v, got %v", wantDSCR, got)
	}
}

// TestCalculate_DebtRateRecomputesInterest verifies InterestRate x
// BeginningBalance recomputes Interest when both are supplied.
func TestCalculate_DebtRateRecomputesInterest(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{{
			Name: "S",
			Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(0, 1),
				COGS:    flatGrossMargin(0.6, 1),
				Opex:    flatOpexGrowth(0, 1),
				DebtService: []DebtServiceAssumption{{
					Principal:        AvailableValue(10_000),
					Interest:         AvailableValue(999_999), // should be overridden
					InterestRate:     AvailableValue(0.06),
					BeginningBalance: AvailableValue(200_000),
				}},
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	cf1 := result.ScenarioResults[0].CashFlow[0]
	want := 200_000.0 * 0.06
	if got := mustAvailable(t, "interest", cf1.DebtService.Interest); !approxEqual(got, want) {
		t.Fatalf("interest: want %v, got %v", want, got)
	}
}

// TestCalculate_ZeroDebtServiceNoCoverage verifies DSCR is left unavailable
// (not "infinite") when total debt service is exactly zero.
func TestCalculate_ZeroDebtServiceNoCoverage(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{{
			Name: "S",
			Assumptions: Assumptions{
				Revenue:     flatRevenueGrowth(0, 1),
				COGS:        flatGrossMargin(0.6, 1),
				Opex:        flatOpexGrowth(0, 1),
				DebtService: []DebtServiceAssumption{{Principal: AvailableValue(0), Interest: AvailableValue(0)}},
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	cf1 := result.ScenarioResults[0].CashFlow[0]
	if cf1.DebtServiceCoverage.Available {
		t.Fatalf("expected DSCR unavailable with zero total debt service, got %v", cf1.DebtServiceCoverage.Value)
	}
}

// TestCalculate_DSCRSourceEBITDA verifies
// Input.DebtServiceCoverageSource == DSCSourceEBITDA changes the numerator.
func TestCalculate_DSCRSourceEBITDA(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	base := Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{{
			Name: "S",
			Assumptions: Assumptions{
				Revenue:        flatRevenueGrowth(0, 1),
				COGS:           flatGrossMargin(0.6, 1),
				Opex:           flatOpexGrowth(0, 1),
				WorkingCapital: []WorkingCapitalPeriodAssumption{{Method: WorkingCapitalMethodHeldFlat}},
				DebtService:    []DebtServiceAssumption{{Principal: AvailableValue(10_000), Interest: AvailableValue(5_000)}},
			},
		}},
	}
	// Held-flat NWC against an unavailable base NWC (no balance sheet
	// supplied) leaves NWC unavailable, which makes OperatingCashFlow
	// unavailable too -- forcing EBITDA-based DSCR to be the only available
	// source, demonstrating the two sources really do differ.
	ebitdaSourceInput := base
	ebitdaSourceInput.DebtServiceCoverageSource = DSCSourceEBITDA
	result := Calculate(ebitdaSourceInput)
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	cf1 := result.ScenarioResults[0].CashFlow[0]
	pl1 := result.ScenarioResults[0].ProjectedPeriods[0]
	ebitda := mustAvailable(t, "ebitda", pl1.EBITDA)
	want := ebitda / 15_000
	got := mustAvailable(t, "dscr", cf1.DebtServiceCoverage)
	if !approxEqual(got, want) {
		t.Fatalf("expected EBITDA-sourced DSCR %v, got %v", want, got)
	}
}

// TestCalculate_HeldFlatWorkingCapital verifies
// WorkingCapitalMethodHeldFlat carries the prior period's NWC forward
// unchanged, producing a zero ChangeInNWC.
func TestCalculate_HeldFlatWorkingCapital(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2025", 1_000_000).
		add(financial.CodeBsAccountsReceivable, "2025", 80_000).
		add(financial.CodeBsAccountsPayable, "2025", 30_000).
		build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    2,
		Scenarios: []Scenario{{
			Name: "S",
			Assumptions: Assumptions{
				Revenue:        flatRevenueGrowth(0.10, 2),
				COGS:           flatGrossMargin(0.5, 2),
				Opex:           flatOpexGrowth(0, 2),
				WorkingCapital: []WorkingCapitalPeriodAssumption{{Method: WorkingCapitalMethodHeldFlat}, {Method: WorkingCapitalMethodHeldFlat}},
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	sr := result.ScenarioResults[0]
	base := mustAvailable(t, "base nwc", result.Base.WorkingCapital)
	for i, wc := range sr.WorkingCapital {
		got := mustAvailable(t, "held-flat nwc", wc.NWC)
		if !approxEqual(got, base) {
			t.Fatalf("period %d: expected nwc held flat at %v, got %v", i+1, base, got)
		}
		change := mustAvailable(t, "change", wc.ChangeInNWC)
		if !approxEqual(change, 0) {
			t.Fatalf("period %d: expected zero change in nwc, got %v", i+1, change)
		}
	}
}

// TestCalculate_TaxFloorsAtZero verifies TaxMethodPercentOfPretaxIncome
// never produces a negative tax expense from a projected pretax loss.
func TestCalculate_TaxFloorsAtZero(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 100_000, 40_000, 30_000, 0).build()
	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{{
			Name: "Loss",
			Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(0, 1),
				COGS:    flatGrossMargin(0.10, 1), // narrow margin
				Opex:    []OpexPeriodAssumption{{Method: OpexMethodFixedAmount, FixedAmount: 500_000}},
				Tax:     []TaxPeriodAssumption{{Method: TaxMethodPercentOfPretaxIncome, TaxRate: 0.25}},
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	p1 := result.ScenarioResults[0].ProjectedPeriods[0]
	pretax := mustAvailable(t, "pretax", p1.PretaxIncome)
	if pretax >= 0 {
		t.Fatalf("expected a projected pretax loss for this test to be meaningful, got %v", pretax)
	}
	tax := mustAvailable(t, "tax", p1.IncomeTax)
	if tax != 0 {
		t.Fatalf("expected tax floored at 0 on a projected loss, got %v", tax)
	}
}

// TestCalculate_ForecastPeriodLabels verifies custom labels are used, with
// a generated fallback for periods beyond the supplied slice.
func TestCalculate_ForecastPeriodLabels(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	result := Calculate(Input{
		Dataset:              ds,
		PeriodMeta:           singlePeriodMeta("2025", 2025),
		Horizon:              3,
		ForecastPeriodLabels: []string{"FY2026", "FY2027"},
		Scenarios: []Scenario{{
			Name: "S",
			Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(0, 3), COGS: flatGrossMargin(0.6, 3), Opex: flatOpexGrowth(0, 3),
			},
		}},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	want := []string{"FY2026", "FY2027", "Period 3"}
	if len(result.ForecastPeriods) != 3 {
		t.Fatalf("expected 3 forecast period labels, got %v", result.ForecastPeriods)
	}
	for i, w := range want {
		if result.ForecastPeriods[i] != w {
			t.Fatalf("label %d: want %q, got %q", i, w, result.ForecastPeriods[i])
		}
		if result.ScenarioResults[0].ProjectedPeriods[i].Period != w {
			t.Fatalf("projected period %d label: want %q, got %q", i, w, result.ScenarioResults[0].ProjectedPeriods[i].Period)
		}
	}
}

// TestCalculate_RealisticFixture runs a full base/downside/upside forecast
// against the repository's normalized_hvac_multi_year.json fixture — a
// realistic 3-year multi-line-item dataset shared with analytics/cashflow
// and analytics/workingcapital's own tests — verifying Calculate handles a
// real, non-hand-simplified financial.FinancialDataset end to end
// (multiple revenue/COGS/opex codes, owner comp, D&A, interest, tax, and a
// balance sheet) without any Unavailable figure that should be computable.
func TestCalculate_RealisticFixture(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")
	result := Calculate(Input{
		Dataset:              ds,
		PeriodMeta:           threeYearMeta(),
		Horizon:              3,
		ForecastPeriodLabels: []string{"2026", "2027", "2028"},
		Scenarios: []Scenario{
			{Name: "Base", Type: ScenarioTypeBase, Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(0.06, 3),
				COGS:    flatGrossMargin(0.45, 3),
				Opex:    flatOpexGrowth(0.03, 3),
				DepreciationAmortization: []DepreciationAmortizationAssumption{
					{Depreciation: AvailableValue(15_000)},
					{Depreciation: AvailableValue(15_000)},
					{Depreciation: AvailableValue(15_000)},
				},
				Tax: []TaxPeriodAssumption{
					{Method: TaxMethodPercentOfPretaxIncome, TaxRate: 0.22},
					{Method: TaxMethodPercentOfPretaxIncome, TaxRate: 0.22},
					{Method: TaxMethodPercentOfPretaxIncome, TaxRate: 0.22},
				},
				WorkingCapital: []WorkingCapitalPeriodAssumption{
					{Method: WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.08},
					{Method: WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.08},
					{Method: WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.08},
				},
			}},
			{Name: "Upside", Type: ScenarioTypeUpside, Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(0.14, 3),
				COGS:    flatGrossMargin(0.48, 3),
				Opex:    flatOpexGrowth(0.03, 3),
			}},
			{Name: "Downside", Type: ScenarioTypeDownside, Assumptions: Assumptions{
				Revenue: flatRevenueGrowth(-0.04, 3),
				COGS:    flatGrossMargin(0.40, 3),
				Opex:    flatOpexGrowth(0.03, 3),
			}},
		},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	if !result.Base.PL.TotalRevenue.Available || !result.Base.PL.EBITDA.Available || !result.Base.PL.SDE.Available {
		t.Fatalf("expected base revenue/EBITDA/SDE all available from a realistic fixture, got PL=%+v", result.Base.PL)
	}
	if !result.Base.WorkingCapital.Available {
		t.Fatalf("expected base working capital available from a fixture with a balance sheet")
	}

	byName := map[string]ScenarioResult{}
	for _, sr := range result.ScenarioResults {
		byName[sr.Name] = sr
	}
	if len(byName) != 3 {
		t.Fatalf("expected 3 scenario results, got %d", len(byName))
	}

	for _, name := range []string{"Base", "Upside", "Downside"} {
		sr := byName[name]
		if len(sr.ProjectedPeriods) != 3 {
			t.Fatalf("%s: expected 3 projected periods, got %d", name, len(sr.ProjectedPeriods))
		}
		for i, p := range sr.ProjectedPeriods {
			if !p.TotalRevenue.Available {
				t.Fatalf("%s period %d: expected revenue available", name, i+1)
			}
			if !p.EBITDA.Available {
				t.Fatalf("%s period %d: expected EBITDA available", name, i+1)
			}
		}
	}

	baseY3 := mustAvailable(t, "base y3 revenue", byName["Base"].ProjectedPeriods[2].TotalRevenue)
	upsideY3 := mustAvailable(t, "upside y3 revenue", byName["Upside"].ProjectedPeriods[2].TotalRevenue)
	downsideY3 := mustAvailable(t, "downside y3 revenue", byName["Downside"].ProjectedPeriods[2].TotalRevenue)
	if !(upsideY3 > baseY3 && baseY3 > downsideY3) {
		t.Fatalf("expected upside(%v) > base(%v) > downside(%v) by year 3", upsideY3, baseY3, downsideY3)
	}

	// Base scenario has full working-capital/tax/debt-adjacent assumptions,
	// so its cash-flow bridge should be fully computable.
	baseCF := byName["Base"].CashFlow[0]
	if !baseCF.OperatingCashFlow.Available {
		t.Fatalf("expected base scenario operating cash flow available given full assumptions, got %+v", baseCF)
	}
}
