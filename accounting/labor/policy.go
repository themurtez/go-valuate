package labor

// ReconciliationTolerance configures how a payroll-register component is
// compared against its corresponding GLPayrollControl figure — mirrors
// cashforecast's tolerance-policy shape. A difference is Reconciled when
// abs(Difference) <= AbsoluteTolerance, or (if PercentTolerance > 0) when
// abs(Difference) <= PercentTolerance * abs(GL amount). This package never
// invents a materiality/tolerance default of its own beyond the safe
// broad value in DefaultPolicy — see the task's section 28.
type ReconciliationTolerance struct {
	AbsoluteTolerance float64 `json:"absolute_tolerance,omitempty"`
	PercentTolerance  float64 `json:"percent_tolerance,omitempty"`
}

// Policy is every caller-configurable threshold this package uses,
// following closequality/journaldiagnostics's single-monolithic-struct
// convention (a better fit here than cashforecast's split-into-several-
// structs style, since this package has fewer independent policy
// subsystems). Nothing that materially changes a Flag or a reconciliation
// determination is a hidden code constant.
type Policy struct {
	// StandardFullTimeHoursPerPeriod is the caller-supplied standard
	// full-time hours used for hours-based FTE (FTE = eligible hours /
	// this value) — see fte.go. Zero means hours-based FTE is unavailable
	// (this package never assumes 40 hours/week universally) — see the
	// task's section 11.
	StandardFullTimeHoursPerPeriod float64 `json:"standard_full_time_hours_per_period,omitempty"`

	// GrowthGapThreshold triggers FlagLaborCostGrowthOutpacesRevenue when
	// (LaborCostGrowth - RevenueGrowth) exceeds this fraction. Default
	// 0.10 (10 percentage points).
	GrowthGapThreshold float64 `json:"growth_gap_threshold,omitempty"`

	// HighOvertimeHoursPercent/HighOvertimePayPercent trigger
	// FlagHighOvertimeShare when OvertimeHoursPercent/OvertimePayPercent
	// exceeds the respective threshold. Default 0.10 (10%) each.
	HighOvertimeHoursPercent float64 `json:"high_overtime_hours_percent,omitempty"`
	HighOvertimePayPercent   float64 `json:"high_overtime_pay_percent,omitempty"`

	// OvertimeTrendThreshold triggers FlagOvertimeIncreasing/
	// FlagOvertimeShareDeclining when adjacent-period OvertimeHoursPercent
	// change exceeds/falls below this fraction. Default 0.03 (3 percentage
	// points).
	OvertimeTrendThreshold float64 `json:"overtime_trend_threshold,omitempty"`

	// HighContractorShare triggers FlagHighContractorShare when
	// ContractorShareOfLaborCost exceeds this fraction. Default 0.30
	// (30%).
	HighContractorShare float64 `json:"high_contractor_share,omitempty"`

	// ContractorShareTrendThreshold triggers
	// FlagContractorShareIncreasing when adjacent-period
	// ContractorShareOfLaborCost change exceeds this fraction. Default
	// 0.05 (5 percentage points).
	ContractorShareTrendThreshold float64 `json:"contractor_share_trend_threshold,omitempty"`

	// LaborCostPercentRevenueTrendThreshold triggers
	// FlagLaborCostPercentRevenueIncreasing/
	// FlagLaborCostPercentRevenueImproving when adjacent-period
	// LaborCostPercentRevenue change exceeds/falls below this fraction.
	// Default 0.03 (3 percentage points).
	LaborCostPercentRevenueTrendThreshold float64 `json:"labor_cost_percent_revenue_trend_threshold,omitempty"`

	// RevenuePerFTETrendThreshold triggers FlagRevenuePerFTEDeclining/
	// FlagRevenuePerFTEImproving when adjacent-period RevenuePerFTE
	// percentage change falls below/exceeds +/- this fraction. Default
	// 0.05 (5%).
	RevenuePerFTETrendThreshold float64 `json:"revenue_per_fte_trend_threshold,omitempty"`

	// MaterialPayrollDifference is the default AbsoluteTolerance fallback
	// used by ReconciliationTolerance when the caller supplies no
	// tolerance at all for a component comparison. Default 0 (any
	// difference is a difference) unless the caller supplies
	// ReconciliationTolerance explicitly.
	MaterialPayrollDifference float64 `json:"material_payroll_difference,omitempty"`

	// ReconciliationTolerance is the tolerance applied to every
	// register-vs-GL component comparison — see reconcile.go.
	ReconciliationTolerance ReconciliationTolerance `json:"reconciliation_tolerance"`

	// PostTerminationPayGraceDays: a PayrollRecord.PayDate up to this many
	// days after a Worker's TerminationDate is treated as a legitimate
	// final pay (informational finding only, not a warning) — see the
	// task's section 37. Default 14.
	PostTerminationPayGraceDays int `json:"post_termination_pay_grace_days,omitempty"`

	// MinimumTrendPeriods is the minimum number of chronological periods
	// required before trend/flag calculations requiring history are
	// computed. Default 2 (adjacent-period comparison needs at least two
	// periods).
	MinimumTrendPeriods int `json:"minimum_trend_periods,omitempty"`

	// DisableHeadcountVsRevenueSignal, when true, suppresses
	// FlagHeadcountGrowthWithRevenueDecline entirely — see the task's
	// section 22 "caller can disable" instruction. Default false
	// (enabled).
	DisableHeadcountVsRevenueSignal bool `json:"disable_headcount_vs_revenue_signal,omitempty"`

	// DisableVariableCompTrend, when true, suppresses any variable-
	// compensation trend flag — see the task's section 25. This package's
	// V1 emits no dedicated variable-comp flag (see variablecomp.go's doc
	// comment), so this currently has no effect; reserved for a future
	// flag without a Policy shape change.
	DisableVariableCompTrend bool `json:"disable_variable_comp_trend,omitempty"`

	// PayrollActiveWorkerApproximation opts into approximating headcount
	// from distinct WorkerIDs appearing in PayrollRecords when no Worker
	// roster is supplied — see the task's section 10 "only if caller
	// explicitly opts in" instruction. Default false.
	PayrollActiveWorkerApproximation bool `json:"payroll_active_worker_approximation,omitempty"`
}

