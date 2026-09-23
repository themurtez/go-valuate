package labor_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
)

func TestRoundTrip_Result(t *testing.T) {
	r := labor.Calculate(fullInput(), labor.DefaultPolicy())
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var r2 labor.Result
	if err := json.Unmarshal(b, &r2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, err := json.Marshal(r2)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(b) != string(b2) {
		t.Error("expected Result to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_Worker(t *testing.T) {
	w := labor.Worker{WorkerID: "W1", WorkerType: labor.WorkerTypeEmployee, Active: true, Department: "Ops"}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var w2 labor.Worker
	if err := json.Unmarshal(b, &w2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(w2)
	if string(b) != string(b2) {
		t.Error("expected Worker to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_PayrollRecord(t *testing.T) {
	records := fullInput().PayrollRecords
	if len(records) == 0 {
		t.Fatal("expected at least one payroll record")
	}
	b, err := json.Marshal(records[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var r2 labor.PayrollRecord
	if err := json.Unmarshal(b, &r2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(r2)
	if string(b) != string(b2) {
		t.Error("expected PayrollRecord to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_ContractorLaborRecord(t *testing.T) {
	c := labor.ContractorLaborRecord{ID: "C1", ContractorID: "CON1", Period: "2025-06", Amount: 1000, Currency: "USD"}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var c2 labor.ContractorLaborRecord
	if err := json.Unmarshal(b, &c2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(c2)
	if string(b) != string(b2) {
		t.Error("expected ContractorLaborRecord to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_PeriodSummary(t *testing.T) {
	r := labor.Calculate(fullInput(), labor.DefaultPolicy())
	if len(r.Periods) == 0 {
		t.Fatal("expected at least one PeriodSummary")
	}
	b, err := json.Marshal(r.Periods[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var p2 labor.PeriodSummary
	if err := json.Unmarshal(b, &p2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(p2)
	if string(b) != string(b2) {
		t.Error("expected PeriodSummary to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_ReconciliationSummary(t *testing.T) {
	r := labor.Calculate(fullInput(), labor.DefaultPolicy())
	b, err := json.Marshal(r.ReconciliationSummary)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var s2 labor.ReconciliationSummary
	if err := json.Unmarshal(b, &s2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(s2)
	if string(b) != string(b2) {
		t.Error("expected ReconciliationSummary to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_GroupSummary(t *testing.T) {
	r := labor.Calculate(fullInput(), labor.DefaultPolicy())
	if len(r.DepartmentSummaries) == 0 {
		t.Skip("no department summaries in fixture")
	}
	b, err := json.Marshal(r.DepartmentSummaries[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var g2 labor.GroupSummary
	if err := json.Unmarshal(b, &g2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(g2)
	if string(b) != string(b2) {
		t.Error("expected GroupSummary to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_Trend(t *testing.T) {
	r := labor.Calculate(fullInput(), labor.DefaultPolicy())
	b, err := json.Marshal(r.Trend)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var t2 labor.TrendResult
	if err := json.Unmarshal(b, &t2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(t2)
	if string(b) != string(b2) {
		t.Error("expected TrendResult to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_IssuesAndFlags(t *testing.T) {
	r := labor.Calculate(fullInput(), labor.DefaultPolicy())

	bi, err := json.Marshal(r.Issues)
	if err != nil {
		t.Fatalf("marshal issues: %v", err)
	}
	var issues2 []labor.Issue
	if err := json.Unmarshal(bi, &issues2); err != nil {
		t.Fatalf("unmarshal issues: %v", err)
	}
	bi2, _ := json.Marshal(issues2)
	if string(bi) != string(bi2) {
		t.Error("expected Issues to round-trip through JSON byte-for-byte")
	}

	bf, err := json.Marshal(r.Flags)
	if err != nil {
		t.Fatalf("marshal flags: %v", err)
	}
	var flags2 []labor.Flag
	if err := json.Unmarshal(bf, &flags2); err != nil {
		t.Fatalf("unmarshal flags: %v", err)
	}
	bf2, _ := json.Marshal(flags2)
	if string(bf) != string(bf2) {
		t.Error("expected Flags to round-trip through JSON byte-for-byte")
	}
}

func TestRoundTrip_Policy(t *testing.T) {
	policy := labor.DefaultPolicy()
	b, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var p2 labor.Policy
	if err := json.Unmarshal(b, &p2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, _ := json.Marshal(p2)
	if string(b) != string(b2) {
		t.Error("expected Policy to round-trip through JSON byte-for-byte")
	}
}
