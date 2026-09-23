package debt

// computeCapacity derives MaximumCapacity from in.Policy against base's
// already-computed coverage numerator, using pricing terms resolved from
// in.ProposedLoans (preferred) or in.ExistingDebt.
func computeCapacity(in Input, base CoverageResult, totalDebtBalance Value) MaximumCapacity {
	var capacity MaximumCapacity

	pricing, ok := resolvePricingTerms(in)
	capacity.PricingTerms = pricing
	capacity.HasPricingTerms = ok

	if ok && in.Policy.MinimumDSCR > 0 && base.CoverageNumerator.Available {
		if maxDebt, feasible := maxDebtUnderDSCR(pricing, base.CoverageNumerator.Amount, in.Policy.MinimumDSCR); feasible {
			capacity.MaxDebtUnderDSCR = AvailableValue(maxDebt)
		}
	}

	capacity.MaxDebtUnderLeverage = maxDebtUnderLeverage(in)

	capacity.CombinedMaximumDebt, capacity.LimitingConstraint = combineCapacity(capacity.MaxDebtUnderDSCR, capacity.MaxDebtUnderLeverage)

	if capacity.CombinedMaximumDebt.Available && totalDebtBalance.Available {
		capacity.Headroom = AvailableValue(capacity.CombinedMaximumDebt.Amount - totalDebtBalance.Amount)
	}

	return capacity
}

// resolvePricingTerms returns the LoanTerms to use for translating a
// maximum annual debt service into a maximum principal: the first entry
// of in.ProposedLoans if non-empty, otherwise the first entry of
// in.ExistingDebt, otherwise ok == false. The chosen LoanTerms must itself
// be structurally valid (see validateLoanTerms) or ok is false.
func resolvePricingTerms(in Input) (LoanTerms, bool) {
	candidates := in.ProposedLoans
	if len(candidates) == 0 {
		candidates = in.ExistingDebt
	}
	if len(candidates) == 0 {
		return LoanTerms{}, false
	}
	t := candidates[0]
	if _, usable := validateLoanTerms(t, ""); !usable {
		return LoanTerms{}, false
	}
	return t, true
}

// maxDebtUnderDSCR solves for the principal P such that a loan priced at
// pricing's rate/amortization/frequency/interest-only structure but scaled
// to principal P produces an annual debt service exactly equal to
// coverageNumerator / minDSCR. Since AmortizationSchedule's
// FirstYearAnnualDebtService is linear in Principal (every term in the
// amortization formula scales proportionally with principal), this is a
// direct ratio: scale pricing's own FirstYearAnnualDebtService by
// (targetDebtService / pricing's own debt service).
//
// feasible is false when pricing.Principal is 0 (nothing to scale from) or
// minDSCR <= 0 (a non-positive DSCR requirement is not meaningful).
func maxDebtUnderDSCR(pricing LoanTerms, coverageNumerator, minDSCR float64) (maxDebt float64, feasible bool) {
	if minDSCR <= 0 || pricing.Principal <= 0 {
		return 0, false
	}
	targetDebtService := coverageNumerator / minDSCR
	if targetDebtService <= 0 {
		return 0, true // no earnings support any debt service at this DSCR
	}

	referenceSchedule := Amortize(referenceLoan(pricing))
	if referenceSchedule.FirstYearAnnualDebtService <= 0 {
		return 0, false
	}

	scale := targetDebtService / referenceSchedule.FirstYearAnnualDebtService
	return referenceLoan(pricing).Principal * scale, true
}

// referenceLoan returns t with Principal normalized to 1.0, so the
// resulting AmortizationSchedule's FirstYearAnnualDebtService is the
// per-dollar-of-principal debt service pricing implies — used by
// maxDebtUnderDSCR to scale to an arbitrary target debt service without
// dividing by the caller's original (possibly zero-relevant-to-this-solve)
// Principal.
func referenceLoan(t LoanTerms) LoanTerms {
	t.Principal = 1.0
	return t
}

// maxDebtUnderLeverage returns EBITDA x the tightest applicable leverage
// cap in Input.Policy. When both MaximumDebtToEBITDA (gross) and
// MaximumNetDebtToEBITDA (net) are set, the net cap is converted to an
// equivalent gross-debt figure by adding back CashAndEquivalents (net debt
// = gross debt - cash, so gross debt = net debt cap x EBITDA + cash), and
// the smaller of the two gross-equivalent figures is used — the more
// restrictive of the two policy limits.
func maxDebtUnderLeverage(in Input) Value {
	if !in.EBITDA.Available || in.EBITDA.Amount <= 0 {
		return Unavailable()
	}

	var candidates []float64

	if in.Policy.MaximumDebtToEBITDA > 0 {
		candidates = append(candidates, in.Policy.MaximumDebtToEBITDA*in.EBITDA.Amount)
	}
	if in.Policy.MaximumNetDebtToEBITDA > 0 {
		netCapGross := in.Policy.MaximumNetDebtToEBITDA * in.EBITDA.Amount
		if in.CashAndEquivalents.Available {
			netCapGross += in.CashAndEquivalents.Amount
		}
		candidates = append(candidates, netCapGross)
	}

	if len(candidates) == 0 {
		return Unavailable()
	}

	min := candidates[0]
	for _, c := range candidates[1:] {
		if c < min {
			min = c
		}
	}
	return AvailableValue(min)
}

// combineCapacity picks the more restrictive (smaller) of dscrMax/leverageMax
// when both are available, falls back to whichever is available when only
// one is, and reports LimitNone with an unavailable Value when neither is.
func combineCapacity(dscrMax, leverageMax Value) (Value, LimitingConstraintCode) {
	switch {
	case dscrMax.Available && leverageMax.Available:
		if dscrMax.Amount <= leverageMax.Amount {
			return dscrMax, LimitDSCR
		}
		return leverageMax, LimitLeverage
	case dscrMax.Available:
		return dscrMax, LimitDSCR
	case leverageMax.Available:
		return leverageMax, LimitLeverage
	default:
		return Unavailable(), LimitNone
	}
}
