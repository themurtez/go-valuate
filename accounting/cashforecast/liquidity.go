package cashforecast

// buildLiquiditySummary computes LiquiditySummary from a scenario's
// already-computed WeeklyForecast slice (weeks must be non-empty and in
// chronological order — see the task's section 30).
func buildLiquiditySummary(weeks []WeeklyForecast, minimumThreshold float64, haveThreshold bool, facilities []CreditFacility) LiquiditySummary {
	s := LiquiditySummary{}
	if len(weeks) == 0 {
		return s
	}

	s.StartingCash = weeks[0].OpeningCash
	s.EndingCash = weeks[len(weeks)-1].EndingCash

	lowest := weeks[0].EndingCash
	lowestWeek := weeks[0].WeekNumber
	for _, w := range weeks {
		s.TotalInflows += w.Inflows.Total
		s.TotalOutflows += w.Outflows.Total
		if w.EndingCash < lowest {
			lowest = w.EndingCash
			lowestWeek = w.WeekNumber
		}
		if w.EndingCash < 0 && !s.NegativeCashReached {
			s.NegativeCashReached = true
			s.FirstNegativeCashWeek = w.WeekNumber
		}
	}
	s.NetChange = s.TotalInflows - s.TotalOutflows
	s.LowestCashBalance = lowest
	s.LowestCashWeek = lowestWeek

	if haveThreshold {
		s.ThresholdAvailable = true
		s.MinimumCashThreshold = minimumThreshold
		var maxGap float64
		for _, w := range weeks {
			if w.Threshold.BelowMinimum {
				if s.FirstWeekBelowMinimum == 0 {
					s.FirstWeekBelowMinimum = w.WeekNumber
				}
				s.WeeksBelowMinimum++
			}
			if w.Threshold.FundingGap > maxGap {
				maxGap = w.Threshold.FundingGap
			}
		}
		s.MaximumFundingGap = maxGap
		s.EndingFundingGap = weeks[len(weeks)-1].Threshold.FundingGap

		s.RequiredFunding = RequiredFunding{Available: true, RequiredAtStart: maxGap, PeakCumulativeGap: maxGap}

		capacity := totalFacilityCapacity(facilities)
		if capacity > 0 {
			s.FacilityCapacity = capacity
			remainder := maxGap - capacity
			if remainder < 0 {
				remainder = 0
			}
			s.GapAfterFacility = AvailableAmount(remainder)
		}
	}

	return s
}

// buildThresholdPosition computes ThresholdPosition for one week's
// EndingCash against the resolved minimum-cash threshold.
func buildThresholdPosition(endingCash, threshold float64, haveThreshold bool) ThresholdPosition {
	if !haveThreshold {
		return ThresholdPosition{}
	}
	tp := ThresholdPosition{Available: true, MinimumCash: threshold}
	if endingCash < threshold {
		tp.BelowMinimum = true
		tp.FundingGap = threshold - endingCash
	} else {
		tp.Headroom = endingCash - threshold
	}
	return tp
}

// buildDeltaVsBase computes DeltaVsBase for a scenario's weeks against the
// base scenario's weeks — see the task's section 29. Both slices must be
// the same length and week-aligned (guaranteed by construction, since
// every scenario is built over the same horizon).
func buildDeltaVsBase(scenarioWeeks, baseWeeks []WeeklyForecast) DeltaVsBase {
	if len(scenarioWeeks) == 0 || len(scenarioWeeks) != len(baseWeeks) {
		return DeltaVsBase{}
	}
	d := DeltaVsBase{Available: true, Weekly: make([]WeeklyDelta, 0, len(scenarioWeeks))}
	for i := range scenarioWeeks {
		wd := WeeklyDelta{
			WeekNumber:       scenarioWeeks[i].WeekNumber,
			NetCashFlowDelta: scenarioWeeks[i].NetCashFlow - baseWeeks[i].NetCashFlow,
			EndingCashDelta:  scenarioWeeks[i].EndingCash - baseWeeks[i].EndingCash,
			FundingGapDelta:  scenarioWeeks[i].Threshold.FundingGap - baseWeeks[i].Threshold.FundingGap,
		}
		d.Weekly = append(d.Weekly, wd)
	}
	// EndingCashDelta/FundingGapDelta are the final week's snapshot
	// deltas (the ending position, not a sum); NetCashFlowDelta is the
	// cumulative change in net cash flow across the whole horizon (a sum
	// over every week, since each week's flow is independent).
	last := len(scenarioWeeks) - 1
	d.EndingCashDelta = scenarioWeeks[last].EndingCash - baseWeeks[last].EndingCash
	d.FundingGapDelta = scenarioWeeks[last].Threshold.FundingGap - baseWeeks[last].Threshold.FundingGap
	for _, wd := range d.Weekly {
		d.NetCashFlowDelta += wd.NetCashFlowDelta
	}

	return d
}
