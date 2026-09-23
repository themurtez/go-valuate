package cashforecast

import "sort"

// sortEvents sorts events by Date, then Direction, then Category, then ID
// — the fixed tie-break order used everywhere this package presents a
// detailed event list (DetailedSchedule) — see the task's section 39.
func sortEvents(events []CashFlowEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.Before(b.Date)
		}
		if a.Direction != b.Direction {
			return a.Direction < b.Direction
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		return a.ID < b.ID
	})
}

// sortIssues sorts issues by Code, then EventID, then SourceID.
func sortIssues(issues []Issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.EventID != b.EventID {
			return a.EventID < b.EventID
		}
		return a.SourceID < b.SourceID
	})
}

// sortFlags sorts flags by FlagCode declaration order, then Scenario, then
// Week.
func sortFlags(flags []Flag) {
	sort.SliceStable(flags, func(i, j int) bool {
		ri, rj := flagRank(flags[i].Code), flagRank(flags[j].Code)
		if ri != rj {
			return ri < rj
		}
		if flags[i].Scenario != flags[j].Scenario {
			return flags[i].Scenario < flags[j].Scenario
		}
		return flags[i].Week < flags[j].Week
	})
}

// sortedEventIDs returns a sorted copy of ids, used wherever provenance
// lists (EventIDs) must not depend on map iteration order.
func sortedEventIDs(ids map[string]bool) []string {
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
