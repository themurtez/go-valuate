// Package fixtures provides synthetic accounting/cashforecast data for
// tests and examples: a set of small, hand-built Input/Options pairs
// covering the scenarios accounting/cashforecast's tests exercise.
// Nothing here is real financial data — every figure is invented for
// illustration, mirroring accounting/ar/fixtures and accounting/ap/fixtures'
// identical synthetic-data convention.
package fixtures

import (
	"time"

	"github.com/themurtez/go-valuate/accounting/cashforecast"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// ForecastStartDate is the fixed start date every fixture below is
// measured from.
const ForecastStartDate = "2025-01-06"

// HealthyForecast returns a well-funded 13-week forecast: ample opening
// cash, modest recurring rent/software, steady AR collections, nothing
// below minimum.
func HealthyForecast() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 250000, Currency: "USD", AsOfDate: date("2025-01-05")},
		RecurringRules: []cashforecast.RecurringRule{
			{ID: "rent", Amount: 6000, Direction: cashforecast.DirectionOutflow, Category: cashforecast.CategoryRent,
				Basis: cashforecast.BasisScheduled, StartDate: date("2025-01-01"), Frequency: cashforecast.FrequencyMonthly},
			{ID: "software", Amount: 800, Direction: cashforecast.DirectionOutflow, Category: cashforecast.CategoryOtherOperatingOutflow,
				Basis: cashforecast.BasisScheduled, StartDate: date("2025-01-01"), Frequency: cashforecast.FrequencyMonthly},
		},
		ARSources: []cashforecast.ARReceivableSource{
			{ReceivableID: "r1", OpenAmount: 40000}, {ReceivableID: "r2", OpenAmount: 25000},
		},
		ARCollections: []cashforecast.ARCollectionAssumption{
			{ID: "c1", ReceivableID: "r1", ExpectedReceiptDate: date("2025-01-15"), ExpectedAmount: 40000, Basis: cashforecast.BasisAssumed},
			{ID: "c2", ReceivableID: "r2", ExpectedReceiptDate: date("2025-02-05"), ExpectedAmount: 25000, Basis: cashforecast.BasisAssumed},
		},
	}
	opts := cashforecast.Options{MinimumCash: cashforecast.MinimumCashPolicy{MinimumCashBalance: 30000}}
	return in, opts
}

// NegativeCashInWeek8 returns a forecast whose ending cash goes negative
// in week 8 due to a large scheduled AP payment against modest opening
// cash.
func NegativeCashInWeek8() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 40000, Currency: "USD"},
		APSources: []cashforecast.APPayableSource{
			{PayableID: "b1", OpenAmount: 90000, DueDate: date("2025-02-24")}, // week 8 (Jan 6 + 7*7=49 days -> Feb 24).
		},
		APPlans: []cashforecast.APPaymentPlan{
			{ID: "p1", PayableID: "b1", PaymentDate: date("2025-02-24"), Amount: 90000, Basis: cashforecast.BasisScheduled},
		},
	}
	opts := cashforecast.Options{MinimumCash: cashforecast.MinimumCashPolicy{MinimumCashBalance: 10000}}
	return in, opts
}

// BelowMinimumButPositive returns a forecast that dips below the minimum
// cash threshold but never goes negative.
func BelowMinimumButPositive() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 25000, Currency: "USD"},
		Events: []cashforecast.CashFlowEvent{
			{ID: "outflow1", Date: date("2025-01-10"), Amount: 15000, Direction: cashforecast.DirectionOutflow,
				Category: cashforecast.CategoryAPPayment, Basis: cashforecast.BasisKnown},
		},
	}
	opts := cashforecast.Options{MinimumCash: cashforecast.MinimumCashPolicy{MinimumCashBalance: 20000}}
	return in, opts
}

