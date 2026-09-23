package dealstructure

import (
	"math"
	"testing"
)

const epsilon = 0.01

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

// TestBuild_Empty covers the degenerate zero-Input case: nothing
// supplied at all.
func TestBuild_Empty(t *testing.T) {
	res := Build(Input{})
	if res.Available {
		t.Fatalf("expected Available=false for empty input, got Result=%+v", res)
	}
	if !HasErrors(res.Errors) {
		t.Fatal("expected at least one SeverityError issue")
	}
}

// TestBuild_SingleLoan covers the simplest financed deal: a purchase
// price funded by buyer equity plus a single fully-amortizing bank term
// loan, no seller note, no earnout, no fees.
func TestBuild_SingleLoan(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(2_000_000),
		BuyerEquity:   AvailableValue(500_000),
		DebtTranches: []DebtTranche{
			{
				Label:              "Bank term loan",
				Amount:             1_500_000,
				AnnualInterestRate: 0.08,
				AmortizationYears:  10,
				TermYears:          10,
				Frequency:          FrequencyMonthly,
			},
		},
	}
	res := Build(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if HasErrors(res.Errors) {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}

	if !res.SourcesAndUses.TotalUses.Available || !approxEqual(res.SourcesAndUses.TotalUses.Amount, 2_000_000) {
		t.Fatalf("expected TotalUses=2,000,000, got %+v", res.SourcesAndUses.TotalUses)
	}
	if !approxEqual(res.SourcesAndUses.TotalDebtFinancing.Amount, 1_500_000) {
		t.Fatalf("expected TotalDebtFinancing=1,500,000, got %v", res.SourcesAndUses.TotalDebtFinancing.Amount)
	}
	if !approxEqual(res.SourcesAndUses.TotalSources.Amount, 2_000_000) {
		t.Fatalf("expected TotalSources=2,000,000 (1.5M debt + 0.5M equity), got %v", res.SourcesAndUses.TotalSources.Amount)
	}
	if !approxEqual(res.SourcesAndUses.FundingGapOrSurplus.Amount, 0) {
		t.Fatalf("expected exactly-funded deal (gap/surplus=0), got %v", res.SourcesAndUses.FundingGapOrSurplus.Amount)
	}

	if len(res.DebtSchedules) != 1 {
		t.Fatalf("expected 1 debt schedule, got %d", len(res.DebtSchedules))
	}
	sched := res.DebtSchedules[0]
	if len(sched.Periods) != 120 {
		t.Fatalf("expected 120 monthly periods over 10 years, got %d", len(sched.Periods))
	}
	// A fully-amortizing loan's final period must bring the balance to
	// exactly zero, with no balloon.
	last := sched.Periods[len(sched.Periods)-1]
	if !approxEqual(last.EndingBalance, 0) {
		t.Fatalf("expected final ending balance ~0, got %v", last.EndingBalance)
	}
	if last.Balloon != 0 {
		t.Fatalf("expected no balloon on a fully-amortizing loan, got %v", last.Balloon)
	}

	// Standard amortization formula check: payment = P x r / (1 - (1+r)^-n)
	r := 0.08 / 12
	n := 120.0
	wantPayment := 1_500_000 * r / (1 - math.Pow(1+r, -n))
	if !approxEqual(sched.PeriodicPrincipalAndInterestPayment, wantPayment) {
		t.Fatalf("expected periodic payment ~%v, got %v", wantPayment, sched.PeriodicPrincipalAndInterestPayment)
	}

	if len(res.AnnualDebtService) != 10 {
		t.Fatalf("expected 10 years of annual debt service, got %d", len(res.AnnualDebtService))
	}
	// Year 1 total payment should equal 12 monthly payments.
	wantYear1 := sched.PeriodicPrincipalAndInterestPayment * 12
	if !approxEqual(res.AnnualDebtService[0].Payment, wantYear1) {
		t.Fatalf("expected year 1 annual debt service ~%v, got %v", wantYear1, res.AnnualDebtService[0].Payment)
	}
}

