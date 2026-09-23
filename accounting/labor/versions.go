package labor

// SchemaVersion identifies this package's public result/schema contract:
// the shape of Worker, PayrollRecord, ContractorLaborRecord, PeriodInfo,
// BusinessMetrics, GLPayrollControl, Policy, PeriodSummary, GroupSummary,
// TrendResult, ReconciliationSummary, Flag, Issue, Coverage, and Result.
// Bump whenever any of that shape changes in a way that could make a
// historical persisted Result not reproduce identically under new code —
// see the repository README's versioning-strategy section, which every
// sibling package's SchemaVersion (ar.SchemaVersion, ap.SchemaVersion,
// cashforecast.SchemaVersion, closequality.SchemaVersion, ...) already
// follows.
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed calculation semantics:
// the labor-cost bridge (bridge.go), cash-vs-expense basis views
// (basis.go), headcount/FTE (headcount.go/fte.go), overtime analytics
// (overtime.go), contractor mix (contractor.go), direct/indirect labor
// split, department/location/cost-center grouping (grouping.go),
// productivity ratios (productivity.go), labor-efficiency and
// labor-cost-vs-revenue trend (trend.go), workforce movement/turnover
// (workforce.go), worker-date consistency (workforcedates.go), duplicate
// detection (duplicates.go), payroll burden, payroll-vs-GL reconciliation
// (reconcile.go), and every flag-trigger rule (flags.go/computeflags.go).
// Bump this whenever any of that changes in a way that could make a
// historical Result not reproduce identically under new code.
const FormulaVersion = "1.0.0"
