package kpi

import "github.com/themurtez/go-valuate/accounting/profitability"

// ProfitabilityMetricCode is the fixed "profitability.*" namespace this
// adapter publishes — task section 32's example list.
const ProfitabilityMetricContributionMargin = "profitability.contribution_margin"

// ProfitabilityMetricValues converts one accounting/profitability.
// BusinessPeriodTotals (one element of Result.BusinessTotals.Periods)
// into this package's MetricValue slice. Reimplements no formula:
// Margins.ContributionMargin is republished exactly as
// accounting/profitability computed it (already a percentage-style
// margin — see accounting/profitability.entityresult.go's marginValue
// helper — so it is correctly AggregationNotAggregatable here, matching
// task section 17's "Margin % -> NOT_AGGREGATABLE" example precisely).
func ProfitabilityMetricValues(bpt profitability.BusinessPeriodTotals) []MetricValue {
	cm := bpt.Margins.ContributionMargin
	return []MetricValue{
		{
			Code: ProfitabilityMetricContributionMargin, Period: bpt.Period,
			Value: cm.Amount, Available: cm.Available,
			Unit: Unit{Kind: UnitPercent}, Aggregation: AggregationNotAggregatable, Source: "accounting/profitability",
		},
	}
}
