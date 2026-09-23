package journaldiagnostics_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics/fixtures"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

func TestReversalSummary_ReportsExplicitPairs(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if !r.ReversalSummary.Available {
		t.Fatal("expected ReversalSummary.Available == true (reversal detection needs no optional metadata)")
	}
	if r.ReversalSummary.ReversalCount < 2 {
		t.Errorf("expected at least 2 explicit reversal pairs in the fixture set, got %d", r.ReversalSummary.ReversalCount)
	}
	found := false
	for _, p := range r.ReversalSummary.Pairs {
		if p.OriginalEntryID == "JE-REV-ORIG-01" && p.ReversingEntryID == "JE-REV-REV-01" {
			found = true
		}
	}
	if !found {
		t.Error("expected JE-REV-ORIG-01/JE-REV-REV-01 pair in ReversalSummary.Pairs")
	}
}

func TestRapidReversal_DetectsQuickReversal(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingRapidReversal)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-REV-ORIG-01") && containsEntryID(f, "JE-REV-REV-01") {
			matched = true
			if !f.Evidence.DaysBetween.Available || f.Evidence.DaysBetween.Amount != 1 {
				t.Errorf("expected DaysBetween == 1, got %+v", f.Evidence.DaysBetween)
			}
		}
	}
	if !matched {
		t.Error("expected JE-REV-ORIG-01/JE-REV-REV-01 (1 day apart) to be flagged as RAPID_REVERSAL")
	}
}

func TestRapidReversal_OutsideWindowNotFlagged(t *testing.T) {
	policy := fixtures.Policy() // RapidReversalDays resolves to DefaultPolicy's 3
	l := fixtures.Ledger()
	l.Entries = append(l.Entries,
		withReversalHelper(journalEntryHelper("JE-SLOWREV-ORIG", "2025-03-01", 500), "", "JE-SLOWREV-REV"),
		withReversalHelper(journalEntryHelper("JE-SLOWREV-REV", "2025-03-20", 500), "JE-SLOWREV-ORIG", ""),
	)
	r := journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), policy)
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingRapidReversal) {
		if containsEntryID(f, "JE-SLOWREV-ORIG") {
			t.Error("a reversal 19 days later must not be flagged as RAPID_REVERSAL under the default 3-day window")
		}
	}
}

func TestCrossPeriodReversal_DetectsDifferentPeriod(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingCrossPeriodReversal)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-PEER-ORIG-01") && containsEntryID(f, "JE-PEER-REV-01") {
			matched = true
			if f.Evidence.OriginalPeriod == f.Evidence.ReversingPeriod {
				t.Error("expected OriginalPeriod != ReversingPeriod for a cross-period reversal")
			}
		}
	}
	if !matched {
		t.Error("expected JE-PEER-ORIG-01 (2025-03) / JE-PEER-REV-01 (2025-04) to be flagged as CROSS_PERIOD_REVERSAL")
	}
}

func TestPeriodEndEarlyReversal_DetectsCombination(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingPeriodEndEntryWithEarlyReversal)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-PEER-ORIG-01") {
			matched = true
			if f.Severity != journaldiagnostics.SeverityHigh {
				t.Errorf("expected HIGH severity, got %s", f.Severity)
			}
		}
	}
	if !matched {
		t.Error("expected JE-PEER-ORIG-01 (material, period-end, reversed 1 day later) to be flagged")
	}
}

func TestReversal_NeverInferredFromOppositeAmountsAlone(t *testing.T) {
	// Two unrelated entries that happen to have exactly opposite amounts on
	// the same accounts, with NO ledger.Reversal relationship declared,
	// must never be treated as a reversal pair.
	l := fixtures.Ledger()
	l.Entries = append(l.Entries,
		journalEntryHelper("JE-COINCIDENCE-01", "2025-03-01", 777),
		ledger.JournalEntry{
			ID: "JE-COINCIDENCE-02", Date: "2025-03-02", Period: "2025-03", Status: ledger.StatusPosted,
			Lines: []ledger.JournalLine{
				{ID: "L1", AccountID: "1000", Debit: 777},
				{ID: "L2", AccountID: "6200", Credit: 777},
			},
		},
	)
	r := journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if r.ReversalSummary.ReversalCount != 2 {
		// Baseline fixture set has exactly 2 explicit reversal pairs; this
		// coincidental opposite-amount pair must not add a 3rd.
		t.Errorf("expected ReversalCount to remain at the baseline 2 explicit pairs (no inference from opposite amounts), got %d", r.ReversalSummary.ReversalCount)
	}
	for _, p := range r.ReversalSummary.Pairs {
		if p.OriginalEntryID == "JE-COINCIDENCE-01" || p.OriginalEntryID == "JE-COINCIDENCE-02" {
			t.Error("coincidentally-opposite entries with no declared Reversal must never appear in ReversalSummary.Pairs")
		}
	}
}

// withReversalHelper mirrors fixtures.go's unexported withReversal helper
// (not usable here directly, since it is unexported in another package).
func withReversalHelper(e ledger.JournalEntry, reversalOf, reversedBy string) ledger.JournalEntry {
	e.Reversal = ledger.Reversal{ReversalOfEntryID: reversalOf, ReversedByEntryID: reversedBy}
	if reversalOf != "" {
		e.Status = ledger.StatusPosted
	} else if reversedBy != "" {
		e.Status = ledger.StatusReversed
	}
	return e
}
