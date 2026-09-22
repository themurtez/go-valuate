package cashflow

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func TestCalculate_NoPeriods_Unavailable(t *testing.T) {
	res := Calculate(Input{Dataset: financial.FinancialDataset{}}, Options{})
	if res.Available {
		t.Fatal("expected Available == false for empty dataset")
	}
	if !HasErrors(res.Errors) {
		t.Fatal("expected an error for empty dataset")
	}
}

// TestCalculate_StrongConversion exercises a business whose operating cash
// flow tracks EBITDA closely (minimal working-capital drag), producing a
// conversion ratio near 1.0 and no weak-conversion flag.
func TestCalculate_StrongConversion(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2023", 1_000_000, 400_000, 200_000, 20_000).
		incomeStatementPeriod("2024", 1_100_000, 440_000, 220_000, 20_000).
		balanceSheetPeriod("2023", 100_000, 50_000, 10_000, 80_000, 20_000).
		balanceSheetPeriod("2024", 102_000, 51_000, 10_500, 82_000, 20_500).
		build()

	// EBITDA 2024 = (1,100,000 - 440,000) - 220,000 + 20,000 (D&A add-back) = 460,000
	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2023": Reported(340_000),
			"2024": Reported(400_000), // 400,000 / 460,000 ~ 0.870
		},
		Capex: map[financial.Period]CashFlowValue{
			"2024": Reported(30_000),
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}

	last := res.Conversion[len(res.Conversion)-1]
	if !last.EBITDAToOperatingCashFlow.Available {
		t.Fatal("expected EBITDAToOperatingCashFlow to be available")
	}
	if got := last.EBITDAToOperatingCashFlow.Value; got < 0.80 || got > 0.90 {
		t.Fatalf("expected conversion ~0.870, got %v", got)
	}

	for _, f := range res.Flags {
		if f.Code == FlagWeakCashConversion {
			t.Fatalf("did not expect FlagWeakCashConversion for strong conversion, got flag: %+v", f)
		}
	}
}

// TestCalculate_WeakConversion exercises a business whose operating cash
// flow is far below EBITDA, triggering FlagWeakCashConversion.
func TestCalculate_WeakConversion(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2023", 1_000_000, 400_000, 200_000, 20_000).
		incomeStatementPeriod("2024", 1_100_000, 440_000, 220_000, 20_000).
		build()

	// EBITDA 2024 = 440,000; OCF reported far below it.
	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2023": Reported(200_000),
			"2024": Reported(150_000), // 150,000 / 440,000 ~ 0.34
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}

	var found bool
	for _, f := range res.Flags {
		if f.Code == FlagWeakCashConversion {
			found = true
			if f.Severity != FlagSeverityWarning {
				t.Fatalf("expected FlagSeverityWarning, got %v", f.Severity)
			}
		}
	}
	if !found {
		t.Fatal("expected FlagWeakCashConversion to trigger")
	}
}

// TestCalculate_WorkingCapitalBuild exercises a business with a large
// increase in net working capital (AR/inventory build), which both
// depresses estimated operating cash flow and triggers
// FlagHighWorkingCapitalBurden.
func TestCalculate_WorkingCapitalBuild(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2023", 1_000_000, 400_000, 200_000, 20_000).
		incomeStatementPeriod("2024", 1_200_000, 480_000, 240_000, 20_000).
		balanceSheetPeriod("2023", 100_000, 50_000, 10_000, 80_000, 20_000).
		balanceSheetPeriod("2024", 220_000, 130_000, 10_000, 85_000, 20_000). // large AR/inventory build
		build()

	// NWC 2023 = (100k+50k+10k) - (80k+20k) = 60,000
	// NWC 2024 = (220k+130k+10k) - (85k+20k) = 255,000
	// ChangeInNWC = 195,000 (large cash use)
	// EBITDA 2024 = 1,200,000 - 480,000 - 240,000 = 480,000
	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
	}, Options{AllowEBITDAEstimate: true})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}

	last := res.History[len(res.History)-1]
	if !last.ChangeInNWC.Available {
		t.Fatal("expected ChangeInNWC to be available")
	}
	if got := last.ChangeInNWC.Value; got < 190_000 || got > 200_000 {
		t.Fatalf("expected ChangeInNWC ~195,000, got %v", got)
	}
	if !last.OperatingCashFlow.Available || !last.OperatingCashFlow.IsEstimate {
		t.Fatal("expected an EBITDA-based OperatingCashFlow estimate")
	}

	var found bool
	for _, f := range res.Flags {
		if f.Code == FlagHighWorkingCapitalBurden {
			found = true
		}
	}
	if !found {
		t.Fatal("expected FlagHighWorkingCapitalBurden to trigger")
	}
}

