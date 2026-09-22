package revenuequality

import (
	"fmt"
	"math"
)

// buildFlags evaluates every deterministic flag rule against result's
// already-computed fields under thresholds, mirroring
// qoe.buildFlags/cashflow.buildFlags's identical "flags read
// already-computed Result fields" convention. Order is fixed: FlagCode's
// declaration order, then by Period — never Go map order.
func buildFlags(result Result, thresholds Thresholds) []Flag {
	var flags []Flag

	if f, ok := decliningRecurringMixFlag(result.RevenueTrend, result.TotalRevenueHistory, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := growthDependentOnNewCustomersFlag(result.CustomerTransitions, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := highLostCustomerRevenueFlag(result.CustomerTransitions, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := volatileRevenueFlag(result.RevenueVolatility, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := onePeriodSpikeFlag(result.TotalRevenueHistory, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := shrinkingExistingCustomerBaseFlag(result.CustomerTransitions, thresholds); ok {
		flags = append(flags, f)
	}

	return flags
}

// decliningRecurringMixFlag compares TotalRevenueHistory's first and last
// available RecurringPercent observations (independent of trend's own
// TotalRevenue-based first/last periods, since a period can have
// TotalRevenue available but RecurringPercent unavailable — see
// PeriodRevenue.RecurringPercent's availability rule).
func decliningRecurringMixFlag(trend Trend, history []PeriodRevenue, t Thresholds) (Flag, bool) {
	var withMix []PeriodRevenue
	for _, p := range history {
		if p.RecurringPercent.Available {
			withMix = append(withMix, p)
		}
	}
	if len(withMix) < 2 {
		return Flag{}, false
	}

	first := withMix[0]
	last := withMix[len(withMix)-1]
	decline := first.RecurringPercent.Value - last.RecurringPercent.Value
	if decline < t.DecliningRecurringMixPoints {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagDecliningRecurringMix,
		Severity:  FlagSeverityWarning,
		Period:    last.Period,
		Message:   fmt.Sprintf("recurring revenue mix fell from %.1f%% (%s) to %.1f%% (%s), a %.1f-point decline at or above the %.1f-point threshold", first.RecurringPercent.Value*100, first.Period, last.RecurringPercent.Value*100, last.Period, decline*100, t.DecliningRecurringMixPoints*100),
		Value:     decline,
		Threshold: t.DecliningRecurringMixPoints,
	}, true
}

// growthDependentOnNewCustomersFlag reads the most recent CustomerTransition
// (last entry in transitions, since transitions is built in chronological
// order — see calculateCustomerTransitions) and compares NewCustomerRevenue
// against TotalRevenueGrowth, so this flag does not require
// TotalRevenueHistory's GrowthPoint to be separately available for the same
// period pair.
func growthDependentOnNewCustomersFlag(transitions []CustomerTransition, t Thresholds) (Flag, bool) {
	if len(transitions) == 0 {
		return Flag{}, false
	}
	last := transitions[len(transitions)-1]
	if !last.NewCustomerRevenue.Available || !last.TotalRevenueGrowth.Available {
		return Flag{}, false
	}

	totalGrowth := last.TotalRevenueGrowth.Value
	if totalGrowth <= 0 {
		return Flag{}, false
	}

	ratio := last.NewCustomerRevenue.Value / totalGrowth
	if ratio < t.NewCustomerGrowthDependenceRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagGrowthDependentOnNewCustomers,
		Severity:  FlagSeverityWarning,
		Period:    last.ToPeriod,
		Message:   fmt.Sprintf("new-customer revenue is %.1f%% of total revenue growth from %s to %s, at or above the %.1f%% threshold", ratio*100, last.FromPeriod, last.ToPeriod, t.NewCustomerGrowthDependenceRatio*100),
		Value:     ratio,
		Threshold: t.NewCustomerGrowthDependenceRatio,
	}, true
}

func highLostCustomerRevenueFlag(transitions []CustomerTransition, t Thresholds) (Flag, bool) {
	if len(transitions) == 0 {
		return Flag{}, false
	}
	last := transitions[len(transitions)-1]
	if !last.LostCustomerRevenue.Available || !last.FromPeriodTotalRevenue.Available {
		return Flag{}, false
	}

	fromTotal := last.FromPeriodTotalRevenue.Value
	if fromTotal <= 0 {
		return Flag{}, false
	}

	ratio := last.LostCustomerRevenue.Value / fromTotal
	if ratio < t.HighLostCustomerRevenueRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagHighLostCustomerRevenue,
		Severity:  FlagSeverityWarning,
		Period:    last.ToPeriod,
		Message:   fmt.Sprintf("lost-customer revenue from %s to %s is %.1f%% of %s's total customer revenue, at or above the %.1f%% threshold", last.FromPeriod, last.ToPeriod, ratio*100, last.FromPeriod, t.HighLostCustomerRevenueRatio*100),
		Value:     ratio,
		Threshold: t.HighLostCustomerRevenueRatio,
	}, true
}

func volatileRevenueFlag(vol VolatilityResult, t Thresholds) (Flag, bool) {
	if !vol.Value.Available || vol.Value.Value < t.VolatileRevenueRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagVolatileRevenue,
		Severity:  FlagSeverityWarning,
		Message:   fmt.Sprintf("period-over-period revenue growth volatility is %.1f%%, at or above the %.1f%% threshold", vol.Value.Value*100, t.VolatileRevenueRatio*100),
		Value:     vol.Value.Value,
		Threshold: t.VolatileRevenueRatio,
	}, true
}

// onePeriodSpikeFlag flags at most one period: the single largest available
// TotalRevenue observation, if it exceeds the average of every other
// available observation by at least Thresholds.OnePeriodSpikeRatio.
// Requires at least 3 available observations (so "every other period's
// average" is meaningful — with only 2 periods, any growth looks like a
// spike relative to a single prior value, which this flag is not intended
// to catch; RevenueTrend/RevenueGrowth already surface a simple two-period
// increase).
func onePeriodSpikeFlag(history []PeriodRevenue, t Thresholds) (Flag, bool) {
	var withValue []PeriodRevenue
	for _, p := range history {
		if p.TotalRevenue.Available {
			withValue = append(withValue, p)
		}
	}
	if len(withValue) < 3 {
		return Flag{}, false
	}

	maxIdx := 0
	for i, p := range withValue {
		if p.TotalRevenue.Value > withValue[maxIdx].TotalRevenue.Value {
			maxIdx = i
		}
	}

	var othersSum float64
	othersCount := len(withValue) - 1
	for i, p := range withValue {
		if i != maxIdx {
			othersSum += p.TotalRevenue.Value
		}
	}
	othersAvg := othersSum / float64(othersCount)
	if othersAvg <= 0 {
		return Flag{}, false
	}

	spike := withValue[maxIdx]
	ratio := (spike.TotalRevenue.Value - othersAvg) / othersAvg
	if ratio < t.OnePeriodSpikeRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagOnePeriodSpike,
		Severity:  FlagSeverityInfo,
		Period:    spike.Period,
		Message:   fmt.Sprintf("revenue in %s is %.1f%% above the average of every other period, at or above the %.1f%% one-period-spike threshold", spike.Period, ratio*100, t.OnePeriodSpikeRatio*100),
		Value:     ratio,
		Threshold: t.OnePeriodSpikeRatio,
	}, true
}

func shrinkingExistingCustomerBaseFlag(transitions []CustomerTransition, t Thresholds) (Flag, bool) {
	if len(transitions) == 0 {
		return Flag{}, false
	}
	last := transitions[len(transitions)-1]
	if !last.ExistingCustomerBaseChange.Available || last.ExistingCustomerBaseChange.Value >= 0 {
		return Flag{}, false
	}
	if !last.FromPeriodTotalRevenue.Available {
		return Flag{}, false
	}

	fromTotal := last.FromPeriodTotalRevenue.Value
	if fromTotal <= 0 {
		return Flag{}, false
	}

	ratio := math.Abs(last.ExistingCustomerBaseChange.Value) / fromTotal
	if ratio < t.ShrinkingExistingBaseRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagShrinkingExistingCustomerBase,
		Severity:  FlagSeverityWarning,
		Period:    last.ToPeriod,
		Message:   fmt.Sprintf("existing customer base revenue shrank %.1f%% from %s to %s, at or above the %.1f%% threshold", ratio*100, last.FromPeriod, last.ToPeriod, t.ShrinkingExistingBaseRatio*100),
		Value:     ratio,
		Threshold: t.ShrinkingExistingBaseRatio,
	}, true
}
