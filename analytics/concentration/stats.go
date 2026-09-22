package concentration

import (
	"math"

	"github.com/themurtez/go-valuate/financial"
)

// trendPoint is one (period, value) observation extracted from History for
// Trend computation.
type trendPoint struct {
	period financial.Period
	value  ConcentrationValue
}

// calculateTrend characterizes history's extract(period)-derived series by
// comparing the first vs. last available observation, in history's own
// order (assumed chronological). Returns TrendUnavailable if fewer than two
// available observations exist. Mirrors
// workingcapital.calculateTrend/revenuequality.calculateTrend's identical
// method exactly, including the zero-first-value fallback.
func calculateTrend(history []PeriodConcentration, extract func(PeriodConcentration) ConcentrationValue) Trend {
	var withValue []trendPoint
	for _, p := range history {
		v := extract(p)
		if v.Available {
			withValue = append(withValue, trendPoint{period: p.Period, value: v})
		}
	}
	if len(withValue) < 2 {
		return Trend{Direction: TrendUnavailable}
	}

	first := withValue[0]
	last := withValue[len(withValue)-1]

	t := Trend{
		FirstPeriod: first.period,
		LastPeriod:  last.period,
		FirstValue:  first.value,
		LastValue:   last.value,
	}

	delta := last.value.Value - first.value.Value
	if first.value.Value == 0 {
		if last.value.Value == 0 {
			t.Direction = TrendStable
			return t
		}
		t.Direction = classifyDirection(delta / math.Abs(last.value.Value))
		return t
	}

	percentChange := delta / math.Abs(first.value.Value)
	t.PercentChange = AvailableValue(percentChange)
	t.Direction = classifyDirection(percentChange)
	return t
}

// classifyDirection maps a percent change to a TrendDirection using
// TrendFlatBandPercent as the stable/non-stable boundary.
func classifyDirection(percentChange float64) TrendDirection {
	if percentChange > TrendFlatBandPercent {
		return TrendIncreasing
	}
	if percentChange < -TrendFlatBandPercent {
		return TrendDeclining
	}
	return TrendStable
}
