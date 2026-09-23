package acquisition

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/analytics/debt"
)

const epsilon = 1e-6

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

func mustAvailable(t *testing.T, v Value, name string) float64 {
	t.Helper()
	if !v.Available {
		t.Fatalf("%s: expected Available, got unavailable", name)
	}
	return v.Amount
}

func mustUnavailable(t *testing.T, v Value, name string) {
	t.Helper()
	if v.Available {
		t.Fatalf("%s: expected unavailable, got %v", name, v.Amount)
	}
}

// TestCalculate_AllCash covers an all-cash deal: no debt tranches, so
// annual debt service is zero, DSCR/leverage are unavailable, and
// post-debt cash flow equals the earnings base (less capex/comp if any).
func TestCalculate_AllCash(t *testing.T) {
	in := Input{
		Target: TargetFinancials{
			Revenue:          AvailableValue(2_000_000),
			NormalizedEBITDA: AvailableValue(400_000),
		},
		Consensus:   ConsensusValuation{Value: AvailableValue(1_800_000), Basis: "enterprise_value"},
		AskingPrice: AvailableValue(1_800_000),
		Financing: Financing{
			BuyerCashContribution: AvailableValue(1_800_000),
		},
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	if len(res.DebtSchedules) != 0 {
		t.Fatalf("expected no debt schedules, got %d", len(res.DebtSchedules))
	}

	ads := mustAvailable(t, res.BaseCase.AnnualDebtService, "AnnualDebtService")
	if ads != 0 {
		t.Fatalf("expected zero annual debt service, got %v", ads)
	}
	mustUnavailable(t, res.BaseCase.DSCR, "DSCR")
	leverage := mustAvailable(t, res.BaseCase.Leverage, "Leverage")
	if leverage != 0 {
		t.Fatalf("expected zero leverage for an all-cash deal (zero debt is a known 0x, not unavailable), got %v", leverage)
	}

	pdcf := mustAvailable(t, res.BaseCase.PostDebtCashFlow, "PostDebtCashFlow")
	if !approxEqual(pdcf, 400_000) {
		t.Fatalf("expected post-debt cash flow == EBITDA (400000) for all-cash deal, got %v", pdcf)
	}

	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueAllCashDeal {
			found = true
		}
	}
	if !found {
		t.Fatal("expected IssueAllCashDeal warning for all-cash deal")
	}
}

// TestCalculate_Leveraged covers a leveraged deal financed with a single
// bank term loan, verifying DSCR, leverage, and post-debt cash flow are
// all derived from analytics/debt's amortization math.
func TestCalculate_Leveraged(t *testing.T) {
	loan := debt.LoanTerms{
		Label:              "Acquisition term loan",
		Principal:          1_500_000,
		AnnualInterestRate: 0.08,
		AmortizationYears:  10,
		Frequency:          debt.FrequencyMonthly,
	}
	in := Input{
		Target: TargetFinancials{
			Revenue:          AvailableValue(2_500_000),
			NormalizedEBITDA: AvailableValue(500_000),
		},
		AskingPrice: AvailableValue(2_000_000),
		Financing: Financing{
			DebtTranches:          []debt.LoanTerms{loan},
			BuyerCashContribution: AvailableValue(500_000),
		},
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.DebtSchedules) != 1 {
		t.Fatalf("expected 1 debt schedule, got %d", len(res.DebtSchedules))
	}

	expectedSchedule := debt.Amortize(loan)
	ads := mustAvailable(t, res.BaseCase.AnnualDebtService, "AnnualDebtService")
	if !approxEqual(ads, expectedSchedule.FirstYearAnnualDebtService) {
		t.Fatalf("expected annual debt service %v, got %v", expectedSchedule.FirstYearAnnualDebtService, ads)
	}

	dscr := mustAvailable(t, res.BaseCase.DSCR, "DSCR")
	expectedDSCR := 500_000 / expectedSchedule.FirstYearAnnualDebtService
	if !approxEqual(dscr, expectedDSCR) {
		t.Fatalf("expected DSCR %v, got %v", expectedDSCR, dscr)
	}

	leverage := mustAvailable(t, res.BaseCase.Leverage, "Leverage")
	expectedLeverage := 1_500_000.0 / 500_000.0
	if !approxEqual(leverage, expectedLeverage) {
		t.Fatalf("expected leverage %v, got %v", expectedLeverage, leverage)
	}

	pdcf := mustAvailable(t, res.BaseCase.PostDebtCashFlow, "PostDebtCashFlow")
	expectedPDCF := 500_000 - expectedSchedule.FirstYearAnnualDebtService
	if !approxEqual(pdcf, expectedPDCF) {
		t.Fatalf("expected post-debt cash flow %v, got %v", expectedPDCF, pdcf)
	}

	priceToEBITDA := mustAvailable(t, res.Multiples.PriceToEBITDA, "PriceToEBITDA")
	if !approxEqual(priceToEBITDA, 4.0) {
		t.Fatalf("expected price/EBITDA of 4.0x, got %v", priceToEBITDA)
	}
}

