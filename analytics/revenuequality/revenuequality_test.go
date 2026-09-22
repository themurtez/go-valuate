package revenuequality

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestCalculate_NoPeriods proves the empty-dataset guard returns
// Available == false with IssueNoPeriods, mirroring
// workingcapital's identical guard test.
func TestCalculate_NoPeriods(t *testing.T) {
	res := Calculate(Input{}, Options{})
	if res.Available {
		t.Fatal("expected Available == false for an empty dataset")
	}
	if !HasErrors(res.Errors) {
		t.Fatal("expected at least one error Issue")
	}
	if res.Errors[0].Code != IssueNoPeriods {
		t.Fatalf("expected IssueNoPeriods, got %s", res.Errors[0].Code)
	}
	if res.FormulaVersion != FormulaVersion {
		t.Fatalf("expected FormulaVersion echoed even when unavailable, got %q", res.FormulaVersion)
	}
}

// TestCalculate_RecurringServiceBusiness covers a business whose revenue is
// dominated by CodeRevRecurring, using the shared SaaS fixture. Verifies
// TotalRevenueHistory's recurring/non-recurring split and a high, stable
// RecurringPercent.
func TestCalculate_RecurringServiceBusiness(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_saas_multi_year.json")
	meta := threeYearMeta()

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.TotalRevenueHistory) != 3 {
		t.Fatalf("expected 3 periods of history, got %d", len(res.TotalRevenueHistory))
	}

	for _, pr := range res.TotalRevenueHistory {
		if !pr.TotalRevenue.Available {
			t.Fatalf("period %s: expected TotalRevenue available", pr.Period)
		}
		if !pr.RecurringRevenue.Available {
			t.Fatalf("period %s: expected RecurringRevenue available (SaaS fixture has REV_RECURRING)", pr.Period)
		}
		if !pr.RecurringPercent.Available {
			t.Fatalf("period %s: expected RecurringPercent available", pr.Period)
		}
		if pr.RecurringPercent.Value <= 0.5 {
			t.Fatalf("period %s: expected RecurringPercent > 50%% for a SaaS-style business, got %.4f", pr.Period, pr.RecurringPercent.Value)
		}
	}

	if res.RevenueTrend.Direction == TrendUnavailable {
		t.Fatal("expected a determinable RevenueTrend with 3 chronologically ordered periods")
	}

	// No FlagDecliningRecurringMix expected for a business whose recurring
	// mix is high and stable/growing across the fixture's history.
	for _, f := range res.Flags {
		if f.Code == FlagDecliningRecurringMix {
			t.Errorf("did not expect FlagDecliningRecurringMix for the SaaS fixture, got %+v", f)
		}
	}
}

// TestCalculate_ProjectBusiness covers a business with zero recurring
// revenue (pure CodeRevProduct), using the shared manufacturer fixture.
// Verifies RecurringRevenue is unavailable throughout (not zero — the code
// is simply absent) while NonRecurringRevenue equals TotalRevenue.
func TestCalculate_ProjectBusiness(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_manufacturer_multi_year.json")
	meta := threeYearMeta()

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	for _, pr := range res.TotalRevenueHistory {
		if pr.RecurringRevenue.Available {
			t.Fatalf("period %s: expected RecurringRevenue unavailable (no REV_RECURRING in manufacturer fixture), got %+v", pr.Period, pr.RecurringRevenue)
		}
		if !pr.NonRecurringRevenue.Available {
			t.Fatalf("period %s: expected NonRecurringRevenue available", pr.Period)
		}
		if !pr.TotalRevenue.Available {
			t.Fatalf("period %s: expected TotalRevenue available", pr.Period)
		}
		if pr.NonRecurringRevenue.Value != pr.TotalRevenue.Value {
			t.Errorf("period %s: expected NonRecurringRevenue == TotalRevenue for a pure-product business, got %.2f vs %.2f", pr.Period, pr.NonRecurringRevenue.Value, pr.TotalRevenue.Value)
		}
		// RecurringPercent requires all three of Recurring/NonRecurring/Total
		// available (see PeriodRevenue.RecurringPercent's doc comment) — with
		// RecurringRevenue unavailable, it must stay unavailable rather than
		// being computed as a false 0%.
		if pr.RecurringPercent.Available {
			t.Errorf("period %s: expected RecurringPercent unavailable when RecurringRevenue itself is unavailable, got %+v", pr.Period, pr.RecurringPercent)
		}
	}
}

