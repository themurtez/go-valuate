package ar_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/accounting/ar/fixtures"
)

func TestFixtures_HealthyPortfolio(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.HealthyPortfolio()}, ar.Options{AsOfDate: asOf})
	if !result.Available || ar.HasErrors(result.Issues) {
		t.Fatalf("expected clean result, issues: %+v", result.Issues)
	}
}

func TestFixtures_AgingHeavyPortfolio(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.AgingHeavyPortfolio()}, ar.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available, issues: %+v", result.Issues)
	}
	if result.PortfolioSummary.OverdueTotal <= 0 {
		t.Errorf("expected material overdue balance, got %v", result.PortfolioSummary.OverdueTotal)
	}
}

func TestFixtures_OneLargeOverdueCustomer_Flags(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.OneLargeOverdueCustomer()}, ar.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available, issues: %+v", result.Issues)
	}
	foundConcentration := false
	for _, f := range result.Flags {
		if f.Code == ar.FlagHighARConcentration {
			foundConcentration = true
		}
	}
	if !foundConcentration {
		t.Errorf("expected FlagHighARConcentration given one dominant customer, got flags: %+v", result.Flags)
	}
}

func TestFixtures_ManySmallCustomers_LowConcentration(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.ManySmallCustomers()}, ar.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available, issues: %+v", result.Issues)
	}
	for _, f := range result.Flags {
		if f.Code == ar.FlagHighARConcentration {
			t.Errorf("did not expect FlagHighARConcentration for a broad, even portfolio")
		}
	}
}

func TestFixtures_DisputedInvoices(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.DisputedInvoices()}, ar.Options{AsOfDate: asOf})
	if result.PortfolioSummary.DisputedAmount != 20000 {
		t.Errorf("DisputedAmount = %v, want 20000", result.PortfolioSummary.DisputedAmount)
	}
}

func TestFixtures_PartialPayments(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.PartialPayments()}, ar.Options{AsOfDate: asOf})
	if result.PortfolioSummary.TotalOpenReceivables != 4000 {
		t.Errorf("TotalOpenReceivables = %v, want 4000", result.PortfolioSummary.TotalOpenReceivables)
	}
}

func TestFixtures_CustomerCredits(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.CustomerCredits()}, ar.Options{AsOfDate: asOf})
	if result.PortfolioSummary.Credits.TotalCreditBalance != -1500 {
		t.Errorf("Credits.TotalCreditBalance = %v, want -1500", result.PortfolioSummary.Credits.TotalCreditBalance)
	}
}

func TestFixtures_MixedCurrency(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.MixedCurrency()}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueMixedCurrency) {
		t.Errorf("expected IssueMixedCurrency, got %+v", result.Issues)
	}
}

func TestFixtures_ZeroAR(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.ZeroAR()}, ar.Options{AsOfDate: asOf})
	if !result.Available {
		t.Fatalf("expected Available=true for empty portfolio")
	}
	if result.PortfolioSummary.TotalOpenReceivables != 0 {
		t.Errorf("expected zero AR")
	}
	if !result.AgingReconciliation.Balanced {
		t.Errorf("expected reconciliation to balance trivially for zero AR")
	}
}

func TestFixtures_FutureDatedInvalidInvoice(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.FutureDatedInvalidInvoice()}, ar.Options{AsOfDate: asOf})
	if !hasIssueCode(result.Issues, ar.IssueFutureInvoice) {
		t.Errorf("expected IssueFutureInvoice, got %+v", result.Issues)
	}
}

func TestFixtures_DeterioratingSnapshots(t *testing.T) {
	snapshots, current := fixtures.DeterioratingSnapshots()
	asOf := mustDate(t, "2025-08-15") // ~76 days past due from 2025-05-31 due date.
	result := ar.Calculate(ar.Input{Receivables: current, Snapshots: snapshots}, ar.Options{AsOfDate: asOf})
	if !result.Migration.Available {
		t.Fatalf("expected Migration.Available=true")
	}
	if result.Migration.DeteriorationAmount <= 0 {
		t.Errorf("expected deterioration, got %+v", result.Migration)
	}
}

func TestFixtures_ImprovingSnapshots(t *testing.T) {
	snapshots, current := fixtures.ImprovingSnapshots()
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: current, Snapshots: snapshots}, ar.Options{AsOfDate: asOf})
	if !result.Migration.Available {
		t.Fatalf("expected Migration.Available=true")
	}
	if result.Migration.CureAmount != 6000 {
		t.Errorf("CureAmount = %v, want 6000", result.Migration.CureAmount)
	}
}

func TestFixtures_GLSubledgerMismatch(t *testing.T) {
	receivables, control := fixtures.GLSubledgerMismatch()
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, ControlAccountBalance: &control})
	if result.ControlAccountReconciliation.Reconciled {
		t.Errorf("expected mismatch to NOT reconcile")
	}
}

func TestFixtures_SalesHistoryForDSO(t *testing.T) {
	asOf := mustDate(t, fixtures.AsOfDate)
	result := ar.Calculate(ar.Input{Receivables: fixtures.HealthyPortfolio(), SalesHistory: fixtures.SalesHistoryForDSO()}, ar.Options{AsOfDate: asOf})
	if !result.DSO.Available {
		t.Fatalf("expected DSO.Available=true")
	}
	if !result.DSOHistory.Available {
		t.Fatalf("expected DSOHistory.Available=true")
	}
}
