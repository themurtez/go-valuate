package kpi

import "github.com/themurtez/go-valuate/accounting/vendorspend"

// VendorSpendMetricCode is the fixed "vendorspend.*" namespace this
// adapter publishes — task section 32's example list.
const VendorSpendMetricTotalSpend = "vendorspend.total_spend"

// VendorSpendMetricValues converts one accounting/vendorspend.
// PeriodSummary's NetSpend into this package's MetricValue slice.
// accounting/vendorspend.Result.PeriodSummaries is the per-period
// breakdown of the overall Result.Bridge; this adapter takes one
// PeriodSummary at a time (mirroring InventoryMetricValues/
// LaborMetricValues' identical "one period-scoped struct per call"
// shape) rather than the whole Result, so a caller converts exactly the
// periods it wants without this adapter making an iteration-order
// decision on the caller's behalf.
func VendorSpendMetricValues(ps vendorspend.PeriodSummary, currency string) []MetricValue {
	return []MetricValue{
		{
			Code: VendorSpendMetricTotalSpend, Period: ps.Period,
			Value: ps.Bridge.NetSpend, Available: true,
			Unit: Unit{Kind: UnitCurrency, CurrencyCode: currency}, Aggregation: AggregationSum, Source: "accounting/vendorspend",
		},
	}
}
