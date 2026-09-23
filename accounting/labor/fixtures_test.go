package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

// TestFixtures_AllScenariosProduceNoUnexpectedErrors runs every fixture
// scenario through Calculate to confirm none of them produce a Go panic
// or an unexpected error-severity Issue (some fixtures — mixed currency,
// missing worker IDs — legitimately produce warnings/errors by design,
// which this test tolerates; it only guards against a fixture that
// crashes Calculate).
func TestFixtures_AllScenariosProduceNoUnexpectedPanics(t *testing.T) {
	run := func(name string, in labor.Input) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Calculate panicked: %v", r)
				}
			}()
			_ = labor.Calculate(in, labor.DefaultPolicy())
		})
	}

	w1, p1 := fixtures.StableServiceBusiness()
	run("StableServiceBusiness", labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: w1, PayrollRecords: p1})

	w2, p2 := fixtures.LaborIntensiveBusiness()
	run("LaborIntensiveBusiness", labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: w2, PayrollRecords: p2})

	p3, m3 := fixtures.GrowingRevenueProportionalLabor()
	run("GrowingRevenueProportionalLabor", labor.Input{Periods: fixtures.ThreeMonthPeriods(), PayrollRecords: p3, BusinessMetrics: m3})

	p4, m4 := fixtures.LaborGrowthOutpacingRevenue()
	run("LaborGrowthOutpacingRevenue", labor.Input{Periods: fixtures.ThreeMonthPeriods(), PayrollRecords: p4, BusinessMetrics: m4})

	w5, m5 := fixtures.RevenueDeclineHeadcountGrowth()
	run("RevenueDeclineHeadcountGrowth", labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: w5, BusinessMetrics: m5})

	run("HighOvertime", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.HighOvertime()})
	run("DecliningOvertime", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.DecliningOvertime()})

	p8, c8 := fixtures.HighContractorShare()
	run("HighContractorShare", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: p8, ContractorRecords: c8})

	p9, c9 := fixtures.ContractorShareIncreasing()
	run("ContractorShareIncreasing", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: p9, ContractorRecords: c9})

	run("MultiDepartmentCompany", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.MultiDepartmentCompany()})
	run("MultiLocationCompany", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.MultiLocationCompany()})
	run("DirectIndirectMix", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.DirectIndirectMix()})

	p14, gl14 := fixtures.ReconciledPayrollAndGL()
	run("ReconciledPayrollAndGL", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: p14, GLControls: []labor.GLPayrollControl{gl14}})

	p15, gl15 := fixtures.PayrollGLMismatch()
	run("PayrollGLMismatch", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: p15, GLControls: []labor.GLPayrollControl{gl15}})

	w16, p16 := fixtures.WorkerPaidAfterTerminationWithinGrace()
	run("WorkerPaidAfterTerminationWithinGrace", labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: w16, PayrollRecords: p16})

	w17, p17 := fixtures.WorkerPaidMateriallyAfterTermination()
	run("WorkerPaidMateriallyAfterTermination", labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: w17, PayrollRecords: p17})

	run("MissingHours", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.MissingHours()})

	p19, m19 := fixtures.ZeroRevenueBusiness()
	run("ZeroRevenueBusiness", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: p19, BusinessMetrics: m19})

	w20, p20 := fixtures.ZeroWorkforce()
	run("ZeroWorkforce", labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: w20, PayrollRecords: p20})

	run("MixedCurrencyInvalid", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.MixedCurrencyInvalid()})
	run("FTECalculationCase", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.FTECalculationCase()})

	w23, p23 := fixtures.OwnerOperatedBusiness()
	run("OwnerOperatedBusiness", labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: w23, PayrollRecords: p23})

	run("BonusHeavyPeriod", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.BonusHeavyPeriod()})
	run("PayrollCashScheduleAvailable", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.PayrollCashScheduleAvailable()})

	periods26, p26 := fixtures.PayrollExpenseOnlyCashUnavailable()
	run("PayrollExpenseOnlyCashUnavailable", labor.Input{Periods: periods26, PayrollRecords: p26})

	run("DuplicatePayrollRecords", labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: fixtures.DuplicatePayrollRecords()})

	w28, p28 := fixtures.KeyWorkerScenario()
	run("KeyWorkerScenario", labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: w28, PayrollRecords: p28})
}

func TestFixtures_PayrollExpenseOnlyCashUnavailable(t *testing.T) {
	periods, records := fixtures.PayrollExpenseOnlyCashUnavailable()
	in := labor.Input{Periods: periods, PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if !p.PayrollBasis.ExpenseBasis.Available {
			t.Error("expected ExpenseBasis available (accrual period matches)")
		}
		if p.PayrollBasis.CashBasis.Available {
			t.Error("expected CashBasis unavailable (pay date falls outside every supplied period window)")
		}
	}
}

func TestFixtures_KeyWorkerConcentration(t *testing.T) {
	workers, records := fixtures.KeyWorkerScenario()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers, PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if !p.KeyWorkerConcentration.Available {
			t.Fatal("expected KeyWorkerConcentration available when a worker is marked KeyWorker")
		}
		if p.KeyWorkerConcentration.KeyWorkerLaborCost <= 0 {
			t.Error("expected nonzero KeyWorkerLaborCost")
		}
	}
}
