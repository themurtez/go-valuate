package debt

import (
	"math"
	"testing"
)

func approxEqual(t *testing.T, got, want, tolerance float64, msg string) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s: got %v, want %v (tolerance %v)", msg, got, want, tolerance)
	}
}

// TestAmortize_StandardLoan verifies the level-payment formula against a
// well-known reference figure: a $100,000 loan at 6% annual, 10-year
// monthly amortization has a monthly payment of $1,110.21 (a standard
// textbook/financial-calculator reference value).
func TestAmortize_StandardLoan(t *testing.T) {
	sched := Amortize(LoanTerms{
		Principal:          100_000,
		AnnualInterestRate: 0.06,
		AmortizationYears:  10,
		Frequency:          FrequencyMonthly,
	})

	if sched.PaymentsPerYear != 12 {
		t.Fatalf("PaymentsPerYear = %d, want 12", sched.PaymentsPerYear)
	}
	approxEqual(t, sched.PeriodicPrincipalAndInterestPayment, 1110.21, 0.01, "monthly payment")
	approxEqual(t, sched.SteadyStateAnnualDebtService, 1110.21*12, 0.1, "steady-state annual debt service")
	approxEqual(t, sched.FirstYearAnnualDebtService, sched.SteadyStateAnnualDebtService, 0.001, "first-year debt service with no I/O period")
}

// TestAmortize_ZeroInterest verifies the straight-line fallback when the
// rate is exactly zero.
func TestAmortize_ZeroInterest(t *testing.T) {
	sched := Amortize(LoanTerms{
		Principal:          120_000,
		AnnualInterestRate: 0,
		AmortizationYears:  10,
		Frequency:          FrequencyAnnual,
	})
	approxEqual(t, sched.PeriodicPrincipalAndInterestPayment, 12_000, 0.001, "zero-interest annual payment")
	approxEqual(t, sched.SteadyStateAnnualDebtService, 12_000, 0.001, "zero-interest annual debt service")
}

// TestAmortize_ZeroPrincipal verifies a zero-principal loan produces a
// zero payment without dividing by zero or NaN-ing out.
func TestAmortize_ZeroPrincipal(t *testing.T) {
	sched := Amortize(LoanTerms{
		Principal:          0,
		AnnualInterestRate: 0.07,
		AmortizationYears:  5,
		Frequency:          FrequencyQuarterly,
	})
	if math.IsNaN(sched.PeriodicPrincipalAndInterestPayment) {
		t.Fatal("payment is NaN for zero principal")
	}
	approxEqual(t, sched.PeriodicPrincipalAndInterestPayment, 0, 1e-9, "zero-principal payment")
}

// TestAmortize_InterestOnlyPeriod verifies a loan with a full-year (or
// more) interest-only period reports interest-only payments for
// FirstYearAnnualDebtService, and that the amortizing payment is still
// sized against the full AmortizationYears (not reduced by the I/O
// years) per LoanTerms.InterestOnlyYears's doc comment.
func TestAmortize_InterestOnlyPeriod(t *testing.T) {
	sched := Amortize(LoanTerms{
		Principal:          200_000,
		AnnualInterestRate: 0.08,
		AmortizationYears:  10,
		Frequency:          FrequencyAnnual,
		InterestOnlyYears:  2,
	})

	wantInterestOnlyPayment := 200_000 * 0.08
	approxEqual(t, sched.PeriodicInterestOnlyPayment, wantInterestOnlyPayment, 0.01, "interest-only payment")
	approxEqual(t, sched.FirstYearAnnualDebtService, wantInterestOnlyPayment, 0.01, "first-year debt service during I/O period")

	// The amortizing payment should match a plain 10-year amortization of
	// the full principal (I/O period defers, not shortens, amortization).
	plain := Amortize(LoanTerms{
		Principal:          200_000,
		AnnualInterestRate: 0.08,
		AmortizationYears:  10,
		Frequency:          FrequencyAnnual,
	})
	approxEqual(t, sched.PeriodicPrincipalAndInterestPayment, plain.PeriodicPrincipalAndInterestPayment, 0.01, "amortizing payment unaffected by I/O period")
}

