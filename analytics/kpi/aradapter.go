package kpi

import "github.com/themurtez/go-valuate/accounting/ar"

// ARMetricCode is the fixed "ar.*" namespace this adapter publishes —
// task section 32's example list.
const (
	ARMetricTotalOpenAR = "ar.total_open_ar"
	ARMetricOver60      = "ar.over_60_amount"
	ARMetricDSO         = "ar.dso"
)

// ARMetricValues converts one accounting/ar.Result (an as-of-date
// snapshot, not a multi-period series — see accounting/ar's own package
// doc) into this package's MetricValue slice for period (a caller-chosen
// label, since accounting/ar.Result itself only carries AsOfDate, not a
// period Code this engine's Period.Code convention requires). This
// adapter reimplements no formula: TotalOpenReceivables and DSO.Value are
// republished exactly as accounting/ar computed them. The one derived
// figure, over-60 exposure, is a straightforward re-sum of
// PortfolioSummary.Buckets already-computed bucket amounts against
// Result.Buckets' own MinDaysPastDue metadata (never a hard-coded bucket
// code assumption, since a caller may configure custom buckets) — still
// "expose existing computed values," not a new formula, since every
// number summed was already computed by accounting/ar itself.
func ARMetricValues(res ar.Result, period string, currency string) []MetricValue {
	currencyUnit := Unit{Kind: UnitCurrency, CurrencyCode: currency}
	if !res.Available {
		return []MetricValue{
			{Code: ARMetricTotalOpenAR, Period: period, Unit: currencyUnit, Aggregation: AggregationSum, Source: "accounting/ar"},
			{Code: ARMetricOver60, Period: period, Unit: currencyUnit, Aggregation: AggregationSum, Source: "accounting/ar"},
			{Code: ARMetricDSO, Period: period, Unit: Unit{Kind: UnitDays}, Aggregation: AggregationNotAggregatable, Source: "accounting/ar"},
		}
	}

	over60, over60Available := sumARBucketsOver(res, 60)

	return []MetricValue{
		{
			Code: ARMetricTotalOpenAR, Period: period,
			Value: res.PortfolioSummary.TotalOpenReceivables, Available: true,
			Unit: currencyUnit, Aggregation: AggregationSum, Source: "accounting/ar",
		},
		{
			Code: ARMetricOver60, Period: period,
			Value: over60, Available: over60Available,
			Unit: currencyUnit, Aggregation: AggregationSum, Source: "accounting/ar",
		},
		{
			Code: ARMetricDSO, Period: period,
			Value: res.DSO.Value, Available: res.DSO.Available,
			Unit: Unit{Kind: UnitDays}, Aggregation: AggregationNotAggregatable, Source: "accounting/ar",
		},
	}
}

// sumARBucketsOver sums PortfolioSummary.Buckets for every bucket whose
// matching Result.Buckets definition has MinDaysPastDue > thresholdDays,
// by BucketCode. ok is false only if the resolved bucket schema has no
// bucket at all past thresholdDays (distinct from a genuinely-zero sum).
func sumARBucketsOver(res ar.Result, thresholdDays int) (float64, bool) {
	qualifying := map[string]bool{}
	for _, b := range res.Buckets {
		if b.MinDaysPastDue > thresholdDays {
			qualifying[b.Code] = true
		}
	}
	if len(qualifying) == 0 {
		return 0, false
	}
	total := 0.0
	for _, ba := range res.PortfolioSummary.Buckets {
		if qualifying[ba.BucketCode] {
			total += ba.Amount
		}
	}
	return total, true
}
