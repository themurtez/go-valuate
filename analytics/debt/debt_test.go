package debt

import "testing"

// standardLoan returns a $500,000 term loan at 7% over 10 years, monthly
// payments — a representative proposed acquisition term loan used across
// several tests below.
func standardLoan() LoanTerms {
	return LoanTerms{
		Label:              "Proposed term loan",
		Principal:          500_000,
		AnnualInterestRate: 0.07,
		AmortizationYears:  10,
		Frequency:          FrequencyMonthly,
	}
}

func TestCalculate_NoInputAtAll(t *testing.T) {
	res := Calculate(Input{})
	if res.Available {
		t.Fatal("expected Available == false with no input at all")
	}
	if !HasErrors(res.Errors) {
		t.Fatal("expected an error when no EBITDA, cash flow, or debt is supplied")
	}
}

// TestCalculate_AmortizingDebt covers the core path: EBITDA + one
// amortizing proposed loan produces DSCR, leverage, and interest coverage.
func TestCalculate_AmortizingDebt(t *testing.T) {
	loan := standardLoan()
	res := Calculate(Input{
		EBITDA:        AvailableValue(200_000),
		ProposedLoans: []LoanTerms{loan},
	})

	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.ProposedSchedules) != 1 {
		t.Fatalf("expected 1 proposed schedule, got %d", len(res.ProposedSchedules))
	}
	if !res.BaseCase.DSCR.Available {
		t.Fatal("expected DSCR to be available")
	}
	wantDSCR := 200_000 / res.BaseCase.AnnualDebtService.Amount
	approxEqual(t, res.BaseCase.DSCR.Amount, wantDSCR, 1e-9, "DSCR")

	if !res.BaseCase.DebtToEBITDA.Available {
		t.Fatal("expected DebtToEBITDA to be available")
	}
	approxEqual(t, res.BaseCase.DebtToEBITDA.Amount, 500_000.0/200_000.0, 1e-9, "DebtToEBITDA")

	if res.BaseCase.NetDebtToEBITDA.Available {
		t.Error("expected NetDebtToEBITDA unavailable without CashAndEquivalents")
	}
	if !res.BaseCase.InterestCoverage.Available {
		t.Fatal("expected InterestCoverage available (derived from schedule)")
	}
}

// TestCalculate_ZeroDebt covers the "no debt at all" path: DSCR is
// unavailable (dividing by zero debt service is not meaningful) but
// leverage ratios read as zero, and IssueNoDebt/FlagNoDebtService are
// both reported.
func TestCalculate_ZeroDebt(t *testing.T) {
	res := Calculate(Input{
		EBITDA: AvailableValue(300_000),
	})

	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if res.BaseCase.DSCR.Available {
		t.Error("expected DSCR unavailable with zero debt service")
	}
	if !res.BaseCase.AnnualDebtService.Available || res.BaseCase.AnnualDebtService.Amount != 0 {
		t.Errorf("expected AnnualDebtService to be available and zero, got %+v", res.BaseCase.AnnualDebtService)
	}

	foundIssue := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoDebt {
			foundIssue = true
		}
	}
	if !foundIssue {
		t.Error("expected IssueNoDebt warning")
	}

	foundFlag := false
	for _, f := range res.Flags {
		if f.Code == FlagNoDebtService {
			foundFlag = true
		}
	}
	if !foundFlag {
		t.Error("expected FlagNoDebtService flag")
	}
}

// TestCalculate_NegativeEBITDA covers a negative-EBITDA business: coverage
// ratios dividing by EBITDA read the negative figure through (DSCR
// reflects a genuinely negative-earnings business rather than being
// silently hidden), leverage ratios are left unavailable per
// DebtToEBITDA's doc comment (dividing by a non-positive EBITDA is
// blocked), and IssueNegativeEBITDA is recorded.
func TestCalculate_NegativeEBITDA(t *testing.T) {
	loan := standardLoan()
	res := Calculate(Input{
		EBITDA:        AvailableValue(-50_000),
		ProposedLoans: []LoanTerms{loan},
	})

	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if res.BaseCase.DebtToEBITDA.Available {
		t.Error("expected DebtToEBITDA unavailable with negative EBITDA")
	}
	if !res.BaseCase.DSCR.Available {
		t.Fatal("expected DSCR available (numerator can be negative)")
	}
	if res.BaseCase.DSCR.Amount >= 0 {
		t.Errorf("expected negative DSCR, got %v", res.BaseCase.DSCR.Amount)
	}

	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNegativeEBITDA {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueNegativeEBITDA warning")
	}
}

