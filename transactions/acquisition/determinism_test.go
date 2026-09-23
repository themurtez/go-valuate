package acquisition

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/analytics/debt"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// analytics/debt/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	in := Input{
		Target: TargetFinancials{
			Revenue:          AvailableValue(3_500_000),
			NormalizedEBITDA: AvailableValue(600_000),
			NormalizedSDE:    AvailableValue(650_000),
		},
		Consensus:   ConsensusValuation{Value: AvailableValue(2_400_000), Basis: "enterprise_value"},
		AskingPrice: AvailableValue(2_600_000),
		Fees: TransactionFees{
			LegalAndAdvisory: AvailableValue(45_000),
			DueDiligence:     AvailableValue(25_000),
			FinancingFees:    AvailableValue(15_000),
		},
		Financing: Financing{
			DebtTranches: []debt.LoanTerms{
				{Label: "Bank term loan", Principal: 1_800_000, AnnualInterestRate: 0.085, AmortizationYears: 10, Frequency: debt.FrequencyMonthly},
				{Label: "Seller note", Principal: 300_000, AnnualInterestRate: 0.05, AmortizationYears: 5, Frequency: debt.FrequencyAnnual, InterestOnlyYears: 1},
			},
			BuyerCashContribution: AvailableValue(600_000),
		},
		WorkingCapital: WorkingCapitalRequirement{Amount: AvailableValue(50_000)},
		Capex:          CapexAssumption{AnnualAmount: AvailableValue(30_000)},
		BuyerCompensation: BuyerCompensationAssumption{
			AnnualAmount: AvailableValue(90_000),
		},
		Scenarios: []ScenarioAdjustment{
			{Label: "Revenue -15%", EBITDAHaircutPercent: 0.20, SDEHaircutPercent: 0.20, RevenueHaircutPercent: 0.15},
			{Label: "Upside case", EBITDAHaircutPercent: -0.10, SDEHaircutPercent: -0.10, RevenueHaircutPercent: -0.08},
		},
		RedFlags: RedFlagThresholds{
			MinimumDSCR:                      1.25,
			MaximumPriceToEBITDA:             4.5,
			MaximumPriceToSDE:                4.0,
			MaximumPremiumToConsensusPercent: 0.10,
			MinimumCashOnCashReturn:          0.15,
			MaximumPaybackYears:              6,
			MaximumDebtToEBITDA:              3.5,
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

// TestCalculate_NoMutationOfInput proves Calculate never mutates
// caller-owned slices (DebtTranches, Scenarios).
func TestCalculate_NoMutationOfInput(t *testing.T) {
	tranches := []debt.LoanTerms{
		{Label: "Bank term loan", Principal: 1_000_000, AnnualInterestRate: 0.08, AmortizationYears: 10, Frequency: debt.FrequencyMonthly},
	}
	scenarios := []ScenarioAdjustment{
		{Label: "Downside", EBITDAHaircutPercent: 0.25},
	}
	in := Input{
		Target: TargetFinancials{
			NormalizedEBITDA: AvailableValue(400_000),
		},
		AskingPrice: AvailableValue(1_500_000),
		Financing:   Financing{DebtTranches: tranches},
		Scenarios:   scenarios,
	}

	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal(in) failed: %v", err)
	}

	_ = Calculate(in)
	_ = Calculate(in)

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal(in) failed: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Calculate mutated caller-owned Input:\nbefore: %s\nafter:  %s", before, after)
	}
}
