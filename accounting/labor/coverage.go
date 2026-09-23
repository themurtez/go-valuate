package labor

// Coverage reports factual data-coverage counts/percentages — never an
// opaque completeness score, per the task's section 51.
type Coverage struct {
	PayrollRecordsSupplied bool `json:"payroll_records_supplied"`
	WorkerRosterSupplied   bool `json:"worker_roster_supplied"`
	ContractorDataSupplied bool `json:"contractor_data_supplied"`

	// HoursCoveragePercent is the fraction (0-1) of included PayrollRecords
	// with HoursRegular.Available (or HoursOvertime.Available) set.
	// Unavailable when no payroll records are supplied.
	HoursCoveragePercent Value `json:"hours_coverage_percent"`
	// DepartmentCoveragePercent/LocationCoveragePercent are the fraction of
	// included PayrollRecords with a non-empty Department/Location.
	DepartmentCoveragePercent Value `json:"department_coverage_percent"`
	LocationCoveragePercent   Value `json:"location_coverage_percent"`

	FTEAvailable              bool `json:"fte_available"`
	FinancialMetricsAvailable bool `json:"financial_metrics_available"`
	GLReconciliationAvailable bool `json:"gl_reconciliation_available"`
}

// buildCoverage computes Coverage from the already-validated/included
// record sets.
func buildCoverage(payroll []PayrollRecord, workers []Worker, contractors []ContractorLaborRecord,
	metrics []BusinessMetrics, glControls []GLPayrollControl, fteAvailable bool) Coverage {

	c := Coverage{
		PayrollRecordsSupplied:    len(payroll) > 0,
		WorkerRosterSupplied:      len(workers) > 0,
		ContractorDataSupplied:    len(contractors) > 0,
		FTEAvailable:              fteAvailable,
		FinancialMetricsAvailable: len(metrics) > 0,
		GLReconciliationAvailable: len(glControls) > 0,
	}

	if len(payroll) > 0 {
		var withHours, withDept, withLoc int
		for _, r := range payroll {
			if r.HoursRegular.Available || r.HoursOvertime.Available {
				withHours++
			}
			if r.Department != "" {
				withDept++
			}
			if r.Location != "" {
				withLoc++
			}
		}
		n := float64(len(payroll))
		c.HoursCoveragePercent = AvailableValue(float64(withHours) / n)
		c.DepartmentCoveragePercent = AvailableValue(float64(withDept) / n)
		c.LocationCoveragePercent = AvailableValue(float64(withLoc) / n)
	}

	return c
}
