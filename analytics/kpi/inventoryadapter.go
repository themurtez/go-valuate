package kpi

import "github.com/themurtez/go-valuate/accounting/inventory"

// InventoryMetricCode is the fixed "inventory.*" namespace this adapter
// publishes — task section 32's example list.
const (
	InventoryMetricTotalValue = "inventory.total_value"
	InventoryMetricTurnover   = "inventory.turnover"
	InventoryMetricDIO        = "inventory.dio"
)

// InventoryMetricValues converts one accounting/inventory.PeriodSummary
// (one element of Result.Periods) plus the overall
// Result.Portfolio.TotalInventoryValue into this package's MetricValue
// slice. Reimplements no formula: Turnover.Value/DIO.Value/
// TotalInventoryValue are republished exactly as accounting/inventory
// computed them. totalInventoryValue is passed separately (rather than
// read from PeriodSummary, which has no such field of its own — it is a
// portfolio-wide, not period-scoped, figure in accounting/inventory's own
// shape) so a caller pairs it with whichever period's PeriodSummary it is
// also converting.
func InventoryMetricValues(ps inventory.PeriodSummary, totalInventoryValue float64, currency string) []MetricValue {
	period := ps.Period.Period
	currencyUnit := Unit{Kind: UnitCurrency, CurrencyCode: currency}
	return []MetricValue{
		{
			Code: InventoryMetricTotalValue, Period: period,
			Value: totalInventoryValue, Available: true,
			Unit: currencyUnit, Aggregation: AggregationLast, Source: "accounting/inventory",
		},
		{
			Code: InventoryMetricTurnover, Period: period,
			Value: ps.Turnover.Value, Available: ps.Turnover.Available,
			Unit: Unit{Kind: UnitRatio}, Aggregation: AggregationNotAggregatable, Source: "accounting/inventory",
		},
		{
			Code: InventoryMetricDIO, Period: period,
			Value: ps.DIO.Value, Available: ps.DIO.Available,
			Unit: Unit{Kind: UnitDays}, Aggregation: AggregationNotAggregatable, Source: "accounting/inventory",
		},
	}
}