// TestCalculate_SellerNote covers a deal financed with two tranches — a
// bank term loan plus a seller note — verifying debt service sums across
// tranches and required equity accounts for both.
func TestCalculate_SellerNote(t *testing.T) {
	bankLoan := debt.LoanTerms{
		Label:              "Bank term loan",
		Principal:          1_000_000,
		AnnualInterestRate: 0.09,
		AmortizationYears:  10,
		Frequency:          debt.FrequencyMonthly,
	}
	sellerNote := debt.LoanTerms{
		Label:              "Seller note",
		Principal:          300_000,
		AnnualInterestRate: 0.06,
		AmortizationYears:  5,
		Frequency:          debt.FrequencyAnnual,
	}
	in := Input{
		Target: TargetFinancials{
			NormalizedEBITDA: AvailableValue(450_000),
		},
		AskingPrice: AvailableValue(1_800_000),
		Fees: TransactionFees{
			LegalAndAdvisory: AvailableValue(50_000),
		},
		Financing: Financing{
			DebtTranches: []debt.LoanTerms{bankLoan, sellerNote},
		},
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.DebtSchedules) != 2 {
		t.Fatalf("expected 2 debt schedules, got %d", len(res.DebtSchedules))
	}

	bankSched := debt.Amortize(bankLoan)
	noteSched := debt.Amortize(sellerNote)
	expectedADS := bankSched.FirstYearAnnualDebtService + noteSched.FirstYearAnnualDebtService

	ads := mustAvailable(t, res.BaseCase.AnnualDebtService, "AnnualDebtService")
	if !approxEqual(ads, expectedADS) {
		t.Fatalf("expected combined annual debt service %v, got %v", expectedADS, ads)
	}

	totalDebt := mustAvailable(t, res.SourcesAndUses.TotalDebtFinancing, "TotalDebtFinancing")
	if !approxEqual(totalDebt, 1_300_000) {
		t.Fatalf("expected total debt financing 1300000, got %v", totalDebt)
	}

	totalUses := mustAvailable(t, res.SourcesAndUses.TotalUses, "TotalUses")
	if !approxEqual(totalUses, 1_850_000) {
		t.Fatalf("expected total uses 1850000 (price + fees), got %v", totalUses)
	}

	requiredEquity := mustAvailable(t, res.SourcesAndUses.RequiredEquity, "RequiredEquity")
	if !approxEqual(requiredEquity, 550_000) {
		t.Fatalf("expected required equity 550000, got %v", requiredEquity)
	}

	// No BuyerCashContribution supplied, so cash contribution should fall
	// back to RequiredEquity.
	cc := mustAvailable(t, res.Returns.CashContribution, "CashContribution")
	if !approxEqual(cc, requiredEquity) {
		t.Fatalf("expected cash contribution to fall back to required equity %v, got %v", requiredEquity, cc)
	}
}

// TestCalculate_PriceAboveValuation covers an asking price above
// consensus value, producing a positive premium.
func TestCalculate_PriceAboveValuation(t *testing.T) {
	in := Input{
		Target: TargetFinancials{
			NormalizedEBITDA: AvailableValue(500_000),
		},
		Consensus:   ConsensusValuation{Value: AvailableValue(2_000_000)},
		AskingPrice: AvailableValue(2_400_000),
	}

	res := Calculate(in)
	premium := mustAvailable(t, res.Consensus.Premium, "Premium")
	if !approxEqual(premium, 400_000) {
		t.Fatalf("expected premium of 400000, got %v", premium)
	}
	premiumPct := mustAvailable(t, res.Consensus.PremiumPercent, "PremiumPercent")
	if !approxEqual(premiumPct, 0.2) {
		t.Fatalf("expected premium percent of 0.20, got %v", premiumPct)
	}
}

