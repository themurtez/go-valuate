// Package vendorspend implements deterministic vendor/supplier spend
// analytics from a portable purchase-record model.
//
// This package is application-independent, like every analytics-style
// package in this repository: it does not require accounting/ap,
// accounting/inventory, accounting/labor, accounting/ledger,
// accounting/statements, or analytics/* input at all. A caller populates
// it directly from an AP/ERP purchase export, a homegrown procurement
// system, or a synthetic fixture — see accounting/vendorspend/fixtures.
// Portable typed adapters demonstrate integration with those sibling
// packages via tests only (*_adapter_test.go); this package's core types
// never import any of them.
//
// # Distinct from accounting/ap
//
// accounting/ap answers "what do we currently owe suppliers" (an
// open-item, balance-as-of-a-date question). This package answers "what
// do we buy from suppliers, over time" (an economic period-spend
// question, driven by purchase/receipt/accrual/cash events, not by
// outstanding-balance snapshots). A supplier can have $0 ending AP and
// still be this business's largest vendor by annual spend, or vice
// versa — see docs/VENDOR_SPEND_ANALYTICS.md's "AP vs. spend" section
// and ap_boundary_test.go's permanent regression test. This package never
// treats an accounting/ap balance as a substitute for spend, and never
// asserts period spend equals ending AP.
//
// # No hidden current-date dependency
//
// Every computation takes explicit caller-supplied Periods with explicit
// start/end dates. Nothing in this package calls time.Now() — essential
// for reproducibility.
//
// Every exported function is pure: no I/O, no mutation of caller-owned
// input (Supplier, SpendRecord, Period, and every slice/map they appear
// in are never modified in place — see immutability_test.go), no
// package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package vendorspend

import "time"

// SourceRef is an opaque, caller-defined pointer back to the originating
// system record (e.g. an ERP purchase-order line or an AP bill-line key).
// This package never interprets it — mirrors accounting/ap.SourceRef's
// identical "opaque reference" convention.
type SourceRef struct {
	System string `json:"system,omitempty"`
	ID     string `json:"id,omitempty"`
}

// Value represents a figure that may or may not be calculable, mirroring
// accounting/ap.AmountValue/analytics/concentration.ConcentrationValue's
// identical availability convention: Available distinguishes "computed to
// be exactly 0" from "cannot be computed because a required input is
// absent" (e.g. a weighted-average unit price when no compatible
// quantity/UOM observation exists).
type Value struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a successfully computed figure.
func AvailableValue(v float64) Value {
	return Value{Available: true, Value: v}
}

// DependencyClass is a caller-supplied, factual operational-dependency
// label for one Supplier — never inferred from spend share or any other
// computed figure (task section 10's explicit "do not infer these labels
// from spend share" rule). This package only ever echoes the caller's own
// classification back in DependencySummary; it computes no risk score of
// its own from it.
type DependencyClass string

const (
	DependencyCritical     DependencyClass = "CRITICAL"
	DependencyStrategic    DependencyClass = "STRATEGIC"
	DependencySingleSource DependencyClass = "SINGLE_SOURCE"
	DependencyReplaceable  DependencyClass = "REPLACEABLE"
	DependencyUnknown      DependencyClass = "UNKNOWN"
)

// isRecognizedDependencyClass reports whether d is one of the fixed
// DependencyClass values (including the explicit UNKNOWN member).
func isRecognizedDependencyClass(d DependencyClass) bool {
	switch d {
	case DependencyCritical, DependencyStrategic, DependencySingleSource, DependencyReplaceable, DependencyUnknown:
		return true
	default:
		return false
	}
}