// TestCalculate_CapexHeavy exercises a business with capex consuming a
// large share of EBITDA, triggering FlagHighCapexBurden and pulling
// FreeCashFlow well below OperatingCashFlow.
func TestCalculate_CapexHeavy(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2023", 1_000_000, 400_000, 200_000, 20_000).
		incomeStatementPeriod("2024", 1_050_000, 420_000, 210_000, 20_000).
		build()

	// EBITDA 2024 = 1,050,000 - 420,000 - 210,000 = 420,000
	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2023": Reported(220_000),
			"2024": Reported(380_000),
		},
		Capex: map[financial.Period]CashFlowValue{
			"2024": Reported(150_000), // 150,000 / 420,000 ~ 0.357, above default 0.25 threshold
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}

	last := res.History[len(res.History)-1]
	if !last.FreeCashFlow.Available {
		t.Fatal("expected FreeCashFlow to be available")
	}
	if got := last.FreeCashFlow.Value; got != 230_000 {
		t.Fatalf("expected FreeCashFlow 230,000 (380,000 - 150,000), got %v", got)
	}

	var found bool
	for _, f := range res.Flags {
		if f.Code == FlagHighCapexBurden {
			found = true
		}
	}
	if !found {
		t.Fatal("expected FlagHighCapexBurden to trigger")
	}
}

// TestCalculate_NegativeCashFlow exercises a loss-making business burning
// cash, verifying CashRunway is computed and FlagLowCashRunway triggers
// when the runway is short.
func TestCalculate_NegativeCashFlow(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2023", 500_000, 300_000, 350_000, 10_000).
		incomeStatementPeriod("2024", 520_000, 310_000, 360_000, 10_000).
		build()

	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2023": Reported(-140_000),
			"2024": Reported(-150_000),
		},
		CashBalance: map[financial.Period]CashFlowValue{
			"2024": Reported(300_000),
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}

	if !res.CashRunway.Available {
		t.Fatal("expected CashRunway to be available for a cash-burning business")
	}
	if !res.CashRunway.MonthlyBurnRate.Available || res.CashRunway.MonthlyBurnRate.Value >= 0 {
		t.Fatalf("expected a negative MonthlyBurnRate, got %+v", res.CashRunway.MonthlyBurnRate)
	}
	// average burn = (-140,000 + -150,000) / 2 = -145,000; runway = 300,000/145,000 ~ 2.07 months
	if !res.CashRunway.MonthsOfRunway.Available {
		t.Fatal("expected MonthsOfRunway to be available")
	}
	if got := res.CashRunway.MonthsOfRunway.Value; got < 1.5 || got > 2.5 {
		t.Fatalf("expected ~2.07 months of runway, got %v", got)
	}

	var found bool
	for _, f := range res.Flags {
		if f.Code == FlagLowCashRunway {
			found = true
		}
	}
	if !found {
		t.Fatal("expected FlagLowCashRunway to trigger")
	}
}

// TestCalculate_ProfitableBusiness_NoRunway verifies CashRunway is left
// unavailable (not a wildly wrong "infinite runway") for a profitable,
// cash-generating business.
func TestCalculate_ProfitableBusiness_NoRunway(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2024", 1_000_000, 400_000, 200_000, 20_000).
		build()

	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2024": Reported(350_000),
		},
		CashBalance: map[financial.Period]CashFlowValue{
			"2024": Reported(500_000),
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}
	if res.CashRunway.Available {
		t.Fatalf("expected CashRunway.Available == false for a profitable business, got %+v", res.CashRunway)
	}
}

