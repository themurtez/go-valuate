package debt

import "math"

// validateLoanTerms reports every structural problem with t as Issues
// (Loan identifies which slice/index t came from), and whether t is usable
// at all for amortization (usable == false means t.AmortizationYears <= 0,
// t.Principal < 0, t.AnnualInterestRate < 0, an unrecognized t.Frequency,
// or t.InterestOnlyYears >= t.AmortizationYears — any of which make the
// amortization formula undefined or nonsensical).
func validateLoanTerms(t LoanTerms, loanRef string) (issues []Issue, usable bool) {
	usable = true

	if t.Principal < 0 {
		issues = append(issues, Issue{
			Code:     IssueInvalidLoanTerms,
			Severity: SeverityError,
			Message:  "principal must be >= 0",
			Loan:     loanRef,
		})
		usable = false
	}
	if t.AnnualInterestRate < 0 {
		issues = append(issues, Issue{
			Code:     IssueInvalidLoanTerms,
			Severity: SeverityError,
			Message:  "annual interest rate must be >= 0",
			Loan:     loanRef,
		})
		usable = false
	}
	if t.AmortizationYears <= 0 {
		issues = append(issues, Issue{
			Code:     IssueInvalidLoanTerms,
			Severity: SeverityError,
			Message:  "amortization years must be > 0",
			Loan:     loanRef,
		})
		usable = false
	}
	if _, ok := paymentsPerYear(t.Frequency); !ok {
		issues = append(issues, Issue{
			Code:     IssueInvalidLoanTerms,
			Severity: SeverityError,
			Message:  "frequency must be one of \"monthly\", \"quarterly\", or \"annual\"",
			Loan:     loanRef,
		})
		usable = false
	}
	if t.InterestOnlyYears < 0 {
		issues = append(issues, Issue{
			Code:     IssueInvalidLoanTerms,
			Severity: SeverityError,
			Message:  "interest-only years must be >= 0",
			Loan:     loanRef,
		})
		usable = false
	} else if t.AmortizationYears > 0 && t.InterestOnlyYears >= t.AmortizationYears {
		issues = append(issues, Issue{
			Code:     IssueInvalidLoanTerms,
			Severity: SeverityError,
			Message:  "interest-only years must be less than amortization years",
			Loan:     loanRef,
		})
		usable = false
	}

	if usable && t.AnnualInterestRate > 1.0 {
		issues = append(issues, Issue{
			Code:     IssueSuspiciousInterestRate,
			Severity: SeverityWarning,
			Message:  "annual interest rate is above 1.0 (100%); verify it was supplied as a decimal (0.08 for 8%), not a whole-number percent",
			Loan:     loanRef,
		})
	}

	return issues, usable
}

// Amortize derives an AmortizationSchedule from t. The caller is
// responsible for having validated t via validateLoanTerms (or accepting
// that a degenerate t produces a degenerate schedule); Calculate never
// calls Amortize on a LoanTerms that failed validation.
//
// The level-payment formula used is the standard fixed-rate amortizing
// loan formula:
//
//	payment = principal x rate / (1 - (1 + rate)^-n)
//
// where rate is the periodic interest rate (AnnualInterestRate /
// paymentsPerYear) and n is the total number of amortizing payments
// (AmortizationYears x paymentsPerYear). If rate is exactly zero, payment
// is simply principal / n (straight-line, no interest).
func Amortize(t LoanTerms) AmortizationSchedule {
	ppy, ok := paymentsPerYear(t.Frequency)
	if !ok {
		return AmortizationSchedule{Terms: t}
	}

	periodicRate := t.AnnualInterestRate / float64(ppy)
	n := t.AmortizationYears * float64(ppy)

	var payment float64
	if periodicRate == 0 {
		payment = t.Principal / n
	} else {
		payment = t.Principal * periodicRate / (1 - math.Pow(1+periodicRate, -n))
	}

	sched := AmortizationSchedule{
		Terms:                               t,
		PaymentsPerYear:                     ppy,
		PeriodicPrincipalAndInterestPayment: payment,
		SteadyStateAnnualDebtService:        payment * float64(ppy),
	}

	if t.InterestOnlyYears > 0 {
		sched.PeriodicInterestOnlyPayment = t.Principal * periodicRate
	}

	sched.FirstYearAnnualDebtService = firstYearDebtService(sched)

	return sched
}

// firstYearDebtService computes the total principal + interest paid in the
// first 12 months of s.Terms, which may fall entirely within an
// interest-only period, entirely within full amortization (no
// interest-only period, or the I/O period is already less than a year and
// this "first year" spans the transition), or straddle the transition.
func firstYearDebtService(s AmortizationSchedule) float64 {
	io := s.Terms.InterestOnlyYears
	if io <= 0 {
		return s.SteadyStateAnnualDebtService
	}
	if io >= 1 {
		return s.PeriodicInterestOnlyPayment * float64(s.PaymentsPerYear)
	}

	// The first year straddles the interest-only-to-amortizing transition:
	// io fraction of the year is interest-only payments, the remainder is
	// full principal-and-interest payments, both pro-rated by
	// PaymentsPerYear.
	ioPayments := io * float64(s.PaymentsPerYear)
	amortizingPayments := float64(s.PaymentsPerYear) - ioPayments
	return ioPayments*s.PeriodicInterestOnlyPayment + amortizingPayments*s.PeriodicPrincipalAndInterestPayment
}
