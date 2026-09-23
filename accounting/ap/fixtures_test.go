package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	apfixtures "github.com/themurtez/go-valuate/accounting/ap/fixtures"
)

// TestFixtures_AllCalculateWithoutErrorIssues sanity-checks that every
// fixture scenario produces a usable Result (no SeverityError issues,
// Available=true) except where a fixture's entire documented purpose is to
// exercise an error path.
func TestFixtures_AllCalculateWithoutErrorIssues(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	scenarios := map[string][]ap.Payable{
		"healthy":              apfixtures.HealthyPayables(),
		"aging-heavy":          apfixtures.AgingHeavyPayables(),
		"one-large-overdue":    apfixtures.OneLargeOverdueSupplier(),
		"many-small-suppliers": apfixtures.ManySmallSuppliers(),
		"disputed":             apfixtures.DisputedBills(),
		"partial-payments":     apfixtures.PartialPayments(),
		"vendor-credits":       apfixtures.SupplierVendorCredits(),
		"zero-ap":              apfixtures.ZeroAP(),
		"near-term-pressure":   apfixtures.NearTermPaymentPressure(),
	}
	for name, payables := range scenarios {
		result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
		if !result.Available {
			t.Errorf("%s: expected Available=true, issues: %+v", name, result.Issues)
			continue
		}
		if ap.HasErrors(result.Issues) {
			t.Errorf("%s: expected no SeverityError issues, got: %+v", name, result.Issues)
		}
	}
}

// TestFixtures_MixedCurrencyAndFutureBillProduceExpectedIssues verifies the
// two fixtures whose entire purpose is to exercise a warning/issue path
// actually do so.
func TestFixtures_MixedCurrencyAndFutureBillProduceExpectedIssues(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)

	mixed := ap.Calculate(ap.Input{Payables: apfixtures.MixedCurrency()}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(mixed.Issues, ap.IssueMixedCurrency) {
		t.Errorf("MixedCurrency fixture: expected IssueMixedCurrency, got %+v", mixed.Issues)
	}

	future := ap.Calculate(ap.Input{Payables: apfixtures.FutureDatedInvalidBill()}, ap.Options{AsOfDate: asOf})
	if !hasIssueCode(future.Issues, ap.IssueFutureBill) {
		t.Errorf("FutureDatedInvalidBill fixture: expected IssueFutureBill, got %+v", future.Issues)
	}
}

func TestFixtures_HistoricalDeteriorationAndImprovement(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)

	detSnaps, detCurrent := apfixtures.DeterioratingSnapshots()
	det := ap.Calculate(ap.Input{Payables: detCurrent, Snapshots: detSnaps}, ap.Options{AsOfDate: asOf})
	if !det.Available {
		t.Fatalf("DeterioratingSnapshots: expected Available=true, issues: %+v", det.Issues)
	}
	if !det.Migration.Available || det.Migration.DeteriorationAmount <= 0 {
		t.Errorf("DeterioratingSnapshots: expected positive DeteriorationAmount, got %+v", det.Migration)
	}

	impSnaps, impCurrent := apfixtures.ImprovingSnapshots()
	imp := ap.Calculate(ap.Input{Payables: impCurrent, Snapshots: impSnaps}, ap.Options{AsOfDate: asOf})
	if !imp.Available {
		t.Fatalf("ImprovingSnapshots: expected Available=true, issues: %+v", imp.Issues)
	}
	if !imp.Migration.Available || imp.Migration.CureAmount <= 0 {
		t.Errorf("ImprovingSnapshots: expected positive CureAmount, got %+v", imp.Migration)
	}
}

func TestFixtures_GLSubledgerMismatch(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	payables, controlBalance := apfixtures.GLSubledgerMismatch()
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, ControlAccountBalance: &controlBalance})
	if result.ControlAccountReconciliation.Reconciled {
		t.Errorf("GLSubledgerMismatch: expected Reconciled=false, got %+v", result.ControlAccountReconciliation)
	}
}

func TestFixtures_DPOHistorySources(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	purchases := ap.Calculate(ap.Input{PurchasesHistory: apfixtures.PurchasesHistoryForDPO()}, ap.Options{AsOfDate: asOf})
	if !purchases.DPOHistory.Available {
		t.Errorf("PurchasesHistoryForDPO: expected DPOHistory.Available=true")
	}
	if purchases.DPOHistory.Points[0].DPO.Basis != ap.DPOBasisPurchases {
		t.Errorf("PurchasesHistoryForDPO: expected PURCHASES basis, got %v", purchases.DPOHistory.Points[0].DPO.Basis)
	}

	cogs := ap.Calculate(ap.Input{PurchasesHistory: apfixtures.COGSHistoryForDPO()}, ap.Options{AsOfDate: asOf})
	if !cogs.DPO.Available {
		t.Errorf("COGSHistoryForDPO: expected DPO.Available=true")
	}
	if cogs.DPO.Basis != ap.DPOBasisCOGS {
		t.Errorf("COGSHistoryForDPO: expected COGS basis, got %v", cogs.DPO.Basis)
	}
}
