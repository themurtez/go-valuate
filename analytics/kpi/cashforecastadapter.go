package kpi

import "github.com/themurtez/go-valuate/accounting/cashforecast"

// CashForecastMetricCode is the fixed "cashforecast.*" namespace this
// adapter publishes — task section 32's example list.
const (
	CashForecastMetricEndingCash        = "cashforecast.ending_cash"
	CashForecastMetricMaximumFundingGap = "cashforecast.maximum_funding_gap"
)

// CashForecastMetricValues converts one accounting/cashforecast.
// ScenarioResult (Result.BaseScenario or one element of Result.Scenarios)
// into this package's MetricValue slice, tagged under period (a
// caller-chosen label — accounting/cashforecast.ScenarioResult itself
// carries no single period Code of its own; it is a 13-week rolling
// forecast, not accounting/cashforecast.Result.HorizonWeeks-many separate
// periods, so this adapter deliberately republishes only the two
// scenario-level summary figures task section 32 lists, at whatever
// single period label the caller's own periodization scheme assigns to
// "this forecast run" — e.g. its ForecastStartDate week). Reimplements no
// formula: EndingCash and MaximumFundingGap are republished exactly as
// accounting/cashforecast computed them.
//
// MaximumFundingGap is reported unavailable (rather than a misleading 0)
// when Summary.ThresholdAvailable is false — accounting/cashforecast
// itself never computes a funding gap without a resolved minimum-cash
// threshold (see LiquiditySummary's own doc comment), and this adapter
// preserves that same distinction rather than collapsing it into 0.
func CashForecastMetricValues(sr cashforecast.ScenarioResult, period string, currency string) []MetricValue {
	currencyUnit := Unit{Kind: UnitCurrency, CurrencyCode: currency}
	return []MetricValue{
		{
			Code: CashForecastMetricEndingCash, Period: period,
			Value: sr.Summary.EndingCash, Available: true,
			Unit: currencyUnit, Aggregation: AggregationLast, Source: "accounting/cashforecast",
		},
		{
			Code: CashForecastMetricMaximumFundingGap, Period: period,
			Value: sr.Summary.MaximumFundingGap, Available: sr.Summary.ThresholdAvailable,
			Unit: currencyUnit, Aggregation: AggregationMax, Source: "accounting/cashforecast",
		},
	}
}