// TestBuild_MultipleTranches covers a deal with two debt tranches of
// differing rates, amortization periods, and payment frequencies,
// proving totals sum correctly across tranches and annual debt service
// aggregates a monthly-pay tranche and an annual-pay tranche in the same
// year bucket.
func TestBuild_MultipleTranches(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(5_000_000),
		BuyerEquity:   AvailableValue(1_000_000),
		DebtTranches: []DebtTranche{
			{Label: "Senior term loan", Amount: 3_000_000, AnnualInterestRate: 0.075, AmortizationYears: 7, TermYears: 7, Frequency: FrequencyMonthly},
			{Label: "Mezzanine", Amount: 1_000_000, AnnualInterestRate: 0.12, AmortizationYears: 5, TermYears: 5, Frequency: FrequencyAnnual},
		},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	if len(res.DebtSchedules) != 2 {
		t.Fatalf("expected 2 debt schedules, got %d", len(res.DebtSchedules))
	}
	if !approxEqual(res.SourcesAndUses.TotalDebtFinancing.Amount, 4_000_000) {
		t.Fatalf("expected TotalDebtFinancing=4,000,000, got %v", res.SourcesAndUses.TotalDebtFinancing.Amount)
	}
	// Sources = 4M debt + 1M equity = 5M; uses = 5M purchase price -> exactly funded.
	if !approxEqual(res.SourcesAndUses.FundingGapOrSurplus.Amount, 0) {
		t.Fatalf("expected exactly-funded deal, got gap/surplus=%v", res.SourcesAndUses.FundingGapOrSurplus.Amount)
	}

	// The mezzanine tranche (5-year term) contributes nothing to years 6-7,
	// which only the 7-year senior loan spans.
	if len(res.AnnualDebtService) != 7 {
		t.Fatalf("expected 7 years of annual debt service (longest tranche), got %d", len(res.AnnualDebtService))
	}
	senior := res.DebtSchedules[0]
	year7 := res.AnnualDebtService[6]
	wantYear7 := senior.PeriodicPrincipalAndInterestPayment * 12
	if !approxEqual(year7.Payment, wantYear7) {
		t.Fatalf("expected year 7 debt service to be senior-loan-only (~%v), got %v", wantYear7, year7.Payment)
	}

	// FinancingPercentages should sum to 1.0 across debt+equity (no
	// seller note/earnout in this deal).
	sumPct := res.FinancingPercentages.DebtPercent.Amount + res.FinancingPercentages.EquityPercent.Amount
	if !approxEqual(sumPct, 1.0) {
		t.Fatalf("expected financing percentages to sum to 1.0, got %v", sumPct)
	}
}

// TestBuild_SellerNote covers a deal partly financed by a seller note,
// proving it is tracked separately from third-party debt tranches in
// both SourcesAndUses and the schedule output.
func TestBuild_SellerNote(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(1_000_000),
		BuyerEquity:   AvailableValue(300_000),
		DebtTranches: []DebtTranche{
			{Label: "Bank loan", Amount: 500_000, AnnualInterestRate: 0.09, AmortizationYears: 5, TermYears: 5, Frequency: FrequencyMonthly},
		},
		SellerNote: SellerNote{
			Included: true,
			Terms: DebtTranche{
				Amount:             200_000,
				AnnualInterestRate: 0.06,
				AmortizationYears:  5,
				TermYears:          5,
				Frequency:          FrequencyAnnual,
			},
		},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	if res.SellerNoteSchedule == nil {
		t.Fatal("expected a non-nil SellerNoteSchedule")
	}
	if res.SellerNoteSchedule.Label != "Seller note" {
		t.Fatalf("expected seller note schedule labeled \"Seller note\", got %q", res.SellerNoteSchedule.Label)
	}
	if !approxEqual(res.SourcesAndUses.SellerNoteAmount.Amount, 200_000) {
		t.Fatalf("expected SellerNoteAmount=200,000, got %v", res.SourcesAndUses.SellerNoteAmount.Amount)
	}
	// Seller note is not counted in TotalDebtFinancing (third-party debt only).
	if !approxEqual(res.SourcesAndUses.TotalDebtFinancing.Amount, 500_000) {
		t.Fatalf("expected TotalDebtFinancing=500,000 (excludes seller note), got %v", res.SourcesAndUses.TotalDebtFinancing.Amount)
	}
	if !approxEqual(res.SourcesAndUses.TotalSources.Amount, 1_000_000) {
		t.Fatalf("expected TotalSources=1,000,000 (500k debt + 200k seller note + 300k equity), got %v", res.SourcesAndUses.TotalSources.Amount)
	}
	if !approxEqual(res.FinancingPercentages.SellerNotePercent.Amount, 0.2) {
		t.Fatalf("expected SellerNotePercent=0.2, got %v", res.FinancingPercentages.SellerNotePercent.Amount)
	}

	// The seller note's schedule should be included in aggregated annual
	// debt service alongside the bank loan.
	if len(res.AnnualDebtService) != 5 {
		t.Fatalf("expected 5 years of annual debt service, got %d", len(res.AnnualDebtService))
	}
}