// TestCalculate_MissingCustomerDetail proves every customer-level output is
// left at its zero value (Available == false / empty) when
// Input.CustomerRevenue is empty, while period-level revenue analysis still
// proceeds fully — the explicit "optional customer-level dataset" contract.
func TestCalculate_MissingCustomerDetail(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_saas_multi_year.json")
	meta := threeYearMeta()

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.CustomerHistory) != 0 {
		t.Fatalf("expected empty CustomerHistory, got %d entries", len(res.CustomerHistory))
	}
	if len(res.CustomerTransitions) != 0 {
		t.Fatalf("expected empty CustomerTransitions, got %d entries", len(res.CustomerTransitions))
	}
	if res.ConcentrationSummary.CustomerCount != 0 {
		t.Fatalf("expected zero-value ConcentrationSummary, got %+v", res.ConcentrationSummary)
	}

	var found bool
	for _, w := range res.Warnings {
		if w.Code == IssueNoCustomerData {
			found = true
		}
	}
	if !found {
		t.Fatal("expected IssueNoCustomerData warning")
	}

	// Period-level revenue analysis must still be fully populated.
	if len(res.TotalRevenueHistory) != 3 {
		t.Fatalf("expected period-level history to still compute fully, got %d periods", len(res.TotalRevenueHistory))
	}
}

// TestCalculate_CustomerChurn covers a two-period scenario with new, lost,
// retained-flat, expanded, and contracted customers all present in the same
// transition, verifying CustomerTransition's exact arithmetic.
func TestCalculate_CustomerChurn(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2024", 100000).
		add(financial.CodeRevProduct, "2025", 110000).
		build()
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}

	customers := []CustomerPeriodRevenue{
		// Retained flat: $20,000 both periods.
		{CustomerKey: "flat-co", Period: "2024", Amount: 20000},
		{CustomerKey: "flat-co", Period: "2025", Amount: 20000},
		// Expanded: $10,000 -> $25,000 (+$15,000).
		{CustomerKey: "grow-co", Period: "2024", Amount: 10000},
		{CustomerKey: "grow-co", Period: "2025", Amount: 25000},
		// Contracted: $30,000 -> $10,000 (-$20,000).
		{CustomerKey: "shrink-co", Period: "2024", Amount: 30000},
		{CustomerKey: "shrink-co", Period: "2025", Amount: 10000},
		// Lost: $15,000 -> gone.
		{CustomerKey: "churned-co", Period: "2024", Amount: 15000},
		// New: appears in 2025 only, $30,000.
		{CustomerKey: "new-co", Period: "2025", Amount: 30000},
	}

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta, CustomerRevenue: customers}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.CustomerTransitions) != 1 {
		t.Fatalf("expected exactly 1 CustomerTransition, got %d", len(res.CustomerTransitions))
	}

	tr := res.CustomerTransitions[0]
	if tr.FromPeriod != "2024" || tr.ToPeriod != "2025" {
		t.Fatalf("expected transition 2024->2025, got %s->%s", tr.FromPeriod, tr.ToPeriod)
	}

	assertRevenue := func(name string, got RevenueValue, want float64) {
		t.Helper()
		if !got.Available {
			t.Errorf("%s: expected Available", name)
			return
		}
		if got.Value != want {
			t.Errorf("%s: expected %.2f, got %.2f", name, want, got.Value)
		}
	}

	assertRevenue("NewCustomerRevenue", tr.NewCustomerRevenue, 30000)
	if tr.NewCustomerCount != 1 {
		t.Errorf("expected NewCustomerCount 1, got %d", tr.NewCustomerCount)
	}

	assertRevenue("LostCustomerRevenue", tr.LostCustomerRevenue, 15000)
	if tr.LostCustomerCount != 1 {
		t.Errorf("expected LostCustomerCount 1, got %d", tr.LostCustomerCount)
	}

	// RetainedRevenue = min(20000,20000) + min(10000,25000) + min(30000,10000)
	//                 = 20000 + 10000 + 10000 = 40000.
	assertRevenue("RetainedRevenue", tr.RetainedRevenue, 40000)
	if tr.RetainedCustomerCount != 3 {
		t.Errorf("expected RetainedCustomerCount 3, got %d", tr.RetainedCustomerCount)
	}

	assertRevenue("ExpansionRevenue", tr.ExpansionRevenue, 15000)
	if tr.ExpandedCustomerCount != 1 {
		t.Errorf("expected ExpandedCustomerCount 1, got %d", tr.ExpandedCustomerCount)
	}

	assertRevenue("ContractionRevenue", tr.ContractionRevenue, 20000)
	if tr.ContractedCustomerCount != 1 {
		t.Errorf("expected ContractedCustomerCount 1, got %d", tr.ContractedCustomerCount)
	}

	// ExistingCustomerBaseChange = (Retained + Expansion - Contraction) - FromTotal
	//   FromTotal (flat+grow+shrink+churned) = 20000+10000+30000+15000 = 75000
	//   (40000 + 15000 - 20000) - 75000 = 35000 - 75000 = -40000.
	assertRevenue("ExistingCustomerBaseChange", tr.ExistingCustomerBaseChange, -40000)

	// This scenario should trigger FlagShrinkingExistingCustomerBase:
	// |-40000| / 75000 = 53.3%, well above the 10% default threshold.
	var foundShrink bool
	for _, f := range res.Flags {
		if f.Code == FlagShrinkingExistingCustomerBase {
			foundShrink = true
		}
	}
	if !foundShrink {
		t.Error("expected FlagShrinkingExistingCustomerBase to trigger")
	}

	// LostCustomerRevenue / FromTotal = 15000/75000 = 20%, above the 15%
	// default threshold.
	var foundLost bool
	for _, f := range res.Flags {
		if f.Code == FlagHighLostCustomerRevenue {
			foundLost = true
		}
	}
	if !foundLost {
		t.Error("expected FlagHighLostCustomerRevenue to trigger")
	}
}

