package ratios

import "fmt"

// buildSignals evaluates every deterministic signal rule against
// comparisons/history (both already computed — see Calculate's call order)
// and thresholds. Signals are appended in SignalCode declaration order,
// matching Result.Signals' documented order — never Go map order.
func buildSignals(comparisons []Comparison, history []PeriodRatios, thresholds Thresholds) []Signal {
	var signals []Signal

	if s, ok := weakeningLiquiditySignal(comparisons, thresholds); ok {
		signals = append(signals, s)
	}
	if s, ok := risingLeverageSignal(comparisons, thresholds); ok {
		signals = append(signals, s)
	}
	if s, ok := marginCompressionSignal(comparisons, thresholds); ok {
		signals = append(signals, s)
	}
	if s, ok := slowingCollectionsSignal(comparisons, thresholds); ok {
		signals = append(signals, s)
	}
	if s, ok := inventoryBuildupSignal(comparisons, thresholds); ok {
		signals = append(signals, s)
	}
	if s, ok := weakInterestCoverageSignal(history, thresholds); ok {
		signals = append(signals, s)
	}
	if s, ok := profitabilityChangeSignal(comparisons, thresholds, true); ok {
		signals = append(signals, s)
	}
	if s, ok := profitabilityChangeSignal(comparisons, thresholds, false); ok {
		signals = append(signals, s)
	}

	return signals
}

// latestComparison returns the chronologically last Comparison for metric
// from comparisons (calculateComparisons already orders each metric's
// comparisons chronologically within itself), and whether one with an
// available Change existed at all.
func latestComparison(comparisons []Comparison, metric string) (Comparison, bool) {
	var last Comparison
	found := false
	for _, c := range comparisons {
		if c.Metric != metric || !c.Change.Available {
			continue
		}
		last = c
		found = true
	}
	return last, found
}

func weakeningLiquiditySignal(comparisons []Comparison, thresholds Thresholds) (Signal, bool) {
	c, ok := latestComparison(comparisons, RatioCurrentRatio)
	if !ok || c.Change.Value > -thresholds.LiquidityDeclineThreshold {
		return Signal{}, false
	}
	decline := -c.Change.Value
	return Signal{
		Code:      SignalWeakeningLiquidity,
		Severity:  SignalSeverityWarning,
		Period:    c.ToPeriod,
		Message:   fmt.Sprintf("current ratio declined by %.2f from %s to %s (%.2f to %.2f), meeting the %.2f decline threshold", decline, c.FromPeriod, c.ToPeriod, c.FromValue.Value, c.ToValue.Value, thresholds.LiquidityDeclineThreshold),
		Value:     decline,
		Threshold: thresholds.LiquidityDeclineThreshold,
	}, true
}

func risingLeverageSignal(comparisons []Comparison, thresholds Thresholds) (Signal, bool) {
	c, ok := latestComparison(comparisons, RatioDebtToEBITDA)
	if !ok || c.Change.Value < thresholds.LeverageIncreaseThreshold {
		return Signal{}, false
	}
	return Signal{
		Code:      SignalRisingLeverage,
		Severity:  SignalSeverityWarning,
		Period:    c.ToPeriod,
		Message:   fmt.Sprintf("debt/EBITDA rose by %.2fx from %s to %s (%.2fx to %.2fx), meeting the %.2fx increase threshold", c.Change.Value, c.FromPeriod, c.ToPeriod, c.FromValue.Value, c.ToValue.Value, thresholds.LeverageIncreaseThreshold),
		Value:     c.Change.Value,
		Threshold: thresholds.LeverageIncreaseThreshold,
	}, true
}

func marginCompressionSignal(comparisons []Comparison, thresholds Thresholds) (Signal, bool) {
	c, ok := latestComparison(comparisons, RatioEBITDAMargin)
	if !ok || c.Change.Value > -thresholds.MarginCompressionThreshold {
		return Signal{}, false
	}
	decline := -c.Change.Value
	return Signal{
		Code:      SignalMarginCompression,
		Severity:  SignalSeverityWarning,
		Period:    c.ToPeriod,
		Message:   fmt.Sprintf("EBITDA margin compressed by %.1f points from %s to %s (%.1f%% to %.1f%%), meeting the %.1f-point threshold", decline*100, c.FromPeriod, c.ToPeriod, c.FromValue.Value*100, c.ToValue.Value*100, thresholds.MarginCompressionThreshold*100),
		Value:     decline,
		Threshold: thresholds.MarginCompressionThreshold,
	}, true
}

