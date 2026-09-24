package profitability_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

func TestViewFor(t *testing.T) {
	r := profitability.Calculate(fullInput(), profitability.DefaultPolicy())
	if r.ViewFor(profitability.DimensionCustomer).Dimension != profitability.DimensionCustomer {
		t.Error("ViewFor(CUSTOMER) mismatch")
	}
	if r.ViewFor(profitability.DimensionJob).Dimension != profitability.DimensionJob {
		t.Error("ViewFor(JOB) mismatch")
	}
	if r.ViewFor(profitability.DimensionProduct).Dimension != profitability.DimensionProduct {
		t.Error("ViewFor(PRODUCT) mismatch")
	}
	if r.ViewFor("NOT_A_DIMENSION").Dimension != "" {
		t.Error("ViewFor with an unrecognized dimension should return the zero value")
	}
}

func TestIntegration_LaborSubcontractorAdapter(t *testing.T) {
	record := labor.ContractorLaborRecord{ID: "CR-1", ContractorID: "VEND-1", Period: "2025-01", Date: mustDate("2025-01-15"), Amount: 5000, Currency: "USD"}
	attrs := profitability.DirectAttributions([2]string{"JOB", "JOB-1"})
	fact := profitability.DirectSubcontractorFactFromContractorRecord(record, attrs)
	if fact.Component != profitability.ComponentDirectSubcontractor {
		t.Errorf("Component = %s, want DIRECT_SUBCONTRACTOR", fact.Component)
	}
	if fact.Amount != 5000 {
		t.Errorf("Amount = %v, want 5000", fact.Amount)
	}
}

func TestValidate_SharedCostPoolInvalidAndDuplicate(t *testing.T) {
	pools := []profitability.SharedCostPool{
		{PoolID: "", Period: "2025-01", Amount: 100},
		{PoolID: "P1", Period: "2025-01", Amount: -5},
		{PoolID: "P2", Period: "2025-01", Amount: 100},
		{PoolID: "P2", Period: "2025-01", Amount: 200},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), SharedCostPools: pools}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidSharedCostPool) {
		t.Error("expected IssueInvalidSharedCostPool")
	}
	if !hasIssue(r.Issues, profitability.IssueDuplicatePool) {
		t.Error("expected IssueDuplicatePool")
	}
}

func TestValidate_AllocationRuleInvalidBasisAndMissingDriverKey(t *testing.T) {
	pools := []profitability.SharedCostPool{{PoolID: "P1", Period: "2025-01", Amount: 100}}
	rules := []profitability.AllocationRule{
		{PoolID: "P1", Dimension: profitability.DimensionCustomer, Basis: "NOT_A_BASIS"},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), SharedCostPools: pools, AllocationRules: rules}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidAllocationRule) {
		t.Error("expected IssueInvalidAllocationRule for an unrecognized basis")
	}

	rules2 := []profitability.AllocationRule{
		{PoolID: "P1", Dimension: profitability.DimensionCustomer, Basis: profitability.AllocationBasisDriver},
	}
	r2 := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), SharedCostPools: pools, AllocationRules: rules2}, profitability.DefaultPolicy())
	if !hasIssue(r2.Issues, profitability.IssueInvalidAllocationRule) {
		t.Error("expected IssueInvalidAllocationRule for DRIVER basis with no driver_key")
	}
}

func TestValidate_DuplicateAllocationRule(t *testing.T) {
	pools := []profitability.SharedCostPool{{PoolID: "P1", Period: "2025-01", Amount: 100}}
	rules := []profitability.AllocationRule{
		{PoolID: "P1", Dimension: profitability.DimensionCustomer, Basis: profitability.AllocationBasisEqual},
		{PoolID: "P1", Dimension: profitability.DimensionCustomer, Basis: profitability.AllocationBasisNetRevenue},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), SharedCostPools: pools, AllocationRules: rules}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueDuplicateAllocationRule) {
		t.Error("expected IssueDuplicateAllocationRule")
	}
}