// TestBuild_Balloon covers a loan amortized over a longer schedule than
// its actual term, with a balloon payment due at term end — proving the
// final period's Balloon/EndingBalance are correct and the balloon
// amount cannot exceed what the amortization schedule would leave
// outstanding.
func TestBuild_Balloon(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(1_000_000),
		BuyerEquity:   AvailableValue(200_000),
		DebtTranches: []DebtTranche{
			{
				Label:              "Bullet loan",
				Amount:             800_000,
				AnnualInterestRate: 0.07,
				AmortizationYears:  20,
				TermYears:          5,
				Frequency:          FrequencyAnnual,
			},
		},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	sched := res.DebtSchedules[0]
	if len(sched.Periods) != 5 {
		t.Fatalf("expected 5 annual periods (TermYears=5), got %d", len(sched.Periods))
	}
	last := sched.Periods[4]
	if last.Balloon <= 0 {
		t.Fatalf("expected a positive balloon at year 5 (loan amortized over 20 years), got %v", last.Balloon)
	}
	if !approxEqual(last.EndingBalance, 0) {
		t.Fatalf("expected ending balance 0 after the balloon payoff, got %v", last.EndingBalance)
	}
	// Exact regression check: the payment must be sized on the full
	// 20-year amortization (not on the 5-year term), and the balloon is
	// whatever that payment naturally leaves outstanding at year 5 — a
	// prior version of this package's payment-sizing incorrectly used
	// TermYears as the amortization exponent even when BalloonAmount was
	// unset, which silently shrank the payment and misreported the
	// balloon.
	wantBalance := 687_778.12
	if !approxEqual(last.Balloon, wantBalance) {
		t.Fatalf("expected balloon ~%.2f (balance after 5 years of a 20-year-amortization payment), got %v", wantBalance, last.Balloon)
	}
}