func slowingCollectionsSignal(comparisons []Comparison, thresholds Thresholds) (Signal, bool) {
	c, ok := latestComparison(comparisons, RatioDaysSalesOutstanding)
	if !ok || c.Change.Value < thresholds.CollectionsSlowdownDays {
		return Signal{}, false
	}
	return Signal{
		Code:      SignalSlowingCollections,
		Severity:  SignalSeverityWarning,
		Period:    c.ToPeriod,
		Message:   fmt.Sprintf("days sales outstanding increased by %.1f days from %s to %s (%.1f to %.1f), meeting the %.1f-day threshold", c.Change.Value, c.FromPeriod, c.ToPeriod, c.FromValue.Value, c.ToValue.Value, thresholds.CollectionsSlowdownDays),
		Value:     c.Change.Value,
		Threshold: thresholds.CollectionsSlowdownDays,
	}, true
}

func inventoryBuildupSignal(comparisons []Comparison, thresholds Thresholds) (Signal, bool) {
	c, ok := latestComparison(comparisons, RatioDaysInventoryOutstanding)
	if !ok || c.Change.Value < thresholds.InventoryBuildupDays {
		return Signal{}, false
	}
	return Signal{
		Code:      SignalInventoryBuildup,
		Severity:  SignalSeverityWarning,
		Period:    c.ToPeriod,
		Message:   fmt.Sprintf("days inventory outstanding increased by %.1f days from %s to %s (%.1f to %.1f), meeting the %.1f-day threshold", c.Change.Value, c.FromPeriod, c.ToPeriod, c.FromValue.Value, c.ToValue.Value, thresholds.InventoryBuildupDays),
		Value:     c.Change.Value,
		Threshold: thresholds.InventoryBuildupDays,
	}, true
}

// weakInterestCoverageSignal checks only the most recent period in history
// (which must already be chronologically ordered when available — see
// Calculate) since InterestCoverage is a level, not a period-over-period
// change, unlike every other signal in this file.
func weakInterestCoverageSignal(history []PeriodRatios, thresholds Thresholds) (Signal, bool) {
	if len(history) == 0 {
		return Signal{}, false
	}
	last := history[len(history)-1]
	if !last.InterestCoverage.Value.Available || last.InterestCoverage.Value.Value > thresholds.WeakInterestCoverageRatio {
		return Signal{}, false
	}
	return Signal{
		Code:      SignalWeakInterestCoverage,
		Severity:  SignalSeverityCritical,
		Period:    last.Period,
		Message:   fmt.Sprintf("interest coverage is %.2fx in %s, at or below the %.2fx threshold", last.InterestCoverage.Value.Value, last.Period, thresholds.WeakInterestCoverageRatio),
		Value:     last.InterestCoverage.Value.Value,
		Threshold: thresholds.WeakInterestCoverageRatio,
	}, true
}

// profitabilityChangeSignal checks EBITDAMargin's most recent
// period-over-period Comparison against
// Thresholds.ProfitabilityChangeThreshold, firing
// SignalImprovingProfitability when improving is true and the change is a
// positive move at or beyond the threshold, or SignalDeterioratingProfitability
// when improving is false and the change is a negative move at or beyond
// the threshold. Both directions read the same underlying Comparison —
// they are mutually exclusive per call (only one of the two can trigger
// from a single Comparison, since a change cannot be both a large positive
// and a large negative move at once) but are evaluated as two separate
// calls (see buildSignals) since they are two distinct SignalCode values a
// caller may want to filter on independently.
func profitabilityChangeSignal(comparisons []Comparison, thresholds Thresholds, improving bool) (Signal, bool) {
	c, ok := latestComparison(comparisons, RatioEBITDAMargin)
	if !ok {
		return Signal{}, false
	}
	if improving {
		if c.Change.Value < thresholds.ProfitabilityChangeThreshold {
			return Signal{}, false
		}
		return Signal{
			Code:      SignalImprovingProfitability,
			Severity:  SignalSeverityInfo,
			Period:    c.ToPeriod,
			Message:   fmt.Sprintf("EBITDA margin improved by %.1f points from %s to %s (%.1f%% to %.1f%%), meeting the %.1f-point threshold", c.Change.Value*100, c.FromPeriod, c.ToPeriod, c.FromValue.Value*100, c.ToValue.Value*100, thresholds.ProfitabilityChangeThreshold*100),
			Value:     c.Change.Value,
			Threshold: thresholds.ProfitabilityChangeThreshold,
		}, true
	}
	if c.Change.Value > -thresholds.ProfitabilityChangeThreshold {
		return Signal{}, false
	}
	decline := -c.Change.Value
	return Signal{
		Code:      SignalDeterioratingProfitability,
		Severity:  SignalSeverityWarning,
		Period:    c.ToPeriod,
		Message:   fmt.Sprintf("EBITDA margin deteriorated by %.1f points from %s to %s (%.1f%% to %.1f%%), meeting the %.1f-point threshold", decline*100, c.FromPeriod, c.ToPeriod, c.FromValue.Value*100, c.ToValue.Value*100, thresholds.ProfitabilityChangeThreshold*100),
		Value:     decline,
		Threshold: thresholds.ProfitabilityChangeThreshold,
	}, true
}
