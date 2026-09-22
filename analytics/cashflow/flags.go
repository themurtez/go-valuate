package cashflow

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/financial"
)

// buildFlags evaluates every deterministic flag rule against result's
// already-computed fields (History, Conversion, RecurringDrains,
// CashRunway, ConversionTrend) under thresholds, restricting
// period-specific checks to the most recent period in History (chronologically
// last when ordering was available, else dataset-order last) — mirroring
// qoe.buildFlags/ratios' identical "flags read the most recent period"
// convention. Order is fixed: FlagCode's declaration order, then by
// Period — never Go map order.
func buildFlags(result Result, thresholds Thresholds) []Flag {
	var flags []Flag

	lastBridge, hasBridge := lastOf(result.History)
	lastConv, hasConv := lastOf(result.Conversion)

	if hasBridge && hasConv {
		if f, ok := weakConversionFlag(lastBridge.Period, lastConv, thresholds); ok {
			flags = append(flags, f)
		}
	}

	if hasBridge {
		if f, ok := highCapexBurdenFlag(lastBridge, thresholds); ok {
			flags = append(flags, f)
		}
		if f, ok := highWorkingCapitalBurdenFlag(lastBridge, thresholds); ok {
			flags = append(flags, f)
		}
		if f, ok := lowDebtServiceCoverageFlag(lastBridge, thresholds); ok {
			flags = append(flags, f)
		}
		if f, ok := distributionsExceedFCFFlag(lastBridge, thresholds); ok {
			flags = append(flags, f)
		}
	}

	if f, ok := lowCashRunwayFlag(result.CashRunway, thresholds); ok {
		flags = append(flags, f)
	}

	if result.ConversionTrend.Direction == TrendDeclining {
		flags = append(flags, Flag{
			Code:     FlagDecliningConversionTrend,
			Severity: FlagSeverityWarning,
			Message:  fmt.Sprintf("EBITDA-to-free-cash-flow conversion declined from %s to %s", result.ConversionTrend.FirstPeriod, result.ConversionTrend.LastPeriod),
			Value:    result.ConversionTrend.PercentChange.Value,
		})
	}

	return flags
}

func lastOf[T any](s []T) (T, bool) {
	var zero T
	if len(s) == 0 {
		return zero, false
	}
	return s[len(s)-1], true
}

func weakConversionFlag(period financial.Period, conv ConversionRatios, t Thresholds) (Flag, bool) {
	var value float64
	var triggered bool
	if conv.EBITDAToFreeCashFlow.Available && conv.EBITDAToFreeCashFlow.Value <= t.WeakConversionRatio {
		value = conv.EBITDAToFreeCashFlow.Value
		triggered = true
	} else if conv.EBITDAToOperatingCashFlow.Available && conv.EBITDAToOperatingCashFlow.Value <= t.WeakConversionRatio {
		value = conv.EBITDAToOperatingCashFlow.Value
		triggered = true
	}
	if !triggered {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagWeakCashConversion,
		Severity:  FlagSeverityWarning,
		Period:    period,
		Message:   fmt.Sprintf("EBITDA-to-cash conversion is %.1f%%, at or below the %.1f%% threshold", value*100, t.WeakConversionRatio*100),
		Value:     value,
		Threshold: t.WeakConversionRatio,
	}, true
}

func highCapexBurdenFlag(b Bridge, t Thresholds) (Flag, bool) {
	if !b.Capex.Available || !b.EBITDA.Available || b.EBITDA.Value == 0 {
		return Flag{}, false
	}
	ratio := math.Abs(b.Capex.Value) / math.Abs(b.EBITDA.Value)
	if ratio < t.HighCapexBurdenRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagHighCapexBurden,
		Severity:  FlagSeverityWarning,
		Period:    b.Period,
		Message:   fmt.Sprintf("capex is %.1f%% of EBITDA, at or above the %.1f%% threshold", ratio*100, t.HighCapexBurdenRatio*100),
		Value:     ratio,
		Threshold: t.HighCapexBurdenRatio,
	}, true
}

func highWorkingCapitalBurdenFlag(b Bridge, t Thresholds) (Flag, bool) {
	if !b.ChangeInNWC.Available || !b.EBITDA.Available || b.EBITDA.Value == 0 {
		return Flag{}, false
	}
	ratio := math.Abs(b.ChangeInNWC.Value) / math.Abs(b.EBITDA.Value)
	if ratio < t.HighWorkingCapitalBurdenRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagHighWorkingCapitalBurden,
		Severity:  FlagSeverityWarning,
		Period:    b.Period,
		Message:   fmt.Sprintf("change in net working capital is %.1f%% of EBITDA, at or above the %.1f%% threshold", ratio*100, t.HighWorkingCapitalBurdenRatio*100),
		Value:     ratio,
		Threshold: t.HighWorkingCapitalBurdenRatio,
	}, true
}

func lowDebtServiceCoverageFlag(b Bridge, t Thresholds) (Flag, bool) {
	total := b.DebtService.Total()
	if !b.OperatingCashFlow.Available || !total.Available || total.Value == 0 {
		return Flag{}, false
	}
	dscr := b.OperatingCashFlow.Value / total.Value
	if dscr > t.LowDebtServiceCoverageRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagLowDebtServiceCoverage,
		Severity:  FlagSeverityCritical,
		Period:    b.Period,
		Message:   fmt.Sprintf("debt service coverage ratio is %.2fx, at or below the %.2fx threshold", dscr, t.LowDebtServiceCoverageRatio),
		Value:     dscr,
		Threshold: t.LowDebtServiceCoverageRatio,
	}, true
}

func distributionsExceedFCFFlag(b Bridge, t Thresholds) (Flag, bool) {
	if !b.OwnerDistributions.Available || !b.FreeCashFlow.Available || b.FreeCashFlow.Value == 0 {
		return Flag{}, false
	}
	ratio := b.OwnerDistributions.Value / b.FreeCashFlow.Value
	if ratio < t.UncoveredDistributionsRatio {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagDistributionsExceedFreeCashFlow,
		Severity:  FlagSeverityWarning,
		Period:    b.Period,
		Message:   fmt.Sprintf("owner distributions are %.1f%% of free cash flow, at or above the %.1f%% threshold", ratio*100, t.UncoveredDistributionsRatio*100),
		Value:     ratio,
		Threshold: t.UncoveredDistributionsRatio,
	}, true
}

func lowCashRunwayFlag(runway CashRunway, t Thresholds) (Flag, bool) {
	if !runway.Available || !runway.MonthsOfRunway.Available {
		return Flag{}, false
	}
	if runway.MonthsOfRunway.Value > t.LowRunwayMonths {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagLowCashRunway,
		Severity:  FlagSeverityCritical,
		Message:   fmt.Sprintf("cash runway is %.1f months, at or below the %.1f month threshold", runway.MonthsOfRunway.Value, t.LowRunwayMonths),
		Value:     runway.MonthsOfRunway.Value,
		Threshold: t.LowRunwayMonths,
	}, true
}
