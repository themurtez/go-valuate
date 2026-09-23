package journaldiagnostics_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics/fixtures"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

func TestPeriodEnd_DetectsMaterialNearEndEntry(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingMaterialPeriodEndEntry)
	if len(found) == 0 {
		t.Fatal("expected at least one MATERIAL_PERIOD_END_ENTRY finding")
	}
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-PE-01") {
			matched = true
			if !f.Evidence.DaysFromPeriodEnd.Available {
				t.Error("expected DaysFromPeriodEnd evidence to be available")
			}
		}
	}
	if !matched {
		t.Error("expected JE-PE-01 (posted 2025-03-30, 1 day from period end) to be flagged")
	}
	if r.RuleAvailability.PeriodEnd != journaldiagnostics.RuleAvailable {
		t.Errorf("expected PeriodEnd rule AVAILABLE, got %s", r.RuleAvailability.PeriodEnd)
	}
}

func TestPeriodEnd_PercentOfTotalComputed(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if !r.PeriodEndSummary.Available {
		t.Fatal("expected PeriodEndSummary to be available")
	}
	if !r.PeriodEndSummary.PercentOfTotal.Available {
		t.Error("expected PercentOfTotal to be available given non-zero total activity")
	}
	if r.PeriodEndSummary.PeriodEndEntryCount == 0 {
		t.Error("expected a non-zero period-end entry count")
	}
}

func TestPostClose_DetectsEntryPostedAfterCloseDate(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingPostCloseEntry)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-POSTCLOSE-01") {
			matched = true
			if f.Severity != journaldiagnostics.SeverityHigh {
				t.Errorf("expected POST_CLOSE_ENTRY severity HIGH, got %s", f.Severity)
			}
		}
	}
	if !matched {
		t.Error("expected JE-POSTCLOSE-01 to be flagged as post-close")
	}
}

func TestPostClose_UnavailableWithoutCloseDate(t *testing.T) {
	window := fixtures.Window()
	window.CloseDate = nil
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), window, fixtures.Policy())
	if r.RuleAvailability.PostClose != journaldiagnostics.RuleUnavailable {
		t.Errorf("expected PostClose UNAVAILABLE without a CloseDate, got %s", r.RuleAvailability.PostClose)
	}
	if len(findingsWithCode(r.Findings, journaldiagnostics.FindingPostCloseEntry)) != 0 {
		t.Error("expected no POST_CLOSE_ENTRY findings without a CloseDate")
	}
}

func TestPostClose_NeverGuessesCloseDate(t *testing.T) {
	// A window with no CloseDate must never produce a post-close finding,
	// even though entries exist that would match under some inferred date.
	window := journaldiagnostics.PeriodWindow{Period: "2025-03", StartDate: "2025-03-01", EndDate: "2025-03-31"}
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), window, fixtures.Policy())
	if len(findingsWithCode(r.Findings, journaldiagnostics.FindingPostCloseEntry)) != 0 {
		t.Error("this package must never guess a close date")
	}
}

func TestWeekend_DetectsSaturdayEntry(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingWeekendEntry)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-WKND-01") {
			matched = true
			if f.Severity != journaldiagnostics.SeverityInfo {
				t.Errorf("expected a weekend entry alone to be INFO severity, got %s", f.Severity)
			}
		}
	}
	if !matched {
		t.Error("expected JE-WKND-01 (Saturday 2025-03-15) to be flagged as a weekend entry")
	}
}

func TestWeekend_UnavailableWithoutAnyTimestamp(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), nil, fixtures.Window(), fixtures.Policy())
	if r.RuleAvailability.Weekend != journaldiagnostics.RuleUnavailable {
		t.Errorf("expected Weekend UNAVAILABLE with no metadata timestamps at all, got %s", r.RuleAvailability.Weekend)
	}
}

func TestWeekend_CustomWorkingDaysRespected(t *testing.T) {
	policy := fixtures.Policy()
	// Treat Saturday as a working day; Friday becomes the new "weekend" day
	// to prove the rule reads WorkingDays rather than hard-coding Sat/Sun.
	policy.WorkingDays = []journaldiagnostics.Weekday{
		journaldiagnostics.Sunday, journaldiagnostics.Monday, journaldiagnostics.Tuesday,
		journaldiagnostics.Wednesday, journaldiagnostics.Thursday, journaldiagnostics.Saturday,
	}
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingWeekendEntry) {
		if containsEntryID(f, "JE-WKND-01") {
			t.Error("JE-WKND-01 falls on Saturday, which was reconfigured as a working day and must not be flagged")
		}
	}
}

func TestOutsideBusinessHours_DetectsLateNightEntry(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingOutsideBusinessHours)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-AFTERHRS-01") {
			matched = true
		}
	}
	if !matched {
		t.Error("expected JE-AFTERHRS-01 (posted 23:00 UTC, business hours 8-18) to be flagged")
	}
}

func TestOutsideBusinessHours_UnavailableWithoutConfig(t *testing.T) {
	policy := fixtures.Policy()
	policy.BusinessHours = journaldiagnostics.BusinessHours{}
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if r.RuleAvailability.OutsideBusinessHours != journaldiagnostics.RuleUnavailable {
		t.Errorf("expected OutsideBusinessHours UNAVAILABLE with no BusinessHours configured, got %s", r.RuleAvailability.OutsideBusinessHours)
	}
}

func TestOutsideBusinessHours_UnavailableWithInvalidTimeZone(t *testing.T) {
	policy := fixtures.Policy()
	policy.BusinessHours = journaldiagnostics.BusinessHours{StartHour: 8, EndHour: 18, TimeZone: "Not/A_Real_Zone"}
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if r.RuleAvailability.OutsideBusinessHours != journaldiagnostics.RuleUnavailable {
		t.Errorf("expected OutsideBusinessHours UNAVAILABLE with an unloadable TimeZone, got %s", r.RuleAvailability.OutsideBusinessHours)
	}
}

func TestOutsideBusinessHours_DateOnlyEntryNeverTreatedAsMidnight(t *testing.T) {
	// An entry with metadata present but no PostedAt/CreatedAt timestamp
	// (only a date-only JournalEntry.Date) must never be evaluated against
	// business hours as if it posted at midnight — midnight (hour 0) is
	// well outside an 8-18 window, so if this package incorrectly assumed
	// midnight for a bare date, this entry would wrongly be flagged.
	l := fixtures.Ledger()
	l.Entries = append(l.Entries, ledger.JournalEntry{
		ID: "JE-DATEONLY-01", Date: "2025-03-19", Period: "2025-03", Status: ledger.StatusPosted,
		Lines: []ledger.JournalLine{{AccountID: "1000", Debit: 100}, {AccountID: "6200", Credit: 100}},
	})
	meta := append([]journaldiagnostics.EntryMetadata{}, fixtures.Metadata()...)
	meta = append(meta, journaldiagnostics.EntryMetadata{EntryID: "JE-DATEONLY-01", Source: journaldiagnostics.SourceManual, PreparerID: "prep-A"})

	r := journaldiagnostics.Calculate(l, meta, fixtures.Window(), fixtures.Policy())
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingOutsideBusinessHours) {
		if containsEntryID(f, "JE-DATEONLY-01") {
			t.Error("a date-only entry with no timestamp must never be treated as a midnight posting for business-hours evaluation")
		}
	}
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingWeekendEntry) {
		if containsEntryID(f, "JE-DATEONLY-01") {
			t.Error("a date-only entry with no timestamp must never be treated as a midnight posting for weekend evaluation")
		}
	}
}