// Supplier is one portable supplier/vendor master record — task section
// 1. SupplierID is this package's identity key throughout; Name is
// display-only and never used for matching or to infer any relationship.
type Supplier struct {
	// SupplierID uniquely identifies this supplier within one analysis.
	// Required; duplicates are flagged (IssueDuplicateSupplier) and only
	// the first occurrence (input order) is used.
	SupplierID string `json:"supplier_id"`
	// Name is an optional human-readable label, carried through for
	// display only. Never used as an identity key.
	Name string `json:"name,omitempty"`
	// Category is a caller-assigned grouping label (e.g. "Raw Materials,"
	// "IT Services"), independent of any SpendRecord.Category value — a
	// caller may set either or both; neither is inferred from the other.
	Category string `json:"category,omitempty"`
	// Country is an optional ISO 3166-style country code, carried through
	// for display/grouping only.
	Country string `json:"country,omitempty"`
	// Active is the caller's own current-status flag, never inferred by
	// this package.
	Active bool `json:"active"`
	// ParentID optionally references another Supplier.SupplierID for a
	// caller-declared parent-company relationship. This package never
	// infers parent/child relationships from Name (task section 1's
	// explicit "do not infer parent-company relationships from names"
	// rule) and performs no automatic parent-child spend rollup — it is
	// carried through and validated for cycles only (see
	// IssueInvalidSupplierParent).
	ParentID string `json:"parent_id,omitempty"`
	// Dependency is a caller-supplied, factual operational-dependency
	// label — see DependencyClass. Zero value ("") is treated as
	// DependencyUnknown.
	Dependency DependencyClass `json:"dependency,omitempty"`
	// PreferredSupplier and ContractedSupplier are caller-supplied policy
	// flags used only by SPEND_WITH_NON_PREFERRED_SUPPLIER/
	// SPEND_OUTSIDE_CONTRACTED_SUPPLIERS (task section 23). Never inferred.
	PreferredSupplier  bool `json:"preferred_supplier,omitempty"`
	ContractedSupplier bool `json:"contracted_supplier,omitempty"`

	SourceRef SourceRef `json:"source_ref,omitempty"`
}

// resolvedDependency returns s.Dependency if recognized, otherwise
// DependencyUnknown.
func (s Supplier) resolvedDependency() DependencyClass {
	if isRecognizedDependencyClass(s.Dependency) && s.Dependency != "" {
		return s.Dependency
	}
	return DependencyUnknown
}

