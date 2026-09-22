package qoe

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// ownerDiscretionaryTypes is the set of adjustments.Type values this
// package treats as "owner-discretionary" for
// FlagLargeOwnerDiscretionaryComponent — every built-in type whose default
// definition names an owner or personal-use expense (see
// adjustments.buildTypeRegistry's doc comments for each).
// TypeOwnerDiscretionaryExpense is itself one of the built-in types (the
// generic bucket for a personal/discretionary expense not covered by a more
// specific type), included here alongside the three specific ones it
// generalizes.
var ownerDiscretionaryTypes = map[adjustments.Type]bool{
	adjustments.TypeOwnerCompensationNormalization: true,
	adjustments.TypeOwnerDiscretionaryExpense:      true,
	adjustments.TypePersonalVehicle:                true,
	adjustments.TypePersonalTravel:                 true,
}

// nonOperatingIncomeTypes is the set of adjustments.Type values this
// package treats as "non-operating income being removed" for
// FlagNonOperatingIncomeSupportingEarnings.
var nonOperatingIncomeTypes = map[adjustments.Type]bool{
	adjustments.TypeNonOperatingIncome: true,
	adjustments.TypeUnusualGain:        true,
}

// buildFlags evaluates every deterministic quality-flag rule against result
// (which must already have History/Adjustments/Recurrence/Ratios/
// Maintainable*/trend fields populated — see Calculate's call order) and
// thresholds. Flags are appended in FlagCode declaration order, then by
// Period within a given code, matching Result.Flags' documented order —
// never Go map order.
func buildFlags(result Result, thresholds Thresholds) []Flag {
	var flags []Flag

	if f, ok := largeNormalizationBurdenFlag(result.Ratios, thresholds); ok {
		flags = append(flags, f)
	}
	flags = append(flags, decliningEBITDADespiteRevenueGrowthFlags(result)...)
	flags = append(flags, volatileEarningsFlags(result, thresholds)...)
	if f, ok := inconsistentMarginsFlag(result.EBITDAMarginTrend, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := largeOwnerDiscretionaryComponentFlag(result.History, thresholds); ok {
		flags = append(flags, f)
	}
	if f, ok := repeatedOneTimeFlag(result.Recurrence); ok {
		flags = append(flags, f)
	}
	if f, ok := nonOperatingIncomeSupportingEarningsFlag(result.History, thresholds); ok {
		flags = append(flags, f)
	}
	flags = append(flags, negativeOrNearZeroMaintainableEarningsFlags(result, thresholds)...)

	return flags
}

func largeNormalizationBurdenFlag(r Ratios, thresholds Thresholds) (Flag, bool) {
	threshold := thresholds.LargeNormalizationBurdenRatio
	worst := 0.0
	worstLabel := ""
	if r.AdjustmentToEBITDA.Available && r.AdjustmentToEBITDA.Value > worst {
		worst = r.AdjustmentToEBITDA.Value
		worstLabel = "EBITDA"
	}
	if r.AdjustmentToSDE.Available && r.AdjustmentToSDE.Value > worst {
		worst = r.AdjustmentToSDE.Value
		worstLabel = "SDE"
	}
	if worstLabel == "" || worst < threshold {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagLargeNormalizationBurden,
		Severity:  FlagSeverityWarning,
		Period:    r.Period,
		Message:   fmt.Sprintf("confirmed adjustments total %.1f%% of reported %s, exceeding the %.1f%% threshold", worst*100, worstLabel, threshold*100),
		Value:     worst,
		Threshold: threshold,
	}, true
}

// decliningEBITDADespiteRevenueGrowthFlags checks every adjacent
// fiscal-year transition covered by both RevenueGrowth and
// EBITDAMarginTrend (via History's ReportedEBITDA series) for revenue
// growing while EBITDA declined. Uses reported (not normalized) EBITDA,
// since this flag is about the underlying business trend, not about
// adjustment normalization.
func decliningEBITDADespiteRevenueGrowthFlags(result Result) []Flag {
	if len(result.RevenueGrowth) == 0 {
		return nil
	}
	ebitdaByPeriod := make(map[string]float64, len(result.History))
	ebitdaAvailable := make(map[string]bool, len(result.History))
	for _, pf := range result.History {
		ebitdaByPeriod[string(pf.Period)] = pf.ReportedEBITDA.Value
		ebitdaAvailable[string(pf.Period)] = pf.ReportedEBITDA.Available
	}

	var flags []Flag
	for _, gp := range result.RevenueGrowth {
		if !gp.Growth.Available || gp.Growth.Value <= 0 {
			continue
		}
		if !ebitdaAvailable[gp.FromPeriod] || !ebitdaAvailable[gp.ToPeriod] {
			continue
		}
		from, to := ebitdaByPeriod[gp.FromPeriod], ebitdaByPeriod[gp.ToPeriod]
		if to >= from {
			continue
		}
		flags = append(flags, Flag{
			Code:     FlagDecliningEBITDADespiteRevenueGrowth,
			Severity: FlagSeverityWarning,
			Period:   financial.Period(gp.ToPeriod),
			Message: fmt.Sprintf(
				"revenue grew %.1f%% from %s to %s while reported EBITDA declined from %.2f to %.2f",
				gp.Growth.Value*100, gp.FromPeriod, gp.ToPeriod, from, to,
			),
			Value:     to - from,
			Threshold: 0,
		})
	}
	return flags
}

func volatileEarningsFlags(result Result, thresholds Thresholds) []Flag {
	threshold := thresholds.VolatileEarningsRatio
	var flags []Flag
	if result.EBITDAVolatility.Value.Available && result.EBITDAVolatility.Value.Value >= threshold {
		flags = append(flags, Flag{
			Code:      FlagVolatileEarnings,
			Severity:  FlagSeverityWarning,
			Message:   fmt.Sprintf("EBITDA year-over-year growth volatility is %.1f%%, exceeding the %.1f%% threshold", result.EBITDAVolatility.Value.Value*100, threshold*100),
			Value:     result.EBITDAVolatility.Value.Value,
			Threshold: threshold,
		})
	}
	if result.SDEVolatility.Value.Available && result.SDEVolatility.Value.Value >= threshold {
		flags = append(flags, Flag{
			Code:      FlagVolatileEarnings,
			Severity:  FlagSeverityWarning,
			Message:   fmt.Sprintf("SDE year-over-year growth volatility is %.1f%%, exceeding the %.1f%% threshold", result.SDEVolatility.Value.Value*100, threshold*100),
			Value:     result.SDEVolatility.Value.Value,
			Threshold: threshold,
		})
	}
	return flags
}

// inconsistentMarginsFlag flags when EBITDAMarginTrend's available margin
// values swing by more than Thresholds.InconsistentMarginSwing (max - min,
// in raw decimal terms) across the fiscal years used. Requires at least two
// available margin points; a single-year trend has no swing to measure.
func inconsistentMarginsFlag(trend []metrics.MarginPoint, thresholds Thresholds) (Flag, bool) {
	var minV, maxV float64
	n := 0
	for _, mp := range trend {
		if !mp.Margin.Available {
			continue
		}
		if n == 0 || mp.Margin.Value < minV {
			minV = mp.Margin.Value
		}
		if n == 0 || mp.Margin.Value > maxV {
			maxV = mp.Margin.Value
		}
		n++
	}
	if n < 2 {
		return Flag{}, false
	}

	swing := maxV - minV
	threshold := thresholds.InconsistentMarginSwing
	if swing < threshold {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagInconsistentMargins,
		Severity:  FlagSeverityWarning,
		Message:   fmt.Sprintf("EBITDA margin swings %.1f percentage points across the fiscal years used (%.1f%% to %.1f%%), exceeding the %.1f-point threshold", swing*100, minV*100, maxV*100, threshold*100),
		Value:     swing,
		Threshold: threshold,
	}, true
}

func largeOwnerDiscretionaryComponentFlag(history []PeriodFigures, thresholds Thresholds) (Flag, bool) {
	if len(history) == 0 {
		return Flag{}, false
	}
	last := history[len(history)-1]
	if !last.NormalizedSDE.Available || last.NormalizedSDE.Value == 0 {
		return Flag{}, false
	}

	total := 0.0
	for _, line := range last.Adjustments.SDEBridge.Applied {
		if ownerDiscretionaryTypes[line.Adjustment.Type] {
			total += line.Adjustment.Amount
		}
	}
	if total == 0 {
		return Flag{}, false
	}

	share := total / math.Abs(last.NormalizedSDE.Value)
	threshold := thresholds.OwnerDiscretionaryShareOfSDE
	if share < threshold {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagLargeOwnerDiscretionaryComponent,
		Severity:  FlagSeverityInfo,
		Period:    last.Period,
		Message:   fmt.Sprintf("owner-discretionary adjustments total %.1f%% of normalized SDE, exceeding the %.1f%% threshold", share*100, threshold*100),
		Value:     share,
		Threshold: threshold,
	}, true
}

func repeatedOneTimeFlag(recurrence []RecurrencePattern) (Flag, bool) {
	var flagged []RecurrencePattern
	for _, r := range recurrence {
		if r.LikelyNotNonRecurring {
			flagged = append(flagged, r)
		}
	}
	if len(flagged) == 0 {
		return Flag{}, false
	}
	msg := fmt.Sprintf("%d adjustment type(s) nominally treated as non-recurring appeared in 2 or more separate periods: ", len(flagged))
	for i, r := range flagged {
		if i > 0 {
			msg += ", "
		}
		msg += fmt.Sprintf("%s (%d periods)", r.Type, r.Count)
	}
	return Flag{
		Code:     FlagRepeatedOneTimeAdjustments,
		Severity: FlagSeverityWarning,
		Message:  msg,
		Value:    float64(len(flagged)),
	}, true
}

func nonOperatingIncomeSupportingEarningsFlag(history []PeriodFigures, thresholds Thresholds) (Flag, bool) {
	if len(history) == 0 {
		return Flag{}, false
	}
	last := history[len(history)-1]
	if !last.ReportedEBITDA.Available || last.ReportedEBITDA.Value == 0 {
		return Flag{}, false
	}

	total := 0.0
	for _, line := range last.Adjustments.EBITDABridge.Applied {
		if nonOperatingIncomeTypes[line.Adjustment.Type] {
			total += line.Adjustment.Amount
		}
	}
	if total == 0 {
		return Flag{}, false
	}

	share := total / math.Abs(last.ReportedEBITDA.Value)
	threshold := thresholds.NonOperatingIncomeShareOfEBITDA
	if share < threshold {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagNonOperatingIncomeSupportingEarnings,
		Severity:  FlagSeverityWarning,
		Period:    last.Period,
		Message:   fmt.Sprintf("non-operating income/gains being removed total %.1f%% of reported EBITDA, exceeding the %.1f%% threshold", share*100, threshold*100),
		Value:     share,
		Threshold: threshold,
	}, true
}

// negativeOrNearZeroMaintainableEarningsFlags implements the two-leg
// materiality-style test documented on
// Thresholds.NearZeroMaintainableEarnings/
// NearZeroMaintainableEarningsPercentOfRevenue: either the absolute floor
// or the percent-of-revenue leg crossing is sufficient, mirroring
// review.IsMaterial's OR logic. Revenue is read from the most recent
// period's Snapshot (result.History's last entry); if unavailable, only
// the absolute-floor leg applies.
func negativeOrNearZeroMaintainableEarningsFlags(result Result, thresholds Thresholds) []Flag {
	var revenue metrics.MetricValue
	if len(result.History) > 0 {
		revenue = result.History[len(result.History)-1].Snapshot.TotalRevenue
	}

	nearZero := func(value float64) (bool, float64) {
		if value <= thresholds.NearZeroMaintainableEarnings {
			return true, thresholds.NearZeroMaintainableEarnings
		}
		if revenue.Available && thresholds.NearZeroMaintainableEarningsPercentOfRevenue > 0 {
			revenueFloor := thresholds.NearZeroMaintainableEarningsPercentOfRevenue * math.Abs(revenue.Value)
			if value <= revenueFloor {
				return true, revenueFloor
			}
		}
		return false, 0
	}

	var flags []Flag
	if result.MaintainableEBITDA.Available {
		if triggered, floor := nearZero(result.MaintainableEBITDA.Value); triggered {
			flags = append(flags, Flag{
				Code:      FlagNegativeOrNearZeroMaintainableEarnings,
				Severity:  FlagSeverityCritical,
				Message:   fmt.Sprintf("maintainable EBITDA is %.2f, at or below the %.2f near-zero floor", result.MaintainableEBITDA.Value, floor),
				Value:     result.MaintainableEBITDA.Value,
				Threshold: floor,
			})
		}
	}
	if result.MaintainableSDE.Available {
		if triggered, floor := nearZero(result.MaintainableSDE.Value); triggered {
			flags = append(flags, Flag{
				Code:      FlagNegativeOrNearZeroMaintainableEarnings,
				Severity:  FlagSeverityCritical,
				Message:   fmt.Sprintf("maintainable SDE is %.2f, at or below the %.2f near-zero floor", result.MaintainableSDE.Value, floor),
				Value:     result.MaintainableSDE.Value,
				Threshold: floor,
			})
		}
	}
	return flags
}
