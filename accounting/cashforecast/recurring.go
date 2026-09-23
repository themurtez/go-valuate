package cashforecast

import (
	"fmt"
	"time"
)

// RecurringFrequency is how often a RecurringRule repeats.
type RecurringFrequency string

const (
	FrequencyWeekly      RecurringFrequency = "WEEKLY"
	FrequencyBiweekly    RecurringFrequency = "BIWEEKLY"
	FrequencySemimonthly RecurringFrequency = "SEMIMONTHLY"
	FrequencyMonthly     RecurringFrequency = "MONTHLY"
	FrequencyQuarterly   RecurringFrequency = "QUARTERLY"
)

func isRecognizedFrequency(f RecurringFrequency) bool {
	switch f {
	case FrequencyWeekly, FrequencyBiweekly, FrequencySemimonthly, FrequencyMonthly, FrequencyQuarterly:
		return true
	default:
		return false
	}
}

// RecurringRule deterministically generates a series of CashFlowEvents —
// rent, software, insurance, contractor payments, or any other
// caller-specified recurring cash flow. This package never infers a
// recurring cost from historical ledger activity — every RecurringRule is
// fully caller-specified (amount, start date, frequency, optional end
// date, category) — see the task's section 21.
type RecurringRule struct {
	// ID is this rule's identifier, used as the prefix for every generated
	// CashFlowEvent's ID (see generatedEventID) and as SourceID on each.
	ID string `json:"id"`
	// Amount is the fixed amount of each occurrence. Must be > 0.
	Amount float64 `json:"amount"`
	// Direction, Category, Basis, Certainty, Commitment, Priority,
	// Description, CounterpartyID, and Dimensions are copied onto every
	// generated CashFlowEvent unchanged.
	Direction      CashDirection `json:"direction"`
	Category       CashCategory  `json:"category"`
	Basis          CashBasis     `json:"basis"`
	Certainty      Certainty     `json:"certainty,omitempty"`
	Commitment     Commitment    `json:"commitment,omitempty"`
	Priority       Priority      `json:"priority,omitempty"`
	Description    string        `json:"description,omitempty"`
	CounterpartyID string        `json:"counterparty_id,omitempty"`
	Dimensions     []Dimension   `json:"dimensions,omitempty"`

	// StartDate is the first occurrence's date. Required.
	StartDate time.Time `json:"start_date"`
	// EndDate, if non-zero, is the last date an occurrence may fall on
	// (inclusive). If zero, occurrences are generated through the end of
	// the forecast horizon only (this package never generates events
	// beyond the horizon it is asked to cover).
	EndDate time.Time `json:"end_date,omitempty"`
	// Frequency is how often occurrences repeat. Required.
	Frequency RecurringFrequency `json:"frequency"`

	// SemimonthlyDays is used only when Frequency == FrequencySemimonthly:
	// the two days-of-month occurrences fall on (e.g. [1, 15]). Each value
	// must be in [1, 28]; a day beyond a short month is never generated
	// (see occurrencesSemimonthly) — a caller wanting "last day of month"
	// semantics uses FrequencyMonthly with day-31 roll behavior instead
	// (see occurrencesMonthly's doc comment).
	SemimonthlyDays [2]int `json:"semimonthly_days,omitempty"`
}

// generatedEventID returns the deterministic ID for the n-th (0-based)
// occurrence of rule.
func generatedEventID(ruleID string, n int) string {
	return fmt.Sprintf("%s#%d", ruleID, n)
}

// occurrenceDates returns every date rule fires on within [rangeStart,
// rangeEnd] inclusive, honoring rule.EndDate when set (clamped to
// rangeEnd), in ascending order. Every branch is a pure calendar
// computation with no calendar-library dependency beyond time.Time
// arithmetic, so behavior is identical across platforms/Go versions.
func occurrenceDates(rule RecurringRule, rangeStart, rangeEnd time.Time) []time.Time {
	if rule.StartDate.IsZero() || !isRecognizedFrequency(rule.Frequency) {
		return nil
	}
	limit := rangeEnd
	if !rule.EndDate.IsZero() && rule.EndDate.Before(limit) {
		limit = rule.EndDate
	}
	if limit.Before(rangeStart) {
		return nil
	}

	switch rule.Frequency {
	case FrequencyWeekly:
		return occurrencesByStep(rule.StartDate, rangeStart, limit, 7)
	case FrequencyBiweekly:
		return occurrencesByStep(rule.StartDate, rangeStart, limit, 14)
	case FrequencySemimonthly:
		return occurrencesSemimonthly(rule, rangeStart, limit)
	case FrequencyMonthly:
		return occurrencesMonthly(rule.StartDate, rangeStart, limit, 1)
	case FrequencyQuarterly:
		return occurrencesMonthly(rule.StartDate, rangeStart, limit, 3)
	default:
		return nil
	}
}

// occurrencesByStep generates dates start, start+stepDays, start+2*
// stepDays, ... and returns those falling within [rangeStart, rangeEnd].
// Uses a direct day-count jump to the first in-range occurrence rather
// than iterating from StartDate one step at a time, so a forecast
// starting long after a rule's StartDate stays O(1) to reach the first
// relevant occurrence.
func occurrencesByStep(start, rangeStart, rangeEnd time.Time, stepDays int) []time.Time {
	start = truncateToDate(start)
	rangeStart = truncateToDate(rangeStart)
	rangeEnd = truncateToDate(rangeEnd)

	var dates []time.Time
	if start.After(rangeEnd) {
		return dates
	}
	n := 0
	if rangeStart.After(start) {
		daysSince := int(rangeStart.Sub(start).Hours() / 24)
		n = daysSince / stepDays
		if daysSince%stepDays != 0 {
			// n currently lands before rangeStart; the next occurrence
			// (n+1) is the first candidate >= rangeStart.
			n++
		}
	}
	for {
		d := start.AddDate(0, 0, n*stepDays)
		if d.After(rangeEnd) {
			break
		}
		if !d.Before(rangeStart) {
			dates = append(dates, d)
		}
		n++
	}
	return dates
}