// TestCalculate_MissingCashFlowStatement verifies that without
// Input.OperatingCashFlow and without Options.AllowEBITDAEstimate, every
// cash-flow figure is left unavailable and IssueNoCashFlowStatement is
// warned, rather than silently defaulting to EBITDA.
func TestCalculate_MissingCashFlowStatement(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2023", 1_000_000, 400_000, 200_000, 20_000).
		incomeStatementPeriod("2024", 1_100_000, 440_000, 220_000, 20_000).
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: threeYearMeta()}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}

	var sawIssue bool
	for _, w := range res.Warnings {
		if w.Code == IssueNoCashFlowStatement {
			sawIssue = true
		}
	}
	if !sawIssue {
		t.Fatal("expected IssueNoCashFlowStatement warning")
	}

	for _, b := range res.History {
		if b.OperatingCashFlow.Available {
			t.Fatalf("expected OperatingCashFlow unavailable without AllowEBITDAEstimate, got %+v for period %s", b.OperatingCashFlow, b.Period)
		}
		if b.EBITDA.Available == false {
			t.Fatalf("expected EBITDA itself to remain available for period %s (only cash figures should be unavailable)", b.Period)
		}
	}
}

// TestCalculate_EstimateVsReported verifies the IsEstimate/EstimateBasis
// distinction: a period with a reported OCF keeps it verbatim
// (IsEstimate == false); a period without one, under
// Options.AllowEBITDAEstimate, gets an EBITDA-based estimate clearly
// labeled as such.
func TestCalculate_EstimateVsReported(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2023", 1_000_000, 400_000, 200_000, 20_000).
		incomeStatementPeriod("2024", 1_100_000, 440_000, 220_000, 20_000).
		balanceSheetPeriod("2023", 100_000, 50_000, 10_000, 80_000, 20_000).
		balanceSheetPeriod("2024", 105_000, 52_000, 10_000, 81_000, 20_000).
		build()

	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2024": Reported(300_000), // 2023 left unreported
		},
	}, Options{AllowEBITDAEstimate: true})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}

	var y2023, y2024 Bridge
	for _, b := range res.History {
		switch b.Period {
		case "2023":
			y2023 = b
		case "2024":
			y2024 = b
		}
	}

	if !y2024.OperatingCashFlow.Available || y2024.OperatingCashFlow.IsEstimate {
		t.Fatalf("expected 2024 OperatingCashFlow to be reported (not estimated), got %+v", y2024.OperatingCashFlow)
	}
	if y2024.OperatingCashFlow.Value != 300_000 {
		t.Fatalf("expected 2024 OperatingCashFlow == 300,000 verbatim, got %v", y2024.OperatingCashFlow.Value)
	}

	// 2023 is the earliest period, so ChangeInNWC has nothing to compare
	// against and is unavailable — meaning the EBITDA-based estimate
	// (which requires ChangeInNWC) cannot be computed for 2023 either.
	if y2023.ChangeInNWC.Available {
		t.Fatalf("expected 2023 ChangeInNWC unavailable (earliest period), got %+v", y2023.ChangeInNWC)
	}
	if y2023.OperatingCashFlow.Available {
		t.Fatalf("expected 2023 OperatingCashFlow unavailable (no reported figure and no ChangeInNWC to estimate from), got %+v", y2023.OperatingCashFlow)
	}

	var sawEstimateIssue bool
	for _, w := range res.Warnings {
		if w.Code == IssueEstimatedFromEBITDA {
			sawEstimateIssue = true
		}
	}
	// No estimate actually succeeded in this fixture (2023 lacks
	// ChangeInNWC), so the estimate issue should not appear.
	if sawEstimateIssue {
		t.Fatal("did not expect IssueEstimatedFromEBITDA since no period had both EBITDA and ChangeInNWC available without a reported figure")
	}
}

// TestCalculate_EstimateSucceeds verifies the estimate issue and IsEstimate
// flag DO appear when a period has both EBITDA and ChangeInNWC available
// but no reported OperatingCashFlow.
func TestCalculate_EstimateSucceeds(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2023", 1_000_000, 400_000, 200_000, 20_000).
		incomeStatementPeriod("2024", 1_100_000, 440_000, 220_000, 20_000).
		balanceSheetPeriod("2023", 100_000, 50_000, 10_000, 80_000, 20_000).
		balanceSheetPeriod("2024", 105_000, 52_000, 10_000, 81_000, 20_000).
		build()

	// No OperatingCashFlow supplied for either period.
	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
	}, Options{AllowEBITDAEstimate: true})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}

	y2024 := res.History[len(res.History)-1]
	if !y2024.OperatingCashFlow.Available || !y2024.OperatingCashFlow.IsEstimate {
		t.Fatalf("expected 2024 OperatingCashFlow to be an estimate, got %+v", y2024.OperatingCashFlow)
	}
	if y2024.OperatingCashFlow.EstimateBasis == "" {
		t.Fatal("expected EstimateBasis to be populated")
	}

	// EBITDA 2024 = (1,100,000-440,000)-220,000+20,000 (D&A add-back) = 460,000
	// NWC2023=60,000 NWC2024=66,000; change=6,000
	// estimate = 460,000 - 6,000 = 454,000
	if got := y2024.OperatingCashFlow.Value; got != 454_000 {
		t.Fatalf("expected estimated OperatingCashFlow 454,000, got %v", got)
	}

	var sawEstimateIssue bool
	for _, w := range res.Warnings {
		if w.Code == IssueEstimatedFromEBITDA {
			sawEstimateIssue = true
		}
	}
	if !sawEstimateIssue {
		t.Fatal("expected IssueEstimatedFromEBITDA warning")
	}
}

