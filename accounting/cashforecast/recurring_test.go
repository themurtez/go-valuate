package cashforecast

import (
	"testing"
	"time"
)

func testDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return d
}

func datesToStrings(dates []time.Time) []string {
	out := make([]string, len(dates))
	for i, d := range dates {
		out[i] = d.Format("2006-01-02")
	}
	return out
}

func assertDateSeries(t *testing.T, got []time.Time, want []string) {
	t.Helper()
	gotStr := datesToStrings(got)
	if len(gotStr) != len(want) {
		t.Fatalf("got %d dates %v, want %d dates %v", len(gotStr), gotStr, len(want), want)
	}
	for i := range want {
		if gotStr[i] != want[i] {
			t.Errorf("date[%d] = %s, want %s (full got=%v want=%v)", i, gotStr[i], want[i], gotStr, want)
		}
	}
}

func TestRecurring_Weekly(t *testing.T) {
	rule := RecurringRule{ID: "r", StartDate: testDate(t, "2025-01-06"), Frequency: FrequencyWeekly}
	got := occurrenceDates(rule, testDate(t, "2025-01-01"), testDate(t, "2025-01-27"))
	assertDateSeries(t, got, []string{"2025-01-06", "2025-01-13", "2025-01-20", "2025-01-27"})
}

func TestRecurring_Biweekly(t *testing.T) {
	rule := RecurringRule{ID: "r", StartDate: testDate(t, "2025-01-03"), Frequency: FrequencyBiweekly}
	got := occurrenceDates(rule, testDate(t, "2025-01-01"), testDate(t, "2025-02-28"))
	assertDateSeries(t, got, []string{"2025-01-03", "2025-01-17", "2025-01-31", "2025-02-14", "2025-02-28"})
}

// TestRecurring_WeeklyStepJumpsToFirstInRange proves occurrencesByStep's
// O(1) jump-to-first-relevant-occurrence logic is correct when rangeStart
// is long after StartDate (not just when it coincides with an occurrence).
func TestRecurring_WeeklyStepJumpsToFirstInRange(t *testing.T) {
	rule := RecurringRule{ID: "r", StartDate: testDate(t, "2020-01-01"), Frequency: FrequencyWeekly}
	// rangeStart falls between two Wednesday occurrences (2025-01-01 is a
	// Wednesday relative to the 2020-01-01 anchor cycle).
	got := occurrenceDates(rule, testDate(t, "2025-06-10"), testDate(t, "2025-06-20"))
	for _, d := range got {
		if d.Before(testDate(t, "2025-06-10")) || d.After(testDate(t, "2025-06-20")) {
			t.Errorf("date %v out of range", d)
		}
	}
	if len(got) == 0 {
		t.Fatalf("expected at least one occurrence in range")
	}
}

func TestRecurring_SemimonthlyDefault(t *testing.T) {
	rule := RecurringRule{ID: "r", StartDate: testDate(t, "2025-01-01"), Frequency: FrequencySemimonthly, SemimonthlyDays: [2]int{1, 15}}
	got := occurrenceDates(rule, testDate(t, "2025-01-01"), testDate(t, "2025-03-31"))
	assertDateSeries(t, got, []string{
		"2025-01-01", "2025-01-15",
		"2025-02-01", "2025-02-15",
		"2025-03-01", "2025-03-15",
	})
}

// TestRecurring_Monthly31st proves the documented day-roll behavior: a
// StartDate anchored on the 31st clamps to each month's last day rather
// than drifting forward the way Go's AddDate(0,1,0) would (Jan 31 + 1
// month normalizes to Mar 3, not Feb 28/29).
func TestRecurring_Monthly31st(t *testing.T) {
	rule := RecurringRule{ID: "r", StartDate: testDate(t, "2025-01-31"), Frequency: FrequencyMonthly}
	got := occurrenceDates(rule, testDate(t, "2025-01-01"), testDate(t, "2025-12-31"))
	assertDateSeries(t, got, []string{
		"2025-01-31", "2025-02-28", "2025-03-31", "2025-04-30",
		"2025-05-31", "2025-06-30", "2025-07-31", "2025-08-31",
		"2025-09-30", "2025-10-31", "2025-11-30", "2025-12-31",
	})
}

// TestRecurring_MonthlyLeapYearFeb29 proves a StartDate of Feb 29 (leap
// year) clamps to Feb 28 in a non-leap year and returns to Feb 29 in the
// next leap year, without drifting the anchor day permanently to 28.
func TestRecurring_MonthlyLeapYearFeb29(t *testing.T) {
	rule := RecurringRule{ID: "r", StartDate: testDate(t, "2024-02-29"), Frequency: FrequencyMonthly}
	got := occurrenceDates(rule, testDate(t, "2024-02-01"), testDate(t, "2024-04-30"))
	assertDateSeries(t, got, []string{"2024-02-29", "2024-03-29", "2024-04-29"})

	// One year later (2025, not a leap year): anchor day 29 still applies
	// to Feb (28 days -> clamps to 28), proving the clamp doesn't
	// permanently downgrade the anchor to 28.
	got2 := occurrenceDates(rule, testDate(t, "2025-02-01"), testDate(t, "2025-02-28"))
	assertDateSeries(t, got2, []string{"2025-02-28"})

	// The following leap year (2028), Feb 29 reappears.
	got3 := occurrenceDates(rule, testDate(t, "2028-02-01"), testDate(t, "2028-02-29"))
	assertDateSeries(t, got3, []string{"2028-02-29"})
}

