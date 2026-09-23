package ar_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
)

func TestFlags_HighOverduePercent(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 10000, 10000, ar.StatusOpen), // 100% overdue
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	found := false
	for _, f := range result.Flags {
		if f.Code == ar.FlagHighOverduePercent && f.CustomerID == "C1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagHighOverduePercent for C1, got %+v", result.Flags)
	}
}

func TestFlags_CustomThresholds(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		// One current, one 10-days-overdue invoice -> 50% of this customer's balance is overdue.
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-07-15"), 1000, 1000, ar.StatusOpen),
		recv("R-2", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-20"), 1000, 1000, ar.StatusOpen),
	}
	// Default threshold (>0.5) should NOT trigger at exactly 50%; a lower threshold should.
	defaultResult := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	lenient := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, Thresholds: ar.Thresholds{HighOverduePercent: 0.1}})

	hasFlag := func(r ar.Result) bool {
		for _, f := range r.Flags {
			if f.Code == ar.FlagHighOverduePercent {
				return true
			}
		}
		return false
	}
	if hasFlag(defaultResult) {
		t.Errorf("expected default threshold (>0.5) to NOT trigger at exactly 50%% overdue")
	}
	if !hasFlag(lenient) {
		t.Errorf("expected 0.1 threshold to trigger FlagHighOverduePercent at 50%% overdue")
	}
}

func TestFlags_DeterministicOrder(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C2", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 5000, 5000, ar.StatusOpen),
		recv("R-2", "C1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 5000, 5000, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})

	// Within the same FlagCode, entries must be sorted by CustomerID ascending.
	var overduePctFlags []ar.Flag
	for _, f := range result.Flags {
		if f.Code == ar.FlagHighOverduePercent {
			overduePctFlags = append(overduePctFlags, f)
		}
	}
	if len(overduePctFlags) != 2 {
		t.Fatalf("expected 2 FlagHighOverduePercent, got %d: %+v", len(overduePctFlags), overduePctFlags)
	}
	if overduePctFlags[0].CustomerID != "C1" || overduePctFlags[1].CustomerID != "C2" {
		t.Errorf("expected C1 before C2, got %s then %s", overduePctFlags[0].CustomerID, overduePctFlags[1].CustomerID)
	}
}

func TestFlags_LargeDisputedBalance(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-05-01"), mustDate(t, "2025-05-31"), 50000, 50000, ar.StatusDisputed),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	found := false
	for _, f := range result.Flags {
		if f.Code == ar.FlagLargeDisputedBalance {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FlagLargeDisputedBalance given large disputed balance and default materiality, got %+v", result.Flags)
	}
}

func TestCollectionPriority_RankedAndLabeled(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C-OLD", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 20000, 20000, ar.StatusOpen),
		recv("R-2", "C-NEW", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-20"), 500, 500, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})
	if !result.CollectionPriority.Available {
		t.Fatalf("expected CollectionPriority.Available=true")
	}
	if result.CollectionPriority.Label == "" {
		t.Errorf("expected non-empty heuristic Label")
	}
	if len(result.CollectionPriority.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.CollectionPriority.Entries))
	}
	if result.CollectionPriority.Entries[0].CustomerID != "C-OLD" {
		t.Errorf("expected C-OLD (larger, older balance) ranked first, got %s", result.CollectionPriority.Entries[0].CustomerID)
	}
	if result.CollectionPriority.Entries[0].Rank != 1 {
		t.Errorf("expected Rank=1 for top entry")
	}
	for _, e := range result.CollectionPriority.Entries {
		if len(e.Factors) == 0 {
			t.Errorf("expected non-empty Factors for %s (components must be returned)", e.CustomerID)
		}
	}
}

func TestCollectionPriority_CustomWeights(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-06-01"), mustDate(t, "2025-06-20"), 1000, 1000, ar.StatusOpen),
	}
	weights := ar.PriorityWeights{BalanceWeight: 1.0}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, PriorityWeights: weights})
	if result.CollectionPriority.Weights.BalanceWeight != 1.0 {
		t.Errorf("expected custom weight to be echoed, got %+v", result.CollectionPriority.Weights)
	}
}

func TestMateriality_AbsoluteThresholdDrivesLargeBalanceFlags(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	// 91+ day overdue balance of 10000.
	receivables := []ar.Receivable{
		recv("R-1", "C1", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 10000, 10000, ar.StatusOpen),
	}
	// A tiny absolute materiality threshold means Large60Plus/Large90PlusBalance
	// resolve to that small figure, so this balance should trigger both.
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf, MaterialityThreshold: 1})

	has := func(code ar.FlagCode) bool {
		for _, f := range result.Flags {
			if f.Code == code {
				return true
			}
		}
		return false
	}
	if !has(ar.FlagLarge60PlusBalance) {
		t.Errorf("expected FlagLarge60PlusBalance with tiny materiality threshold, got %+v", result.Flags)
	}
	if !has(ar.FlagLarge90PlusBalance) {
		t.Errorf("expected FlagLarge90PlusBalance with tiny materiality threshold, got %+v", result.Flags)
	}
}

func TestMateriality_AssessmentPercentUnavailableWhenTotalZero(t *testing.T) {
	m := ar.MaterialityAssessment{}
	if m.PercentOfTotalAR.Available {
		t.Errorf("zero-value MaterialityAssessment should have PercentOfTotalAR unavailable")
	}
}

func TestMateriality_CustomerSummaryWiredIn(t *testing.T) {
	asOf := mustDate(t, "2025-06-30")
	receivables := []ar.Receivable{
		// Dominant, clearly material overdue customer.
		recv("R-1", "C-BIG", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 95000, 95000, ar.StatusOpen),
		// Tiny overdue customer, immaterial under default 5%-of-AR policy.
		recv("R-2", "C-SMALL", mustDate(t, "2025-01-01"), mustDate(t, "2025-01-31"), 50, 50, ar.StatusOpen),
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{AsOfDate: asOf})

	var big, small ar.CustomerSummary
	for _, cs := range result.CustomerSummaries {
		switch cs.CustomerID {
		case "C-BIG":
			big = cs
		case "C-SMALL":
			small = cs
		}
	}
	if !big.Materiality.Material {
		t.Errorf("expected C-BIG's overdue balance to be material, got %+v", big.Materiality)
	}
	if small.Materiality.Material {
		t.Errorf("expected C-SMALL's overdue balance to be immaterial, got %+v", small.Materiality)
	}
	if !big.Materiality.PercentOfTotalAR.Available {
		t.Errorf("expected PercentOfTotalAR.Available=true for C-BIG")
	}
}