// Period is one caller-supplied explicit chronological period boundary —
// task section 2. This package never infers a fiscal calendar and never
// calls time.Now().
type Period struct {
	// Period is this period's label (e.g. "2025-06", "2025-Q2"), matching
	// SpendRecord.Period's convention.
	Period    string    `json:"period"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	// SequenceInYear orders periods for chronological sorting when two
	// periods could otherwise tie (e.g. StartDate equal). Optional; when
	// zero for every period, StartDate ordering alone is used.
	SequenceInYear int `json:"sequence_in_year,omitempty"`
}

// Valid reports whether p's StartDate/EndDate are both set and consistent
// (StartDate <= EndDate) and Period is non-empty.
func (p Period) Valid() bool {
	if p.Period == "" || p.StartDate.IsZero() || p.EndDate.IsZero() {
		return false
	}
	return !p.EndDate.Before(p.StartDate)
}

// SpendType is a controlled, high-level economic-purchase taxonomy — task
// section 4. This package deliberately does not attempt a chart-of-
// accounts replacement; finer subcategorization is the caller's own
// Category/Subcategory strings.
type SpendType string

const (
	SpendTypeGoods               SpendType = "GOODS"
	SpendTypeInventory           SpendType = "INVENTORY"
	SpendTypeRawMaterial         SpendType = "RAW_MATERIAL"
	SpendTypeSubcontractor       SpendType = "SUBCONTRACTOR"
	SpendTypeProfessionalService SpendType = "PROFESSIONAL_SERVICE"
	SpendTypeSoftware            SpendType = "SOFTWARE"
	SpendTypeRent                SpendType = "RENT"
	SpendTypeUtilities           SpendType = "UTILITIES"
	SpendTypeMarketing           SpendType = "MARKETING"
	SpendTypeLogistics           SpendType = "LOGISTICS"
	SpendTypeInsurance           SpendType = "INSURANCE"
	SpendTypeOtherOperating      SpendType = "OTHER_OPERATING"
	SpendTypeCapex               SpendType = "CAPEX"
	SpendTypeOther               SpendType = "OTHER"
)

func isRecognizedSpendType(t SpendType) bool {
	switch t {
	case SpendTypeGoods, SpendTypeInventory, SpendTypeRawMaterial, SpendTypeSubcontractor,
		SpendTypeProfessionalService, SpendTypeSoftware, SpendTypeRent, SpendTypeUtilities,
		SpendTypeMarketing, SpendTypeLogistics, SpendTypeInsurance, SpendTypeOtherOperating,
		SpendTypeCapex, SpendTypeOther:
		return true
	default:
		return false
	}
}

// SpendEffect distinguishes an ordinary purchase from a credit/refund/
// reversal against prior spend — task section 5. Amount is expected
// non-negative regardless of Effect; Effect (not Amount's sign) is what
// determines whether a record adds to or reduces gross/net spend. This
// mirrors accounting/ap.DocumentType's identical "type, not sign,
// determines semantics" convention, extended to three reducing kinds
// instead of one.
type SpendEffect string

const (
	EffectNormal   SpendEffect = "NORMAL"
	EffectCredit   SpendEffect = "CREDIT"
	EffectRefund   SpendEffect = "REFUND"
	EffectReversal SpendEffect = "REVERSAL"
)

func isRecognizedEffect(e SpendEffect) bool {
	switch e {
	case EffectNormal, EffectCredit, EffectRefund, EffectReversal:
		return true
	default:
		return false
	}
}

// isReducingEffect reports whether e reduces net spend (CREDIT/REFUND/
// REVERSAL), as opposed to EffectNormal which adds to it.
func isReducingEffect(e SpendEffect) bool {
	switch e {
	case EffectCredit, EffectRefund, EffectReversal:
		return true
	default:
		return false
	}
}

// SpendBasis is the explicit economic basis a SpendRecord was recognized
// under — task section 6. This package never silently mixes bases; see
// IssueMixedSpendBasis and Result.SpendByBasis.
type SpendBasis string

const (
	BasisAccrual  SpendBasis = "ACCRUAL"
	BasisPurchase SpendBasis = "PURCHASE"
	BasisReceipt  SpendBasis = "RECEIPT"
	BasisCash     SpendBasis = "CASH"
	BasisUnknown  SpendBasis = "UNKNOWN"
)

func isRecognizedBasis(b SpendBasis) bool {
	switch b {
	case BasisAccrual, BasisPurchase, BasisReceipt, BasisCash, BasisUnknown:
		return true
	default:
		return false
	}
}

func resolvedBasis(b SpendBasis) SpendBasis {
	if isRecognizedBasis(b) && b != "" {
		return b
	}
	return BasisUnknown
}

// RecurrenceType is a caller-declared recurrence classification — task
// section 19. Never overwritten by this package's own
// FlagObservedRepeatedSpend pattern detection, which is reported
// separately (see RecurringSpend).
type RecurrenceType string

const (
	RecurrenceRecurring RecurrenceType = "RECURRING"
	RecurrenceOneTime   RecurrenceType = "ONE_TIME"
	RecurrenceIrregular RecurrenceType = "IRREGULAR"
	RecurrenceUnknown   RecurrenceType = "UNKNOWN"
)

func isRecognizedRecurrence(r RecurrenceType) bool {
	switch r {
	case RecurrenceRecurring, RecurrenceOneTime, RecurrenceIrregular, RecurrenceUnknown:
		return true
	default:
		return false
	}
}

func resolvedRecurrence(r RecurrenceType) RecurrenceType {
	if isRecognizedRecurrence(r) && r != "" {
		return r
	}
	return RecurrenceUnknown
}

// CommitmentType is a caller-declared committed-vs-discretionary
// classification — task section 20. Never inferred from SpendType.
type CommitmentType string

const (
	CommitmentCommitted     CommitmentType = "COMMITTED"
	CommitmentDeferrable    CommitmentType = "DEFERRABLE"
	CommitmentDiscretionary CommitmentType = "DISCRETIONARY"
	CommitmentUnknown       CommitmentType = "UNKNOWN"
)

func isRecognizedCommitment(c CommitmentType) bool {
	switch c {
	case CommitmentCommitted, CommitmentDeferrable, CommitmentDiscretionary, CommitmentUnknown:
		return true
	default:
		return false
	}
}

func resolvedCommitment(c CommitmentType) CommitmentType {
	if isRecognizedCommitment(c) && c != "" {
		return c
	}
	return CommitmentUnknown
}

// SpendRecord is one economic spend record — the primary input to this
// package (task section 3). A caller assembles a slice of these from
// whatever purchasing/AP source it has; this package has no opinion on
// where it came from.
type SpendRecord struct {
	// SpendID uniquely identifies this record within one analysis.
	// Required; duplicate IDs are an input issue (IssueDuplicateSpend,
	// task section 22) and only the first occurrence (input order) is
	// used.
	SpendID string `json:"spend_id"`
	// SupplierID identifies the supplier this spend was with. Required;
	// must reference a Supplier in Input.Suppliers (see
	// IssueUnknownSupplier).
	SupplierID string `json:"supplier_id"`
	// Period is the reporting period this spend falls in, matching a
	// Period.Period label in Input.Periods.
	Period string `json:"period"`
	// Date is this record's economic date (its meaning depends on Basis —
	// e.g. the accrual date, the purchase-order date, the goods-receipt
	// date, or the cash-payment date). Required.
	Date time.Time `json:"date"`

	// Amount is this record's economic amount. Expected non-negative
	// regardless of Effect — see SpendEffect's doc comment for why Effect,
	// not Amount's sign, determines gross/net semantics. A negative
	// Amount is flagged (IssueNegativeAmount) but not by itself excluded.
	Amount float64 `json:"amount"`
	// Currency is this record's ISO 4217-style currency code. Required;
	// a mix of currencies within one analysis is flagged unless resolved
	// — see Options.ReportingCurrency and IssueMixedCurrency.
	Currency string `json:"currency"`

	// Category/Subcategory are caller-assigned grouping labels, plain
	// strings (mirroring financial.Code/accounting/ap.Dimension's
	// identical "never hard-code the taxonomy" rationale) rather than a
	// closed enum — orthogonal to SpendType, which IS a closed, coarse
	// taxonomy (task section 4).
	Category    string `json:"category,omitempty"`
	Subcategory string `json:"subcategory,omitempty"`

	// Quantity/UnitPrice/UnitOfMeasure support unit-price analytics (task
	// sections 8, 14-18). All three optional; UnitPriceAnalysis never
	// compares or aggregates across incompatible UnitOfMeasure values
	// (task section 14 — "no inferred UOM conversion").
	Quantity      Value  `json:"quantity"`
	UnitPrice     Value  `json:"unit_price"`
	UnitOfMeasure string `json:"unit_of_measure,omitempty"`

	Description string `json:"description,omitempty"`
	ReferenceID string `json:"reference_id,omitempty"`

	SpendType  SpendType      `json:"spend_type,omitempty"`
	Effect     SpendEffect    `json:"effect,omitempty"`
	Recurrence RecurrenceType `json:"recurrence,omitempty"`
	Commitment CommitmentType `json:"commitment,omitempty"`
	Basis      SpendBasis     `json:"basis,omitempty"`

	// ProductID, when supplied, identifies a specific caller-defined
	// product/SKU for cross-supplier price comparison and observed-
	// single-source analysis (task sections 16-17). Never inferred from
	// Description (task section 16's explicit "do not infer product
	// identity from descriptions" rule).
	ProductID string `json:"product_id,omitempty"`

	Location   string `json:"location,omitempty"`
	Department string `json:"department,omitempty"`
	CostCenter string `json:"cost_center,omitempty"`

	SourceRef SourceRef `json:"source_ref,omitempty"`
}

// resolvedSpendType returns r.SpendType if recognized, otherwise
// SpendTypeOther.
func (r SpendRecord) resolvedSpendType() SpendType {
	if isRecognizedSpendType(r.SpendType) && r.SpendType != "" {
		return r.SpendType
	}
	return SpendTypeOther
}

// resolvedEffect returns r.Effect if recognized, otherwise EffectNormal —
// the safe default for a caller's existing purchase data that predates
// this field, mirroring accounting/ap.resolvedDocumentType's identical
// convention.
func (r SpendRecord) resolvedEffect() SpendEffect {
	if isRecognizedEffect(r.Effect) && r.Effect != "" {
		return r.Effect
	}
	return EffectNormal
}