// TestBuild_ExplicitBalloonAmount covers a DebtTranche with an explicit
// BalloonAmount set on a loan that would otherwise fully amortize within
// its term (AmortizationYears > TermYears leaves room for a balloon;
// TermYears is intentionally less than AmortizationYears here — see
// DebtTranche.TermYears's doc comment — but the balloon itself is sized
// explicitly via BalloonAmount rather than left to whatever balance
// TermYears alone would leave outstanding), proving the payment is sized
// so the schedule lands on exactly that balance at TermYears.
func TestBuild_ExplicitBalloonAmount(t *testing.T) {
	tranche := DebtTranche{
		Amount:             1_000_000,
		AnnualInterestRate: 0.06,
		AmortizationYears:  15,
		TermYears:          10,
		Frequency:          FrequencyAnnual,
		BalloonAmount:      300_000,
	}
	in := Input{
		PurchasePrice: AvailableValue(1_000_000),
		BuyerEquity:   AvailableValue(0),
		DebtTranches:  []DebtTranche{tranche},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	sched := res.DebtSchedules[0]
	last := sched.Periods[len(sched.Periods)-1]
	if !approxEqual(last.Balloon, 300_000) {
		t.Fatalf("expected explicit balloon of 300,000 in the final period, got %v", last.Balloon)
	}
	if !approxEqual(last.EndingBalance, 0) {
		t.Fatalf("expected ending balance 0 after the balloon payoff, got %v", last.EndingBalance)
	}
	if sched.BalloonPayment != 300_000 {
		t.Fatalf("expected schedule.BalloonPayment=300,000, got %v", sched.BalloonPayment)
	}
}

// TestBuild_InterestOnlyPeriod covers a tranche with an interest-only
// period at the start, proving the first ioPeriods periods have zero
// principal and an unchanged balance.
func TestBuild_InterestOnlyPeriod(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(500_000),
		BuyerEquity:   AvailableValue(100_000),
		DebtTranches: []DebtTranche{
			{
				Amount:             400_000,
				AnnualInterestRate: 0.08,
				AmortizationYears:  10,
				TermYears:          10,
				Frequency:          FrequencyAnnual,
				InterestOnlyYears:  2,
			},
		},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	sched := res.DebtSchedules[0]
	if len(sched.Periods) != 10 {
		t.Fatalf("expected 10 periods, got %d", len(sched.Periods))
	}
	for i := 0; i < 2; i++ {
		p := sched.Periods[i]
		if p.Principal != 0 {
			t.Fatalf("period %d: expected zero principal during interest-only period, got %v", i+1, p.Principal)
		}
		if !approxEqual(p.EndingBalance, 400_000) {
			t.Fatalf("period %d: expected unchanged balance during interest-only period, got %v", i+1, p.EndingBalance)
		}
		wantInterest := 400_000 * 0.08
		if !approxEqual(p.Interest, wantInterest) {
			t.Fatalf("period %d: expected interest ~%v, got %v", i+1, wantInterest, p.Interest)
		}
	}
	// Amortization begins in period 3; the balance should decline from
	// there through the final period, which pays off to zero.
	if sched.Periods[2].Principal <= 0 {
		t.Fatalf("expected positive principal once amortization begins (period 3), got %v", sched.Periods[2].Principal)
	}
	last := sched.Periods[len(sched.Periods)-1]
	if !approxEqual(last.EndingBalance, 0) {
		t.Fatalf("expected full amortization by term end, got ending balance %v", last.EndingBalance)
	}
}

// TestBuild_InterestOnlyEntireTerm covers a tranche whose entire
// TermYears falls inside its own InterestOnlyYears (e.g. a 2-year
// interest-only bridge loan due in full at year 2) — a case a prior
// version of this package's amortization engine left unresolved
// (EndingBalance still showing the full original principal, with no
// Balloon recorded) because the interest-only branch returned before
// ever checking whether the period was also the tranche's final one.
// The full original principal, having never amortized, must come due as
// a balloon in the final period.
func TestBuild_InterestOnlyEntireTerm(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(500_000),
		BuyerEquity:   AvailableValue(0),
		DebtTranches: []DebtTranche{
			{
				Amount:             500_000,
				AnnualInterestRate: 0.08,
				AmortizationYears:  10,
				TermYears:          2,
				Frequency:          FrequencyAnnual,
				InterestOnlyYears:  2,
			},
		},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	sched := res.DebtSchedules[0]
	if len(sched.Periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(sched.Periods))
	}

	first := sched.Periods[0]
	if first.Principal != 0 || first.Balloon != 0 {
		t.Fatalf("expected period 1 to be a plain interest-only payment, got %+v", first)
	}
	if !approxEqual(first.EndingBalance, 500_000) {
		t.Fatalf("expected unchanged balance after period 1, got %v", first.EndingBalance)
	}

	last := sched.Periods[1]
	if !approxEqual(last.Balloon, 500_000) {
		t.Fatalf("expected the full original principal (500,000) due as a balloon at term end, got %v", last.Balloon)
	}
	if !approxEqual(last.EndingBalance, 0) {
		t.Fatalf("expected ending balance 0 after the balloon payoff, got %v", last.EndingBalance)
	}
	wantInterest := 500_000 * 0.08
	if !approxEqual(last.Interest, wantInterest) {
		t.Fatalf("expected final-period interest ~%v, got %v", wantInterest, last.Interest)
	}
	wantPayment := wantInterest + 500_000
	if !approxEqual(last.Payment, wantPayment) {
		t.Fatalf("expected final-period payment (interest + balloon) ~%v, got %v", wantPayment, last.Payment)
	}
}

