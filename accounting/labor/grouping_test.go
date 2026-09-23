package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestGrouping_Department(t *testing.T) {
	records := fixtures.MultiDepartmentCompany()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	if len(r.DepartmentSummaries) != 3 {
		t.Fatalf("expected 3 departments, got %d", len(r.DepartmentSummaries))
	}
	// Sorted by normalized name: Engineering, Sales, Support.
	if r.DepartmentSummaries[0].GroupKey != "Engineering" {
		t.Errorf("first department = %s, want Engineering (alphabetical)", r.DepartmentSummaries[0].GroupKey)
	}
	for _, d := range r.DepartmentSummaries {
		if d.GroupKey == "Engineering" && d.Headcount != 2 {
			t.Errorf("Engineering headcount = %d, want 2", d.Headcount)
		}
	}
}

func TestGrouping_Location(t *testing.T) {
	records := fixtures.MultiLocationCompany()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	if len(r.LocationSummaries) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(r.LocationSummaries))
	}
	for _, l := range r.LocationSummaries {
		if l.GroupKey == "Austin" && l.Headcount != 2 {
			t.Errorf("Austin headcount = %d, want 2", l.Headcount)
		}
	}
}

func TestGrouping_CostCenter(t *testing.T) {
	records := []labor.PayrollRecord{
		{ID: "CC-1", WorkerID: "CCW1", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: 1000, CostCenter: "CC100", Currency: "USD"},
		{ID: "CC-2", WorkerID: "CCW2", Period: "2025-06", PayDate: pdate("2025-06-30"), RegularPay: 2000, CostCenter: "CC200", Currency: "USD"},
	}
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())
	if len(r.CostCenterSummaries) != 2 {
		t.Fatalf("expected 2 cost centers, got %d", len(r.CostCenterSummaries))
	}
}

func TestGrouping_ShareOfTotalLaborCost(t *testing.T) {
	records := fixtures.MultiDepartmentCompany()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	var sum float64
	for _, d := range r.DepartmentSummaries {
		if !d.ShareOfTotalLaborCost.Available {
			t.Errorf("department %s: expected ShareOfTotalLaborCost available", d.GroupKey)
			continue
		}
		sum += d.ShareOfTotalLaborCost.Amount
	}
	if diff := sum - 1.0; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("shares should sum to 1.0, got %v", sum)
	}
}

func TestGrouping_DirectIndirectSplit(t *testing.T) {
	records := fixtures.DirectIndirectMix()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		s := p.DirectIndirectSplit
		if s.DirectLaborCost <= 0 || s.IndirectLaborCost <= 0 || s.UnclassifiedLaborCost <= 0 {
			t.Errorf("expected all three DirectIndirectSplit categories nonzero, got %+v", s)
		}
		if !s.DirectLaborPercent.Available {
			t.Error("expected DirectLaborPercent available")
		}
	}
}

func TestGrouping_DirectIndirectNeverInferredFromDepartmentName(t *testing.T) {
	// A record with Department "Production" (which sounds "direct") but no
	// explicit LaborClass must be Unclassified, never auto-classified as
	// Direct.
	records := []labor.PayrollRecord{
		{ID: "NI-1", WorkerID: "NIW1", Period: "2025-06", PayDate: pdate("2025-06-30"),
			RegularPay: 1000, Department: "Production", Currency: "USD"},
	}
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if p.DirectIndirectSplit.DirectLaborCost != 0 {
			t.Error("expected DirectLaborCost == 0 for unclassified record regardless of department name")
		}
		if p.DirectIndirectSplit.UnclassifiedLaborCost != 1000 {
			t.Errorf("expected UnclassifiedLaborCost == 1000, got %v", p.DirectIndirectSplit.UnclassifiedLaborCost)
		}
	}
}
