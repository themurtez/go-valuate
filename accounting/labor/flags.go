package labor

// FlagSeverity mirrors cashforecast.FlagSeverity/journaldiagnostics's role:
// a structured, matchable urgency signal, never inferred from Message
// text.
type FlagSeverity string

const (
	FlagSeverityInfo     FlagSeverity = "info"
	FlagSeverityWarning  FlagSeverity = "warning"
	FlagSeverityCritical FlagSeverity = "critical"
)

// FlagCode is a stable identifier for one kind of deterministic
// labor-analytics review signal — as opposed to an Issue, which is an
// input/validation problem (see issues.go). This package never derives a
// hidden workforce/composite score; every flag is a simple, documented
// threshold comparison the caller can fully see and override via Policy.
// Every generated Flag.Message uses neutral, factual, non-legal,
// non-worker-decision language — see safety_test.go.
type FlagCode string

const (
	// FlagLaborCostGrowthOutpacesRevenue means adjacent-period
	// LaborCostGrowth exceeds RevenueGrowth by more than
	// Policy.GrowthGapThreshold. Requires both growth measures available.
	FlagLaborCostGrowthOutpacesRevenue FlagCode = "LABOR_COST_GROWTH_OUTPACES_REVENUE"
	// FlagHeadcountGrowthWithRevenueDecline means headcount grew
	// period-over-period while revenue declined. A review indicator, not
	// a conclusion about staffing quality. Suppressed entirely when
	// Policy.DisableHeadcountVsRevenueSignal is true.
	FlagHeadcountGrowthWithRevenueDecline FlagCode = "HEADCOUNT_GROWTH_WITH_REVENUE_DECLINE"
	// FlagHighOvertimeShare means OvertimeHoursPercent or
	// OvertimePayPercent exceeds its respective Policy threshold for a
	// period.
	FlagHighOvertimeShare FlagCode = "HIGH_OVERTIME_SHARE"
	// FlagOvertimeIncreasing means adjacent-period OvertimeHoursPercent
	// increased by more than Policy.OvertimeTrendThreshold.
	FlagOvertimeIncreasing FlagCode = "OVERTIME_INCREASING"
	// FlagOvertimeShareDeclining means adjacent-period
	// OvertimeHoursPercent decreased by more than
	// Policy.OvertimeTrendThreshold — a positive signal.
	FlagOvertimeShareDeclining FlagCode = "OVERTIME_SHARE_DECLINING"
	// FlagHighContractorShare means ContractorShareOfLaborCost exceeds
	// Policy.HighContractorShare for a period.
	FlagHighContractorShare FlagCode = "HIGH_CONTRACTOR_SHARE"
	// FlagContractorShareIncreasing means adjacent-period
	// ContractorShareOfLaborCost increased by more than
	// Policy.ContractorShareTrendThreshold.
	FlagContractorShareIncreasing FlagCode = "CONTRACTOR_SHARE_INCREASING"
	// FlagLaborCostPercentRevenueIncreasing means adjacent-period
	// LaborCostPercentRevenue increased by more than
	// Policy.LaborCostPercentRevenueTrendThreshold.
	FlagLaborCostPercentRevenueIncreasing FlagCode = "LABOR_COST_PERCENT_REVENUE_INCREASING"
	// FlagLaborCostPercentRevenueImproving means adjacent-period
	// LaborCostPercentRevenue decreased by more than
	// Policy.LaborCostPercentRevenueTrendThreshold — a positive signal.
	FlagLaborCostPercentRevenueImproving FlagCode = "LABOR_COST_PERCENT_REVENUE_IMPROVING"
	// FlagRevenuePerFTEDeclining means adjacent-period RevenuePerFTE
	// declined by more than Policy.RevenuePerFTETrendThreshold.
	FlagRevenuePerFTEDeclining FlagCode = "REVENUE_PER_FTE_DECLINING"
	// FlagRevenuePerFTEImproving means adjacent-period RevenuePerFTE
	// improved by more than Policy.RevenuePerFTETrendThreshold — a
	// positive signal.
	FlagRevenuePerFTEImproving FlagCode = "REVENUE_PER_FTE_IMPROVING"
	// FlagPayrollGLMismatch means at least one ReconciliationSummary
	// component is UNRECONCILED (difference exceeds tolerance).
	FlagPayrollGLMismatch FlagCode = "PAYROLL_GL_MISMATCH"
	// FlagPayrollGLReconciled means every available ReconciliationSummary
	// component is Reconciled — a positive signal, emitted only when at
	// least one component was actually compared.
	FlagPayrollGLReconciled FlagCode = "PAYROLL_GL_RECONCILED"
	// FlagWorkerDateInconsistency means a PayrollRecord's PayDate falls
	// before the referenced Worker's HireDate, or materially after
	// TerminationDate (beyond Policy.PostTerminationPayGraceDays) — see
	// workforcedates.go.
	FlagWorkerDateInconsistency FlagCode = "WORKER_DATE_INCONSISTENCY"
	// FlagPossibleDuplicatePayrollRecord means two distinct PayrollRecord
	// IDs share the same worker, pay date, and component amounts —
	// conservatively labeled "possible," never asserted as a duplicate
	// payment. See duplicates.go.
	FlagPossibleDuplicatePayrollRecord FlagCode = "POSSIBLE_DUPLICATE_PAYROLL_RECORD"
)

// flagCodeOrder fixes FlagCode declaration order for deterministic Flags
// sorting — never Go map order.
var flagCodeOrder = []FlagCode{
	FlagLaborCostGrowthOutpacesRevenue,
	FlagHeadcountGrowthWithRevenueDecline,
	FlagHighOvertimeShare,
	FlagOvertimeIncreasing,
	FlagOvertimeShareDeclining,
	FlagHighContractorShare,
	FlagContractorShareIncreasing,
	FlagLaborCostPercentRevenueIncreasing,
	FlagLaborCostPercentRevenueImproving,
	FlagRevenuePerFTEDeclining,
	FlagRevenuePerFTEImproving,
	FlagPayrollGLMismatch,
	FlagPayrollGLReconciled,
	FlagWorkerDateInconsistency,
	FlagPossibleDuplicatePayrollRecord,
}

func flagRank(c FlagCode) int {
	for i, fc := range flagCodeOrder {
		if fc == c {
			return i
		}
	}
	return len(flagCodeOrder)
}

// Flag is one deterministic labor-analytics review signal Calculate
// triggered.
type Flag struct {
	Code      FlagCode     `json:"code"`
	Severity  FlagSeverity `json:"severity"`
	Period    string       `json:"period,omitempty"`
	Message   string       `json:"message"`
	Value     float64      `json:"value,omitempty"`
	Threshold float64      `json:"threshold,omitempty"`
}
