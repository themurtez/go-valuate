package journaldiagnostics_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics/fixtures"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

func TestCalculate_Fixtures_NoErrorIssues(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if journaldiagnostics.HasErrors(r.Issues) {
		t.Fatalf("expected no error issues, got: %+v", r.Issues)
	}
	if r.PopulationSummary.AnalyzedEntries == 0 {
		t.Fatal("expected a non-zero analyzed population")
	}
	if r.SchemaVersion != journaldiagnostics.SchemaVersion || r.FormulaVersion != journaldiagnostics.FormulaVersion {
		t.Errorf("expected Result versions to echo package constants, got schema=%s formula=%s", r.SchemaVersion, r.FormulaVersion)
	}
}

func TestBuildPopulation_ExcludesInvalidEntries(t *testing.T) {
	l := fixtures.BrokenLedgerUnbalancedEntry()
	r := journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), fixtures.Policy())

	cleanResult := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if r.PopulationSummary.AnalyzedEntries != cleanResult.PopulationSummary.AnalyzedEntries {
		t.Errorf("expected the broken (unbalanced) entry to be excluded from analyzed population: got %d analyzed, clean baseline is %d",
			r.PopulationSummary.AnalyzedEntries, cleanResult.PopulationSummary.AnalyzedEntries)
	}
	if r.PopulationSummary.ExcludedEntries != 1 {
		t.Errorf("expected exactly 1 excluded entry, got %d", r.PopulationSummary.ExcludedEntries)
	}
	if len(r.LedgerIssues) == 0 {
		t.Error("expected LedgerIssues to carry through the unbalanced-entry validation error")
	}
	found := false
	for _, iss := range r.Issues {
		if iss.Code == journaldiagnostics.IssueLedgerValidationFailed {
			found = true
		}
	}
	if !found {
		t.Error("expected an IssueLedgerValidationFailed summary issue")
	}

	for _, f := range r.Findings {
		for _, id := range f.EntryIDs {
			if id == "JE-BROKEN-01" {
				t.Errorf("excluded entry JE-BROKEN-01 must not appear in any Finding, found in %s", f.Code)
			}
		}
	}
}

func TestBuildPopulation_PostedOnlyDefaultScope(t *testing.T) {
	l := fixtures.Ledger()
	l.Entries = append(l.Entries, ledger.JournalEntry{
		ID: "JE-DRAFT-01", Date: "2025-03-15", Period: "2025-03", Status: ledger.StatusDraft,
		Lines: []ledger.JournalLine{{AccountID: "1000", Debit: 100}, {AccountID: "6200", Credit: 100}},
	})
	l.Entries = append(l.Entries, ledger.JournalEntry{
		ID: "JE-VOID-01", Date: "2025-03-15", Period: "2025-03", Status: ledger.StatusVoided,
		Lines: []ledger.JournalLine{{AccountID: "1000", Debit: 200}, {AccountID: "6200", Credit: 200}},
	})

	r := journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range r.Findings {
		for _, id := range f.EntryIDs {
			if id == "JE-DRAFT-01" || id == "JE-VOID-01" {
				t.Errorf("draft/voided entry %s must not appear in Findings by default", id)
			}
		}
	}
}

func TestBuildPopulation_IncludeStatusesOptIn(t *testing.T) {
	l := fixtures.Ledger()
	l.Entries = append(l.Entries, ledger.JournalEntry{
		ID: "JE-DRAFT-02", Date: "2025-03-15", Period: "2025-03", Status: ledger.StatusDraft,
		Lines: []ledger.JournalLine{{AccountID: "1000", Debit: 100}, {AccountID: "6200", Credit: 100}},
	})

	policy := fixtures.Policy()
	policy.IncludeStatuses = []ledger.EntryStatus{ledger.StatusPosted, ledger.StatusReversed, ledger.StatusDraft}
	r := journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), policy)

	baseline := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if r.PopulationSummary.InScopeEntries != baseline.PopulationSummary.InScopeEntries+1 {
		t.Errorf("expected draft entry to be included in scope when explicitly opted in: got %d, baseline %d",
			r.PopulationSummary.InScopeEntries, baseline.PopulationSummary.InScopeEntries)
	}
}

