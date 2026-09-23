package dealstructure

import "fmt"

// Build derives a full Result from in. It never mutates any caller-owned
// input and performs no I/O.
func Build(in Input) Result {
	result := Result{FormulaVersion: FormulaVersion}

	var issues []Issue

	debtSchedules, debtIssues := buildDebtSchedules(in.DebtTranches)
	issues = append(issues, debtIssues...)
	result.DebtSchedules = debtSchedules

	var sellerSchedule *AmortizationSchedule
	if in.SellerNote.Included {
		sellerIssues, usable := validateTrancheTerms(in.SellerNote.Terms, "seller_note")
		issues = append(issues, sellerIssues...)
		if usable {
			s := amortize(in.SellerNote.Terms)
			s.Label = "Seller note"
			sellerSchedule = &s
		}
	}
	result.SellerNoteSchedule = sellerSchedule

	earnoutSchedule, earnoutIssues, totalEarnout := buildEarnoutSchedule(in.Earnout)
	issues = append(issues, earnoutIssues...)
	result.EarnoutSchedule = earnoutSchedule

	hasFinancingInput := in.BuyerEquity.Available || len(in.DebtTranches) > 0 || in.SellerNote.Included || in.Earnout.Included
	if !in.PurchasePrice.Available && !hasFinancingInput {
		issues = append(issues, Issue{
			Code:     IssueNoPurchasePrice,
			Severity: SeverityError,
			Message:  "no purchase price, buyer equity, debt tranches, seller note, or earnout supplied; nothing to structure",
		})
		result.Errors = issues
		return result
	}
	result.Available = true

	if !in.PurchasePrice.Available {
		issues = append(issues, Issue{
			Code:     IssueNoPurchasePrice,
			Severity: SeverityWarning,
			Message:  "no purchase price supplied; total uses, required equity, and funding gap/surplus are unavailable",
		})
	}
	if !hasFinancingInput {
		issues = append(issues, Issue{
			Code:     IssueNoFinancingSources,
			Severity: SeverityWarning,
			Message:  "no buyer equity, debt tranches, seller note, or earnout supplied; total sources is a known zero",
		})
	}

	totalDebt := sumTrancheAmounts(debtSchedules)
	sellerNoteAmount := AvailableValue(0)
	if sellerSchedule != nil {
		sellerNoteAmount = AvailableValue(sellerSchedule.Terms.Amount)
	}
	totalEarnoutValue := AvailableValue(totalEarnout)

	result.SourcesAndUses = computeSourcesAndUses(in, totalDebt, sellerNoteAmount, totalEarnoutValue)
	result.FinancingPercentages = computeFinancingPercentages(result.SourcesAndUses)

	if result.SourcesAndUses.FundingGapOrSurplus.Available && result.SourcesAndUses.FundingGapOrSurplus.Amount < 0 {
		issues = append(issues, Issue{
			Code:     IssueFundingGap,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("total sources fall short of total uses by %.2f; the deal as structured does not fully fund its uses", -result.SourcesAndUses.FundingGapOrSurplus.Amount),
		})
	}

	allSchedules := append([]AmortizationSchedule{}, debtSchedules...)
	if sellerSchedule != nil {
		allSchedules = append(allSchedules, *sellerSchedule)
	}
	result.AnnualDebtService = aggregateAnnualDebtService(allSchedules)

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
// returning one AmortizationSchedule per valid entry (invalid entries
// are skipped entirely, not included as a zero-value schedule) plus one
// IssueInvalidTrancheTerms per invalid entry.
func buildDebtSchedules(tranches []DebtTranche) ([]AmortizationSchedule, []Issue) {
	var schedules []AmortizationSchedule
	var issues []Issue
	for i, t := range tranches {
		ref := fmt.Sprintf("debt_tranches[%d]", i)
		trancheIssues, usable := validateTrancheTerms(t, ref)
		issues = append(issues, trancheIssues...)
		if !usable {
			continue
		}
		s := amortize(t)
		s.Label = t.Label
		schedules = append(schedules, s)
	}
	return schedules, issues
}

// sumTrancheAmounts sums every schedule's Terms.Amount. Returns
// AvailableValue(0) (not Unavailable) for an empty slice — "no debt" is
// a known figure of zero, not a missing one (mirroring
// acquisition.sumPrincipal's identical convention).
func sumTrancheAmounts(schedules []AmortizationSchedule) Value {
	var sum float64
	for _, s := range schedules {
		sum += s.Terms.Amount
	}
	return AvailableValue(sum)
}
