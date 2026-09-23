package inventory

import (
	"sort"
	"time"
)

// PeriodInfo is one caller-supplied, explicitly dated analysis period —
// task section 11. Unlike several analytics-style sibling packages that
// order periods via a fiscal-year/sequence pair, this package's turnover/
// DIO/purchases-vs-usage analysis is naturally interval-based (a period
// has a start and end date, and "days" for a DIO calculation must be a
// real day count), so StartDate/EndDate/Days are the ordering and
// arithmetic basis directly.
type PeriodInfo struct {
	// Period is a caller-supplied display label (e.g. "2025-Q1",
	// "2025-06"). Not used for ordering — StartDate is.
	Period    string    `json:"period"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	// Days is this period's day count, used as the DIO multiplier. If
	// zero, it is derived from EndDate - StartDate + 1 (inclusive) — see
	// resolvedPeriodDays.
	Days int `json:"days,omitempty"`
}

// resolvedPeriodDays returns p.Days if positive, otherwise the inclusive
// day count between StartDate and EndDate (0 if either is zero-value or
// EndDate is before StartDate).
func resolvedPeriodDays(p PeriodInfo) int {
	if p.Days > 0 {
		return p.Days
	}
	if p.StartDate.IsZero() || p.EndDate.IsZero() || p.EndDate.Before(p.StartDate) {
		return 0
	}
	return int(p.EndDate.Sub(p.StartDate).Hours()/24) + 1
}

// sortedPeriods returns a copy of periods sorted chronologically by
// StartDate ascending (ties broken by EndDate ascending, then Period
// label ascending) — the deterministic order every historical
// calculation in this package uses, independent of caller input order.
func sortedPeriods(periods []PeriodInfo) []PeriodInfo {
	out := make([]PeriodInfo, len(periods))
	copy(out, periods)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].StartDate.Equal(out[j].StartDate) {
			return out[i].StartDate.Before(out[j].StartDate)
		}
		if !out[i].EndDate.Equal(out[j].EndDate) {
			return out[i].EndDate.Before(out[j].EndDate)
		}
		return out[i].Period < out[j].Period
	})
	return out
}
