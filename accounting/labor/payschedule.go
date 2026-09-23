package labor

import "sort"

// PayFrequency is a closed taxonomy for observed pay-date spacing — the
// task's section 32. This package never infers employer payroll policy
// from sparse records; PayFrequencyIrregular is reported whenever the
// observed spacing does not cleanly match one fixed interval.
type PayFrequency string

const (
	PayFrequencyWeekly      PayFrequency = "WEEKLY"
	PayFrequencyBiweekly    PayFrequency = "BIWEEKLY"
	PayFrequencySemimonthly PayFrequency = "SEMIMONTHLY"
	PayFrequencyMonthly     PayFrequency = "MONTHLY"
	PayFrequencyIrregular   PayFrequency = "IRREGULAR"
)

// PayScheduleSummary is the task's section 32 optional observed pay-date
// spacing summary, computed only from actual distinct PayDate values
// across the supplied payroll records (never from a caller-declared
// frequency field, since PayrollRecord has none — this package infers
// only the descriptive spacing pattern of dates actually observed, which
// is different from asserting employer policy).
type PayScheduleSummary struct {
	Available        bool         `json:"available"`
	Frequency        PayFrequency `json:"frequency,omitempty"`
	DistinctPayDates int          `json:"distinct_pay_dates"`
	// MedianIntervalDays is the median number of days between consecutive
	// distinct pay dates.
	MedianIntervalDays float64 `json:"median_interval_days,omitempty"`
}

// buildPayScheduleSummary requires at least 3 distinct pay dates (2
// intervals) before reporting anything, since a single interval cannot
// reliably distinguish weekly-with-a-gap from biweekly.
func buildPayScheduleSummary(payroll []PayrollRecord) PayScheduleSummary {
	dateSet := map[string]bool{}
	for _, r := range payroll {
		if !r.PayDate.IsZero() {
			dateSet[r.PayDate.Format("2006-01-02")] = true
		}
	}
	if len(dateSet) < 3 {
		return PayScheduleSummary{}
	}

	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	parsed := make([]int64, 0, len(dates))
	for _, d := range dates {
		t := mustParseDate(d)
		parsed = append(parsed, t.Unix())
	}

	intervals := make([]float64, 0, len(parsed)-1)
	for i := 1; i < len(parsed); i++ {
		days := float64(parsed[i]-parsed[i-1]) / 86400
		intervals = append(intervals, days)
	}

	median := medianFloat(intervals)

	freq := PayFrequencyIrregular
	switch {
	case withinTolerance(median, 7, 1):
		freq = PayFrequencyWeekly
	case withinTolerance(median, 14, 1.5):
		freq = PayFrequencyBiweekly
	case withinTolerance(median, 15, 2):
		freq = PayFrequencySemimonthly
	case withinTolerance(median, 30, 3):
		freq = PayFrequencyMonthly
	}

	return PayScheduleSummary{
		Available:          true,
		Frequency:          freq,
		DistinctPayDates:   len(dateSet),
		MedianIntervalDays: median,
	}
}

func withinTolerance(v, target, tol float64) bool {
	diff := v - target
	if diff < 0 {
		diff = -diff
	}
	return diff <= tol
}

func medianFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := append([]float64{}, vals...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
