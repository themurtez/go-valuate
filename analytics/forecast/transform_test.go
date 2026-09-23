package forecast

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestApplyRevenueShock_ShiftsGrowthRateAndDoesNotMutate verifies the shock
// shifts every in-window period's growth rate and never mutates the input
// Assumptions.
func TestApplyRevenueShock_ShiftsGrowthRateAndDoesNotMutate(t *testing.T) {
	base := Assumptions{Revenue: flatRevenueGrowth(0.10, 3)}
	shocked := ApplyRevenueShock(base, -0.15, 1, 3)

	for i, p := range shocked.Revenue {
		want := 0.10 - 0.15
		if !approxEqual(p.GrowthRate, want) {
			t.Fatalf("period %d: want growth rate %v, got %v", i, want, p.GrowthRate)
		}
	}
	// Original untouched.
	for i, p := range base.Revenue {
		if !approxEqual(p.GrowthRate, 0.10) {
			t.Fatalf("original mutated at period %d: %v", i, p.GrowthRate)
		}
	}
}

// TestApplyRevenueShock_PartialWindow verifies fromPeriod scopes the shock
// to periods >= fromPeriod only.
func TestApplyRevenueShock_PartialWindow(t *testing.T) {
	base := Assumptions{Revenue: flatRevenueGrowth(0.10, 4)}
	shocked := ApplyRevenueShock(base, -0.05, 3, 4) // shock periods 3,4 only (index 2,3)

	for i := 0; i < 2; i++ {
		if !approxEqual(shocked.Revenue[i].GrowthRate, 0.10) {
			t.Fatalf("period %d should be unshocked: got %v", i+1, shocked.Revenue[i].GrowthRate)
		}
	}
	for i := 2; i < 4; i++ {
		want := 0.05
		if !approxEqual(shocked.Revenue[i].GrowthRate, want) {
			t.Fatalf("period %d should be shocked to %v: got %v", i+1, want, shocked.Revenue[i].GrowthRate)
		}
	}
}

// TestApplyRevenueShock_SkipsFixedAmountPeriods verifies a period using
// RevenueMethodFixedAmount is left untouched (shifting a "growth rate" has
// no meaning against a fixed dollar figure).
func TestApplyRevenueShock_SkipsFixedAmountPeriods(t *testing.T) {
	base := Assumptions{Revenue: []RevenuePeriodAssumption{
		{Method: RevenueMethodFixedAmount, FixedAmount: 500_000},
		{Method: RevenueMethodGrowthRate, GrowthRate: 0.10},
	}}
	shocked := ApplyRevenueShock(base, -0.20, 1, 2)
	if shocked.Revenue[0].FixedAmount != 500_000 {
		t.Fatalf("fixed-amount period should be untouched, got %v", shocked.Revenue[0].FixedAmount)
	}
	if !approxEqual(shocked.Revenue[1].GrowthRate, -0.10) {
		t.Fatalf("growth-rate period should be shocked, got %v", shocked.Revenue[1].GrowthRate)
	}
}

// TestApplyRevenueShock_EndToEnd verifies the shocked Assumptions actually
// produces lower projected revenue when run through Calculate.
func TestApplyRevenueShock_EndToEnd(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	base := Assumptions{
		Revenue: flatRevenueGrowth(0.10, 1),
		COGS:    flatGrossMargin(0.6, 1),
		Opex:    flatOpexGrowth(0, 1),
	}
	downside := ApplyRevenueShock(base, -0.30, 1, 1)

	result := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    1,
		Scenarios: []Scenario{
			{Name: "Base", Assumptions: base},
			{Name: "Downside", Assumptions: downside},
		},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	baseRev := mustAvailable(t, "base rev", result.ScenarioResults[0].ProjectedPeriods[0].TotalRevenue)
	downsideRev := mustAvailable(t, "downside rev", result.ScenarioResults[1].ProjectedPeriods[0].TotalRevenue)
	if downsideRev >= baseRev {
		t.Fatalf("expected shocked revenue (%v) < base revenue (%v)", downsideRev, baseRev)
	}
	wantDownside := 1_000_000.0 * (1 + 0.10 - 0.30)
	if !approxEqual(downsideRev, wantDownside) {
		t.Fatalf("downside revenue: want %v, got %v", wantDownside, downsideRev)
	}
}