// TestBuild_FundingGap covers a deal whose sources fall short of its
// uses, proving FundingGapOrSurplus is negative and IssueFundingGap is
// recorded as a warning (not an error — the caller may still want the
// computed figures to see the size of the gap).
func TestBuild_FundingGap(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(2_000_000),
		BuyerEquity:   AvailableValue(200_000),
		DebtTranches: []DebtTranche{
			{Amount: 1_000_000, AnnualInterestRate: 0.08, AmortizationYears: 10, TermYears: 10, Frequency: FrequencyMonthly},
		},
	}
	res := Build(in)
	if !res.Available {
		t.Fatalf("expected Available despite the funding gap, errors=%+v", res.Errors)
	}
	if HasErrors(res.Errors) {
		t.Fatalf("a funding gap should be a warning, not an error: %+v", res.Errors)
	}
	if !res.SourcesAndUses.FundingGapOrSurplus.Available {
		t.Fatal("expected FundingGapOrSurplus to be available")
	}
	// Sources = 1.0M debt + 0.2M equity = 1.2M; uses = 2.0M -> gap of -0.8M.
	if !approxEqual(res.SourcesAndUses.FundingGapOrSurplus.Amount, -800_000) {
		t.Fatalf("expected funding gap of -800,000, got %v", res.SourcesAndUses.FundingGapOrSurplus.Amount)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueFundingGap {
			found = true
		}
	}
	if !found {
		t.Fatal("expected IssueFundingGap warning")
	}
}

// TestBuild_FundingSurplus covers a deal whose sources exceed its uses.
func TestBuild_FundingSurplus(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(1_000_000),
		BuyerEquity:   AvailableValue(500_000),
		DebtTranches: []DebtTranche{
			{Amount: 700_000, AnnualInterestRate: 0.08, AmortizationYears: 10, TermYears: 10, Frequency: FrequencyMonthly},
		},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	// Sources = 700k debt + 500k equity = 1.2M; uses = 1.0M -> surplus of +0.2M.
	if !approxEqual(res.SourcesAndUses.FundingGapOrSurplus.Amount, 200_000) {
		t.Fatalf("expected funding surplus of 200,000, got %v", res.SourcesAndUses.FundingGapOrSurplus.Amount)
	}
	for _, w := range res.Warnings {
		if w.Code == IssueFundingGap {
			t.Fatal("did not expect IssueFundingGap warning on a funding surplus")
		}
	}
}

