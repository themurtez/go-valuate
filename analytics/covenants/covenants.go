package covenants

import (
	"fmt"
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// Calculate derives a full Result from in. It never mutates any
// caller-owned input and performs no I/O.
func Calculate(in Input) Result {
	result := Result{FormulaVersion: FormulaVersion}

	if len(in.Tests) == 0 {
		result.Errors = []Issue{{
			Code:     IssueNoTests,
			Severity: SeverityError,
			Message:  "no covenant tests supplied; nothing to evaluate",
		}}
		return result
	}
	result.Available = true

	var issues []Issue
	seen := make(map[string]bool, len(in.Tests))

	results := make([]TestResult, len(in.Tests))
	for i, t := range in.Tests {
		ref := fmt.Sprintf("tests[%d]", i)

		if t.CovenantID != "" {
			key := t.CovenantID + "\x00" + string(t.Period)
			if seen[key] {
				issues = append(issues, Issue{
					Code:       IssueDuplicateCovenantID,
					Severity:   SeverityWarning,
					Message:    fmt.Sprintf("%s: covenant_id %q for period %q is duplicated across tests; both are evaluated independently", ref, t.CovenantID, t.Period),
					CovenantID: t.CovenantID,
				})
			}
			seen[key] = true
		}

		tr, testIssues := evaluateTest(t, ref)
		results[i] = tr
		issues = append(issues, testIssues...)
	}
	result.Tests = results

	result.Summary = buildSummary(results)

	for i := range issues {
		if issues[i].Severity == SeverityError {
			result.Errors = append(result.Errors, issues[i])
		} else {
			result.Warnings = append(result.Warnings, issues[i])
		}
	}

	return result
}

// buildSummary aggregates results into a Summary, including a
// per-period breakdown sorted lexically by financial.Period — see
// financial.Period's doc comment on why this package draws no
// chronological inference beyond lexical order.
func buildSummary(results []TestResult) Summary {
	s := Summary{TestCount: len(results)}

	var periods []financial.Period
	positionOf := make(map[financial.Period]int)

	for _, tr := range results {
		switch tr.Status {
		case StatusFail:
			s.Breaches++
			s.BreachedCovenantIDs = append(s.BreachedCovenantIDs, tr.CovenantID)
		case StatusUnavailable:
			s.Unavailable++
			s.UnavailableCovenantIDs = append(s.UnavailableCovenantIDs, tr.CovenantID)
		}
		if tr.WarningBufferStatus == WarningBufferWithinBuffer {
			s.NearBreaches++
			s.NearBreachCovenantIDs = append(s.NearBreachCovenantIDs, tr.CovenantID)
		}

		if _, ok := positionOf[tr.Period]; !ok {
			positionOf[tr.Period] = len(periods)
			periods = append(periods, tr.Period)
		}
	}

	sort.Slice(periods, func(i, j int) bool { return periods[i] < periods[j] })
	for i, p := range periods {
		positionOf[p] = i
	}

	byPeriod := make([]PeriodSummary, len(periods))
	for i, p := range periods {
		byPeriod[i] = PeriodSummary{Period: p}
	}
	for _, tr := range results {
		pos := positionOf[tr.Period]
		byPeriod[pos].TestCount++
		switch tr.Status {
		case StatusFail:
			byPeriod[pos].Breaches++
		case StatusUnavailable:
			byPeriod[pos].Unavailable++
		}
		if tr.WarningBufferStatus == WarningBufferWithinBuffer {
			byPeriod[pos].NearBreaches++
		}
	}

	s.ByPeriod = byPeriod
	return s
}