// TestCalculate_InterestOnlyProposedLoan verifies an interest-only period
// on the proposed loan reduces first-year debt service (and therefore
// raises first-year DSCR) relative to a fully amortizing loan of the same
// size.
func TestCalculate_InterestOnlyProposedLoan(t *testing.T) {
	amortizing := standardLoan()
	interestOnly := standardLoan()
	interestOnly.InterestOnlyYears = 2

	ebitda := AvailableValue(80_000)

	resAmortizing := Calculate(Input{EBITDA: ebitda, ProposedLoans: []LoanTerms{amortizing}})
	resInterestOnly := Calculate(Input{EBITDA: ebitda, ProposedLoans: []LoanTerms{interestOnly}})

	if !resAmortizing.BaseCase.DSCR.Available || !resInterestOnly.BaseCase.DSCR.Available {
		t.Fatal("expected both DSCRs available")
	}
	if resInterestOnly.BaseCase.DSCR.Amount <= resAmortizing.BaseCase.DSCR.Amount {
		t.Errorf("expected interest-only DSCR (%v) > amortizing DSCR (%v)", resInterestOnly.BaseCase.DSCR.Amount, resAmortizing.BaseCase.DSCR.Amount)
	}
}

// TestCalculate_MaxDebtUnderDSCR verifies the maximum-debt-under-DSCR
// solve: the resulting principal, when amortized under the same pricing
// terms, produces annual debt service that implies (approximately) the
// requested minimum DSCR.
func TestCalculate_MaxDebtUnderDSCR(t *testing.T) {
	loan := standardLoan()
	res := Calculate(Input{
		EBITDA:        AvailableValue(300_000),
		ProposedLoans: []LoanTerms{loan},
		Policy:        LenderPolicy{MinimumDSCR: 1.25},
	})

	if !res.Capacity.MaxDebtUnderDSCR.Available {
		t.Fatalf("expected MaxDebtUnderDSCR available, capacity=%+v", res.Capacity)
	}

	scaled := loan
	scaled.Principal = res.Capacity.MaxDebtUnderDSCR.Amount
	sched := Amortize(scaled)
	impliedDSCR := 300_000 / sched.FirstYearAnnualDebtService
	approxEqual(t, impliedDSCR, 1.25, 1e-6, "implied DSCR at solved max debt")
}

// TestCalculate_MaxDebtUnderLeverageCap verifies the leverage-cap solve is
// a direct EBITDA x cap multiplication, and that supplying both a gross
// and net cap picks the more restrictive one.
func TestCalculate_MaxDebtUnderLeverageCap(t *testing.T) {
	res := Calculate(Input{
		EBITDA: AvailableValue(400_000),
		Policy: LenderPolicy{MaximumDebtToEBITDA: 3.0},
	})
	if !res.Capacity.MaxDebtUnderLeverage.Available {
		t.Fatal("expected MaxDebtUnderLeverage available")
	}
	approxEqual(t, res.Capacity.MaxDebtUnderLeverage.Amount, 1_200_000, 1e-6, "max debt under gross leverage cap")

	// Net cap: cash offsets the cap, so the gross-equivalent max debt is
	// higher than the raw multiple would imply.
	resNet := Calculate(Input{
		EBITDA:             AvailableValue(400_000),
		CashAndEquivalents: AvailableValue(100_000),
		Policy:             LenderPolicy{MaximumNetDebtToEBITDA: 2.5},
	})
	if !resNet.Capacity.MaxDebtUnderLeverage.Available {
		t.Fatal("expected MaxDebtUnderLeverage available for net cap")
	}
	approxEqual(t, resNet.Capacity.MaxDebtUnderLeverage.Amount, 2.5*400_000+100_000, 1e-6, "max debt under net leverage cap")

	// Both caps supplied: the smaller gross-equivalent figure wins.
	resBoth := Calculate(Input{
		EBITDA:             AvailableValue(400_000),
		CashAndEquivalents: AvailableValue(100_000),
		Policy:             LenderPolicy{MaximumDebtToEBITDA: 3.0, MaximumNetDebtToEBITDA: 2.5},
	})
	approxEqual(t, resBoth.Capacity.MaxDebtUnderLeverage.Amount, 1_100_000, 1e-6, "combined leverage cap picks the more restrictive")
}

