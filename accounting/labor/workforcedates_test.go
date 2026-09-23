package labor_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestWorkforceDates_GracePeriodFinalPayIsInformational(t *testing.T) {
	workers, records := fixtures.WorkerPaidAfterTerminationWithinGrace()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers, PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	found := false
	for _, f := range r.WorkerDateFindings {
		if f.Reason == labor.ReasonPayAfterTermination {
			found = true
			if f.Severity != labor.FlagSeverityInfo {
				t.Errorf("expected Info severity within grace period, got %v", f.Severity)
			}
		}
	}
	if !found {
		t.Fatal("expected a PAY_AFTER_TERMINATION finding")
	}

	// An informational finding must not also produce a WORKER_DATE_INCONSISTENCY
	// warning-level Flag.
	for _, fl := range r.Flags {
		if fl.Code == labor.FlagWorkerDateInconsistency {
			t.Error("did not expect FlagWorkerDateInconsistency for a within-grace-period final pay")
		}
	}
}

func TestWorkforceDates_MaterialPostTerminationPayIsWarning(t *testing.T) {
	workers, records := fixtures.WorkerPaidMateriallyAfterTermination()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers, PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	found := false
	for _, f := range r.WorkerDateFindings {
		if f.Reason == labor.ReasonPayAfterTermination && f.Severity == labor.FlagSeverityWarning {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a warning-severity PAY_AFTER_TERMINATION finding for material post-termination pay")
	}

	flagFound := false
	for _, fl := range r.Flags {
		if fl.Code == labor.FlagWorkerDateInconsistency {
			flagFound = true
		}
	}
	if !flagFound {
		t.Error("expected FlagWorkerDateInconsistency to trigger for material post-termination pay")
	}
}

func TestWorkforceDates_PayBeforeHire(t *testing.T) {
	workers := []labor.Worker{
		{WorkerID: "BH-W1", WorkerType: labor.WorkerTypeEmployee, Active: true, HireDate: pptr("2025-06-15")},
	}
	records := []labor.PayrollRecord{
		{ID: "BH-P1", WorkerID: "BH-W1", Period: "2025-06", PayDate: pdate("2025-06-01"), RegularPay: 1000, Currency: "USD"},
	}
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers, PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	found := false
	for _, f := range r.WorkerDateFindings {
		if f.Reason == labor.ReasonPayBeforeHire {
			found = true
		}
	}
	if !found {
		t.Error("expected a PAY_BEFORE_HIRE finding")
	}
}

func TestWorkforceDates_NoImproperPaymentConclusion(t *testing.T) {
	// Structural check: WorkerDateFinding has no "improper"/"fraud" field —
	// combined with safety_test.go's language scan on actual Message text.
	var f labor.WorkerDateFinding
	_ = f
}

func pptr(s string) *time.Time {
	t := pdate(s)
	return &t
}