// TestApplyMarginShock verifies gross-margin-percent shifting and that a
// growth-rate/fixed-amount COGS period is left untouched.
func TestApplyMarginShock(t *testing.T) {
	base := Assumptions{COGS: []COGSPeriodAssumption{
		{Method: COGSMethodGrossMarginPercent, GrossMarginPercent: 0.60},
		{Method: COGSMethodGrowthRate, GrowthRate: 0.05},
	}}
	shocked := ApplyMarginShock(base, -0.10, 1, 2)
	if !approxEqual(shocked.COGS[0].GrossMarginPercent, 0.50) {
		t.Fatalf("margin period: want 0.50, got %v", shocked.COGS[0].GrossMarginPercent)
	}
	if !approxEqual(shocked.COGS[1].GrowthRate, 0.05) {
		t.Fatalf("growth-rate period should be untouched, got %v", shocked.COGS[1].GrowthRate)
	}
	// Original untouched.
	if !approxEqual(base.COGS[0].GrossMarginPercent, 0.60) {
		t.Fatalf("original mutated: %v", base.COGS[0].GrossMarginPercent)
	}
}

// TestApplyExpenseShock_Aggregate verifies the no-codes aggregate path.
func TestApplyExpenseShock_Aggregate(t *testing.T) {
	base := Assumptions{Opex: flatOpexGrowth(0.05, 2)}
	shocked := ApplyExpenseShock(base, 0.10, nil, 1, 2)
	for i, p := range shocked.Opex {
		want := 0.15
		if !approxEqual(p.GrowthRate, want) {
			t.Fatalf("period %d: want %v, got %v", i, want, p.GrowthRate)
		}
	}
	if !approxEqual(base.Opex[0].GrowthRate, 0.05) {
		t.Fatalf("original mutated: %v", base.Opex[0].GrowthRate)
	}
}

// TestApplyExpenseShock_TargetedCodes verifies a per-code shock only
// affects the named codes, appending a new override where none existed and
// shifting an existing growth-rate override where one did.
func TestApplyExpenseShock_TargetedCodes(t *testing.T) {
	base := Assumptions{
		Opex: []OpexPeriodAssumption{
			{
				Method:     OpexMethodGrowthRate,
				GrowthRate: 0.03,
				CodeOverrides: []OpexCodeAssumption{
					{Code: financial.CodeOpexRent, Method: OpexMethodGrowthRate, GrowthRate: 0.02},
				},
			},
		},
	}
	shocked := ApplyExpenseShock(base, 0.20, []financial.Code{financial.CodeOpexRent, financial.CodeOpexMarketing}, 1, 1)

	// Aggregate rate untouched.
	if !approxEqual(shocked.Opex[0].GrowthRate, 0.03) {
		t.Fatalf("aggregate rate should be untouched by a targeted shock, got %v", shocked.Opex[0].GrowthRate)
	}

	var rentRate, marketingRate float64
	var foundRent, foundMarketing bool
	for _, ov := range shocked.Opex[0].CodeOverrides {
		switch ov.Code {
		case financial.CodeOpexRent:
			rentRate, foundRent = ov.GrowthRate, true
		case financial.CodeOpexMarketing:
			marketingRate, foundMarketing = ov.GrowthRate, true
		}
	}
	if !foundRent || !approxEqual(rentRate, 0.22) {
		t.Fatalf("rent override: want existing 0.02 shifted by 0.20 = 0.22, got %v (found=%v)", rentRate, foundRent)
	}
	if !foundMarketing || !approxEqual(marketingRate, 0.20) {
		t.Fatalf("marketing override: want newly-appended 0.20, got %v (found=%v)", marketingRate, foundMarketing)
	}
	// Original's override slice untouched.
	if len(base.Opex[0].CodeOverrides) != 1 {
		t.Fatalf("original CodeOverrides slice was mutated: %+v", base.Opex[0].CodeOverrides)
	}
	if !approxEqual(base.Opex[0].CodeOverrides[0].GrowthRate, 0.02) {
		t.Fatalf("original override rate mutated: %v", base.Opex[0].CodeOverrides[0].GrowthRate)
	}
}

