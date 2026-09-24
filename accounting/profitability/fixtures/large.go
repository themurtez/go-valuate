package fixtures

import (
	"strconv"
	"time"

	"github.com/themurtez/go-valuate/accounting/profitability"
)

func itoa(v int) string { return strconv.Itoa(v) }

// LargePopulation generates a synthetic large-scale dataset for
// benchmarking — task section 74: numPeriods chronological monthly
// periods, numEntities entities per dimension (CUSTOMER/JOB/PRODUCT
// simultaneously — every dimension enabled at once, task section 74's
// "three dimensions enabled simultaneously"), approximately numFacts
// facts total spread evenly across entities/periods (each fact
// 100%-attributed to one customer, one job, and one product,
// exercising all three dimensions per fact), numPools shared-cost pools
// per period with a NET_REVENUE allocation rule for each of the three
// dimensions.
//
// Every slice is pre-allocated to its known final length and every fact
// is built from a small set of pre-formatted entity-ID strings (rather
// than re-formatting itoa(entityIndex) inside the innermost loop) — a
// real >100-second generation cost at the 1,000,000-fact/100,000-entity
// scale was caught during development, dominated by repeated small-slice
// re-allocation and per-fact strconv.Itoa calls rather than anything in
// Calculate itself.
func LargePopulation(numFacts, numEntities, numPeriods, numPools, numRules int) (
	periods []profitability.PeriodInfo,
	entities []profitability.Entity,
	facts []profitability.Fact,
	pools []profitability.SharedCostPool,
	rules []profitability.AllocationRule,
) {
	periods = make([]profitability.PeriodInfo, 0, numPeriods)
	start := largeDate("2020-01-01")
	for i := 0; i < numPeriods; i++ {
		s := start.AddDate(0, i, 0)
		e := s.AddDate(0, 1, -1)
		periods = append(periods, profitability.PeriodInfo{
			Period: s.Format("2006-01"), StartDate: s, EndDate: e, Days: int(e.Sub(s).Hours()/24) + 1,
		})
	}

	entityIDs := make([]string, numEntities)
	for i := range entityIDs {
		entityIDs[i] = "E" + itoa(i)
	}

	entities = make([]profitability.Entity, 0, numEntities*3)
	for i, id := range entityIDs {
		group := "G" + itoa(i%20)
		entities = append(entities,
			profitability.Entity{Dimension: profitability.DimensionCustomer, EntityID: id, Group: group, Active: true},
			profitability.Entity{Dimension: profitability.DimensionJob, EntityID: id, Group: group, Active: true},
			profitability.Entity{Dimension: profitability.DimensionProduct, EntityID: id, Group: group, Active: true},
		)
	}

	// Distribute numFacts round-robin across every (period, entity) slot
	// so every entity/period pair gets at least a share of the total,
	// without ever generating more than numFacts facts even when
	// numFacts < numEntities*numPeriods (the previous version's
	// factsPerEntityPeriod<1-clamped-to-1 always generated at least
	// numEntities*numPeriods facts regardless of numFacts, silently
	// overshooting the requested scale by a large factor).
	if numFacts <= 0 {
		numFacts = 1
	}
	slots := numEntities * numPeriods
	if slots <= 0 {
		slots = 1
	}

	// factAttrs is reused per entity (only 3 distinct attribution sets
	// exist across the whole population, one per entity) rather than
	// allocated fresh per fact.
	factAttrsByEntity := make([][]profitability.Attribution, numEntities)
	for i, id := range entityIDs {
		factAttrsByEntity[i] = []profitability.Attribution{
			{Dimension: profitability.DimensionCustomer, EntityID: id, Share: 1},
			{Dimension: profitability.DimensionJob, EntityID: id, Share: 1},
			{Dimension: profitability.DimensionProduct, EntityID: id, Share: 1},
		}
	}

	facts = make([]profitability.Fact, 0, numFacts)
	factID := 0
	for factID < numFacts {
		slot := factID % slots
		periodIdx := slot / numEntities
		entityIdx := slot % numEntities
		component := profitability.ComponentGrossRevenue
		switch factID % 3 {
		case 1:
			component = profitability.ComponentDirectMaterial
		case 2:
			component = profitability.ComponentDirectLabor
		}
		facts = append(facts, profitability.Fact{
			FactID: "F" + itoa(factID), Period: periods[periodIdx].Period, Component: component,
			Amount: 100 + float64(factID%50), Currency: "USD", Attributions: factAttrsByEntity[entityIdx],
		})
		factID++
	}

	pools = make([]profitability.SharedCostPool, 0, numPools)
	for i := 0; i < numPools; i++ {
		period := periods[i%len(periods)].Period
		pools = append(pools, profitability.SharedCostPool{
			PoolID: "POOL" + itoa(i), Period: period, Amount: 10000, Category: "overhead",
		})
	}

	rulesPerPool := numRules / numPools
	if rulesPerPool < 1 {
		rulesPerPool = 1
	}
	dims := []profitability.Dimension{profitability.DimensionCustomer, profitability.DimensionJob, profitability.DimensionProduct}
	rules = make([]profitability.AllocationRule, 0, numRules)
	ruleCount := 0
	for i := 0; i < numPools && ruleCount < numRules; i++ {
		for d := 0; d < rulesPerPool && d < len(dims) && ruleCount < numRules; d++ {
			rules = append(rules, profitability.AllocationRule{
				PoolID: "POOL" + itoa(i), Dimension: dims[d], Basis: profitability.AllocationBasisNetRevenue,
			})
			ruleCount++
		}
	}

	return periods, entities, facts, pools, rules
}

func largeDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
