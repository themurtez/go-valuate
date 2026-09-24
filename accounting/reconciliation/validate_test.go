package reconciliation

import (
	"math"
	"testing"
)

func TestValidate_DuplicateBookItemKeepsFirstOccurrence(t *testing.T) {
	items := []BookItem{
		{ItemID: "B1", Amount: 100, Direction: DirectionOutflow, Description: "first"},
		{ItemID: "B1", Amount: 200, Direction: DirectionOutflow, Description: "second"},
	}
	out, issues := dedupBookItems(items, "")
	if len(out) != 1 {
		t.Fatalf("expected 1 item after dedup, got %d", len(out))
	}
	if out[0].Description != "first" {
		t.Fatalf("expected first occurrence kept, got %+v", out[0])
	}
	found := false
	for _, i := range issues {
		if i.Code == IssueDuplicateBookItem && i.Severity == IssueSeverityWarning {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected duplicate warning issue, got %+v", issues)
	}
}

func TestValidate_EmptyItemIDExcludedAsError(t *testing.T) {
	items := []BookItem{{ItemID: "", Amount: 100, Direction: DirectionOutflow}}
	out, issues := dedupBookItems(items, "")
	if len(out) != 0 {
		t.Fatalf("expected empty-ID item excluded, got %+v", out)
	}
	if len(issues) != 1 || issues[0].Severity != IssueSeverityError {
		t.Fatalf("expected 1 error issue, got %+v", issues)
	}
}

func TestValidate_NonFiniteAmountExcluded(t *testing.T) {
	items := []BookItem{{ItemID: "B1", Amount: math.Inf(1), Direction: DirectionOutflow}}
	out, issues := dedupBookItems(items, "")
	if len(out) != 0 {
		t.Fatalf("expected non-finite amount excluded, got %+v", out)
	}
	if len(issues) != 1 || issues[0].Code != IssueNonFiniteAmount {
		t.Fatalf("expected IssueNonFiniteAmount, got %+v", issues)
	}
}

func TestValidate_UnrecognizedDirectionExcluded(t *testing.T) {
	items := []BookItem{{ItemID: "B1", Amount: 100, Direction: "SIDEWAYS"}}
	out, issues := dedupBookItems(items, "")
	if len(out) != 0 {
		t.Fatalf("expected unrecognized direction excluded, got %+v", out)
	}
	if len(issues) != 1 || issues[0].Code != IssueInvalidDirection {
		t.Fatalf("expected IssueInvalidDirection, got %+v", issues)
	}
}

func TestValidate_NegativeAmountWarnedNotExcluded(t *testing.T) {
	items := []BookItem{{ItemID: "B1", Amount: -50, Direction: DirectionOutflow}}
	out, issues := dedupBookItems(items, "")
	if len(out) != 1 {
		t.Fatalf("expected negative-amount item still included (warned, not excluded), got %+v", out)
	}
	if len(issues) != 1 || issues[0].Code != IssueInvalidAmount || issues[0].Severity != IssueSeverityWarning {
		t.Fatalf("expected IssueInvalidAmount warning, got %+v", issues)
	}
}

func TestValidate_ExternalItemsSameRules(t *testing.T) {
	items := []ExternalItem{
		{ItemID: "E1", Amount: 100, Direction: DirectionOutflow},
		{ItemID: "E1", Amount: 200, Direction: DirectionOutflow},
	}
	out, issues := dedupExternalItems(items, "")
	if len(out) != 1 {
		t.Fatalf("expected 1 item after dedup, got %d", len(out))
	}
	if len(issues) != 1 || issues[0].Code != IssueDuplicateExternalItem {
		t.Fatalf("expected IssueDuplicateExternalItem, got %+v", issues)
	}
}

func TestValidate_ReconcilingItemUnrecognizedTypeExcluded(t *testing.T) {
	items := []ReconcilingItem{{ItemID: "R1", Type: "BOGUS", Side: ReconcilingSideBook, Amount: 10}}
	out, issues := dedupReconcilingItems(items)
	if len(out) != 0 {
		t.Fatalf("expected unrecognized type excluded, got %+v", out)
	}
	if len(issues) != 1 || issues[0].Code != IssueInvalidReconcilingItem {
		t.Fatalf("expected IssueInvalidReconcilingItem, got %+v", issues)
	}
}

func TestValidate_ReportingCurrencyMismatchWarned(t *testing.T) {
	items := []BookItem{{ItemID: "B1", Amount: 100, Direction: DirectionOutflow, Currency: "EUR"}}
	out, issues := dedupBookItems(items, "USD")
	if len(out) != 1 {
		t.Fatalf("expected item still included despite currency mismatch, got %+v", out)
	}
	found := false
	for _, i := range issues {
		if i.Code == IssueMixedCurrency {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueMixedCurrency, got %+v", issues)
	}
}
