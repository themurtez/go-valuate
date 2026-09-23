// Package fixtures provides synthetic accounting/journaldiagnostics data
// for tests: a small chart of accounts plus journal entries and
// EntryMetadata deliberately constructed to exercise this package's
// diagnostic rules — one scenario per documented rule family. Nothing here
// is real financial data.
package fixtures

import (
	"fmt"
	"time"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

// Chart returns a small chart of accounts covering every AccountType,
// sized to give the rare/new-account and account-relative baseline rules
// something to compare against.
func Chart() []ledger.Account {
	return []ledger.Account{
		{ID: "1000", Number: "1000", Name: "Cash", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1100", Number: "1100", Name: "Accounts Receivable", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "1500", Number: "1500", Name: "Suspense Account", Type: ledger.AccountAsset, Currency: "USD", Active: true},
		{ID: "2000", Number: "2000", Name: "Accounts Payable", Type: ledger.AccountLiability, Currency: "USD", Active: true},
		{ID: "3000", Number: "3000", Name: "Owner's Equity", Type: ledger.AccountEquity, Currency: "USD", Active: true},
		{ID: "3900", Number: "3900", Name: "Retained Earnings", Type: ledger.AccountEquity, Currency: "USD", Active: true},
		{ID: "4000", Number: "4000", Name: "Consulting Revenue", Type: ledger.AccountRevenue, Currency: "USD", Active: true},
		{ID: "6000", Number: "6000", Name: "Salaries Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
		{ID: "6100", Number: "6100", Name: "Rent Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
		{ID: "6200", Number: "6200", Name: "Office Supplies Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
		{ID: "6900", Number: "6900", Name: "Rarely Used Misc Expense", Type: ledger.AccountExpense, Currency: "USD", Active: true},
		{ID: "6999", Number: "6999", Name: "Brand New Expense Account", Type: ledger.AccountExpense, Currency: "USD", Active: true},
	}
}

func line(id, account string, debit, credit float64) ledger.JournalLine {
	return ledger.JournalLine{ID: id, AccountID: account, Debit: debit, Credit: credit}
}

func posted(id, date, period, desc string, lines ...ledger.JournalLine) ledger.JournalEntry {
	return ledger.JournalEntry{ID: id, Date: date, Period: period, Description: desc, Status: ledger.StatusPosted, Lines: lines}
}

// Window is the standard PeriodWindow these fixtures are built against: a
// March 2025 monthly close, with an explicit CloseDate a few days into
// April.
func Window() journaldiagnostics.PeriodWindow {
	closeDate := "2025-04-03"
	return journaldiagnostics.PeriodWindow{
		Period: "2025-03", StartDate: "2025-03-01", EndDate: "2025-03-31", CloseDate: &closeDate,
	}
}

func ts(date string, hour int) *time.Time {
	t, _ := time.ParseInLocation("2006-01-02 15:04", fmt.Sprintf("%s %02d:00", date, hour), time.UTC)
	return &t
}

// Entries returns a synthetic set of journal entries spanning ordinary
// clean activity plus one scenario per documented rule family — see the
// package doc comment.
func Entries() []ledger.JournalEntry {
	return []ledger.JournalEntry{
		// Ordinary clean recurring activity, giving several accounts a
		// baseline history for the MAD/rare-account rules to compare
		// against.
		posted("JE-001", "2025-03-03", "2025-03", "March rent payment", line("L1", "6100", 3000, 0), line("L2", "1000", 0, 3000)),
		posted("JE-002", "2025-03-05", "2025-03", "Consulting invoice #501", line("L1", "1100", 15000, 0), line("L2", "4000", 0, 15000)),
		posted("JE-003", "2025-03-07", "2025-03", "March payroll", line("L1", "6000", 22000, 0), line("L2", "1000", 0, 22000)),
		posted("JE-004", "2025-03-10", "2025-03", "Office supplies", line("L1", "6200", 450, 0), line("L2", "1000", 0, 450)),
		posted("JE-005", "2025-03-12", "2025-03", "Consulting invoice #502", line("L1", "1100", 18000, 0), line("L2", "4000", 0, 18000)),
		posted("JE-006", "2025-03-14", "2025-03", "Office supplies", line("L1", "6200", 380, 0), line("L2", "1000", 0, 380)),
		posted("JE-007", "2025-03-17", "2025-03", "Cash collected on invoice #501", line("L1", "1000", 15000, 0), line("L2", "1100", 0, 15000)),

		// Large manual entry — material, absolute and MAD-relative outlier
		// against 6200's small baseline above.
		posted("JE-LARGE-01", "2025-03-11", "2025-03", "Manual adjustment to office supplies", line("L1", "6200", 50000, 0), line("L2", "1000", 0, 50000)),

		// Period-end adjusting entry (within 3 days of month-end).
		posted("JE-PE-01", "2025-03-30", "2025-03", "Period-end accrual adjustment", line("L1", "6000", 5000, 0), line("L2", "2000", 0, 5000)),

		// Post-close entry: effective date within March, but metadata
		// (see Metadata) posts it in April after CloseDate.
		posted("JE-POSTCLOSE-01", "2025-03-28", "2025-03", "Late adjustment posted after close", line("L1", "6100", 1200, 0), line("L2", "1000", 0, 1200)),

		// Weekend entry: 2025-03-15 is a Saturday.
		posted("JE-WKND-01", "2025-03-15", "2025-03", "Weekend correction entry", line("L1", "6200", 600, 0), line("L2", "1000", 0, 600)),

		// Outside-business-hours entry (metadata timestamp at 23:00 UTC).
		posted("JE-AFTERHRS-01", "2025-03-18", "2025-03", "Late night correction", line("L1", "6200", 700, 0), line("L2", "1000", 0, 700)),

		// Round-dollar activity.
		posted("JE-ROUND-01", "2025-03-19", "2025-03", "Round dollar adjustment", line("L1", "6200", 10000, 0), line("L2", "1000", 0, 10000)),

		// Exact duplicate pair (same date, same lines).
		posted("JE-DUP-01", "2025-03-20", "2025-03", "Vendor payment", line("L1", "2000", 2500, 0), line("L2", "1000", 0, 2500)),
		posted("JE-DUP-02", "2025-03-20", "2025-03", "Vendor payment", line("L1", "2000", 2500, 0), line("L2", "1000", 0, 2500)),

		// Possible duplicate within date window (same content, 2 days
		// apart).
		posted("JE-NEARDUP-01", "2025-03-21", "2025-03", "Supplies reimbursement", line("L1", "6200", 900, 0), line("L2", "1000", 0, 900)),
		posted("JE-NEARDUP-02", "2025-03-23", "2025-03", "Supplies reimbursement", line("L1", "6200", 900, 0), line("L2", "1000", 0, 900)),

		// Repeated identical amounts across unrelated entries.
		posted("JE-REPEAT-01", "2025-03-04", "2025-03", "Miscellaneous expense", line("L1", "6900", 1234, 0), line("L2", "1000", 0, 1234)),
		posted("JE-REPEAT-02", "2025-03-09", "2025-03", "Miscellaneous expense", line("L1", "6900", 1234, 0), line("L2", "1000", 0, 1234)),
		posted("JE-REPEAT-03", "2025-03-16", "2025-03", "Miscellaneous expense", line("L1", "6900", 1234, 0), line("L2", "1000", 0, 1234)),

		// Rare account: 6900 has 3 entries above (JE-REPEAT-*), just at the
		// rare-account boundary for a 4th material posting.
		posted("JE-RAREACCT-01", "2025-03-24", "2025-03", "Misc expense adjustment", line("L1", "6900", 8000, 0), line("L2", "1000", 0, 8000)),

		// New account: 6999 has no other activity anywhere in this fixture
		// set.
		posted("JE-NEWACCT-01", "2025-03-25", "2025-03", "First use of new expense account", line("L1", "6999", 9000, 0), line("L2", "1000", 0, 9000)),

		// Opposite-normal-balance movement: revenue account debited
		// (credit memo without status flag), not declared as a reversal.
		posted("JE-OPPOSITE-01", "2025-03-26", "2025-03", "Revenue correction", line("L1", "4000", 4000, 0), line("L2", "1100", 0, 4000)),

		// Manual entry directly affecting equity, near period end.
		posted("JE-EQUITY-01", "2025-03-29", "2025-03", "Manual equity adjustment", line("L1", "3900", 6000, 0), line("L2", "1000", 0, 6000)),

		// Explicit rapid reversal: original + reversal 1 day apart.
		withReversal(posted("JE-REV-ORIG-01", "2025-03-06", "2025-03", "Entry later reversed", line("L1", "6200", 2000, 0), line("L2", "1000", 0, 2000)), "", "JE-REV-REV-01"),
		withReversal(posted("JE-REV-REV-01", "2025-03-07", "2025-03", "Reversal of JE-REV-ORIG-01", line("L1", "1000", 2000, 0), line("L2", "6200", 0, 2000)), "JE-REV-ORIG-01", ""),

		// Period-end entry with early reversal: material, posted near
		// period end, reversed shortly after.
		withReversal(posted("JE-PEER-ORIG-01", "2025-03-31", "2025-03", "Period-end entry reversed early next period", line("L1", "6000", 12000, 0), line("L2", "2000", 0, 12000)), "", "JE-PEER-REV-01"),
		withReversal(posted("JE-PEER-REV-01", "2025-04-01", "2025-04", "Reversal of JE-PEER-ORIG-01", line("L1", "2000", 12000, 0), line("L2", "6000", 0, 12000)), "JE-PEER-ORIG-01", ""),

		// Threshold-clustering: several entries just under a $10,000
		// approval threshold, same date.
		posted("JE-THRESH-01", "2025-03-22", "2025-03", "Vendor payment near threshold", line("L1", "2000", 9500, 0), line("L2", "1000", 0, 9500)),
		posted("JE-THRESH-02", "2025-03-22", "2025-03", "Vendor payment near threshold", line("L1", "2000", 9600, 0), line("L2", "1000", 0, 9600)),

		// Split-entry clustering: two same-date, same-account-pattern
		// entries individually below a threshold but combining to meet it.
		posted("JE-SPLIT-01", "2025-03-27", "2025-03", "Vendor payment part 1", line("L1", "2000", 6000, 0), line("L2", "1000", 0, 6000)),
		posted("JE-SPLIT-02", "2025-03-27", "2025-03", "Vendor payment part 2", line("L1", "2000", 6000, 0), line("L2", "1000", 0, 6000)),

		// Missing/generic descriptions.
		posted("JE-BLANK-01", "2025-03-08", "2025-03", "", line("L1", "6200", 300, 0), line("L2", "1000", 0, 300)),
		posted("JE-GENERIC-01", "2025-03-13", "2025-03", "Adjustment", line("L1", "6200", 320, 0), line("L2", "1000", 0, 320)),

		// Same preparer/approver (see Metadata) — content itself is
		// unremarkable.
		posted("JE-SAMEPA-01", "2025-03-15", "2025-03", "Vendor bill", line("L1", "2000", 1500, 0), line("L2", "1000", 0, 1500)),

		// Recurring system-generated journal, suppressed from
		// repeated-amount detection via metadata.
		posted("JE-RECUR-01", "2025-03-01", "2025-03", "Recurring software subscription", line("L1", "6200", 500, 0), line("L2", "1000", 0, 500)),
		posted("JE-RECUR-02", "2025-03-01", "2025-03", "Recurring software subscription", line("L1", "6200", 500, 0), line("L2", "1000", 0, 500)),
		posted("JE-RECUR-03", "2025-03-01", "2025-03", "Recurring software subscription", line("L1", "6200", 500, 0), line("L2", "1000", 0, 500)),

		// Insufficient-history scenario: an isolated small entry on a
		// low-activity account (6900 has only 4 entries total in this
		// fixture set — below the default MinBaselineObservations of 5).
		posted("JE-ISOLATED-01", "2025-03-02", "2025-03", "Isolated small entry", line("L1", "1500", 100, 0), line("L2", "1000", 0, 100)),
	}
}

func withReversal(e ledger.JournalEntry, reversalOf, reversedBy string) ledger.JournalEntry {
	e.Reversal = ledger.Reversal{ReversalOfEntryID: reversalOf, ReversedByEntryID: reversedBy}
	if reversalOf != "" {
		e.Status = ledger.StatusPosted
	} else if reversedBy != "" {
		e.Status = ledger.StatusReversed
	}
	return e
}

// Metadata returns EntryMetadata for the subset of Entries that need it to
// exercise metadata-dependent rules (manual/source mix, period-end/post-
// close timestamps, weekend/business-hours, preparer/approver, batch/
// external references, recurring suppression). Entries not listed here are
// intentionally left without metadata, to also exercise "unavailable due
// to missing metadata" behavior for callers that only supply a subset.
func Metadata() []journaldiagnostics.EntryMetadata {
	manual := journaldiagnostics.SourceManual
	system := journaldiagnostics.SourceSystem
	recurring := journaldiagnostics.SourceRecurring
	imported := journaldiagnostics.SourceImport

	return []journaldiagnostics.EntryMetadata{
		{EntryID: "JE-001", Source: system, PostedAt: ts("2025-03-03", 10), PreparerID: "sys-payroll", ApproverID: "sys-auto"},
		{EntryID: "JE-002", Source: imported, PostedAt: ts("2025-03-05", 9), PreparerID: "prep-A", ApproverID: "appr-B"},
		{EntryID: "JE-LARGE-01", Source: manual, PostedAt: ts("2025-03-11", 14), PreparerID: "prep-A", ApproverID: "appr-B", ExternalRef: "MANUAL-ADJ-1001"},
		{EntryID: "JE-PE-01", Source: manual, PostedAt: ts("2025-03-30", 11), PreparerID: "prep-C", ApproverID: "appr-B"},
		{EntryID: "JE-POSTCLOSE-01", Source: manual, PostedAt: ts("2025-04-05", 10), PreparerID: "prep-C", ApproverID: "appr-B"},
		{EntryID: "JE-WKND-01", Source: manual, PostedAt: ts("2025-03-15", 10), PreparerID: "prep-A", ApproverID: "appr-B"},
		{EntryID: "JE-AFTERHRS-01", Source: manual, PostedAt: ts("2025-03-18", 23), PreparerID: "prep-A", ApproverID: "appr-B"},
		{EntryID: "JE-EQUITY-01", Source: manual, PostedAt: ts("2025-03-29", 15), PreparerID: "prep-C", ApproverID: "appr-B"},
		{EntryID: "JE-REV-ORIG-01", Source: manual, PostedAt: ts("2025-03-06", 9)},
		{EntryID: "JE-REV-REV-01", Source: manual, PostedAt: ts("2025-03-07", 9)},
		{EntryID: "JE-PEER-ORIG-01", Source: manual, PostedAt: ts("2025-03-31", 16)},
		{EntryID: "JE-PEER-REV-01", Source: manual, PostedAt: ts("2025-04-01", 9)},
		{EntryID: "JE-SAMEPA-01", Source: manual, PostedAt: ts("2025-03-15", 13), PreparerID: "prep-D", ApproverID: "prep-D"},
		{EntryID: "JE-RECUR-01", Source: recurring, PostedAt: ts("2025-03-01", 6)},
		{EntryID: "JE-RECUR-02", Source: recurring, PostedAt: ts("2025-03-01", 6)},
		{EntryID: "JE-RECUR-03", Source: recurring, PostedAt: ts("2025-03-01", 6)},
		{EntryID: "JE-THRESH-01", Source: manual, PostedAt: ts("2025-03-22", 10)},
		{EntryID: "JE-THRESH-02", Source: manual, PostedAt: ts("2025-03-22", 11)},
		{EntryID: "JE-SPLIT-01", Source: manual, PostedAt: ts("2025-03-27", 9), PreparerID: "prep-E"},
		{EntryID: "JE-SPLIT-02", Source: manual, PostedAt: ts("2025-03-27", 10), PreparerID: "prep-E"},
	}
}

// SensitiveAccountPolicy returns a caller-supplied review policy marking
// the suspense account (1500) sensitive.
func SensitiveAccountPolicy() []journaldiagnostics.AccountReviewPolicy {
	return []journaldiagnostics.AccountReviewPolicy{
		{Label: "Suspense accounts", AccountIDs: []string{"1500"}},
	}
}

// Policy returns a Policy tuned to actually trigger every scenario Entries
// builds, given Window — a caller-realistic configuration, not the bare
// DefaultPolicy zero-value gaps.
func Policy() journaldiagnostics.Policy {
	p := journaldiagnostics.DefaultPolicy()
	p.MaterialAmount = 4000
	p.RoundDollarMinAmount = 1000
	p.LargeEntryAbsoluteThreshold = 40000
	p.RepeatedAmountMinAmount = 1000
	p.MinRepeatedAmountCount = 3
	// RareAccountMaxHistoricalEntries is raised above the package default
	// (2) to 3 specifically so the JE-RAREACCT-01 fixture scenario (account
	// 6900 has 3 other historical entries — JE-REPEAT-01/02/03 — before this
	// 4th material posting) lands as "rare" rather than "ordinary": 3 other
	// entries is still a thin history for a 4th material posting to stand
	// out against, and this value is what a caller would plausibly tune it
	// to in practice.
	p.RareAccountMaxHistoricalEntries = 3
	p.ApprovalThreshold = 10000
	p.ThresholdClusterMinCount = 2
	p.GenericDescriptions = []string{"Adjustment", "Journal", "Misc", "Reclass"}
	p.RequiredReferenceFields = []string{"external_ref", "reference", "external_reference", "batch_id"}
	p.HighVolumePreparerCount = 3
	p.BusinessHours = journaldiagnostics.BusinessHours{StartHour: 8, EndHour: 18, TimeZone: "UTC"}
	p.SensitiveAccounts = SensitiveAccountPolicy()
	return p
}

// Ledger bundles Chart and Entries into a ledger.Ledger.
func Ledger() ledger.Ledger {
	return ledger.Ledger{Accounts: Chart(), Entries: Entries()}
}

// BrokenLedgerUnbalancedEntry returns a ledger containing one structurally
// invalid (unbalanced) entry alongside otherwise-valid entries, to exercise
// this package's "invalid entries excluded from diagnostics but surfaced
// via Issues/LedgerIssues" behavior.
func BrokenLedgerUnbalancedEntry() ledger.Ledger {
	entries := append([]ledger.JournalEntry{}, Entries()...)
	entries = append(entries, posted("JE-BROKEN-01", "2025-03-20", "2025-03", "Unbalanced entry", line("L1", "6200", 500, 0), line("L2", "1000", 0, 400)))
	return ledger.Ledger{Accounts: Chart(), Entries: entries}
}
