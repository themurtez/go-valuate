package ratios

import (
	"math"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// calculateGrowthSeries computes one GrowthPoint per chronologically
// adjacent pair in periods/series (which must be the same length and
// already chronologically ordered — see chronologicalPeriods in ratios.go).
// Mirrors financial/metrics.calculateYoYGrowth's exact formula
// ((to-from)/|from|, zero-base handling) but is not restricted to
// fiscal-year-to-fiscal-year comparisons: this package computes growth
// across whatever chronological sequence Input.PeriodMeta establishes,
// since analytics/ratios carries forward none of metrics.Trend's MVP
// fiscal-year-only restriction (see metrics.Trend's doc comment for that
// restriction's own rationale, which does not apply here — this package
// was designed after that restriction was already understood, not before
// it).
func calculateGrowthSeries(periods []financial.Period, series []metrics.MetricValue) []GrowthPoint {
	if len(series) < 2 {
		return nil
	}
	points := make([]GrowthPoint, 0, len(series)-1)
	for i := 1; i < len(series); i++ {
		from, to := series[i-1], series[i]
		point := GrowthPoint{FromPeriod: periods[i-1], ToPeriod: periods[i]}
		switch {
		case !from.Available || !to.Available:
			// Growth left Unavailable.
		case from.Value == 0:
			point.GrowthFromZeroBase = true
		default:
			point.Growth = metrics.AvailableValue((to.Value - from.Value) / math.Abs(from.Value))
		}
		points = append(points, point)
	}
	return points
}

// buildGrowth computes every Growth series from history, which must
// already be in chronological order (the same orderedHistory Calculate
// builds via chronologicalPeriods).
func buildGrowth(periods []financial.Period, history []PeriodRatios) Growth {
	revenue := make([]metrics.MetricValue, len(history))
	grossProfit := make([]metrics.MetricValue, len(history))
	ebitda := make([]metrics.MetricValue, len(history))
	netIncome := make([]metrics.MetricValue, len(history))
	for i, h := range history {
		revenue[i] = h.Snapshot.TotalRevenue
		grossProfit[i] = h.Snapshot.GrossProfit
		ebitda[i] = h.Snapshot.EBITDA
		netIncome[i] = h.Snapshot.NetIncome
	}
	return Growth{
		RevenueGrowth:     calculateGrowthSeries(periods, revenue),
		GrossProfitGrowth: calculateGrowthSeries(periods, grossProfit),
		EBITDAGrowth:      calculateGrowthSeries(periods, ebitda),
		NetIncomeGrowth:   calculateGrowthSeries(periods, netIncome),
	}
}
