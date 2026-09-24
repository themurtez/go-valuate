package profitability_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

func hasIssue(issues []profitability.Issue, code profitability.IssueCode) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestValidate_DuplicatePeriod(t *testing.T) {
	periods := append(fixtures.TwoMonthPeriods(), fixtures.TwoMonthPeriods()[0])
	r := profitability.Calculate(profitability.Input{Periods: periods}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueDuplicatePeriod) {
		t.Error("expected IssueDuplicatePeriod")
	}
}

func TestValidate_InvalidPeriod(t *testing.T) {
	r := profitability.Calculate(profitability.Input{Periods: []profitability.PeriodInfo{{Period: ""}}}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidPeriod) {
		t.Error("expected IssueInvalidPeriod")
	}
}

func TestValidate_DuplicateEntity(t *testing.T) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true},
		{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueDuplicateEntity) {
		t.Error("expected IssueDuplicateEntity")
	}
}

func TestValidate_UnknownEntity(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C-UNKNOWN"})},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueUnknownEntity) {
		t.Error("expected IssueUnknownEntity")
	}
}

func TestValidate_DuplicateFact(t *testing.T) {
	facts := []profitability.Fact{
		{FactID: "DUP", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100, Currency: "USD"},
		{FactID: "DUP", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 999, Currency: "USD"},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Facts: facts}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueDuplicateFact) {
		t.Error("expected IssueDuplicateFact")
	}
	if got := r.BusinessTotals.AllPeriod.RevenueBridge.GrossRevenue; got != 100 {
		t.Errorf("expected only the first occurrence (100) to be used, got %v", got)
	}
}

func TestValidate_InvalidComponent(t *testing.T) {
	facts := []profitability.Fact{{FactID: "F1", Period: "2025-01", Component: "NOT_REAL", Amount: 100, Currency: "USD"}}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Facts: facts}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidComponent) {
		t.Error("expected IssueInvalidComponent")
	}
}

func TestValidate_NegativeAmountRejected(t *testing.T) {
	facts := []profitability.Fact{{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: -1, Currency: "USD"}}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Facts: facts}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueNegativeAmount) {
		t.Error("expected IssueNegativeAmount")
	}
}

func TestValidate_AttributionExceeds100Percent(t *testing.T) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true},
		{Dimension: profitability.DimensionCustomer, EntityID: "C2", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100, Currency: "USD",
			Attributions: []profitability.Attribution{
				{Dimension: profitability.DimensionCustomer, EntityID: "C1", Share: 0.7},
				{Dimension: profitability.DimensionCustomer, EntityID: "C2", Share: 0.7},
			}},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueAttributionExceeds100Percent) {
		t.Error("expected IssueAttributionExceeds100Percent")
	}
	// Never renormalized: this fact must contribute nothing to C1/C2 for
	// the dimension whose attribution was rejected.
	for _, e := range r.CustomerView.AllPeriod {
		if e.RevenueBridge.GrossRevenue != 0 {
			t.Errorf("entity %s got %v revenue; an over-100%% attribution must never be silently renormalized", e.EntityID, e.RevenueBridge.GrossRevenue)
		}
	}
}

func TestValidate_DuplicateAttributionEntityWithinDimension(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100, Currency: "USD",
			Attributions: []profitability.Attribution{
				{Dimension: profitability.DimensionCustomer, EntityID: "C1", Share: 0.3},
				{Dimension: profitability.DimensionCustomer, EntityID: "C1", Share: 0.3},
			}},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidAttribution) {
		t.Error("expected IssueInvalidAttribution for duplicate entity attribution within one dimension")
	}
}

func TestValidate_MixedCurrencyExcluded(t *testing.T) {
	facts := []profitability.Fact{
		{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100, Currency: "USD"},
		{FactID: "F2", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 200, Currency: "EUR"},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Facts: facts}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueMixedCurrency) {
		t.Error("expected IssueMixedCurrency")
	}
	if got := r.BusinessTotals.AllPeriod.RevenueBridge.GrossRevenue; got != 100 {
		t.Errorf("expected only the first-currency-seen (USD, 100) to be used, got %v", got)
	}
}

func TestValidate_InputModeConflict(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
	}
	summaries := []profitability.EntityPeriodSummaryInput{
		{Dimension: profitability.DimensionCustomer, EntityID: "C1", Period: "2025-01", GrossRevenue: profitability.AvailableValue(500)},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts, EntityPeriodSummaries: summaries}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInputModeConflict) {
		t.Error("expected IssueInputModeConflict")
	}
	// The detailed fact must win; the summary row's 500 must not be used.
	for _, e := range r.CustomerView.AllPeriod {
		if e.RevenueBridge.GrossRevenue != 100 {
			t.Errorf("entity %s revenue = %v, want 100 (the detailed fact, not the conflicting summary)", e.EntityID, e.RevenueBridge.GrossRevenue)
		}
	}
}

func TestValidate_InvalidAllocationRuleUnknownPool(t *testing.T) {
	rules := []profitability.AllocationRule{{PoolID: "NOPE", Dimension: profitability.DimensionCustomer, Basis: profitability.AllocationBasisEqual}}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), AllocationRules: rules}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidAllocationRule) {
		t.Error("expected IssueInvalidAllocationRule for a rule referencing an unknown pool")
	}
}

func TestValidate_InvalidFixedWeights(t *testing.T) {
	pools := []profitability.SharedCostPool{{PoolID: "P1", Period: "2025-01", Amount: 100}}
	rules := []profitability.AllocationRule{{PoolID: "P1", Dimension: profitability.DimensionCustomer, Basis: profitability.AllocationBasisFixedWeight}}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), SharedCostPools: pools, AllocationRules: rules}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidFixedWeights) {
		t.Error("expected IssueInvalidFixedWeights for FIXED_WEIGHT basis with no weights")
	}
}

func TestValidate_HasErrors(t *testing.T) {
	issues := []profitability.Issue{{Code: profitability.IssueInvalidPeriod, Severity: profitability.SeverityWarning}}
	if profitability.HasErrors(issues) {
		t.Error("expected HasErrors to be false when only warnings are present")
	}
	issues = append(issues, profitability.Issue{Code: profitability.IssueNonFiniteAmount, Severity: profitability.SeverityError})
	if !profitability.HasErrors(issues) {
		t.Error("expected HasErrors to be true when an error-severity Issue is present")
	}
}
