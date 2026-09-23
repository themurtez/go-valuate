package debt

import "fmt"

// buildFlags evaluates every deterministic flag rule against result's
// already-computed fields under in.Policy. Order is fixed: FlagCode's
// declaration order, then by Scenario — never Go map order.
func buildFlags(in Input, result Result) []Flag {
	var flags []Flag

	if f, ok := belowMinimumDSCRFlag("", result.BaseCase, in.Policy); ok {
		flags = append(flags, f)
	}
	if f, ok := aboveLeverageCapFlag(result.BaseCase, in.Policy); ok {
		flags = append(flags, f)
	}
	if f, ok := belowMinimumFixedChargeCoverageFlag(result.BaseCase, in.Policy); ok {
		flags = append(flags, f)
	}
	if f, ok := negativeHeadroomFlag(result.Capacity); ok {
		flags = append(flags, f)
	}
	for _, sc := range result.Scenarios {
		if sc.BreachesMinimumDSCR {
			flags = append(flags, Flag{
				Code:      FlagScenarioBreachesDSCR,
				Severity:  FlagSeverityWarning,
				Scenario:  sc.Scenario.Label,
				Message:   fmt.Sprintf("scenario %q breaches the minimum DSCR of %.2fx with a DSCR of %.2fx", sc.Scenario.Label, in.Policy.MinimumDSCR, sc.Coverage.DSCR.Amount),
				Value:     sc.Coverage.DSCR.Amount,
				Threshold: in.Policy.MinimumDSCR,
			})
		}
	}
	if noDebtFlag, ok := noDebtServiceFlag(in, result.BaseCase); ok {
		flags = append(flags, noDebtFlag)
	}

	return flags
}

func belowMinimumDSCRFlag(scenario string, cov CoverageResult, policy LenderPolicy) (Flag, bool) {
	if policy.MinimumDSCR <= 0 || !cov.DSCR.Available || cov.DSCR.Amount >= policy.MinimumDSCR {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagBelowMinimumDSCR,
		Severity:  FlagSeverityCritical,
		Scenario:  scenario,
		Message:   fmt.Sprintf("DSCR is %.2fx, below the minimum policy threshold of %.2fx", cov.DSCR.Amount, policy.MinimumDSCR),
		Value:     cov.DSCR.Amount,
		Threshold: policy.MinimumDSCR,
	}, true
}

func aboveLeverageCapFlag(cov CoverageResult, policy LenderPolicy) (Flag, bool) {
	if policy.MaximumDebtToEBITDA > 0 && cov.DebtToEBITDA.Available && cov.DebtToEBITDA.Amount > policy.MaximumDebtToEBITDA {
		return Flag{
			Code:      FlagAboveLeverageCap,
			Severity:  FlagSeverityCritical,
			Message:   fmt.Sprintf("debt/EBITDA is %.2fx, above the maximum policy threshold of %.2fx", cov.DebtToEBITDA.Amount, policy.MaximumDebtToEBITDA),
			Value:     cov.DebtToEBITDA.Amount,
			Threshold: policy.MaximumDebtToEBITDA,
		}, true
	}
	if policy.MaximumNetDebtToEBITDA > 0 && cov.NetDebtToEBITDA.Available && cov.NetDebtToEBITDA.Amount > policy.MaximumNetDebtToEBITDA {
		return Flag{
			Code:      FlagAboveLeverageCap,
			Severity:  FlagSeverityCritical,
			Message:   fmt.Sprintf("net debt/EBITDA is %.2fx, above the maximum policy threshold of %.2fx", cov.NetDebtToEBITDA.Amount, policy.MaximumNetDebtToEBITDA),
			Value:     cov.NetDebtToEBITDA.Amount,
			Threshold: policy.MaximumNetDebtToEBITDA,
		}, true
	}
	return Flag{}, false
}

func belowMinimumFixedChargeCoverageFlag(cov CoverageResult, policy LenderPolicy) (Flag, bool) {
	if policy.MinimumFixedChargeCoverage <= 0 || !cov.FixedChargeCoverage.Available || cov.FixedChargeCoverage.Amount >= policy.MinimumFixedChargeCoverage {
		return Flag{}, false
	}
	return Flag{
		Code:      FlagBelowMinimumFixedChargeCoverage,
		Severity:  FlagSeverityCritical,
		Message:   fmt.Sprintf("fixed-charge coverage is %.2fx, below the minimum policy threshold of %.2fx", cov.FixedChargeCoverage.Amount, policy.MinimumFixedChargeCoverage),
		Value:     cov.FixedChargeCoverage.Amount,
		Threshold: policy.MinimumFixedChargeCoverage,
	}, true
}

func negativeHeadroomFlag(capacity MaximumCapacity) (Flag, bool) {
	if !capacity.Headroom.Available || capacity.Headroom.Amount >= 0 {
		return Flag{}, false
	}
	return Flag{
		Code:     FlagNegativeHeadroom,
		Severity: FlagSeverityWarning,
		Message:  fmt.Sprintf("current debt exceeds combined capacity by %.2f", -capacity.Headroom.Amount),
		Value:    capacity.Headroom.Amount,
	}, true
}

func noDebtServiceFlag(in Input, base CoverageResult) (Flag, bool) {
	hasDebtInput := len(in.ExistingDebt) > 0 || len(in.ProposedLoans) > 0 || in.ExistingDebtBalance.Available
	if hasDebtInput {
		return Flag{}, false
	}
	if !base.AnnualDebtService.Available || base.AnnualDebtService.Amount != 0 {
		return Flag{}, false
	}
	return Flag{
		Code:     FlagNoDebtService,
		Severity: FlagSeverityInfo,
		Message:  "no debt or loan terms supplied; annual debt service is treated as zero and DSCR/leverage ratios are unavailable",
	}, true
}