func TestBuildPopulation_VoidedNeverIncludedEvenIfExplicit(t *testing.T) {
	l := fixtures.Ledger()
	l.Entries = append(l.Entries, ledger.JournalEntry{
		ID: "JE-VOID-02", Date: "2025-03-15", Period: "2025-03", Status: ledger.StatusVoided,
		Lines: []ledger.JournalLine{{AccountID: "1000", Debit: 100}, {AccountID: "6200", Credit: 100}},
	})
	policy := fixtures.Policy()
	policy.IncludeStatuses = []ledger.EntryStatus{ledger.StatusPosted, ledger.StatusReversed, ledger.StatusVoided}
	r := journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), policy)

	baseline := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if r.PopulationSummary.InScopeEntries != baseline.PopulationSummary.InScopeEntries {
		t.Errorf("VOIDED must never be included even when explicitly listed: got %d in-scope, baseline %d",
			r.PopulationSummary.InScopeEntries, baseline.PopulationSummary.InScopeEntries)
	}
}

func TestBuildPopulation_ReversedEntriesIncludedByDefault(t *testing.T) {
	foundReversed := false
	for _, e := range fixtures.Entries() {
		if e.EffectiveStatus() == ledger.StatusReversed {
			foundReversed = true
			break
		}
	}
	if !foundReversed {
		t.Fatal("fixture setup expected: at least one StatusReversed entry")
	}

	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if r.PopulationSummary.AnalyzedEntries < 1 {
		t.Fatal("expected a non-empty analyzed population including reversed entries")
	}
}

func TestEntryMagnitude_IsTotalDebitsNotSum(t *testing.T) {
	e := ledger.JournalEntry{
		ID: "JE-MAG-01", Date: "2025-03-01", Status: ledger.StatusPosted,
		Lines: []ledger.JournalLine{
			{AccountID: "1000", Debit: 500},
			{AccountID: "1100", Debit: 500},
			{AccountID: "2000", Credit: 1000},
		},
	}
	got := journaldiagnostics.EntryMagnitude(e)
	if got != 1000 {
		t.Errorf("expected EntryMagnitude (total debits) == 1000, got %v (debits+credits would incorrectly be 2000)", got)
	}
}

func TestCalculate_InvalidPeriodWindowIsIssue(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), journaldiagnostics.PeriodWindow{}, fixtures.Policy())
	found := false
	for _, iss := range r.Issues {
		if iss.Code == journaldiagnostics.IssueInvalidPeriod {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidPeriod for an empty PeriodWindow")
	}

	rBackwards := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), journaldiagnostics.PeriodWindow{
		Period: "2025-03", StartDate: "2025-03-31", EndDate: "2025-03-01",
	}, fixtures.Policy())
	found = false
	for _, iss := range rBackwards.Issues {
		if iss.Code == journaldiagnostics.IssueInvalidPeriod {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidPeriod when StartDate is after EndDate")
	}
}

func TestCalculate_DuplicateAndUnknownMetadataIssues(t *testing.T) {
	meta := append([]journaldiagnostics.EntryMetadata{}, fixtures.Metadata()...)
	meta = append(meta, journaldiagnostics.EntryMetadata{EntryID: "JE-001", Source: journaldiagnostics.SourceManual})
	meta = append(meta, journaldiagnostics.EntryMetadata{EntryID: "JE-DOES-NOT-EXIST", Source: journaldiagnostics.SourceManual})

	r := journaldiagnostics.Calculate(fixtures.Ledger(), meta, fixtures.Window(), fixtures.Policy())

	var hasDup, hasUnknown bool
	for _, iss := range r.Issues {
		if iss.Code == journaldiagnostics.IssueDuplicateMetadataEntry {
			hasDup = true
		}
		if iss.Code == journaldiagnostics.IssueUnknownMetadataEntry {
			hasUnknown = true
		}
	}
	if !hasDup {
		t.Error("expected IssueDuplicateMetadataEntry for a duplicated EntryID")
	}
	if !hasUnknown {
		t.Error("expected IssueUnknownMetadataEntry for metadata referencing a nonexistent entry")
	}
}

func TestCoverage_ReflectsMetadataCompleteness(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if r.Coverage.EntriesAnalyzed != r.PopulationSummary.AnalyzedEntries {
		t.Errorf("Coverage.EntriesAnalyzed (%d) should match PopulationSummary.AnalyzedEntries (%d)",
			r.Coverage.EntriesAnalyzed, r.PopulationSummary.AnalyzedEntries)
	}
	if r.Coverage.PercentWithPostedAt <= 0 || r.Coverage.PercentWithPostedAt > 1 {
		t.Errorf("expected PercentWithPostedAt in (0, 1], got %v", r.Coverage.PercentWithPostedAt)
	}

	rNoMeta := journaldiagnostics.Calculate(fixtures.Ledger(), nil, fixtures.Window(), fixtures.Policy())
	if rNoMeta.Coverage.PercentWithPostedAt != 0 {
		t.Errorf("expected 0%% coverage with no metadata supplied, got %v", rNoMeta.Coverage.PercentWithPostedAt)
	}
}
