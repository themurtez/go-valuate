package acquisition

import "fmt"

// buildFlags evaluates every deterministic flag rule against result's
// already-computed fields under in.RedFlags. Order is fixed: FlagCode's
// declaration order, then by Scenario — never Go map order.
func buildFlags(in Input, result Result) []Flag {
	var flags []Flag

	if f, ok := belowMinimumDSCRFlag("", result.BaseCase, in.RedFlags); ok {
		flags = append(flags, f)
	}
	if f, ok := aboveMaximumPriceToEBITDAFlag(result.Multiples, in.RedFlags); ok {
		flags = append(flags, f)
	}
	if f, ok := aboveMaximumPriceToSDEFlag(result.Multiples, in.RedFlags); ok {
		flags = append(flags, f)
	}
	if f, ok := aboveMaximumPremiumFlag(result.Consensus, in.RedFlags); ok {
		flags = append(flags, f)
	}
	if f, ok := belowMinimumCashOnCashReturnFlag(result.Returns, in.RedFlags); ok {
		flags = append(flags, f)
	}
	if f, ok := aboveMaximumPaybackFlag(result.Returns, in.RedFlags); ok {
		flags = append(flags, f)
	}
	if f, ok := aboveMaximumLeverageFlag(result.BaseCase, in.RedFlags); ok {
		flags = append(flags, f)
	}
	if f, ok := negativePostDebtCashFlowFlag(result.BaseCase); ok {
		flags = append(flags, f)
	}
	for _, sc := range result.Scenarios {
		if sc.BreachesMinimumDSCR {
			flags = append(flags, Flag{
				Code:      FlagScenarioBreachesDSCR,
				Severity:  FlagSeverityWarning,
				Scenario:  sc.Scenario.Label,
				Message:   fmt.Sprintf("scenario %q breaches the minimum DSCR of %.2fx with a DSCR of %.2fx", sc.Scenario.Label, in.RedFlags.MinimumDSCR, sc.Coverage.DSCR.Amount),
				Value:     sc.Coverage.DSCR.Amount,
				Threshold: in.RedFlags.MinimumDSCR,
			})
		}
	}
	for _, sc := range result.Scenarios {
		if sc.HasNegativeCashFlow {
			flags = append(flags, Flag{
				Code:     FlagScenarioNegativeCashFlow,
				Severity: FlagSeverityWarning,
				Scenario: sc.Scenario.Label,
				Message:  fmt.Sprintf("scenario %q produces negative post-debt cash flow of %.2f", sc.Scenario.Label, sc.Coverage.PostDebtCashFlow.Amount),
				Value:    sc.Coverage.PostDebtCashFlow.Amount,
			})
		}
	}

	return flags
}

func belowMinimumDSCRFlag(scenario string, cov CoverageResult, thresholds RedFlagThresholds) (Flag, bool) {
	if thresholds.MinimumDSCR <= 0 || !cov.DSCR.Available || cov.DSCR.Amount >= thresholds.MinimumDSCR {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagBelowMinimumDSCR,
		Severity:  FlagSeverityCritical,
		Scenario:  scenario,
		Message:   fmt.Sprintf("DSCR is %.2fx, below the minimum threshold of %.2fx", cov.DSCR.Amount, thresholds.MinimumDSCR),
		Value:     cov.DSCR.Amount,
		Threshold: thresholds.MinimumDSCR,
	}, true
}

func aboveMaximumPriceToEBITDAFlag(m PriceMultiples, thresholds RedFlagThresholds) (Flag, bool) {
	if thresholds.MaximumPriceToEBITDA <= 0 || !m.PriceToEBITDA.Available || m.PriceToEBITDA.Amount <= thresholds.MaximumPriceToEBITDA {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagAboveMaximumPriceToEBITDA,
		Severity:  FlagSeverityWarning,
		Message:   fmt.Sprintf("asking price is %.2fx EBITDA, above the maximum threshold of %.2fx", m.PriceToEBITDA.Amount, thresholds.MaximumPriceToEBITDA),
		Value:     m.PriceToEBITDA.Amount,
		Threshold: thresholds.MaximumPriceToEBITDA,
	}, true
}

