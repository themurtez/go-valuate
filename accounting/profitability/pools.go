package profitability

// SharedCostPool is one caller-declared pool of overhead/shared cost for
// a period — task section 20. This package never infers a pool from
// expense data; a caller who wants "G&A" or "shared facilities cost"
// allocated must supply it explicitly here.
type SharedCostPool struct {
	// PoolID uniquely identifies this pool within one analysis. Required;
	// duplicates are flagged (see IssueDuplicatePool) and only the first
	// occurrence (input order) is used.
	PoolID string `json:"pool_id"`
	Period string `json:"period"`
	// Amount is this pool's non-negative total cost for Period.
	Amount      float64 `json:"amount"`
	Category    string  `json:"category,omitempty"`
	Description string  `json:"description,omitempty"`

	SourceRef SourceRef `json:"source_ref,omitempty"`
}

// AllocationBasis is how a SharedCostPool is distributed across a
// Dimension's entities — task section 22. There is no default basis (see
// the task's "no hidden default basis" instruction); a caller must pick
// one explicitly per (PoolID, Dimension) via AllocationRule.
type AllocationBasis string

const (
	// AllocationBasisNetRevenue allocates in proportion to each entity's
	// Net Revenue for the pool's Period.
	AllocationBasisNetRevenue AllocationBasis = "NET_REVENUE"
	// AllocationBasisDirectCost allocates in proportion to each entity's
	// total direct cost for the pool's Period.
	AllocationBasisDirectCost AllocationBasis = "DIRECT_COST"
	// AllocationBasisDirectLaborCost allocates in proportion to each
	// entity's DIRECT_LABOR cost for the pool's Period.
	AllocationBasisDirectLaborCost AllocationBasis = "DIRECT_LABOR_COST"
	// AllocationBasisDriver allocates in proportion to each entity's
	// DriverObservation.Value for AllocationRule.DriverKey and the pool's
	// Period.
	AllocationBasisDriver AllocationBasis = "DRIVER"
	// AllocationBasisFixedWeight allocates using AllocationRule's
	// explicit FixedWeights, ignoring any computed entity amount.
	AllocationBasisFixedWeight AllocationBasis = "FIXED_WEIGHT"
	// AllocationBasisEqual splits the pool equally across every entity
	// referenced by AllocationRule.FixedWeights (weights ignored, only
	// EntityID membership used) — or, if FixedWeights is empty, across
	// every entity in Dimension with a computed EntityPeriodResult for
	// the pool's Period.
	AllocationBasisEqual AllocationBasis = "EQUAL"
)

func isRecognizedAllocationBasis(b AllocationBasis) bool {
	switch b {
	case AllocationBasisNetRevenue, AllocationBasisDirectCost, AllocationBasisDirectLaborCost,
		AllocationBasisDriver, AllocationBasisFixedWeight, AllocationBasisEqual:
		return true
	default:
		return false
	}
}

// AllocationWeight is one entity's explicit fixed weight for
// AllocationBasisFixedWeight (or membership marker for
// AllocationBasisEqual) — task section 22.
type AllocationWeight struct {
	EntityID string  `json:"entity_id"`
	Weight   float64 `json:"weight"`
}

// AllocationRule declares how one SharedCostPool is distributed across
// one Dimension's entities — task section 22. The same PoolID may carry
// separate rules for CUSTOMER, JOB, and PRODUCT independently — task
// section 23; each is an independent analytical allocation and this
// package never aggregates allocated costs across dimension views.
type AllocationRule struct {
	PoolID    string          `json:"pool_id"`
	Dimension Dimension       `json:"dimension"`
	Basis     AllocationBasis `json:"basis"`
	// DriverKey is required (and only meaningful) when Basis ==
	// AllocationBasisDriver.
	DriverKey string `json:"driver_key,omitempty"`
	// FixedWeights is required when Basis == AllocationBasisFixedWeight,
	// and optionally restricts AllocationBasisEqual's equal-split
	// membership to just these EntityIDs (weights themselves ignored in
	// that case).
	FixedWeights []AllocationWeight `json:"fixed_weights,omitempty"`
}

// ruleKey identifies one AllocationRule's (pool, dimension) slot — task
// section 24 "no duplicate pool/dimension rule."
type ruleKey struct {
	PoolID    string
	Dimension Dimension
}

// AllocationTraceEntry preserves one pool/dimension/entity allocation
// computation — task section 26. Always present (even for an
// entity that received $0) whenever a pool was successfully allocated for
// that dimension/period, so a caller can audit exactly how each entity's
// AllocatedSharedCosts figure was derived.
type AllocationTraceEntry struct {
	PoolID    string          `json:"pool_id"`
	Dimension Dimension       `json:"dimension"`
	Basis     AllocationBasis `json:"basis"`
	DriverKey string          `json:"driver_key,omitempty"`
	EntityID  string          `json:"entity_id"`
	Period    string          `json:"period"`
	// DriverAmount is the entity's raw basis amount used for this
	// allocation (net revenue, direct cost, direct labor cost, driver
	// value, or fixed weight) before conversion to a share.
	DriverAmount float64 `json:"driver_amount"`
	// AllocationShare is DriverAmount / sum(DriverAmount across entities)
	// for this pool/dimension/period — i.e. this entity's fraction of the
	// allocated total.
	AllocationShare float64 `json:"allocation_share"`
	AllocatedAmount float64 `json:"allocated_amount"`
}

// PoolAllocationStatus reports why a pool/dimension/period was or was not
// allocated — task sections 21, 25, 54.
type PoolAllocationStatus string

const (
	// PoolStatusNotAllocated means the caller supplied no AllocationRule
	// for this (PoolID, Dimension) — allocation is opt-in, task section
	// 21 — the pool's Amount is reported fully as unallocated for this
	// dimension/period, with no issue (this is expected/default
	// behavior, not a failure).
	PoolStatusNotAllocated PoolAllocationStatus = "NOT_ALLOCATED"
	// PoolStatusAllocated means every entity's share was resolved and the
	// pool's Amount was fully distributed (Allocated + Unallocated ==
	// PoolAmount within tolerance, Unallocated ~= 0).
	PoolStatusAllocated PoolAllocationStatus = "ALLOCATED"
	// PoolStatusDenominatorUnavailable means an AllocationRule existed
	// for this (PoolID, Dimension) but its basis denominator was zero or
	// unavailable (e.g. AllocationBasisNetRevenue with total net revenue
	// == 0 across every referenced entity) — the pool is left fully
	// unallocated for this dimension/period and a structured Issue is
	// recorded (IssueAllocationDenominatorUnavailable). This package
	// never falls back to equal allocation in this case — task section
	// 25.
	PoolStatusDenominatorUnavailable PoolAllocationStatus = "DENOMINATOR_UNAVAILABLE"
)

// PoolAllocationResult is one SharedCostPool's allocation outcome for one
// Dimension and Period — task sections 26-27.
type PoolAllocationResult struct {
	PoolID            string                 `json:"pool_id"`
	Dimension         Dimension              `json:"dimension"`
	Period            string                 `json:"period"`
	Status            PoolAllocationStatus   `json:"status"`
	PoolAmount        float64                `json:"pool_amount"`
	AllocatedAmount   float64                `json:"allocated_amount"`
	UnallocatedAmount float64                `json:"unallocated_amount"`
	Trace             []AllocationTraceEntry `json:"trace,omitempty"`
}
