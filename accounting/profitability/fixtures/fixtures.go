// Package fixtures provides reusable synthetic profitability scenarios
// for accounting/profitability's own tests and for any caller building
// integration tests against this package — task section 68.
package fixtures

import (
	"time"

	"github.com/themurtez/go-valuate/accounting/profitability"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// TwoMonthPeriods returns two chronological calendar-month PeriodInfo
// rows: 2025-01 and 2025-02.
func TwoMonthPeriods() []profitability.PeriodInfo {
	return []profitability.PeriodInfo{
		{Period: "2025-01", StartDate: date("2025-01-01"), EndDate: date("2025-01-31"), Days: 31},
		{Period: "2025-02", StartDate: date("2025-02-01"), EndDate: date("2025-02-28"), Days: 28},
	}
}

// ThreeMonthPeriods returns three chronological calendar-month PeriodInfo
// rows: 2025-01, 2025-02, 2025-03.
func ThreeMonthPeriods() []profitability.PeriodInfo {
	return append(TwoMonthPeriods(), profitability.PeriodInfo{
		Period: "2025-03", StartDate: date("2025-03-01"), EndDate: date("2025-03-31"), Days: 31,
	})
}

// HighRevenueLowMarginCustomer returns one customer entity plus facts
// producing high net revenue but thin gross/contribution margin — task
// section 68 "high revenue / low margin."
func HighRevenueLowMarginCustomer() ([]profitability.Entity, []profitability.Fact) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "CUST-HRLM", Name: "Big Volume Co", Group: "Enterprise", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "HRLM-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 100000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-HRLM"})},
		{FactID: "HRLM-MAT", Period: "2025-01", Component: profitability.ComponentDirectMaterial, Amount: 70000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-HRLM"})},
		{FactID: "HRLM-LAB", Period: "2025-01", Component: profitability.ComponentDirectLabor, Amount: 25000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-HRLM"})},
	}
	return entities, facts
}

// NegativeContributionCustomer returns a customer whose contribution
// profit is negative — task section 68 "negative contribution."
func NegativeContributionCustomer() ([]profitability.Entity, []profitability.Fact) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "CUST-NEG", Name: "Underwater Customer", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "NEG-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 10000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-NEG"})},
		{FactID: "NEG-MAT", Period: "2025-01", Component: profitability.ComponentDirectMaterial, Amount: 6000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-NEG"})},
		{FactID: "NEG-LAB", Period: "2025-01", Component: profitability.ComponentDirectLabor, Amount: 5000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-NEG"})},
		{FactID: "NEG-COMM", Period: "2025-01", Component: profitability.ComponentVariableCommission, Amount: 1000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-NEG"})},
	}
	return entities, facts
}

// HighReturnsDiscountsCustomer returns a customer with high return and
// discount rates — task section 68 "high returns/discounts."
func HighReturnsDiscountsCustomer() ([]profitability.Entity, []profitability.Fact) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "CUST-RD", Name: "High Returns Customer", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "RD-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 50000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-RD"})},
		{FactID: "RD-RET", Period: "2025-01", Component: profitability.ComponentReturn, Amount: 8000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-RD"})},
		{FactID: "RD-DISC", Period: "2025-01", Component: profitability.ComponentDiscount, Amount: 7000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-RD"})},
	}
	return entities, facts
}

// LaborHeavyJob returns a job entity dominated by direct labor cost —
// task section 68 "labor-heavy jobs."
func LaborHeavyJob() ([]profitability.Entity, []profitability.Fact) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionJob, EntityID: "JOB-LABOR", Name: "Labor Heavy Job", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "JL-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 40000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"JOB", "JOB-LABOR"})},
		{FactID: "JL-LAB", Period: "2025-01", Component: profitability.ComponentDirectLabor, Amount: 28000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"JOB", "JOB-LABOR"})},
	}
	return entities, facts
}

