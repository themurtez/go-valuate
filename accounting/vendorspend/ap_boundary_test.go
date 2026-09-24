package vendorspend_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/ap"
	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

// TestAPBoundary_SpendNeverBecomesEndingAP is a permanent regression test
// — task section 24. A supplier with $100,000 of annual spend and only
// $10,000 of ending AP (because most bills were paid during the year)
// must report NetSpend == 100,000, never silently collapsing to the
// ending-AP figure. This is the single most important distinction this
// package exists to enforce: accounting/ap answers "what do we owe
// today," this package answers "what did we buy this year," and the two
// numbers are expected to differ.
func TestAPBoundary_SpendNeverBecomesEndingAP(t *testing.T) {
	const supplierAnnualSpend = 100000.0
	const endingAPBalance = 10000.0

	// Twelve monthly bills of $100,000/12 each, eleven of which are paid
	// (OpenAmount 0) and one still open for exactly endingAPBalance — a
	// realistic "we bought a lot, but paid most of it" shape.
	monthlyAmount := supplierAnnualSpend / 12
	var payables []ap.Payable
	var periods []vendorspend.Period
	var spendRecords []vendorspend.SpendRecord

	for m := 1; m <= 12; m++ {
		billDate := time.Date(2025, time.Month(m), 5, 0, 0, 0, 0, time.UTC)
		periodLabel := billDate.Format("2006-01")
		periods = append(periods, vendorspend.Period{
			Period: periodLabel, StartDate: time.Date(2025, time.Month(m), 1, 0, 0, 0, 0, time.UTC),
			EndDate: billDate.AddDate(0, 1, -5), SequenceInYear: m,
		})

		open := 0.0
		status := ap.StatusPaid
		if m == 12 {
			open = endingAPBalance
			status = ap.StatusOpen
		}
		payables = append(payables, ap.Payable{
			ID: "BILL-" + periodLabel, SupplierID: "SUP-BOUNDARY", SupplierName: "Boundary Test Vendor",
			BillDate: billDate, DueDate: billDate.AddDate(0, 1, 0),
			OriginalAmount: monthlyAmount, OpenAmount: open, Currency: "USD", Status: status,
		})

		// The SAME economic purchases, converted independently into this
		// package's own SpendRecord shape (not via the adapter, to prove
		// the boundary holds even for a caller who populates both paths
		// separately from the same source system).
		spendRecords = append(spendRecords, vendorspend.SpendRecord{
			SpendID: "SPEND-" + periodLabel, SupplierID: "SUP-BOUNDARY", Period: periodLabel, Date: billDate,
			Amount: monthlyAmount, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
		})
	}

	// Sanity-check the AP side actually models what the test claims: ending
	// AP (sum of OpenAmount) is endingAPBalance.
	var totalOpenAP float64
	for _, p := range payables {
		totalOpenAP += p.OpenAmount
	}
	if totalOpenAP != endingAPBalance {
		t.Fatalf("test setup error: ending AP = %v, want %v", totalOpenAP, endingAPBalance)
	}

	result := vendorspend.Calculate(vendorspend.Input{
		Periods:      periods,
		Suppliers:    []vendorspend.Supplier{{SupplierID: "SUP-BOUNDARY", Name: "Boundary Test Vendor", Active: true}},
		SpendRecords: spendRecords,
	}, vendorspend.Options{})

	if !result.Available {
		t.Fatalf("expected Available result, got Issues: %+v", result.Issues)
	}
	// almostEqual (not exact ==) since supplierAnnualSpend/12 is not an
	// exact binary float and summing it back 12 times accumulates a
	// sub-cent rounding difference — this is floating-point precision in
	// the TEST's own arithmetic, not a package correctness issue.
	if !almostEqual(result.Bridge.NetSpend, supplierAnnualSpend) {
		t.Fatalf("NetSpend = %v, want the full annual spend %v (must NEVER collapse to ending AP = %v)",
			result.Bridge.NetSpend, supplierAnnualSpend, endingAPBalance)
	}
	if almostEqual(result.Bridge.NetSpend, endingAPBalance) {
		t.Fatalf("REGRESSION: NetSpend equals ending AP balance (%v) — spend has collapsed to an AP balance", endingAPBalance)
	}
}

// TestAPBoundary_AdapterAlsoPreservesGrossSpend proves the same
// invariant holds when SpendRecords are built via
// SpendRecordsFromPayables (the typed AP adapter) rather than populated
// independently — the adapter must not itself introduce an
// OpenAmount-based collapse.
func TestAPBoundary_AdapterAlsoPreservesGrossSpend(t *testing.T) {
	payables := []ap.Payable{
		{ID: "B-1", SupplierID: "SUP-ADAPT", BillDate: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			DueDate: time.Date(2025, 2, 5, 0, 0, 0, 0, time.UTC), OriginalAmount: 50000, OpenAmount: 0, Currency: "USD", Status: ap.StatusPaid},
		{ID: "B-2", SupplierID: "SUP-ADAPT", BillDate: time.Date(2025, 2, 5, 0, 0, 0, 0, time.UTC),
			DueDate: time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC), OriginalAmount: 50000, OpenAmount: 5000, Currency: "USD", Status: ap.StatusPartiallyPaid},
	}
	periodOf := func(billDate string) string { return billDate[:7] }
	spendRecords := vendorspend.SpendRecordsFromPayables(payables, periodOf)

	var sumOriginal float64
	for _, p := range payables {
		sumOriginal += p.OriginalAmount
	}

	periods := []vendorspend.Period{
		{Period: "2025-01", StartDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), SequenceInYear: 1},
		{Period: "2025-02", StartDate: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC), SequenceInYear: 2},
	}
	result := vendorspend.Calculate(vendorspend.Input{
		Periods:      periods,
		Suppliers:    []vendorspend.Supplier{{SupplierID: "SUP-ADAPT", Active: true}},
		SpendRecords: spendRecords,
	}, vendorspend.Options{})

	if !result.Available {
		t.Fatalf("expected Available result, got Issues: %+v", result.Issues)
	}
	if result.Bridge.NetSpend != sumOriginal {
		t.Fatalf("adapter-derived NetSpend = %v, want sum of OriginalAmount = %v (must use OriginalAmount, not OpenAmount)",
			result.Bridge.NetSpend, sumOriginal)
	}
}
