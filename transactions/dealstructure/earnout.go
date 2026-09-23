package dealstructure

import (
	"fmt"
	"sort"
)

// buildEarnoutSchedule validates every entry in payments, returning one
// EarnoutPayment per valid entry sorted by PeriodNumber (ties broken by
// original input order, via a stable sort), plus one
// IssueInvalidEarnoutPayment per invalid entry. Returns (nil, nil, 0)
// when earnout.Included is false.
func buildEarnoutSchedule(earnout Earnout) ([]EarnoutPayment, []Issue, float64) {
	if !earnout.Included {
		return nil, nil, 0
	}

	var schedule []EarnoutPayment
	var issues []Issue
	var total float64

	for i, p := range earnout.Payments {
		ref := fmt.Sprintf("earnout.payments[%d]", i)
		if p.PeriodNumber < 1 || p.Amount < 0 {
			issues = append(issues, Issue{
				Code:     IssueInvalidEarnoutPayment,
				Severity: SeverityError,
				Message:  "earnout payment must have a period number >= 1 and a non-negative amount; excluded from the schedule",
				Tranche:  ref,
			})
			continue
		}
		schedule = append(schedule, p)
		total += p.Amount
	}

	sort.SliceStable(schedule, func(i, j int) bool {
		return schedule[i].PeriodNumber < schedule[j].PeriodNumber
	})

	return schedule, issues, total
}
