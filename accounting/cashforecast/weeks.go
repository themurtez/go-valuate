package cashforecast

import "time"

// WeekAlignment controls where Week 1 starts relative to
// Input.ForecastStartDate.
type WeekAlignment string

const (
	// AlignCallerStartDate (the default) makes Week 1 start exactly on
	// Input.ForecastStartDate, per the task's recommended convention:
	// Week 1 spans [ForecastStartDate, ForecastStartDate+6 days], Week 2
	// starts immediately after, and so on. This package never assumes a
	// Monday/Sunday week without the caller explicitly choosing one of the
	// other two alignments.
	AlignCallerStartDate WeekAlignment = "CALLER_START_DATE"
	// AlignMonday makes Week 1 start on the Monday on or before
	// ForecastStartDate.
	AlignMonday WeekAlignment = "MONDAY"
	// AlignSunday makes Week 1 start on the Sunday on or before
	// ForecastStartDate.
	AlignSunday WeekAlignment = "SUNDAY"
)

func isRecognizedAlignment(a WeekAlignment) bool {
	switch a {
	case "", AlignCallerStartDate, AlignMonday, AlignSunday:
		return true
	default:
		return false
	}
}

func resolvedAlignment(a WeekAlignment) WeekAlignment {
	if a == "" {
		return AlignCallerStartDate
	}
	return a
}

// DefaultHorizonWeeks is the default and documented forecast horizon: 13
// weeks. Options.HorizonWeeks may override this within [1, MaxHorizonWeeks]
// when a caller's design genuinely needs a different horizon, but neither
// the package name nor its documentation lets that configurability
// complicate the core 13-week contract — see the task's explicit
// instruction.
const DefaultHorizonWeeks = 13

// MaxHorizonWeeks is the largest horizon Options.HorizonWeeks accepts.
const MaxHorizonWeeks = 52

func resolvedHorizonWeeks(n int) int {
	if n <= 0 {
		return DefaultHorizonWeeks
	}
	return n
}

// weekBounds is one week's [StartDate, EndDate] inclusive calendar range.
type weekBounds struct {
	WeekNumber int
	StartDate  time.Time
	EndDate    time.Time
}

// alignedWeek1Start returns Week 1's start date under alignment, given
// forecastStart.
func alignedWeek1Start(forecastStart time.Time, alignment WeekAlignment) time.Time {
	switch resolvedAlignment(alignment) {
	case AlignMonday:
		// time.Weekday: Sunday=0, Monday=1, ..., Saturday=6. Days to step
		// back to reach the on-or-before Monday.
		back := int(forecastStart.Weekday()) - int(time.Monday)
		if back < 0 {
			back += 7
		}
		return forecastStart.AddDate(0, 0, -back)
	case AlignSunday:
		back := int(forecastStart.Weekday()) - int(time.Sunday)
		if back < 0 {
			back += 7
		}
		return forecastStart.AddDate(0, 0, -back)
	default: // AlignCallerStartDate
		return forecastStart
	}
}

// buildWeekBounds returns horizonWeeks consecutive weekBounds starting at
// alignedWeek1Start(forecastStart, alignment), each spanning exactly 7
// calendar days (start inclusive, end inclusive, 6 days after start).
func buildWeekBounds(forecastStart time.Time, alignment WeekAlignment, horizonWeeks int) []weekBounds {
	start := alignedWeek1Start(forecastStart, alignment)
	weeks := make([]weekBounds, 0, horizonWeeks)
	for i := 0; i < horizonWeeks; i++ {
		weekStart := start.AddDate(0, 0, i*7)
		weekEnd := weekStart.AddDate(0, 0, 6)
		weeks = append(weeks, weekBounds{WeekNumber: i + 1, StartDate: weekStart, EndDate: weekEnd})
	}
	return weeks
}

// dateWithinInclusive reports whether d falls within [start, end]
// inclusive, comparing calendar dates only (time-of-day is ignored — every
// date in this package is treated as a calendar date).
func dateWithinInclusive(d, start, end time.Time) bool {
	d = truncateToDate(d)
	start = truncateToDate(start)
	end = truncateToDate(end)
	return !d.Before(start) && !d.After(end)
}

// truncateToDate strips time-of-day and location so date comparisons are
// never affected by a caller supplying a time.Time with a non-midnight
// time component or a non-UTC location.
func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// weekIndexForDate returns the 0-based index into weeks whose range
// contains d, and false if d falls outside every week (before the first
// week's start or after the last week's end).
func weekIndexForDate(weeks []weekBounds, d time.Time) (int, bool) {
	dd := truncateToDate(d)
	if len(weeks) == 0 {
		return 0, false
	}
	if dd.Before(truncateToDate(weeks[0].StartDate)) {
		return 0, false
	}
	if dd.After(truncateToDate(weeks[len(weeks)-1].EndDate)) {
		return 0, false
	}
	for i, w := range weeks {
		if dateWithinInclusive(dd, w.StartDate, w.EndDate) {
			return i, true
		}
	}
	return 0, false
}
