// Package fixtures provides synthetic accounting/ar data for tests and
// examples: a set of small, hand-built receivables portfolios covering the
// scenarios accounting/ar's tests exercise. Nothing here is real financial
// data — every figure is invented for illustration, mirroring
// accounting/ledger/fixtures and accounting/statements' own synthetic-data
// convention.
package fixtures

import (
	"time"

	"github.com/themurtez/go-valuate/accounting/ar"
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

// HealthyPortfolio returns a small, mostly-current AR portfolio: a handful
// of customers, mostly paying on time, nothing overdue past 30 days.
func HealthyPortfolio() []ar.Receivable {
	return []ar.Receivable{
		{
			ID: "INV-1001", CustomerID: "CUST-A", CustomerName: "Acme Co",
			InvoiceDate: date("2025-06-05"), DueDate: date("2025-07-05"),
			OriginalAmount: 10000, OpenAmount: 10000, Currency: "USD",
			Status: ar.StatusOpen, TermsDays: 30,
		},
		{
			ID: "INV-1002", CustomerID: "CUST-B", CustomerName: "Beta LLC",
			InvoiceDate: date("2025-06-01"), DueDate: date("2025-06-15"),
			OriginalAmount: 5000, OpenAmount: 5000, Currency: "USD",
			Status: ar.StatusOpen, TermsDays: 14,
		},
		{
			ID: "INV-1003", CustomerID: "CUST-C", CustomerName: "Gamma Inc",
			InvoiceDate: date("2025-05-01"), DueDate: date("2025-05-31"),
			OriginalAmount: 7500, OpenAmount: 2500, Currency: "USD",
			Status: ar.StatusPartiallyPaid, TermsDays: 30,
		},
		{
			ID: "INV-1004", CustomerID: "CUST-A", CustomerName: "Acme Co",
			InvoiceDate: date("2025-04-01"), DueDate: date("2025-05-01"),
			OriginalAmount: 3000, OpenAmount: 0, Currency: "USD",
			Status: ar.StatusPaid, TermsDays: 30,
		},
	}
}

// AgingHeavyPortfolio returns a portfolio with significant balances spread
// across every overdue bucket, including 91+.
func AgingHeavyPortfolio() []ar.Receivable {
	return []ar.Receivable{
		{ID: "INV-2001", CustomerID: "CUST-X", CustomerName: "Xylo Corp",
			InvoiceDate: date("2025-06-10"), DueDate: date("2025-07-10"),
			OriginalAmount: 4000, OpenAmount: 4000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
		{ID: "INV-2002", CustomerID: "CUST-X", CustomerName: "Xylo Corp",
			InvoiceDate: date("2025-05-20"), DueDate: date("2025-06-19"),
			OriginalAmount: 6000, OpenAmount: 6000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
		{ID: "INV-2003", CustomerID: "CUST-Y", CustomerName: "Yotta Ltd",
			InvoiceDate: date("2025-05-01"), DueDate: date("2025-05-31"),
			OriginalAmount: 9000, OpenAmount: 9000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
		{ID: "INV-2004", CustomerID: "CUST-Y", CustomerName: "Yotta Ltd",
			InvoiceDate: date("2025-04-01"), DueDate: date("2025-05-01"),
			OriginalAmount: 8000, OpenAmount: 8000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
		{ID: "INV-2005", CustomerID: "CUST-Z", CustomerName: "Zed Partners",
			InvoiceDate: date("2025-02-01"), DueDate: date("2025-03-03"),
			OriginalAmount: 15000, OpenAmount: 15000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
	}
}

// OneLargeOverdueCustomer returns a portfolio dominated by one customer
// with a large 90+ overdue balance, plus a few small, current customers —
// used to exercise concentration/flag scenarios.
func OneLargeOverdueCustomer() []ar.Receivable {
	return []ar.Receivable{
		{ID: "INV-3001", CustomerID: "CUST-BIG", CustomerName: "Big Client Inc",
			InvoiceDate: date("2025-01-15"), DueDate: date("2025-02-14"),
			OriginalAmount: 100000, OpenAmount: 100000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
		{ID: "INV-3002", CustomerID: "CUST-SMALL1", CustomerName: "Small One",
			InvoiceDate: date("2025-06-10"), DueDate: date("2025-07-10"),
			OriginalAmount: 1200, OpenAmount: 1200, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
		{ID: "INV-3003", CustomerID: "CUST-SMALL2", CustomerName: "Small Two",
			InvoiceDate: date("2025-06-12"), DueDate: date("2025-07-12"),
			OriginalAmount: 900, OpenAmount: 900, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
	}
}

// ManySmallCustomers returns a broad, low-concentration portfolio of many
// small, mostly-current customers.
func ManySmallCustomers() []ar.Receivable {
	var out []ar.Receivable
	custIDs := []string{"C1", "C2", "C3", "C4", "C5", "C6", "C7", "C8", "C9", "C10"}
	for i, cid := range custIDs {
		out = append(out, ar.Receivable{
			ID: "INV-SM-" + cid, CustomerID: cid, CustomerName: "Customer " + cid,
			InvoiceDate: date("2025-06-01"), DueDate: date("2025-06-15"),
			OriginalAmount: float64(500 + i*50), OpenAmount: float64(500 + i*50),
			Currency: "USD", Status: ar.StatusOpen, TermsDays: 14,
		})
	}
	return out
}

// DisputedInvoices returns a portfolio including disputed receivables that
// still age.
func DisputedInvoices() []ar.Receivable {
	return []ar.Receivable{
		{ID: "INV-4001", CustomerID: "CUST-D1", CustomerName: "Disputer One",
			InvoiceDate: date("2025-03-01"), DueDate: date("2025-03-31"),
			OriginalAmount: 20000, OpenAmount: 20000, Currency: "USD", Status: ar.StatusDisputed, TermsDays: 30},
		{ID: "INV-4002", CustomerID: "CUST-D2", CustomerName: "Disputer Two",
			InvoiceDate: date("2025-06-05"), DueDate: date("2025-07-05"),
			OriginalAmount: 3000, OpenAmount: 3000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
	}
}

// PartialPayments returns receivables with a mix of full, partial, and
// zero payments applied.
func PartialPayments() []ar.Receivable {
	return []ar.Receivable{
		{ID: "INV-5001", CustomerID: "CUST-P1", CustomerName: "Payer One",
			InvoiceDate: date("2025-05-01"), DueDate: date("2025-05-31"),
			OriginalAmount: 10000, OpenAmount: 4000, Currency: "USD", Status: ar.StatusPartiallyPaid, TermsDays: 30},
		{ID: "INV-5002", CustomerID: "CUST-P1", CustomerName: "Payer One",
			InvoiceDate: date("2025-06-01"), DueDate: date("2025-07-01"),
			OriginalAmount: 5000, OpenAmount: 0, Currency: "USD", Status: ar.StatusPaid, TermsDays: 30},
	}
}

// CustomerCredits returns a portfolio including credit memos, exercising
// both no-netting (default) and net-by-customer policy paths.
func CustomerCredits() []ar.Receivable {
	return []ar.Receivable{
		{ID: "INV-6001", CustomerID: "CUST-CR", CustomerName: "Credit Customer",
			InvoiceDate: date("2025-06-01"), DueDate: date("2025-07-01"),
			OriginalAmount: 8000, OpenAmount: 8000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
		{ID: "CM-6002", CustomerID: "CUST-CR", CustomerName: "Credit Customer",
			DocumentType: ar.DocumentTypeCreditMemo,
			InvoiceDate:  date("2025-06-10"), DueDate: date("2025-06-10"),
			OriginalAmount: -1500, OpenAmount: -1500, Currency: "USD", Status: ar.StatusOpen,
		},
	}
}

// MixedCurrency returns a portfolio spanning two currencies, used to
// exercise IssueMixedCurrency handling.
func MixedCurrency() []ar.Receivable {
	return []ar.Receivable{
		{ID: "INV-7001", CustomerID: "CUST-US", CustomerName: "US Co",
			InvoiceDate: date("2025-06-01"), DueDate: date("2025-07-01"),
			OriginalAmount: 5000, OpenAmount: 5000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
		{ID: "INV-7002", CustomerID: "CUST-EU", CustomerName: "EU Co",
			InvoiceDate: date("2025-06-01"), DueDate: date("2025-07-01"),
			OriginalAmount: 4500, OpenAmount: 4500, Currency: "EUR", Status: ar.StatusOpen, TermsDays: 30},
	}
}

// ZeroAR returns an empty receivables portfolio.
func ZeroAR() []ar.Receivable {
	return nil
}

// FutureDatedInvalidInvoice returns a portfolio containing one receivable
// whose InvoiceDate is after AsOfDate — an issue-worthy input.
func FutureDatedInvalidInvoice() []ar.Receivable {
	return []ar.Receivable{
		{ID: "INV-8001", CustomerID: "CUST-F", CustomerName: "Future Co",
			InvoiceDate: date("2025-08-01"), DueDate: date("2025-08-31"),
			OriginalAmount: 2000, OpenAmount: 2000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
	}
}

// DeterioratingSnapshots returns a two-snapshot sequence (plus a
// corresponding "current" receivables list) showing a portfolio's aging
// worsening over time — for migration/trend deterioration tests.
func DeterioratingSnapshots() (snapshots []ar.Snapshot, current []ar.Receivable) {
	snapshots = []ar.Snapshot{
		{
			AsOfDate: "2025-05-31",
			Receivables: []ar.Receivable{
				{ID: "INV-9001", CustomerID: "CUST-DET", CustomerName: "Deteriorating Co",
					InvoiceDate: date("2025-05-01"), DueDate: date("2025-05-31"),
					OriginalAmount: 5000, OpenAmount: 5000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
			},
		},
	}
	current = []ar.Receivable{
		{ID: "INV-9001", CustomerID: "CUST-DET", CustomerName: "Deteriorating Co",
			InvoiceDate: date("2025-05-01"), DueDate: date("2025-05-31"),
			OriginalAmount: 5000, OpenAmount: 5000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
	}
	return snapshots, current
}

// ImprovingSnapshots returns a two-state sequence (one snapshot plus a
// current list) where the receivable has been paid off between the two
// states — for migration/trend improvement tests.
func ImprovingSnapshots() (snapshots []ar.Snapshot, current []ar.Receivable) {
	snapshots = []ar.Snapshot{
		{
			AsOfDate: "2025-04-30",
			Receivables: []ar.Receivable{
				{ID: "INV-9101", CustomerID: "CUST-IMP", CustomerName: "Improving Co",
					InvoiceDate: date("2025-02-01"), DueDate: date("2025-03-03"),
					OriginalAmount: 6000, OpenAmount: 6000, Currency: "USD", Status: ar.StatusOpen, TermsDays: 30},
			},
		},
	}
	current = nil // fully paid; no longer open as of the current AsOfDate.
	return snapshots, current
}

// GLSubledgerMismatch returns a receivables portfolio paired with a GL
// control-account balance that deliberately does not match the subledger
// total.
func GLSubledgerMismatch() (receivables []ar.Receivable, controlBalance float64) {
	receivables = HealthyPortfolio()
	return receivables, 999999
}

// SalesHistoryForDSO returns a simple SalesPeriod series usable for DSO
// tests, paired with HealthyPortfolio's total open AR.
func SalesHistoryForDSO() []ar.SalesPeriod {
	return []ar.SalesPeriod{
		{Period: "2025-Q1", SalesAmount: 60000, Days: 90, Basis: ar.SalesBasisCreditSales, EndingAR: floatPtr(20000)},
		{Period: "2025-Q2", SalesAmount: 65000, Days: 91, Basis: ar.SalesBasisCreditSales, EndingAR: floatPtr(17500)},
	}
}

func floatPtr(v float64) *float64 { return &v }
