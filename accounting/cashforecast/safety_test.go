package cashforecast

import "testing"

func TestSafety_NonFiniteAmountExcluded(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "nan", Date: testDate(t, "2025-01-08"), Amount: nan(), Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
			{ID: "inf", Date: testDate(t, "2025-01-08"), Amount: posInf(), Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
			{ID: "good", Date: testDate(t, "2025-01-08"), Amount: 500, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
	}
	if result.BaseScenario.Weekly[0].Inflows.Total != 500 {
		t.Errorf("week 1 inflow = %v, want 500 (NaN/Inf events excluded)", result.BaseScenario.Weekly[0].Inflows.Total)
	}
	if !hasIssueCode(result.Issues, IssueNonFiniteAmount) {
		t.Errorf("expected IssueNonFiniteAmount, got %+v", result.Issues)
	}
}

func nan() float64    { var z float64; return z / z }
func posInf() float64 { var z float64; return 1 / z }

func TestSafety_NegativeAmountRejected(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "bad", Date: testDate(t, "2025-01-08"), Amount: -100, Direction: DirectionOutflow, Category: CategoryAPPayment, Basis: BasisKnown},
		},
	}
	result := Calculate(in, Options{})

	if !hasIssueCode(result.Issues, IssueNegativeAmount) {
		t.Errorf("expected IssueNegativeAmount, got %+v", result.Issues)
	}
	if result.BaseScenario.Weekly[0].Outflows.Total != 0 {
		t.Errorf("week 1 outflow = %v, want 0 (negative-amount event excluded)", result.BaseScenario.Weekly[0].Outflows.Total)
	}
}

func TestSafety_CategoryDirectionMismatchRejected(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Events: []CashFlowEvent{
			// AP_PAYMENT is an outflow category; supplying it with
			// Direction=INFLOW must be rejected, not silently accepted.
			{ID: "mismatch", Date: testDate(t, "2025-01-08"), Amount: 100, Direction: DirectionInflow, Category: CategoryAPPayment, Basis: BasisKnown},
		},
	}
	result := Calculate(in, Options{})

	if !hasIssueCode(result.Issues, IssueCategoryDirectionMismatch) {
		t.Errorf("expected IssueCategoryDirectionMismatch, got %+v", result.Issues)
	}
	if result.BaseScenario.Weekly[0].Inflows.Total != 0 {
		t.Errorf("week 1 inflow = %v, want 0 (mismatched event excluded)", result.BaseScenario.Weekly[0].Inflows.Total)
	}
}

func TestSafety_MixedCurrencyFlagged(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		CashAccounts: []CashAccount{
			{AccountID: "a1", Balance: 500, Currency: "USD"},
			{AccountID: "a2", Balance: 500, Currency: "EUR"},
		},
	}
	result := Calculate(in, Options{ReportingCurrency: "USD"})

	if !hasIssueCode(result.Issues, IssueMixedCurrency) {
		t.Errorf("expected IssueMixedCurrency, got %+v", result.Issues)
	}
}

func TestSafety_DuplicateEventIDOnlyFirstUsed(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Events: []CashFlowEvent{
			{ID: "dup", Date: testDate(t, "2025-01-08"), Amount: 100, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
			{ID: "dup", Date: testDate(t, "2025-01-09"), Amount: 999, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
		},
	}
	result := Calculate(in, Options{})

	if !hasIssueCode(result.Issues, IssueDuplicateEvent) {
		t.Errorf("expected IssueDuplicateEvent, got %+v", result.Issues)
	}
	if result.BaseScenario.Weekly[0].Inflows.Total != 100 {
		t.Errorf("week 1 inflow = %v, want 100 (only first occurrence used)", result.BaseScenario.Weekly[0].Inflows.Total)
	}
}

func TestSafety_InvalidHorizonRejected(t *testing.T) {
	in := Input{ForecastStartDate: testDate(t, "2025-01-06"), OpeningCash: OpeningCash{Amount: 1000, Currency: "USD"}}
	result := Calculate(in, Options{HorizonWeeks: 100})

	if result.Available {
		t.Fatalf("expected Available=false for HorizonWeeks exceeding MaxHorizonWeeks")
	}
	if !hasIssueCode(result.Issues, IssueInvalidHorizon) {
		t.Errorf("expected IssueInvalidHorizon, got %+v", result.Issues)
	}
}

func TestSafety_RestrictedCashExcludedFromAvailableBalance(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		CashAccounts: []CashAccount{
			{AccountID: "op", Balance: 10000, Currency: "USD"},
			{AccountID: "trust", Balance: 90000, Currency: "USD", Restricted: true},
		},
	}
	result := Calculate(in, Options{})

	if !result.Available {
		t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
	}
	if result.OpeningPosition.TotalCash != 100000 {
		t.Errorf("OpeningPosition.TotalCash = %v, want 100000", result.OpeningPosition.TotalCash)
	}
	if result.OpeningPosition.UnrestrictedCash != 10000 {
		t.Errorf("OpeningPosition.UnrestrictedCash = %v, want 10000 (restricted excluded)", result.OpeningPosition.UnrestrictedCash)
	}
	if result.OpeningPosition.RestrictedCash != 90000 {
		t.Errorf("OpeningPosition.RestrictedCash = %v, want 90000", result.OpeningPosition.RestrictedCash)
	}
	// The weekly rollforward must use UnrestrictedCash, not TotalCash.
	if result.BaseScenario.Weekly[0].OpeningCash != 10000 {
		t.Errorf("week 1 OpeningCash = %v, want 10000 (must use unrestricted cash only)", result.BaseScenario.Weekly[0].OpeningCash)
	}
}