// TestCalculate_CustomerTransition_FromPeriodTotalWithExpansion is a
// regression test for a bug caught during development: FromPeriodTotalRevenue
// (and every ratio derived from it) must equal the true sum of FromPeriod
// customer revenue, not a reconstruction from RetainedRevenue +
// ContractionRevenue - ExpansionRevenue + LostCustomerRevenue, which
// undercounts by 2x ExpansionRevenue whenever a retained customer expanded
// (RetainedRevenue is already min(from,to) per customer, so
// ExpansionRevenue is additional revenue on top of it, not a component to
// subtract back out). This scenario isolates a single expanding customer
// with no new/lost customers, where the bug's effect is starkest: true
// FromPeriodTotalRevenue is 100, but the buggy reconstruction computed 50.
func TestCalculate_CustomerTransition_FromPeriodTotalWithExpansion(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2024", 100).
		add(financial.CodeRevProduct, "2025", 150).
		build()
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
	customers := []CustomerPeriodRevenue{
		{CustomerKey: "solo-co", Period: "2024", Amount: 100},
		{CustomerKey: "solo-co", Period: "2025", Amount: 150},
	}

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta, CustomerRevenue: customers}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.CustomerTransitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(res.CustomerTransitions))
	}

	tr := res.CustomerTransitions[0]
	if !tr.FromPeriodTotalRevenue.Available || tr.FromPeriodTotalRevenue.Value != 100 {
		t.Fatalf("expected FromPeriodTotalRevenue == 100 (the true FromPeriod customer total), got %+v", tr.FromPeriodTotalRevenue)
	}
	if !tr.ToPeriodTotalRevenue.Available || tr.ToPeriodTotalRevenue.Value != 150 {
		t.Fatalf("expected ToPeriodTotalRevenue == 150, got %+v", tr.ToPeriodTotalRevenue)
	}
	if !tr.TotalRevenueGrowth.Available || tr.TotalRevenueGrowth.Value != 50 {
		t.Fatalf("expected TotalRevenueGrowth == 50, got %+v", tr.TotalRevenueGrowth)
	}
	if tr.ExpansionRevenue.Value != 50 {
		t.Fatalf("expected ExpansionRevenue == 50, got %.2f", tr.ExpansionRevenue.Value)
	}
	if tr.RetainedRevenue.Value != 100 {
		t.Fatalf("expected RetainedRevenue == min(100,150) == 100, got %.2f", tr.RetainedRevenue.Value)
	}
}

