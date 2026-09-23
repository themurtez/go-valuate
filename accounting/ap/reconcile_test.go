package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	apfixtures "github.com/themurtez/go-valuate/accounting/ap/fixtures"
)

func TestReconcile_AgingAlwaysBalances(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	for name, payables := range map[string][]ap.Payable{
		"healthy":     apfixtures.HealthyPayables(),
		"aging-heavy": apfixtures.AgingHeavyPayables(),
		"disputed":    apfixtures.DisputedBills(),
		"partial":     apfixtures.PartialPayments(),
		"credits":     apfixtures.SupplierVendorCredits(),
	} {
		result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
		if !result.Available {
			t.Fatalf("%s: expected Available=true, issues: %+v", name, result.Issues)
		}
		if !result.AgingReconciliation.Balanced {
			t.Errorf("%s: expected AgingReconciliation.Balanced=true, got %+v", name, result.AgingReconciliation)
		}
		if result.AgingReconciliation.TotalOpenPayables != result.PortfolioSummary.TotalOpenPayables {
			t.Errorf("%s: AgingReconciliation.TotalOpenPayables mismatch", name)
		}
	}
}

func TestReconcile_GLControlAccountMatch(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	payables := apfixtures.HealthyPayables()
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	matching := result.PortfolioSummary.TotalOpenPayables

	withControl := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, ControlAccountBalance: &matching})
	if !withControl.ControlAccountReconciliation.Available {
		t.Fatalf("expected ControlAccountReconciliation.Available=true")
	}
	if !withControl.ControlAccountReconciliation.Reconciled {
		t.Errorf("expected Reconciled=true for matching balances, got %+v", withControl.ControlAccountReconciliation)
	}
	if hasIssueCode(withControl.Issues, ap.IssueControlAccountMismatch) {
		t.Errorf("did not expect IssueControlAccountMismatch for matching balances")
	}
}

func TestReconcile_GLControlAccountMismatch(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	payables, controlBalance := apfixtures.GLSubledgerMismatch()
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, ControlAccountBalance: &controlBalance})
	if !result.ControlAccountReconciliation.Available {
		t.Fatalf("expected ControlAccountReconciliation.Available=true")
	}
	if result.ControlAccountReconciliation.Reconciled {
		t.Errorf("expected Reconciled=false for deliberately mismatched balances")
	}
	if !hasIssueCode(result.Issues, ap.IssueControlAccountMismatch) {
		t.Errorf("expected IssueControlAccountMismatch, got %+v", result.Issues)
	}
	foundFlag := false
	for _, f := range result.Flags {
		if f.Code == ap.FlagControlAccountMismatch {
			foundFlag = true
		}
	}
	if !foundFlag {
		t.Errorf("expected FlagControlAccountMismatch, flags: %+v", result.Flags)
	}
}

func TestReconcile_NeverAdjustsEitherSide(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	payables, controlBalance := apfixtures.GLSubledgerMismatch()
	subledgerBefore := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf}).PortfolioSummary.TotalOpenPayables

	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, ControlAccountBalance: &controlBalance})
	if result.PortfolioSummary.TotalOpenPayables != subledgerBefore {
		t.Errorf("subledger total changed after supplying a mismatched control balance: %v != %v", result.PortfolioSummary.TotalOpenPayables, subledgerBefore)
	}
	if result.ControlAccountReconciliation.ControlAccountBalance != controlBalance {
		t.Errorf("control account balance was altered: %v != %v", result.ControlAccountReconciliation.ControlAccountBalance, controlBalance)
	}
}

func TestReconcile_ControlAccountNotSuppliedIsUnavailable(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	result := ap.Calculate(ap.Input{Payables: apfixtures.HealthyPayables()}, ap.Options{AsOfDate: asOf})
	if result.ControlAccountReconciliation.Available {
		t.Errorf("expected ControlAccountReconciliation.Available=false when not supplied")
	}
}
