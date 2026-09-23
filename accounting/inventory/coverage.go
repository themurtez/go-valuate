package inventory

// Coverage reports factual data-coverage counts/percentages — never an
// opaque quality score, per task section 55.
type Coverage struct {
	ItemsSupplied int `json:"items_supplied"`

	SnapshotCoverage       Value `json:"snapshot_coverage"`
	UnitCostCoverage       Value `json:"unit_cost_coverage"`
	InventoryValueCoverage Value `json:"inventory_value_coverage"`
	MovementCoverage       Value `json:"movement_coverage"`
	AgeCoverage            Value `json:"age_coverage"`
	LocationCoverage       Value `json:"location_coverage"`
	CategoryCoverage       Value `json:"category_coverage"`
	COGSCoverage           Value `json:"cogs_coverage"`

	GLReconciliationAvailable bool  `json:"gl_reconciliation_available"`
	StockPolicyCoverage       Value `json:"stock_policy_coverage"`
}

func buildCoverage(order []string, states map[string]*itemState, movements []Movement, financials []PeriodFinancials, glControls []GLControl, stockPolicies map[string]StockPolicy, agingRows []ItemAging) Coverage {
	c := Coverage{ItemsSupplied: len(order), GLReconciliationAvailable: len(glControls) > 0}
	if len(order) == 0 {
		return c
	}

	n := float64(len(order))
	var withSnapshot, withUnitCost, withValue, withLocation, withCategory, withPolicy int
	for _, id := range order {
		st := states[id]
		if len(st.latestByLocationLot) > 0 {
			withSnapshot++
		}
		if st.item.Location != "" {
			withLocation++
		}
		if st.item.Category != "" {
			withCategory++
		}
		if _, ok := stockPolicies[id]; ok {
			withPolicy++
		}
		for _, snap := range st.latestByLocationLot {
			if snap.UnitCost.Available {
				withUnitCost++
			}
			if v, _ := resolveSnapshotValue(snap); v.Available {
				withValue++
			}
			break // one representative snapshot per item is sufficient for item-level coverage.
		}
	}
	c.SnapshotCoverage = AvailableValue(float64(withSnapshot) / n)
	c.UnitCostCoverage = AvailableValue(float64(withUnitCost) / n)
	c.InventoryValueCoverage = AvailableValue(float64(withValue) / n)
	c.LocationCoverage = AvailableValue(float64(withLocation) / n)
	c.CategoryCoverage = AvailableValue(float64(withCategory) / n)
	c.StockPolicyCoverage = AvailableValue(float64(withPolicy) / n)

	itemsWithMovement := map[string]bool{}
	for _, m := range movements {
		itemsWithMovement[m.ItemID] = true
	}
	c.MovementCoverage = AvailableValue(float64(len(itemsWithMovement)) / n)

	if len(agingRows) > 0 {
		var known int
		for _, r := range agingRows {
			if r.Evidence != AgeEvidenceUnknown {
				known++
			}
		}
		c.AgeCoverage = AvailableValue(float64(known) / float64(len(agingRows)))
	}

	if len(financials) > 0 {
		var withCOGS int
		for _, f := range financials {
			if f.COGS.Available {
				withCOGS++
			}
		}
		c.COGSCoverage = AvailableValue(float64(withCOGS) / float64(len(financials)))
	}

	return c
}