// TestBuild_InvalidTerms covers several kinds of structurally invalid
// DebtTranche input, proving each is excluded and reported as a
// SeverityError Issue rather than silently amortized anyway.
func TestBuild_InvalidTerms(t *testing.T) {
	cases := []struct {
		name    string
		tranche DebtTranche
	}{
		{"negative amount", DebtTranche{Amount: -100, AnnualInterestRate: 0.08, AmortizationYears: 5, TermYears: 5, Frequency: FrequencyAnnual}},
		{"negative rate", DebtTranche{Amount: 100_000, AnnualInterestRate: -0.01, AmortizationYears: 5, TermYears: 5, Frequency: FrequencyAnnual}},
		{"zero amortization years", DebtTranche{Amount: 100_000, AnnualInterestRate: 0.08, AmortizationYears: 0, TermYears: 5, Frequency: FrequencyAnnual}},
		{"zero term years", DebtTranche{Amount: 100_000, AnnualInterestRate: 0.08, AmortizationYears: 5, TermYears: 0, Frequency: FrequencyAnnual}},
		{"term exceeds amortization", DebtTranche{Amount: 100_000, AnnualInterestRate: 0.08, AmortizationYears: 5, TermYears: 10, Frequency: FrequencyAnnual}},
		{"unrecognized frequency", DebtTranche{Amount: 100_000, AnnualInterestRate: 0.08, AmortizationYears: 5, TermYears: 5, Frequency: "weekly"}},
		{"interest-only exceeds amortization", DebtTranche{Amount: 100_000, AnnualInterestRate: 0.08, AmortizationYears: 5, TermYears: 5, Frequency: FrequencyAnnual, InterestOnlyYears: 5}},
		{"negative balloon", DebtTranche{Amount: 100_000, AnnualInterestRate: 0.08, AmortizationYears: 5, TermYears: 5, Frequency: FrequencyAnnual, BalloonAmount: -1}},
		{"balloon exceeds amortizing balance", DebtTranche{Amount: 100_000, AnnualInterestRate: 0.08, AmortizationYears: 5, TermYears: 5, Frequency: FrequencyAnnual, BalloonAmount: 99_999_999}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := Input{
				PurchasePrice: AvailableValue(500_000),
				DebtTranches:  []DebtTranche{tc.tranche},
			}
			res := Build(in)
			if !res.Available {
				t.Fatalf("expected Available (purchase price alone is enough), got errors=%+v", res.Errors)
			}
			if len(res.DebtSchedules) != 0 {
				t.Fatalf("expected the invalid tranche to be excluded, got %d schedules", len(res.DebtSchedules))
			}
			if !HasErrors(res.Errors) {
				t.Fatal("expected a SeverityError issue for the invalid tranche")
			}
			foundCode := false
			for _, e := range res.Errors {
				if e.Code == IssueInvalidTrancheTerms {
					foundCode = true
				}
			}
			if !foundCode {
				t.Fatal("expected IssueInvalidTrancheTerms")
			}
		})
	}
}

// TestBuild_Earnout covers a deal with a deterministic earnout schedule,
// proving payments are sorted by period, invalid entries are excluded
// (reported as a SeverityError Issue, mirroring
// IssueInvalidTrancheTerms's identical "excluded, not blocking" severity
// convention — Result.Available stays true and every other figure is
// still computed), and the total is folded into sources.
func TestBuild_Earnout(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(1_000_000),
		BuyerEquity:   AvailableValue(700_000),
		Earnout: Earnout{
			Included: true,
			Payments: []EarnoutPayment{
				{PeriodNumber: 2, Amount: 150_000, Label: "Year 2"},
				{PeriodNumber: 1, Amount: 100_000, Label: "Year 1"},
				{PeriodNumber: 3, Amount: -5, Label: "invalid"},
			},
		},
	}
	res := Build(in)
	if !res.Available {
		t.Fatalf("expected Available despite the one invalid earnout entry, errors=%+v", res.Errors)
	}
	if len(res.EarnoutSchedule) != 2 {
		t.Fatalf("expected 2 valid earnout payments, got %d", len(res.EarnoutSchedule))
	}
	if res.EarnoutSchedule[0].PeriodNumber != 1 || res.EarnoutSchedule[1].PeriodNumber != 2 {
		t.Fatalf("expected earnout schedule sorted by period number, got %+v", res.EarnoutSchedule)
	}
	if !approxEqual(res.SourcesAndUses.TotalEarnoutAmount.Amount, 250_000) {
		t.Fatalf("expected TotalEarnoutAmount=250,000 (excludes invalid entry), got %v", res.SourcesAndUses.TotalEarnoutAmount.Amount)
	}
	// Sources = 700k equity + 250k earnout = 950k; uses = 1.0M -> gap of -50k.
	if !approxEqual(res.SourcesAndUses.FundingGapOrSurplus.Amount, -50_000) {
		t.Fatalf("expected funding gap of -50,000, got %v", res.SourcesAndUses.FundingGapOrSurplus.Amount)
	}
	foundInvalid := false
	for _, e := range res.Errors {
		if e.Code == IssueInvalidEarnoutPayment {
			foundInvalid = true
		}
	}
	if !foundInvalid {
		t.Fatal("expected IssueInvalidEarnoutPayment error for the negative-amount entry")
	}
}

