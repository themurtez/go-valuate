package closechecklist

import (
	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

// CloseQualityGateFacts converts a closequality.Result into a set of
// portable GateFacts — section 12. This package never recomputes
// closequality's own dimension logic; it only reads the already-computed
// Status/DimensionResult values.
//
// GateCode naming follows "closequality.<name>": one overall gate
// ("closequality.overall") plus one gate per assessed closequality
// Dimension, using the fixed mapping in closeQualityDimensionGateCodes.
// A dimension the caller never asked closequality to assess
// (DimensionUnassessed) is surfaced as GateUnavailable rather than
// silently omitted, so a task's GateRule referencing it always sees a
// GateFact (never a "missing gate" the caller has to separately reason
// about).
func CloseQualityGateFacts(result closequality.Result) []GateFact {
	facts := []GateFact{{
		GateCode:     "closequality.overall",
		Status:       closeQualityStatusToGate(result.Status),
		SourceModule: "closequality",
		SourceCode:   string(result.Status),
	}}

	for _, dim := range result.Dimensions {
		code, ok := closeQualityDimensionGateCodes[dim.Dimension]
		if !ok {
			continue
		}
		facts = append(facts, GateFact{
			GateCode:     code,
			Status:       closeQualityDimensionStatusToGate(dim.Status),
			SourceModule: "closequality",
			SourceCode:   string(dim.Dimension),
		})
	}
	return facts
}

// closeQualityDimensionGateCodes maps a closequality.Dimension to the
// stable GateCode this adapter emits for it. Only the dimensions the
// task spec calls out by example (section 12) are mapped; other
// dimensions are simply not surfaced as a separate gate (a caller who
// wants one can extend this mapping or gate on "closequality.overall").
var closeQualityDimensionGateCodes = map[closequality.Dimension]string{
	closequality.DimensionLedgerIntegrity:             "closequality.ledger_integrity",
	closequality.DimensionFinancialStatementIntegrity: "closequality.statement_integrity",
	closequality.DimensionARControl:                   "closequality.ar_control",
	closequality.DimensionAPControl:                   "closequality.ap_control",
	closequality.DimensionJournalReview:               "closequality.journal_review",
}

func closeQualityStatusToGate(s closequality.Status) GateStatus {
	switch s {
	case closequality.StatusReady:
		return GatePass
	case closequality.StatusReadyWithWarnings:
		return GateWarning
	case closequality.StatusNotReady:
		return GateFail
	case closequality.StatusUnassessed:
		return GateUnavailable
	default:
		return GateUnavailable
	}
}

func closeQualityDimensionStatusToGate(s closequality.DimensionStatus) GateStatus {
	switch s {
	case closequality.DimensionPass:
		return GatePass
	case closequality.DimensionWarning:
		return GateWarning
	case closequality.DimensionBlocking:
		return GateFail
	case closequality.DimensionUnassessed:
		return GateUnavailable
	default:
		return GateUnavailable
	}
}

// ReconciliationGateFacts converts one reconciliation.Result into a
// single portable GateFact — section 13. gateCode is caller-supplied
// (e.g. "reconciliation.cash_main") since this package never infers
// which account/gate identity a given reconciliation represents.
func ReconciliationGateFacts(gateCode string, result reconciliation.Result) GateFact {
	return GateFact{
		GateCode:     gateCode,
		Status:       reconciliationStatusToGate(result.Status),
		SourceModule: "reconciliation",
		SourceCode:   string(result.Type),
		SourceRef:    result.AccountID,
	}
}

func reconciliationStatusToGate(s reconciliation.Status) GateStatus {
	switch s {
	case reconciliation.StatusReconciled:
		return GatePass
	case reconciliation.StatusReconciledWithItems:
		return GateWarning
	case reconciliation.StatusUnreconciled:
		return GateFail
	case reconciliation.StatusIncomplete, reconciliation.StatusInvalid:
		return GateUnavailable
	default:
		return GateUnavailable
	}
}

// JournalDiagnosticsGateFact converts a journaldiagnostics.Result into a
// single portable GateFact — section 14's "optional sibling gates,"
// scoped to journaldiagnostics's own existing severity semantics (never
// a new calculation). FAIL means at least one HIGH-severity Finding;
// WARNING means at least one WARNING-severity Finding and no HIGH;
// PASS means no Findings of either severity.
func JournalDiagnosticsGateFact(gateCode string, result journaldiagnostics.Result) GateFact {
	status := GatePass
	for _, f := range result.Findings {
		switch f.Severity {
		case journaldiagnostics.SeverityHigh:
			status = GateFail
		case journaldiagnostics.SeverityWarning:
			if status != GateFail {
				status = GateWarning
			}
		}
	}
	return GateFact{
		GateCode:     gateCode,
		Status:       status,
		SourceModule: "journaldiagnostics",
		SourceCode:   result.Period,
	}
}

// ARGateFact converts an accounting/ar.Result's own Available/Issues
// into a portable GateFact — section 14. This adapter surfaces only
// ar's existing Available/error-Issue semantics; it never recomputes AR
// aging/control logic itself.
func ARGateFact(gateCode string, available bool, hasErrorIssue bool) GateFact {
	status := GateUnavailable
	switch {
	case !available:
		status = GateUnavailable
	case hasErrorIssue:
		status = GateFail
	default:
		status = GatePass
	}
	return GateFact{GateCode: gateCode, Status: status, SourceModule: "ar"}
}

// APGateFact mirrors ARGateFact for accounting/ap.
func APGateFact(gateCode string, available bool, hasErrorIssue bool) GateFact {
	status := GateUnavailable
	switch {
	case !available:
		status = GateUnavailable
	case hasErrorIssue:
		status = GateFail
	default:
		status = GatePass
	}
	return GateFact{GateCode: gateCode, Status: status, SourceModule: "ap"}
}

// StatementsGateFact converts an accounting/statements dataset
// availability into a portable GateFact — section 14.
func StatementsGateFact(gateCode string, available bool) GateFact {
	status := GateFail
	if available {
		status = GatePass
	}
	return GateFact{GateCode: gateCode, Status: status, SourceModule: "statements"}
}
