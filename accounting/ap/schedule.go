package ap

import (
	"sort"
	"time"
)

// DueScheduleHorizon is one caller-configurable horizon bucket in the
// due-date schedule, expressed in days from AsOfDate to DueDate (NOT days
// past due — this is a forward-looking schedule of bills not yet due,
// plus one bucket for bills already past due). Mirrors BucketDefinition's
// shape/validation style but is a distinct type since its axis (days
// until due) is the mirror image of the aging axis (days past due).
type DueScheduleHorizon struct {
	// Code is a stable identifier (e.g. "NEXT_7_DAYS").
	Code string `json:"code"`
	// Label is an optional human-readable display label.
	Label string `json:"label,omitempty"`
	// MinDaysUntilDue is this horizon's inclusive lower bound. Negative
	// values represent "past due"; PastDue (below) is the single
	// dedicated past-due horizon rather than a horizon needing a negative
	// bound, so ordinary forward horizons keep MinDaysUntilDue >= 0.
	MinDaysUntilDue int `json:"min_days_until_due"`
	// MaxDaysUntilDue is this horizon's inclusive upper bound. Ignored
	// (open-ended) when HasMax is false.
	MaxDaysUntilDue int `json:"max_days_until_due,omitempty"`
	// HasMax is false for exactly one forward horizon: the terminal,
	// open-ended one (e.g. "90+ future").
	HasMax bool `json:"has_max"`
	// PastDue marks the single dedicated "already past due" horizon
	// (task section 12's "past due" bucket) rather than a numeric range —
	// every payable with DaysPastDue > 0 (per the resolved AgingBasis)
	// falls here, regardless of MinDaysUntilDue/MaxDaysUntilDue/HasMax,
	// which are ignored for this horizon.
	PastDue bool `json:"past_due,omitempty"`
}

// DefaultDueScheduleHorizons returns this package's default schedule:
// past due, next 7 days, 8-14 days, 15-30 days, 31-60 days, 61-90 days,
// 90+ future — exactly the horizons the task specifies.
func DefaultDueScheduleHorizons() []DueScheduleHorizon {
	return []DueScheduleHorizon{
		{Code: "PAST_DUE", Label: "Past Due", PastDue: true},
		{Code: "NEXT_7_DAYS", Label: "Next 7 Days", MinDaysUntilDue: 0, MaxDaysUntilDue: 7, HasMax: true},
		{Code: "8_14_DAYS", Label: "8-14 Days", MinDaysUntilDue: 8, MaxDaysUntilDue: 14, HasMax: true},
		{Code: "15_30_DAYS", Label: "15-30 Days", MinDaysUntilDue: 15, MaxDaysUntilDue: 30, HasMax: true},
		{Code: "31_60_DAYS", Label: "31-60 Days", MinDaysUntilDue: 31, MaxDaysUntilDue: 60, HasMax: true},
		{Code: "61_90_DAYS", Label: "61-90 Days", MinDaysUntilDue: 61, MaxDaysUntilDue: 90, HasMax: true},
		{Code: "90_PLUS_FUTURE", Label: "90+ Days (Future)", MinDaysUntilDue: 91, HasMax: false},
	}
}

func resolvedDueScheduleHorizons(defs []DueScheduleHorizon) []DueScheduleHorizon {
	if len(defs) > 0 {
		return defs
	}
	return DefaultDueScheduleHorizons()
}

// DueScheduleEntry is one horizon's aggregated open-AP figures.
type DueScheduleEntry struct {
	HorizonCode string  `json:"horizon_code"`
	Amount      float64 `json:"amount"`
	BillCount   int     `json:"bill_count"`
}

// DueSchedule is section 12's future contractual obligations schedule:
// known open AP grouped by due date, in horizon order. This is explicitly
// NOT a forecast — it contains no projection, no assumption about future
// purchasing, and no probability of payment; it is a grouping of bills
// that already exist as of AsOfDate.
type DueSchedule struct {
	Available bool                 `json:"available"`
	Horizons  []DueScheduleHorizon `json:"horizons"`
	Entries   []DueScheduleEntry   `json:"entries,omitempty"`
	// Label is a fixed, always-present disclaimer distinguishing this
	// schedule from a cash forecast.
	Label string `json:"label"`
}

const dueScheduleLabel = "known open AP grouped by due date; not a forecast of future purchasing or a projection of payment probability"

// daysUntilDue returns DueDate - AsOfDate in whole days. Negative means
// already past due.
func daysUntilDue(asOf, dueDate time.Time) int {
	if dueDate.IsZero() {
		return 0
	}
	d := int(dueDate.Sub(asOf).Hours() / 24)
	return d
}

// horizonForRow returns the Code of the DueScheduleHorizon row falls
// into: the dedicated PastDue horizon if row.daysPastDue > 0, otherwise
// the first forward horizon (in horizons' given order, expected sorted by
// MinDaysUntilDue ascending) whose [Min, Max] range contains
// daysUntilDue.
func horizonForRow(horizons []DueScheduleHorizon, row payableAging, daysUntilDueVal int) (string, bool) {
	if row.daysPastDue > 0 {
		for _, h := range horizons {
			if h.PastDue {
				return h.Code, true
			}
		}
		return "", false
	}
	for _, h := range horizons {
		if h.PastDue {
			continue
		}
		if daysUntilDueVal < h.MinDaysUntilDue {
			continue
		}
		if !h.HasMax || daysUntilDueVal <= h.MaxDaysUntilDue {
			return h.Code, true
		}
	}
	return "", false
}

// buildDueSchedule groups included, non-credit rows by due-date horizon.
// asOf and basis together determine whether a row counts as "past due"
// consistently with the rest of this package's aging.
func buildDueSchedule(rows []payableAging, horizons []DueScheduleHorizon, asOf time.Time) DueSchedule {
	sorted := make([]DueScheduleHorizon, len(horizons))
	copy(sorted, horizons)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].PastDue != sorted[j].PastDue {
			return sorted[i].PastDue // PastDue horizon sorts first.
		}
		return sorted[i].MinDaysUntilDue < sorted[j].MinDaysUntilDue
	})

	sums := make(map[string]float64, len(sorted))
	counts := make(map[string]int, len(sorted))
	any := false
	for _, row := range rows {
		if !row.includedInAgg || row.isCredit || row.p.OpenAmount == 0 {
			continue
		}
		until := daysUntilDue(asOf, row.p.DueDate)
		code, ok := horizonForRow(sorted, row, until)
		if !ok {
			continue
		}
		sums[code] += row.p.OpenAmount
		counts[code]++
		any = true
	}

	if !any {
		return DueSchedule{Horizons: sorted, Label: dueScheduleLabel}
	}

	entries := make([]DueScheduleEntry, 0, len(sorted))
	for _, h := range sorted {
		entries = append(entries, DueScheduleEntry{HorizonCode: h.Code, Amount: sums[h.Code], BillCount: counts[h.Code]})
	}

	return DueSchedule{Available: true, Horizons: sorted, Entries: entries, Label: dueScheduleLabel}
}