// TestBuild_ZeroRateLoan covers a zero-interest-rate tranche, exercising
// the straight-line branch of the amortization formula.
func TestBuild_ZeroRateLoan(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(500_000),
		BuyerEquity:   AvailableValue(0),
		DebtTranches: []DebtTranche{
			{Amount: 500_000, AnnualInterestRate: 0, AmortizationYears: 5, TermYears: 5, Frequency: FrequencyAnnual},
		},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	sched := res.DebtSchedules[0]
	wantPayment := 500_000.0 / 5
	if !approxEqual(sched.PeriodicPrincipalAndInterestPayment, wantPayment) {
		t.Fatalf("expected straight-line payment of %v, got %v", wantPayment, sched.PeriodicPrincipalAndInterestPayment)
	}
	for _, p := range sched.Periods {
		if p.Interest != 0 {
			t.Fatalf("expected zero interest on a zero-rate loan, got %v", p.Interest)
		}
	}
}

// TestBuild_ClosingAdjustments covers CashAcquired reducing uses and
// AssumedDebt increasing them.
func TestBuild_ClosingAdjustments(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(1_000_000),
		BuyerEquity:   AvailableValue(1_000_000),
		ClosingAdjustments: ClosingAdjustments{
			CashAcquired: AvailableValue(50_000),
			AssumedDebt:  AvailableValue(20_000),
		},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	// Uses = 1,000,000 + 20,000 (assumed debt) - 50,000 (cash acquired) = 970,000.
	if !approxEqual(res.SourcesAndUses.TotalUses.Amount, 970_000) {
		t.Fatalf("expected TotalUses=970,000, got %v", res.SourcesAndUses.TotalUses.Amount)
	}
}

// TestBuild_Fees covers TransactionFees folded into TotalUses.
func TestBuild_Fees(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(1_000_000),
		BuyerEquity:   AvailableValue(1_100_000),
		Fees: TransactionFees{
			LegalAndAdvisory: AvailableValue(30_000),
			DueDiligence:     AvailableValue(20_000),
			FinancingFees:    AvailableValue(10_000),
			Other:            AvailableValue(5_000),
		},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	if !approxEqual(res.SourcesAndUses.TotalUses.Amount, 1_065_000) {
		t.Fatalf("expected TotalUses=1,065,000 (1M price + 65k fees), got %v", res.SourcesAndUses.TotalUses.Amount)
	}
}

// TestBuild_WorkingCapital covers a WorkingCapitalContribution folded
// into TotalUses.
func TestBuild_WorkingCapital(t *testing.T) {
	in := Input{
		PurchasePrice:  AvailableValue(1_000_000),
		BuyerEquity:    AvailableValue(1_075_000),
		WorkingCapital: WorkingCapitalContribution{Amount: AvailableValue(75_000)},
	}
	res := Build(in)
	if !res.Available || HasErrors(res.Errors) {
		t.Fatalf("expected clean Available result, errors=%+v", res.Errors)
	}
	if !approxEqual(res.SourcesAndUses.TotalUses.Amount, 1_075_000) {
		t.Fatalf("expected TotalUses=1,075,000, got %v", res.SourcesAndUses.TotalUses.Amount)
	}
}

