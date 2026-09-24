package closechecklist_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

func TestAdapters_CloseQualityGateFacts_MapsStatuses(t *testing.T) {
	cases := []struct {
		status closequality.Status
		want   closechecklist.GateStatus
	}{
		{closequality.StatusReady, closechecklist.GatePass},
		{closequality.StatusReadyWithWarnings, closechecklist.GateWarning},
		{closequality.StatusNotReady, closechecklist.GateFail},
		{closequality.StatusUnassessed, closechecklist.GateUnavailable},
	}
	for _, c := range cases {
		facts := closechecklist.CloseQualityGateFacts(closequality.Result{Status: c.status})
		if len(facts) == 0 || facts[0].GateCode != "closequality.overall" {
			t.Fatalf("expected first fact to be closequality.overall, got %+v", facts)
		}
		if facts[0].Status != c.want {
			t.Errorf("status %s -> gate %s, want %s", c.status, facts[0].Status, c.want)
		}
	}
}

func TestAdapters_CloseQualityGateFacts_DimensionsMapped(t *testing.T) {
	result := closequality.Result{
		Status: closequality.StatusReady,
		Dimensions: []closequality.DimensionResult{
			{Dimension: closequality.DimensionLedgerIntegrity, Status: closequality.DimensionPass},
			{Dimension: closequality.DimensionARControl, Status: closequality.DimensionWarning},
			{Dimension: closequality.DimensionAPControl, Status: closequality.DimensionBlocking},
			{Dimension: closequality.DimensionJournalReview, Status: closequality.DimensionUnassessed},
			{Dimension: closequality.DimensionFinancialStatementIntegrity, Status: closequality.DimensionPass},
			// Not in the mapped set — should be skipped.
			{Dimension: closequality.DimensionDataCompleteness, Status: closequality.DimensionPass},
		},
	}
	facts := closechecklist.CloseQualityGateFacts(result)
	byCode := map[string]closechecklist.GateFact{}
	for _, f := range facts {
		byCode[f.GateCode] = f
	}
	want := map[string]closechecklist.GateStatus{
		"closequality.ledger_integrity":    closechecklist.GatePass,
		"closequality.ar_control":          closechecklist.GateWarning,
		"closequality.ap_control":          closechecklist.GateFail,
		"closequality.journal_review":      closechecklist.GateUnavailable,
		"closequality.statement_integrity": closechecklist.GatePass,
	}
	for code, status := range want {
		got, ok := byCode[code]
		if !ok {
			t.Errorf("expected gate %s to be present, got %+v", code, facts)
			continue
		}
		if got.Status != status {
			t.Errorf("gate %s status = %s, want %s", code, got.Status, status)
		}
	}
	if _, ok := byCode["closequality.data_completeness"]; ok {
		t.Errorf("did not expect a gate for an unmapped dimension")
	}
}

func TestAdapters_ReconciliationGateFacts(t *testing.T) {
	cases := []struct {
		status reconciliation.Status
		want   closechecklist.GateStatus
	}{
		{reconciliation.StatusReconciled, closechecklist.GatePass},
		{reconciliation.StatusReconciledWithItems, closechecklist.GateWarning},
		{reconciliation.StatusUnreconciled, closechecklist.GateFail},
		{reconciliation.StatusIncomplete, closechecklist.GateUnavailable},
		{reconciliation.StatusInvalid, closechecklist.GateUnavailable},
	}
	for _, c := range cases {
		fact := closechecklist.ReconciliationGateFacts("reconciliation.cash_main", reconciliation.Result{Status: c.status, AccountID: "1000", Type: reconciliation.TypeBank})
		if fact.Status != c.want {
			t.Errorf("status %s -> gate %s, want %s", c.status, fact.Status, c.want)
		}
		if fact.SourceRef != "1000" {
			t.Errorf("expected SourceRef to carry AccountID, got %q", fact.SourceRef)
		}
	}
}

func TestAdapters_JournalDiagnosticsGateFact(t *testing.T) {
	cases := []struct {
		findings []journaldiagnostics.Finding
		want     closechecklist.GateStatus
	}{
		{nil, closechecklist.GatePass},
		{[]journaldiagnostics.Finding{{Severity: journaldiagnostics.SeverityInfo}}, closechecklist.GatePass},
		{[]journaldiagnostics.Finding{{Severity: journaldiagnostics.SeverityWarning}}, closechecklist.GateWarning},
		{[]journaldiagnostics.Finding{{Severity: journaldiagnostics.SeverityHigh}}, closechecklist.GateFail},
		{[]journaldiagnostics.Finding{{Severity: journaldiagnostics.SeverityWarning}, {Severity: journaldiagnostics.SeverityHigh}}, closechecklist.GateFail},
	}
	for _, c := range cases {
		fact := closechecklist.JournalDiagnosticsGateFact("closequality.journal_review", journaldiagnostics.Result{Findings: c.findings})
		if fact.Status != c.want {
			t.Errorf("findings %+v -> gate %s, want %s", c.findings, fact.Status, c.want)
		}
	}
}

func TestAdapters_ARAndAPGateFact(t *testing.T) {
	if got := closechecklist.ARGateFact("g", false, false).Status; got != closechecklist.GateUnavailable {
		t.Errorf("AR unavailable -> %s, want UNAVAILABLE", got)
	}
	if got := closechecklist.ARGateFact("g", true, true).Status; got != closechecklist.GateFail {
		t.Errorf("AR available+error -> %s, want FAIL", got)
	}
	if got := closechecklist.ARGateFact("g", true, false).Status; got != closechecklist.GatePass {
		t.Errorf("AR available+clean -> %s, want PASS", got)
	}
	if got := closechecklist.APGateFact("g", false, false).Status; got != closechecklist.GateUnavailable {
		t.Errorf("AP unavailable -> %s, want UNAVAILABLE", got)
	}
	if got := closechecklist.APGateFact("g", true, true).Status; got != closechecklist.GateFail {
		t.Errorf("AP available+error -> %s, want FAIL", got)
	}
	if got := closechecklist.APGateFact("g", true, false).Status; got != closechecklist.GatePass {
		t.Errorf("AP available+clean -> %s, want PASS", got)
	}
}

func TestAdapters_StatementsGateFact(t *testing.T) {
	if got := closechecklist.StatementsGateFact("g", false).Status; got != closechecklist.GateFail {
		t.Errorf("unavailable -> %s, want FAIL", got)
	}
	if got := closechecklist.StatementsGateFact("g", true).Status; got != closechecklist.GatePass {
		t.Errorf("available -> %s, want PASS", got)
	}
}
