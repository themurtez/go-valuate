package acquisition

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/analytics/debt"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (multiples, consensus comparison, sources and uses, debt schedules,
// coverage, returns, scenarios, flags, issues) — mirroring
// analytics/debt/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	in := Input{
		Target: TargetFinancials{
			Revenue:          AvailableValue(3_000_000),
			NormalizedEBITDA: AvailableValue(550_000),
			NormalizedSDE:    AvailableValue(600_000),
		},
		Consensus:   ConsensusValuation{Value: AvailableValue(2_200_000), Basis: "enterprise_value"},
		AskingPrice: AvailableValue(2_400_000),
		Fees: TransactionFees{
			LegalAndAdvisory: AvailableValue(40_000),
			DueDiligence:     AvailableValue(20_000),
		},
		Financing: Financing{
			DebtTranches: []debt.LoanTerms{
				{Label: "Bank term loan", Principal: 1_600_000, AnnualInterestRate: 0.08, AmortizationYears: 10, Frequency: debt.FrequencyMonthly},
				{Label: "Seller note", Principal: 250_000, AnnualInterestRate: 0.05, AmortizationYears: 5, Frequency: debt.FrequencyAnnual},
			},
			BuyerCashContribution: AvailableValue(550_000),
		},
		WorkingCapital: WorkingCapitalRequirement{Amount: AvailableValue(40_000)},
		Capex:          CapexAssumption{AnnualAmount: AvailableValue(25_000)},
		BuyerCompensation: BuyerCompensationAssumption{
			AnnualAmount: AvailableValue(75_000),
		},
		Scenarios: []ScenarioAdjustment{
			{Label: "Downside", EBITDAHaircutPercent: 0.20, SDEHaircutPercent: 0.20, RevenueHaircutPercent: 0.10},
		},
		RedFlags: RedFlagThresholds{
			MinimumDSCR:          1.25,
			MaximumPriceToEBITDA: 4.5,
			MaximumDebtToEBITDA:  3.5,
		},
	}
	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
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
}

// TestResult_JSONRoundTrip_Empty covers the degenerate zero-Input case,
// ensuring an unavailable Result still round-trips cleanly.
func TestResult_JSONRoundTrip_Empty(t *testing.T) {
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
		t.Fatalf("round-trip mismatch:\nfirst:  %s\nsecond: %s", first, second)
	}
}