// TestBuild_SuspiciousInterestRate covers a rate supplied above 1.0
// (100%), almost always a units mistake (a whole-number percent instead
// of a decimal) — flagged as an advisory SeverityWarning while the
// tranche is still amortized using the rate as supplied, mirroring
// analytics/debt.IssueSuspiciousInterestRate's identical convention.
func TestBuild_SuspiciousInterestRate(t *testing.T) {
	in := Input{
		PurchasePrice: AvailableValue(500_000),
		BuyerEquity:   AvailableValue(0),
		DebtTranches: []DebtTranche{
			{Amount: 500_000, AnnualInterestRate: 8, AmortizationYears: 5, TermYears: 5, Frequency: FrequencyAnnual},
		},
	}
	res := Build(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if HasErrors(res.Errors) {
		t.Fatalf("a suspicious rate should be a warning, not an error: %+v", res.Errors)
	}
	if len(res.DebtSchedules) != 1 {
		t.Fatalf("expected the tranche to still be amortized despite the suspicious rate, got %d schedules", len(res.DebtSchedules))
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueInvalidTrancheTerms {
			found = true
		}
	}
	if !found {
		t.Fatal("expected an IssueInvalidTrancheTerms warning for the suspicious rate")
	}
}

// TestBuild_NoFinancingSources covers a purchase price supplied with no
// buyer equity, debt, seller note, or earnout at all — TotalSources
// should be a known zero and IssueNoFinancingSources recorded.
func TestBuild_NoFinancingSources(t *testing.T) {
	in := Input{PurchasePrice: AvailableValue(1_000_000)}
	res := Build(in)
	if !res.Available {
		t.Fatalf("expected Available (a purchase price alone is enough), errors=%+v", res.Errors)
	}
	if !res.SourcesAndUses.TotalSources.Available || res.SourcesAndUses.TotalSources.Amount != 0 {
		t.Fatalf("expected TotalSources to be a known zero, got %+v", res.SourcesAndUses.TotalSources)
	}
	if !approxEqual(res.SourcesAndUses.FundingGapOrSurplus.Amount, -1_000_000) {
		t.Fatalf("expected a full funding gap of -1,000,000, got %v", res.SourcesAndUses.FundingGapOrSurplus.Amount)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoFinancingSources {
			found = true
		}
	}
	if !found {
		t.Fatal("expected IssueNoFinancingSources warning")
	}
}

// TestBuild_NoPurchasePriceOnlyFinancing covers financing supplied
// without a purchase price: schedules and TotalSources are still
// computed, but TotalUses/RequiredEquity/FundingGapOrSurplus are
// unavailable.
func TestBuild_NoPurchasePriceOnlyFinancing(t *testing.T) {
	in := Input{
		BuyerEquity: AvailableValue(200_000),
		DebtTranches: []DebtTranche{
			{Amount: 800_000, AnnualInterestRate: 0.08, AmortizationYears: 10, TermYears: 10, Frequency: FrequencyMonthly},
		},
	}
	res := Build(in)
	if !res.Available {
		t.Fatalf("expected Available (financing alone is enough), errors=%+v", res.Errors)
	}
	if res.SourcesAndUses.TotalUses.Available {
		t.Fatal("expected TotalUses unavailable with no purchase price")
	}
	if res.SourcesAndUses.FundingGapOrSurplus.Available {
		t.Fatal("expected FundingGapOrSurplus unavailable with no purchase price")
	}
	if !res.SourcesAndUses.TotalSources.Available || !approxEqual(res.SourcesAndUses.TotalSources.Amount, 1_000_000) {
		t.Fatalf("expected TotalSources=1,000,000 even without a purchase price, got %+v", res.SourcesAndUses.TotalSources)
	}
	if len(res.DebtSchedules) != 1 {
		t.Fatalf("expected the debt schedule to still be computed, got %d", len(res.DebtSchedules))
	}
	foundNoPurchasePrice := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoPurchasePrice {
			foundNoPurchasePrice = true
		}
	}
	if !foundNoPurchasePrice {
		t.Fatal("expected IssueNoPurchasePrice warning")
	}
}
