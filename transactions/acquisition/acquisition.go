package acquisition

import (
	"fmt"

	"github.com/themurtez/go-valuate/analytics/debt"
)

// Calculate derives a full Result from in. It never mutates any
// caller-owned input and performs no I/O.
func Calculate(in Input) Result {
	result := Result{FormulaVersion: FormulaVersion, RedFlags: in.RedFlags}

	var issues []Issue

	schedules, tranchIssues := buildDebtSchedules(in.Financing.DebtTranches)
	issues = append(issues, tranchIssues...)
	result.DebtSchedules = schedules

	hasEarningsBase := in.Target.NormalizedEBITDA.Available || in.Target.NormalizedSDE.Available
	hasAnyInput := in.AskingPrice.Available || hasEarningsBase || in.Consensus.Value.Available ||
		len(in.Financing.DebtTranches) > 0 || in.Financing.BuyerCashContribution.Available
	if !hasAnyInput {
		issues = append(issues, Issue{
			Code:     IssueNoAskingPrice,
			Severity: SeverityError,
			Message:  "no asking price, earnings base, consensus value, or financing input supplied; nothing to screen",
		})
		result.Errors = issues
		return result
	}
	result.Available = true

	if !in.AskingPrice.Available {
		issues = append(issues, Issue{
			Code:     IssueNoAskingPrice,
			Severity: SeverityWarning,
			Message:  "no asking price supplied; every price-multiple and premium/discount figure is unavailable",
		})
	}
	if !hasEarningsBase {
		issues = append(issues, Issue{
			Code:     IssueNoEarningsBase,
			Severity: SeverityWarning,
			Message:  "neither normalized EBITDA nor normalized SDE supplied; every earnings-based multiple, coverage, and leverage figure is unavailable",
		})
	}
	if !in.Consensus.Value.Available {
		issues = append(issues, Issue{
			Code:     IssueNoConsensusValue,
			Severity: SeverityWarning,
			Message:  "no consensus valuation supplied; premium/discount-to-consensus figures are unavailable",
		})
	}
	if len(in.Financing.DebtTranches) == 0 {
		issues = append(issues, Issue{
			Code:     IssueAllCashDeal,
			Severity: SeverityWarning,
			Message:  "no debt tranches supplied; annual debt service is treated as zero and DSCR/leverage figures requiring nonzero debt service are unavailable",
		})
	}
	if in.RedFlags == (RedFlagThresholds{}) {
		issues = append(issues, Issue{
			Code:     IssueNoRedFlagThresholds,
			Severity: SeverityWarning,
			Message:  "no red-flag thresholds supplied; every red-flag check is skipped",
		})
	}

	result.Multiples = computeMultiples(in.AskingPrice, in.Target)
	result.Consensus = computeConsensusComparison(in.AskingPrice, in.Consensus)

	totalDebt := sumPrincipal(schedules)
	result.SourcesAndUses = computeSourcesAndUses(in, totalDebt)

	annualDebtService := sumAnnualDebtService(schedules)
	result.BaseCase = computeCoverage(in.Target.NormalizedEBITDA, in.Target.NormalizedSDE, annualDebtService, totalDebt, in.Capex, in.BuyerCompensation)

	cashContribution := resolveCashContribution(in.Financing.BuyerCashContribution, result.SourcesAndUses.RequiredEquity)
	if !cashContribution.Available {
		issues = append(issues, Issue{
			Code:     IssueNoCashContribution,
			Severity: SeverityWarning,
			Message:  "no buyer cash contribution or computable required equity available; cash-on-cash return and payback period are unavailable",
		})
	}
	result.Returns = computeReturns(cashContribution, result.BaseCase.PostDebtCashFlow)

	result.Scenarios = computeScenarios(in, schedules, totalDebt, cashContribution)

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

// buildDebtSchedules validates and amortizes every entry in tranches,
// returning one debt.AmortizationSchedule per valid entry (invalid
// entries are skipped entirely, not included as a zero-value schedule)
// plus one IssueInvalidLoanTerms per invalid entry.
func buildDebtSchedules(tranches []debt.LoanTerms) ([]debt.AmortizationSchedule, []Issue) {
	var schedules []debt.AmortizationSchedule
	var issues []Issue
	for i, t := range tranches {
		ref := fmt.Sprintf("debt_tranches[%d]", i)
		if !validLoanTerms(t) {
			issues = append(issues, Issue{
				Code:     IssueInvalidLoanTerms,
				Severity: SeverityError,
				Message:  "tranche has invalid loan terms (principal/rate must be >= 0, amortization years must be > 0, frequency must be recognized, interest-only years must be less than amortization years) and was excluded",
				Tranche:  ref,
			})
			continue
		}
		schedules = append(schedules, debt.Amortize(t))
	}
	return schedules, issues
}

// validLoanTerms mirrors the structural validation debt.Amortize itself
// depends on its caller having already performed (see
// debt.Amortize's doc comment: it assumes valid input). This package
// re-implements that same check rather than importing an unexported
// function, since debt.Amortize on a structurally invalid LoanTerms
// otherwise silently returns a degenerate zero-value-ish schedule instead
// of being excluded.
func validLoanTerms(t debt.LoanTerms) bool {
	if t.Principal < 0 || t.AnnualInterestRate < 0 || t.AmortizationYears <= 0 || t.InterestOnlyYears < 0 {
		return false
	}
	if t.InterestOnlyYears >= t.AmortizationYears {
		return false
	}
	switch t.Frequency {
	case debt.FrequencyMonthly, debt.FrequencyQuarterly, debt.FrequencyAnnual:
		return true
	default:
		return false
	}
}

// sumPrincipal sums every schedule's Terms.Principal. Returns
// AvailableValue(0) (not Unavailable) for an empty slice — "no debt" is a
// known figure of zero, not a missing one (see SourcesAndUses.
// TotalDebtFinancing's doc comment).
func sumPrincipal(schedules []debt.AmortizationSchedule) Value {
	var sum float64
	for _, s := range schedules {
		sum += s.Terms.Principal
	}
	return AvailableValue(sum)
}

// sumAnnualDebtService sums every schedule's FirstYearAnnualDebtService.
// Returns AvailableValue(0) (not Unavailable) for an empty slice, mirroring
// sumPrincipal.
func sumAnnualDebtService(schedules []debt.AmortizationSchedule) Value {
	var sum float64
	for _, s := range schedules {
		sum += s.FirstYearAnnualDebtService
	}
	return AvailableValue(sum)
}

// resolveCashContribution returns supplied if available, otherwise
// requiredEquity — see Financing.BuyerCashContribution's doc comment.
func resolveCashContribution(supplied, requiredEquity Value) Value {
	if supplied.Available {
		return supplied
	}
	return requiredEquity
}