// SubcontractorHeavyJob returns a job entity dominated by direct
// subcontractor cost — task section 68 "subcontractor-heavy jobs."
func SubcontractorHeavyJob() ([]profitability.Entity, []profitability.Fact) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionJob, EntityID: "JOB-SUB", Name: "Subcontractor Heavy Job", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "JS-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 60000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"JOB", "JOB-SUB"})},
		{FactID: "JS-SUB", Period: "2025-01", Component: profitability.ComponentDirectSubcontractor, Amount: 42000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"JOB", "JOB-SUB"})},
	}
	return entities, facts
}

// ProductWithFulfillmentCost returns a product entity with direct
// material and fulfillment cost — task section 68 "product direct/
// fulfillment cost."
func ProductWithFulfillmentCost() ([]profitability.Entity, []profitability.Fact) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionProduct, EntityID: "PROD-FUL", Name: "Fulfillment Product", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "PF-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 30000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"PRODUCT", "PROD-FUL"})},
		{FactID: "PF-MAT", Period: "2025-01", Component: profitability.ComponentDirectMaterial, Amount: 12000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"PRODUCT", "PROD-FUL"})},
		{FactID: "PF-FUL", Period: "2025-01", Component: profitability.ComponentDirectFulfillment, Amount: 5000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"PRODUCT", "PROD-FUL"})},
	}
	return entities, facts
}

// PartialAttributionAcrossTwoProducts returns one $1,000 revenue fact
// split 60% P1 / 20% P2 (20% left unattributed) — task section 68
// "partial attribution," matching the task's own section 67 worked
// example.
func PartialAttributionAcrossTwoProducts() ([]profitability.Entity, []profitability.Fact) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionProduct, EntityID: "P1", Name: "Product One", Active: true},
		{Dimension: profitability.DimensionProduct, EntityID: "P2", Name: "Product Two", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "PARTIAL-1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 1000, Currency: "USD",
			Attributions: []profitability.Attribution{
				{Dimension: profitability.DimensionProduct, EntityID: "P1", Share: 0.6},
				{Dimension: profitability.DimensionProduct, EntityID: "P2", Share: 0.2},
			}},
	}
	return entities, facts
}

// UnattributedFacts returns facts with no Attributions at all for any
// dimension — task section 68 "unattributed facts."
func UnattributedFacts() []profitability.Fact {
	return []profitability.Fact{
		{FactID: "UNATTR-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 5000, Currency: "USD"},
		{FactID: "UNATTR-COST", Period: "2025-01", Component: profitability.ComponentDirectOther, Amount: 2000, Currency: "USD"},
	}
}

// SharedCostAllocationScenario returns two customers, facts giving them
// different net revenue, one shared-cost pool, and a NET_REVENUE
// allocation rule for CUSTOMER — task section 68 "shared-cost
// allocation."
func SharedCostAllocationScenario() ([]profitability.Entity, []profitability.Fact, []profitability.SharedCostPool, []profitability.AllocationRule) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "CUST-A", Name: "Customer A", Active: true},
		{Dimension: profitability.DimensionCustomer, EntityID: "CUST-B", Name: "Customer B", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "ALLOC-A-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 80000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-A"})},
		{FactID: "ALLOC-B-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 20000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-B"})},
	}
	pools := []profitability.SharedCostPool{
		{PoolID: "POOL-GA", Period: "2025-01", Amount: 10000, Category: "G&A", Description: "General and administrative overhead"},
	}
	rules := []profitability.AllocationRule{
		{PoolID: "POOL-GA", Dimension: profitability.DimensionCustomer, Basis: profitability.AllocationBasisNetRevenue},
	}
	return entities, facts, pools, rules
}