// TestCalculate_CombinedCapacityPicksLimitingConstraint verifies
// CombinedMaximumDebt picks the smaller of the DSCR-implied and
// leverage-implied maximums, and reports which one it was.
func TestCalculate_CombinedCapacityPicksLimitingConstraint(t *testing.T) {
	loan := standardLoan()

	// A generous leverage cap but a tight DSCR requirement: DSCR should
	// limit.
	res := Calculate(Input{
		EBITDA:        AvailableValue(150_000),
		ProposedLoans: []LoanTerms{loan},
		Policy:        LenderPolicy{MinimumDSCR: 2.0, MaximumDebtToEBITDA: 10.0},
	})
	if res.Capacity.LimitingConstraint != LimitDSCR {
		t.Errorf("expected LimitDSCR, got %s (capacity=%+v)", res.Capacity.LimitingConstraint, res.Capacity)
	}
	if !res.Capacity.CombinedMaximumDebt.Available {
		t.Fatal("expected CombinedMaximumDebt available")
	}
	approxEqual(t, res.Capacity.CombinedMaximumDebt.Amount, res.Capacity.MaxDebtUnderDSCR.Amount, 1e-6, "combined = DSCR limit")

	// A tight leverage cap but a lenient DSCR requirement: leverage should
	// limit.
	res2 := Calculate(Input{
		EBITDA:        AvailableValue(150_000),
		ProposedLoans: []LoanTerms{loan},
		Policy:        LenderPolicy{MinimumDSCR: 1.01, MaximumDebtToEBITDA: 1.0},
	})
	if res2.Capacity.LimitingConstraint != LimitLeverage {
		t.Errorf("expected LimitLeverage, got %s (capacity=%+v)", res2.Capacity.LimitingConstraint, res2.Capacity)
	}

	// Headroom should be CombinedMaximumDebt - current total debt.
	wantHeadroom := res.Capacity.CombinedMaximumDebt.Amount - res.BaseCase.TotalDebtBalance.Amount
	if !res.Capacity.Headroom.Available {
		t.Fatal("expected Headroom available")
	}
	approxEqual(t, res.Capacity.Headroom.Amount, wantHeadroom, 1e-6, "headroom")
}

