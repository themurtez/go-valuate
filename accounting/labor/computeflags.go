package labor

// computeFlagsInput bundles everything computeFlags needs, kept as one
// struct so the function signature stays readable — mirrors
// cashforecast.flagInputs/ar.flagInputs's identical convention.
type computeFlagsInput struct {
	periods            []PeriodSummary
	trend              TrendResult
	reconciliation     ReconciliationSummary
	overtimeByDept     []OvertimeByDepartment
	workerDateFindings []WorkerDateFinding
	duplicates         []DuplicateGroup
	policy             Policy
}

// computeFlags evaluates every FlagCode rule and returns triggered flags
// sorted by FlagCode declaration order, then Period — never Go map order.
// Every Message uses neutral, factual, non-legal, non-worker-decision
// language — see safety_test.go.
func computeFlags(in computeFlagsInput) []Flag {
	var flags []Flag
	p := in.policy

	if in.trend.LaborCostGrowth.Available && in.trend.RevenueGrowth.Available {
		gap := in.trend.LaborCostGrowth.Amount - in.trend.RevenueGrowth.Amount
		if gap > p.GrowthGapThreshold {
			flags = append(flags, Flag{Code: FlagLaborCostGrowthOutpacesRevenue, Severity: FlagSeverityWarning,
				Value: gap, Threshold: p.GrowthGapThreshold,
				Message: "labor cost growth exceeds revenue growth by more than the configured review threshold"})
		}
	}

	if !p.DisableHeadcountVsRevenueSignal && in.trend.HeadcountGrowth.Available && in.trend.RevenueGrowth.Available {
		if in.trend.HeadcountGrowth.Amount > 0 && in.trend.RevenueGrowth.Amount < 0 {
			flags = append(flags, Flag{Code: FlagHeadcountGrowthWithRevenueDecline, Severity: FlagSeverityWarning,
				Value: in.trend.HeadcountGrowth.Amount, Threshold: 0,
				Message: "headcount increased while revenue declined period-over-period; a review indicator, not a conclusion about staffing levels"})
		}
	}

	// Per-period overtime/contractor/labor-cost-percent-revenue level
	// flags.
	for _, s := range in.periods {
		period := s.Period.Period
		if s.Overtime.Available {
			if s.Overtime.OvertimeHoursPercent.Available && s.Overtime.OvertimeHoursPercent.Amount > p.HighOvertimeHoursPercent {
				flags = append(flags, Flag{Code: FlagHighOvertimeShare, Severity: FlagSeverityWarning, Period: period,
					Value: s.Overtime.OvertimeHoursPercent.Amount, Threshold: p.HighOvertimeHoursPercent,
					Message: "overtime hours represent a share of total hours above the configured review threshold"})
			} else if s.Overtime.OvertimePayPercent.Available && s.Overtime.OvertimePayPercent.Amount > p.HighOvertimePayPercent {
				flags = append(flags, Flag{Code: FlagHighOvertimeShare, Severity: FlagSeverityWarning, Period: period,
					Value: s.Overtime.OvertimePayPercent.Amount, Threshold: p.HighOvertimePayPercent,
					Message: "overtime pay represents a share of gross pay above the configured review threshold"})
			}
		}
		if s.ContractorMix.ContractorShareOfLaborCost.Available && s.ContractorMix.ContractorShareOfLaborCost.Amount > p.HighContractorShare {
			flags = append(flags, Flag{Code: FlagHighContractorShare, Severity: FlagSeverityWarning, Period: period,
				Value: s.ContractorMix.ContractorShareOfLaborCost.Amount, Threshold: p.HighContractorShare,
				Message: "contractor labor represents a share of total labor cost above the configured review threshold"})
		}
	}

	if in.trend.OvertimeShare.Available && len(in.trend.OvertimeShare.Points) >= 2 {
		change := absoluteAdjacentChange(in.trend.OvertimeShare)
		latestPeriod := in.trend.OvertimeShare.Points[len(in.trend.OvertimeShare.Points)-1].Period
		if change.Available {
			if change.Amount > p.OvertimeTrendThreshold {
				flags = append(flags, Flag{Code: FlagOvertimeIncreasing, Severity: FlagSeverityWarning, Period: latestPeriod,
					Value: change.Amount, Threshold: p.OvertimeTrendThreshold,
					Message: "overtime share of total hours increased more than the configured review threshold from the prior period"})
			} else if change.Amount < -p.OvertimeTrendThreshold {
				flags = append(flags, Flag{Code: FlagOvertimeShareDeclining, Severity: FlagSeverityInfo, Period: latestPeriod,
					Value: change.Amount, Threshold: p.OvertimeTrendThreshold,
					Message: "overtime share of total hours decreased more than the configured review threshold from the prior period"})
			}
		}
	}

	if change := contractorShareAdjacentChange(in.periods); change.Available {
		latestPeriod := in.periods[len(in.periods)-1].Period.Period
		if change.Amount > p.ContractorShareTrendThreshold {
			flags = append(flags, Flag{Code: FlagContractorShareIncreasing, Severity: FlagSeverityWarning, Period: latestPeriod,
				Value: change.Amount, Threshold: p.ContractorShareTrendThreshold,
				Message: "contractor share of total labor cost increased more than the configured review threshold from the prior period"})
		}
	}

	if in.trend.LaborCostPercentRevenueChange.Available {
		latestPeriod := in.trend.LaborCostPercentRevenue.Points[len(in.trend.LaborCostPercentRevenue.Points)-1].Period
		change := in.trend.LaborCostPercentRevenueChange.Amount
		if change > p.LaborCostPercentRevenueTrendThreshold {
			flags = append(flags, Flag{Code: FlagLaborCostPercentRevenueIncreasing, Severity: FlagSeverityWarning, Period: latestPeriod,
				Value: change, Threshold: p.LaborCostPercentRevenueTrendThreshold,
				Message: "labor cost as a percent of revenue increased more than the configured review threshold from the prior period"})
		} else if change < -p.LaborCostPercentRevenueTrendThreshold {
			flags = append(flags, Flag{Code: FlagLaborCostPercentRevenueImproving, Severity: FlagSeverityInfo, Period: latestPeriod,
				Value: change, Threshold: p.LaborCostPercentRevenueTrendThreshold,
				Message: "labor cost as a percent of revenue decreased more than the configured review threshold from the prior period"})
		}
	}

	if in.trend.RevenuePerFTEChange.Available {
		latestPeriod := in.trend.RevenuePerFTE.Points[len(in.trend.RevenuePerFTE.Points)-1].Period
		change := in.trend.RevenuePerFTEChange.Amount
		if change < -p.RevenuePerFTETrendThreshold {
			flags = append(flags, Flag{Code: FlagRevenuePerFTEDeclining, Severity: FlagSeverityWarning, Period: latestPeriod,
				Value: change, Threshold: p.RevenuePerFTETrendThreshold,
				Message: "revenue per FTE declined more than the configured review threshold from the prior period"})
		} else if change > p.RevenuePerFTETrendThreshold {
			flags = append(flags, Flag{Code: FlagRevenuePerFTEImproving, Severity: FlagSeverityInfo, Period: latestPeriod,
				Value: change, Threshold: p.RevenuePerFTETrendThreshold,
				Message: "revenue per FTE improved more than the configured review threshold from the prior period"})
		}
	}

	switch in.reconciliation.Status {
	case ReconciliationStatusUnreconciled, ReconciliationStatusReconciledWithDifferences:
		flags = append(flags, Flag{Code: FlagPayrollGLMismatch, Severity: FlagSeverityWarning,
			Message: "one or more payroll register components differ from the corresponding GL control balance beyond tolerance"})
	case ReconciliationStatusReconciled:
		flags = append(flags, Flag{Code: FlagPayrollGLReconciled, Severity: FlagSeverityInfo,
			Message: "every compared payroll register component matches its corresponding GL control balance within tolerance"})
	}

	for _, f := range in.workerDateFindings {
		if f.Severity != FlagSeverityWarning {
			continue
		}
		flags = append(flags, Flag{Code: FlagWorkerDateInconsistency, Severity: FlagSeverityWarning,
			Message: f.Message})
	}

	if len(in.duplicates) > 0 {
		flags = append(flags, Flag{Code: FlagPossibleDuplicatePayrollRecord, Severity: FlagSeverityWarning,
			Value:   float64(len(in.duplicates)),
			Message: "two or more payroll records share the same worker, pay date, and component amounts"})
	}

	sortFlags(flags)
	return flags
}

// contractorShareAdjacentChange returns the last-vs-second-to-last
// percentage-point change in ContractorShareOfLaborCost across
// chronologically ordered periods, unavailable if fewer than 2 periods
// have the metric available.
func contractorShareAdjacentChange(periods []PeriodSummary) Value {
	var pts []float64
	for _, s := range periods {
		if s.ContractorMix.ContractorShareOfLaborCost.Available {
			pts = append(pts, s.ContractorMix.ContractorShareOfLaborCost.Amount)
		}
	}
	if len(pts) < 2 {
		return Unavailable()
	}
	return AvailableValue(pts[len(pts)-1] - pts[len(pts)-2])
}