// TestApplyCustomerLossShock verifies a step-down in revenue at fromPeriod
// that persists (as a lowered base) through subsequent periods, using
// end-to-end Calculate figures for clarity.
func TestApplyCustomerLossShock(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	base := Assumptions{
		Revenue: flatRevenueGrowth(0.10, 3),
		COGS:    flatGrossMargin(0.6, 3),
		Opex:    flatOpexGrowth(0, 3),
	}
	baseResult := Calculate(Input{
		Dataset: ds, PeriodMeta: singlePeriodMeta("2025", 2025), Horizon: 3,
		Scenarios: []Scenario{{Name: "Base", Assumptions: base}},
	})
	if !baseResult.Available {
		t.Fatalf("errors: %+v", baseResult.Errors)
	}
	baseP1Revenue := mustAvailable(t, "base p1 revenue", baseResult.ScenarioResults[0].ProjectedPeriods[0].TotalRevenue)

	shocked := ApplyCustomerLossShock(base, 0.25, baseResult.Base.PL, 2, 3)
	shockedResult := Calculate(Input{
		Dataset: ds, PeriodMeta: singlePeriodMeta("2025", 2025), Horizon: 3,
		Scenarios: []Scenario{{Name: "Shocked", Assumptions: shocked}},
	})
	if !shockedResult.Available {
		t.Fatalf("errors: %+v", shockedResult.Errors)
	}
	sp := shockedResult.ScenarioResults[0].ProjectedPeriods

	// Period 1 (before the shock window) is unaffected.
	shockedP1Revenue := mustAvailable(t, "shocked p1 revenue", sp[0].TotalRevenue)
	if !approxEqual(shockedP1Revenue, baseP1Revenue) {
		t.Fatalf("period 1 should be unaffected by a period-2 shock: base %v, shocked %v", baseP1Revenue, shockedP1Revenue)
	}

	// Period 2 drops by ~25% relative to what it otherwise would have been.
	baseP2Revenue := mustAvailable(t, "base p2 revenue", baseResult.ScenarioResults[0].ProjectedPeriods[1].TotalRevenue)
	shockedP2Revenue := mustAvailable(t, "shocked p2 revenue", sp[1].TotalRevenue)
	wantP2 := baseP2Revenue * 0.75
	if !approxEqual(shockedP2Revenue, wantP2) {
		t.Fatalf("period 2 revenue: want %v (25%% loss), got %v", wantP2, shockedP2Revenue)
	}

	// Period 3 continues compounding at the original 10% rate from the
	// newly-lowered period-2 base, not snapping back to the unshocked
	// trajectory.
	shockedP3Revenue := mustAvailable(t, "shocked p3 revenue", sp[2].TotalRevenue)
	wantP3 := wantP2 * 1.10
	if !approxEqual(shockedP3Revenue, wantP3) {
		t.Fatalf("period 3 revenue: want %v (compounding from lowered base), got %v", wantP3, shockedP3Revenue)
	}
}

