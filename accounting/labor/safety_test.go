package labor_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

// prohibitedTerms are the words this package's generated text must never
// contain, per the task's sections 63-64: payroll-law/legal-conclusion
// terms, worker-decision terms, and (for consistency with sibling
// packages like journaldiagnostics) fraud-adjacent terms. Matching is
// case-insensitive and substring-based (so "terminated" also fails on
// "terminate").
var prohibitedTerms = []string{
	// Payroll-law / legal-conclusion terms — section 63.
	"violation", "illegal", "noncompliant", "non-compliant", "misclassif",
	"underpaid", "underpayment", "unlawful", "breach",
	// Worker-decision / ranking terms — section 64.
	"fire", "terminate", "layoff", "lay off", "discipline", "promote", "demote",
	"best employee", "worst employee", "best worker", "worst worker",
	"performance score", "ranking",
	// Fraud-adjacent terms — kept consistent with journaldiagnostics's
	// identical non-fraud boundary even though this package's domain
	// (payroll analytics) rarely produces fraud-adjacent language.
	"fraud", "fraudulent", "theft", "embezzlement", "misconduct",
}

func scanForProhibitedLanguage(t *testing.T, label, text string) {
	t.Helper()
	lower := strings.ToLower(text)
	for _, term := range prohibitedTerms {
		if strings.Contains(lower, term) {
			t.Errorf("%s contains prohibited term %q: %q", label, term, text)
		}
	}
}

// buildSafetyFixtureInput assembles an Input that exercises as many
// Flag/Issue/WorkerDateFinding code paths as possible in one Calculate
// call, so the language scan below is meaningful.
func buildSafetyFixtureInput() labor.Input {
	var periods []labor.PeriodInfo
	periods = append(periods, fixtures.ThreeMonthPeriods()...)

	var workers []labor.Worker
	var payroll []labor.PayrollRecord
	var contractors []labor.ContractorLaborRecord

	w1, p1 := fixtures.StableServiceBusiness()
	workers = append(workers, w1...)
	payroll = append(payroll, p1...)

	payroll = append(payroll, fixtures.HighOvertime()...)
	payroll = append(payroll, fixtures.DecliningOvertime()...)

	p2, c2 := fixtures.HighContractorShare()
	payroll = append(payroll, p2...)
	contractors = append(contractors, c2...)

	p3, c3 := fixtures.ContractorShareIncreasing()
	payroll = append(payroll, p3...)
	contractors = append(contractors, c3...)

	wg, pg := fixtures.WorkerPaidAfterTerminationWithinGrace()
	workers = append(workers, wg...)
	payroll = append(payroll, pg...)

	wm, pm := fixtures.WorkerPaidMateriallyAfterTermination()
	workers = append(workers, wm...)
	payroll = append(payroll, pm...)

	payroll = append(payroll, fixtures.DuplicatePayrollRecords()...)

	lgRecords, metrics := fixtures.LaborGrowthOutpacingRevenue()
	payroll = append(payroll, lgRecords...)

	rdWorkers, rdMetrics := fixtures.RevenueDeclineHeadcountGrowth()
	workers = append(workers, rdWorkers...)
	metrics = append(metrics, rdMetrics...)

	_, mismatchGL := fixtures.PayrollGLMismatch()
	glControls := []labor.GLPayrollControl{mismatchGL}

	_, reconciledGL := fixtures.ReconciledPayrollAndGL()
	_ = reconciledGL

	return labor.Input{
		Periods:           periods,
		Workers:           workers,
		PayrollRecords:    payroll,
		ContractorRecords: contractors,
		BusinessMetrics:   metrics,
		GLControls:        glControls,
	}
}

func TestSafety_NoProhibitedLanguageInFlagMessages(t *testing.T) {
	r := labor.Calculate(buildSafetyFixtureInput(), labor.DefaultPolicy())
	if len(r.Flags) == 0 {
		t.Fatal("expected at least one Flag from the fixture set to make this scan meaningful")
	}
	seen := make(map[labor.FlagCode]bool)
	for _, f := range r.Flags {
		scanForProhibitedLanguage(t, "Flag "+string(f.Code)+" Message", f.Message)
		seen[f.Code] = true
	}
	if len(seen) < 5 {
		t.Errorf("expected the fixture set to exercise at least 5 distinct FlagCodes for full message coverage, got %d: %v", len(seen), seen)
	}
}

func TestSafety_NoProhibitedLanguageInIssueMessages(t *testing.T) {
	in := labor.Input{
		Periods: fixtures.TwoMonthPeriods(),
		PayrollRecords: append(
			fixtures.MixedCurrencyInvalid(),
			labor.PayrollRecord{ID: "BAD-1", WorkerID: "", Period: "2025-06", PayDate: date("2025-06-30")},
		),
	}
	r := labor.Calculate(in, labor.DefaultPolicy())
	if len(r.Issues) == 0 {
		t.Fatal("expected at least one Issue from the broken-input fixture")
	}
	for _, iss := range r.Issues {
		scanForProhibitedLanguage(t, "Issue "+string(iss.Code)+" Message", iss.Message)
	}
}

func TestSafety_NoProhibitedLanguageInWorkerDateFindings(t *testing.T) {
	r := labor.Calculate(buildSafetyFixtureInput(), labor.DefaultPolicy())
	if len(r.WorkerDateFindings) == 0 {
		t.Fatal("expected at least one WorkerDateFinding from the fixture set")
	}
	for _, f := range r.WorkerDateFindings {
		scanForProhibitedLanguage(t, "WorkerDateFinding "+string(f.Reason)+" Message", f.Message)
	}
}

func TestSafety_NaNInfThresholdsRejected(t *testing.T) {
	policy := labor.DefaultPolicy()
	policy.MaterialPayrollDifference = math.NaN()
	r := labor.Calculate(fullInput(), policy)

	found := false
	for _, iss := range r.Issues {
		if iss.Code == labor.IssueInvalidPolicy {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidPolicy for a NaN MaterialPayrollDifference")
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "NaN") || strings.Contains(string(b), "Infinity") {
		t.Error("Result JSON must never contain NaN or Infinity")
	}
}

func TestSafety_NonFiniteAmountsExcludedNotPropagated(t *testing.T) {
	in := fullInput()
	in.PayrollRecords = append(in.PayrollRecords, labor.PayrollRecord{
		ID: "INF-1", WorkerID: "W1", Period: "2025-06", PayDate: date("2025-06-30"),
		RegularPay: math.Inf(1), Currency: "USD",
	})
	r := labor.Calculate(in, labor.DefaultPolicy())

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "Inf") {
		t.Error("Result JSON must never contain a non-finite value from a bad payroll record")
	}
	for _, p := range r.Periods {
		for _, id := range p.Provenance.PayrollRecordIDs {
			if id == "INF-1" {
				t.Error("a non-finite-amount payroll record must be excluded from provenance")
			}
		}
	}
}

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