// occurrencesMonthly generates one occurrence every stepMonths months,
// anchored to StartDate's day-of-month, and returns those within
// [rangeStart, rangeEnd].
//
// Day-roll behavior for a StartDate anchored on the 29th/30th/31st: when a
// target month has fewer days than the anchor day, the occurrence falls
// on that month's LAST day instead of rolling into the next month (e.g. a
// StartDate of Jan 31 produces Feb 28/29, Mar 31, Apr 30, ...). This
// avoids the drift that AddDate(0, 1, 0) on Jan 31 would otherwise cause
// (Jan 31 + 1 month = Mar 3 in Go's time package, since Feb 31 normalizes
// forward) — see recurring_test.go's TestRecurring_Monthly31st and
// TestRecurring_MonthlyFebruary/TestRecurring_MonthlyLeapYear.
func occurrencesMonthly(start, rangeStart, rangeEnd time.Time, stepMonths int) []time.Time {
	start = truncateToDate(start)
	rangeStart = truncateToDate(rangeStart)
	rangeEnd = truncateToDate(rangeEnd)
	if start.After(rangeEnd) {
		return nil
	}
	anchorDay := start.Day()

	var dates []time.Time
	for n := 0; ; n++ {
		d := monthlyOccurrence(start, anchorDay, n*stepMonths)
		if d.After(rangeEnd) {
			break
		}
		if !d.Before(rangeStart) {
			dates = append(dates, d)
		}
	}
	return dates
}

// monthlyOccurrence returns the date monthsAhead months after start's
// month, on anchorDay, clamped to that target month's last day if
// anchorDay exceeds it.
func monthlyOccurrence(start time.Time, anchorDay, monthsAhead int) time.Time {
	y, m := start.Year(), int(start.Month())
	totalMonths := (m - 1) + monthsAhead
	targetYear := y + totalMonths/12
	targetMonth := time.Month(totalMonths%12 + 1)
	lastDay := daysInMonth(targetYear, targetMonth)
	day := anchorDay
	if day > lastDay {
		day = lastDay
	}
	return time.Date(targetYear, targetMonth, day, 0, 0, 0, 0, time.UTC)
}

// daysInMonth returns the number of days in the given year/month,
// correctly handling leap years (day 0 of the following month is the
// last day of this one).
func daysInMonth(year int, month time.Month) int {
	firstOfNext := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	lastOfThis := firstOfNext.AddDate(0, 0, -1)
	return lastOfThis.Day()
}

// occurrencesSemimonthly generates two occurrences per month at
// rule.SemimonthlyDays[0] and [1] (each clamped to [1,28], so every month
// — including February — always produces exactly two dates), starting no
// earlier than rule.StartDate's month, within [rangeStart, rangeEnd].
func occurrencesSemimonthly(rule RecurringRule, rangeStart, rangeEnd time.Time) []time.Time {
	start := truncateToDate(rule.StartDate)
	rangeStart = truncateToDate(rangeStart)
	rangeEnd = truncateToDate(rangeEnd)
	if start.After(rangeEnd) {
		return nil
	}
	d1, d2 := clampSemimonthlyDay(rule.SemimonthlyDays[0]), clampSemimonthlyDay(rule.SemimonthlyDays[1])

	var dates []time.Time
	y, m := start.Year(), start.Month()
	for {
		monthStart := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
		if monthStart.After(rangeEnd) {
			break
		}
		for _, day := range []int{d1, d2} {
			d := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
			if d.Before(start) || d.Before(rangeStart) || d.After(rangeEnd) {
				continue
			}
			dates = append(dates, d)
		}
		if m == time.December {
			y, m = y+1, time.January
		} else {
			m++
		}
	}
	sortTimes(dates)
	return dates
}

func clampSemimonthlyDay(d int) int {
	if d < 1 {
		return 1
	}
	if d > 28 {
		return 28
	}
	return d
}

func sortTimes(dates []time.Time) {
	for i := 1; i < len(dates); i++ {
		for j := i; j > 0 && dates[j].Before(dates[j-1]); j-- {
			dates[j], dates[j-1] = dates[j-1], dates[j]
		}
	}
}

// generateRecurringEvents deterministically expands rule into one
// CashFlowEvent per occurrence within [rangeStart, rangeEnd].
func generateRecurringEvents(rule RecurringRule, rangeStart, rangeEnd time.Time) []CashFlowEvent {
	dates := occurrenceDates(rule, rangeStart, rangeEnd)
	events := make([]CashFlowEvent, 0, len(dates))
	for i, d := range dates {
		events = append(events, CashFlowEvent{
			ID:             generatedEventID(rule.ID, i),
			Date:           d,
			Amount:         rule.Amount,
			Direction:      rule.Direction,
			Category:       rule.Category,
			Description:    rule.Description,
			CounterpartyID: rule.CounterpartyID,
			SourceType:     SourceRecurringRule,
			SourceID:       rule.ID,
			Basis:          rule.Basis,
			Certainty:      rule.Certainty,
			Commitment:     rule.Commitment,
			Priority:       rule.Priority,
			Dimensions:     append([]Dimension{}, rule.Dimensions...),
		})
	}
	return events
}
