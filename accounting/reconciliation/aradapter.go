package reconciliation

import "github.com/themurtez/go-valuate/accounting/ar"

// ARSubledgerBalance reports ar.Result's own subledger ending balance for
// use as the book side (or, if reconciling AR subledger against the GL
// control account, the external/control comparison figure — see
// ARSubledgerBookBalance and ARControlExternalBalance) of an AR-control
// reconciliation. This adapter never recomputes aging — it reads
// ar.Result.PortfolioSummary.TotalOpenReceivables as-is (task section 34:
// "Do not recompute aging").
func ARSubledgerBalance(result ar.Result) (float64, bool) {
	if !result.Available {
		return 0, false
	}
	return result.PortfolioSummary.TotalOpenReceivables, true
}

// ARSubledgerBookBalance builds a BalanceInput for the book side of an
// "AR subledger vs GL control" reconciliation (task section 34), using
// result's own already-computed TotalOpenReceivables as EndingBalance.
// asOfDate is the caller's AsOfDate (echoed as the balance's date for
// provenance) — this adapter does not read a date off ar.Result itself
// since ar.Result.AsOfDate reflects the aging calculation's own AsOfDate,
// which the caller is expected to have already aligned with the
// reconciliation's own Input.AsOfDate.
func ARSubledgerBookBalance(result ar.Result, asOfDate string) BalanceInput {
	bal, ok := ARSubledgerBalance(result)
	if !ok {
		return BalanceInput{}
	}
	return BalanceInput{EndingBalance: &bal, EndingBalanceDate: asOfDate}
}

// ARControlExternalBalance builds a BalanceInput for the external
// (GL-control) side of an "AR subledger vs GL control" reconciliation,
// from result.ControlAccountReconciliation.ControlAccountBalance — this
// is the same GL control balance ar.Result was already given via its own
// Options.ControlAccountBalance, republished here so a caller does not
// need to pass that figure twice.
func ARControlExternalBalance(result ar.Result, asOfDate string) BalanceInput {
	if !result.Available || !result.ControlAccountReconciliation.Available {
		return BalanceInput{}
	}
	bal := result.ControlAccountReconciliation.ControlAccountBalance
	return BalanceInput{EndingBalance: &bal, EndingBalanceDate: asOfDate}
}

// BookItemsFromARReceivables converts result's open receivables (task
// section 34's "optional open-item detail") into BookItems, one per
// receivable, using OpenAmount as Amount (always DirectionInflow, since
// an open receivable is an asset-increasing balance from the book's
// perspective — a credit memo's negative OpenAmount is preserved via a
// negative Amount rather than flipped to DirectionOutflow, since this
// adapter reports the literal signed figure ar already computed rather
// than reinterpreting it). This is a convenience for transaction-level
// AR-control reconciliation; result itself does not expose the
// underlying []ar.Receivable, so this function instead takes it
// directly.
func BookItemsFromARReceivables(receivables []ar.Receivable) []BookItem {
	items := make([]BookItem, 0, len(receivables))
	for _, r := range receivables {
		items = append(items, BookItem{
			ItemID:      r.ID,
			Date:        r.InvoiceDate.Format(dateLayout),
			Amount:      abs(r.OpenAmount),
			Direction:   directionFor(r.OpenAmount),
			Reference:   r.InvoiceNumber,
			Description: r.CustomerName,
			Currency:    r.Currency,
			SourceType:  "ar",
			SourceID:    r.ID,
			SourceRef:   SourceRef{System: "ar", ID: r.ID},
		})
	}
	return items
}

func directionFor(signedAmount float64) Direction {
	if signedAmount < 0 {
		return DirectionOutflow
	}
	return DirectionInflow
}
