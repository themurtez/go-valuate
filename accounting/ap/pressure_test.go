package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
)

func pressureWindow(windows []ap.PaymentPressureWindow, days int) (ap.PaymentPressureWindow, bool) {
	for _, w := range windows {
		if w.Days == days {
			return w, true
		}
	}
	return ap.PaymentPressureWindow{}, false
}

func TestPressure_NoLiquidityInputUnavailable(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 3), 1000, 1000, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if result.PaymentPressure.Available {
		t.Errorf("expected PaymentPressure.Available=false when Options.PaymentPressure is nil (never fabricate liquidity)")
	}
}

func TestPressure_SufficientCash(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-7", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 5), 5000, 5000, ap.StatusOpen),
	}
	cash := 100000.0
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{
		AsOfDate:        asOf,
		PaymentPressure: &ap.PaymentPressureInput{CashAvailable: &cash},
	})
	if !result.PaymentPressure.Available {
		t.Fatalf("expected PaymentPressure.Available=true, issues: %+v", result.Issues)
	}
	w7, ok := pressureWindow(result.PaymentPressure.Windows, 7)
	if !ok {
		t.Fatalf("expected a 7-day window")
	}
	if w7.APDue != 5000 {
		t.Errorf("APDue = %v, want 5000", w7.APDue)
	}
	if !w7.NearTermCoverage.Available || w7.NearTermCoverage.Value != 20 {
		t.Errorf("NearTermCoverage = %+v, want 20 (100000/5000)", w7.NearTermCoverage)
	}
	if !w7.Shortfall.Available || w7.Shortfall.Value >= 0 {
		t.Errorf("Shortfall = %+v, want negative (no shortfall, cash covers AP due)", w7.Shortfall)
	}
}

func TestPressure_Shortfall(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-7", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 5), 50000, 50000, ap.StatusOpen),
	}
	cash := 10000.0
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{
		AsOfDate:        asOf,
		PaymentPressure: &ap.PaymentPressureInput{CashAvailable: &cash},
	})
	w7, ok := pressureWindow(result.PaymentPressure.Windows, 7)
	if !ok {
		t.Fatalf("expected a 7-day window")
	}
	if !w7.Shortfall.Available || w7.Shortfall.Value != 40000 {
		t.Errorf("Shortfall = %+v, want 40000 (50000 AP due - 10000 cash)", w7.Shortfall)
	}
	if !w7.NearTermCoverage.Available || w7.NearTermCoverage.Value != 0.2 {
		t.Errorf("NearTermCoverage = %+v, want 0.2", w7.NearTermCoverage)
	}

	foundFlag := false
	for _, f := range result.Flags {
		if f.Code == ap.FlagNearTermPaymentPressure {
			foundFlag = true
		}
	}
	if !foundFlag {
		t.Errorf("expected FlagNearTermPaymentPressure when 7-day coverage < 1.0, flags: %+v", result.Flags)
	}
}

func TestPressure_7_14_30DayWindowsAreCumulative(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-5", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 5), 1000, 1000, ap.StatusOpen),
		bill("B-10", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 10), 2000, 2000, ap.StatusOpen),
		bill("B-25", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 25), 3000, 3000, ap.StatusOpen),
	}
	cash := 0.0
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{
		AsOfDate:        asOf,
		PaymentPressure: &ap.PaymentPressureInput{CashAvailable: &cash},
	})
	w7, _ := pressureWindow(result.PaymentPressure.Windows, 7)
	w14, _ := pressureWindow(result.PaymentPressure.Windows, 14)
	w30, _ := pressureWindow(result.PaymentPressure.Windows, 30)
	if w7.APDue != 1000 {
		t.Errorf("7-day APDue = %v, want 1000", w7.APDue)
	}
	if w14.APDue != 3000 {
		t.Errorf("14-day APDue = %v, want 3000 (cumulative: 1000+2000)", w14.APDue)
	}
	if w30.APDue != 6000 {
		t.Errorf("30-day APDue = %v, want 6000 (cumulative: 1000+2000+3000)", w30.APDue)
	}
}

func TestPressure_ExcludeDisputedOptIn(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 3), 1000, 1000, ap.StatusOpen),
		bill("B-2", "S1", mustDate(t, "2025-06-01"), asOf.AddDate(0, 0, 3), 5000, 5000, ap.StatusDisputed),
	}
	cash := 0.0

	included := ap.Calculate(ap.Input{Payables: payables}, ap.Options{
		AsOfDate:        asOf,
		PaymentPressure: &ap.PaymentPressureInput{CashAvailable: &cash},
	})
	w7, _ := pressureWindow(included.PaymentPressure.Windows, 7)
	if w7.APDue != 6000 {
		t.Errorf("expected disputed included by default: APDue = %v, want 6000", w7.APDue)
	}

	excluded := ap.Calculate(ap.Input{Payables: payables}, ap.Options{
		AsOfDate:                           asOf,
		PaymentPressure:                    &ap.PaymentPressureInput{CashAvailable: &cash},
		ExcludeDisputedFromPaymentPressure: true,
	})
	w7Excluded, _ := pressureWindow(excluded.PaymentPressure.Windows, 7)
	if w7Excluded.APDue != 1000 {
		t.Errorf("expected disputed excluded when opted in: APDue = %v, want 1000", w7Excluded.APDue)
	}
	if !excluded.PaymentPressure.ExcludedDisputed {
		t.Errorf("expected ExcludedDisputed=true to be echoed on the result")
	}

	// Disputed bill must still appear in ordinary aging regardless of the
	// payment-pressure exclusion flag -- disputed bills are never
	// excluded from aging itself.
	if excluded.PortfolioSummary.DisputedAmount != 5000 {
		t.Errorf("DisputedAmount = %v, want 5000 (disputed still ages normally)", excluded.PortfolioSummary.DisputedAmount)
	}
}