const (
	defaultGrowthGapThreshold                    = 0.10
	defaultHighOvertimeHoursPercent              = 0.10
	defaultHighOvertimePayPercent                = 0.10
	defaultOvertimeTrendThreshold                = 0.03
	defaultHighContractorShare                   = 0.30
	defaultContractorShareTrendThreshold         = 0.05
	defaultLaborCostPercentRevenueTrendThreshold = 0.03
	defaultRevenuePerFTETrendThreshold           = 0.05
	defaultPostTerminationPayGraceDays           = 14
	defaultMinimumTrendPeriods                   = 2
)

// DefaultPolicy returns this package's baseline, broadly-safe Policy.
// Every threshold here is a review-attention trigger point, not a
// payroll-law standard — see the task's section 42 "do not invent
// payroll-law thresholds" instruction.
func DefaultPolicy() Policy {
	return Policy{
		GrowthGapThreshold:                    defaultGrowthGapThreshold,
		HighOvertimeHoursPercent:              defaultHighOvertimeHoursPercent,
		HighOvertimePayPercent:                defaultHighOvertimePayPercent,
		OvertimeTrendThreshold:                defaultOvertimeTrendThreshold,
		HighContractorShare:                   defaultHighContractorShare,
		ContractorShareTrendThreshold:         defaultContractorShareTrendThreshold,
		LaborCostPercentRevenueTrendThreshold: defaultLaborCostPercentRevenueTrendThreshold,
		RevenuePerFTETrendThreshold:           defaultRevenuePerFTETrendThreshold,
		PostTerminationPayGraceDays:           defaultPostTerminationPayGraceDays,
		MinimumTrendPeriods:                   defaultMinimumTrendPeriods,
	}
}

// resolvePolicy merges p over DefaultPolicy field by field: a zero-valued
// field takes the default. Fields with a legitimate zero default
// (StandardFullTimeHoursPerPeriod, MaterialPayrollDifference,
// ReconciliationTolerance, the Disable*/opt-in bools) are never
// defaulted away from zero.
func resolvePolicy(p Policy) Policy {
	d := DefaultPolicy()
	if p.GrowthGapThreshold == 0 {
		p.GrowthGapThreshold = d.GrowthGapThreshold
	}
	if p.HighOvertimeHoursPercent == 0 {
		p.HighOvertimeHoursPercent = d.HighOvertimeHoursPercent
	}
	if p.HighOvertimePayPercent == 0 {
		p.HighOvertimePayPercent = d.HighOvertimePayPercent
	}
	if p.OvertimeTrendThreshold == 0 {
		p.OvertimeTrendThreshold = d.OvertimeTrendThreshold
	}
	if p.HighContractorShare == 0 {
		p.HighContractorShare = d.HighContractorShare
	}
	if p.ContractorShareTrendThreshold == 0 {
		p.ContractorShareTrendThreshold = d.ContractorShareTrendThreshold
	}
	if p.LaborCostPercentRevenueTrendThreshold == 0 {
		p.LaborCostPercentRevenueTrendThreshold = d.LaborCostPercentRevenueTrendThreshold
	}
	if p.RevenuePerFTETrendThreshold == 0 {
		p.RevenuePerFTETrendThreshold = d.RevenuePerFTETrendThreshold
	}
	if p.PostTerminationPayGraceDays == 0 {
		p.PostTerminationPayGraceDays = d.PostTerminationPayGraceDays
	}
	if p.MinimumTrendPeriods == 0 {
		p.MinimumTrendPeriods = d.MinimumTrendPeriods
	}
	return p
}

// validatePolicy checks Policy for structurally invalid configuration:
// non-finite thresholds and a negative grace-day count.
func validatePolicy(p Policy) []Issue {
	var issues []Issue
	checkFinite := func(v float64, label string) {
		if isNonFinite(v) {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError,
				Message: "policy threshold " + label + " is non-finite"})
		}
	}
	checkFinite(p.StandardFullTimeHoursPerPeriod, "standard_full_time_hours_per_period")
	checkFinite(p.GrowthGapThreshold, "growth_gap_threshold")
	checkFinite(p.HighOvertimeHoursPercent, "high_overtime_hours_percent")
	checkFinite(p.HighOvertimePayPercent, "high_overtime_pay_percent")
	checkFinite(p.OvertimeTrendThreshold, "overtime_trend_threshold")
	checkFinite(p.HighContractorShare, "high_contractor_share")
	checkFinite(p.ContractorShareTrendThreshold, "contractor_share_trend_threshold")
	checkFinite(p.LaborCostPercentRevenueTrendThreshold, "labor_cost_percent_revenue_trend_threshold")
	checkFinite(p.RevenuePerFTETrendThreshold, "revenue_per_fte_trend_threshold")
	checkFinite(p.MaterialPayrollDifference, "material_payroll_difference")
	checkFinite(p.ReconciliationTolerance.AbsoluteTolerance, "reconciliation_tolerance.absolute_tolerance")
	checkFinite(p.ReconciliationTolerance.PercentTolerance, "reconciliation_tolerance.percent_tolerance")
	if p.PostTerminationPayGraceDays < 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError,
			Message: "policy post_termination_pay_grace_days must not be negative"})
	}
	return issues
}
