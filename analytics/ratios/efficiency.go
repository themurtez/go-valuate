package ratios

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// daysInPeriod is the fixed day-count this package uses for every days-
// outstanding/cash-conversion-cycle calculation (DaysSalesOutstanding,
// DaysInventoryOutstanding, DaysPayableOutstanding, CashConversionCycle).
//
// This package always uses 365, regardless of a period's actual
// granularity (fiscal year, quarter, or month) or PeriodInfo.Type. A
// quarterly or monthly period's revenue/COGS figure is that period's own
// (not annualized) figure, so dividing by 365 rather than ~91 or ~30 would
// understate turnover-implied days for a sub-annual period. This package
// deliberately does not attempt to annualize sub-annual figures or branch
// this constant by PeriodInfo.Type: annualizing a single quarter's revenue
// by multiplying by 4 assumes no seasonality, which this package has no
// basis to assume one way or the other (analytics/workingcapital's
// SeasonalProfile exists specifically because that assumption often does
// NOT hold) — so rather than silently introducing that assumption, every
// days-outstanding figure this package reports is documented here as valid
// exactly as computed: "days of the reported period's own revenue/COGS
// currently tied up," which is directly comparable only against other
// periods of the same granularity. A caller comparing a quarterly DSO
// against an annual one must annualize the underlying revenue/COGS
// themselves, with whatever seasonality assumption is appropriate for
// their business, before calling this package.
const daysInPeriod = 365.0

// assetTurnover computes Total Revenue / Total Assets — how efficiently
// assets generate revenue. Available only if both are available and total
// assets is nonzero.
func assetTurnover(s metrics.Snapshot, totalAssets metrics.MetricValue, period financial.Period) Ratio {
	return marginRatio(RatioAssetTurnover, "Total Revenue / Total Assets", s.TotalRevenue, totalAssets, metrics.MetricTotalRevenue, "Total Revenue", "TOTAL_ASSETS", "Total Assets", period)
}

// receivablesTurnover computes Total Revenue / Accounts Receivable. Available
// only if both are available and accounts receivable is nonzero.
func receivablesTurnover(s metrics.Snapshot, period financial.Period) Ratio {
	return marginRatio(RatioReceivablesTurnover, "Total Revenue / Accounts Receivable", s.TotalRevenue, s.AccountsReceivable, metrics.MetricTotalRevenue, "Total Revenue", metrics.MetricAccountsReceivable, "Accounts Receivable", period)
}

// inventoryTurnover computes Total COGS / Inventory — the conventional
// COGS-basis turnover ratio (not revenue-basis), since inventory is
// consumed at cost, not sold at revenue value. Available only if both are
// available and inventory is nonzero.
func inventoryTurnover(s metrics.Snapshot, period financial.Period) Ratio {
	return marginRatio(RatioInventoryTurnover, "Total COGS / Inventory", s.TotalCOGS, s.Inventory, metrics.MetricTotalCOGS, "Total COGS", metrics.MetricInventory, "Inventory", period)
}

// daysSalesOutstanding computes (Accounts Receivable / Total Revenue) *
// daysInPeriod — the average number of days of the period's own revenue
// currently held in receivables. Available only if both inputs are
// available and revenue is nonzero. See daysInPeriod's doc comment for the
// fixed 365-day convention and its cross-granularity caveat.
func daysSalesOutstanding(s metrics.Snapshot, period financial.Period) Ratio {
	return daysRatio(RatioDaysSalesOutstanding, "(Accounts Receivable / Total Revenue) * 365", s.AccountsReceivable, s.TotalRevenue, metrics.MetricAccountsReceivable, "Accounts Receivable", metrics.MetricTotalRevenue, "Total Revenue", period)
}

// daysInventoryOutstanding computes (Inventory / Total COGS) * daysInPeriod
// — the average number of days of the period's own COGS currently held in
// inventory. Available only if both inputs are available and COGS is
// nonzero.
func daysInventoryOutstanding(s metrics.Snapshot, period financial.Period) Ratio {
	return daysRatio(RatioDaysInventoryOutstanding, "(Inventory / Total COGS) * 365", s.Inventory, s.TotalCOGS, metrics.MetricInventory, "Inventory", metrics.MetricTotalCOGS, "Total COGS", period)
}

// daysPayableOutstanding computes (Accounts Payable / Total COGS) *
// daysInPeriod — the average number of days the business takes to pay its
// own COGS-related payables. Available only if both inputs are available
// and COGS is nonzero.
func daysPayableOutstanding(s metrics.Snapshot, period financial.Period) Ratio {
	return daysRatio(RatioDaysPayableOutstanding, "(Accounts Payable / Total COGS) * 365", s.AccountsPayable, s.TotalCOGS, metrics.MetricAccountsPayable, "Accounts Payable", metrics.MetricTotalCOGS, "Total COGS", period)
}

// daysRatio is the shared (numerator / denominator) * daysInPeriod pattern
// every days-outstanding ratio in this file follows: Available only if
// both numerator and denominator are available and the denominator is
// nonzero.
func daysRatio(name, formula string, numerator, denominator metrics.MetricValue, numCode, numLabel, denomCode, denomLabel string, period financial.Period) Ratio {
	r := Ratio{Metric: name, Period: period, Formula: formula}
	if !numerator.Available || !denominator.Available || denominator.Value == 0 {
		return r
	}
	r.Value = metrics.AvailableValue((numerator.Value / denominator.Value) * daysInPeriod)
	r.Components = []Component{
		{Code: numCode, Label: numLabel, Amount: numerator.Value},
		{Code: denomCode, Label: denomLabel, Amount: denominator.Value},
	}
	return r
}

// cashConversionCycle computes DSO + DIO - DPO — the number of days cash is
// tied up in the operating cycle before being collected. Available only if
// all three component days ratios are available; this package does not
// partially compute a cash-conversion-cycle figure from only some of the
// three, since a caller reading a CCC figure expects it to be the complete
// standard formula, not a silently narrower one.
func cashConversionCycle(dso, dio, dpo Ratio, period financial.Period) Ratio {
	r := Ratio{Metric: RatioCashConversionCycle, Period: period, Formula: "Days Sales Outstanding + Days Inventory Outstanding - Days Payable Outstanding"}
	if !dso.Value.Available || !dio.Value.Available || !dpo.Value.Available {
		return r
	}
	r.Value = metrics.AvailableValue(dso.Value.Value + dio.Value.Value - dpo.Value.Value)
	r.Components = []Component{
		{Code: RatioDaysSalesOutstanding, Label: "Days Sales Outstanding", Amount: dso.Value.Value},
		{Code: RatioDaysInventoryOutstanding, Label: "Days Inventory Outstanding", Amount: dio.Value.Value},
		{Code: RatioDaysPayableOutstanding, Label: "Days Payable Outstanding", Amount: -dpo.Value.Value},
	}
	return r
}
