package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestCoverage_PayrollAndRosterSupplied(t *testing.T) {
	workers, records := fixtures.StableServiceBusiness()
	r := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers, PayrollRecords: records}, labor.DefaultPolicy())

	if !r.Coverage.PayrollRecordsSupplied {
		t.Error("expected PayrollRecordsSupplied true")
	}
	if !r.Coverage.WorkerRosterSupplied {
		t.Error("expected WorkerRosterSupplied true")
	}
	if r.Coverage.ContractorDataSupplied {
		t.Error("expected ContractorDataSupplied false")
	}
	if !r.Coverage.HoursCoveragePercent.Available {
		t.Error("expected HoursCoveragePercent available when payroll records exist")
	}
	if r.Coverage.HoursCoveragePercent.Amount != 0 {
		t.Errorf("expected HoursCoveragePercent 0 (fixture has no hours), got %v", r.Coverage.HoursCoveragePercent.Amount)
	}
}

func TestCoverage_DepartmentAndLocationPercent(t *testing.T) {
	records := fixtures.MultiDepartmentCompany()
	r := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}, labor.DefaultPolicy())

	if !r.Coverage.DepartmentCoveragePercent.Available || r.Coverage.DepartmentCoveragePercent.Amount != 1.0 {
		t.Errorf("expected DepartmentCoveragePercent 1.0, got %+v", r.Coverage.DepartmentCoveragePercent)
	}
	if !r.Coverage.LocationCoveragePercent.Available || r.Coverage.LocationCoveragePercent.Amount != 0 {
		t.Errorf("expected LocationCoveragePercent 0 (no location set), got %+v", r.Coverage.LocationCoveragePercent)
	}
}

func TestCoverage_EmptyWhenNoDataSupplied(t *testing.T) {
	r := labor.Calculate(labor.Input{Periods: fixtures.TwoMonthPeriods()}, labor.DefaultPolicy())
	if r.Coverage.PayrollRecordsSupplied {
		t.Error("expected PayrollRecordsSupplied false")
	}
	if r.Coverage.HoursCoveragePercent.Available {
		t.Error("expected HoursCoveragePercent unavailable with zero payroll records")
	}
}

func TestCoverage_GLAndFinancialMetricsAvailability(t *testing.T) {
	records, gl := fixtures.ReconciledPayrollAndGL()
	metrics := []labor.BusinessMetrics{{Period: "2025-06", Revenue: labor.AvailableValue(100000)}}
	r := labor.Calculate(labor.Input{
		Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records,
		GLControls: []labor.GLPayrollControl{gl}, BusinessMetrics: metrics,
	}, labor.DefaultPolicy())

	if !r.Coverage.GLReconciliationAvailable {
		t.Error("expected GLReconciliationAvailable true")
	}
	if !r.Coverage.FinancialMetricsAvailable {
		t.Error("expected FinancialMetricsAvailable true")
	}
}