// TestApplyOneTimeCostShock verifies a single-period cost addition that
// does not compound into later periods — checked across three periods so
// the fix (auto-pinning period+1's growth base) is shown to hold beyond
// just the immediate next period: period 3 must grow cleanly from period
// 2's now-organic figure, not re-inherit any trace of the period-1 spike.
func TestApplyOneTimeCostShock(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	base := Assumptions{
		Revenue: flatRevenueGrowth(0, 3),
		COGS:    flatGrossMargin(0.6, 3),
		Opex:    flatOpexGrowth(0.05, 3),
	}
	shocked := ApplyOneTimeCostShock(base, 50_000, financial.CodeOpexOther, 1, 3)

	result := Calculate(Input{
		Dataset: ds, PeriodMeta: singlePeriodMeta("2025", 2025), Horizon: 3,
		Scenarios: []Scenario{
			{Name: "Base", Assumptions: base},
			{Name: "Shocked", Assumptions: shocked},
		},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	baseOpex := result.ScenarioResults[0].ProjectedPeriods
	shockedOpex := result.ScenarioResults[1].ProjectedPeriods

	p1Base := mustAvailable(t, "base opex p1", baseOpex[0].TotalOpex)
	p1Shocked := mustAvailable(t, "shocked opex p1", shockedOpex[0].TotalOpex)
	if !approxEqual(p1Shocked, p1Base+50_000) {
		t.Fatalf("period 1 opex: want base+50000=%v, got %v", p1Base+50_000, p1Shocked)
	}

	for i := 1; i < 3; i++ {
		wantBase := mustAvailable(t, "base opex", baseOpex[i].TotalOpex)
		gotShocked := mustAvailable(t, "shocked opex", shockedOpex[i].TotalOpex)
		if !approxEqual(gotShocked, wantBase) {
			t.Fatalf("period %d opex should not carry the one-time cost forward: base %v, shocked %v", i+1, wantBase, gotShocked)
		}
	}
}

// TestOpexMethodExcludeAmount_Direct unit-tests projectOpex's handling of
// OpexMethodExcludeAmount directly (rather than only through the
// ApplyOneTimeCostShock helper that sets it automatically): the excluded
// amount is subtracted from the prior period's reported figure for that
// code before GrowthRate is applied.
func TestOpexMethodExcludeAmount_Direct(t *testing.T) {
	prior := PeriodPL{OpexLines: []LineItem{
		{Code: financial.CodeOpexOther, Amount: 150_000}, // 100,000 organic + 50,000 one-time
	}}
	assumption := OpexPeriodAssumption{
		CodeOverrides: []OpexCodeAssumption{
			{Code: financial.CodeOpexOther, Method: OpexMethodExcludeAmount, GrowthRate: 0.10, ExcludeFromBase: 50_000},
		},
	}
	lines, total := projectOpex(assumption, prior)
	want := 100_000.0 * 1.10 // (150,000 - 50,000) * 1.10
	if len(lines) != 1 || !approxEqual(lines[0].Amount, want) {
		t.Fatalf("want single line at %v, got %+v", want, lines)
	}
	if got := mustAvailable(t, "total", total); !approxEqual(got, want) {
		t.Fatalf("total: want %v, got %v", want, got)
	}
}

// TestApplyOneTimeCostShock_Stacks verifies applying the helper twice to
// the same code/period accumulates rather than replaces.
func TestApplyOneTimeCostShock_Stacks(t *testing.T) {
	base := Assumptions{Opex: flatOpexGrowth(0, 1)}
	once := ApplyOneTimeCostShock(base, 10_000, financial.CodeOpexOther, 1, 1)
	twice := ApplyOneTimeCostShock(once, 5_000, financial.CodeOpexOther, 1, 1)

	if len(twice.Opex[0].CodeOverrides) != 1 {
		t.Fatalf("expected a single accumulated override, got %d", len(twice.Opex[0].CodeOverrides))
	}
	if !approxEqual(twice.Opex[0].CodeOverrides[0].FixedAmount, 15_000) {
		t.Fatalf("expected accumulated amount 15000, got %v", twice.Opex[0].CodeOverrides[0].FixedAmount)
	}
}

// TestApplyDebtRateShock verifies InterestRate shifts only when both
// InterestRate and BeginningBalance are already available.
func TestApplyDebtRateShock(t *testing.T) {
	base := Assumptions{DebtService: []DebtServiceAssumption{
		{InterestRate: AvailableValue(0.05), BeginningBalance: AvailableValue(100_000)},
		{Interest: AvailableValue(5_000)}, // flat figure, no rate/balance
	}}
	shocked := ApplyDebtRateShock(base, 0.02, 1, 2)

	got := mustAvailable(t, "shocked rate", shocked.DebtService[0].InterestRate)
	if !approxEqual(got, 0.07) {
		t.Fatalf("want rate 0.07, got %v", got)
	}
	if shocked.DebtService[1].InterestRate.Available {
		t.Fatalf("period without rate/balance should stay untouched")
	}
	if !approxEqual(shocked.DebtService[1].Interest.Value, 5_000) {
		t.Fatalf("flat interest figure should be untouched, got %v", shocked.DebtService[1].Interest.Value)
	}
}

// TestApplyDebtRateShock_EndToEnd verifies the shocked rate actually
// increases projected interest expense and lowers DSCR through Calculate.
func TestApplyDebtRateShock_EndToEnd(t *testing.T) {
	ds := newDataset().simpleBasePeriod("2025", 1_000_000, 400_000, 300_000, 0).build()
	base := Assumptions{
		Revenue: flatRevenueGrowth(0, 1),
		COGS:    flatGrossMargin(0.6, 1),
		Opex:    flatOpexGrowth(0, 1),
		DebtService: []DebtServiceAssumption{
			{Principal: AvailableValue(10_000), InterestRate: AvailableValue(0.05), BeginningBalance: AvailableValue(200_000)},
		},
	}
	shocked := ApplyDebtRateShock(base, 0.03, 1, 1)

	result := Calculate(Input{
		Dataset: ds, PeriodMeta: singlePeriodMeta("2025", 2025), Horizon: 1,
		Scenarios: []Scenario{
			{Name: "Base", Assumptions: base},
			{Name: "Shocked", Assumptions: shocked},
		},
	})
	if !result.Available {
		t.Fatalf("errors: %+v", result.Errors)
	}
	baseInterest := mustAvailable(t, "base interest", result.ScenarioResults[0].CashFlow[0].DebtService.Interest)
	shockedInterest := mustAvailable(t, "shocked interest", result.ScenarioResults[1].CashFlow[0].DebtService.Interest)
	if shockedInterest <= baseInterest {
		t.Fatalf("expected shocked interest (%v) > base interest (%v)", shockedInterest, baseInterest)
	}
	wantShocked := 200_000.0 * 0.08
	if !approxEqual(shockedInterest, wantShocked) {
		t.Fatalf("shocked interest: want %v, got %v", wantShocked, shockedInterest)
	}
}

// TestTransformHelpers_EmptyWindowIsNoOp verifies every helper treats a
// fromPeriod beyond horizon as a valid no-op, per periodRange's doc
// comment, rather than panicking or erroring.
func TestTransformHelpers_EmptyWindowIsNoOp(t *testing.T) {
	base := Assumptions{
		Revenue:     flatRevenueGrowth(0.10, 2),
		COGS:        flatGrossMargin(0.5, 2),
		Opex:        flatOpexGrowth(0.05, 2),
		DebtService: []DebtServiceAssumption{{InterestRate: AvailableValue(0.05), BeginningBalance: AvailableValue(1000)}, {}},
	}
	_ = ApplyRevenueShock(base, -0.5, 10, 2)
	_ = ApplyMarginShock(base, -0.5, 10, 2)
	_ = ApplyExpenseShock(base, 0.5, nil, 10, 2)
	_ = ApplyDebtRateShock(base, 0.5, 10, 2)
	// No panic is the test; also spot-check one field is genuinely untouched.
	r := ApplyRevenueShock(base, -0.5, 10, 2)
	if !approxEqual(r.Revenue[0].GrowthRate, 0.10) || !approxEqual(r.Revenue[1].GrowthRate, 0.10) {
		t.Fatalf("expected no periods shocked when fromPeriod > horizon, got %+v", r.Revenue)
	}
}