// LargeWeek1Payroll returns a forecast dominated by a large first-week
// payroll run.
func LargeWeek1Payroll() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 100000, Currency: "USD"},
		Payroll: []cashforecast.PayrollEvent{
			{ID: "pr1", Date: date("2025-01-10"), EmployeeNetCash: 45000, EmployerTaxes: 4500, EmployeeWithholdingsRemitted: 8000, Benefits: 2000},
		},
	}
	return in, cashforecast.Options{}
}

// ARHeavyReceipts returns a forecast whose inflows are dominated by
// scheduled AR collections across several weeks.
func ARHeavyReceipts() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 20000, Currency: "USD"},
		ARSources: []cashforecast.ARReceivableSource{
			{ReceivableID: "r1", OpenAmount: 30000}, {ReceivableID: "r2", OpenAmount: 45000}, {ReceivableID: "r3", OpenAmount: 20000},
		},
		ARCollections: []cashforecast.ARCollectionAssumption{
			{ID: "c1", ReceivableID: "r1", ExpectedReceiptDate: date("2025-01-13"), ExpectedAmount: 30000, Basis: cashforecast.BasisAssumed},
			{ID: "c2", ReceivableID: "r2", ExpectedReceiptDate: date("2025-01-27"), ExpectedAmount: 45000, Basis: cashforecast.BasisAssumed},
			{ID: "c3", ReceivableID: "r3", ExpectedReceiptDate: date("2025-02-10"), ExpectedAmount: 20000, Basis: cashforecast.BasisAssumed},
		},
	}
	return in, cashforecast.Options{}
}

// DelayedARDownside returns a base input plus a DOWNSIDE scenario that
// delays every AR collection by 14 days.
func DelayedARDownside() (cashforecast.Input, cashforecast.Options) {
	in, opts := ARHeavyReceipts()
	in.Scenarios = []cashforecast.Scenario{
		{Label: "DOWNSIDE", Transforms: []cashforecast.EventTransform{
			{Kind: cashforecast.TransformDelayInflows, DelayDays: 14, Category: cashforecast.CategoryARCollection},
		}},
	}
	return in, opts
}

// APHeavyFirstMonth returns a forecast whose outflows are concentrated in
// AP payments during the first four weeks.
func APHeavyFirstMonth() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 80000, Currency: "USD"},
		APSources: []cashforecast.APPayableSource{
			{PayableID: "b1", OpenAmount: 20000, DueDate: date("2025-01-10")},
			{PayableID: "b2", OpenAmount: 15000, DueDate: date("2025-01-18")},
			{PayableID: "b3", OpenAmount: 18000, DueDate: date("2025-01-28")},
		},
		APPlans: []cashforecast.APPaymentPlan{
			{ID: "p1", PayableID: "b1", PaymentDate: date("2025-01-10"), Amount: 20000},
			{ID: "p2", PayableID: "b2", PaymentDate: date("2025-01-18"), Amount: 15000},
			{ID: "p3", PayableID: "b3", PaymentDate: date("2025-01-28"), Amount: 18000},
		},
	}
	return in, cashforecast.Options{}
}

// VendorPaymentDelayScenario returns APHeavyFirstMonth plus a scenario
// that delays every AP payment by 10 days (a caller testing a
// "stretch payables" liquidity lever).
func VendorPaymentDelayScenario() (cashforecast.Input, cashforecast.Options) {
	in, opts := APHeavyFirstMonth()
	in.Scenarios = []cashforecast.Scenario{
		{Label: "STRETCH_PAYABLES", Transforms: []cashforecast.EventTransform{
			{Kind: cashforecast.TransformDelayOutflows, DelayDays: 10, Category: cashforecast.CategoryAPPayment},
		}},
	}
	return in, opts
}

