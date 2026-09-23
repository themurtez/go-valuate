package ap

// SchemaVersion identifies this package's public result/schema contract:
// the shape of Payable, SupplierPayment, BucketDefinition, SupplierSummary,
// Result, and Issue. Bump whenever any of that shape changes in a way that
// could make a historical persisted Result not reproduce identically under
// new code — see the repository README's versioning-strategy section,
// which every sibling package's SchemaVersion (ar.SchemaVersion,
// ledger.SchemaVersion, statements.SchemaVersion, ...) already follows.
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed calculation semantics:
// aging-basis/bucket assignment (buckets.go), DPO (dpo.go), historical
// trend and migration (trends.go), payment/terms metrics (payments.go),
// concentration (concentration.go), due-date schedule and payment pressure
// (schedule.go), and every flag-trigger rule (flags.go). Bump this
// whenever any of that changes in a way that could make a historical
// Result not reproduce identically under new code.
const FormulaVersion = "1.0.0"
