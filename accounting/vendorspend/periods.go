package vendorspend

import "sort"

// chronologicalPeriods returns the Period labels present in byLabel,
// ordered by StartDate ascending, then SequenceInYear ascending, then
// Period label ascending (a stable, fully deterministic tiebreak).
func chronologicalPeriods(byLabel map[string]Period) []string {
	periods := make([]Period, 0, len(byLabel))
	for _, p := range byLabel {
		periods = append(periods, p)
	}
	sort.SliceStable(periods, func(i, j int) bool {
		if !periods[i].StartDate.Equal(periods[j].StartDate) {
			return periods[i].StartDate.Before(periods[j].StartDate)
		}
		if periods[i].SequenceInYear != periods[j].SequenceInYear {
			return periods[i].SequenceInYear < periods[j].SequenceInYear
		}
		return periods[i].Period < periods[j].Period
	})
	out := make([]string, 0, len(periods))
	for _, p := range periods {
		out = append(out, p.Period)
	}
	return out
}

// groupBySpendPeriod buckets records by their Period field.
func groupBySpendPeriod(records []SpendRecord) map[string][]SpendRecord {
	out := map[string][]SpendRecord{}
	for _, r := range records {
		out[r.Period] = append(out[r.Period], r)
	}
	return out
}
