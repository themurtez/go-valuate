package labor_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/cashforecast"
	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func parseDateForTest(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func parseDateMustForTest(s string) time.Time {
	t, err := parseDateForTest(s)
	if err != nil {
		panic(err)
	}
	return t
}

// TestIntegration_CashForecastPayrollEvents proves known payroll cash
// obligations (a caller who knows the actual employer cash outlay per pay
// date) can feed accounting/cashforecast as CategoryPayroll outflow
// events — the task's section 31/62. This package has no compile-time
// dependency on accounting/cashforecast; this test performs the
// conversion at the call site, exactly as a real integrating caller
// would.
func TestIntegration_CashForecastPayrollEvents(t *testing.T) {
	records := fixtures.PayrollCashScheduleAvailable()
	cashEvents := labor.CashForecastPayrollEvents(records)
	if len(cashEvents) == 0 {
		t.Fatal("expected at least one PayrollCashEvent")
	}

	var events []cashforecast.CashFlowEvent
	for _, e := range cashEvents {
		d, err := parseDateForTest(e.Date)
		if err != nil {
			t.Fatalf("parse date %q: %v", e.Date, err)
		}
		events = append(events, cashforecast.CashFlowEvent{
			ID:         "payroll#" + e.SourceID,
			Date:       d,
			Amount:     e.Amount,
			Direction:  cashforecast.DirectionOutflow,
			Category:   cashforecast.CategoryPayroll,
			SourceType: cashforecast.SourceManual,
			SourceID:   e.SourceID,
			Basis:      cashforecast.BasisKnown,
		})
	}

	in := cashforecast.Input{
		ForecastStartDate: parseDateMustForTest("2025-05-01"),
		OpeningCash:       cashforecast.OpeningCash{Amount: 50000, Currency: "USD"},
		Events:            events,
	}
	result := cashforecast.Calculate(in, cashforecast.Options{})
	if !result.Available {
		t.Fatal("expected cashforecast.Result.Available true")
	}
	if result.BaseScenario.Summary.TotalOutflows <= 0 {
		t.Error("expected nonzero total outflows from payroll cash events")
	}
}

// TestIntegration_FinancialMetricsAdapter demonstrates converting a
// simple external revenue/gross-profit/EBITDA source into labor's
// portable BusinessMetrics — the task's section 61. This package never
// depends on financial/analytics packages; the conversion is the
// caller's own responsibility, demonstrated here as a plain field
// mapping.
func TestIntegration_FinancialMetricsAdapter(t *testing.T) {
	// Stand-in for a real financial-statement/analytics Result: any
	// source with Revenue/GrossProfit/EBITDA-shaped fields converts via
	// the same one-to-one mapping.
	type externalMetrics struct {
		Period      string
		Revenue     float64
		GrossProfit float64
		EBITDA      float64
	}
	external := []externalMetrics{
		{Period: "2025-06", Revenue: 500000, GrossProfit: 250000, EBITDA: 100000},
	}

	var metrics []labor.BusinessMetrics
	for _, e := range external {
		metrics = append(metrics, labor.BusinessMetrics{
			Period:      e.Period,
			Revenue:     labor.AvailableValue(e.Revenue),
			GrossProfit: labor.AvailableValue(e.GrossProfit),
			EBITDA:      labor.AvailableValue(e.EBITDA),
		})
	}

	workers, records := fixtures.StableServiceBusiness()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), Workers: workers, PayrollRecords: records, BusinessMetrics: metrics}
	r := labor.Calculate(in, labor.DefaultPolicy())

	for _, p := range r.Periods {
		if p.Period.Period == "2025-06" && !p.Productivity.RevenuePerEmployee.Available {
			t.Error("expected RevenuePerEmployee available after financial-metrics adapter conversion")
		}
	}
}

// TestIntegration_LedgerAdapter demonstrates aggregating caller-identified
// GL account balances into a GLPayrollControl via
// GLControlFromLedgerBalances — the task's section 30. Account-to-category
// mapping is entirely caller-declared; no inference from account names.
func TestIntegration_LedgerAdapter(t *testing.T) {
	balances := []labor.LedgerAccountBalance{
		{AccountID: "6000", Balance: 10000}, // gross wages
		{AccountID: "6100", Balance: 800},   // employer taxes
		{AccountID: "6200", Balance: 500},   // benefits
		{AccountID: "9999", Balance: 12345}, // unmapped — must be excluded
	}
	mapping := []labor.LedgerAccountMapping{
		{AccountID: "6000", Category: labor.LedgerCategoryGrossWages},
		{AccountID: "6100", Category: labor.LedgerCategoryEmployerTaxes},
		{AccountID: "6200", Category: labor.LedgerCategoryBenefits},
	}
	control := labor.GLControlFromLedgerBalances("2025-06", balances, mapping)

	if !control.GrossWages.Available || control.GrossWages.Amount != 10000 {
		t.Errorf("GrossWages = %+v, want available 10000", control.GrossWages)
	}
	if !control.TotalLaborCost.Available || control.TotalLaborCost.Amount != 11300 {
		t.Errorf("TotalLaborCost = %+v, want available 11300 (unmapped account excluded)", control.TotalLaborCost)
	}

	records, _ := fixtures.ReconciledPayrollAndGL()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records, GLControls: []labor.GLPayrollControl{control}}
	r := labor.Calculate(in, labor.DefaultPolicy())
	if r.ReconciliationSummary.Status == labor.ReconciliationStatusUnavailable {
		t.Error("expected reconciliation available after ledger adapter conversion")
	}
}

// TestIntegration_StatementReconciliation proves a labor register's
// payroll-expense total can reconcile against a caller-supplied "GL
// control" figure representing the corresponding statement line —
// demonstrating the task's section 29 without hardwiring any
// accounting/statements account code into this package. The GL control
// figure here stands in for whatever accounting/statements reports as
// its payroll/wages expense row; this package only ever consumes a plain
// float64 GLPayrollControl.TotalLaborCost, never a statements.Row.
func TestIntegration_StatementReconciliation(t *testing.T) {
	_, records := fixtures.StableServiceBusiness()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	unreconciled := labor.Calculate(in, labor.DefaultPolicy())

	var juneTotal float64
	for _, p := range unreconciled.Periods {
		if p.Period.Period == "2025-06" {
			juneTotal = p.LaborCostBridge.TotalLaborCost
		}
	}
	if juneTotal == 0 {
		t.Fatal("expected nonzero June total labor cost")
	}

	// Simulate a statement-builder payroll-expense line exactly matching
	// the register total.
	glFromStatement := labor.GLPayrollControl{
		Period:         "2025-06",
		TotalLaborCost: labor.AvailableValue(juneTotal),
	}
	in.GLControls = []labor.GLPayrollControl{glFromStatement}
	reconciled := labor.Calculate(in, labor.DefaultPolicy())

	if reconciled.ReconciliationSummary.Status != labor.ReconciliationStatusReconciled {
		t.Errorf("status = %v, want RECONCILED when GL matches register exactly", reconciled.ReconciliationSummary.Status)
	}
}
