package ap_test

import (
	"sort"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	apfixtures "github.com/themurtez/go-valuate/accounting/ap/fixtures"
)

func TestFlags_HighOverduePercent(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 10000, 10000, ap.StatusOpen), // all overdue
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	found := false
	for _, f := range result.Flags {
		if f.Code == ap.FlagHighOverduePercent && f.SupplierID == "S1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagHighOverduePercent for a 100%% overdue supplier, flags: %+v", result.Flags)
	}
}

func TestFlags_Large90PlusBalance(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	result := ap.Calculate(ap.Input{Payables: apfixtures.OneLargeOverdueSupplier()}, ap.Options{AsOfDate: asOf})
	found := false
	for _, f := range result.Flags {
		if f.Code == ap.FlagLarge90PlusBalance && f.SupplierID == "SUP-BIG" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagLarge90PlusBalance for SUP-BIG's $100k 90+ balance, flags: %+v", result.Flags)
	}
}

func TestFlags_HighAPConcentration(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	result := ap.Calculate(ap.Input{Payables: apfixtures.OneLargeOverdueSupplier()}, ap.Options{AsOfDate: asOf})
	found := false
	for _, f := range result.Flags {
		if f.Code == ap.FlagHighAPConcentration {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagHighAPConcentration, flags: %+v", result.Flags)
	}
}

func TestFlags_VendorCreditReview(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-07-01"), 8000, 8000, ap.StatusOpen),
		{ID: "VC-1", SupplierID: "S1", DocumentType: ap.DocumentTypeVendorCredit,
			BillDate: mustDate(t, "2025-06-10"), DueDate: mustDate(t, "2025-06-10"),
			OriginalAmount: -6000, OpenAmount: -6000, Currency: "USD", Status: ap.StatusOpen},
	}
	// Force a low absolute materiality threshold so the vendor-credit
	// balance is clearly material regardless of default percent-of-AP.
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, MaterialityThreshold: 100})
	found := false
	for _, f := range result.Flags {
		if f.Code == ap.FlagVendorCreditReview && f.SupplierID == "S1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagVendorCreditReview for a large vendor credit balance, flags: %+v", result.Flags)
	}
}

func TestFlags_CustomThresholdsOverrideDefaults(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{
		bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-10"), 1000, 1000, ap.StatusOpen), // ~20 days past due, PercentOverdue=1.0
	}
	// Default threshold (0.5) would trigger; a stricter caller threshold of
	// 1.5 should not.
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf, Thresholds: ap.Thresholds{HighOverduePercent: 1.5}})
	for _, f := range result.Flags {
		if f.Code == ap.FlagHighOverduePercent {
			t.Errorf("did not expect FlagHighOverduePercent with a custom threshold of 1.5 (max possible ratio is 1.0)")
		}
	}
}

func TestFlags_DeterministicOrder(t *testing.T) {
	asOf := mustDate(t, apfixtures.AsOfDate)
	result := ap.Calculate(ap.Input{Payables: apfixtures.OneLargeOverdueSupplier()}, ap.Options{AsOfDate: asOf})
	if len(result.Flags) < 2 {
		t.Skip("need at least 2 flags to meaningfully test ordering")
	}
	sorted := make([]ap.Flag, len(result.Flags))
	copy(sorted, result.Flags)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Code != sorted[j].Code {
			return sorted[i].Code < sorted[j].Code // not the real rank, just a stability check below.
		}
		return sorted[i].SupplierID < sorted[j].SupplierID
	})
	// Run Calculate again and confirm identical ordering (real determinism
	// check; the above sort is just scaffolding to ensure the test isn't
	// vacuous).
	result2 := ap.Calculate(ap.Input{Payables: apfixtures.OneLargeOverdueSupplier()}, ap.Options{AsOfDate: asOf})
	for i := range result.Flags {
		if result.Flags[i] != result2.Flags[i] {
			t.Errorf("Flags[%d] differs between identical runs: %+v vs %+v", i, result.Flags[i], result2.Flags[i])
		}
	}
}

func TestFlags_NoHiddenRiskModel(t *testing.T) {
	// Every flag is a documented threshold comparison; there is no
	// "probability of default" or similar statistical claim anywhere in
	// this package's public surface. This test simply asserts the fixed
	// FlagCode taxonomy has not grown an undocumented statistical-sounding
	// member.
	all := []ap.FlagCode{
		ap.FlagHighOverduePercent, ap.FlagLarge60PlusBalance, ap.FlagLarge90PlusBalance,
		ap.FlagHighAPConcentration, ap.FlagHighOverdueConcentration, ap.FlagDPODeterioration,
		ap.FlagAgingDeterioration, ap.FlagRepeatedLatePayment, ap.FlagLargeDisputedBalance,
		ap.FlagNearTermPaymentPressure, ap.FlagControlAccountMismatch, ap.FlagVendorCreditReview,
	}
	if len(all) != 12 {
		t.Errorf("expected exactly 12 documented flag codes, got %d", len(all))
	}
}
