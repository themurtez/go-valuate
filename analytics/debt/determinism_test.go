package debt

import (
	"encoding/json"
	"testing"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// analytics/cashflow/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	in := Input{
		EBITDA:             AvailableValue(500_000),
		CashFlow:           AvailableValue(420_000),
		CashAndEquivalents: AvailableValue(150_000),
		InterestExpense:    AvailableValue(35_000),
		ExistingDebt: []LoanTerms{
			{Label: "Equipment note", Principal: 250_000, AnnualInterestRate: 0.055, AmortizationYears: 7, Frequency: FrequencyMonthly},
		},
		ProposedLoans: []LoanTerms{
			{Label: "Acquisition term loan", Principal: 1_200_000, AnnualInterestRate: 0.08, AmortizationYears: 10, Frequency: FrequencyMonthly, InterestOnlyYears: 1},
		},
		FixedCharges: FixedChargeInputs{
			LeasePayments: AvailableValue(60_000),
			CashTaxes:     AvailableValue(40_000),
		},
		Policy: LenderPolicy{
			MinimumDSCR:                1.25,
			MaximumDebtToEBITDA:        4.0,
			MaximumNetDebtToEBITDA:     3.5,
			MinimumFixedChargeCoverage: 1.1,
		},
		DownsideScenarios: []DownsideScenario{
			{Label: "Revenue -10%", EBITDAHaircutPercent: 0.10, CashFlowHaircutPercent: 0.12},
			{Label: "Severe recession", EBITDAHaircutPercent: 0.35, CashFlowHaircutPercent: 0.40},
		},
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
