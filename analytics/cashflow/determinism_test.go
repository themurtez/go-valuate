package cashflow

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// analytics/workingcapital/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_saas_multi_year.json")
	meta := threeYearMeta()

	in := Input{
		Dataset:    ds,
		PeriodMeta: meta,
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2024": Reported(500_000),
			"2025": Reported(550_000),
		},
		Capex: map[financial.Period]CashFlowValue{
			"2025": Reported(60_000),
		},
		DebtService: map[financial.Period]DebtServiceFigure{
			"2025": {Principal: Reported(80_000), Interest: Reported(15_000)},
		},
		OwnerDistributions: map[financial.Period]CashFlowValue{
			"2025": Reported(150_000),
		},
		CashBalance: map[financial.Period]CashFlowValue{
			"2025": Reported(400_000),
		},
	}
	opts := Options{AllowEBITDAEstimate: true}

	first, err := json.Marshal(Calculate(in, opts))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in, opts))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}

// TestCalculate_DeterministicAcrossMapOrdering proves Calculate's output
// does not depend on Go's randomized map iteration order, exercising every
// internal map (nwcByPeriod, RecurringDrains accumulation, per-period input
// maps) across many repeated runs against the same input maps.
func TestCalculate_DeterministicAcrossMapOrdering(t *testing.T) {
	ds := newDataset().
		incomeStatementPeriod("2023", 1_000_000, 400_000, 200_000, 20_000).
		incomeStatementPeriod("2024", 1_100_000, 440_000, 220_000, 20_000).
		incomeStatementPeriod("2025", 1_200_000, 480_000, 240_000, 20_000).
		balanceSheetPeriod("2023", 100_000, 50_000, 10_000, 80_000, 20_000).
		balanceSheetPeriod("2024", 105_000, 52_000, 10_500, 81_000, 20_500).
		balanceSheetPeriod("2025", 110_000, 54_000, 11_000, 82_000, 21_000).
		build()

	in := Input{
		Dataset:    ds,
		PeriodMeta: threeYearMeta(),
		OperatingCashFlow: map[financial.Period]CashFlowValue{
			"2023": Reported(300_000),
			"2024": Reported(320_000),
		},
		Capex: map[financial.Period]CashFlowValue{
			"2024": Reported(50_000),
			"2025": Reported(60_000),
		},
		DebtService: map[financial.Period]DebtServiceFigure{
			"2024": {Principal: Reported(40_000), Interest: Reported(8_000)},
			"2025": {Principal: Reported(42_000), Interest: Reported(7_000)},
		},
		OwnerDistributions: map[financial.Period]CashFlowValue{
			"2024": Reported(50_000),
			"2025": Reported(60_000),
		},
		CashTaxesPaid: map[financial.Period]CashFlowValue{
			"2024": Reported(30_000),
		},
		CashBalance: map[financial.Period]CashFlowValue{
			"2024": Reported(250_000),
			"2025": Reported(280_000),
		},
	}
	opts := Options{AllowEBITDAEstimate: true}

	first, err := json.Marshal(Calculate(in, opts))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 20; i++ {
		got, err := json.Marshal(Calculate(in, opts))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: output differs; suspect map-order nondeterminism", i)
		}
	}
}
