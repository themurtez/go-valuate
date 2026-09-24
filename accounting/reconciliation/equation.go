package reconciliation

// ReconcilingEffects summarizes the signed sum of ReconcilingItems on one
// side, split by whether the item's Type is a "book-adds" (increases the
// book-adjusted balance toward the external side) or general effect —
// this package does not classify Type into a fixed sign convention (a
// caller's Amount is already signed correctly for direct addition — see
// ReconcilingItem.Amount's doc comment), so ReconcilingEffects is purely
// a factual sum plus a count, never a re-derivation of sign from Type.
type ReconcilingEffects struct {
	Count int     `json:"count"`
	Total float64 `json:"total"`
}

// EquationResult is the applied reconciliation equation (task section
// 26):
//
//	AdjustedBookBalance     = BookEndingBalance + BookSideReconcilingEffects
//	AdjustedExternalBalance = ExternalEndingBalance + ExternalSideReconcilingEffects
//	Difference              = AdjustedBookBalance - AdjustedExternalBalance
//
// Available only when both BookEndingBalance and ExternalEndingBalance
// are available (see Balance.EndingAvailable) — this package never
// computes a partial or assumed Difference from only one side.
type EquationResult struct {
	Available bool `json:"available"`

	BookEndingBalance     float64 `json:"book_ending_balance,omitempty"`
	ExternalEndingBalance float64 `json:"external_ending_balance,omitempty"`

	BookReconcilingEffects     ReconcilingEffects `json:"book_reconciling_effects"`
	ExternalReconcilingEffects ReconcilingEffects `json:"external_reconciling_effects"`

	AdjustedBookBalance     float64 `json:"adjusted_book_balance,omitempty"`
	AdjustedExternalBalance float64 `json:"adjusted_external_balance,omitempty"`

	Difference float64 `json:"difference,omitempty"`
	// WithinTolerance is true when abs(Difference) <= MatchingPolicy's
	// resolved balance tolerance.
	WithinTolerance bool `json:"within_tolerance"`
}

// applyReconciliationEquation computes EquationResult from bookBalance/
// externalBalance and the (already-validated, already-deduplicated)
// reconcilingItems, per the fixed formula documented on EquationResult —
// task section 26: "Use it consistently."
func applyReconciliationEquation(bookBalance, externalBalance Balance, reconcilingItems []ReconcilingItem, tolerance float64) EquationResult {
	var r EquationResult

	r.BookReconcilingEffects = sumReconcilingEffects(reconcilingItems, ReconcilingSideBook)
	r.ExternalReconcilingEffects = sumReconcilingEffects(reconcilingItems, ReconcilingSideExternal)

	if !bookBalance.EndingAvailable || !externalBalance.EndingAvailable {
		return r
	}

	r.Available = true
	r.BookEndingBalance = bookBalance.EndingBalance
	r.ExternalEndingBalance = externalBalance.EndingBalance

	r.AdjustedBookBalance = r.BookEndingBalance + r.BookReconcilingEffects.Total
	r.AdjustedExternalBalance = r.ExternalEndingBalance + r.ExternalReconcilingEffects.Total
	r.Difference = r.AdjustedBookBalance - r.AdjustedExternalBalance
	r.WithinTolerance = abs(r.Difference) <= tolerance

	return r
}

func sumReconcilingEffects(items []ReconcilingItem, side ReconcilingSide) ReconcilingEffects {
	var e ReconcilingEffects
	for _, it := range items {
		if it.Side != side {
			continue
		}
		e.Count++
		e.Total += it.Amount
	}
	return e
}