// TestCalculate_PriceBelowValuation covers an asking price below
// consensus value, producing a negative premium (a discount) — and
// verifies the premium red flag never triggers for a discount regardless
// of the threshold.
func TestCalculate_PriceBelowValuation(t *testing.T) {
	in := Input{
		Target: TargetFinancials{
			NormalizedEBITDA: AvailableValue(500_000),
		},
		Consensus:   ConsensusValuation{Value: AvailableValue(2_000_000)},
		AskingPrice: AvailableValue(1_600_000),
		RedFlags: RedFlagThresholds{
			MaximumPremiumToConsensusPercent: 0.05,
		},
	}

	res := Calculate(in)
	premium := mustAvailable(t, res.Consensus.Premium, "Premium")
	if !approxEqual(premium, -400_000) {
		t.Fatalf("expected premium of -400000, got %v", premium)
	}
	premiumPct := mustAvailable(t, res.Consensus.PremiumPercent, "PremiumPercent")
	if !approxEqual(premiumPct, -0.2) {
		t.Fatalf("expected premium percent of -0.20, got %v", premiumPct)
	}

	for _, f := range res.Flags {
		if f.Code == FlagAboveMaximumPremiumToConsensus {
			t.Fatalf("expected no premium red flag for a discount, got %+v", f)
		}
	}
}

// TestCalculate_WeakDSCR covers a deal whose base-case DSCR falls below a
// caller-supplied minimum, verifying FlagBelowMinimumDSCR triggers.
func TestCalculate_WeakDSCR(t *testing.T) {
	loan := debt.LoanTerms{
		Principal:          2_000_000,
		AnnualInterestRate: 0.10,
		AmortizationYears:  7,
		Frequency:          debt.FrequencyMonthly,
	}
	in := Input{
		Target: TargetFinancials{
			NormalizedEBITDA: AvailableValue(300_000),
		},
		AskingPrice: AvailableValue(2_200_000),
		Financing: Financing{
			DebtTranches: []debt.LoanTerms{loan},
		},
		RedFlags: RedFlagThresholds{
			MinimumDSCR: 1.25,
		},
	}

	res := Calculate(in)
	dscr := mustAvailable(t, res.BaseCase.DSCR, "DSCR")
	if dscr >= 1.25 {
		t.Fatalf("expected weak DSCR below 1.25, got %v", dscr)
	}

	found := false
	for _, f := range res.Flags {
		if f.Code == FlagBelowMinimumDSCR {
			found = true
			if f.Severity != FlagSeverityCritical {
				t.Fatalf("expected critical severity, got %v", f.Severity)
			}
		}
	}
	if !found {
		t.Fatal("expected FlagBelowMinimumDSCR to trigger")
	}
}

// TestCalculate_NegativeCashFlow covers a deal whose post-debt cash flow
// is negative, verifying FlagNegativePostDebtCashFlow triggers
// unconditionally (no threshold required).
func TestCalculate_NegativeCashFlow(t *testing.T) {
	loan := debt.LoanTerms{
		Principal:          3_000_000,
		AnnualInterestRate: 0.09,
		AmortizationYears:  5,
		Frequency:          debt.FrequencyMonthly,
	}
	in := Input{
		Target: TargetFinancials{
			NormalizedEBITDA: AvailableValue(300_000),
		},
		AskingPrice: AvailableValue(3_200_000),
		Financing: Financing{
			DebtTranches: []debt.LoanTerms{loan},
		},
	}

	res := Calculate(in)
	pdcf := mustAvailable(t, res.BaseCase.PostDebtCashFlow, "PostDebtCashFlow")
	if pdcf >= 0 {
		t.Fatalf("expected negative post-debt cash flow, got %v", pdcf)
	}

	found := false
	for _, f := range res.Flags {
		if f.Code == FlagNegativePostDebtCashFlow {
			found = true
		}
	}
	if !found {
		t.Fatal("expected FlagNegativePostDebtCashFlow to trigger")
	}
}

