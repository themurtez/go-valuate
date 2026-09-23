package benchmarks

import "fmt"

// evaluateMetric evaluates a single MetricRequest, returning its
// Comparison plus any Issue found. ref identifies this request for
// Issue.MetricID when r.MetricID itself is empty (the problem being
// reported).
func evaluateMetric(r MetricRequest, ref string) (Comparison, []Issue) {
	cmp := Comparison{
		MetricID:             r.MetricID,
		Label:                r.Label,
		Period:               r.Period,
		Direction:            r.Direction,
		CompanyValue:         r.CompanyValue,
		SourceIndustryLabel:  r.Benchmark.IndustryLabel,
		SourceSizeLabel:      r.Benchmark.SizeLabel,
		SourceGeographyLabel: r.Benchmark.GeographyLabel,
		Source:               r.Benchmark.Source,
		Form:                 r.Benchmark.Form,
		Favorable:            FavorableNotApplicable,
	}
	if cmp.Label == "" {
		cmp.Label = r.MetricID
	}

	var issues []Issue
	metricRef := r.MetricID
	if metricRef == "" {
		metricRef = ref
	}

	if r.MetricID == "" {
		issues = append(issues, Issue{
			Code:     IssueMissingMetricID,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%s: metric_id is empty; comparison reported as unavailable", ref),
			MetricID: metricRef,
		})
		return cmp, issues
	}
	cmp.Available = true

	if r.Benchmark.Source.Name == "" {
		issues = append(issues, Issue{
			Code:     IssueMissingSourceName,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%s: benchmark source name is empty", metricRef),
			MetricID: metricRef,
		})
	}

	stats, badCode := deriveBenchmarkStats(r.Benchmark)
	switch badCode {
	case IssueInvalidForm:
		issues = append(issues, Issue{
			Code:     IssueInvalidForm,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%s: benchmark form %q is not a recognized BenchmarkForm value; no benchmark comparison possible", metricRef, r.Benchmark.Form),
			MetricID: metricRef,
		})
		return cmp, issues
	case IssueBenchmarkDataMissing:
		issues = append(issues, Issue{
			Code:     IssueBenchmarkDataMissing,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%s: benchmark form %q was declared but its data field was empty; no benchmark comparison possible", metricRef, r.Benchmark.Form),
			MetricID: metricRef,
		})
		return cmp, issues
	}

	if stats.insufficientPoints {
		issues = append(issues, Issue{
			Code:     IssueInsufficientPercentileBands,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%s: fewer than two distinct benchmark points available; no interpolated percentile estimate possible", metricRef),
			MetricID: metricRef,
		})
	}
	if stats.nonMonotonic {
		issues = append(issues, Issue{
			Code:     IssueNonMonotonicBenchmarkPoints,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%s: benchmark points do not have value non-decreasing with percentile; percentile/band/range are not calculable from a self-contradictory table", metricRef),
			MetricID: metricRef,
		})
	}

	cmp.BenchmarkMedian = stats.median
	if !stats.nonMonotonic {
		cmp.BenchmarkRange = benchmarkRange(stats)
	}
	if !r.CompanyValue.Available {
		issues = append(issues, Issue{
			Code:     IssueCompanyValueUnavailable,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%s: company value is unavailable; benchmark-side fields are still reported but no comparison can be made", metricRef),
			MetricID: metricRef,
		})
		return cmp, issues
	}

	if !stats.nonMonotonic {
		cmp.Percentile = companyPercentile(r.Benchmark.Form, stats, r.CompanyValue.Amount)
		cmp.Band = classifyBand(stats, r.CompanyValue.Amount)
	}

	if cmp.BenchmarkMedian.Available {
		diff := r.CompanyValue.Amount - cmp.BenchmarkMedian.Amount
		cmp.Difference = AvailableValue(diff)
		if cmp.BenchmarkMedian.Amount != 0 {
			cmp.RelativeDifference = AvailableValue(diff / absFloat(cmp.BenchmarkMedian.Amount))
		} else {
			issues = append(issues, Issue{
				Code:     IssueBenchmarkMedianZero,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("%s: benchmark median is exactly 0; relative difference is undefined", metricRef),
				MetricID: metricRef,
			})
		}
		cmp.Favorable = classifyFavorable(r.Direction, diff)
	}

	return cmp, issues
}

// benchmarkRange derives the benchmark population's known [min, max]
// extent from stats.points, when any points are known. Unavailable when
// stats.points is empty (FormMedian with no percentile data at all).
func benchmarkRange(stats benchmarkStats) Range {
	if len(stats.points) == 0 {
		return Range{}
	}
	return Range{
		Min: AvailableValue(stats.points[0].Value),
		Max: AvailableValue(stats.points[len(stats.points)-1].Value),
	}
}

// companyPercentile estimates what percentile of the benchmark population
// v falls at, given form/stats. Unavailable for FormMedian (a single
// point supports no percentile estimate) or when stats.insufficientPoints.
func companyPercentile(form BenchmarkForm, stats benchmarkStats, v float64) Value {
	if form == FormMedian || stats.insufficientPoints || len(stats.points) < 2 {
		return Unavailable()
	}
	return AvailableValue(percentileOfValue(stats.points, v))
}

// classifyBand classifies v's placement against stats.points. Reports
// BandUnavailable when fewer than two points are known (matching
// companyPercentile's own threshold, since a band is just a coarser
// bucketing of the same percentile estimate).
func classifyBand(stats benchmarkStats, v float64) Band {
	if stats.insufficientPoints || len(stats.points) < 2 {
		return BandUnavailable
	}
	min := stats.points[0].Value
	max := stats.points[len(stats.points)-1].Value
	if v < min {
		return BandBelowMin
	}
	if v > max {
		return BandAboveMax
	}
	pct := percentileOfValue(stats.points, v)
	switch {
	case pct <= 25:
		return BandQ1
	case pct <= 50:
		return BandQ2
	case pct <= 75:
		return BandQ3
	default:
		return BandQ4
	}
}

// classifyFavorable classifies diff (company value minus benchmark
// median) per dir. Never guesses when dir is DirectionUnspecified or
// DirectionNeutral (see Direction's doc comment).
func classifyFavorable(dir Direction, diff float64) Favorable {
	if diff == 0 {
		switch dir {
		case DirectionHigherIsBetter, DirectionLowerIsBetter:
			return FavorableEqual
		default:
			return FavorableNotApplicable
		}
	}
	switch dir {
	case DirectionHigherIsBetter:
		if diff > 0 {
			return FavorableYes
		}
		return FavorableNo
	case DirectionLowerIsBetter:
		if diff < 0 {
			return FavorableYes
		}
		return FavorableNo
	default:
		return FavorableNotApplicable
	}
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
