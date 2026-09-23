package inventory

// InventoryClass is a caller-defined manufacturing/merchandising stage
// tag — task section 46. Purely a caller-supplied label: this package
// never infers a class from Item.Name or SKU.
type InventoryClass string

const (
	ClassRawMaterial  InventoryClass = "RAW_MATERIAL"
	ClassWIP          InventoryClass = "WIP"
	ClassFinishedGood InventoryClass = "FINISHED_GOOD"
	ClassMerchandise  InventoryClass = "MERCHANDISE"
	ClassSupplies     InventoryClass = "SUPPLIES"
	ClassOther        InventoryClass = "OTHER"
)

// Item is one portable inventory item/SKU record. ItemID is this
// package's identity key throughout — SKU and Name are display/reference
// fields only, never used for matching or business-meaning inference (task
// section 3: "do not infer business meaning from names").
type Item struct {
	// ID uniquely identifies this item within one analysis. Required;
	// duplicates are flagged (see IssueDuplicateItem) and only the first
	// occurrence (in input order) is used.
	ID string `json:"id"`
	// SKU is an optional human-readable stock-keeping-unit code, carried
	// through for display only.
	SKU string `json:"sku,omitempty"`
	// Name is an optional human-readable label, carried through for
	// display only. Never used as an identity key or to infer Category/
	// Class.
	Name string `json:"name,omitempty"`

	Category    string `json:"category,omitempty"`
	Subcategory string `json:"subcategory,omitempty"`
	// Class is this item's optional inventory-composition class — see
	// InventoryClass. Empty means unclassified; unclassified items are
	// still fully analyzed everywhere except CompositionSummary's
	// by-class breakdown.
	Class InventoryClass `json:"class,omitempty"`

	// UnitOfMeasure is this item's unit of measure (e.g. "EA", "KG", "L").
	// Used for UOM-safety checks — see uom.go. Optional but recommended;
	// an item with no UnitOfMeasure can still be valued but never
	// contributes to a compatible-UOM quantity aggregate.
	UnitOfMeasure string `json:"unit_of_measure,omitempty"`

	// Active is a caller-supplied lifecycle flag, carried through for
	// display/filtering only. This package never excludes an inactive
	// item from analysis on its own — see PortfolioSummary.ActiveItemCount
	// for how it is surfaced.
	Active bool `json:"active"`
	// Currency is this item's reporting currency for cost/value fields.
	// Required for value calculations; a mix of currencies within one
	// analysis is flagged unless resolved — see Options.ReportingCurrency
	// (via Policy) and IssueMixedCurrency.
	Currency string `json:"currency,omitempty"`

	Location string `json:"location,omitempty"`

	SourceRef SourceRef `json:"source_ref,omitempty"`
}
