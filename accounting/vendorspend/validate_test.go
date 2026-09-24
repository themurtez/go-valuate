package vendorspend_test

import (
	"math"
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
	"github.com/themurtez/go-valuate/accounting/vendorspend/fixtures"
)

func hasIssueCode(issues []vendorspend.Issue, code vendorspend.IssueCode) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

// TestValidate_DuplicateSpendID verifies a repeated SpendID is flagged
// and only the first occurrence is used.
func TestValidate_DuplicateSpendID(t *testing.T) {
	periods := fixtures.SixMonthPeriods()
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}}
	records := []vendorspend.SpendRecord{
		{SpendID: "DUP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 100, Currency: "USD"},
		{SpendID: "DUP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC), Amount: 999, Currency: "USD"},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})
	if !hasIssueCode(result.Issues, vendorspend.IssueDuplicateSpend) {
		t.Errorf("expected IssueDuplicateSpend, got %+v", result.Issues)
	}
	if result.Bridge.NetSpend != 100 {
		t.Errorf("NetSpend = %v, want 100 (only the first occurrence used)", result.Bridge.NetSpend)
	}
}

// TestValidate_UnknownSupplier verifies a SpendRecord referencing an
// unregistered SupplierID is excluded and flagged.
func TestValidate_UnknownSupplier(t *testing.T) {
	periods := fixtures.SixMonthPeriods()
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-KNOWN", Active: true}}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-UNKNOWN", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 100, Currency: "USD"},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})
	if !hasIssueCode(result.Issues, vendorspend.IssueUnknownSupplier) {
		t.Errorf("expected IssueUnknownSupplier, got %+v", result.Issues)
	}
	if result.Available {
		t.Errorf("expected Available == false (no valid spend records survived), got Bridge=%+v", result.Bridge)
	}
}

// TestValidate_NonFiniteAmount verifies NaN/Inf amounts are excluded and
// flagged.
func TestValidate_NonFiniteAmount(t *testing.T) {
	periods := fixtures.SixMonthPeriods()
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 100, Currency: "USD"},
		{SpendID: "SP-NAN", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC), Amount: math.Inf(-1), Currency: "USD"},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})
	if !hasIssueCode(result.Issues, vendorspend.IssueNonFiniteAmount) {
		t.Errorf("expected IssueNonFiniteAmount, got %+v", result.Issues)
	}
	if result.Bridge.NetSpend != 100 {
		t.Errorf("NetSpend = %v, want 100 (non-finite record excluded)", result.Bridge.NetSpend)
	}
}

// TestValidate_InvalidEffect verifies an unrecognized Effect value
// excludes the record (Effect determines gross/net semantics, so a
// silently-defaulted Effect could misstate spend direction).
func TestValidate_InvalidEffect(t *testing.T) {
	periods := fixtures.SixMonthPeriods()
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			Amount: 100, Currency: "USD", Effect: "NOT_A_REAL_EFFECT"},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})
	if !hasIssueCode(result.Issues, vendorspend.IssueInvalidEffect) {
		t.Errorf("expected IssueInvalidEffect, got %+v", result.Issues)
	}
	if result.Available {
		t.Errorf("expected Available == false, got Bridge=%+v", result.Bridge)
	}
}

// TestValidate_InvalidSpendType_DefaultsToOther verifies an unrecognized
// SpendType is advisory only (record still included, resolved to OTHER).
func TestValidate_InvalidSpendType_DefaultsToOther(t *testing.T) {
	periods := fixtures.SixMonthPeriods()
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			Amount: 100, Currency: "USD", SpendType: "NOT_A_REAL_TYPE"},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})
	if !hasIssueCode(result.Issues, vendorspend.IssueInvalidSpendType) {
		t.Errorf("expected IssueInvalidSpendType (advisory), got %+v", result.Issues)
	}
	if !result.Available {
		t.Fatalf("expected Available == true (invalid SpendType is advisory only), got Issues: %+v", result.Issues)
	}
	if result.Bridge.NetSpend != 100 {
		t.Errorf("NetSpend = %v, want 100 (record still included)", result.Bridge.NetSpend)
	}
	found := false
	for _, ts := range result.PeriodSummaries[0].SpendByType {
		if ts.Key == string(vendorspend.SpendTypeOther) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the record to resolve into SpendTypeOther's bucket, got %+v", result.PeriodSummaries[0].SpendByType)
	}
}

