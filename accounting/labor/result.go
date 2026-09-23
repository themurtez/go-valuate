package labor

// Provenance preserves enough source identity to trace a period's
// aggregates back to individual records — the task's section 52. No
// protected HR detail is exposed here because none is collected by this
// package in the first place (see Worker's doc comment).
type Provenance struct {
	PayrollRecordIDs    []string `json:"payroll_record_ids,omitempty"`
	WorkerIDs           []string `json:"worker_ids,omitempty"`
	ContractorRecordIDs []string `json:"contractor_record_ids,omitempty"`
}

// PeriodSummary is one period's full labor-analytics output — the task's
// section 48.
type PeriodSummary struct {
	Period PeriodInfo `json:"period"`

	LaborCostBridge LaborCostBridge `json:"labor_cost_bridge"`
	PayrollBasis    PayrollBasis    `json:"payroll_basis"`

	Headcount         Headcount         `json:"headcount"`
	WorkforceMovement WorkforceMovement `json:"workforce_movement"`
	FTE               FTESummary        `json:"fte"`

	Overtime OvertimeSummary `json:"overtime"`

	ContractorMix ContractorMix `json:"contractor_mix"`

	DirectIndirectSplit DirectIndirectSplit `json:"direct_indirect_split"`

	AverageLaborCost     AverageLaborCost     `json:"average_labor_cost"`
	VariableCompensation VariableCompensation `json:"variable_compensation"`
	Productivity         Productivity         `json:"productivity"`

	Reconciliation ReconciliationSummary `json:"reconciliation"`

	KeyWorkerConcentration KeyWorkerConcentration `json:"key_worker_concentration,omitempty"`

	Provenance Provenance `json:"provenance"`
}

// Result is this package's top-level Calculate output.
type Result struct {
	SchemaVersion  string `json:"schema_version"`
	FormulaVersion string `json:"formula_version"`

	Periods []PeriodSummary `json:"periods"`

	DepartmentSummaries []GroupSummary `json:"department_summaries,omitempty"`
	LocationSummaries   []GroupSummary `json:"location_summaries,omitempty"`
	CostCenterSummaries []GroupSummary `json:"cost_center_summaries,omitempty"`

	OvertimeByDepartment []OvertimeByDepartment `json:"overtime_by_department,omitempty"`

	Trend TrendResult `json:"trend"`

	ReconciliationSummary ReconciliationSummary `json:"reconciliation_summary"`

	PaySchedule PayScheduleSummary `json:"pay_schedule"`

	WorkerDateFindings []WorkerDateFinding `json:"worker_date_findings,omitempty"`
	DuplicateGroups    []DuplicateGroup    `json:"duplicate_groups,omitempty"`

	Flags  []Flag  `json:"flags,omitempty"`
	Issues []Issue `json:"issues,omitempty"`

	Coverage Coverage `json:"coverage"`
}
