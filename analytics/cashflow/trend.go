package cashflow

import (
	"math"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// extractOCF pulls the OperatingCashFlow series from history, in the same
// (assumed chronological) order.
func extractOCF(history []Bridge) []cashValuePeriod {
	out := make([]cashValuePeriod, len(history))
	for i, b := range history {
		out[i] = cashValuePeriod{period: b.Period, value: b.OperatingCashFlow.Value, available: b.OperatingCashFlow.Available}
	}
	return out
}

// extractFCF pulls the FreeCashFlow series from history.
func extractFCF(history []Bridge) []cashValuePeriod {
	out := make([]cashValuePeriod, len(history))
	for i, b := range history {
		out[i] = cashValuePeriod{period: b.Period, value: b.FreeCashFlow.Value, available: b.FreeCashFlow.Available}
	}
	return out
}

// extractConversion pulls the EBITDAToFreeCashFlow series from conversion.
func extractConversion(conversion []ConversionRatios) []cashValuePeriod {
	out := make([]cashValuePeriod, len(conversion))
	for i, c := range conversion {
		out[i] = cashValuePeriod{period: c.Period, value: c.EBITDAToFreeCashFlow.Value, available: c.EBITDAToFreeCashFlow.Available}
	}
	return out
}

// cashValuePeriod is an internal (period, value, availability) tuple used
// only to feed calculateTrend, so that function doesn't need three
// near-identical overloads for Bridge/ConversionRatios fields.
type cashValuePeriod struct {
	period    financial.Period
	value     float64
	available bool
}

// calculateTrend characterizes series' first vs. last available
// observation, mirroring workingcapital.calculateTrend/
// ratios.calculateRatioTrend's identical method exactly (see either for the
// full rationale, including the zero-first-value fallback below).
func calculateTrend(series []cashValuePeriod) Trend {
	var withValue []cashValuePeriod
	for _, v := range series {
		if v.available {
			withValue = append(withValue, v)
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
		FirstValue:  metrics.AvailableValue(first.value),
		LastValue:   metrics.AvailableValue(last.value),
	}

	delta := last.value - first.value
	if first.value == 0 {
		if last.value == 0 {
			t.Direction = TrendStable
			return t
		}
		t.Direction = classifyDirection(delta / math.Abs(last.value))
		return t
	}

	percentChange := delta / math.Abs(first.value)
	t.PercentChange = metrics.AvailableValue(percentChange)
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
