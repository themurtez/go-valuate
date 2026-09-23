package debt

// computeCoverage derives one CoverageResult from ebitda/cashFlow (the
// scenario's, possibly haircut, figures), schedules (the debt to sum
// annual debt service over), totalDebtBalance/interestExpense (already
// resolved for this view), cash, and fixedCharges.
func computeCoverage(ebitda, cashFlow Value, schedules []AmortizationSchedule, totalDebtBalance, interestExpense, cash Value, fc FixedChargeInputs) CoverageResult {
	var cr CoverageResult

	cr.NumeratorSource, cr.CoverageNumerator = resolveCoverageNumerator(ebitda, cashFlow)

	cr.AnnualDebtService = sumAnnualDebtService(schedules)

	if cr.CoverageNumerator.Available && cr.AnnualDebtService.Available && cr.AnnualDebtService.Amount != 0 {
		cr.DSCR = AvailableValue(cr.CoverageNumerator.Amount / cr.AnnualDebtService.Amount)
	}

	cr.FixedChargeCoverage = computeFixedChargeCoverage(cr.CoverageNumerator, cr.AnnualDebtService, fc)

	if cr.CoverageNumerator.Available && interestExpense.Available && interestExpense.Amount != 0 {
		cr.InterestCoverage = AvailableValue(cr.CoverageNumerator.Amount / interestExpense.Amount)
	}

	cr.TotalDebtBalance = totalDebtBalance

	if totalDebtBalance.Available && ebitda.Available && ebitda.Amount > 0 {
		cr.DebtToEBITDA = AvailableValue(totalDebtBalance.Amount / ebitda.Amount)
	}
	if totalDebtBalance.Available && cash.Available && ebitda.Available && ebitda.Amount > 0 {
		cr.NetDebtToEBITDA = AvailableValue((totalDebtBalance.Amount - cash.Amount) / ebitda.Amount)
	}

	return cr
}

// resolveCoverageNumerator returns cashFlow if available (CoverageSourceCashFlow),
// otherwise ebitda if available and positive-or-zero
// (CoverageSourceEBITDA), otherwise CoverageSourceUnavailable.
//
// A negative EBITDA is deliberately still returned here (not silently
// converted to Unavailable) with CoverageSourceEBITDA, since
// IssueNegativeEBITDA already warns about it at the Result level and the
// downstream DSCR/interest-coverage computations already guard against a
// zero or meaningless denominator — this function's only job is picking
// which figure to use, not judging its sign.
func resolveCoverageNumerator(ebitda, cashFlow Value) (CoverageNumeratorSource, Value) {
	if cashFlow.Available {
		return CoverageSourceCashFlow, cashFlow
	}
	if ebitda.Available {
		return CoverageSourceEBITDA, ebitda
	}
	return CoverageSourceUnavailable, Unavailable()
}

// sumAnnualDebtService sums FirstYearAnnualDebtService across schedules.
// Returns AvailableValue(0) (not Unavailable) when schedules is empty,
// since "no debt" is a known figure of zero debt service, not a missing
// one — the caller-facing IssueNoDebt Warning is what flags this
// distinctly for a caller who wants to know why DSCR reads as
// "unavailable" (see CoverageResult.DSCR's doc comment: zero debt service
// makes DSCR itself unavailable even though AnnualDebtService is a known
// zero).
func sumAnnualDebtService(schedules []AmortizationSchedule) Value {
	var sum float64
	for _, s := range schedules {
		sum += s.FirstYearAnnualDebtService
	}
	return AvailableValue(sum)
}

// computeFixedChargeCoverage implements:
//
//	(numerator - CashTaxes - [UnfinancedCapex ? CapitalExpenditures : 0] + LeasePayments)
//	/ (annualDebtService + CurrentPortionLongTermDebt + LeasePayments)
//
// Available only when numerator and annualDebtService are available and at
// least one FixedChargeInputs field is available (otherwise this is
// identical to DSCR and not worth reporting separately) and the
// denominator is nonzero.
func computeFixedChargeCoverage(numerator, annualDebtService Value, fc FixedChargeInputs) Value {
	if !numerator.Available || !annualDebtService.Available {
		return Unavailable()
	}
	if !fc.LeasePayments.Available && !fc.CurrentPortionLongTermDebt.Available &&
		!fc.CashTaxes.Available && !fc.CapitalExpenditures.Available {
		return Unavailable()
	}

	num := numerator.Amount
	if fc.CashTaxes.Available {
		num -= fc.CashTaxes.Amount
	}
	if fc.UnfinancedCapex && fc.CapitalExpenditures.Available {
		num -= fc.CapitalExpenditures.Amount
	}
	if fc.LeasePayments.Available {
		num += fc.LeasePayments.Amount
	}

	denom := annualDebtService.Amount
	if fc.CurrentPortionLongTermDebt.Available {
		denom += fc.CurrentPortionLongTermDebt.Amount
	}
	if fc.LeasePayments.Available {
		denom += fc.LeasePayments.Amount
	}

	if denom == 0 {
		return Unavailable()
	}
	return AvailableValue(num / denom)
}
