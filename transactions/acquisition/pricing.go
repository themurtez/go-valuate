package acquisition

// computeMultiples derives PriceMultiples from askingPrice and target.
func computeMultiples(askingPrice Value, target TargetFinancials) PriceMultiples {
	var m PriceMultiples

	if askingPrice.Available && target.Revenue.Available && target.Revenue.Amount != 0 {
		m.PriceToRevenue = AvailableValue(askingPrice.Amount / target.Revenue.Amount)
	}
	if askingPrice.Available && target.NormalizedEBITDA.Available && target.NormalizedEBITDA.Amount > 0 {
		m.PriceToEBITDA = AvailableValue(askingPrice.Amount / target.NormalizedEBITDA.Amount)
	}
	if askingPrice.Available && target.NormalizedSDE.Available && target.NormalizedSDE.Amount > 0 {
		m.PriceToSDE = AvailableValue(askingPrice.Amount / target.NormalizedSDE.Amount)
	}

	return m
}

// computeConsensusComparison derives ConsensusComparison from askingPrice
// and consensus.
func computeConsensusComparison(askingPrice Value, consensus ConsensusValuation) ConsensusComparison {
	c := ConsensusComparison{ConsensusValue: consensus.Value}

	if !askingPrice.Available || !consensus.Value.Available {
		return c
	}

	premium := askingPrice.Amount - consensus.Value.Amount
	c.Premium = AvailableValue(premium)

	if consensus.Value.Amount != 0 {
		c.PremiumPercent = AvailableValue(premium / consensus.Value.Amount)
	}

	return c
}

// computeSourcesAndUses derives SourcesAndUses from in and the already-
// resolved totalDebtFinancing.
func computeSourcesAndUses(in Input, totalDebtFinancing Value) SourcesAndUses {
	var su SourcesAndUses
	su.TotalDebtFinancing = totalDebtFinancing

	if !in.AskingPrice.Available {
		return su
	}

	total := in.AskingPrice.Amount
	total += in.Fees.total()
	if in.WorkingCapital.Amount.Available {
		total += in.WorkingCapital.Amount.Amount
	}
	su.TotalUses = AvailableValue(total)

	if totalDebtFinancing.Available {
		su.RequiredEquity = AvailableValue(total - totalDebtFinancing.Amount)
	}

	return su
}

// resolveEarningsBase returns ebitda if available (EarningsSourceEBITDA),
// otherwise sde if available (EarningsSourceSDE), otherwise
// EarningsSourceUnavailable.
//
// A non-positive earnings figure is deliberately still returned here (not
// silently converted to Unavailable), mirroring
// debt.resolveCoverageNumerator's rationale: this function's only job is
// picking which figure to use, not judging its sign. Downstream DSCR/
// leverage computations already guard against a non-positive denominator.
func resolveEarningsBase(ebitda, sde Value) (EarningsBaseSource, Value) {
	if ebitda.Available {
		return EarningsSourceEBITDA, ebitda
	}
	if sde.Available {
		return EarningsSourceSDE, sde
	}
	return EarningsSourceUnavailable, Unavailable()
}

// computeCoverage derives one CoverageResult from ebitda/sde (the
// scenario's, possibly haircut, figures), annualDebtService/
// totalDebtFinancing (already resolved for this view), capex, and
// buyerComp.
func computeCoverage(ebitda, sde, annualDebtService, totalDebtFinancing Value, capex CapexAssumption, buyerComp BuyerCompensationAssumption) CoverageResult {
	var cr CoverageResult

	cr.EarningsBaseSource, cr.CoverageNumerator = resolveEarningsBase(ebitda, sde)
	cr.AnnualDebtService = annualDebtService

	if cr.CoverageNumerator.Available && annualDebtService.Available && annualDebtService.Amount != 0 {
		cr.DSCR = AvailableValue(cr.CoverageNumerator.Amount / annualDebtService.Amount)
	}

	if cr.CoverageNumerator.Available && annualDebtService.Available {
		postDebt := cr.CoverageNumerator.Amount - annualDebtService.Amount
		if capex.AnnualAmount.Available {
			postDebt -= capex.AnnualAmount.Amount
		}
		if buyerComp.AnnualAmount.Available {
			postDebt -= buyerComp.AnnualAmount.Amount
		}
		cr.PostDebtCashFlow = AvailableValue(postDebt)
	}

	if totalDebtFinancing.Available && cr.CoverageNumerator.Available && cr.CoverageNumerator.Amount > 0 {
		cr.Leverage = AvailableValue(totalDebtFinancing.Amount / cr.CoverageNumerator.Amount)
	}

	return cr
}

// computeReturns derives ReturnMetrics from cashContribution and
// postDebtCashFlow.
func computeReturns(cashContribution, postDebtCashFlow Value) ReturnMetrics {
	r := ReturnMetrics{CashContribution: cashContribution}

	if cashContribution.Available && postDebtCashFlow.Available && cashContribution.Amount > 0 {
		r.CashOnCashReturn = AvailableValue(postDebtCashFlow.Amount / cashContribution.Amount)
	}
	if cashContribution.Available && postDebtCashFlow.Available && postDebtCashFlow.Amount > 0 {
		r.PaybackPeriodYears = AvailableValue(cashContribution.Amount / postDebtCashFlow.Amount)
	}

	return r
}