// TestValidate_DuplicateSupplierID verifies a repeated SupplierID is
// flagged and only the first occurrence is used.
func TestValidate_DuplicateSupplierID(t *testing.T) {
	periods := fixtures.SixMonthPeriods()
	suppliers := []vendorspend.Supplier{
		{SupplierID: "SUP-DUP", Name: "First", Active: true},
		{SupplierID: "SUP-DUP", Name: "Second", Active: false},
	}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-DUP", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 100, Currency: "USD"},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})
	if !hasIssueCode(result.Issues, vendorspend.IssueDuplicateSupplier) {
		t.Errorf("expected IssueDuplicateSupplier, got %+v", result.Issues)
	}
	if len(result.SupplierSummaries) != 1 || result.SupplierSummaries[0].SupplierName != "First" {
		t.Errorf("expected the FIRST supplier occurrence to be used, got %+v", result.SupplierSummaries)
	}
}

// TestValidate_SupplierParentCycle verifies a ParentID cycle is detected
// without infinite-looping.
func TestValidate_SupplierParentCycle(t *testing.T) {
	periods := fixtures.SixMonthPeriods()
	suppliers := []vendorspend.Supplier{
		{SupplierID: "SUP-A", ParentID: "SUP-B", Active: true},
		{SupplierID: "SUP-B", ParentID: "SUP-A", Active: true},
	}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-A", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 100, Currency: "USD"},
	}
	done := make(chan vendorspend.Result, 1)
	go func() {
		done <- vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})
	}()
	select {
	case result := <-done:
		if !hasIssueCode(result.Issues, vendorspend.IssueInvalidSupplierParent) {
			t.Errorf("expected IssueInvalidSupplierParent for a parent cycle, got %+v", result.Issues)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Calculate did not return within 5s — likely an infinite loop in cycle detection")
	}
}

// TestValidate_MissingCurrency_Excludes verifies a record with no
// Currency is excluded (required field).
func TestValidate_MissingCurrency_Excludes(t *testing.T) {
	periods := fixtures.SixMonthPeriods()
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), Amount: 100},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})
	if !hasIssueCode(result.Issues, vendorspend.IssueMixedCurrency) {
		t.Errorf("expected IssueMixedCurrency for missing currency, got %+v", result.Issues)
	}
	if result.Available {
		t.Errorf("expected Available == false, got Bridge=%+v", result.Bridge)
	}
}

// TestValidate_QuantityWithoutUOM_Flagged verifies a record supplying
// Quantity/UnitPrice but no UnitOfMeasure is flagged (unusable for
// unit-price analytics) though the record itself is not excluded.
func TestValidate_QuantityWithoutUOM_Flagged(t *testing.T) {
	periods := fixtures.SixMonthPeriods()
	suppliers := []vendorspend.Supplier{{SupplierID: "SUP-1", Active: true}}
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-01", Date: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC),
			Amount: 100, Currency: "USD", Quantity: vendorspend.AvailableValue(10), UnitPrice: vendorspend.AvailableValue(10)},
	}
	result := vendorspend.Calculate(vendorspend.Input{Periods: periods, Suppliers: suppliers, SpendRecords: records}, vendorspend.Options{})
	if !hasIssueCode(result.Issues, vendorspend.IssueInvalidUOM) {
		t.Errorf("expected IssueInvalidUOM, got %+v", result.Issues)
	}
	if !result.Available {
		t.Fatalf("expected Available == true (advisory only), got Issues: %+v", result.Issues)
	}
	if len(result.UnitPricePoints) != 0 {
		t.Errorf("expected no UnitPricePoints (no usable UOM), got %+v", result.UnitPricePoints)
	}
}