// TestCalculate_DownsideShocks covers Input.Scenarios: a downside case
// (EBITDA/revenue haircut) that pushes DSCR below the minimum and
// produces negative cash flow, and an upside case (negative haircut) that
// does not.
func TestCalculate_DownsideShocks(t *testing.T) {
	loan := debt.LoanTerms{
		Principal:          1_200_000,
		AnnualInterestRate: 0.08,
		AmortizationYears:  10,
		Frequency:          debt.FrequencyMonthly,
	}
	in := Input{
		Target: TargetFinancials{
			Revenue:          AvailableValue(3_000_000),
			NormalizedEBITDA: AvailableValue(400_000),
		},
		AskingPrice: AvailableValue(1_600_000),
		Financing: Financing{
			DebtTranches: []debt.LoanTerms{loan},
		},
		RedFlags: RedFlagThresholds{
			MinimumDSCR: 1.3,
		},
		Scenarios: []ScenarioAdjustment{
			{Label: "Severe downside", EBITDAHaircutPercent: 0.65, RevenueHaircutPercent: 0.3},
			{Label: "Upside", EBITDAHaircutPercent: -0.1, RevenueHaircutPercent: -0.05},
		},
	}

	res := Calculate(in)
	if len(res.Scenarios) != 2 {
		t.Fatalf("expected 2 scenario results, got %d", len(res.Scenarios))
	}

	downside := res.Scenarios[0]
	if downside.Scenario.Label != "Severe downside" {
		t.Fatalf("expected first scenario to be Severe downside, got %q", downside.Scenario.Label)
	}
	downEBITDA := mustAvailable(t, downside.Multiples.PriceToEBITDA, "downside PriceToEBITDA")
	baseEBITDAMultiple := mustAvailable(t, res.Multiples.PriceToEBITDA, "base PriceToEBITDA")
	if downEBITDA <= baseEBITDAMultiple {
		t.Fatalf("expected downside price/EBITDA multiple (%v) to be worse (higher) than base case (%v)", downEBITDA, baseEBITDAMultiple)
	}
	if !downside.HasNegativeCashFlow {
		t.Fatal("expected severe downside scenario to have negative cash flow")
	}
	if !downside.BreachesMinimumDSCR {
		t.Fatal("expected severe downside scenario to breach minimum DSCR")
	}

	upside := res.Scenarios[1]
	if upside.HasNegativeCashFlow {
		t.Fatal("expected upside scenario to have positive cash flow")
	}

	foundScenarioDSCRFlag := false
	foundScenarioCashFlowFlag := false
	for _, f := range res.Flags {
		if f.Code == FlagScenarioBreachesDSCR && f.Scenario == "Severe downside" {
			foundScenarioDSCRFlag = true
		}
		if f.Code == FlagScenarioNegativeCashFlow && f.Scenario == "Severe downside" {
			foundScenarioCashFlowFlag = true
		}
	}
	if !foundScenarioDSCRFlag {
		t.Fatal("expected FlagScenarioBreachesDSCR for the severe downside scenario")
	}
	if !foundScenarioCashFlowFlag {
		t.Fatal("expected FlagScenarioNegativeCashFlow for the severe downside scenario")
	}
}

// TestCalculate_SDEFallback covers a sole-proprietor-style target with
// only NormalizedSDE (no NormalizedEBITDA), verifying the earnings base
// falls back to SDE for multiples/coverage/leverage.
func TestCalculate_SDEFallback(t *testing.T) {
	loan := debt.LoanTerms{
		Principal:          400_000,
		AnnualInterestRate: 0.07,
		AmortizationYears:  7,
		Frequency:          debt.FrequencyMonthly,
	}
	in := Input{
		Target: TargetFinancials{
			NormalizedSDE: AvailableValue(180_000),
		},
		AskingPrice: AvailableValue(450_000),
		Financing: Financing{
			DebtTranches: []debt.LoanTerms{loan},
		},
	}

	res := Calculate(in)
	mustUnavailable(t, res.Multiples.PriceToEBITDA, "PriceToEBITDA")
	priceToSDE := mustAvailable(t, res.Multiples.PriceToSDE, "PriceToSDE")
	if !approxEqual(priceToSDE, 2.5) {
		t.Fatalf("expected price/SDE of 2.5x, got %v", priceToSDE)
	}

	if res.BaseCase.EarningsBaseSource != EarningsSourceSDE {
		t.Fatalf("expected earnings base source SDE, got %v", res.BaseCase.EarningsBaseSource)
	}
	mustAvailable(t, res.BaseCase.DSCR, "DSCR")
	mustAvailable(t, res.BaseCase.Leverage, "Leverage")
}

