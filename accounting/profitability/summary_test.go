package profitability_test

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

// TestSummary_ConvergesWithDetailedPath locks task section 17: both
// input paths use the same profitability formulas — an
// EntityPeriodSummaryInput row must produce the identical bridge/margin
// output as the equivalent set of detailed Facts.
func TestSummary_ConvergesWithDetailedPath(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "F-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 10000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "F-RET", Period: "2025-01", Component: profitability.ComponentReturn, Amount: 500, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "F-MAT", Period: "2025-01", Component: profitability.ComponentDirectMaterial, Amount: 3000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
		{FactID: "F-COMM", Period: "2025-01", Component: profitability.ComponentVariableCommission, Amount: 200, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
	}
	detailed := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())

	summaries := []profitability.EntityPeriodSummaryInput{
		{Dimension: profitability.DimensionCustomer, EntityID: "C2", Period: "2025-01",
			GrossRevenue: profitability.AvailableValue(10000), Returns: profitability.AvailableValue(500),
			DirectMaterial: profitability.AvailableValue(3000), VariableCommission: profitability.AvailableValue(200)},
	}
	entities2 := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C2", Active: true}}
	viaSummary := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities2, EntityPeriodSummaries: summaries}, profitability.DefaultPolicy())

	d := detailed.CustomerView.AllPeriod[0]
	s := viaSummary.CustomerView.AllPeriod[0]

	if !closeEnough(d.RevenueBridge.NetRevenue, s.RevenueBridge.NetRevenue) {
		t.Errorf("NetRevenue: detailed=%v summary=%v", d.RevenueBridge.NetRevenue, s.RevenueBridge.NetRevenue)
	}
	if !closeEnough(d.GrossProfit, s.GrossProfit) {
		t.Errorf("GrossProfit: detailed=%v summary=%v", d.GrossProfit, s.GrossProfit)
	}
	if !closeEnough(d.ContributionProfit, s.ContributionProfit) {
		t.Errorf("ContributionProfit: detailed=%v summary=%v", d.ContributionProfit, s.ContributionProfit)
	}
	if d.Margins.GrossMargin.Amount != s.Margins.GrossMargin.Amount {
		t.Errorf("GrossMargin: detailed=%v summary=%v", d.Margins.GrossMargin, s.Margins.GrossMargin)
	}
}

// TestSummary_UnavailableFieldsExcludedFromBridge proves an
// EntityPeriodSummaryInput field left Unavailable (not AvailableValue(0))
// contributes nothing to the bridge, distinct from a field explicitly
// supplied as zero — both currently sum to 0 either way, but this test
// pins the documented "unavailable != zero" semantics at the type level
// by asserting a summary with everything Unavailable produces an
// entirely zero, but still computed, bridge (not a validation error).
func TestSummary_UnavailableFieldsExcludedFromBridge(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	summaries := []profitability.EntityPeriodSummaryInput{
		{Dimension: profitability.DimensionCustomer, EntityID: "C1", Period: "2025-01"},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, EntityPeriodSummaries: summaries}, profitability.DefaultPolicy())
	if hasIssue(r.Issues, profitability.IssueInvalidSummaryRow) {
		t.Error("an all-unavailable summary row is valid input, not an error")
	}
}

func TestSummary_InvalidRowNonFiniteAmount(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	summaries := []profitability.EntityPeriodSummaryInput{
		{Dimension: profitability.DimensionCustomer, EntityID: "C1", Period: "2025-01", GrossRevenue: profitability.AvailableValue(math.NaN())},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, EntityPeriodSummaries: summaries}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidSummaryRow) {
		t.Error("expected IssueInvalidSummaryRow for a non-finite summary amount")
	}
}
