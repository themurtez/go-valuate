package vendorspend_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
	"github.com/themurtez/go-valuate/accounting/vendorspend/fixtures"
)

func float64Ptr(v float64) *float64 { return &v }

// TestControlReconciliation_Match verifies a control total matching
// VendorSpend within tolerance reports Reconciled == true.
func TestControlReconciliation_Match(t *testing.T) {
	in := vendorspend.Input{
		Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DiversifiedSuppliers(), SpendRecords: fixtures.DiversifiedSpend(),
		Controls: vendorspend.ControlTotals{Purchases: float64Ptr(240000)}, // must equal the actual computed NetSpend for this fixture
	}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Available {
		t.Fatalf("expected Available, got Issues: %+v", result.Issues)
	}
	// Set the control to the package's OWN computed NetSpend so the test
	// asserts reconciliation logic, not a hand-guessed fixture total.
	in.Controls.Purchases = float64Ptr(result.Bridge.NetSpend)
	result2 := vendorspend.Calculate(in, vendorspend.Options{})
	if !result2.ControlReconciliation.Purchases.Available {
		t.Fatalf("expected Purchases reconciliation Available")
	}
	if !result2.ControlReconciliation.Purchases.Reconciled {
		t.Errorf("expected Reconciled == true when control matches VendorSpend exactly, got %+v", result2.ControlReconciliation.Purchases)
	}
	for _, f := range result2.Flags {
		if f.Code == vendorspend.FlagControlTotalMismatch {
			t.Errorf("did not expect FlagControlTotalMismatch when reconciled")
		}
	}
}

// TestControlReconciliation_Mismatch verifies a control total that
// differs from VendorSpend beyond tolerance reports Reconciled == false
// and triggers FlagControlTotalMismatch.
func TestControlReconciliation_Mismatch(t *testing.T) {
	in := vendorspend.Input{
		Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DiversifiedSuppliers(), SpendRecords: fixtures.DiversifiedSpend(),
		Controls: vendorspend.ControlTotals{Purchases: float64Ptr(1)}, // wildly different from actual spend
	}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.ControlReconciliation.Purchases.Available {
		t.Fatalf("expected Purchases reconciliation Available")
	}
	if result.ControlReconciliation.Purchases.Reconciled {
		t.Errorf("expected Reconciled == false for a wildly mismatched control, got %+v", result.ControlReconciliation.Purchases)
	}
	found := false
	for _, f := range result.Flags {
		if f.Code == vendorspend.FlagControlTotalMismatch {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagControlTotalMismatch, got flags: %+v", result.Flags)
	}
}

// TestControlReconciliation_UnavailableWhenNotSupplied verifies an
// unsupplied control total (nil, not zero) leaves that component's
// reconciliation Available == false — never silently treated as zero.
func TestControlReconciliation_UnavailableWhenNotSupplied(t *testing.T) {
	in := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DiversifiedSuppliers(), SpendRecords: fixtures.DiversifiedSpend()}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	cr := result.ControlReconciliation
	for name, c := range map[string]vendorspend.ComponentReconciliation{
		"Purchases": cr.Purchases, "ExpenseSpend": cr.ExpenseSpend, "CapexSpend": cr.CapexSpend,
		"InventoryPurchases": cr.InventoryPurchases, "ContractorSpend": cr.ContractorSpend,
	} {
		if c.Available {
			t.Errorf("%s: expected Available == false with no control supplied, got %+v", name, c)
		}
	}
}

// TestControlReconciliation_OffsettingDifferencesStayVisible verifies
// task section 29's "offsetting component differences must remain
// visible" rule: two DIFFERENT control totals with opposite-direction
// mismatches must each report their own Difference independently, never
// netted against each other.
func TestControlReconciliation_OffsettingDifferencesStayVisible(t *testing.T) {
	in := vendorspend.Input{
		Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.DiversifiedSuppliers(), SpendRecords: fixtures.DiversifiedSpend(),
	}
	baseline := vendorspend.Calculate(in, vendorspend.Options{})
	netSpend := baseline.Bridge.NetSpend

	in.Controls = vendorspend.ControlTotals{
		Purchases:    float64Ptr(netSpend + 1000), // over by 1000
		ExpenseSpend: float64Ptr(netSpend - 1000), // under by 1000
	}
	result := vendorspend.Calculate(in, vendorspend.Options{})

	// Difference is defined as VendorSpend - ControlAmount (see
	// ComponentReconciliation's doc comment). Purchases' control is
	// over-stated (netSpend+1000), so VendorSpend is LESS than the
	// control: Difference is negative. ExpenseSpend's control is
	// under-stated (netSpend-1000), so VendorSpend is GREATER than the
	// control: Difference is positive. The two must not be equal (each
	// component is reconciled independently, never netted).
	if result.ControlReconciliation.Purchases.Difference == result.ControlReconciliation.ExpenseSpend.Difference {
		t.Fatalf("expected different Difference signs for Purchases vs ExpenseSpend, got both = %v",
			result.ControlReconciliation.Purchases.Difference)
	}
	if result.ControlReconciliation.Purchases.Difference >= 0 {
		t.Errorf("Purchases.Difference = %v, want < 0 (VendorSpend is LESS than the over-stated control)", result.ControlReconciliation.Purchases.Difference)
	}
	if result.ControlReconciliation.ExpenseSpend.Difference <= 0 {
		t.Errorf("ExpenseSpend.Difference = %v, want > 0 (VendorSpend is GREATER than the under-stated control)", result.ControlReconciliation.ExpenseSpend.Difference)
	}
	// Both should be flagged as mismatched — neither offsets the other.
	mismatchCount := 0
	for _, f := range result.Flags {
		if f.Code == vendorspend.FlagControlTotalMismatch {
			mismatchCount++
		}
	}
	if mismatchCount != 2 {
		t.Errorf("expected 2 FlagControlTotalMismatch entries (one per mismatched component), got %d", mismatchCount)
	}
}
