package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
)

func scheduleEntry(entries []ap.DueScheduleEntry, code string) (ap.DueScheduleEntry, bool) {
	for _, e := range entries {
		if e.HorizonCode == code {
			return e, true
		}
	}
	return ap.DueScheduleEntry{}, false
}

func TestSchedule_DefaultHorizons(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-PAST", "S1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 1000, 1000, ap.StatusOpen), // past due
		bill("B-7", "S1", mustDate(t, "2025-06-25"), asOf.AddDate(0, 0, 5), 2000, 2000, ap.StatusOpen),        // due in 5 days
		bill("B-14", "S1", mustDate(t, "2025-06-25"), asOf.AddDate(0, 0, 10), 3000, 3000, ap.StatusOpen),      // due in 10 days -> 8-14
		bill("B-30", "S1", mustDate(t, "2025-06-25"), asOf.AddDate(0, 0, 25), 4000, 4000, ap.StatusOpen),      // due in 25 days -> 15-30
		bill("B-60", "S1", mustDate(t, "2025-06-25"), asOf.AddDate(0, 0, 50), 5000, 5000, ap.StatusOpen),      // due in 50 days -> 31-60
		bill("B-90", "S1", mustDate(t, "2025-06-25"), asOf.AddDate(0, 0, 80), 6000, 6000, ap.StatusOpen),      // due in 80 days -> 61-90
		bill("B-FUT", "S1", mustDate(t, "2025-06-25"), asOf.AddDate(0, 0, 200), 7000, 7000, ap.StatusOpen),    // due in 200 days -> 90+ future
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if !result.DueSchedule.Available {
		t.Fatalf("expected DueSchedule.Available=true, issues: %+v", result.Issues)
	}

	cases := []struct {
		code string
		want float64
	}{
		{"PAST_DUE", 1000},
		{"NEXT_7_DAYS", 2000},
		{"8_14_DAYS", 3000},
		{"15_30_DAYS", 4000},
		{"31_60_DAYS", 5000},
		{"61_90_DAYS", 6000},
		{"90_PLUS_FUTURE", 7000},
	}
	for _, c := range cases {
		e, ok := scheduleEntry(result.DueSchedule.Entries, c.code)
		if !ok {
			t.Errorf("horizon %s not found in entries: %+v", c.code, result.DueSchedule.Entries)
			continue
		}
		if e.Amount != c.want {
			t.Errorf("horizon %s amount = %v, want %v", c.code, e.Amount, c.want)
		}
	}
	if result.DueSchedule.Label == "" {
		t.Errorf("expected non-empty DueSchedule.Label distinguishing this from a forecast")
	}
}

func TestSchedule_IsNotAForecast_OnlyKnownBills(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	// A single known bill; the schedule should reflect exactly this and
	// nothing projected/assumed.
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 3), 1000, 1000, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	var total float64
	for _, e := range result.DueSchedule.Entries {
		total += e.Amount
	}
	if total != 1000 {
		t.Errorf("total scheduled amount = %v, want exactly 1000 (no projection/fabrication)", total)
	}
}

func TestSchedule_CallerConfigurableHorizons(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	custom := []ap.DueScheduleHorizon{
		{Code: "OVERDUE", PastDue: true},
		{Code: "NEXT_3_DAYS", MinDaysUntilDue: 0, MaxDaysUntilDue: 3, HasMax: true},
		{Code: "REST", MinDaysUntilDue: 4, HasMax: false},
	}
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 2), 1000, 1000, ap.StatusOpen),
		bill("B-2", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 100), 2000, 2000, ap.StatusOpen),
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, DueScheduleHorizons: custom})
	e1, ok1 := scheduleEntry(result.DueSchedule.Entries, "NEXT_3_DAYS")
	e2, ok2 := scheduleEntry(result.DueSchedule.Entries, "REST")
	if !ok1 || e1.Amount != 1000 {
		t.Errorf("NEXT_3_DAYS = %+v (ok=%v), want 1000", e1, ok1)
	}
	if !ok2 || e2.Amount != 2000 {
		t.Errorf("REST = %+v (ok=%v), want 2000", e2, ok2)
	}
}