// TestAmortize_InterestOnlyStraddlesFirstYear verifies a fractional I/O
// period (e.g. 0.5 years) blends interest-only and amortizing payments
// within the first 12 months.
func TestAmortize_InterestOnlyStraddlesFirstYear(t *testing.T) {
	sched := Amortize(LoanTerms{
		Principal:          200_000,
		AnnualInterestRate: 0.08,
		AmortizationYears:  10,
		Frequency:          FrequencyMonthly,
		InterestOnlyYears:  0.5,
	})

	// 6 months interest-only + 6 months amortizing.
	wantFirstYear := 6*sched.PeriodicInterestOnlyPayment + 6*sched.PeriodicPrincipalAndInterestPayment
	approxEqual(t, sched.FirstYearAnnualDebtService, wantFirstYear, 0.01, "straddled first-year debt service")

	if sched.FirstYearAnnualDebtService >= sched.SteadyStateAnnualDebtService {
		t.Errorf("expected straddled first-year debt service (%v) to be less than steady-state (%v), since part of the year is interest-only", sched.FirstYearAnnualDebtService, sched.SteadyStateAnnualDebtService)
	}
}

func TestValidateLoanTerms(t *testing.T) {
	tests := []struct {
		name       string
		terms      LoanTerms
		wantUsable bool
		wantCodes  []IssueCode
	}{
		{
			name: "valid",
			terms: LoanTerms{
				Principal: 100_000, AnnualInterestRate: 0.06, AmortizationYears: 10, Frequency: FrequencyMonthly,
			},
			wantUsable: true,
		},
		{
			name: "negative principal",
			terms: LoanTerms{
				Principal: -1, AnnualInterestRate: 0.06, AmortizationYears: 10, Frequency: FrequencyMonthly,
			},
			wantUsable: false,
			wantCodes:  []IssueCode{IssueInvalidLoanTerms},
		},
		{
			name: "negative rate",
			terms: LoanTerms{
				Principal: 100_000, AnnualInterestRate: -0.01, AmortizationYears: 10, Frequency: FrequencyMonthly,
			},
			wantUsable: false,
			wantCodes:  []IssueCode{IssueInvalidLoanTerms},
		},
		{
			name: "zero amortization years",
			terms: LoanTerms{
				Principal: 100_000, AnnualInterestRate: 0.06, AmortizationYears: 0, Frequency: FrequencyMonthly,
			},
			wantUsable: false,
			wantCodes:  []IssueCode{IssueInvalidLoanTerms},
		},
		{
			name: "invalid frequency",
			terms: LoanTerms{
				Principal: 100_000, AnnualInterestRate: 0.06, AmortizationYears: 10, Frequency: "weekly",
			},
			wantUsable: false,
			wantCodes:  []IssueCode{IssueInvalidLoanTerms},
		},
		{
			name: "interest-only >= amortization",
			terms: LoanTerms{
				Principal: 100_000, AnnualInterestRate: 0.06, AmortizationYears: 5, Frequency: FrequencyMonthly, InterestOnlyYears: 5,
			},
			wantUsable: false,
			wantCodes:  []IssueCode{IssueInvalidLoanTerms},
		},
		{
			name: "suspicious rate still usable",
			terms: LoanTerms{
				Principal: 100_000, AnnualInterestRate: 8, AmortizationYears: 10, Frequency: FrequencyMonthly,
			},
			wantUsable: true,
			wantCodes:  []IssueCode{IssueSuspiciousInterestRate},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, usable := validateLoanTerms(tt.terms, "test[0]")
			if usable != tt.wantUsable {
				t.Errorf("usable = %v, want %v", usable, tt.wantUsable)
			}
			if len(issues) != len(tt.wantCodes) {
				t.Fatalf("issues = %+v, want codes %v", issues, tt.wantCodes)
			}
			for i, code := range tt.wantCodes {
				if issues[i].Code != code {
					t.Errorf("issues[%d].Code = %s, want %s", i, issues[i].Code, code)
				}
				if issues[i].Loan != "test[0]" {
					t.Errorf("issues[%d].Loan = %s, want test[0]", i, issues[i].Loan)
				}
			}
		})
	}
}
