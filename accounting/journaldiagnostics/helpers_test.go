package journaldiagnostics_test

import (
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

// findingsWithCode filters fs to Findings with the given Code.
func findingsWithCode(fs []journaldiagnostics.Finding, code journaldiagnostics.FindingCode) []journaldiagnostics.Finding {
	var out []journaldiagnostics.Finding
	for _, f := range fs {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

// containsEntryID reports whether f.EntryIDs contains id.
func containsEntryID(f journaldiagnostics.Finding, id string) bool {
	for _, e := range f.EntryIDs {
		if e == id {
			return true
		}
	}
	return false
}

// journalEntryHelper builds a minimal balanced 2-line posted entry (cash
// credited, office-supplies expense debited) for tests that need one extra
// entry with a specific ID/date/amount injected into the fixture ledger,
// without repeating the same 6-line literal at every call site.
func journalEntryHelper(id, date string, amount float64) ledger.JournalEntry {
	return ledger.JournalEntry{
		ID: id, Date: date, Period: "2025-03", Status: ledger.StatusPosted,
		Lines: []ledger.JournalLine{
			{ID: "L1", AccountID: "6200", Debit: amount},
			{ID: "L2", AccountID: "1000", Credit: amount},
		},
	}
}
