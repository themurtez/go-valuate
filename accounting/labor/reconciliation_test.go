package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestReconciliation_ExactMatch(t *testing.T) {
	records, gl := fixtures.ReconciledPayrollAndGL()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records, GLControls: []labor.GLPayrollControl{gl}}
	r := labor.Calculate(in, labor.DefaultPolicy())

	if r.ReconciliationSummary.Status != labor.ReconciliationStatusReconciled {
		t.Errorf("status = %v, want RECONCILED", r.ReconciliationSummary.Status)
	}
	for _, c := range r.ReconciliationSummary.Components {
		if c.Register.Available && c.GL.Available && !c.Reconciled {
			t.Errorf("component %s not reconciled: %+v", c.Component, c)
		}
	}
}

func TestReconciliation_WithinTolerance(t *testing.T) {
	records, gl := fixtures.ReconciledPayrollAndGL()
	gl.GrossWages = labor.AvailableValue(10002) // small difference
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records, GLControls: []labor.GLPayrollControl{gl}}
	policy := labor.DefaultPolicy()
	policy.ReconciliationTolerance.AbsoluteTolerance = 5
	r := labor.Calculate(in, policy)

	for _, c := range r.ReconciliationSummary.Components {
		if c.Component == "gross_wages" && !c.Reconciled {
			t.Errorf("expected gross_wages reconciled within tolerance, got %+v", c)
		}
	}
}

func TestReconciliation_MaterialMismatch(t *testing.T) {
	records, gl := fixtures.PayrollGLMismatch()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records, GLControls: []labor.GLPayrollControl{gl}}
	r := labor.Calculate(in, labor.DefaultPolicy())

	if r.ReconciliationSummary.Status == labor.ReconciliationStatusReconciled {
		t.Error("expected reconciliation NOT fully reconciled for material mismatch fixture")
	}
	foundMismatchFlag := false
	for _, f := range r.Flags {
		if f.Code == labor.FlagPayrollGLMismatch {
			foundMismatchFlag = true
		}
	}
	if !foundMismatchFlag {
		t.Error("expected FlagPayrollGLMismatch to trigger")
	}
}

func TestReconciliation_ComponentMismatchWithTotalMatchStillReported(t *testing.T) {
	// Deliberately craft GL control where individual components don't
	// match but total happens to. Per section 59, component-level
	// differences must never be hidden behind a matching total.
	records := []labor.PayrollRecord{
		{ID: "CM-1", WorkerID: "CMW1", Period: "2025-06", PayDate: pdate("2025-06-30"),
			RegularPay: 10000, EmployerTaxes: 800, BenefitsCost: 500, Currency: "USD"},
	}
	gl := labor.GLPayrollControl{
		Period:         "2025-06",
		GrossWages:     labor.AvailableValue(10500), // off by 500
		EmployerTaxes:  labor.AvailableValue(800),
		Benefits:       labor.AvailableValue(0), // off by -500 (net cancels in some naive total)
		TotalLaborCost: labor.AvailableValue(11300),
	}
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records, GLControls: []labor.GLPayrollControl{gl}}
	r := labor.Calculate(in, labor.DefaultPolicy())

	if r.ReconciliationSummary.Status != labor.ReconciliationStatusReconciledWithDifferences &&
		r.ReconciliationSummary.Status != labor.ReconciliationStatusUnreconciled {
		t.Errorf("expected status to reflect component-level differences, got %v", r.ReconciliationSummary.Status)
	}
	var grossReconciled, benefitsReconciled bool
	for _, c := range r.ReconciliationSummary.Components {
		if c.Component == "gross_wages" {
			grossReconciled = c.Reconciled
		}
		if c.Component == "benefits" {
			benefitsReconciled = c.Reconciled
		}
	}
	if grossReconciled {
		t.Error("expected gross_wages component NOT reconciled (500 difference, 0 tolerance)")
	}
	if benefitsReconciled {
		t.Error("expected benefits component NOT reconciled (500 difference, 0 tolerance)")
	}
}

func TestReconciliation_UnavailableWithoutGL(t *testing.T) {
	_, records := fixtures.StableServiceBusiness()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	if r.ReconciliationSummary.Status != labor.ReconciliationStatusUnavailable {
		t.Errorf("status = %v, want UNAVAILABLE", r.ReconciliationSummary.Status)
	}
	if r.Coverage.GLReconciliationAvailable {
		t.Error("expected GLReconciliationAvailable false without GL controls")
	}
}
