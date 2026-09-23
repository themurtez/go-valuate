package debt

import "fmt"

// Calculate derives a full Result from in. It never mutates any
// caller-owned input and performs no I/O.
func Calculate(in Input) Result {
	result := Result{FormulaVersion: FormulaVersion, Policy: in.Policy}

	var issues []Issue

	existingSchedules, existingIssues := buildSchedules(in.ExistingDebt, "existing_debt")
	proposedSchedules, proposedIssues := buildSchedules(in.ProposedLoans, "proposed_loans")
	issues = append(issues, existingIssues...)
	issues = append(issues, proposedIssues...)

	result.ExistingSchedules = existingSchedules
	result.ProposedSchedules = proposedSchedules

	hasDebtInput := len(in.ExistingDebt) > 0 || len(in.ProposedLoans) > 0 || in.ExistingDebtBalance.Available
	if !in.EBITDA.Available && !in.CashFlow.Available && !hasDebtInput {
		issues = append(issues, Issue{
			Code:     IssueNoEBITDA,
			Severity: SeverityError,
			Message:  "no EBITDA, cash flow, or debt input supplied; nothing to analyze",
		})
		result.Errors = issues
		return result
	}
	result.Available = true

	if !in.EBITDA.Available {
		issues = append(issues, Issue{
			Code:     IssueNoEBITDA,
			Severity: SeverityWarning,
			Message:  "no EBITDA supplied; coverage falls back to cash flow if available, and leverage/capacity figures that require EBITDA are unavailable",
		})
	} else if in.EBITDA.Amount <= 0 {
		issues = append(issues, Issue{
			Code:     IssueNegativeEBITDA,
			Severity: SeverityWarning,
			Message:  "EBITDA is zero or negative; ratios dividing by EBITDA are left unavailable rather than reporting a negative or infinite coverage/leverage figure",
		})
	}

	allSchedules := append(append([]AmortizationSchedule{}, existingSchedules...), proposedSchedules...)
	if len(allSchedules) == 0 && !in.ExistingDebtBalance.Available {
		issues = append(issues, Issue{
			Code:     IssueNoDebt,
			Severity: SeverityWarning,
			Message:  "no loan terms or existing debt balance supplied; debt service is treated as zero",
		})
	}

	if in.Policy == (LenderPolicy{}) {
		issues = append(issues, Issue{
			Code:     IssueNoLenderPolicy,
			Severity: SeverityWarning,
			Message:  "no lender policy thresholds supplied; maximum-capacity calculations are skipped",
		})
	}

	totalDebtBalance := resolveTotalDebtBalance(in, allSchedules)
	interestExpense := resolveInterestExpense(in, allSchedules)

	result.BaseCase = computeCoverage(in.EBITDA, in.CashFlow, allSchedules, totalDebtBalance, interestExpense, in.CashAndEquivalents, in.FixedCharges)
	result.ExistingOnlyCase = computeCoverage(in.EBITDA, in.CashFlow, existingSchedules, resolveTotalDebtBalanceFor(in, existingSchedules, false), resolveInterestExpenseFor(existingSchedules, Value{}), in.CashAndEquivalents, in.FixedCharges)

	result.Capacity = computeCapacity(in, result.BaseCase, totalDebtBalance)

	result.Scenarios = computeScenarios(in, allSchedules, totalDebtBalance, interestExpense)

	result.Flags = buildFlags(in, result)

	for i := range issues {
		if issues[i].Severity == SeverityError {
			result.Errors = append(result.Errors, issues[i])
		} else {
			result.Warnings = append(result.Warnings, issues[i])
		}
	}

	return result
}

// buildSchedules validates and amortizes every entry in terms, returning
// one AmortizationSchedule per valid entry (invalid entries are skipped
// entirely, not included as a zero-value schedule) plus every Issue found,
// with Loan set to "<sliceName>[i]" for the i'th entry.
func buildSchedules(terms []LoanTerms, sliceName string) ([]AmortizationSchedule, []Issue) {
	var schedules []AmortizationSchedule
	var issues []Issue
	for i, t := range terms {
		ref := fmt.Sprintf("%s[%d]", sliceName, i)
		loanIssues, usable := validateLoanTerms(t, ref)
		issues = append(issues, loanIssues...)
		if !usable {
			continue
		}
		schedules = append(schedules, Amortize(t))
	}
	return schedules, issues
}

// resolveTotalDebtBalance returns in.ExistingDebtBalance if available,
// otherwise the sum of every valid loan's Terms.Principal across both
// existing and proposed schedules.
func resolveTotalDebtBalance(in Input, allSchedules []AmortizationSchedule) Value {
	return resolveTotalDebtBalanceFor(in, allSchedules, true)
}

// resolveTotalDebtBalanceFor is resolveTotalDebtBalance's shared
// implementation. When useSuppliedBalance is false (the "existing debt
// only" view), Input.ExistingDebtBalance is never substituted, since that
// field represents the caller's total balance across all debt including
// any proposed loans — using it for the existing-only case would silently
// double-count or mismatch scope. In that case the schedules' own
// principal sum is always used.
func resolveTotalDebtBalanceFor(in Input, schedules []AmortizationSchedule, useSuppliedBalance bool) Value {
	if useSuppliedBalance && in.ExistingDebtBalance.Available {
		return in.ExistingDebtBalance
	}
	if len(schedules) == 0 {
		return Unavailable()
	}
	var sum float64
	for _, s := range schedules {
		sum += s.Terms.Principal
	}
	return AvailableValue(sum)
}

// resolveInterestExpense returns in.InterestExpense if available,
// otherwise the sum of every schedule's implied first-year interest
// (FirstYearAnnualDebtService minus the principal actually amortized in
// year one, approximated here as the periodic-payment-implied interest
// component: for a schedule with no interest-only period this is
// SteadyStateAnnualDebtService minus Terms.Principal/AmortizationYears'
// straight-line approximation is avoided in favor of the exact per-period
// rate x outstanding-balance calculation in interestComponentYearOne).
func resolveInterestExpense(in Input, allSchedules []AmortizationSchedule) Value {
	return resolveInterestExpenseFor(allSchedules, in.InterestExpense)
}

func resolveInterestExpenseFor(schedules []AmortizationSchedule, supplied Value) Value {
	if supplied.Available {
		return supplied
	}
	if len(schedules) == 0 {
		return Unavailable()
	}
	var sum float64
	for _, s := range schedules {
		sum += interestComponentYearOne(s)
	}
	return AvailableValue(sum)
}

// interestComponentYearOne returns the interest portion of s's first-year
// debt service, computed period-by-period against the declining balance
// (or the flat interest-only payment during an interest-only period)
// rather than assumed proportional to the payment — a level-payment loan's
// interest share of each payment declines every period, so this cannot be
// approximated as a fixed fraction of FirstYearAnnualDebtService.
func interestComponentYearOne(s AmortizationSchedule) float64 {
	ppy := s.PaymentsPerYear
	if ppy == 0 {
		return 0
	}
	periodicRate := s.Terms.AnnualInterestRate / float64(ppy)
	balance := s.Terms.Principal

	ioPeriods := int(s.Terms.InterestOnlyYears * float64(ppy))
	var interest float64
	periodsThisYear := ppy
	for p := 0; p < periodsThisYear; p++ {
		periodInterest := balance * periodicRate
		interest += periodInterest
		if p < ioPeriods {
			continue // interest-only: balance unchanged
		}
		principalPaid := s.PeriodicPrincipalAndInterestPayment - periodInterest
		balance -= principalPaid
	}
	return interest
}
