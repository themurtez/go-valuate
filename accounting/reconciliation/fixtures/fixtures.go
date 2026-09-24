// Package fixtures provides synthetic accounting/reconciliation data for
// tests: bank, credit-card, loan, and generic balance-only scenarios —
// task sections 61-64. Nothing here is real financial data; every figure
// is invented for illustration, mirroring every sibling accounting/*
// fixtures package's identical synthetic-data convention.
package fixtures

import (
	"github.com/themurtez/go-valuate/accounting/ledger"
	ledgerfixtures "github.com/themurtez/go-valuate/accounting/ledger/fixtures"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

// BankLedgerAndChart returns the shared ServiceBusiness ledger's chart
// and entries — the same ledger.Account "1000" (Cash) BankBookItems is
// built from via reconciliation.BookItemsFromLedger, so a caller
// exercising the ledger adapter can use exactly this chart/entries pair.
func BankLedgerAndChart() ([]ledger.JournalEntry, ledger.ChartOfAccounts) {
	entries := ledgerfixtures.ServiceBusinessEntries()
	chart := ledger.BuildChartOfAccounts(ledgerfixtures.ServiceBusinessChart())
	return entries, chart
}

// BankBookItems converts ServiceBusiness's Cash account (ID "1000")
// activity for January 2025 into BookItems via BookItemsFromLedger — the
// task section 61 "synthetic cash ledger" half of the bank-style
// fixture.
func BankBookItems() []reconciliation.BookItem {
	entries, chart := BankLedgerAndChart()
	return reconciliation.BookItemsFromLedger(entries, chart, reconciliation.LedgerBookItemsOptions{
		AccountID: "1000",
		StartDate: "2025-01-01",
		EndDate:   "2025-01-31",
	})
}

// BankExternalItems returns normalized bank-statement items designed to
// exercise every scenario task section 61 asks for against
// BankBookItems: exact matches (owner contribution, payroll, invoice
// collection), an outstanding check (rent payment not yet cleared by the
// bank), a deposit in transit (an invoice-collection deposit the bank
// has not yet posted), a bank fee with no book-side counterpart, a
// repeated ambiguous amount pair, and a stale unmatched item.
func BankExternalItems() []reconciliation.ExternalItem {
	return []reconciliation.ExternalItem{
		// Exact match: owner capital contribution (matches SVC-JE-1/1000).
		{ItemID: "STMT-1", Date: "2025-01-06", Amount: 50000, Direction: reconciliation.DirectionInflow, Reference: "DEP-CAPITAL", Description: "Deposit"},
		// Exact match: payroll withdrawal (matches SVC-JE-3/1000).
		{ItemID: "STMT-2", Date: "2025-01-15", Amount: 8000, Direction: reconciliation.DirectionOutflow, Reference: "PAYROLL-JAN", Description: "Payroll debit"},
		// Deposit in transit: invoice collection posted on the books
		// 2025-01-25 but not yet shown on this (earlier-dated) statement
		// cutoff — deliberately absent from external items; see
		// BankReconcilingItems' DEPOSIT_IN_TRANSIT entry, which explains it.
		//
		// Outstanding check: rent payment (SVC-JE-4/1000, $3000) is
		// deliberately absent from external items — see
		// BankReconcilingItems' OUTSTANDING_CHECK entry.
		//
		// Unrecorded bank fee — no book-side counterpart at all.
		{ItemID: "STMT-3", Date: "2025-01-31", Amount: 35, Direction: reconciliation.DirectionOutflow, Reference: "FEE-MONTHLY", Description: "Monthly service fee"},
		// Repeated ambiguous amount pair: two same-day, same-amount,
		// no-reference debits with no book-side counterpart at all
		// (illustrating task section 46's "insufficient evidence" case
		// via items that simply have no match candidate rather than a
		// false pairing).
		{ItemID: "STMT-4", Date: "2025-01-28", Amount: 25, Direction: reconciliation.DirectionOutflow, Description: "Card purchase"},
		{ItemID: "STMT-5", Date: "2025-01-28", Amount: 25, Direction: reconciliation.DirectionOutflow, Description: "Card purchase"},
		// Stale unmatched item: dated near month-start, still unmatched as
		// of a much-later AsOfDate — see BankAsOfDateStale.
		{ItemID: "STMT-6", Date: "2025-01-02", Amount: 12, Direction: reconciliation.DirectionOutflow, Description: "ATM fee"},
	}
}

// BankReconcilingItems returns the explicit reconciling items explaining
// BankBookItems/BankExternalItems' remaining differences — task section
// 61.
func BankReconcilingItems() []reconciliation.ReconcilingItem {
	return []reconciliation.ReconcilingItem{
		// Classic bank-rec convention: outstanding checks/deposits in
		// transit adjust the EXTERNAL (bank statement) side toward the
		// books, since the bank has not yet processed them.
		{ItemID: "RECON-1", Type: reconciliation.ReconcilingOutstandingCheck, Side: reconciliation.ReconcilingSideExternal,
			Amount: -3000, Date: "2025-01-20", Description: "Rent check not yet cleared by the bank", RelatedItemID: "SVC-JE-4/L2"},
		{ItemID: "RECON-2", Type: reconciliation.ReconcilingDepositInTransit, Side: reconciliation.ReconcilingSideExternal,
			Amount: 12000, Date: "2025-01-25", Description: "Invoice collection deposit not yet shown on the statement", RelatedItemID: "SVC-JE-5/L1"},
		// The bank fee (STMT-3) is already reflected in ExternalItems'
		// activity total; it adjusts the BOOK side toward the bank since
		// it has not yet been recorded on the books.
		{ItemID: "RECON-3", Type: reconciliation.ReconcilingUnrecordedFee, Side: reconciliation.ReconcilingSideBook,
			Amount: -35, Date: "2025-01-31", Description: "Bank fee not yet recorded on the books", RelatedItemID: "STMT-3"},
	}
}

// BankAsOfDate is a deliberately later-than-fixture AsOfDate, chosen so
// STMT-6 (dated 2025-01-02) reads as clearly stale under a modest
// stale-days threshold.
const BankAsOfDate = "2025-03-01"

// BankInput assembles the full bank-reconciliation Input.
func BankInput() reconciliation.Input {
	book := 50000.0 + 12000.0 - 8000.0 - 3000.0 + 12000.0 - 12000.0 // matches ServiceBusiness's ending Cash balance
	external := 50000.0 - 8000.0 - 35 - 25 - 25 - 12
	return reconciliation.Input{
		AccountID:         "1000",
		ExternalAccountID: "BANK-ACCT-001",
		Period:            "2025-01",
		AsOfDate:          BankAsOfDate,
		Type:              reconciliation.TypeBank,
		BookItems:         BankBookItems(),
		ExternalItems:     BankExternalItems(),
		BookBalance:       reconciliation.BalanceInput{EndingBalance: &book},
		ExternalBalance:   reconciliation.BalanceInput{EndingBalance: &external},
		ReconcilingItems:  BankReconcilingItems(),
		Policy: reconciliation.MatchingPolicy{
			AmountTolerance:        0.01,
			DateWindowDays:         5,
			ReferenceNormalization: reconciliation.ReferenceNormalization{Trim: true, CaseFold: true},
			StaleDaysThreshold:     30,
			Materiality:            reconciliation.MaterialityPolicy{AbsoluteAmount: 100},
		},
	}
}

// --- Credit card (task section 62) ---

// CreditCardInput builds a liability-oriented (REVERSED) credit-card
// reconciliation: purchases, a payment, a statement fee, a repeated
// amount, and a caller-confirmed manual match.
func CreditCardInput() reconciliation.Input {
	// Book side records each transaction from the CARDHOLDER's cash
	// perspective: a purchase is an OUTFLOW-equivalent (increases what is
	// owed), a payment to the issuer is an INFLOW-equivalent (reduces
	// what is owed, mirroring cash "returning" toward the book's own
	// zero-balance frame). Under Policy.Orientation = REVERSED, the
	// external (card issuer statement) side's Direction is recorded from
	// the ISSUER's own AR perspective instead — the exact opposite of the
	// book's framing for the same economic event — so a purchase (which
	// increases what the issuer is owed, from the issuer's perspective)
	// is DirectionInflow here, and a payment (which reduces it) is
	// DirectionOutflow. OrientedSignedAmount then flips the external
	// side's sign once more, landing both sides on the same signed
	// figure for comparison — see the package doc comment's
	// "Orientation" section.
	book := []reconciliation.BookItem{
		{ItemID: "CC-B1", Date: "2025-02-03", Amount: 120, Direction: reconciliation.DirectionOutflow, Description: "Office supplies purchase"},
		{ItemID: "CC-B2", Date: "2025-02-10", Amount: 300, Direction: reconciliation.DirectionInflow, Reference: "PMT-FEB", Description: "Payment to card issuer"},
		{ItemID: "CC-B3", Date: "2025-02-15", Amount: 45, Direction: reconciliation.DirectionOutflow, Description: "Software subscription"},
		{ItemID: "CC-B4", Date: "2025-02-20", Amount: 60, Direction: reconciliation.DirectionOutflow, Description: "Repeated vendor charge"},
		{ItemID: "CC-B5", Date: "2025-02-20", Amount: 60, Direction: reconciliation.DirectionOutflow, Description: "Repeated vendor charge"},
	}
	external := []reconciliation.ExternalItem{
		{ItemID: "CC-E1", Date: "2025-02-03", Amount: 120, Direction: reconciliation.DirectionInflow, Description: "Office supplies purchase"},
		{ItemID: "CC-E2", Date: "2025-02-11", Amount: 300, Direction: reconciliation.DirectionOutflow, Reference: "pmt-feb", Description: "Payment received"},
		{ItemID: "CC-E3", Date: "2025-02-16", Amount: 45, Direction: reconciliation.DirectionInflow, Description: "Software subscription"},
		{ItemID: "CC-E4", Date: "2025-02-21", Amount: 60, Direction: reconciliation.DirectionInflow, Description: "Repeated vendor charge"},
		{ItemID: "CC-E5", Date: "2025-02-21", Amount: 60, Direction: reconciliation.DirectionInflow, Description: "Repeated vendor charge"},
		{ItemID: "CC-E6", Date: "2025-02-28", Amount: 15, Direction: reconciliation.DirectionInflow, Description: "Annual fee"},
	}
	confirmed := []reconciliation.ConfirmedMatch{
		// Manual match: caller has already confirmed CC-B3 <-> CC-E3
		// despite the 1-day date difference (illustrating task section
		// 12/81's confirmed-match precedence — this pairing is unaffected
		// by whether auto-matching would also have found it).
		{MatchID: "MANUAL-CC-1", BookItemIDs: []string{"CC-B3"}, ExternalItemIDs: []string{"CC-E3"}, Note: "Confirmed by controller"},
	}
	return reconciliation.Input{
		AccountID:         "CC-CONTROL",
		ExternalAccountID: "CARD-ACCT-9001",
		Period:            "2025-02",
		AsOfDate:          "2025-03-05",
		Type:              reconciliation.TypeCreditCard,
		BookItems:         book,
		ExternalItems:     external,
		ConfirmedMatches:  confirmed,
		Policy: reconciliation.MatchingPolicy{
			AmountTolerance:        0.01,
			DateWindowDays:         3,
			ReferenceNormalization: reconciliation.ReferenceNormalization{Trim: true, CaseFold: true},
			Orientation:            reconciliation.OrientationReversed,
			Materiality:            reconciliation.MaterialityPolicy{AbsoluteAmount: 50},
		},
	}
}

// --- Loan (task section 63) ---

// LoanInput builds a generic loan-balance reconciliation: opening
// balance, a principal payment, a fee/interest external item, and an
// ending statement balance that deliberately does not tie (illustrating
// a genuine mismatch, not an amortization schedule).
func LoanInput() reconciliation.Input {
	// A loan is a liability: a principal payment reduces what is owed
	// (DirectionOutflow, decreasing the balance), while an interest/fee
	// charge increases it (DirectionInflow, per this package's raw
	// signed convention — no Orientation flip is needed here since both
	// sides already describe the SAME loan-balance frame, unlike the
	// credit-card fixture's issuer-vs-cardholder framing difference).
	opening := 100000.0
	bookEnding := 98000.0      // opening - 2000 principal payment
	statementEnding := 98150.0 // opening - 2000 principal + 150 fee/interest, per the lender's own statement

	book := []reconciliation.BookItem{
		{ItemID: "LOAN-B1", Date: "2025-03-05", Amount: 2000, Direction: reconciliation.DirectionOutflow, Description: "Principal payment"},
	}
	external := []reconciliation.ExternalItem{
		{ItemID: "LOAN-E1", Date: "2025-03-05", Amount: 2000, Direction: reconciliation.DirectionOutflow, Description: "Principal payment"},
		{ItemID: "LOAN-E2", Date: "2025-03-31", Amount: 150, Direction: reconciliation.DirectionInflow, Description: "Interest and fee charge"},
	}

	return reconciliation.Input{
		AccountID:         "LOAN-BOOK",
		ExternalAccountID: "LENDER-LOAN-4477",
		Period:            "2025-03",
		AsOfDate:          "2025-04-05",
		Type:              reconciliation.TypeLoan,
		BookItems:         book,
		ExternalItems:     external,
		BookBalance:       reconciliation.BalanceInput{OpeningBalance: &opening, EndingBalance: &bookEnding},
		ExternalBalance:   reconciliation.BalanceInput{OpeningBalance: &opening, EndingBalance: &statementEnding},
		Policy: reconciliation.MatchingPolicy{
			AmountTolerance: 0.01,
			DateWindowDays:  3,
			DeriveBalances:  true,
			Materiality:     reconciliation.MaterialityPolicy{AbsoluteAmount: 500},
		},
	}
}

// --- Generic balance-only scenarios (task section 64) ---

// InventoryControlBalanceOnlyInput builds a balance-only inventory
// control-account reconciliation: no transaction-level items, just
// subledger vs GL ending balances.
func InventoryControlBalanceOnlyInput() reconciliation.Input {
	subledger := 250000.0
	gl := 250000.0
	return reconciliation.Input{
		AccountID:         "INV-SUBLEDGER",
		ExternalAccountID: "GL-1300-INVENTORY",
		Period:            "2025-01",
		AsOfDate:          "2025-02-01",
		Type:              reconciliation.TypeInventoryControl,
		BookBalance:       reconciliation.BalanceInput{EndingBalance: &subledger},
		ExternalBalance:   reconciliation.BalanceInput{EndingBalance: &gl},
		Policy:            reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	}
}

// PayrollClearingBalanceOnlyInput builds a balance-only payroll-clearing
// reconciliation with a deliberate small unreconciled difference.
func PayrollClearingBalanceOnlyInput() reconciliation.Input {
	control := 45000.0
	clearing := 44980.0
	return reconciliation.Input{
		AccountID:         "PAYROLL-CONTROL",
		ExternalAccountID: "GL-2100-PAYROLL-CLEARING",
		Period:            "2025-01",
		AsOfDate:          "2025-02-01",
		Type:              reconciliation.TypePayrollClearing,
		BookBalance:       reconciliation.BalanceInput{EndingBalance: &control},
		ExternalBalance:   reconciliation.BalanceInput{EndingBalance: &clearing},
		Policy:            reconciliation.MatchingPolicy{AmountTolerance: 0.01, Materiality: reconciliation.MaterialityPolicy{AbsoluteAmount: 100}},
	}
}

// IntercompanyBalanceOnlyInput builds a balance-only intercompany
// reconciliation with an explicit REVERSED orientation: Entity A's books
// show a $75,000 receivable-from-B (a positive asset balance), while
// Entity B's own books show the reciprocal $75,000 as a payable-to-A —
// naturally the opposite sign in B's own debit-positive convention.
// REVERSED orientation is what lets these two, correctly-oriented-per-
// their-own-books figures compare as economically identical rather than
// as a $150,000 discrepancy.
func IntercompanyBalanceOnlyInput() reconciliation.Input {
	entityABalance := 75000.0
	entityBReciprocal := -75000.0
	return reconciliation.Input{
		AccountID:         "INTERCO-A-DUE-FROM-B",
		ExternalAccountID: "INTERCO-B-DUE-TO-A",
		Period:            "2025-01",
		AsOfDate:          "2025-02-01",
		Type:              reconciliation.TypeIntercompany,
		BookBalance:       reconciliation.BalanceInput{EndingBalance: &entityABalance},
		ExternalBalance:   reconciliation.BalanceInput{EndingBalance: &entityBReciprocal},
		Policy:            reconciliation.MatchingPolicy{AmountTolerance: 0.01, Orientation: reconciliation.OrientationReversed},
	}
}