// TestCalculate_GrowthThroughNewCustomers covers a scenario where all
// period-over-period growth comes from new customers while the existing
// base is flat, verifying FlagGrowthDependentOnNewCustomers triggers.
func TestCalculate_GrowthThroughNewCustomers(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2024", 50000).
		add(financial.CodeRevProduct, "2025", 100000).
		build()
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}

	customers := []CustomerPeriodRevenue{
		// Existing customer, perfectly flat.
		{CustomerKey: "steady-co", Period: "2024", Amount: 50000},
		{CustomerKey: "steady-co", Period: "2025", Amount: 50000},
		// All growth from a brand-new customer.
		{CustomerKey: "new-co", Period: "2025", Amount: 50000},
	}

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta, CustomerRevenue: customers}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.CustomerTransitions) != 1 {
		t.Fatalf("expected exactly 1 CustomerTransition, got %d", len(res.CustomerTransitions))
	}

	tr := res.CustomerTransitions[0]
	if tr.ExistingCustomerBaseChange.Value != 0 {
		t.Errorf("expected existing base to be perfectly flat (0 change), got %.2f", tr.ExistingCustomerBaseChange.Value)
	}
	if tr.NewCustomerRevenue.Value != 50000 {
		t.Errorf("expected NewCustomerRevenue 50000, got %.2f", tr.NewCustomerRevenue.Value)
	}

	var found bool
	for _, f := range res.Flags {
		if f.Code == FlagGrowthDependentOnNewCustomers {
			found = true
			if f.Value != 1.0 {
				t.Errorf("expected new-customer share of growth == 1.0 (100%%), got %.4f", f.Value)
			}
		}
	}
	if !found {
		t.Error("expected FlagGrowthDependentOnNewCustomers to trigger when 100%% of growth is new-customer revenue")
	}
}

// TestCalculate_VolatileRevenue covers a five-year revenue series that
// swings sharply up and down, verifying RevenueVolatility is available and
// FlagVolatileRevenue triggers.
func TestCalculate_VolatileRevenue(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2021", 100000).
		add(financial.CodeRevProduct, "2022", 180000). // +80%
		add(financial.CodeRevProduct, "2023", 90000).  // -50%
		add(financial.CodeRevProduct, "2024", 170000). // +89%
		add(financial.CodeRevProduct, "2025", 95000).  // -44%
		build()
	meta := fiveYearMeta()

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if !res.RevenueVolatility.Value.Available {
		t.Fatal("expected RevenueVolatility.Value available with 5 chronologically ordered periods")
	}
	if res.RevenueVolatility.SampleSize != 4 {
		t.Fatalf("expected 4 growth observations (5 periods), got %d", res.RevenueVolatility.SampleSize)
	}

	var found bool
	for _, f := range res.Flags {
		if f.Code == FlagVolatileRevenue {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagVolatileRevenue to trigger for a swing this large, got flags=%+v", res.Flags)
	}
}

// TestCalculate_OnePeriodSpike covers a five-year series with one dramatic
// outlier period, verifying FlagOnePeriodSpike identifies exactly that
// period.
func TestCalculate_OnePeriodSpike(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2021", 100000).
		add(financial.CodeRevProduct, "2022", 105000).
		add(financial.CodeRevProduct, "2023", 400000). // spike
		add(financial.CodeRevProduct, "2024", 102000).
		add(financial.CodeRevProduct, "2025", 98000).
		build()
	meta := fiveYearMeta()

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	var spike *Flag
	for i := range res.Flags {
		if res.Flags[i].Code == FlagOnePeriodSpike {
			spike = &res.Flags[i]
		}
	}
	if spike == nil {
		t.Fatalf("expected FlagOnePeriodSpike to trigger, got flags=%+v", res.Flags)
	}
	if spike.Period != "2023" {
		t.Errorf("expected spike period 2023, got %s", spike.Period)
	}
}

// TestCalculate_DecliningRecurringMix covers a business whose recurring
// share of revenue falls meaningfully over time, verifying
// FlagDecliningRecurringMix triggers.
func TestCalculate_DecliningRecurringMix(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevRecurring, "2023", 80000).
		add(financial.CodeRevProduct, "2023", 20000).
		add(financial.CodeRevRecurring, "2024", 60000).
		add(financial.CodeRevProduct, "2024", 60000).
		add(financial.CodeRevRecurring, "2025", 40000).
		add(financial.CodeRevProduct, "2025", 100000).
		build()
	meta := threeYearMeta()

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	first := res.TotalRevenueHistory[0]
	last := res.TotalRevenueHistory[len(res.TotalRevenueHistory)-1]
	if first.RecurringPercent.Value <= last.RecurringPercent.Value {
		t.Fatalf("expected recurring mix to decline: first=%.4f last=%.4f", first.RecurringPercent.Value, last.RecurringPercent.Value)
	}

	var found bool
	for _, f := range res.Flags {
		if f.Code == FlagDecliningRecurringMix {
			found = true
		}
	}
	if !found {
		t.Error("expected FlagDecliningRecurringMix to trigger")
	}
}