// TestCalculate_CapexAndBuyerCompensation covers PostDebtCashFlow's
// deduction of both an ongoing capex assumption and a buyer compensation
// assumption.
func TestCalculate_CapexAndBuyerCompensation(t *testing.T) {
	in := Input{
		Target: TargetFinancials{
			NormalizedSDE: AvailableValue(300_000),
		},
		AskingPrice: AvailableValue(900_000),
		Capex:       CapexAssumption{AnnualAmount: AvailableValue(20_000)},
		BuyerCompensation: BuyerCompensationAssumption{
			AnnualAmount: AvailableValue(80_000),
		},
	}

	res := Calculate(in)
	pdcf := mustAvailable(t, res.BaseCase.PostDebtCashFlow, "PostDebtCashFlow")
	// All-cash: annual debt service is 0, so PostDebtCashFlow = 300000 -
	// 0 - 20000 (capex) - 80000 (comp) = 200000.
	if !approxEqual(pdcf, 200_000) {
		t.Fatalf("expected post-debt cash flow of 200000, got %v", pdcf)
	}
}

// TestCalculate_InvalidLoanTerms covers an invalid debt tranche being
// excluded from the schedules and reported as an error Issue, while the
// rest of the analysis still proceeds using the remaining valid tranche.
func TestCalculate_InvalidLoanTerms(t *testing.T) {
	validLoan := debt.LoanTerms{
		Principal:          500_000,
		AnnualInterestRate: 0.07,
		AmortizationYears:  10,
		Frequency:          debt.FrequencyMonthly,
	}
	invalidLoan := debt.LoanTerms{
		Principal:          -100_000,
		AnnualInterestRate: 0.07,
		AmortizationYears:  10,
		Frequency:          debt.FrequencyMonthly,
	}
	in := Input{
		Target: TargetFinancials{
			NormalizedEBITDA: AvailableValue(200_000),
		},
		AskingPrice: AvailableValue(900_000),
		Financing: Financing{
			DebtTranches: []debt.LoanTerms{validLoan, invalidLoan},
		},
	}

	res := Calculate(in)
	if len(res.DebtSchedules) != 1 {
		t.Fatalf("expected 1 valid debt schedule, got %d", len(res.DebtSchedules))
	}

	found := false
	for _, e := range res.Errors {
		if e.Code == IssueInvalidLoanTerms && e.Tranche == "debt_tranches[1]" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected IssueInvalidLoanTerms error for debt_tranches[1]")
	}
}

// TestCalculate_Empty covers the degenerate zero-Input case: no
// Available result, a single error Issue, and every other field
// zero-value.
func TestCalculate_Empty(t *testing.T) {
	res := Calculate(Input{})
	if res.Available {
		t.Fatal("expected Available == false for empty input")
	}
	if len(res.Errors) != 1 || res.Errors[0].Code != IssueNoAskingPrice {
		t.Fatalf("expected a single IssueNoAskingPrice error, got %+v", res.Errors)
	}
}

// TestCalculate_NoRedFlagThresholds verifies that with a zero-value
// RedFlagThresholds, no red-flag checks fire even for numbers that would
// otherwise trigger them, and IssueNoRedFlagThresholds is recorded.
func TestCalculate_NoRedFlagThresholds(t *testing.T) {
	loan := debt.LoanTerms{
		Principal:          3_000_000,
		AnnualInterestRate: 0.09,
		AmortizationYears:  5,
		Frequency:          debt.FrequencyMonthly,
	}
	in := Input{
		Target: TargetFinancials{
			NormalizedEBITDA: AvailableValue(300_000),
		},
		AskingPrice: AvailableValue(3_200_000),
		Financing: Financing{
			DebtTranches: []debt.LoanTerms{loan},
		},
	}

	res := Calculate(in)
	for _, f := range res.Flags {
		switch f.Code {
		case FlagBelowMinimumDSCR, FlagAboveMaximumPriceToEBITDA, FlagAboveMaximumPriceToSDE,
			FlagAboveMaximumPremiumToConsensus, FlagBelowMinimumCashOnCashReturn,
			FlagAboveMaximumPayback, FlagAboveMaximumLeverage:
			t.Fatalf("expected no threshold-driven flags without RedFlags set, got %v", f.Code)
		}
	}

	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoRedFlagThresholds {
			found = true
		}
	}
	if !found {
		t.Fatal("expected IssueNoRedFlagThresholds warning")
	}
}
