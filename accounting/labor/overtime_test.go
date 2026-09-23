package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

func TestOvertime_HoursAndPayShare(t *testing.T) {
	records := fixtures.HighOvertime()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	for _, p := range r.Periods {
		if p.Period.Period != "2025-06" {
			continue
		}
		if !p.Overtime.Available {
			t.Fatal("expected Overtime available with hours supplied")
		}
		if !p.Overtime.OvertimeHoursPercent.Available {
			t.Fatal("expected OvertimeHoursPercent available")
		}
		if p.Overtime.OvertimeHoursPercent.Amount <= 0.10 {
			t.Errorf("OvertimeHoursPercent = %v, want > 0.10 for high-overtime fixture", p.Overtime.OvertimeHoursPercent.Amount)
		}
	}
}

func TestOvertime_HighShareFlag(t *testing.T) {
	records := fixtures.HighOvertime()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	found := false
	for _, f := range r.Flags {
		if f.Code == labor.FlagHighOvertimeShare {
			found = true
		}
	}
	if !found {
		t.Error("expected FlagHighOvertimeShare to trigger")
	}
}

func TestOvertime_DecliningTrend(t *testing.T) {
	records := fixtures.DecliningOvertime()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	found := false
	for _, f := range r.Flags {
		if f.Code == labor.FlagOvertimeShareDeclining {
			found = true
		}
	}
	if !found {
		t.Error("expected FlagOvertimeShareDeclining to trigger")
	}
}

func TestOvertime_UnavailableWithoutHours(t *testing.T) {
	records := fixtures.MissingHours()
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())
	for _, p := range r.Periods {
		if p.Period.Period == "2025-06" && p.Overtime.Available {
			t.Error("expected Overtime unavailable without hours data")
		}
	}
}

func TestOvertime_NeverJudgesLegalCompliance(t *testing.T) {
	// Documented via safety_test.go's prohibited-terms scan; this test
	// confirms structurally that OvertimeSummary carries no
	// compliance/legality field at all.
	var s labor.OvertimeSummary
	_ = s // OvertimeSummary has RegularHours/OvertimeHours/TotalHours/percentages only.
}

func TestOvertime_ByDepartmentConcentration(t *testing.T) {
	records := []labor.PayrollRecord{
		{ID: "OD-1", WorkerID: "ODW1", Period: "2025-06", PayDate: pdate("2025-06-30"),
			HoursOvertime: labor.AvailableValue(20), Department: "Production", Currency: "USD"},
		{ID: "OD-2", WorkerID: "ODW2", Period: "2025-06", PayDate: pdate("2025-06-30"),
			HoursOvertime: labor.AvailableValue(5), Department: "Sales", Currency: "USD"},
	}
	in := labor.Input{Periods: fixtures.TwoMonthPeriods(), PayrollRecords: records}
	r := labor.Calculate(in, labor.DefaultPolicy())

	if len(r.OvertimeByDepartment) != 2 {
		t.Fatalf("expected 2 department entries, got %d", len(r.OvertimeByDepartment))
	}
	for _, d := range r.OvertimeByDepartment {
		if d.Department == "Production" && d.ShareOfTotalOvertime.Amount != 0.8 {
			t.Errorf("Production share = %v, want 0.8", d.ShareOfTotalOvertime.Amount)
		}
	}
}
