package profitability_test

import (
	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/profitability/fixtures"
)

// fullInput assembles an Input exercising most of this package's paths
// in one Calculate call, for round-trip/immutability/determinism/safety
// tests that just need "a realistic, non-trivial input" rather than a
// scenario-specific fixture.
func fullInput() profitability.Input {
	var entities []profitability.Entity
	var facts []profitability.Fact

	e1, f1 := fixtures.HighRevenueLowMarginCustomer()
	entities = append(entities, e1...)
	facts = append(facts, f1...)

	e2, f2 := fixtures.NegativeContributionCustomer()
	entities = append(entities, e2...)
	facts = append(facts, f2...)

	e3, f3 := fixtures.HighReturnsDiscountsCustomer()
	entities = append(entities, e3...)
	facts = append(facts, f3...)

	e4, f4 := fixtures.LaborHeavyJob()
	entities = append(entities, e4...)
	facts = append(facts, f4...)

	e5, f5 := fixtures.SubcontractorHeavyJob()
	entities = append(entities, e5...)
	facts = append(facts, f5...)

	e6, f6 := fixtures.ProductWithFulfillmentCost()
	entities = append(entities, e6...)
	facts = append(facts, f6...)

	e7, f7 := fixtures.PartialAttributionAcrossTwoProducts()
	entities = append(entities, e7...)
	facts = append(facts, f7...)

	facts = append(facts, fixtures.UnattributedFacts()...)

	eAlloc, fAlloc, pools, rules := fixtures.SharedCostAllocationScenario()
	entities = append(entities, eAlloc...)
	facts = append(facts, fAlloc...)

	controls := []profitability.ControlTotals{
		{Period: "2025-01", NetRevenue: profitability.AvailableValue(0)},
	}

	drivers := []profitability.DriverObservation{
		{ObservationID: "DRV-1", Dimension: profitability.DimensionCustomer, EntityID: "CUST-HRLM", Period: "2025-01", DriverKey: "units_sold", Value: 100, Unit: "units"},
	}

	return profitability.Input{
		Periods:         fixtures.TwoMonthPeriods(),
		Entities:        entities,
		Facts:           facts,
		Drivers:         drivers,
		SharedCostPools: pools,
		AllocationRules: rules,
		Controls:        controls,
	}
}
