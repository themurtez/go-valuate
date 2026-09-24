package profitability

import (
	"sort"
	"strings"
)

// normalizeGroupName lower-cases and trims s for stable, locale-naive
// grouping comparisons — mirrors labor.normalizeGroupName's identical
// rationale.
func normalizeGroupName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// sortedStringKeys returns the keys of m sorted ascending by normalized
// (case-folded) comparison, but returns the original key strings — the
// fixed deterministic order used instead of relying on Go map iteration
// order.
func sortedStringKeys[V any](m map[string]V) []string {
	type keyed struct {
		raw  string
		norm string
	}
	pairs := make([]keyed, 0, len(m))
	for k := range m {
		pairs = append(pairs, keyed{raw: k, norm: normalizeGroupName(k)})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].norm != pairs[j].norm {
			return pairs[i].norm < pairs[j].norm
		}
		return pairs[i].raw < pairs[j].raw
	})
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.raw
	}
	return out
}

func sortIssues(issues []Issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if issueRank(a.Code) != issueRank(b.Code) {
			return issueRank(a.Code) < issueRank(b.Code)
		}
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		if a.Period != b.Period {
			return a.Period < b.Period
		}
		return a.Message < b.Message
	})
}

func dedupeAndSortIssues(issues []Issue) []Issue {
	seen := map[Issue]bool{}
	out := make([]Issue, 0, len(issues))
	for _, iss := range issues {
		if seen[iss] {
			continue
		}
		seen[iss] = true
		out = append(out, iss)
	}
	sortIssues(out)
	return out
}

func sortFlags(flags []Flag) {
	sort.SliceStable(flags, func(i, j int) bool {
		a, b := flags[i], flags[j]
		if severityRank(a.Severity) != severityRank(b.Severity) {
			return severityRank(a.Severity) < severityRank(b.Severity)
		}
		if flagRank(a.Code) != flagRank(b.Code) {
			return flagRank(a.Code) < flagRank(b.Code)
		}
		if dimensionRank(a.Dimension) != dimensionRank(b.Dimension) {
			return dimensionRank(a.Dimension) < dimensionRank(b.Dimension)
		}
		if a.EntityID != b.EntityID {
			return a.EntityID < b.EntityID
		}
		if a.Period != b.Period {
			return a.Period < b.Period
		}
		return a.PoolID < b.PoolID
	})
}

// sortPeriodInfos sorts p chronologically by StartDate, falling back to
// Period label for ties/zero dates.
func sortPeriodInfos(p []PeriodInfo) {
	sort.SliceStable(p, func(i, j int) bool {
		if !p[i].StartDate.Equal(p[j].StartDate) {
			return p[i].StartDate.Before(p[j].StartDate)
		}
		return p[i].Period < p[j].Period
	})
}
