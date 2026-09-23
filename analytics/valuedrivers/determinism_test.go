package valuedrivers

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// analytics/benchmarks/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{
		{ID: "REV_GROWTH", Type: DriverRevenueGrowth, RevenueGrowth: &RevenueGrowthParams{GrowthPercent: 0.08}},
		{ID: "MARGIN", Type: DriverMarginChange, MarginChange: &MarginChangeParams{MarginPointsDelta: -0.015, RevenueBase: 8_000_000}},
		{ID: "MULTIPLE", Type: DriverMultipleChange, MultipleChange: &MultipleChangeParams{Methods: []MultipleChangeMethod{MultipleChangeSDE, MultipleChangeEBITDA}, ChangeDelta: 0.25}},
		{ID: "DEBT", Type: DriverDebtChange, DebtChange: &DebtChangeParams{ExcessCashDelta: 50_000, OtherDebtDelta: 25_000}},
		{ID: "BAD", Type: "UNKNOWN_TYPE"},
	}
	in.Scenarios = []Scenario{
		{ID: "COMBINED", Label: "Combined downside", Drivers: []Driver{
			{ID: "GROW", Type: DriverRevenueGrowth, RevenueGrowth: &RevenueGrowthParams{GrowthPercent: -0.05}},
			{ID: "LOSE_CUSTOMER", Type: DriverCustomerLossImpact, CustomerLossImpact: &CustomerLossImpactParams{
				RevenueAtRiskAmount: 500_000, EarningsMarginOnLostRevenue: 0.35,
			}},
		}},
		{ID: "EMPTY_SCENARIO"},
	}

	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}

// TestCalculate_Deterministic_WeightsMapOrder proves that Go's randomized
// map iteration order over Input.Weights never leaks into output — every
// consensus.Calculate call must consume Weights via a stable, order-
// independent lookup (keyed access, not iteration), mirroring the map-
// order determinism bugs caught in analytics/revenuequality (HHI) and
// analytics/forecast (COGS sum) per this package's own project memory.
func TestCalculate_Deterministic_WeightsMapOrder(t *testing.T) {
	in := baseInput()
	in.Weights = map[valuation.Code]float64{
		valuation.CodeDCF:                      0.4,
		valuation.CodeCapitalizationOfEarnings: 0.1,
		valuation.CodeEBITDAMultiple:           0.3,
		valuation.CodeSDEMultiple:              0.2,
	}
	in.Drivers = []Driver{{ID: "GROW", Type: DriverRevenueGrowth, RevenueGrowth: &RevenueGrowthParams{GrowthPercent: 0.05}}}

	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 20; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs — possible map-iteration-order nondeterminism", i)
		}
	}
}
