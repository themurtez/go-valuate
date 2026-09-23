// Package fixtures provides synthetic accounting/closequality data for
// tests: a full ledger -> statements -> AR -> AP -> journal diagnostics
// chain built on the ServiceBusiness scenario shared by
// accounting/ledger/fixtures and accounting/statements/fixtures, plus
// standalone AR/AP/reconciliation/close-task scenarios for close-quality
// scenarios that do not need the full chain. Nothing here is real
// financial data.
package fixtures

import (
	"time"

	"github.com/themurtez/go-valuate/accounting/ap"
	apfixtures "github.com/themurtez/go-valuate/accounting/ap/fixtures"
	"github.com/themurtez/go-valuate/accounting/ar"
	arfixtures "github.com/themurtez/go-valuate/accounting/ar/fixtures"
	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/ledger"
	ledgerfixtures "github.com/themurtez/go-valuate/accounting/ledger/fixtures"
	"github.com/themurtez/go-valuate/accounting/statements"
	statementsfixtures "github.com/themurtez/go-valuate/accounting/statements/fixtures"
	"github.com/themurtez/go-valuate/financial"
)

// Period returns the PeriodInfo used by every fixture below: the
// ServiceBusiness ledger scenario's month, 2025-01, closed shortly
// after month end.
func Period() closequality.PeriodInfo {
	return closequality.PeriodInfo{
		Period:    "2025-01",
		StartDate: "2025-01-01",
		EndDate:   "2025-01-31",
		CloseDate: "2025-02-03",
		Lock:      closequality.PeriodClosed,
	}
}

// Ledger returns the shared ServiceBusiness ledger, with one added
// month-end closing entry rolling net income (revenue 12000 minus
// expenses 11000 = 1000) into Retained Earnings — without it the
// balance sheet would not balance, since ServiceBusinessEntries alone
// never closes income-statement accounts into equity. This mirrors a
// genuinely closed period, which is what a "clean close" fixture needs
// to represent.
func Ledger() ledger.Ledger {
	entries := ledgerfixtures.ServiceBusinessEntries()
	entries = append(entries, closingEntry())
	return ledger.Ledger{
		Accounts: ledgerfixtures.ServiceBusinessChart(),
		Entries:  entries,
	}
}

func closingEntry() ledger.JournalEntry {
	return ledger.JournalEntry{
		ID: "SVC-JE-CLOSE", Date: "2025-01-31", Period: "2025-01", Status: ledger.StatusPosted,
		Description: "Month-end close: net income to retained earnings", Source: "closing",
		Lines: []ledger.JournalLine{
			{ID: "L1", AccountID: "4000", Debit: 12000},
			{ID: "L2", AccountID: "6000", Credit: 8000},
			{ID: "L3", AccountID: "6100", Credit: 3000},
			{ID: "L4", AccountID: "3900", Credit: 1000},
		},
	}
}

// StatementsResult builds statements.Result from the ServiceBusiness
// ledger and its corresponding mappings.
func StatementsResult() statements.Result {
	l := Ledger()
	return statements.Build(statements.Input{
		Source:    statements.SourceLedger,
		Chart:     l.Accounts,
		Entries:   l.Entries,
		Periods:   []financial.Period{"2025-01"},
		Mappings:  statementsfixtures.ServiceBusinessMappings(),
		Selection: statements.SelectionBoth,
	}, statements.Options{})
}

// asOf is the AR/AP analysis date shared by every closequality fixture.
func asOf() time.Time { return date("2025-01-31") }

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// ARResultClean returns an ar.Result with a control-account balance that
// matches the subledger exactly.
func ARResultClean() ar.Result {
	receivables := arfixtures.HealthyPortfolio()
	total := 0.0
	for _, r := range receivables {
		total += r.OpenAmount
	}
	return ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{
		AsOfDate:              asOf(),
		ControlAccountBalance: floatPtr(total),
	})
}

// ARResultControlMismatch returns an ar.Result whose control-account
// balance deliberately does not match the subledger.
func ARResultControlMismatch() ar.Result {
	receivables, controlBalance := arfixtures.GLSubledgerMismatch()
	return ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{
		AsOfDate:              asOf(),
		ControlAccountBalance: floatPtr(controlBalance),
	})
}

