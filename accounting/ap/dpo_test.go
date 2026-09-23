package ap_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
)

func TestDPO_UsingPurchasesBasis(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 20000, 20000, ap.StatusOpen)}
	history := []ap.PayablesPeriod{
		{Period: "2025", DenominatorAmount: 200000, Days: 365, Basis: ap.DPOBasisPurchases},
	}
	result := ap.Calculate(ap.Input{Payables: payables, PurchasesHistory: history}, ap.Options{AsOfDate: asOf})
	if !result.DPO.Available {
		t.Fatalf("expected DPO.Available=true, issues: %+v", result.Issues)
	}
	want := (20000.0 / 200000.0) * 365
	if result.DPO.Value != want {
		t.Errorf("DPO.Value = %v, want %v", result.DPO.Value, want)
	}
	if result.DPO.Basis != ap.DPOBasisPurchases {
		t.Errorf("DPO.Basis = %v, want PURCHASES", result.DPO.Basis)
	}
}

func TestDPO_UsingCOGSProxy(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 20000, 20000, ap.StatusOpen)}
	history := []ap.PayablesPeriod{
		{Period: "2025", DenominatorAmount: 200000, Days: 365, Basis: ap.DPOBasisCOGS},
	}
	result := ap.Calculate(ap.Input{Payables: payables, PurchasesHistory: history}, ap.Options{AsOfDate: asOf})
	if !result.DPO.Available {
		t.Fatalf("expected DPO.Available=true")
	}
	if result.DPO.Basis != ap.DPOBasisCOGS {
		t.Errorf("DPO.Basis = %v, want COGS — must be labeled, never silently presented as purchases", result.DPO.Basis)
	}
	want := (20000.0 / 200000.0) * 365
	if result.DPO.Value != want {
		t.Errorf("DPO.Value = %v, want %v", result.DPO.Value, want)
	}
}

func TestDPO_MissingDenominatorUnavailable(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 20000, 20000, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{AsOfDate: asOf})
	if result.DPO.Available {
		t.Errorf("expected DPO.Available=false with no PurchasesHistory supplied")
	}
	if !hasIssueCode(result.Issues, ap.IssueMissingDenominatorForDPO) {
		t.Errorf("expected IssueMissingDenominatorForDPO, got %+v", result.Issues)
	}
}

func TestDPO_ZeroDenominatorUnavailable(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 20000, 20000, ap.StatusOpen)}
	history := []ap.PayablesPeriod{{Period: "2025", DenominatorAmount: 0, Days: 365}}
	result := ap.Calculate(ap.Input{Payables: payables, PurchasesHistory: history}, ap.Options{AsOfDate: asOf})
	if result.DPO.Available {
		t.Errorf("expected DPO.Available=false with zero denominator (never NaN/Inf)")
	}
}

func TestDPO_History(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	ap1 := 12000.0
	ap2 := 18000.0
	history := []ap.PayablesPeriod{
		{Period: "2025-Q1", DenominatorAmount: 40000, Days: 90, EndingAP: &ap1},
		{Period: "2025-Q2", DenominatorAmount: 45000, Days: 91, EndingAP: &ap2},
	}
	result := ap.Calculate(ap.Input{Payables: nil, PurchasesHistory: history}, ap.Options{AsOfDate: asOf})
	if !result.DPOHistory.Available {
		t.Fatalf("expected DPOHistory.Available=true")
	}
	if len(result.DPOHistory.Points) != 2 {
		t.Fatalf("expected 2 DPO history points, got %d", len(result.DPOHistory.Points))
	}
	want1 := (12000.0 / 40000.0) * 90
	want2 := (18000.0 / 45000.0) * 91
	if result.DPOHistory.Points[0].DPO.Value != want1 {
		t.Errorf("Points[0].DPO.Value = %v, want %v", result.DPOHistory.Points[0].DPO.Value, want1)
	}
	if result.DPOHistory.Points[1].DPO.Value != want2 {
		t.Errorf("Points[1].DPO.Value = %v, want %v", result.DPOHistory.Points[1].DPO.Value, want2)
	}
	if !result.DPOHistory.FirstVsLastChange.Available {
		t.Fatalf("expected FirstVsLastChange.Available=true")
	}
	wantChange := want2 - want1
	if result.DPOHistory.FirstVsLastChange.Value != wantChange {
		t.Errorf("FirstVsLastChange = %v, want %v", result.DPOHistory.FirstVsLastChange.Value, wantChange)
	}
	if result.DPOHistory.Trend != "deteriorating" {
		t.Errorf("Trend = %q, want deteriorating (DPO increased)", result.DPOHistory.Trend)
	}
}

func TestDPO_HistoryNeverReconstructsFromCurrentBills(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	// PayablesPeriod with no EndingAP supplied should be skipped entirely
	// -- DPOHistory must never infer a historical AP figure from today's
	// open-item list.
	history := []ap.PayablesPeriod{
		{Period: "2025-Q1", DenominatorAmount: 40000, Days: 90}, // no EndingAP
	}
	payables := []ap.Payable{bill("B-1", "S1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-15"), 99999, 99999, ap.StatusOpen)}
	result := ap.Calculate(ap.Input{Payables: payables, PurchasesHistory: history}, ap.Options{AsOfDate: asOf})
	if result.DPOHistory.Available {
		t.Errorf("expected DPOHistory.Available=false when no period supplies EndingAP, got %+v", result.DPOHistory)
	}
}