func TestRecurring_QuarterlyFromMonthEnd(t *testing.T) {
	rule := RecurringRule{ID: "r", StartDate: testDate(t, "2025-11-30"), Frequency: FrequencyQuarterly}
	got := occurrenceDates(rule, testDate(t, "2025-11-01"), testDate(t, "2026-11-30"))
	// Nov 30 -> Feb 28 (2026 not leap) -> May 30 -> Aug 30 -> Nov 30.
	assertDateSeries(t, got, []string{"2025-11-30", "2026-02-28", "2026-05-30", "2026-08-30", "2026-11-30"})
}

func TestRecurring_EndDateRespected(t *testing.T) {
	rule := RecurringRule{ID: "r", StartDate: testDate(t, "2025-01-01"), EndDate: testDate(t, "2025-02-15"), Frequency: FrequencyMonthly}
	got := occurrenceDates(rule, testDate(t, "2025-01-01"), testDate(t, "2025-12-31"))
	// Occurrences: Jan 1, Feb 1 (both <= EndDate of Feb 15); Mar 1 is
	// excluded since it falls after EndDate.
	assertDateSeries(t, got, []string{"2025-01-01", "2025-02-01"})
}

func TestRecurring_InvalidRuleProducesNoOccurrences(t *testing.T) {
	rule := RecurringRule{ID: "r", Frequency: FrequencyMonthly} // zero StartDate
	got := occurrenceDates(rule, testDate(t, "2025-01-01"), testDate(t, "2025-12-31"))
	if len(got) != 0 {
		t.Errorf("expected no occurrences for zero StartDate, got %v", got)
	}

	rule2 := RecurringRule{ID: "r", StartDate: testDate(t, "2025-01-01"), Frequency: "BOGUS"}
	got2 := occurrenceDates(rule2, testDate(t, "2025-01-01"), testDate(t, "2025-12-31"))
	if len(got2) != 0 {
		t.Errorf("expected no occurrences for unrecognized frequency, got %v", got2)
	}
}

func TestRecurring_GenerateEventsCarriesFields(t *testing.T) {
	rule := RecurringRule{
		ID: "rent-1", Amount: 2500, Direction: DirectionOutflow, Category: CategoryRent,
		Basis: BasisScheduled, Certainty: CertaintyHigh, Commitment: CommitmentRequired,
		Description: "Office rent", CounterpartyID: "landlord-1",
		StartDate: testDate(t, "2025-01-01"), Frequency: FrequencyMonthly,
	}
	events := generateRecurringEvents(rule, testDate(t, "2025-01-01"), testDate(t, "2025-03-31"))
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	for i, e := range events {
		if e.Amount != 2500 || e.Direction != DirectionOutflow || e.Category != CategoryRent {
			t.Errorf("event %d fields not carried through: %+v", i, e)
		}
		if e.SourceType != SourceRecurringRule || e.SourceID != "rent-1" {
			t.Errorf("event %d provenance not set: %+v", i, e)
		}
		wantID := generatedEventID("rent-1", i)
		if e.ID != wantID {
			t.Errorf("event %d ID = %s, want %s", i, e.ID, wantID)
		}
	}
}

// TestRecurring_GeneratedEventsDoNotMutateRuleDimensions proves
// generateRecurringEvents never lets a generated event share a backing
// array with rule.Dimensions.
func TestRecurring_GeneratedEventsDoNotMutateRuleDimensions(t *testing.T) {
	rule := RecurringRule{
		ID: "r", Amount: 100, Direction: DirectionOutflow, Category: CategoryRent, Basis: BasisScheduled,
		StartDate: testDate(t, "2025-01-01"), Frequency: FrequencyMonthly,
		Dimensions: []Dimension{{Key: "location", Value: "hq"}},
	}
	events := generateRecurringEvents(rule, testDate(t, "2025-01-01"), testDate(t, "2025-02-28"))
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events")
	}
	events[0].Dimensions[0].Value = "mutated"
	if rule.Dimensions[0].Value != "hq" {
		t.Errorf("mutating events[0].Dimensions mutated rule.Dimensions: %+v", rule.Dimensions)
	}
	if events[1].Dimensions[0].Value != "hq" {
		t.Errorf("mutating events[0].Dimensions leaked into events[1]: %+v", events[1].Dimensions)
	}
}
