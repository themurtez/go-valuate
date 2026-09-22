package concentration

import "fmt"

// buildFlags evaluates every deterministic flag rule against result's
// already-computed fields under thresholds, mirroring
// qoe.buildFlags/cashflow.buildFlags/revenuequality.buildFlags's identical
// "flags read already-computed Result fields" convention. Order is fixed:
// FlagCode's declaration order, then by Period — never Go map order.
func buildFlags(result Result, thresholds Thresholds) []Flag {
	var flags []Flag

	latest := latestPeriodConcentration(result.History)

	if f, ok := highLargestEntityFlag(latest, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := highTop5Flag(latest, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := highHHIFlag(latest, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := increasingConcentrationFlag(result.LargestShareTrend, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := highScenarioImpactFlag(result.Scenarios, thresholds); ok {
		flags = append(flags, f)
	}

	return flags
}

// latestPeriodConcentration returns the last entry in history, or the zero
// PeriodConcentration if history is empty.
func latestPeriodConcentration(history []PeriodConcentration) PeriodConcentration {
	if len(history) == 0 {
		return PeriodConcentration{}
	}
	return history[len(history)-1]
}

func highLargestEntityFlag(latest PeriodConcentration, t Thresholds) (Flag, bool) {
	if !latest.LargestEntityShare.Available || latest.LargestEntityShare.Value < t.HighLargestEntityShareRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagHighLargestEntityConcentration,
		Severity:  FlagSeverityWarning,
		Period:    latest.Period,
		Message:   fmt.Sprintf("largest entity's share of %s's total is %.1f%%, at or above the %.1f%% threshold", latest.Period, latest.LargestEntityShare.Value*100, t.HighLargestEntityShareRatio*100),
		Value:     latest.LargestEntityShare.Value,
		Threshold: t.HighLargestEntityShareRatio,
	}, true
}

// highTop5Flag finds a TopNShare entry with N == 5 in latest.TopNShares; if
// Policy.TopN did not include 5, this flag never triggers (see
// Thresholds.HighTop5ShareRatio's doc comment).
func highTop5Flag(latest PeriodConcentration, t Thresholds) (Flag, bool) {
	for _, s := range latest.TopNShares {
		if s.N != 5 {
			continue
		}
		if !s.Share.Available || s.Share.Value < t.HighTop5ShareRatio {
			return Flag{}, false
		}
		return Flag{
			Code:      FlagHighTop5Concentration,
			Severity:  FlagSeverityWarning,
			Period:    latest.Period,
			Message:   fmt.Sprintf("top 5 entities' share of %s's total is %.1f%%, at or above the %.1f%% threshold", latest.Period, s.Share.Value*100, t.HighTop5ShareRatio*100),
			Value:     s.Share.Value,
			Threshold: t.HighTop5ShareRatio,
		}, true
	}
	return Flag{}, false
}

func highHHIFlag(latest PeriodConcentration, t Thresholds) (Flag, bool) {
	if !latest.HHI.Available || latest.HHI.Value < t.HighHHI {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagHighHHI,
		Severity:  FlagSeverityWarning,
		Period:    latest.Period,
		Message:   fmt.Sprintf("HHI for %s is %.0f, at or above the %.0f highly-concentrated threshold", latest.Period, latest.HHI.Value, t.HighHHI),
		Value:     latest.HHI.Value,
		Threshold: t.HighHHI,
	}, true
}

func increasingConcentrationFlag(trend Trend, t Thresholds) (Flag, bool) {
	if trend.Direction == TrendUnavailable || !trend.FirstValue.Available || !trend.LastValue.Available {
		return Flag{}, false
	}
	increase := trend.LastValue.Value - trend.FirstValue.Value
	if increase < t.IncreasingLargestShareTrendPoints {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagIncreasingConcentration,
		Severity:  FlagSeverityWarning,
		Period:    trend.LastPeriod,
		Message:   fmt.Sprintf("largest entity's share rose from %.1f%% (%s) to %.1f%% (%s), a %.1f-point increase at or above the %.1f-point threshold", trend.FirstValue.Value*100, trend.FirstPeriod, trend.LastValue.Value*100, trend.LastPeriod, increase*100, t.IncreasingLargestShareTrendPoints*100),
		Value:     increase,
		Threshold: t.IncreasingLargestShareTrendPoints,
	}, true
}

// highScenarioImpactFlag evaluates ScenarioLostLargestEntity's, or the
// smallest-N ScenarioTopNLoss's (whichever has a lower N > 0), first
// available RevenueImpactPercent against the threshold — see
// Thresholds.HighScenarioRevenueImpactRatio's doc comment.
func highScenarioImpactFlag(scenarios []Scenario, t Thresholds) (Flag, bool) {
	var candidate *Scenario
	for i := range scenarios {
		s := &scenarios[i]
		if !s.RevenueImpactPercent.Available {
			continue
		}
		if candidate == nil || s.N < candidate.N {
			candidate = s
		}
	}
	if candidate == nil || candidate.RevenueImpactPercent.Value < t.HighScenarioRevenueImpactRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagHighScenarioImpact,
		Severity:  FlagSeverityCritical,
		Period:    candidate.Period,
		Message:   fmt.Sprintf("losing the top %d entit(ies) in %s would remove %.1f%% of total revenue, at or above the %.1f%% threshold", candidate.N, candidate.Period, candidate.RevenueImpactPercent.Value*100, t.HighScenarioRevenueImpactRatio*100),
		Value:     candidate.RevenueImpactPercent.Value,
		Threshold: t.HighScenarioRevenueImpactRatio,
	}, true
}
