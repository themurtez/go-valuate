package cashforecast_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/cashforecast"
	cffixtures "github.com/themurtez/go-valuate/accounting/cashforecast/fixtures"
)

// TestFixtures_AllProduceAvailableResults exercises every fixture through
// Calculate, proving each one is valid input that produces
// Result.Available == true (or, where a fixture is deliberately invalid,
// is documented as such). Mirrors accounting/ar's and accounting/ap's
// identical fixtures_test.go convention.
func TestFixtures_AllProduceAvailableResults(t *testing.T) {
	cases := []struct {
		name string
		fn   func() (cashforecast.Input, cashforecast.Options)
	}{
		{"HealthyForecast", cffixtures.HealthyForecast},
		{"NegativeCashInWeek8", cffixtures.NegativeCashInWeek8},
		{"BelowMinimumButPositive", cffixtures.BelowMinimumButPositive},
		{"LargeWeek1Payroll", cffixtures.LargeWeek1Payroll},
		{"ARHeavyReceipts", cffixtures.ARHeavyReceipts},
		{"DelayedARDownside", cffixtures.DelayedARDownside},
		{"APHeavyFirstMonth", cffixtures.APHeavyFirstMonth},
		{"VendorPaymentDelayScenario", cffixtures.VendorPaymentDelayScenario},
		{"RecurringExpenseMix", cffixtures.RecurringExpenseMix},
		{"OwnerContributionAndDistribution", cffixtures.OwnerContributionAndDistribution},
		{"DiscretionaryCapexDeferral", cffixtures.DiscretionaryCapexDeferral},
		{"UnscheduledARAndAP", cffixtures.UnscheduledARAndAP},
		{"StaleARAndAPSources", cffixtures.StaleARAndAPSources},
		{"CreditFacilityAvailable", cffixtures.CreditFacilityAvailable},
		{"CreditFacilityInsufficient", cffixtures.CreditFacilityInsufficient},
		{"RestrictedCashPortfolio", cffixtures.RestrictedCashPortfolio},
		{"ZeroOpeningCash", cffixtures.ZeroOpeningCash},
		{"ExactHorizonBoundaryEvent", cffixtures.ExactHorizonBoundaryEvent},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in, opts := c.fn()
			result := cashforecast.Calculate(in, opts)
			if !result.Available {
				t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
			}
			if cashforecast.HasErrors(result.Issues) {
				t.Errorf("unexpected error-level issues: %+v", result.Issues)
			}
		})
	}
}

// TestFixtures_MixedCurrencyInvalidIsFlaggedNotSilentlyAccepted proves the
// one deliberately-invalid fixture still produces a usable (not rejected)
// result with the currency problem surfaced as an Issue — matching
// accounting/ap's "issue, not reject" precedent for mixed currency.
func TestFixtures_MixedCurrencyInvalidIsFlaggedNotSilentlyAccepted(t *testing.T) {
	in, opts := cffixtures.MixedCurrencyInvalid()
	result := cashforecast.Calculate(in, opts)

	if !result.Available {
		t.Fatalf("expected Available=true (issue, not reject), got Issues=%+v", result.Issues)
	}
	found := false
	for _, i := range result.Issues {
		if i.Code == cashforecast.IssueMixedCurrency {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueMixedCurrency, got %+v", result.Issues)
	}
}

// TestFixtures_NegativeCashInWeek8ActuallyGoesNegative proves the fixture
// lives up to its name.
func TestFixtures_NegativeCashInWeek8ActuallyGoesNegative(t *testing.T) {
	in, opts := cffixtures.NegativeCashInWeek8()
	result := cashforecast.Calculate(in, opts)

	if !result.BaseScenario.Summary.NegativeCashReached {
		t.Fatalf("expected NegativeCashReached=true")
	}
	if result.BaseScenario.Summary.FirstNegativeCashWeek != 8 {
		t.Errorf("FirstNegativeCashWeek = %d, want 8", result.BaseScenario.Summary.FirstNegativeCashWeek)
	}
}

// TestFixtures_UnscheduledARAndAPBothNonzero proves the fixture actually
// leaves both AR and AP partially unscheduled.
func TestFixtures_UnscheduledARAndAPBothNonzero(t *testing.T) {
	in, opts := cffixtures.UnscheduledARAndAP()
	result := cashforecast.Calculate(in, opts)

	if result.UnscheduledAR.Amount == 0 {
		t.Errorf("expected UnscheduledAR.Amount > 0")
	}
	if result.UnscheduledAP.Amount == 0 {
		t.Errorf("expected UnscheduledAP.Amount > 0")
	}
}

// TestFixtures_CreditFacilityInsufficientFlagged proves that fixture's
// facility capacity really is smaller than the funding gap.
func TestFixtures_CreditFacilityInsufficientFlagged(t *testing.T) {
	in, opts := cffixtures.CreditFacilityInsufficient()
	result := cashforecast.Calculate(in, opts)

	found := false
	for _, f := range result.Flags {
		if f.Code == cashforecast.FlagFacilityInsufficientForGap {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagFacilityInsufficientForGap, got %+v", result.Flags)
	}
}

// TestFixtures_ExactHorizonBoundaryEventSplit proves the on-boundary
// event is included in weekly totals while the past-boundary event is
// excluded and reported via BeyondHorizon.
func TestFixtures_ExactHorizonBoundaryEventSplit(t *testing.T) {
	in, opts := cffixtures.ExactHorizonBoundaryEvent()
	result := cashforecast.Calculate(in, opts)

	lastWeek := result.BaseScenario.Weekly[len(result.BaseScenario.Weekly)-1]
	if lastWeek.Inflows.Total != 1000 {
		t.Errorf("last week inflow = %v, want 1000 (on-boundary event included)", lastWeek.Inflows.Total)
	}
	if result.BeyondHorizon.Count != 1 || result.BeyondHorizon.Amount != 2000 {
		t.Errorf("BeyondHorizon = %+v, want Count=1 Amount=2000", result.BeyondHorizon)
	}
}
