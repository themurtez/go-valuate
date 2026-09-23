package forecast

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// fullFeaturedInput builds an Input exercising every nested type in
// Result: multiple scenarios, revenue code overrides, COGS/opex/D&A/capex/
// working-capital/tax/debt-service assumptions — used by both the
// round-trip and determinism tests, mirroring analytics/variance's
// pattern of a single shared realistic fixture builder.
func fullFeaturedInput() Input {
	ds := newDataset().
		add(financial.CodeRevProduct, "2025", 700_000).
		add(financial.CodeRevService, "2025", 300_000).
		add(financial.CodeCogsMaterial, "2025", 350_000).
		add(financial.CodeCogsFreight, "2025", 50_000).
		add(financial.CodeOpexPayroll, "2025", 250_000).
		add(financial.CodeOpexOwnerComp, "2025", 60_000).
		add(financial.CodeOpexRent, "2025", 40_000).
		add(financial.CodeDepreciation, "2025", 25_000).
		add(financial.CodeAmortization, "2025", 5_000).
		add(financial.CodeInterestExpense, "2025", 12_000).
		add(financial.CodeIncomeTax, "2025", 30_000).
		add(financial.CodeBsAccountsReceivable, "2025", 120_000).
		add(financial.CodeBsInventory, "2025", 60_000).
		add(financial.CodeBsAccountsPayable, "2025", 80_000).
		build()

	assumptions := Assumptions{
		Revenue: []RevenuePeriodAssumption{
			{
				Method:     RevenueMethodGrowthRate,
				GrowthRate: 0.08,
				CodeOverrides: []RevenueCodeAssumption{
					{Code: financial.CodeRevService, Method: RevenueMethodGrowthRate, GrowthRate: 0.15},
				},
			},
			{Method: RevenueMethodGrowthRate, GrowthRate: 0.06},
		},
		COGS: []COGSPeriodAssumption{
			{Method: COGSMethodGrossMarginPercent, GrossMarginPercent: 0.55},
			{Method: COGSMethodGrossMarginPercent, GrossMarginPercent: 0.56},
		},
		Opex: []OpexPeriodAssumption{
			{
				Method:     OpexMethodGrowthRate,
				GrowthRate: 0.04,
				CodeOverrides: []OpexCodeAssumption{
					{Code: financial.CodeOpexRent, Method: OpexMethodFixedAmount, FixedAmount: 42_000},
				},
			},
			{Method: OpexMethodGrowthRate, GrowthRate: 0.04},
		},
		DepreciationAmortization: []DepreciationAmortizationAssumption{
			{Depreciation: AvailableValue(26_000), Amortization: AvailableValue(5_000)},
			{Depreciation: AvailableValue(27_000), Amortization: AvailableValue(5_000)},
		},
		Capex: []CapexAssumption{
			{Capex: AvailableValue(30_000)},
			{Capex: AvailableValue(15_000)},
		},
		WorkingCapital: []WorkingCapitalPeriodAssumption{
			{Method: WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.10},
			{Method: WorkingCapitalMethodPercentOfRevenue, PercentOfRevenue: 0.10},
		},
		Tax: []TaxPeriodAssumption{
			{Method: TaxMethodPercentOfPretaxIncome, TaxRate: 0.25},
			{Method: TaxMethodPercentOfPretaxIncome, TaxRate: 0.25},
		},
		DebtService: []DebtServiceAssumption{
			{Principal: AvailableValue(20_000), InterestRate: AvailableValue(0.06), BeginningBalance: AvailableValue(150_000)},
			{Principal: AvailableValue(20_000), InterestRate: AvailableValue(0.06), BeginningBalance: AvailableValue(130_000)},
		},
	}

	downside := ApplyRevenueShock(assumptions, -0.10, 1, 2)
	downside = ApplyMarginShock(downside, -0.05, 1, 2)

	return Input{
		Dataset:              ds,
		PeriodMeta:           singlePeriodMeta("2025", 2025),
		Horizon:              2,
		ForecastPeriodLabels: []string{"FY2026", "FY2027"},
		Scenarios: []Scenario{
			{Name: "Base", Type: ScenarioTypeBase, Assumptions: assumptions},
			{Name: "Downside", Type: ScenarioTypeDownside, Assumptions: downside},
		},
	}
}

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (BaseFinancials, multiple ScenarioResults, PeriodPL, WorkingCapitalPeriod,
// CashFlowPeriod, TraceStep, Warnings) — mirroring analytics/variance's
// identical round-trip test shape.
func TestResult_JSONRoundTrip(t *testing.T) {
	res := Calculate(fullFeaturedInput())
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.ScenarioResults) != 2 {
		t.Fatalf("expected 2 scenario results to exercise round-trip fully, got %d", len(res.ScenarioResults))
	}

	first, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !json.Valid(first) {
		t.Fatal("expected valid JSON output")
	}

	var roundTripped Result
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Result did not round-trip byte-for-byte through marshal -> unmarshal -> marshal")
	}
}

// TestResult_JSONRoundTrip_Unavailable proves the blocking-error
// Available == false shape also round-trips cleanly.
func TestResult_JSONRoundTrip_Unavailable(t *testing.T) {
	res := Calculate(Input{})
	first, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var roundTripped Result
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("unavailable Result did not round-trip byte-for-byte")
	}
}

// TestAssumptions_JSONRoundTrip verifies Assumptions itself (the type a
// caller most commonly builds, stores, and passes around independently of
// a full Input/Result) round-trips byte-for-byte.
func TestAssumptions_JSONRoundTrip(t *testing.T) {
	a := fullFeaturedInput().Scenarios[0].Assumptions
	first, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var roundTripped Assumptions
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	second, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Assumptions did not round-trip byte-for-byte")
	}
}