// TestCalculate_ConcentrationSummary covers a simple concentration
// scenario: one dominant customer plus several small ones, verifying
// TopNShares and HHI.
func TestCalculate_ConcentrationSummary(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2025", 100000).
		build()
	meta := map[financial.Period]PeriodInfo{
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}

	customers := []CustomerPeriodRevenue{
		{CustomerKey: "big-co", Period: "2025", Amount: 70000, Segment: "enterprise"},
		{CustomerKey: "mid-co", Period: "2025", Amount: 20000, Segment: "smb"},
		{CustomerKey: "small-co", Period: "2025", Amount: 10000, Segment: "smb"},
	}

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta, CustomerRevenue: customers}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	cs := res.ConcentrationSummary
	if cs.Period != "2025" {
		t.Fatalf("expected summary period 2025, got %s", cs.Period)
	}
	if cs.CustomerCount != 3 {
		t.Fatalf("expected 3 customers, got %d", cs.CustomerCount)
	}
	if !cs.TotalCustomerRevenue.Available || cs.TotalCustomerRevenue.Value != 100000 {
		t.Fatalf("expected TotalCustomerRevenue 100000, got %+v", cs.TotalCustomerRevenue)
	}
	if !cs.UnallocatedRevenue.Available || cs.UnallocatedRevenue.Value != 0 {
		t.Fatalf("expected UnallocatedRevenue 0 (customer data fully reconciles), got %+v", cs.UnallocatedRevenue)
	}

	if len(cs.TopNShares) != 3 { // default policy: 1, 5, 10
		t.Fatalf("expected 3 TopNShares (default policy top 1/5/10), got %d", len(cs.TopNShares))
	}
	top1 := cs.TopNShares[0]
	if top1.N != 1 {
		t.Fatalf("expected first TopNShare N=1, got %d", top1.N)
	}
	if !top1.Percent.Available || top1.Percent.Value != 0.7 {
		t.Fatalf("expected top-1 share 70%%, got %+v", top1.Percent)
	}

	// HHI = (0.7^2 + 0.2^2 + 0.1^2) * 10000 = (0.49+0.04+0.01)*10000 = 5400.
	if !cs.HHI.Available {
		t.Fatal("expected HHI available")
	}
	if diff := cs.HHI.Value - 5400; diff > 0.01 || diff < -0.01 {
		t.Fatalf("expected HHI ~5400, got %.2f", cs.HHI.Value)
	}

	if len(cs.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(cs.Segments))
	}
}

// TestCalculate_CustomerRevenueUnreconciled proves UnallocatedRevenue
// surfaces a mismatch between Dataset's total revenue and the sum of
// customer-level rows without treating it as an error.
func TestCalculate_CustomerRevenueUnreconciled(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2025", 100000).
		build()
	meta := map[financial.Period]PeriodInfo{
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}

	// Customer data covers only $60,000 of the $100,000 total.
	customers := []CustomerPeriodRevenue{
		{CustomerKey: "partial-co", Period: "2025", Amount: 60000},
	}

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta, CustomerRevenue: customers}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if !res.ConcentrationSummary.UnallocatedRevenue.Available {
		t.Fatal("expected UnallocatedRevenue available")
	}
	if res.ConcentrationSummary.UnallocatedRevenue.Value != 40000 {
		t.Fatalf("expected UnallocatedRevenue 40000, got %.2f", res.ConcentrationSummary.UnallocatedRevenue.Value)
	}
	if HasErrors(res.Errors) {
		t.Error("a reconciliation mismatch must not be reported as an error")
	}
}

