package kpi

import "github.com/themurtez/go-valuate/accounting/ap"

// APMetricCode is the fixed "ap.*" namespace this adapter publishes —
// task section 32's example list.
const (
	APMetricTotalOpenAP = "ap.total_open_ap"
	APMetricDPO         = "ap.dpo"
)

// APMetricValues converts one accounting/ap.Result (an as-of-date
// snapshot, mirroring accounting/ar.Result's identical shape and this
// package's ARMetricValues' identical "caller supplies the period label"
// convention) into this package's MetricValue slice. Reimplements no
// formula: TotalOpenPayables and DPO.Value are republished exactly as
// accounting/ap computed them.
func APMetricValues(res ap.Result, period string, currency string) []MetricValue {
	currencyUnit := Unit{Kind: UnitCurrency, CurrencyCode: currency}
	if !res.Available {
		return []MetricValue{
			{Code: APMetricTotalOpenAP, Period: period, Unit: currencyUnit, Aggregation: AggregationSum, Source: "accounting/ap"},
			{Code: APMetricDPO, Period: period, Unit: Unit{Kind: UnitDays}, Aggregation: AggregationNotAggregatable, Source: "accounting/ap"},
		}
	}
	return []MetricValue{
		{
			Code: APMetricTotalOpenAP, Period: period,
			Value: res.PortfolioSummary.TotalOpenPayables, Available: true,
			Unit: currencyUnit, Aggregation: AggregationSum, Source: "accounting/ap",
		},
		{
			Code: APMetricDPO, Period: period,
			Value: res.DPO.Value, Available: res.DPO.Available,
			Unit: Unit{Kind: UnitDays}, Aggregation: AggregationNotAggregatable, Source: "accounting/ap",
		},
	}
}
