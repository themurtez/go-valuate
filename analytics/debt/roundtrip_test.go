package debt

import (
	"encoding/json"
	"testing"
)

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (schedules, coverage, capacity, scenarios, flags, issues) — mirroring
// analytics/cashflow/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	in := Input{
		EBITDA:             AvailableValue(500_000),
		CashAndEquivalents: AvailableValue(150_000),
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
			MinimumDSCR:         1.25,
			MaximumDebtToEBITDA: 4.0,
		},
		DownsideScenarios: []DownsideScenario{
			{Label: "Revenue -10%", EBITDAHaircutPercent: 0.10},
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
