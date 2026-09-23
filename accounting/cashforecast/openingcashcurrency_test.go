package cashforecast

import "testing"

// TestOpeningCashCurrency_MismatchedAccountExcludedFromActualBalance is a
// regression test for a real bug found by code review: validateOpeningCash
// excluded a currency-mismatched CashAccount from its own
// IssueOpeningCashMismatch sum, but resolveOpeningPosition unconditionally
// summed every account's Balance into OpeningPosition regardless of
// Currency — so a EUR-denominated account under a USD ReportingCurrency
// was treated as suspect by validation yet fully (and silently,
// unconverted) included in the actual cash figure every week's balance
// rolls forward from.
func TestOpeningCashCurrency_MismatchedAccountExcludedFromActualBalance(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		CashAccounts: []CashAccount{
			{AccountID: "usd-acct", Balance: 10000, Currency: "USD"},
			{AccountID: "eur-acct", Balance: 5000, Currency: "EUR"}, // mismatched currency.
		},
	}
	result := Calculate(in, Options{ReportingCurrency: "USD"})

	if !result.Available {
		t.Fatalf("expected Available=true, got %+v", result.Issues)
	}
	if !hasIssueCode(result.Issues, IssueMixedCurrency) {
		t.Errorf("expected IssueMixedCurrency, got %+v", result.Issues)
	}
	// The EUR account must be excluded from the actual opening position —
	// only the USD account's 10000 should count.
	if result.OpeningPosition.TotalCash != 10000 {
		t.Errorf("OpeningPosition.TotalCash = %v, want 10000 (EUR account excluded)", result.OpeningPosition.TotalCash)
	}
	if result.OpeningPosition.UnrestrictedCash != 10000 {
		t.Errorf("OpeningPosition.UnrestrictedCash = %v, want 10000 (EUR account excluded)", result.OpeningPosition.UnrestrictedCash)
	}
	if result.BaseScenario.Weekly[0].OpeningCash != 10000 {
		t.Errorf("week 1 OpeningCash = %v, want 10000 (rollforward must use the currency-consistent total)", result.BaseScenario.Weekly[0].OpeningCash)
	}
}

// TestOpeningCashCurrency_NoReportingCurrencySpecifiedIncludesEverything
// proves that without an explicit ReportingCurrency, every account still
// counts (there's nothing to compare against, so nothing is excluded) —
// this only kicks in once a reporting currency is actually set.
func TestOpeningCashCurrency_NoReportingCurrencySpecifiedIncludesEverything(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		CashAccounts: []CashAccount{
			{AccountID: "a1", Balance: 10000, Currency: "USD"},
			{AccountID: "a2", Balance: 5000, Currency: "EUR"},
		},
	}
	result := Calculate(in, Options{}) // no ReportingCurrency.

	if result.OpeningPosition.TotalCash != 15000 {
		t.Errorf("OpeningPosition.TotalCash = %v, want 15000 (no reporting currency to compare against)", result.OpeningPosition.TotalCash)
	}
}