// TestCalculate_NoLenderPolicySkipsCapacity verifies that with no policy
// thresholds set, every Capacity field is left unavailable and
// IssueNoLenderPolicy is recorded, rather than this package inventing a
// default cap.
func TestCalculate_NoLenderPolicySkipsCapacity(t *testing.T) {
	res := Calculate(Input{
		EBITDA:        AvailableValue(300_000),
		ProposedLoans: []LoanTerms{standardLoan()},
	})
	if res.Capacity.MaxDebtUnderDSCR.Available || res.Capacity.MaxDebtUnderLeverage.Available || res.Capacity.CombinedMaximumDebt.Available {
		t.Errorf("expected no capacity figures without a policy, got %+v", res.Capacity)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoLenderPolicy {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueNoLenderPolicy warning")
	}
}

// TestCalculate_DownsideScenarios verifies a downside scenario stresses
// EBITDA and recomputes DSCR with debt service held fixed, and that
// BreachesMinimumDSCR triggers correctly against Policy.MinimumDSCR.
func TestCalculate_DownsideScenarios(t *testing.T) {
	loan := standardLoan()
	res := Calculate(Input{
		EBITDA:        AvailableValue(120_000),
		ProposedLoans: []LoanTerms{loan},
		Policy:        LenderPolicy{MinimumDSCR: 1.25},
		DownsideScenarios: []DownsideScenario{
			{Label: "Mild downturn", EBITDAHaircutPercent: 0.10},
			{Label: "Severe downturn", EBITDAHaircutPercent: 0.40},
		},
	})

	if len(res.Scenarios) != 2 {
		t.Fatalf("expected 2 scenario results, got %d", len(res.Scenarios))
	}

	mild := res.Scenarios[0]
	severe := res.Scenarios[1]

	approxEqual(t, mild.Coverage.CoverageNumerator.Amount, 120_000*0.90, 1e-6, "mild scenario EBITDA")
	approxEqual(t, severe.Coverage.CoverageNumerator.Amount, 120_000*0.60, 1e-6, "severe scenario EBITDA")

	// Debt service must be identical across scenarios (only earnings are
	// stressed).
	approxEqual(t, mild.Coverage.AnnualDebtService.Amount, severe.Coverage.AnnualDebtService.Amount, 1e-9, "debt service held fixed across scenarios")

	if severe.Coverage.DSCR.Amount >= mild.Coverage.DSCR.Amount {
		t.Errorf("expected severe DSCR (%v) < mild DSCR (%v)", severe.Coverage.DSCR.Amount, mild.Coverage.DSCR.Amount)
	}

	if !severe.BreachesMinimumDSCR {
		t.Error("expected severe scenario to breach minimum DSCR")
	}

	foundFlag := false
	for _, f := range res.Flags {
		if f.Code == FlagScenarioBreachesDSCR && f.Scenario == "Severe downturn" {
			foundFlag = true
		}
	}
	if !foundFlag {
		t.Error("expected FlagScenarioBreachesDSCR for the severe scenario")
	}
}

// TestCalculate_InvalidLoanTermsExcluded verifies an invalid LoanTerms
// entry is excluded from schedules/debt service but still reported as an
// Issue, and a mix of valid + invalid entries still processes the valid
// ones.
func TestCalculate_InvalidLoanTermsExcluded(t *testing.T) {
	valid := standardLoan()
	invalid := LoanTerms{Principal: -1, AnnualInterestRate: 0.05, AmortizationYears: 5, Frequency: FrequencyMonthly}

	res := Calculate(Input{
		EBITDA:        AvailableValue(200_000),
		ProposedLoans: []LoanTerms{valid, invalid},
	})

	if len(res.ProposedSchedules) != 1 {
		t.Fatalf("expected 1 valid schedule (invalid excluded), got %d", len(res.ProposedSchedules))
	}
	if !HasErrors(res.Errors) {
		t.Error("expected an error for the invalid loan terms")
	}
	foundLoanRef := false
	for _, e := range res.Errors {
		if e.Loan == "proposed_loans[1]" {
			foundLoanRef = true
		}
	}
	if !foundLoanRef {
		t.Errorf("expected an error referencing proposed_loans[1], got %+v", res.Errors)
	}
}

// TestCalculate_FixedChargeCoverage verifies the fixed-charge coverage
// formula against a hand-computed expectation.
func TestCalculate_FixedChargeCoverage(t *testing.T) {
	loan := standardLoan()
	res := Calculate(Input{
		EBITDA:        AvailableValue(200_000),
		ProposedLoans: []LoanTerms{loan},
		FixedCharges: FixedChargeInputs{
			LeasePayments: AvailableValue(24_000),
			CashTaxes:     AvailableValue(15_000),
		},
	})

	if !res.BaseCase.FixedChargeCoverage.Available {
		t.Fatalf("expected FixedChargeCoverage available, base case=%+v", res.BaseCase)
	}
	wantNumerator := 200_000.0 - 15_000.0 + 24_000.0
	wantDenominator := res.BaseCase.AnnualDebtService.Amount + 24_000
	approxEqual(t, res.BaseCase.FixedChargeCoverage.Amount, wantNumerator/wantDenominator, 1e-9, "fixed charge coverage")
}

// TestCalculate_ExistingOnlyCaseExcludesProposed verifies
// Result.ExistingOnlyCase reflects only ExistingDebt, distinct from
// BaseCase which includes ProposedLoans too.
func TestCalculate_ExistingOnlyCaseExcludesProposed(t *testing.T) {
	existing := LoanTerms{Principal: 100_000, AnnualInterestRate: 0.05, AmortizationYears: 5, Frequency: FrequencyAnnual}
	proposed := standardLoan()

	res := Calculate(Input{
		EBITDA:        AvailableValue(200_000),
		ExistingDebt:  []LoanTerms{existing},
		ProposedLoans: []LoanTerms{proposed},
	})

	if res.ExistingOnlyCase.AnnualDebtService.Amount >= res.BaseCase.AnnualDebtService.Amount {
		t.Errorf("expected existing-only debt service (%v) < combined base case debt service (%v)", res.ExistingOnlyCase.AnnualDebtService.Amount, res.BaseCase.AnnualDebtService.Amount)
	}
	if !res.ExistingOnlyCase.TotalDebtBalance.Available || res.ExistingOnlyCase.TotalDebtBalance.Amount != 100_000 {
		t.Errorf("expected existing-only total debt balance 100000, got %+v", res.ExistingOnlyCase.TotalDebtBalance)
	}
}

// TestCalculate_CashFlowPreferredOverEBITDA verifies DSCR's numerator
// prefers Input.CashFlow over Input.EBITDA when both are supplied, while
// leverage ratios still use EBITDA specifically.
func TestCalculate_CashFlowPreferredOverEBITDA(t *testing.T) {
	loan := standardLoan()
	res := Calculate(Input{
		EBITDA:        AvailableValue(200_000),
		CashFlow:      AvailableValue(150_000),
		ProposedLoans: []LoanTerms{loan},
	})

	if res.BaseCase.NumeratorSource != CoverageSourceCashFlow {
		t.Errorf("expected CoverageSourceCashFlow, got %s", res.BaseCase.NumeratorSource)
	}
	wantDSCR := 150_000 / res.BaseCase.AnnualDebtService.Amount
	approxEqual(t, res.BaseCase.DSCR.Amount, wantDSCR, 1e-9, "DSCR uses cash flow numerator")

	// Leverage still uses EBITDA, not cash flow.
	approxEqual(t, res.BaseCase.DebtToEBITDA.Amount, 500_000.0/200_000.0, 1e-9, "DebtToEBITDA uses EBITDA")
}