// TestCalculate_DebtServiceCoverage verifies DebtServiceCoverage-related
// bridge fields (FreeCashFlowToOwner, FreeCashFlowToFirm) and
// FlagLowDebtServiceCoverage.
func TestCalculate_DebtServiceCoverage(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2024", 1_000_000, 400_000, 200_000, 20_000).
		build()

	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2024": Reported(300_000),
		},
		DebtService: map[financial.Period]DebtServiceFigure{
			"2024": {Principal: Reported(200_000), Interest: Reported(50_000)},
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}

	last := res.History[len(res.History)-1]
	if !last.FreeCashFlowToOwner.Available {
		t.Fatal("expected FreeCashFlowToOwner to be available")
	}
	// FCF = 300,000 (no capex supplied -> estimate capex=0); FCFtoOwner = 300,000 - (200,000+50,000) = 50,000
	if got := last.FreeCashFlowToOwner.Value; got != 50_000 {
		t.Fatalf("expected FreeCashFlowToOwner 50,000, got %v", got)
	}
	if !last.FreeCashFlowToFirm.Available {
		t.Fatal("expected FreeCashFlowToFirm to be available")
	}
	// FCFtoFirm = 300,000 + 50,000 (interest add-back) = 350,000
	if got := last.FreeCashFlowToFirm.Value; got != 350_000 {
		t.Fatalf("expected FreeCashFlowToFirm 350,000, got %v", got)
	}

	// DSCR = 300,000 / 250,000 = 1.2, below default 1.25 threshold
	var found bool
	for _, f := range res.Flags {
		if f.Code == FlagLowDebtServiceCoverage {
			found = true
		}
	}
	if !found {
		t.Fatal("expected FlagLowDebtServiceCoverage to trigger")
	}
}

// TestCalculate_OwnerDistributionsCoverage verifies
// FlagDistributionsExceedFreeCashFlow.
func TestCalculate_OwnerDistributionsCoverage(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2024", 1_000_000, 400_000, 200_000, 20_000).
		build()

	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2024": Reported(300_000),
		},
		OwnerDistributions: map[financial.Period]CashFlowValue{
			"2024": Reported(280_000), // 280,000/300,000 ~ 0.933, above 0.75 threshold
		},
	}, Options{})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}

	var found bool
	for _, f := range res.Flags {
		if f.Code == FlagDistributionsExceedFreeCashFlow {
			found = true
		}
	}
	if !found {
		t.Fatal("expected FlagDistributionsExceedFreeCashFlow to trigger")
	}
}

// TestCalculate_RealisticFixture exercises the repository's realistic
// multi-year HVAC fixture end to end, verifying Calculate does not error
// and produces a sensible EBITDA-based bridge even with no cash-flow
// statement supplied (estimate mode on).
func TestCalculate_RealisticFixture(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_hvac_multi_year.json")

	res := Calculate(Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
	}, Options{AllowEBITDAEstimate: true})

	if !res.Available {
		t.Fatalf("expected Available == true, errors: %v", res.Errors)
	}
	if len(res.History) != 3 {
		t.Fatalf("expected 3 periods of history, got %d", len(res.History))
	}
	for _, b := range res.History {
		if !b.EBITDA.Available {
			t.Fatalf("expected EBITDA available for period %s", b.Period)
		}
	}
	// First period has no preceding period to diff against, so its
	// estimate is unavailable; later periods should have estimates.
	last := res.History[len(res.History)-1]
	if !last.OperatingCashFlow.Available {
		t.Fatal("expected the last period's OperatingCashFlow to be estimated")
	}
}
