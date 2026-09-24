package profitability_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

// prohibitedTerms are the words this package's generated text must never
// contain — task sections 63-64: pricing/termination/discontinuation
// recommendation terms, and employee-evaluation-adjacent terms (this
// package has no worker/employee concept of its own, but DIRECT_LABOR
// facts may originate from a workforce, so the same discipline applies to
// any message this package itself generates).
var prohibitedTerms = []string{
	"drop customer", "fire customer", "discontinue product", "discontinue this product",
	"raise price", "raise prices", "lower price", "lower prices", "increase price", "increase prices",
	"bad customer", "bad product", "bad job",
	"should be dropped", "should be discontinued", "should be fired", "should be terminated",
	"terminate this customer", "terminate the customer", "terminate this product",
	"best customer", "worst customer", "best product", "worst product",
	"recommend", "recommendation",
	"employee performance", "worker performance", "performance score", "performance ranking",
}

func scanForProhibitedLanguage(t *testing.T, label, text string) {
	t.Helper()
	lower := strings.ToLower(text)
	for _, term := range prohibitedTerms {
		if strings.Contains(lower, term) {
			t.Errorf("%s contains prohibited term %q: %q", label, term, text)
		}
	}
}

func buildSafetyFixtureInput() profitability.Input {
	var entities []profitability.Entity
	var facts []profitability.Fact

	e1, f1 := fixtures.NegativeContributionCustomer()
	entities = append(entities, e1...)
	facts = append(facts, f1...)

	e2, f2 := fixtures.HighReturnsDiscountsCustomer()
	entities = append(entities, e2...)
	facts = append(facts, f2...)

	eAlloc, fAlloc, pools, rules := fixtures.SharedCostAllocationScenario()
	entities = append(entities, eAlloc...)
	facts = append(facts, fAlloc...)

	facts = append(facts, fixtures.UnattributedFacts()...)

	_, mismatchControls := fixtures.ControlReconciliationMismatch()

	policy := profitability.DefaultPolicy()
	policy.NegativeContributionMateriality = profitability.MaterialityPolicy{AbsoluteAmount: 1}
	policy.UnattributedMateriality = profitability.MaterialityPolicy{AbsoluteAmount: 1}
	policy.ControlMateriality = profitability.MaterialityPolicy{AbsoluteAmount: 1}
	policy.MinRevenueAttributionCoverage = 0.99
	policy.MinCostAttributionCoverage = 0.99

	return profitability.Input{
		Periods:         fixtures.TwoMonthPeriods(),
		Entities:        entities,
		Facts:           facts,
		SharedCostPools: pools,
		AllocationRules: rules,
		Controls:        mismatchControls,
	}
}

func TestSafety_NoProhibitedLanguageInFlagMessages(t *testing.T) {
	policy := profitability.DefaultPolicy()
	policy.NegativeContributionMateriality = profitability.MaterialityPolicy{AbsoluteAmount: 1}
	policy.UnattributedMateriality = profitability.MaterialityPolicy{AbsoluteAmount: 1}
	policy.ControlMateriality = profitability.MaterialityPolicy{AbsoluteAmount: 1}
	policy.MinRevenueAttributionCoverage = 0.99
	policy.MinCostAttributionCoverage = 0.99

	r := profitability.Calculate(buildSafetyFixtureInput(), policy)
	if len(r.Flags) == 0 {
		t.Fatal("expected at least one Flag from the fixture set to make this scan meaningful")
	}
	seen := map[profitability.FlagCode]bool{}
	for _, f := range r.Flags {
		scanForProhibitedLanguage(t, "Flag "+string(f.Code)+" Message", f.Message)
		seen[f.Code] = true
	}
	if len(seen) < 4 {
		t.Errorf("expected the fixture set to exercise at least 4 distinct FlagCodes for full message coverage, got %d: %v", len(seen), seen)
	}
}

func TestSafety_NoProhibitedLanguageInIssueMessages(t *testing.T) {
	in := profitability.Input{
		Periods: fixtures.TwoMonthPeriods(),
		Facts: []profitability.Fact{
			{FactID: "BAD-1", Period: "2025-01", Component: "NOT_A_COMPONENT", Amount: 100, Currency: "USD"},
			{FactID: "BAD-1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: -5, Currency: "USD"},
		},
	}
	r := profitability.Calculate(in, profitability.DefaultPolicy())
	if len(r.Issues) == 0 {
		t.Fatal("expected at least one Issue from the broken-input fixture")
	}
	for _, iss := range r.Issues {
		scanForProhibitedLanguage(t, "Issue "+string(iss.Code)+" Message", iss.Message)
	}
}

func TestSafety_NaNInfThresholdsRejected(t *testing.T) {
	policy := profitability.DefaultPolicy()
	policy.HighReturnRate = math.NaN()
	r := profitability.Calculate(fullInput(), policy)

	found := false
	for _, iss := range r.Issues {
		if iss.Code == profitability.IssueInvalidPolicy {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidPolicy for a NaN HighReturnRate")
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "NaN") || strings.Contains(string(b), "Infinity") {
		t.Error("Result JSON must never contain NaN or Infinity")
	}
}

func TestSafety_NonFiniteAmountsExcludedNotPropagated(t *testing.T) {
	in := fullInput()
	in.Facts = append(in.Facts, profitability.Fact{
		FactID: "INF-1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: math.Inf(1), Currency: "USD",
	})
	r := profitability.Calculate(in, profitability.DefaultPolicy())

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "Inf") {
		t.Error("Result JSON must never contain a non-finite value from a bad fact")
	}
	for _, p := range r.CustomerView.EntityPeriods {
		for _, id := range p.FactIDs {
			if id == "INF-1" {
				t.Error("a non-finite-amount fact must be excluded from provenance")
			}
		}
	}
}
