package dealstructure

// computeSourcesAndUses derives SourcesAndUses from in and the
// already-resolved totalDebtFinancing/sellerNoteAmount/totalEarnout.
func computeSourcesAndUses(in Input, totalDebtFinancing, sellerNoteAmount, totalEarnout Value) SourcesAndUses {
	su := SourcesAndUses{
		TotalDebtFinancing: totalDebtFinancing,
		SellerNoteAmount:   sellerNoteAmount,
		TotalEarnoutAmount: totalEarnout,
		BuyerEquity:        in.BuyerEquity,
	}

	nonEquitySources := totalDebtFinancing.Amount + sellerNoteAmount.Amount + totalEarnout.Amount
	sourcesTotal := nonEquitySources
	if in.BuyerEquity.Available {
		sourcesTotal += in.BuyerEquity.Amount
	}
	su.TotalSources = AvailableValue(sourcesTotal)

	if !in.PurchasePrice.Available {
		return su
	}

	uses := in.PurchasePrice.Amount
	uses += in.Fees.total()
	if in.WorkingCapital.Amount.Available {
		uses += in.WorkingCapital.Amount.Amount
	}
	if in.ClosingAdjustments.AssumedDebt.Available {
		uses += in.ClosingAdjustments.AssumedDebt.Amount
	}
	if in.ClosingAdjustments.CashAcquired.Available {
		uses -= in.ClosingAdjustments.CashAcquired.Amount
	}
	su.TotalUses = AvailableValue(uses)

	su.RequiredEquity = AvailableValue(uses - nonEquitySources)
	su.FundingGapOrSurplus = AvailableValue(sourcesTotal - uses)

	return su
}

// computeFinancingPercentages derives FinancingPercentages from an
// already-computed SourcesAndUses. Every field is available only when
// TotalSources is available and strictly positive (a zero or unknown
// total sources denominator makes a percentage undefined).
func computeFinancingPercentages(su SourcesAndUses) FinancingPercentages {
	var fp FinancingPercentages
	if !su.TotalSources.Available || su.TotalSources.Amount <= 0 {
		return fp
	}
	total := su.TotalSources.Amount

	fp.DebtPercent = AvailableValue(su.TotalDebtFinancing.Amount / total)
	fp.SellerNotePercent = AvailableValue(su.SellerNoteAmount.Amount / total)
	fp.EarnoutPercent = AvailableValue(su.TotalEarnoutAmount.Amount / total)
	if su.BuyerEquity.Available {
		fp.EquityPercent = AvailableValue(su.BuyerEquity.Amount / total)
	}

	return fp
}
