package vendorspend_test

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

// TestProfitabilityAdapter_OnlyClassifiedRecordsConvert verifies task
// section 28's "only when caller explicitly classifies the spend" rule:
// a SpendRecord absent from the classifications map is never converted,
// even though every other field would be usable.
func TestProfitabilityAdapter_OnlyClassifiedRecordsConvert(t *testing.T) {
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-1", Period: "2025-03", Date: time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
			Amount: 2000, Currency: "USD", Effect: vendorspend.EffectNormal},
		{SpendID: "SP-2", SupplierID: "SUP-1", Period: "2025-03", Date: time.Date(2025, 3, 12, 0, 0, 0, 0, time.UTC),
			Amount: 3000, Currency: "USD", Effect: vendorspend.EffectNormal}, // intentionally unclassified
	}
	classifications := map[string]vendorspend.ProfitabilityClassification{
		"SP-1": {
			Component:    profitability.ComponentDirectMaterial,
			Attributions: []profitability.Attribution{profitability.DirectAttribution(profitability.DimensionCustomer, "CUST-1")},
		},
	}
	facts := vendorspend.FactsFromSpendRecords(records, classifications)
	if len(facts) != 1 {
		t.Fatalf("expected exactly 1 Fact (only SP-1 was classified), got %d: %+v", len(facts), facts)
	}
	if facts[0].FactID != "SP-1" {
		t.Errorf("FactID = %q, want SP-1", facts[0].FactID)
	}
	if facts[0].Component != profitability.ComponentDirectMaterial {
		t.Errorf("Component = %v, want ComponentDirectMaterial", facts[0].Component)
	}
	if facts[0].Amount != 2000 {
		t.Errorf("Amount = %v, want 2000", facts[0].Amount)
	}
}

// TestProfitabilityAdapter_UsesAbsoluteNetSpend verifies a reducing-
// effect SpendRecord (a credit) converts to a non-negative Fact.Amount,
// mirroring profitability.Fact's own "Amount is always non-negative"
// contract.
func TestProfitabilityAdapter_UsesAbsoluteNetSpend(t *testing.T) {
	records := []vendorspend.SpendRecord{
		{SpendID: "SP-CREDIT", SupplierID: "SUP-1", Period: "2025-03", Date: time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
			Amount: 500, Currency: "USD", Effect: vendorspend.EffectCredit},
	}
	classifications := map[string]vendorspend.ProfitabilityClassification{
		"SP-CREDIT": {Component: profitability.ComponentDirectMaterial},
	}
	facts := vendorspend.FactsFromSpendRecords(records, classifications)
	if len(facts) != 1 {
		t.Fatalf("expected 1 Fact, got %d", len(facts))
	}
	if facts[0].Amount != 500 {
		t.Errorf("Amount = %v, want 500 (non-negative magnitude)", facts[0].Amount)
	}
}
