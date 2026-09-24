package kpi

import "github.com/themurtez/go-valuate/accounting/labor"

// LaborMetricCode is the fixed "labor.*" namespace this adapter
// publishes — task section 32's example list.
const (
	LaborMetricTotalLaborCost = "labor.total_labor_cost"
	LaborMetricFTE            = "labor.fte"
)

// LaborMetricValues converts one accounting/labor.PeriodSummary (one
// element of Result.Periods) into this package's MetricValue slice.
// accounting/labor.PeriodSummary.Period.Period already carries this
// period's own label, so no separate period argument is needed (unlike
// FinancialMetricValues, whose financial/metrics.Snapshot has no such
// label of its own). This adapter reimplements no formula — it only
// republishes LaborCostBridge.TotalLaborCost and FTESummary.FTE, both
// already computed by accounting/labor — task section 32.
func LaborMetricValues(ps labor.PeriodSummary, currency string) []MetricValue {
	period := ps.Period.Period
	currencyUnit := Unit{Kind: UnitCurrency, CurrencyCode: currency}
	return []MetricValue{
		{
			Code: LaborMetricTotalLaborCost, Period: period,
			Value: ps.LaborCostBridge.TotalLaborCost, Available: true,
			Unit: currencyUnit, Aggregation: AggregationSum, Source: "accounting/labor",
		},
		{
			Code: LaborMetricFTE, Period: period,
			Value: ps.FTE.FTE, Available: ps.FTE.Available,
			Unit: Unit{Kind: UnitCount}, Aggregation: AggregationSum, Source: "accounting/labor",
		},
	}
}