// TestCalculate_CustomerPeriodNotInDataset proves a customer row referencing
// a period absent from Dataset is excluded from customer-level outputs and
// surfaced as an advisory warning rather than failing the whole
// calculation.
func TestCalculate_CustomerPeriodNotInDataset(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevProduct, "2025", 100000).
		build()
	meta := map[financial.Period]PeriodInfo{
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}

	customers := []CustomerPeriodRevenue{
		{CustomerKey: "co-a", Period: "2025", Amount: 100000},
		{CustomerKey: "co-b", Period: "2099", Amount: 5000}, // not in Dataset
	}

	res := Calculate(Input{Dataset: ds, PeriodMeta: meta, CustomerRevenue: customers}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	var found bool
	for _, w := range res.Warnings {
		if w.Code == IssueCustomerPeriodNotInDataset {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueCustomerPeriodNotInDataset warning")
	}
	if res.ConcentrationSummary.CustomerCount != 1 {
		t.Fatalf("expected the invalid row excluded, leaving 1 customer, got %d", res.ConcentrationSummary.CustomerCount)
	}
}

// TestCalculate_RecurringFlagCoverage proves CustomerPeriodTotal correctly
// distinguishes "no row had a RecurringFlag" from "every flagged row was
// non-recurring."
func TestCalculate_RecurringFlagCoverage(t *testing.T) {
	ds := newDataset().add(financial.CodeRevProduct, "2025", 100000).build()
	meta := map[financial.Period]PeriodInfo{"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025}}

	t.Run("no flags set", func(t *testing.T) {
		customers := []CustomerPeriodRevenue{{CustomerKey: "co-a", Period: "2025", Amount: 50000}}
		res := Calculate(Input{Dataset: ds, PeriodMeta: meta, CustomerRevenue: customers}, Options{})
		if len(res.CustomerHistory) != 1 {
			t.Fatalf("expected 1 CustomerHistory entry, got %d", len(res.CustomerHistory))
		}
		if res.CustomerHistory[0].RecurringCustomerRevenue.Available {
			t.Error("expected RecurringCustomerRevenue unavailable when no row set RecurringFlag")
		}
		if res.CustomerHistory[0].RecurringFlagCoverage != 0 {
			t.Errorf("expected RecurringFlagCoverage 0, got %.4f", res.CustomerHistory[0].RecurringFlagCoverage)
		}
	})

	t.Run("all flagged non-recurring", func(t *testing.T) {
		customers := []CustomerPeriodRevenue{{CustomerKey: "co-a", Period: "2025", Amount: 50000, RecurringFlag: boolPtr(false)}}
		res := Calculate(Input{Dataset: ds, PeriodMeta: meta, CustomerRevenue: customers}, Options{})
		if !res.CustomerHistory[0].RecurringCustomerRevenue.Available {
			t.Fatal("expected RecurringCustomerRevenue available (flag was set, even though false)")
		}
		if res.CustomerHistory[0].RecurringCustomerRevenue.Value != 0 {
			t.Errorf("expected RecurringCustomerRevenue 0, got %.2f", res.CustomerHistory[0].RecurringCustomerRevenue.Value)
		}
		if res.CustomerHistory[0].RecurringFlagCoverage != 1.0 {
			t.Errorf("expected full RecurringFlagCoverage, got %.4f", res.CustomerHistory[0].RecurringFlagCoverage)
		}
	})
}

// TestCalculate_NoPeriodMeta proves ordering-dependent outputs are left
// unavailable (with an advisory warning) while per-period history still
// computes in dataset lexical order, mirroring
// workingcapital's identical IssueNoPeriodMeta contract.
func TestCalculate_NoPeriodMeta(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_saas_multi_year.json")

	res := Calculate(Input{Dataset: ds}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.TotalRevenueHistory) != 3 {
		t.Fatalf("expected per-period history still computed, got %d periods", len(res.TotalRevenueHistory))
	}
	if res.RevenueTrend.Direction != TrendUnavailable {
		t.Errorf("expected RevenueTrend unavailable without PeriodMeta, got %s", res.RevenueTrend.Direction)
	}
	if len(res.RevenueGrowth) != 0 {
		t.Errorf("expected RevenueGrowth unavailable without PeriodMeta, got %d points", len(res.RevenueGrowth))
	}

	var found bool
	for _, w := range res.Warnings {
		if w.Code == IssueNoPeriodMeta {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueNoPeriodMeta warning")
	}
}
