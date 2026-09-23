package inventory

// SchemaVersion identifies this package's public result/schema contract:
// the shape of Item, InventorySnapshot, Movement, StockPolicy, PeriodInfo,
// PeriodFinancials, GLControl, Policy, PeriodSummary, ItemSummary,
// AgingSummary, Reconciliation, Result, Flag, and Issue. Bump whenever any
// of that shape changes in a way that could make a historical persisted
// Result not reproduce identically under new code — see the repository
// README's versioning-strategy section, which every sibling package's
// SchemaVersion (ar.SchemaVersion, ap.SchemaVersion, labor.SchemaVersion,
// ...) already follows.
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed calculation semantics:
// value/quantity resolution (value.go), turnover/DIO (turnover.go), aging
// buckets and slow/non-moving thresholds (aging.go), last-movement/
// velocity (movement.go/velocity.go), stock-policy comparison
// (stockpolicy.go), concentration (concentration.go), purchases-vs-usage
// and inventory-build signals (purchaseusage.go), adjustment/write-off
// review (adjustments.go), expiry review (expiry.go), reconciliation and
// rollforwards (reconcile.go), and every flag-trigger rule (flags.go).
// Bump this whenever any of that changes in a way that could make a
// historical Result not reproduce identically under new code.
const FormulaVersion = "1.0.0"
