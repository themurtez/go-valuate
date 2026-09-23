// Package inventory implements deterministic inventory analytics for
// accountants, controllers, CFOs, and operating/business-advisory
// workflows: on-hand quantity/value, turnover and Days Inventory
// Outstanding (DIO), value aging and slow/non-moving detection, last-
// movement and velocity metrics, stock-level (min/target/max/reorder)
// comparison, inventory value concentration, purchases-vs-usage trends,
// adjustment/write-off review, expiry review, GL/subledger reconciliation,
// and quantity/value rollforwards — from a portable, caller-supplied
// inventory-facts model.
//
// # This is not an ERP, WMS, costing engine, or tax-inventory system
//
// This package never posts inventory transactions, manages warehouse
// operations, performs barcode/scanning, creates purchase orders, executes
// replenishment, forecasts demand, computes an economic order quantity,
// runs a FIFO/LIFO/specific-identification costing engine, renders a
// GAAP/IFRS write-down opinion, posts an automatic journal entry, or
// calculates product profitability (see docs/INVENTORY_ANALYTICS.md's
// "Explicit non-goals" section and the main README's [What this project
// intentionally does not
// contain](../../README.md#what-this-project-intentionally-does-not-contain)).
// Every quantity, cost, and value figure is a fact the caller supplies or a
// simple, documented arithmetic combination of caller-supplied facts; this
// package never invents a cost, a demand estimate, or a shelf life.
//
// # Two convergent input levels
//
// A caller may supply detailed SKU-level facts (Item + InventorySnapshot +
// Movement records) or already-aggregated period summary data (beginning/
// ending inventory, COGS, purchases, units) via PeriodFinancials — see
// Input's doc comment. Both paths converge on the same PeriodSummary shape
// wherever the underlying semantics are equivalent (turnover, DIO,
// purchases-vs-usage); item/SKU-level detail (aging, slow-moving,
// concentration, stock policy) is naturally only available from the
// detailed path, since it has no period-summary equivalent.
//
// # Costing boundary
//
// This package never derives an authoritative FIFO/LIFO/specific-
// identification cost basis from raw purchase/shipment history. Every
// InventorySnapshot.UnitCost, Movement.UnitCost/Amount, and
// PeriodFinancials.COGS is caller-supplied. Accounting valuation method
// remains the caller/source-system's responsibility — see docs/
// INVENTORY_ANALYTICS.md's "Costing boundary" section.
//
// # Relationship to sibling packages
//
// This package mirrors accounting/ar and accounting/ap's shape in spirit
// (an open-item-like analytics engine with aging, concentration, and GL
// reconciliation) but shares no domain model with either — an inventory
// item is not a receivable/payable. It is independent of accounting/ledger
// and accounting/statements: this package never imports either, and
// integration (subledger-to-GL-control reconciliation, statement-builder
// inventory balance, working-capital component, ratios DIO) is
// demonstrated only via portable typed input and adapter tests — see
// statementsadapter_test.go, workingcapitaladapter_test.go, and
// ratiosadapter_test.go. It reuses analytics/concentration directly for
// value-concentration share/HHI math (see concentration.go) since that
// package's Observation tuple already fits this package's "value by
// item/category/location" need exactly, the same way accounting/ar and
// accounting/ap already reuse it.
//
// # No hidden current-date dependency
//
// Every snapshot/aging calculation takes an explicit AsOfDate from caller
// input. Every historical calculation takes explicit, caller-ordered
// PeriodInfo values. Nothing in this package calls time.Now() — essential
// for reproducibility and for building historical analyses from any date
// — see determinism_test.go.
//
// # Purity, immutability, and determinism
//
// Every exported function is pure: no I/O, no mutation of caller-owned
// input (Item, InventorySnapshot, Movement, StockPolicy, PeriodInfo,
// PeriodFinancials, GLControl, Policy, Dimension, and every slice/map they
// appear in are never modified in place — see immutability_test.go), no
// package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package inventory

import "math"

// isNonFinite reports whether v is NaN or +/-Inf — the shared guard every
// quantity/cost/value-bearing field in this package is checked against.
func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}

// amountTolerance is the floating-point comparison tolerance used
// throughout this package's value validation and reconciliation, matching
// accounting/ar's, accounting/ap's, and accounting/labor's identical
// money-comparison tolerance.
const amountTolerance = 0.005

// quantityTolerance is the floating-point comparison tolerance used for
// quantity (unit) validation and rollforward reconciliation. Distinct from
// amountTolerance because quantities and monetary values are different
// units of measure with no shared "reasonable rounding" magnitude.
const quantityTolerance = 0.0005

// Value represents a single monetary or ratio figure that may or may not
// be available, distinguishing "computed/reported to be exactly 0" from
// "unknown because a required input was absent." This package's own local
// copy of the convention every sibling package in this repository
// duplicates rather than importing another package's Value — see
// labor.Value's doc comment for the full rationale (each package's Value
// is a distinct Go type serving that package's own JSON contract; sharing
// one across packages would couple their independent schema-versioning
// stories).
type Value struct {
	Available bool    `json:"available"`
	Amount    float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may
// legitimately be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// Qty represents a single quantity figure that may or may not be
// available, carrying its own UnitOfMeasure so a consumer never mistakes
// an aggregated quantity for one expressed in a specific, meaningful unit.
// Distinct from Value (money) because quantities from incompatible units
// of measure must never be summed — see UOM safety in the package doc and
// uom.go.
type Qty struct {
	Available bool    `json:"available"`
	Amount    float64 `json:"amount"`
	// UnitOfMeasure is the unit Amount is expressed in. Empty when
	// Available is false, or when Amount aggregates a single item whose
	// own UnitOfMeasure was not supplied.
	UnitOfMeasure string `json:"unit_of_measure,omitempty"`
}

// UnavailableQty is the canonical zero-information Qty.
func UnavailableQty() Qty { return Qty{} }

// AvailableQty reports a Qty for a known quantity in the given unit of
// measure.
func AvailableQty(v float64, uom string) Qty {
	return Qty{Available: true, Amount: v, UnitOfMeasure: uom}
}

// SourceRef is an opaque, caller-defined pointer back to the originating
// system record (e.g. an inventory-management-system SKU key or a
// warehouse-management-system movement ID). This package never interprets
// it — mirrors ar.SourceRef/ap.SourceRef/labor.SourceRef's identical
// "opaque reference" convention.
type SourceRef struct {
	System string `json:"system,omitempty"`
	ID     string `json:"id,omitempty"`
}

// Dimension is one lightweight, optional analysis tag — mirrors
// ar.Dimension/ap.Dimension/ledger.Dimension's identical rationale: Key is
// an open string rather than a closed enum, since this package never
// hard-codes a dimension taxonomy.
type Dimension struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Conventional Dimension.Key values. These are suggestions, not a closed
// set.
const (
	DimensionLocation     string = "location"
	DimensionCategory     string = "category"
	DimensionBusinessUnit string = "business_unit"
	DimensionRegion       string = "region"
)
