package reconciliation

import "github.com/themurtez/go-valuate/accounting/ap"

// APSubledgerBalance mirrors ARSubledgerBalance for ap.Result — task
// section 35: "Mirror AR for AP." Reads
// result.PortfolioSummary.TotalOpenPayables as-is; never recomputes
// aging.
func APSubledgerBalance(result ap.Result) (float64, bool) {
	if !result.Available {
		return 0, false
	}
	return result.PortfolioSummary.TotalOpenPayables, true
}

// APSubledgerBookBalance mirrors ARSubledgerBookBalance for AP.
func APSubledgerBookBalance(result ap.Result, asOfDate string) BalanceInput {
	bal, ok := APSubledgerBalance(result)
	if !ok {
		return BalanceInput{}
	}
	return BalanceInput{EndingBalance: &bal, EndingBalanceDate: asOfDate}
}

// APControlExternalBalance mirrors ARControlExternalBalance for AP.
func APControlExternalBalance(result ap.Result, asOfDate string) BalanceInput {
	if !result.Available || !result.ControlAccountReconciliation.Available {
		return BalanceInput{}
	}
	bal := result.ControlAccountReconciliation.ControlAccountBalance
	return BalanceInput{EndingBalance: &bal, EndingBalanceDate: asOfDate}
}

// BookItemsFromAPPayables mirrors BookItemsFromARReceivables for AP
// (task section 35). A bill's positive OpenAmount (owed to the supplier)
// is preserved as a positive signed Amount/DirectionInflow — this
// adapter reports the literal figure ap already computed, matching the
// convention BookItemsFromARReceivables uses for AR, rather than
// reinterpreting it as a cash outflow (that reinterpretation is a
// caller/Orientation concern, not this adapter's — see
// MatchingPolicy.Orientation).
func BookItemsFromAPPayables(payables []ap.Payable) []BookItem {
	items := make([]BookItem, 0, len(payables))
	for _, p := range payables {
		items = append(items, BookItem{
			ItemID:      p.ID,
			Date:        p.BillDate.Format(dateLayout),
			Amount:      abs(p.OpenAmount),
			Direction:   directionFor(p.OpenAmount),
			Reference:   p.BillNumber,
			Description: p.SupplierName,
			Currency:    p.Currency,
			SourceType:  "ap",
			SourceID:    p.ID,
			SourceRef:   SourceRef{System: "ap", ID: p.ID},
		})
	}
	return items
}
