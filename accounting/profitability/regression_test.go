package profitability_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

// TestRegression_MaterialityZeroThresholdMeansAlwaysMaterial locks a
// code-review fix: an unconfigured (all-zero) MaterialityPolicy must
// treat every nonzero amount as material, mirroring review.IsMaterial's
// "unconfigured never silently suppresses" convention — otherwise
// FlagNegativeContribution/FlagNegativeAllocatedProfit/
// FlagMaterialUnattributedRevenue/FlagMaterialUnattributedCost/
// FlagControlTotalMismatch and the NegativeContribution/
// NegativeAllocatedProfit rankings are permanently dead under
// DefaultPolicy().
func TestRegression_MaterialityZeroThresholdMeansAlwaysMaterial(t *testing.T) {
	entities, facts := fixtures.NegativeContributionCustomer()
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())

	if !hasFlag(r.CustomerView.Flags, profitability.FlagNegativeContribution) {
		t.Error("expected FlagNegativeContribution under DefaultPolicy() (unconfigured materiality must not suppress it)")
	}
	found := false
	for _, n := range r.CustomerView.Rankings.NegativeContribution {
		if n.EntityID == "CUST-NEG" {
			found = true
		}
	}
	if !found {
		t.Error("expected CUST-NEG in NegativeContribution ranking under DefaultPolicy()")
	}
}

// TestRegression_MaterialityZeroAmountNeverMaterial proves the
// unconfigured-materiality fallback does not turn an exact-zero
// difference into a false positive (e.g. a perfectly reconciled control
// total, or zero unattributed revenue, must never be flagged).
func TestRegression_MaterialityZeroAmountNeverMaterial(t *testing.T) {
	facts, controls := fixtures.ControlReconciliationMatch()
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Facts: facts, Controls: controls}, profitability.DefaultPolicy())
	if hasFlag(r.Flags, profitability.FlagControlTotalMismatch) {
		t.Error("a perfectly reconciled control total must never be flagged, even under the unconfigured-materiality fallback")
	}
}

// TestRegression_AllocationExcludesPhantomEntityInsteadOfDroppingDollars
// locks a code-review fix: a FIXED_WEIGHT/EQUAL AllocationRule naming an
// EntityID with no fact/summary activity that period must exclude that
// entity (and report IssueAllocationEntityExcluded) rather than silently
// computing an AllocatedAmount that no EntityPeriodResult can ever
// receive.
func TestRegression_AllocationExcludesPhantomEntityInsteadOfDroppingDollars(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
	}
	pools := []profitability.SharedCostPool{{PoolID: "P1", Period: "2025-01", Amount: 1000}}
	rules := []profitability.AllocationRule{
		{PoolID: "P1", Dimension: profitability.DimensionCustomer, Basis: profitability.AllocationBasisFixedWeight,
			FixedWeights: []profitability.AllocationWeight{{EntityID: "C1", Weight: 1}, {EntityID: "GHOST", Weight: 1}}},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts, SharedCostPools: pools, AllocationRules: rules}, profitability.DefaultPolicy())

	if !hasIssue(r.Issues, profitability.IssueAllocationEntityExcluded) {
		t.Error("expected IssueAllocationEntityExcluded for GHOST")
	}
	if len(r.CustomerView.AllPeriod) != 1 {
		t.Fatalf("expected exactly 1 real entity, got %d", len(r.CustomerView.AllPeriod))
	}
	c1 := r.CustomerView.AllPeriod[0]
	if !closeEnough(c1.AllocatedSharedCosts, 1000) {
		t.Errorf("C1 should receive the FULL pool amount (1000) once GHOST is excluded, got %v", c1.AllocatedSharedCosts)
	}
	// The pool must still report ALLOCATED (not phantom-inflated) and
	// AllocatedAmount == PoolAmount, i.e. every real dollar landed
	// somewhere.
	for _, res := range r.CustomerView.AllocationResults {
		if res.PoolID != "P1" {
			continue
		}
		if !closeEnough(res.AllocatedAmount, 1000) {
			t.Errorf("PoolAllocationResult.AllocatedAmount = %v, want 1000 (all to C1, none vanishing to GHOST)", res.AllocatedAmount)
		}
	}
}

