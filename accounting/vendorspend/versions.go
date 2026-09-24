package vendorspend

// SchemaVersion identifies this package's fixed data shapes: Supplier,
// Period, SpendRecord, Policy, DependencyMetadata, ControlTotals, and
// every Result sub-shape. Bump this whenever a field is added, removed,
// or its meaning changes in a way that could break a caller's stored
// JSON — see the repository README's versioning-strategy section, which
// this constant follows exactly (financial.SchemaVersion, ap.SchemaVersion,
// inventory.SchemaVersion, profitability.SchemaVersion, etc.).
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed formula set: the
// gross/credit/net bridge, supplier/category/department/location/cost-
// center summaries, concentration (via analytics/concentration reuse),
// new/lost-supplier detection, growth/share-trend calculation, unit-price
// and price-volume decomposition, recurrence/commitment mix, tail-spend,
// duplicate-like detection, control reconciliation, coverage, and every
// flag-trigger rule. Bump this whenever any of that changes in a way that
// could make a historical Result not reproduce identically under new
// code.
const FormulaVersion = "1.0.0"