func aboveMaximumPriceToSDEFlag(m PriceMultiples, thresholds RedFlagThresholds) (Flag, bool) {
	if thresholds.MaximumPriceToSDE <= 0 || !m.PriceToSDE.Available || m.PriceToSDE.Amount <= thresholds.MaximumPriceToSDE {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagAboveMaximumPriceToSDE,
		Severity:  FlagSeverityWarning,
		Message:   fmt.Sprintf("asking price is %.2fx SDE, above the maximum threshold of %.2fx", m.PriceToSDE.Amount, thresholds.MaximumPriceToSDE),
		Value:     m.PriceToSDE.Amount,
		Threshold: thresholds.MaximumPriceToSDE,
	}, true
}

// aboveMaximumPremiumFlag never triggers on a discount (a negative
// PremiumPercent) regardless of magnitude — see
// RedFlagThresholds.MaximumPremiumToConsensusPercent's doc comment.
func aboveMaximumPremiumFlag(c ConsensusComparison, thresholds RedFlagThresholds) (Flag, bool) {
	if thresholds.MaximumPremiumToConsensusPercent <= 0 || !c.PremiumPercent.Available || c.PremiumPercent.Amount <= thresholds.MaximumPremiumToConsensusPercent {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagAboveMaximumPremiumToConsensus,
		Severity:  FlagSeverityWarning,
		Message:   fmt.Sprintf("asking price is %.1f%% above consensus value, above the maximum threshold of %.1f%%", c.PremiumPercent.Amount*100, thresholds.MaximumPremiumToConsensusPercent*100),
		Value:     c.PremiumPercent.Amount,
		Threshold: thresholds.MaximumPremiumToConsensusPercent,
	}, true
}

func belowMinimumCashOnCashReturnFlag(r ReturnMetrics, thresholds RedFlagThresholds) (Flag, bool) {
	if thresholds.MinimumCashOnCashReturn <= 0 || !r.CashOnCashReturn.Available || r.CashOnCashReturn.Amount >= thresholds.MinimumCashOnCashReturn {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagBelowMinimumCashOnCashReturn,
		Severity:  FlagSeverityWarning,
		Message:   fmt.Sprintf("cash-on-cash return is %.1f%%, below the minimum threshold of %.1f%%", r.CashOnCashReturn.Amount*100, thresholds.MinimumCashOnCashReturn*100),
		Value:     r.CashOnCashReturn.Amount,
		Threshold: thresholds.MinimumCashOnCashReturn,
	}, true
}

func aboveMaximumPaybackFlag(r ReturnMetrics, thresholds RedFlagThresholds) (Flag, bool) {
	if thresholds.MaximumPaybackYears <= 0 || !r.PaybackPeriodYears.Available || r.PaybackPeriodYears.Amount <= thresholds.MaximumPaybackYears {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagAboveMaximumPayback,
		Severity:  FlagSeverityWarning,
		Message:   fmt.Sprintf("simple payback period is %.1f years, above the maximum threshold of %.1f years", r.PaybackPeriodYears.Amount, thresholds.MaximumPaybackYears),
		Value:     r.PaybackPeriodYears.Amount,
		Threshold: thresholds.MaximumPaybackYears,
	}, true
}

func aboveMaximumLeverageFlag(cov CoverageResult, thresholds RedFlagThresholds) (Flag, bool) {
	if thresholds.MaximumDebtToEBITDA <= 0 || !cov.Leverage.Available || cov.Leverage.Amount <= thresholds.MaximumDebtToEBITDA {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagAboveMaximumLeverage,
		Severity:  FlagSeverityCritical,
		Message:   fmt.Sprintf("leverage is %.2fx, above the maximum threshold of %.2fx", cov.Leverage.Amount, thresholds.MaximumDebtToEBITDA),
		Value:     cov.Leverage.Amount,
		Threshold: thresholds.MaximumDebtToEBITDA,
	}, true
}

func negativePostDebtCashFlowFlag(cov CoverageResult) (Flag, bool) {
	if !cov.PostDebtCashFlow.Available || cov.PostDebtCashFlow.Amount >= 0 {
		return Flag{}, false
	}
	return Flag{
		Code:     FlagNegativePostDebtCashFlow,
		Severity: FlagSeverityCritical,
		Message:  fmt.Sprintf("post-debt cash flow is negative: %.2f", cov.PostDebtCashFlow.Amount),
		Value:    cov.PostDebtCashFlow.Amount,
	}, true
}