// TestRegression_SummaryOnlyDimensionHasNonZeroCoverage locks a
// code-review fix: AttributionCoverage/DimensionReconciliation for a
// summary-input-only dimension/period must reflect the real economic
// amounts, not report zero activity while EntityPeriods/AllPeriod show
// real numbers.
func TestRegression_SummaryOnlyDimensionHasNonZeroCoverage(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	summaries := []profitability.EntityPeriodSummaryInput{
		{Dimension: profitability.DimensionCustomer, EntityID: "C1", Period: "2025-01", GrossRevenue: profitability.AvailableValue(100000)},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, EntityPeriodSummaries: summaries}, profitability.DefaultPolicy())

	var cov profitability.AttributionCoverage
	for _, c := range r.CustomerView.AttributionCoverage {
		if c.Period == "2025-01" {
			cov = c
		}
	}
	if cov.TotalRevenueAmount != 100000 {
		t.Errorf("AttributionCoverage.TotalRevenueAmount = %v, want 100000 for a summary-only dimension/period", cov.TotalRevenueAmount)
	}
	if cov.AttributedRevenueAmount != 100000 {
		t.Errorf("AttributionCoverage.AttributedRevenueAmount = %v, want 100000", cov.AttributedRevenueAmount)
	}

	var recon profitability.DimensionReconciliation
	for _, dr := range r.DimensionReconciliation {
		if dr.Dimension == profitability.DimensionCustomer && dr.Period == "2025-01" {
			recon = dr
		}
	}
	if recon.BusinessAmount != 100000 {
		t.Errorf("DimensionReconciliation.BusinessAmount = %v, want 100000", recon.BusinessAmount)
	}
	if !recon.Reconciled {
		t.Error("expected DimensionReconciliation.Reconciled == true for a fully-attributed summary-only slot")
	}
}

// TestRegression_BusinessTotalsIncludeUnrelatedSummaryRowInSamePeriod
// locks a code-review fix: BusinessTotals must include a summary-only
// (Dimension, EntityID, Period) slot even when a wholly separate Fact
// exists in the same period for a DIFFERENT dimension/entity — the
// previous "any fact anywhere in this period" gate silently dropped it.
func TestRegression_BusinessTotalsIncludeUnrelatedSummaryRowInSamePeriod(t *testing.T) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionJob, EntityID: "J1", Active: true},
		{Dimension: profitability.DimensionCustomer, EntityID: "C9", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"JOB", "J1"})},
	}
	summaries := []profitability.EntityPeriodSummaryInput{
		{Dimension: profitability.DimensionCustomer, EntityID: "C9", Period: "2025-01", GrossRevenue: profitability.AvailableValue(5000)},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts, EntityPeriodSummaries: summaries}, profitability.DefaultPolicy())

	var jan profitability.BusinessPeriodTotals
	for _, p := range r.BusinessTotals.Periods {
		if p.Period == "2025-01" {
			jan = p
		}
	}
	wantRevenue := 100.0 + 5000.0
	if !closeEnough(jan.RevenueBridge.GrossRevenue, wantRevenue) {
		t.Errorf("BusinessTotals 2025-01 gross revenue = %v, want %v (Fact + unrelated summary row, both counted)", jan.RevenueBridge.GrossRevenue, wantRevenue)
	}
}

// TestRegression_ZeroAmountFactStillCountsAsAttributed locks a
// code-review fix: a Fact with Amount == 0 but a valid, complete
// Attribution (Share > 0) must be marked attributed, not silently
// treated as unattributed just because its dollar contribution is zero.
func TestRegression_ZeroAmountFactStillCountsAsAttributed(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	facts := []profitability.Fact{
		{FactID: "ZERO-1", Period: "2025-01", Component: profitability.ComponentOtherRevenue, Amount: 0, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "C1"})},
	}
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, profitability.DefaultPolicy())

	var cov profitability.AttributionCoverage
	for _, c := range r.CustomerView.AttributionCoverage {
		if c.Period == "2025-01" {
			cov = c
		}
	}
	if cov.TotalFactCount != 1 {
		t.Fatalf("TotalFactCount = %d, want 1", cov.TotalFactCount)
	}
	if cov.AttributedFactCount != 1 {
		t.Errorf("AttributedFactCount = %d, want 1 (a $0 fact with a complete valid attribution is still attributed)", cov.AttributedFactCount)
	}
}

// TestRegression_AttributionToleranceScalesWithFactAmount locks a
// code-review fix: Policy.AttributionTolerance (a share fraction) must
// be applied as a fraction of each Fact's own Amount for the
// unattributed-remainder check, not compared directly against a raw
// dollar remainder using an unrelated fixed constant.
func TestRegression_AttributionToleranceScalesWithFactAmount(t *testing.T) {
	entities := []profitability.Entity{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Active: true}}
	// A $1,000,000 fact attributed at 0.9999 (99.99%) share leaves a
	// $100 remainder — 0.0001 of the amount. With
	// Policy.AttributionTolerance = 0.001 (0.1%, looser than the
	// package's default 0.0001), this remainder must be swallowed as
	// "fully attributed," not reported as $100 unattributed.
	facts := []profitability.Fact{
		{FactID: "F1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1000000, Currency: "USD",
			Attributions: []profitability.Attribution{{Dimension: profitability.DimensionCustomer, EntityID: "C1", Share: 0.9999}}},
	}
	policy := profitability.DefaultPolicy()
	policy.AttributionTolerance = 0.001
	r := profitability.Calculate(profitability.Input{Periods: fixtures.TwoMonthPeriods(), Entities: entities, Facts: facts}, policy)

	u := r.CustomerView.Unattributed["2025-01"]
	if u.UnattributedRevenue != 0 {
		t.Errorf("UnattributedRevenue = %v, want 0 under a 0.001 AttributionTolerance covering a 0.0001 relative remainder", u.UnattributedRevenue)
	}
}