func TestValidate_InvalidControlTotal(t *testing.T) {
	controls := []profitability.ControlTotals{
		{Period: ""},
		{Period: "2025-01", NetRevenue: profitability.AvailableValue(100)},
		{Period: "2025-01", NetRevenue: profitability.AvailableValue(200)},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Controls: controls}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidControlTotal) {
		t.Error("expected IssueInvalidControlTotal for an empty period and for a duplicate period")
	}
}

func TestValidate_NegativeFixedWeightRejected(t *testing.T) {
	pools := []profitability.SharedCostPool{{PoolID: "P1", Period: "2025-01", Amount: 100}}
	rules := []profitability.AllocationRule{
		{PoolID: "P1", Dimension: profitability.DimensionCustomer, Basis: profitability.AllocationBasisFixedWeight,
			FixedWeights: []profitability.AllocationWeight{{EntityID: "C1", Weight: -1}}},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), SharedCostPools: pools, AllocationRules: rules}, profitability.DefaultPolicy())
	if !hasIssue(r.Issues, profitability.IssueInvalidFixedWeights) {
		t.Error("expected IssueInvalidFixedWeights for a negative weight")
	}
}

func TestPolicy_NegativeTopNRejected(t *testing.T) {
	policy := profitability.DefaultPolicy()
	policy.TopN = -1
	r := profitability.Calculate(fixtures_input(), policy)
	if !hasIssue(r.Issues, profitability.IssueInvalidPolicy) {
		t.Error("expected IssueInvalidPolicy for a negative TopN")
	}
}

func fixtures_input() profitability.Input {
	entities, facts := fixtures.HighRevenueLowMarginCustomer()
	return profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}
}

func TestFixtures_ScenariosProduceNonEmptyOutput(t *testing.T) {
	scenarios := []func() ([]profitability.Entity, []profitability.Fact){
		fixtures.HighRevenueLowMarginCustomer,
		fixtures.NegativeContributionCustomer,
		fixtures.HighReturnsDiscountsCustomer,
		fixtures.LaborHeavyJob,
		fixtures.SubcontractorHeavyJob,
		fixtures.ProductWithFulfillmentCost,
		fixtures.PartialAttributionAcrossTwoProducts,
		fixtures.GroupCategorySummaryScenario,
	}
	for i, scenario := range scenarios {
		entities, facts := scenario()
		if len(entities) == 0 || len(facts) == 0 {
			t.Errorf("scenario %d returned empty entities/facts", i)
		}
		r := profitability.Calculate(profitability.Input{Periods: fixtures.ThreeMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())
		if profitability.HasErrors(r.Issues) {
			t.Errorf("scenario %d produced unexpected error-severity Issues: %+v", i, r.Issues)
		}
	}

	if len(fixtures.UnattributedFacts()) == 0 {
		t.Error("UnattributedFacts should be non-empty")
	}
	e, f, p, ru := fixtures.SharedCostAllocationScenario()
	if len(e) == 0 || len(f) == 0 || len(p) == 0 || len(ru) == 0 {
		t.Error("SharedCostAllocationScenario should be fully populated")
	}
	e2, f2, p2, ru2 := fixtures.DifferentBasisPerDimensionScenario()
	if len(e2) == 0 || len(f2) == 0 || len(p2) == 0 || len(ru2) == 0 {
		t.Error("DifferentBasisPerDimensionScenario should be fully populated")
	}
	cf, cc := fixtures.ControlReconciliationMatch()
	if len(cf) == 0 || len(cc) == 0 {
		t.Error("ControlReconciliationMatch should be fully populated")
	}
	mf, mc := fixtures.ControlReconciliationMismatch()
	if len(mf) == 0 || len(mc) == 0 {
		t.Error("ControlReconciliationMismatch should be fully populated")
	}
	if fixtures.DimensionNotApplicablePolicy().DimensionApplicability[profitability.DimensionJob] != profitability.ApplicabilityNotApplicable {
		t.Error("DimensionNotApplicablePolicy should mark JOB not applicable")
	}
}
