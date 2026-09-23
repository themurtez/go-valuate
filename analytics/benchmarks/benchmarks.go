package benchmarks

import "fmt"

// Calculate derives a full Result from in. It never mutates any
// caller-owned input and performs no I/O.
func Calculate(in Input) Result {
	result := Result{FormulaVersion: FormulaVersion}

	if len(in.Metrics) == 0 {
		result.Errors = []Issue{{
			Code:     IssueNoMetrics,
			Severity: SeverityError,
			Message:  "no metrics supplied; nothing to compare",
		}}
		return result
	}
	result.Available = true

	var issues []Issue
	seen := make(map[string]bool, len(in.Metrics))

	comparisons := make([]Comparison, len(in.Metrics))
	for i, r := range in.Metrics {
		ref := fmt.Sprintf("metrics[%d]", i)

		if r.MetricID != "" {
			key := r.MetricID + "\x00" + string(r.Period)
			if seen[key] {
				issues = append(issues, Issue{
					Code:     IssueDuplicateMetricID,
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("%s: metric_id %q for period %q is duplicated across metrics; both are evaluated independently", ref, r.MetricID, r.Period),
					MetricID: r.MetricID,
				})
			}
			seen[key] = true
		}

		cmp, metricIssues := evaluateMetric(r, ref)
		comparisons[i] = cmp
		issues = append(issues, metricIssues...)
	}
	result.Comparisons = comparisons

	result.Summary = buildSummary(comparisons)

	for i := range issues {
		if issues[i].Severity == SeverityError {
			result.Errors = append(result.Errors, issues[i])
		} else {
			result.Warnings = append(result.Warnings, issues[i])
		}
	}

	return result
}

// buildSummary aggregates comparisons into a Summary.
func buildSummary(comparisons []Comparison) Summary {
	s := Summary{MetricCount: len(comparisons)}

	for _, c := range comparisons {
		if !c.Available {
			s.UnavailableCount++
			continue
		}
		switch c.Favorable {
		case FavorableYes:
			s.FavorableCount++
		case FavorableNo:
			s.UnfavorableCount++
			s.UnfavorableMetricIDs = append(s.UnfavorableMetricIDs, c.MetricID)
		}
	}

	return s
}