// DifferentBasisPerDimensionScenario returns one pool with different
// allocation bases per dimension: EQUAL for CUSTOMER, FIXED_WEIGHT for
// JOB — task section 68 "different allocation basis per dimension."
func DifferentBasisPerDimensionScenario() ([]profitability.Entity, []profitability.Fact, []profitability.SharedCostPool, []profitability.AllocationRule) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "CUST-X", Active: true},
		{Dimension: profitability.DimensionCustomer, EntityID: "CUST-Y", Active: true},
		{Dimension: profitability.DimensionJob, EntityID: "JOB-X", Active: true},
		{Dimension: profitability.DimensionJob, EntityID: "JOB-Y", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "DBP-1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 10000, Currency: "USD",
			Attributions: []profitability.Attribution{
				{Dimension: profitability.DimensionCustomer, EntityID: "CUST-X", Share: 1},
				{Dimension: profitability.DimensionJob, EntityID: "JOB-X", Share: 1},
			}},
		{FactID: "DBP-2", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 30000, Currency: "USD",
			Attributions: []profitability.Attribution{
				{Dimension: profitability.DimensionCustomer, EntityID: "CUST-Y", Share: 1},
				{Dimension: profitability.DimensionJob, EntityID: "JOB-Y", Share: 1},
			}},
	}
	pools := []profitability.SharedCostPool{
		{PoolID: "POOL-MIX", Period: "2025-01", Amount: 4000},
	}
	rules := []profitability.AllocationRule{
		{PoolID: "POOL-MIX", Dimension: profitability.DimensionCustomer, Basis: profitability.AllocationBasisEqual},
		{PoolID: "POOL-MIX", Dimension: profitability.DimensionJob, Basis: profitability.AllocationBasisFixedWeight,
			FixedWeights: []profitability.AllocationWeight{{EntityID: "JOB-X", Weight: 3}, {EntityID: "JOB-Y", Weight: 1}}},
	}
	return entities, facts, pools, rules
}

// GroupCategorySummaryScenario returns three customers across two
// groups, for GroupSummary/CategorySummary testing — task section 68
// "group/category summaries."
func GroupCategorySummaryScenario() ([]profitability.Entity, []profitability.Fact) {
	entities := []profitability.Entity{
		{Dimension: profitability.DimensionCustomer, EntityID: "CUST-G1-A", Group: "Enterprise", Category: "Retail", Active: true},
		{Dimension: profitability.DimensionCustomer, EntityID: "CUST-G1-B", Group: "Enterprise", Category: "Wholesale", Active: true},
		{Dimension: profitability.DimensionCustomer, EntityID: "CUST-G2-A", Group: "SMB", Category: "Retail", Active: true},
	}
	facts := []profitability.Fact{
		{FactID: "GRP-1", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 10000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-G1-A"})},
		{FactID: "GRP-2", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 20000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-G1-B"})},
		{FactID: "GRP-3", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 5000, Currency: "USD",
			Attributions: profitability.DirectAttributions([2]string{"CUSTOMER", "CUST-G2-A"})},
	}
	return entities, facts
}

// ControlReconciliationMatch returns facts plus a ControlTotals row that
// matches the computed business totals exactly — task section 68
// "control reconciliation match."
func ControlReconciliationMatch() ([]profitability.Fact, []profitability.ControlTotals) {
	facts := []profitability.Fact{
		{FactID: "CTRL-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 10000, Currency: "USD"},
		{FactID: "CTRL-MAT", Period: "2025-01", Component: profitability.ComponentDirectMaterial, Amount: 4000, Currency: "USD"},
	}
	controls := []profitability.ControlTotals{
		{Period: "2025-01", NetRevenue: profitability.AvailableValue(10000), DirectCost: profitability.AvailableValue(4000)},
	}
	return facts, controls
}

// ControlReconciliationMismatch returns facts plus a ControlTotals row
// that deliberately disagrees with the computed business totals — task
// section 68 "control reconciliation mismatch."
func ControlReconciliationMismatch() ([]profitability.Fact, []profitability.ControlTotals) {
	facts := []profitability.Fact{
		{FactID: "MIS-REV", Period: "2025-01", Component: profitability.ComponentGrossRevenue, Amount: 10000, Currency: "USD"},
	}
	controls := []profitability.ControlTotals{
		{Period: "2025-01", NetRevenue: profitability.AvailableValue(9000)},
	}
	return facts, controls
}

// DimensionNotApplicablePolicy returns a Policy marking JOB
// NOT_APPLICABLE — task section 68 "dimension not applicable."
func DimensionNotApplicablePolicy() profitability.Policy {
	p := profitability.DefaultPolicy()
	p.DimensionApplicability = map[profitability.Dimension]profitability.Applicability{
		profitability.DimensionJob: profitability.ApplicabilityNotApplicable,
	}
	return p
}
