package cashforecast

import "testing"

// TestWeekAlignment_Monday proves AlignMonday backs Week 1's start up to
// the Monday on or before ForecastStartDate.
func TestWeekAlignment_Monday(t *testing.T) {
	// 2025-01-08 is a Wednesday; the Monday on/before it is 2025-01-06.
	weeks := buildWeekBounds(testDate(t, "2025-01-08"), AlignMonday, 2)
	if got := weeks[0].StartDate.Format("2006-01-02"); got != "2025-01-06" {
		t.Errorf("week 1 start = %s, want 2025-01-06", got)
	}
	if got := weeks[0].EndDate.Format("2006-01-02"); got != "2025-01-12" {
		t.Errorf("week 1 end = %s, want 2025-01-12", got)
	}
}

// TestWeekAlignment_MondayExactMonday proves that when ForecastStartDate
// is already a Monday, AlignMonday keeps Week 1 starting on that same
// date (no unnecessary backward shift).
func TestWeekAlignment_MondayExactMonday(t *testing.T) {
	weeks := buildWeekBounds(testDate(t, "2025-01-06"), AlignMonday, 1) // 2025-01-06 is a Monday.
	if got := weeks[0].StartDate.Format("2006-01-02"); got != "2025-01-06" {
		t.Errorf("week 1 start = %s, want 2025-01-06 (no shift needed)", got)
	}
}

func TestWeekAlignment_Sunday(t *testing.T) {
	// 2025-01-08 is a Wednesday; the Sunday on/before it is 2025-01-05.
	weeks := buildWeekBounds(testDate(t, "2025-01-08"), AlignSunday, 1)
	if got := weeks[0].StartDate.Format("2006-01-02"); got != "2025-01-05" {
		t.Errorf("week 1 start = %s, want 2025-01-05", got)
	}
}

func TestWeekAlignment_SundayExactSunday(t *testing.T) {
	weeks := buildWeekBounds(testDate(t, "2025-01-05"), AlignSunday, 1) // 2025-01-05 is a Sunday.
	if got := weeks[0].StartDate.Format("2006-01-02"); got != "2025-01-05" {
		t.Errorf("week 1 start = %s, want 2025-01-05 (no shift needed)", got)
	}
}

func TestWeekAlignment_CallerStartDateDefault(t *testing.T) {
	weeks := buildWeekBounds(testDate(t, "2025-01-08"), "", 1)
	if got := weeks[0].StartDate.Format("2006-01-02"); got != "2025-01-08" {
		t.Errorf("week 1 start = %s, want 2025-01-08 (default alignment keeps caller's date)", got)
	}
}

func TestWeekAlignment_CalculateWiresAlignmentThrough(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-08"), // Wednesday
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
	}
	result := Calculate(in, Options{WeekAlignment: AlignMonday, HorizonWeeks: 1})

	if result.BaseScenario.Weekly[0].StartDate != "2025-01-06" {
		t.Errorf("week 1 StartDate = %s, want 2025-01-06 (MONDAY alignment)", result.BaseScenario.Weekly[0].StartDate)
	}
	if result.WeekAlignment != AlignMonday {
		t.Errorf("Result.WeekAlignment = %s, want MONDAY", result.WeekAlignment)
	}
}

func TestWeekAlignment_UnrecognizedValueIsRejectedByIsRecognized(t *testing.T) {
	if isRecognizedAlignment("BOGUS") {
		t.Errorf("expected BOGUS to be unrecognized")
	}
	if !isRecognizedAlignment(AlignMonday) || !isRecognizedAlignment(AlignSunday) || !isRecognizedAlignment(AlignCallerStartDate) || !isRecognizedAlignment("") {
		t.Errorf("expected all real alignment values (and empty) to be recognized")
	}
}
