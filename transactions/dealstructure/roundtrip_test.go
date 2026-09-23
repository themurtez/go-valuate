package dealstructure

import (
	"encoding/json"
	"testing"
)

// buildFullInput returns a fully-populated Input exercising every field:
// multiple debt tranches, a seller note, an earnout, fees, working
// capital, closing adjustments, an interest-only tranche, and a balloon
// tranche.
func buildFullInput() Input {
	return Input{
		PurchasePrice: AvailableValue(5_000_000),
		BuyerEquity:   AvailableValue(1_200_000),
		DebtTranches: []DebtTranche{
			{Label: "Senior term loan", Amount: 2_500_000, AnnualInterestRate: 0.08, AmortizationYears: 10, TermYears: 10, Frequency: FrequencyMonthly},
			{Label: "Bridge loan", Amount: 500_000, AnnualInterestRate: 0.10, AmortizationYears: 7, TermYears: 3, Frequency: FrequencyQuarterly, InterestOnlyYears: 1},
			{Label: "Equipment note", Amount: 300_000, AnnualInterestRate: 0.065, AmortizationYears: 15, TermYears: 8, Frequency: FrequencyAnnual, BalloonAmount: 150_000},
		},
		SellerNote: SellerNote{
			Included: true,
			Terms:    DebtTranche{Amount: 400_000, AnnualInterestRate: 0.05, AmortizationYears: 5, TermYears: 5, Frequency: FrequencyAnnual},
		},
		Earnout: Earnout{
			Included: true,
			Payments: []EarnoutPayment{
				{PeriodNumber: 1, Amount: 100_000, Label: "Year 1 milestone"},
				{PeriodNumber: 2, Amount: 100_000, Label: "Year 2 milestone"},
			},
		},
		Fees: TransactionFees{
			LegalAndAdvisory: AvailableValue(60_000),
			DueDiligence:     AvailableValue(35_000),
			FinancingFees:    AvailableValue(20_000),
			Other:            AvailableValue(5_000),
		},
		WorkingCapital: WorkingCapitalContribution{Amount: AvailableValue(80_000)},
		ClosingAdjustments: ClosingAdjustments{
			CashAcquired: AvailableValue(50_000),
			AssumedDebt:  AvailableValue(25_000),
		},
	}
}

// TestResult_JSONRoundTrip proves Result marshals, unmarshals, and
// re-marshals to byte-identical output, exercising every nested type
// (sources and uses, financing percentages, debt schedules, seller note
// schedule, annual debt service, earnout schedule, issues) — mirroring
// acquisition/roundtrip_test.go.
func TestResult_JSONRoundTrip(t *testing.T) {
	res := Build(buildFullInput())
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
	res := Build(Input{})

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

// TestInput_JSONRoundTrip proves Input itself (every nested type a
// caller would construct and might persist/replay) round-trips cleanly.
func TestInput_JSONRoundTrip(t *testing.T) {
	in := buildFullInput()

	first, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped Input
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

	// The re-built Result from the round-tripped Input must match the
	// original Result exactly, proving no field was lost in transit.
	wantResult, err := json.Marshal(Build(in))
	if err != nil {
		t.Fatalf("json.Marshal(Build(in)) failed: %v", err)
	}
	gotResult, err := json.Marshal(Build(roundTripped))
	if err != nil {
		t.Fatalf("json.Marshal(Build(roundTripped)) failed: %v", err)
	}
	if string(wantResult) != string(gotResult) {
		t.Fatalf("Build(roundTripped Input) differs from Build(original Input):\nwant: %s\ngot:  %s", wantResult, gotResult)
	}
}
