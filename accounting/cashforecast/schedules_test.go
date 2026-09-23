package cashforecast

import "testing"

func TestSchedules_Payroll(t *testing.T) {
	events := buildPayrollEvents([]PayrollEvent{
		{ID: "pr1", Date: testDate(t, "2025-01-10"), EmployeeNetCash: 8000, EmployerTaxes: 800, EmployeeWithholdingsRemitted: 1200, Benefits: 300, OtherCash: 50},
	})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	want := 8000.0 + 800 + 1200 + 300 + 50
	if events[0].Amount != want {
		t.Errorf("Amount = %v, want %v (sum of all cash-out components)", events[0].Amount, want)
	}
	if events[0].Category != CategoryPayroll || events[0].Direction != DirectionOutflow {
		t.Errorf("unexpected Category/Direction: %+v", events[0])
	}
	if events[0].SourceType != SourcePayrollSchedule || events[0].SourceID != "pr1" {
		t.Errorf("unexpected provenance: %+v", events[0])
	}
}

func TestSchedules_TaxCategories(t *testing.T) {
	cases := []struct {
		tax  TaxCategory
		want CashCategory
	}{
		{TaxSales, CategorySalesTax},
		{TaxPayrollRemit, CategoryPayrollTax},
		{TaxIncome, CategoryIncomeTax},
		{TaxOther, CategoryOtherOperatingOutflow},
		{"UNKNOWN", CategoryOtherOperatingOutflow},
	}
	for _, c := range cases {
		events := buildTaxEvents([]TaxEvent{{ID: "t1", Date: testDate(t, "2025-01-10"), Amount: 100, Category: c.tax}})
		if len(events) != 1 || events[0].Category != c.want {
			t.Errorf("tax category %s -> %v, want %v", c.tax, events[0].Category, c.want)
		}
	}
}

func TestSchedules_DebtServiceAndCapex(t *testing.T) {
	debtEvents := buildDebtServiceEvents([]DebtServiceEvent{
		{ID: "d1", Date: testDate(t, "2025-01-10"), Amount: 5000, LoanLabel: "Term loan A"},
	})
	if len(debtEvents) != 1 || debtEvents[0].Category != CategoryDebtService || debtEvents[0].Direction != DirectionOutflow {
		t.Errorf("unexpected debt service event: %+v", debtEvents)
	}
	if debtEvents[0].Description != "Term loan A" {
		t.Errorf("Description = %s, want 'Term loan A'", debtEvents[0].Description)
	}

	capexEvents := buildCapexEvents([]CapexEvent{
		{ID: "c1", Date: testDate(t, "2025-02-01"), Amount: 15000, Description: "New equipment", Commitment: CommitmentDiscretionary},
	})
	if len(capexEvents) != 1 || capexEvents[0].Category != CategoryCapex || capexEvents[0].Commitment != CommitmentDiscretionary {
		t.Errorf("unexpected capex event: %+v", capexEvents)
	}
}

func TestSchedules_FinancingEventTypes(t *testing.T) {
	cases := []struct {
		typ           FinancingEventType
		wantCategory  CashCategory
		wantDirection CashDirection
	}{
		{FinancingLoanProceeds, CategoryLoanProceeds, DirectionInflow},
		{FinancingLineOfCreditDraw, CategoryLoanProceeds, DirectionInflow},
		{FinancingLoanRepayment, CategoryDebtService, DirectionOutflow},
		{FinancingEquityContribution, CategoryOwnerContribution, DirectionInflow},
		{FinancingOwnerContribution, CategoryOwnerContribution, DirectionInflow},
		{FinancingOwnerDistribution, CategoryOwnerDistribution, DirectionOutflow},
	}
	for _, c := range cases {
		events, issues := buildFinancingEvents([]FinancingEvent{{ID: "f1", Date: testDate(t, "2025-01-10"), Amount: 1000, Type: c.typ}})
		if len(issues) != 0 {
			t.Errorf("type %s: unexpected issues %+v", c.typ, issues)
		}
		if len(events) != 1 || events[0].Category != c.wantCategory || events[0].Direction != c.wantDirection {
			t.Errorf("type %s: got Category=%v Direction=%v, want Category=%v Direction=%v",
				c.typ, events[0].Category, events[0].Direction, c.wantCategory, c.wantDirection)
		}
	}
}

func TestSchedules_FinancingEventUnrecognizedTypeFlagged(t *testing.T) {
	events, issues := buildFinancingEvents([]FinancingEvent{{ID: "f1", Date: testDate(t, "2025-01-10"), Amount: 1000, Type: "BOGUS"}})
	if len(events) != 0 {
		t.Errorf("expected 0 events for unrecognized type, got %d", len(events))
	}
	if !hasIssueCode(issues, IssueInvalidCategory) {
		t.Errorf("expected IssueInvalidCategory, got %+v", issues)
	}
}

// TestSchedules_FinancingIntegration proves a FinancingEvent flows all the
// way through Calculate into a scenario's weekly totals with the
// financing cash-flow-class split populated correctly.
func TestSchedules_FinancingIntegration(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Financing: []FinancingEvent{
			{ID: "loan1", Date: testDate(t, "2025-01-08"), Amount: 50000, Type: FinancingLoanProceeds},
		},
	}
	result := Calculate(in, Options{})
	if !result.Available {
		t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
	}
	w := result.BaseScenario.Weekly[0]
	if w.Inflows.Total != 50000 {
		t.Errorf("week 1 inflow = %v, want 50000", w.Inflows.Total)
	}
	if w.FinancingInflows != 50000 {
		t.Errorf("week 1 FinancingInflows = %v, want 50000", w.FinancingInflows)
	}
	if w.OperatingInflows != 0 {
		t.Errorf("week 1 OperatingInflows = %v, want 0 (financing should not count as operating)", w.OperatingInflows)
	}
}