// RecurringExpenseMix returns a forecast combining monthly rent, biweekly
// payroll, and a quarterly tax remittance — the recurring/schedule
// combination the task's fixtures list calls for.
func RecurringExpenseMix() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 150000, Currency: "USD"},
		RecurringRules: []cashforecast.RecurringRule{
			{ID: "rent", Amount: 7000, Direction: cashforecast.DirectionOutflow, Category: cashforecast.CategoryRent,
				Basis: cashforecast.BasisScheduled, StartDate: date("2025-01-01"), Frequency: cashforecast.FrequencyMonthly},
			{ID: "payroll", Amount: 22000, Direction: cashforecast.DirectionOutflow, Category: cashforecast.CategoryPayroll,
				Basis: cashforecast.BasisScheduled, StartDate: date("2025-01-10"), Frequency: cashforecast.FrequencyBiweekly},
		},
		Tax: []cashforecast.TaxEvent{
			{ID: "t1", Date: date("2025-03-15"), Amount: 12000, Category: cashforecast.TaxIncome},
		},
		DebtService: []cashforecast.DebtServiceEvent{
			{ID: "d1", Date: date("2025-01-15"), Amount: 4000, LoanLabel: "Equipment note"},
			{ID: "d2", Date: date("2025-02-15"), Amount: 4000, LoanLabel: "Equipment note"},
			{ID: "d3", Date: date("2025-03-15"), Amount: 4000, LoanLabel: "Equipment note"},
		},
		Capex: []cashforecast.CapexEvent{
			{ID: "cap1", Date: date("2025-02-01"), Amount: 18000, Description: "Warehouse racking", Commitment: cashforecast.CommitmentDiscretionary},
		},
	}
	return in, cashforecast.Options{}
}

// OwnerContributionAndDistribution returns a forecast with an early
// owner contribution followed by a later owner distribution.
func OwnerContributionAndDistribution() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 10000, Currency: "USD"},
		Financing: []cashforecast.FinancingEvent{
			{ID: "f1", Date: date("2025-01-08"), Amount: 50000, Type: cashforecast.FinancingOwnerContribution, Description: "Owner capital injection"},
			{ID: "f2", Date: date("2025-03-01"), Amount: 15000, Type: cashforecast.FinancingOwnerDistribution, Description: "Quarterly distribution"},
		},
	}
	return in, cashforecast.Options{}
}

// DiscretionaryCapexDeferral returns a base input plus a scenario that
// removes a discretionary capex event entirely.
func DiscretionaryCapexDeferral() (cashforecast.Input, cashforecast.Options) {
	in, opts := RecurringExpenseMix()
	in.Scenarios = []cashforecast.Scenario{
		{Label: "DEFER_CAPEX", Transforms: []cashforecast.EventTransform{
			{Kind: cashforecast.TransformRemoveEvent, RemoveEventID: "capex#cap1"},
		}},
	}
	return in, opts
}

// UnscheduledARAndAP returns a forecast where AR/AP sources are supplied
// but only partially scheduled, so UnscheduledAR/UnscheduledAP are both
// nonzero.
func UnscheduledARAndAP() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 50000, Currency: "USD"},
		ARSources: []cashforecast.ARReceivableSource{
			{ReceivableID: "r1", OpenAmount: 100000}, {ReceivableID: "r2", OpenAmount: 20000},
		},
		ARCollections: []cashforecast.ARCollectionAssumption{
			{ID: "c1", ReceivableID: "r1", ExpectedReceiptDate: date("2025-01-15"), ExpectedAmount: 80000},
		},
		APSources: []cashforecast.APPayableSource{
			{PayableID: "b1", OpenAmount: 75000, DueDate: date("2025-01-20")},
		},
		APPlans: []cashforecast.APPaymentPlan{
			{ID: "p1", PayableID: "b1", PaymentDate: date("2025-01-18"), Amount: 60000},
		},
	}
	return in, cashforecast.Options{}
}

// StaleARAndAPSources returns a forecast with old AR/AP snapshot dates
// under a tight staleness policy.
func StaleARAndAPSources() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 50000, Currency: "USD"},
		ARSources:         []cashforecast.ARReceivableSource{{ReceivableID: "r1", OpenAmount: 10000}},
		APSources:         []cashforecast.APPayableSource{{PayableID: "b1", OpenAmount: 5000, DueDate: date("2025-01-20")}},
		ARSnapshotDate:    date("2024-11-01"),
		APSnapshotDate:    date("2024-11-01"),
	}
	opts := cashforecast.Options{Staleness: cashforecast.StalenessPolicy{MaxARAgeDays: 14, MaxAPAgeDays: 14}}
	return in, opts
}