// APResultClean returns an ap.Result with a control-account balance that
// matches the subledger exactly.
func APResultClean() ap.Result {
	payables := apfixtures.HealthyPayables()
	total := 0.0
	for _, p := range payables {
		total += p.OpenAmount
	}
	return ap.Calculate(ap.Input{Payables: payables}, ap.Options{
		AsOfDate:              asOf(),
		ControlAccountBalance: floatPtr(total),
	})
}

// APResultControlMismatch returns an ap.Result whose control-account
// balance deliberately does not match the subledger.
func APResultControlMismatch() ap.Result {
	payables, controlBalance := apfixtures.GLSubledgerMismatch()
	return ap.Calculate(ap.Input{Payables: payables}, ap.Options{
		AsOfDate:              asOf(),
		ControlAccountBalance: floatPtr(controlBalance),
	})
}

// JournalDiagnosticsWindow is the PeriodWindow matching Period() above.
func JournalDiagnosticsWindow() journaldiagnostics.PeriodWindow {
	closeDate := "2025-02-03"
	return journaldiagnostics.PeriodWindow{
		Period: "2025-01", StartDate: "2025-01-01", EndDate: "2025-01-31", CloseDate: &closeDate,
	}
}

// JournalDiagnosticsResultClean runs journaldiagnostics over the
// ServiceBusiness ledger with default policy and no metadata — a
// realistic "clean, ordinary activity" result.
func JournalDiagnosticsResultClean() journaldiagnostics.Result {
	l := Ledger()
	return journaldiagnostics.Calculate(l, nil, JournalDiagnosticsWindow(), journaldiagnostics.DefaultPolicy())
}

// JournalDiagnosticsResultWithPostCloseEntry returns a journal
// diagnostics result over a ledger with one material entry posted after
// the close date.
func JournalDiagnosticsResultWithPostCloseEntry() journaldiagnostics.Result {
	l := Ledger()
	l.Entries = append(l.Entries, ledger.JournalEntry{
		ID: "SVC-JE-POSTCLOSE", Date: "2025-01-31", Period: "2025-01", Status: ledger.StatusPosted,
		Description: "Late adjusting entry", Source: "manual",
		Lines: []ledger.JournalLine{
			{ID: "L1", AccountID: "6000", Debit: 15000},
			{ID: "L2", AccountID: "1000", Credit: 15000},
		},
	})
	postedAt := date("2025-02-10")
	metadata := []journaldiagnostics.EntryMetadata{
		{EntryID: "SVC-JE-POSTCLOSE", Source: journaldiagnostics.SourceManual, PostedAt: &postedAt},
	}
	policy := journaldiagnostics.DefaultPolicy()
	policy.MaterialAmount = 1000
	return journaldiagnostics.Calculate(l, metadata, JournalDiagnosticsWindow(), policy)
}

func floatPtr(v float64) *float64 { return &v }

// CleanCloseInput assembles a fully wired, clean Input across every
// upstream module: ledger, statements, AR, AP, and journal diagnostics
// all agree, with critical accounts reconciled and required close tasks
// completed. Calculate(CleanCloseInput(), CleanPolicy()) is asserted to
// be READY in the integration test.
func CleanCloseInput() closequality.Input {
	l := Ledger()
	return closequality.Input{
		Period:             Period(),
		Ledger:             l,
		LedgerProvided:     true,
		Statements:         StatementsResult(),
		AR:                 ARResultClean(),
		AP:                 APResultClean(),
		JournalDiagnostics: JournalDiagnosticsResultClean(),
		Reconciliations:    CleanReconciliations(),
		CloseTasks:         CompletedCloseTasks(),
	}
}

// CleanPolicy is the Policy paired with CleanCloseInput.
func CleanPolicy() closequality.Policy {
	p := closequality.DefaultPolicy()
	p.CriticalAccounts = []string{"1000"}
	p.RequiredReconciliationAccounts = []string{"1000"}
	p.Applicability = closequality.Applicability{
		ARRequired:                 true,
		APRequired:                 true,
		StatementsRequired:         true,
		JournalDiagnosticsRequired: true,
		ReconciliationsRequired:    true,
		CloseTasksRequired:         true,
	}
	return p
}

