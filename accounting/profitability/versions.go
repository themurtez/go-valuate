package profitability

// SchemaVersion identifies this package's public result/schema contract:
// the shape of Entity, Fact, Attribution, PeriodInfo,
// EntityPeriodSummaryInput, DriverObservation, SharedCostPool,
// AllocationRule, ControlTotals, Policy, EntityPeriodResult,
// DimensionView, BusinessTotals, Flag, Issue, Coverage, and Result. Bump
// whenever any of that shape changes in a way that could make a
// historical persisted Result not reproduce identically under new code —
// see the repository README's versioning-strategy section, which every
// sibling package's SchemaVersion already follows.
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed calculation semantics:
// the profitability bridge (bridge.go), attribution resolution
// (attribution.go), allocation math (allocation.go), coverage
// (coverage.go), rankings (rankings.go), trend/margin-leakage
// (trend.go), and every flag-trigger rule (computeflags.go). Bump this
// whenever any of that changes in a way that could make a historical
// Result not reproduce identically under new code.
const FormulaVersion = "1.0.0"
