package profitability_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/inventory"
	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/ledger"
	ledgerfixtures "github.com/themurtez/go-valuate/accounting/ledger/fixtures"
	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/financial"
)

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// TestIntegration_Statements demonstrates task section 39: building
// ControlTotals from a financial.FinancialDataset.
func TestIntegration_Statements(t *testing.T) {
	dataset := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeRevProduct, Period: "2025-01", Amount: 50000},
			{Code: financial.CodeCogsMaterial, Period: "2025-01", Amount: 20000},
		},
	}
	ct := profitability.ControlTotalsFromFinancialDataset(dataset, "2025-01")
	if !ct.NetRevenue.Available || ct.NetRevenue.Amount != 50000 {
		t.Errorf("NetRevenue = %+v, want 50000", ct.NetRevenue)
	}
	if !ct.DirectCost.Available || ct.DirectCost.Amount != 20000 {
		t.Errorf("DirectCost = %+v, want 20000", ct.DirectCost)
	}

	entities, facts := fixtures.HighRevenueLowMarginCustomer()
	r := profitability.Calculate(profitability.Input{
		Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts,
		Controls: []profitability.ControlTotals{ct},
	}, profitability.DefaultPolicy())
	if len(r.ControlReconciliation) == 0 {
		t.Fatal("expected ControlReconciliation entries")
	}
}

// TestIntegration_Labor demonstrates task section 40: converting an
// accounting/labor PayrollRecord into a DIRECT_LABOR Fact via explicit
// attribution.
func TestIntegration_Labor(t *testing.T) {
	record := labor.PayrollRecord{
		ID: "PR-1", WorkerID: "W1", Period: "2025-01", PayDate: mustDate("2025-01-31"),
		RegularPay: 4000, EmployerTaxes: 300, BenefitsCost: 200, Currency: "USD",
	}
	attrs := profitability.DirectAttributions([2]string{"JOB", "JOB-1"})
	fact := profitability.DirectLaborFactFromPayrollRecord(record, attrs)

	if fact.Component != profitability.ComponentDirectLabor {
		t.Errorf("Component = %s, want DIRECT_LABOR", fact.Component)
	}
	wantAmount := 4000.0 + 300 + 200
	if fact.Amount != wantAmount {
		t.Errorf("Amount = %v, want %v", fact.Amount, wantAmount)
	}

	entities := []profitability.Entity{{Dimension: profitability.DimensionJob, EntityID: "JOB-1", Active: true}}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: []profitability.Fact{fact}}, profitability.DefaultPolicy())
	if len(r.JobView.AllPeriod) != 1 || r.JobView.AllPeriod[0].DirectCostBridge.DirectLabor != wantAmount {
		t.Errorf("unexpected JobView result: %+v", r.JobView.AllPeriod)
	}

	// No employee-performance/productivity field exists on Fact at all —
	// the type system itself enforces task section 40's "no
	// employee-performance semantics" boundary.
}

// TestIntegration_Inventory demonstrates task section 41: adapting an
// authoritative accounting/inventory UnitCost, and confirms the adapter
// stays unavailable (rather than inventing a cost) when UnitCost is not
// supplied.
func TestIntegration_Inventory(t *testing.T) {
	snapshot := inventory.InventorySnapshot{
		ID: "SNAP-1", ItemID: "ITEM-1", AsOfDate: mustDate("2025-01-31"),
		UnitCost: inventory.AvailableValue(12.50), Currency: "USD",
	}
	attrs := profitability.DirectAttributions([2]string{"PRODUCT", "PROD-1"})
	fact, ok := profitability.ProductCostFactFromInventorySnapshot(snapshot, 100, "INV-COST-1", "2025-01", attrs)
	if !ok {
		t.Fatal("expected the adapter to succeed with an available UnitCost")
	}
	if fact.Amount != 1250 {
		t.Errorf("Amount = %v, want 1250 (100 units x 12.50)", fact.Amount)
	}
	if fact.Component != profitability.ComponentDirectMaterial {
		t.Errorf("Component = %s, want DIRECT_MATERIAL", fact.Component)
	}

	noCost := inventory.InventorySnapshot{ID: "SNAP-2", ItemID: "ITEM-2", AsOfDate: mustDate("2025-01-31")}
	_, ok2 := profitability.ProductCostFactFromInventorySnapshot(noCost, 50, "INV-COST-2", "2025-01", attrs)
	if ok2 {
		t.Error("expected the adapter to report unavailable when UnitCost is not supplied, never invent a cost")
	}
}

// TestIntegration_Ledger demonstrates task section 42: an explicit
// caller-supplied ledger account -> component/dimension mapping (no
// name-based inference).
func TestIntegration_Ledger(t *testing.T) {
	chart := ledger.BuildChartOfAccounts(ledgerfixtures.ServiceBusinessChart())
	entries := ledgerfixtures.ServiceBusinessEntries()
	balances := ledger.CalculateBalances(chart, entries, ledger.BalanceOptions{})

	mapping := []profitability.LedgerAccountMapping{
		{AccountID: "4000", Component: profitability.ComponentGrossRevenue,
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-LEDGER"})},
	}
	facts := profitability.FactsFromLedgerBalances(balances, mapping, "2025-01")
	if len(facts) != 1 {
		t.Fatalf("expected 1 mapped fact, got %d", len(facts))
	}
	if facts[0].Component != profitability.ComponentGrossRevenue {
		t.Errorf("Component = %s, want GROSS_REVENUE", facts[0].Component)
	}
	if facts[0].Amount != 12000 {
		t.Errorf("Amount = %v, want 12000 (consulting revenue account movement)", facts[0].Amount)
	}

	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "CUST-LEDGER", Active: true}}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())
	if len(r.CustomerView.AllPeriod) != 1 || r.CustomerView.AllPeriod[0].RevenueBridge.GrossRevenue != 12000 {
		t.Errorf("unexpected CustomerView result: %+v", r.CustomerView.AllPeriod)
	}
}

// TestIntegration_Concentration demonstrates task section 43: reusing
// analytics/concentration for revenue concentration, and confirms the
// figures are byte-identical to calling analytics/concentration directly.
func TestIntegration_Concentration(t *testing.T) {
	entities, facts := fixtures.GroupCategorySummaryScenario()
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())

	observations := profitability.ConcentrationObservationsFromEntityPeriods(r.CustomerView.EntityPeriods)
	periodMeta := map[financial.Period]concentration.PeriodInfo{
		"2025-01": {Type: concentration.PeriodTypeMonth, FiscalYear: 2025, SequenceInYear: 1},
	}
	concResult := concentration.Calculate(concentration.Input{
		Basis: concentration.BasisCustomerRevenue, Observations: observations, PeriodMeta: periodMeta,
	}, concentration.Options{})
	if !concResult.Available {
		t.Fatal("expected a usable concentration result")
	}

	rc := profitability.RevenueConcentrationFromConcentrationResult(profitability.DimensionCustomer, concResult)
	if !rc.Top1Share.Available {
		t.Fatal("expected Top1Share available")
	}
	wantTop1 := concResult.History[len(concResult.History)-1].LargestEntityShare.Value
	if rc.Top1Share.Amount != wantTop1 {
		t.Errorf("Top1Share = %v, want %v (byte-identical to analytics/concentration's own figure)", rc.Top1Share.Amount, wantTop1)
	}
}