// CleanReconciliations returns a reconciled cash-account status.
func CleanReconciliations() []closequality.ReconciliationStatus {
	return []closequality.ReconciliationStatus{
		{
			AccountID:         "1000",
			Status:            closequality.ReconciliationReconciled,
			AsOfDate:          "2025-01-31",
			BookBalance:       closequality.AvailableValue(50000),
			ReconciledBalance: closequality.AvailableValue(50000),
			Difference:        closequality.AvailableValue(0),
		},
	}
}

// UnreconciledCashStatus returns an unreconciled status for the same
// critical cash account CleanReconciliations reconciles.
func UnreconciledCashStatus() []closequality.ReconciliationStatus {
	return []closequality.ReconciliationStatus{
		{
			AccountID:  "1000",
			Status:     closequality.ReconciliationUnreconciled,
			AsOfDate:   "2025-01-31",
			Difference: closequality.AvailableValue(1200),
		},
	}
}

// CompletedCloseTasks returns a required close task in COMPLETED status.
func CompletedCloseTasks() []closequality.CloseTaskStatus {
	return []closequality.CloseTaskStatus{
		{Code: "bank-reconciliation", Description: "Complete bank reconciliation", Required: true, Status: closequality.CloseTaskCompleted},
		{Code: "review-financials", Description: "Controller review of financials", Required: false, Status: closequality.CloseTaskNotStarted},
	}
}

// IncompleteCloseTasks returns a required close task still NOT_STARTED.
func IncompleteCloseTasks() []closequality.CloseTaskStatus {
	return []closequality.CloseTaskStatus{
		{Code: "bank-reconciliation", Description: "Complete bank reconciliation", Required: true, Status: closequality.CloseTaskNotStarted},
	}
}

// BlockedCloseTasks returns a required close task in BLOCKED status.
func BlockedCloseTasks() []closequality.CloseTaskStatus {
	return []closequality.CloseTaskStatus{
		{Code: "bank-reconciliation", Description: "Complete bank reconciliation", Required: true, Status: closequality.CloseTaskBlocked},
	}
}

// SuspenseAccountExpectations returns an AccountExpectation declaring
// account "2100" (Accrued Payroll, repurposed here as a suspense
// account for fixture purposes) as a clearing account that must zero
// out by period end.
func SuspenseAccountExpectations() []closequality.AccountExpectation {
	materiality := 1.0
	return []closequality.AccountExpectation{
		{AccountID: "2100", ShouldClear: true, Label: "Suspense clearing account", Materiality: &materiality},
	}
}

// StaleSuspenseBalanceAge returns BalanceAge metadata making the
// suspense account's balance appear 120 days old.
func StaleSuspenseBalanceAge() []closequality.BalanceAge {
	return []closequality.BalanceAge{
		{AccountID: "2100", OldestOpenDate: "2024-10-01", Amount: 500},
	}
}

// UnbalancedLedger returns a Ledger built from
// ledgerfixtures.UnbalancedJournalEntries paired with ServiceBusinessChart.
func UnbalancedLedger() ledger.Ledger {
	return ledger.Ledger{
		Accounts: ledgerfixtures.ServiceBusinessChart(),
		Entries:  ledgerfixtures.UnbalancedJournalEntries(),
	}
}

// UnmappedMaterialStatementsResult builds a statements.Result with a
// material unmapped account, using statementsfixtures.UnmappedMaterialAccountMappings
// (which omits the mapping for a material account).
func UnmappedMaterialStatementsResult() statements.Result {
	l := Ledger()
	mappings := statementsfixtures.UnmappedMaterialAccountMappings()
	return statements.Build(statements.Input{
		Source:    statements.SourceLedger,
		Chart:     l.Accounts,
		Entries:   l.Entries,
		Periods:   []financial.Period{"2025-01"},
		Mappings:  mappings,
		Selection: statements.SelectionBoth,
	}, statements.Options{})
}
