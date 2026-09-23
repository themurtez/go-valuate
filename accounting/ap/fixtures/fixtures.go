// Package fixtures provides synthetic accounting/ap data for tests and
// examples: a set of small, hand-built payables portfolios covering the
// scenarios accounting/ap's tests exercise. Nothing here is real financial
// data — every figure is invented for illustration, mirroring
// accounting/ar/fixtures' identical synthetic-data convention.
package fixtures

import (
	"time"

	"github.com/themurtez/go-valuate/accounting/ap"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// AsOfDate is the fixed analysis date every fixture below is aged against.
const AsOfDate = "2025-06-30"

// HealthyPayables returns a small, mostly-current AP portfolio: a handful
// of suppliers, mostly paid on time, nothing overdue past 30 days.
func HealthyPayables() []ap.Payable {
	return []ap.Payable{
		{
			ID: "BILL-1001", SupplierID: "SUP-A", SupplierName: "Acme Supply Co",
			BillDate: date("2025-06-05"), DueDate: date("2025-07-05"),
			OriginalAmount: 10000, OpenAmount: 10000, Currency: "USD",
			Status: ap.StatusOpen, TermsDays: 30,
		},
		{
			ID: "BILL-1002", SupplierID: "SUP-B", SupplierName: "Beta Materials LLC",
			BillDate: date("2025-06-01"), DueDate: date("2025-06-15"),
			OriginalAmount: 5000, OpenAmount: 5000, Currency: "USD",
			Status: ap.StatusOpen, TermsDays: 14,
		},
		{
			ID: "BILL-1003", SupplierID: "SUP-C", SupplierName: "Gamma Logistics Inc",
			BillDate: date("2025-05-01"), DueDate: date("2025-05-31"),
			OriginalAmount: 7500, OpenAmount: 2500, Currency: "USD",
			Status: ap.StatusPartiallyPaid, TermsDays: 30,
		},
		{
			ID: "BILL-1004", SupplierID: "SUP-A", SupplierName: "Acme Supply Co",
			BillDate: date("2025-04-01"), DueDate: date("2025-05-01"),
			OriginalAmount: 3000, OpenAmount: 0, Currency: "USD",
			Status: ap.StatusPaid, TermsDays: 30,
		},
	}
}

// AgingHeavyPayables returns a portfolio with significant balances spread
// across every overdue bucket, including 91+.
func AgingHeavyPayables() []ap.Payable {
	return []ap.Payable{
		{ID: "BILL-2001", SupplierID: "SUP-X", SupplierName: "Xylo Components",
			BillDate: date("2025-06-10"), DueDate: date("2025-07-10"),
			OriginalAmount: 4000, OpenAmount: 4000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
		{ID: "BILL-2002", SupplierID: "SUP-X", SupplierName: "Xylo Components",
			BillDate: date("2025-05-20"), DueDate: date("2025-06-19"),
			OriginalAmount: 6000, OpenAmount: 6000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
		{ID: "BILL-2003", SupplierID: "SUP-Y", SupplierName: "Yotta Freight Ltd",
			BillDate: date("2025-05-01"), DueDate: date("2025-05-31"),
			OriginalAmount: 9000, OpenAmount: 9000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
		{ID: "BILL-2004", SupplierID: "SUP-Y", SupplierName: "Yotta Freight Ltd",
			BillDate: date("2025-04-01"), DueDate: date("2025-05-01"),
			OriginalAmount: 8000, OpenAmount: 8000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
		{ID: "BILL-2005", SupplierID: "SUP-Z", SupplierName: "Zed Raw Materials",
			BillDate: date("2025-02-01"), DueDate: date("2025-03-03"),
			OriginalAmount: 15000, OpenAmount: 15000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
	}
}

// OneLargeOverdueSupplier returns a portfolio dominated by one supplier
// with a large 90+ overdue balance, plus a few small, current suppliers —
// used to exercise concentration/flag scenarios.
func OneLargeOverdueSupplier() []ap.Payable {
	return []ap.Payable{
		{ID: "BILL-3001", SupplierID: "SUP-BIG", SupplierName: "Big Supplier Inc",
			BillDate: date("2025-01-15"), DueDate: date("2025-02-14"),
			OriginalAmount: 100000, OpenAmount: 100000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
		{ID: "BILL-3002", SupplierID: "SUP-SMALL1", SupplierName: "Small One",
			BillDate: date("2025-06-10"), DueDate: date("2025-07-10"),
			OriginalAmount: 1200, OpenAmount: 1200, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
		{ID: "BILL-3003", SupplierID: "SUP-SMALL2", SupplierName: "Small Two",
			BillDate: date("2025-06-12"), DueDate: date("2025-07-12"),
			OriginalAmount: 900, OpenAmount: 900, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
	}
}

// ManySmallSuppliers returns a broad, low-concentration portfolio of many
// small, mostly-current suppliers.
func ManySmallSuppliers() []ap.Payable {
	var out []ap.Payable
	supIDs := []string{"S1", "S2", "S3", "S4", "S5", "S6", "S7", "S8", "S9", "S10"}
	for i, sid := range supIDs {
		out = append(out, ap.Payable{
			ID: "BILL-SM-" + sid, SupplierID: sid, SupplierName: "Supplier " + sid,
			BillDate: date("2025-06-01"), DueDate: date("2025-06-15"),
			OriginalAmount: float64(500 + i*50), OpenAmount: float64(500 + i*50),
			Currency: "USD", Status: ap.StatusOpen, TermsDays: 14,
		})
	}
	return out
}

// DisputedBills returns a portfolio including disputed payables that
// still age.
func DisputedBills() []ap.Payable {
	return []ap.Payable{
		{ID: "BILL-4001", SupplierID: "SUP-D1", SupplierName: "Disputed Vendor One",
			BillDate: date("2025-03-01"), DueDate: date("2025-03-31"),
			OriginalAmount: 20000, OpenAmount: 20000, Currency: "USD", Status: ap.StatusDisputed, TermsDays: 30},
		{ID: "BILL-4002", SupplierID: "SUP-D2", SupplierName: "Disputed Vendor Two",
			BillDate: date("2025-06-05"), DueDate: date("2025-07-05"),
			OriginalAmount: 3000, OpenAmount: 3000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
	}
}

// PartialPayments returns payables with a mix of full, partial, and zero
// payments applied.
func PartialPayments() []ap.Payable {
	return []ap.Payable{
		{ID: "BILL-5001", SupplierID: "SUP-P1", SupplierName: "Payee One",
			BillDate: date("2025-05-01"), DueDate: date("2025-05-31"),
			OriginalAmount: 10000, OpenAmount: 4000, Currency: "USD", Status: ap.StatusPartiallyPaid, TermsDays: 30},
		{ID: "BILL-5002", SupplierID: "SUP-P1", SupplierName: "Payee One",
			BillDate: date("2025-06-01"), DueDate: date("2025-07-01"),
			OriginalAmount: 5000, OpenAmount: 0, Currency: "USD", Status: ap.StatusPaid, TermsDays: 30},
	}
}

// SupplierVendorCredits returns a portfolio including vendor credits,
// exercising both no-netting (default) and net-by-supplier policy paths.
func SupplierVendorCredits() []ap.Payable {
	return []ap.Payable{
		{ID: "BILL-6001", SupplierID: "SUP-CR", SupplierName: "Credit Supplier",
			BillDate: date("2025-06-01"), DueDate: date("2025-07-01"),
			OriginalAmount: 8000, OpenAmount: 8000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
		{ID: "VC-6002", SupplierID: "SUP-CR", SupplierName: "Credit Supplier",
			DocumentType: ap.DocumentTypeVendorCredit,
			BillDate:     date("2025-06-10"), DueDate: date("2025-06-10"),
			OriginalAmount: -1500, OpenAmount: -1500, Currency: "USD", Status: ap.StatusOpen,
		},
	}
}

// MixedCurrency returns a portfolio spanning two currencies, used to
// exercise IssueMixedCurrency handling.
func MixedCurrency() []ap.Payable {
	return []ap.Payable{
		{ID: "BILL-7001", SupplierID: "SUP-US", SupplierName: "US Supplier",
			BillDate: date("2025-06-01"), DueDate: date("2025-07-01"),
			OriginalAmount: 5000, OpenAmount: 5000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
		{ID: "BILL-7002", SupplierID: "SUP-EU", SupplierName: "EU Supplier",
			BillDate: date("2025-06-01"), DueDate: date("2025-07-01"),
			OriginalAmount: 4500, OpenAmount: 4500, Currency: "EUR", Status: ap.StatusOpen, TermsDays: 30},
	}
}

// ZeroAP returns an empty payables portfolio.
func ZeroAP() []ap.Payable {
	return nil
}

// FutureDatedInvalidBill returns a portfolio containing one payable whose
// BillDate is after AsOfDate — an issue-worthy input.
func FutureDatedInvalidBill() []ap.Payable {
	return []ap.Payable{
		{ID: "BILL-8001", SupplierID: "SUP-F", SupplierName: "Future Supplier",
			BillDate: date("2025-08-01"), DueDate: date("2025-08-31"),
			OriginalAmount: 2000, OpenAmount: 2000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
	}
}

// DeterioratingSnapshots returns a two-snapshot sequence (plus a
// corresponding "current" payables list) showing a portfolio's aging
// worsening over time — for migration/trend deterioration tests.
func DeterioratingSnapshots() (snapshots []ap.Snapshot, current []ap.Payable) {
	snapshots = []ap.Snapshot{
		{
			AsOfDate: "2025-05-31",
			Payables: []ap.Payable{
				{ID: "BILL-9001", SupplierID: "SUP-DET", SupplierName: "Deteriorating Supplier",
					BillDate: date("2025-05-01"), DueDate: date("2025-05-31"),
					OriginalAmount: 5000, OpenAmount: 5000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
			},
		},
	}
	current = []ap.Payable{
		{ID: "BILL-9001", SupplierID: "SUP-DET", SupplierName: "Deteriorating Supplier",
			BillDate: date("2025-05-01"), DueDate: date("2025-05-31"),
			OriginalAmount: 5000, OpenAmount: 5000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
	}
	return snapshots, current
}

// ImprovingSnapshots returns a two-state sequence (one snapshot plus a
// current list) where the payable has been paid off between the two
// states — for migration/trend improvement tests.
func ImprovingSnapshots() (snapshots []ap.Snapshot, current []ap.Payable) {
	snapshots = []ap.Snapshot{
		{
			AsOfDate: "2025-04-30",
			Payables: []ap.Payable{
				{ID: "BILL-9101", SupplierID: "SUP-IMP", SupplierName: "Improving Supplier",
					BillDate: date("2025-02-01"), DueDate: date("2025-03-03"),
					OriginalAmount: 6000, OpenAmount: 6000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
			},
		},
	}
	current = nil // fully paid; no longer open as of the current AsOfDate.
	return snapshots, current
}

// GLSubledgerMismatch returns a payables portfolio paired with a GL
// control-account balance that deliberately does not match the subledger
// total.
func GLSubledgerMismatch() (payables []ap.Payable, controlBalance float64) {
	payables = HealthyPayables()
	return payables, 999999
}

// PurchasesHistoryForDPO returns a simple PayablesPeriod series usable for
// DPO tests, paired with HealthyPayables' total open AP.
func PurchasesHistoryForDPO() []ap.PayablesPeriod {
	return []ap.PayablesPeriod{
		{Period: "2025-Q1", DenominatorAmount: 40000, Days: 90, Basis: ap.DPOBasisPurchases, EndingAP: floatPtr(12000)},
		{Period: "2025-Q2", DenominatorAmount: 45000, Days: 91, Basis: ap.DPOBasisPurchases, EndingAP: floatPtr(17500)},
	}
}

// COGSHistoryForDPO returns a PayablesPeriod series using COGS as the
// explicit proxy basis, for DPO-via-COGS tests.
func COGSHistoryForDPO() []ap.PayablesPeriod {
	return []ap.PayablesPeriod{
		{Period: "2025", DenominatorAmount: 200000, Days: 365, Basis: ap.DPOBasisCOGS, EndingAP: floatPtr(20000)},
	}
}

// NearTermPaymentPressure returns a portfolio with a large amount due
// within the next 7 days, for payment-pressure tests.
func NearTermPaymentPressure() []ap.Payable {
	return []ap.Payable{
		{ID: "BILL-10001", SupplierID: "SUP-NT1", SupplierName: "Near-Term One",
			BillDate: date("2025-06-01"), DueDate: date("2025-07-03"),
			OriginalAmount: 25000, OpenAmount: 25000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
		{ID: "BILL-10002", SupplierID: "SUP-NT2", SupplierName: "Near-Term Two",
			BillDate: date("2025-06-05"), DueDate: date("2025-07-02"),
			OriginalAmount: 15000, OpenAmount: 15000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 30},
	}
}

// LongTermsNotLate returns a supplier with long contractual terms (net
// 60) who consistently pays on or before the due date — for
// terms-vs-lateness distinction tests.
func LongTermsNotLate() ([]ap.Payable, []ap.SupplierPayment) {
	payables := []ap.Payable{
		// Already paid, on time -- exercises PaymentTiming.
		{ID: "BILL-11001", SupplierID: "SUP-LONG", SupplierName: "Long Terms Supplier",
			BillDate: date("2025-04-01"), DueDate: date("2025-05-31"),
			OriginalAmount: 10000, OpenAmount: 0, Currency: "USD", Status: ap.StatusPaid, TermsDays: 60},
		// Still open (not yet due) -- exercises TermsAnalysis.AverageTermsDays,
		// which is scoped to aging-included (still-outstanding) payables and
		// so never sees a fully PAID bill like BILL-11001 above.
		{ID: "BILL-11002", SupplierID: "SUP-LONG", SupplierName: "Long Terms Supplier",
			BillDate: date("2025-06-01"), DueDate: date("2025-07-31"),
			OriginalAmount: 12000, OpenAmount: 12000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 60},
	}
	payments := []ap.SupplierPayment{
		{ID: "PMT-11001", PayableID: "BILL-11001", SupplierID: "SUP-LONG", Date: date("2025-05-28"), Amount: 10000},
	}
	return payables, payments
}

// ChronicallyLatePayer returns a supplier with short contractual terms
// (net 15) who is repeatedly paid well after the due date — for
// repeated-late-payment flag tests.
func ChronicallyLatePayer() ([]ap.Payable, []ap.SupplierPayment) {
	payables := []ap.Payable{
		{ID: "BILL-12001", SupplierID: "SUP-LATE", SupplierName: "Chronically Late Payee",
			BillDate: date("2025-01-01"), DueDate: date("2025-01-16"),
			OriginalAmount: 1000, OpenAmount: 0, Currency: "USD", Status: ap.StatusPaid, TermsDays: 15},
		{ID: "BILL-12002", SupplierID: "SUP-LATE", SupplierName: "Chronically Late Payee",
			BillDate: date("2025-02-01"), DueDate: date("2025-02-16"),
			OriginalAmount: 1000, OpenAmount: 0, Currency: "USD", Status: ap.StatusPaid, TermsDays: 15},
		{ID: "BILL-12003", SupplierID: "SUP-LATE", SupplierName: "Chronically Late Payee",
			BillDate: date("2025-03-01"), DueDate: date("2025-03-16"),
			OriginalAmount: 1000, OpenAmount: 0, Currency: "USD", Status: ap.StatusPaid, TermsDays: 15},
		// Still open -- FlagRepeatedLatePayment is only evaluated for
		// suppliers appearing in SupplierSummaries (i.e. with at least one
		// currently outstanding balance), matching accounting/ar's
		// identical scoping. Without this row, SUP-LATE's three fully-paid
		// bills above never produce a SupplierSummary entry and the flag
		// can never fire, even though the late-payment history is real.
		{ID: "BILL-12004", SupplierID: "SUP-LATE", SupplierName: "Chronically Late Payee",
			BillDate: date("2025-06-01"), DueDate: date("2025-06-16"),
			OriginalAmount: 1000, OpenAmount: 1000, Currency: "USD", Status: ap.StatusOpen, TermsDays: 15},
	}
	payments := []ap.SupplierPayment{
		{ID: "PMT-12001", PayableID: "BILL-12001", SupplierID: "SUP-LATE", Date: date("2025-02-05"), Amount: 1000},
		{ID: "PMT-12002", PayableID: "BILL-12002", SupplierID: "SUP-LATE", Date: date("2025-03-08"), Amount: 1000},
		{ID: "PMT-12003", PayableID: "BILL-12003", SupplierID: "SUP-LATE", Date: date("2025-03-25"), Amount: 1000},
	}
	return payables, payments
}

func floatPtr(v float64) *float64 { return &v }
