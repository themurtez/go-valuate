package profitability

// DriverObservation is one caller-supplied activity-driver measurement
// for one entity/period — task section 18. Portable: DriverKey is an
// open string (e.g. "units_sold", "labor_hours", "orders",
// "transactions", "shipments"), never a closed enum, since the set of
// meaningful drivers varies by business.
type DriverObservation struct {
	// ObservationID uniquely identifies this observation within one
	// analysis. Required; duplicates for the same (Dimension, EntityID,
	// Period, DriverKey) are flagged (see IssueDuplicateDriver) and only
	// the first occurrence (input order) is used.
	ObservationID string    `json:"observation_id"`
	Dimension     Dimension `json:"dimension"`
	EntityID      string    `json:"entity_id"`
	Period        string    `json:"period"`
	DriverKey     string    `json:"driver_key"`
	// Value is this observation's non-negative magnitude.
	Value float64 `json:"value"`
	// Unit is an optional caller-defined unit label (e.g. "hours",
	// "units"), carried through for display only.
	Unit      string    `json:"unit,omitempty"`
	SourceRef SourceRef `json:"source_ref,omitempty"`
}

// driverObsKey identifies one DriverObservation's dedup/lookup slot.
type driverObsKey struct {
	Dimension Dimension
	EntityID  string
	Period    string
	DriverKey string
}

// PerUnitMetrics reports economics per unit of a caller-selected primary
// driver — task section 19. Every field is a Value; a zero or
// unavailable denominator yields Unavailable, never NaN/Inf.
type PerUnitMetrics struct {
	// DriverKey is the primary driver this entity's per-unit metrics were
	// computed against (Policy.PrimaryDriverKey for this entity's
	// Dimension), empty if none was configured or none was available for
	// this entity/period.
	DriverKey           string `json:"driver_key,omitempty"`
	DriverUnits         Value  `json:"driver_units"`
	NetRevenuePerUnit   Value  `json:"net_revenue_per_unit"`
	DirectCostPerUnit   Value  `json:"direct_cost_per_unit"`
	GrossProfitPerUnit  Value  `json:"gross_profit_per_unit"`
	ContributionPerUnit Value  `json:"contribution_per_unit"`
}

// buildPerUnitMetrics computes PerUnitMetrics for one entity/period given
// its resolved driver total (if any) and bridge totals.
func buildPerUnitMetrics(driverKey string, driverUnits Value, netRevenue, directCostTotal, grossProfit, contribution float64, revenueAvailable, directCostAvailable, contributionAvailable bool) PerUnitMetrics {
	m := PerUnitMetrics{DriverKey: driverKey, DriverUnits: driverUnits}
	if !driverUnits.Available || driverUnits.Amount == 0 {
		return m
	}
	if revenueAvailable {
		m.NetRevenuePerUnit = AvailableValue(netRevenue / driverUnits.Amount)
	}
	if directCostAvailable {
		m.DirectCostPerUnit = AvailableValue(directCostTotal / driverUnits.Amount)
	}
	if revenueAvailable && directCostAvailable {
		m.GrossProfitPerUnit = AvailableValue(grossProfit / driverUnits.Amount)
	}
	if contributionAvailable {
		m.ContributionPerUnit = AvailableValue(contribution / driverUnits.Amount)
	}
	return m
}
