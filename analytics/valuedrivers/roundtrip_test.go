package valuedrivers

import (
	"encoding/json"
	"testing"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (baseline, one-factor-at-a-time, combined scenarios, method deltas,
// linkages, changed inputs, assumptions, issues) — mirroring
// analytics/benchmarks/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{
		{ID: "REV_GROWTH", Label: "8% revenue growth", Type: DriverRevenueGrowth, RevenueGrowth: &RevenueGrowthParams{GrowthPercent: 0.08}},
		{ID: "SDE_CHANGE", Type: DriverSDEChange, SDEChange: &SDEChangeParams{AmountDelta: 25_000}},
		{ID: "MULTIPLE", Type: DriverMultipleChange, MultipleChange: &MultipleChangeParams{Methods: []MultipleChangeMethod{MultipleChangeEBITDA}, NewValue: 4.5}},
		{ID: "CAP_RATE", Type: DriverCapRateChange, CapRateChange: &CapRateChangeParams{ChangeDelta: 0.01}},
		{ID: "DISCOUNT_RATE", Type: DriverDiscountRateChange, DiscountRateChange: &DiscountRateChangeParams{TerminalGrowthRateDelta: -0.01}},
		{ID: "DEBT", Type: DriverDebtChange, DebtChange: &DebtChangeParams{ShortTermDebtDelta: 10_000}},
		{ID: "NWC", Type: DriverWorkingCapitalChange, WorkingCapitalChange: &WorkingCapitalChangeParams{Field: BridgeFieldOtherDebt, Amount: 15_000}},
		{ID: "OWNER_COMP", Type: DriverOwnerCompensationAdjustment, OwnerCompensationAdjustment: &OwnerCompensationAdjustmentParams{Amount: 40_000, AppliesBeyondSDE: true}},
		{ID: "CUSTOMER_LOSS", Type: DriverCustomerLossImpact, CustomerLossImpact: &CustomerLossImpactParams{RevenueBase: 5_000_000, RevenueAtRiskPercent: 0.10, EarningsMarginOnLostRevenue: 0.25}},
		{ID: "RULE", Type: DriverMethodMultipleRule, MethodMultipleRule: &MethodMultipleRuleParams{Method: MultipleChangeSDE, NewMultiple: 3.2, TriggerLabel: "Recurring revenue %", TriggerValue: "65%"}},
		{ID: "MISSING_PARAMS", Type: DriverMarginChange},
	}
	in.Scenarios = []Scenario{
		{ID: "COMBINED", Label: "Combined scenario", Drivers: []Driver{
			{ID: "G1", Type: DriverRevenueGrowth, RevenueGrowth: &RevenueGrowthParams{GrowthPercent: 0.10}},
			{ID: "M1", Type: DriverMultipleChange, MultipleChange: &MultipleChangeParams{Methods: []MultipleChangeMethod{MultipleChangeSDE}, ChangeDelta: 0.3}},
		}},
		{ID: "EMPTY"},
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, got %+v", res)
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
		t.Fatalf("round-trip mismatch:\nfirst:  %s\nsecond: %s", first, second)
	}

	if len(roundTripped.OneFactorAtATime) != len(res.OneFactorAtATime) {
		t.Fatalf("expected %d one-factor-at-a-time entries after round-trip, got %d", len(res.OneFactorAtATime), len(roundTripped.OneFactorAtATime))
	}
	if len(roundTripped.Scenarios) != len(res.Scenarios) {
		t.Fatalf("expected %d scenario entries after round-trip, got %d", len(res.Scenarios), len(roundTripped.Scenarios))
	}
}

// TestInput_JSONRoundTrip proves Input itself (every Driver params type,
// selected by DriverType) round-trips cleanly — a caller building Input
// from persisted JSON (e.g. a saved scenario library) must recover the
// exact same typed Driver.
func TestInput_JSONRoundTrip(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{
		{ID: "A", Type: DriverRevenueGrowth, RevenueGrowth: &RevenueGrowthParams{GrowthPercent: 0.1}},
		{ID: "B", Type: DriverMethodMultipleRule, MethodMultipleRule: &MethodMultipleRuleParams{Method: MultipleChangeEBITDA, NewMultiple: 5, TriggerLabel: "x", TriggerValue: "y"}},
	}

	first, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped Input
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if len(roundTripped.Drivers) != 2 {
		t.Fatalf("expected 2 drivers after round-trip, got %d", len(roundTripped.Drivers))
	}
	if roundTripped.Drivers[0].RevenueGrowth == nil || roundTripped.Drivers[0].RevenueGrowth.GrowthPercent != 0.1 {
		t.Fatalf("expected RevenueGrowth params to survive round-trip, got %+v", roundTripped.Drivers[0])
	}
	if roundTripped.Drivers[1].MethodMultipleRule == nil || roundTripped.Drivers[1].MethodMultipleRule.NewMultiple != 5 {
		t.Fatalf("expected MethodMultipleRule params to survive round-trip, got %+v", roundTripped.Drivers[1])
	}

	res1 := Calculate(in)
	res2 := Calculate(roundTripped)
	b1, _ := json.Marshal(res1)
	b2, _ := json.Marshal(res2)
	if string(b1) != string(b2) {
		t.Fatal("Calculate(original) and Calculate(round-tripped) produced different output")
	}
}
