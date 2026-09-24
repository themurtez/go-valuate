package vendorspend_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/ap"
	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

// TestAPAdapter_VendorCreditMapsToEffectCredit verifies a
// DocumentTypeVendorCredit Payable (negative OriginalAmount by
// accounting/ap convention) converts to EffectCredit with a
// non-negative Amount.
func TestAPAdapter_VendorCreditMapsToEffectCredit(t *testing.T) {
	payables := []ap.Payable{
		{ID: "CR-1", SupplierID: "SUP-1", DocumentType: ap.DocumentTypeVendorCredit,
			BillDate: time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC), DueDate: time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
			OriginalAmount: -750, OpenAmount: -750, Currency: "USD", Status: ap.StatusOpen},
	}
	records := vendorspend.SpendRecordsFromPayables(payables, func(d string) string { return d[:7] })
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	r := records[0]
	if r.Effect != vendorspend.EffectCredit {
		t.Errorf("Effect = %v, want EffectCredit", r.Effect)
	}
	if r.Amount != 750 {
		t.Errorf("Amount = %v, want 750 (non-negative magnitude)", r.Amount)
	}
}

// TestAPAdapter_OrdinaryBillMapsToEffectNormal verifies an ordinary bill
// maps to EffectNormal with its OriginalAmount preserved as-is.
func TestAPAdapter_OrdinaryBillMapsToEffectNormal(t *testing.T) {
	payables := []ap.Payable{
		{ID: "B-1", SupplierID: "SUP-1", BillDate: time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
			DueDate: time.Date(2025, 4, 10, 0, 0, 0, 0, time.UTC), OriginalAmount: 5000, OpenAmount: 5000, Currency: "USD", Status: ap.StatusOpen},
	}
	records := vendorspend.SpendRecordsFromPayables(payables, func(d string) string { return d[:7] })
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Effect != vendorspend.EffectNormal {
		t.Errorf("Effect = %v, want EffectNormal", records[0].Effect)
	}
	if records[0].Amount != 5000 {
		t.Errorf("Amount = %v, want 5000", records[0].Amount)
	}
}

// TestAPAdapter_UsesBasisAccrual verifies every converted record carries
// BasisAccrual, never silently defaulting to BasisUnknown.
func TestAPAdapter_UsesBasisAccrual(t *testing.T) {
	payables := []ap.Payable{
		{ID: "B-1", SupplierID: "SUP-1", BillDate: time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
			DueDate: time.Date(2025, 4, 10, 0, 0, 0, 0, time.UTC), OriginalAmount: 1000, Currency: "USD", Status: ap.StatusOpen},
	}
	records := vendorspend.SpendRecordsFromPayables(payables, func(d string) string { return d[:7] })
	if records[0].Basis != vendorspend.BasisAccrual {
		t.Errorf("Basis = %v, want BasisAccrual", records[0].Basis)
	}
}
