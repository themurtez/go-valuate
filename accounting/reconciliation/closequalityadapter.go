package reconciliation

import "github.com/themurtez/go-valuate/accounting/closequality"

// CloseQualityReconciliationStatus converts a Result into a
// closequality.ReconciliationStatus, preserving status/as-of-date/
// difference/unresolved-count/source-ref — task section 58: "Create a
// small adapter from reconciliation result to
// accounting/closequality.ReconciliationStatus... Preserve: status, as
// of date, difference, unresolved count, source ref."
//
// UnresolvedCount is the total count of material unmatched book +
// external items plus outstanding ReconcilingItems, matching what
// closequality.ReconciliationStatus.UnresolvedCount is documented to
// mean (an "unresolved items remain" signal feeding
// FindingReconciliationItemsOutstanding).
func CloseQualityReconciliationStatus(result Result) closequality.ReconciliationStatus {
	status := closeQualityStatusFor(result.Status)

	var diff closequality.Value
	var bookBal closequality.Value
	var reconciledBal closequality.Value
	if result.Equation.Available {
		diff = closequality.AvailableValue(result.Equation.Difference)
		bookBal = closequality.AvailableValue(result.Equation.AdjustedBookBalance)
		reconciledBal = closequality.AvailableValue(result.Equation.AdjustedExternalBalance)
	}

	unresolved := 0
	for _, f := range result.Findings {
		switch f.Code {
		case FindingMaterialUnmatchedBookItem, FindingMaterialUnmatchedExternalItem, FindingAmbiguousMatch:
			unresolved++
		}
	}
	unresolved += len(result.ReconcilingItems)

	return closequality.ReconciliationStatus{
		AccountID:         result.AccountID,
		Status:            status,
		AsOfDate:          result.AsOfDate,
		BookBalance:       bookBal,
		ReconciledBalance: reconciledBal,
		Difference:        diff,
		UnresolvedCount:   unresolved,
		SourceRef:         result.ExternalAccountID,
	}
}

// closeQualityStatusFor maps this package's Status to
// closequality.ReconciliationState — task section 59's integration
// requires a critical account reconciled here to NOT block closequality,
// and unreconciled here to correctly produce NOT_READY under a critical-
// account policy; this mapping is what makes both true, by preserving
// exactly the same reconciled/unreconciled/unavailable distinction both
// packages already separately define.
func closeQualityStatusFor(s Status) closequality.ReconciliationState {
	switch s {
	case StatusReconciled:
		return closequality.ReconciliationReconciled
	case StatusReconciledWithItems:
		return closequality.ReconciliationReconciledWithItems
	case StatusUnreconciled:
		return closequality.ReconciliationUnreconciled
	case StatusIncomplete, StatusInvalid:
		return closequality.ReconciliationUnavailable
	default:
		return closequality.ReconciliationUnavailable
	}
}