// CreditFacilityAvailable returns a forecast with a funding gap fully
// covered by an available credit facility.
func CreditFacilityAvailable() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 10000, Currency: "USD"},
		Events: []cashforecast.CashFlowEvent{
			{ID: "big", Date: date("2025-01-08"), Amount: 30000, Direction: cashforecast.DirectionOutflow,
				Category: cashforecast.CategoryAPPayment, Basis: cashforecast.BasisKnown},
		},
		Facilities: []cashforecast.CreditFacility{{FacilityID: "loc-1", AvailableToDraw: 100000}},
	}
	opts := cashforecast.Options{MinimumCash: cashforecast.MinimumCashPolicy{MinimumCashBalanceExplicitZero: true}}
	return in, opts
}

// CreditFacilityInsufficient returns a forecast whose funding gap
// exceeds available facility capacity.
func CreditFacilityInsufficient() (cashforecast.Input, cashforecast.Options) {
	in, opts := CreditFacilityAvailable()
	in.Facilities = []cashforecast.CreditFacility{{FacilityID: "loc-1", AvailableToDraw: 5000}}
	return in, opts
}

// RestrictedCashPortfolio returns a forecast with both unrestricted and
// restricted cash accounts, proving the restricted portion is excluded
// from the available balance.
func RestrictedCashPortfolio() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		CashAccounts: []cashforecast.CashAccount{
			{AccountID: "operating", Balance: 30000, Currency: "USD"},
			{AccountID: "payroll-trust", Balance: 15000, Currency: "USD", Restricted: true},
		},
	}
	return in, cashforecast.Options{}
}

// MixedCurrencyInvalid returns a forecast with inconsistent currencies
// across CashAccounts, expected to raise IssueMixedCurrency.
func MixedCurrencyInvalid() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		CashAccounts: []cashforecast.CashAccount{
			{AccountID: "usd-acct", Balance: 20000, Currency: "USD"},
			{AccountID: "eur-acct", Balance: 10000, Currency: "EUR"},
		},
	}
	opts := cashforecast.Options{ReportingCurrency: "USD"}
	return in, opts
}

// ZeroOpeningCash returns a forecast starting from exactly $0 opening
// cash.
func ZeroOpeningCash() (cashforecast.Input, cashforecast.Options) {
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 0, Currency: "USD"},
		Events: []cashforecast.CashFlowEvent{
			{ID: "e1", Date: date("2025-01-08"), Amount: 5000, Direction: cashforecast.DirectionInflow,
				Category: cashforecast.CategoryCashSale, Basis: cashforecast.BasisKnown},
		},
	}
	return in, cashforecast.Options{}
}

// ExactHorizonBoundaryEvent returns a forecast with one event landing on
// the exact last day of the default 13-week horizon and one event one day
// beyond it (BeyondHorizon).
func ExactHorizonBoundaryEvent() (cashforecast.Input, cashforecast.Options) {
	// Week 13 ends on ForecastStartDate + 12*7 + 6 days = 2025-01-06 + 90
	// days = 2025-04-06.
	in := cashforecast.Input{
		ForecastStartDate: date(ForecastStartDate),
		OpeningCash:       cashforecast.OpeningCash{Amount: 10000, Currency: "USD"},
		Events: []cashforecast.CashFlowEvent{
			{ID: "on-boundary", Date: date("2025-04-06"), Amount: 1000, Direction: cashforecast.DirectionInflow,
				Category: cashforecast.CategoryCashSale, Basis: cashforecast.BasisKnown},
			{ID: "past-boundary", Date: date("2025-04-07"), Amount: 2000, Direction: cashforecast.DirectionInflow,
				Category: cashforecast.CategoryCashSale, Basis: cashforecast.BasisKnown},
		},
	}
	return in, cashforecast.Options{}
}
