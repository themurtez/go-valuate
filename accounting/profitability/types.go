// Package profitability implements deterministic customer/job/product
// profitability analytics: the same underlying economic facts
// (revenue/returns/discounts/direct costs/variable costs), attributed
// explicitly to one or more of three analytical dimensions — CUSTOMER,
// JOB, and PRODUCT — and rolled up into independent per-dimension
// profitability views, plus optional shared-cost allocation and
// business-level reconciliation.
//
// # One fact, three views
//
// A caller never duplicates the same revenue/cost event three times to
// analyze it from three angles. Each Fact is recorded once and carries
// explicit per-dimension Attribution: the same $1,000 invoice line can be
// 100% attributed to customer C1, 100% attributed to job J10, and split
// 60/40 across products P1/P2 — and each of CustomerView, JobView, and
// ProductView reconciles independently back to the same underlying fact
// population. Customer-view totals, job-view totals, and product-view
// totals are never summed together — see docs/PROFITABILITY_ANALYTICS.md's
// "one-fact/multi-view principle" section and TestInvariant_NoDoubleCountAcrossDimensions.
//
// # This is not a pricing optimizer, ERP, CRM, or AI recommendation engine
//
// This package performs deterministic arithmetic and diagnostics only. It
// never recommends a price, never recommends dropping a customer or
// discontinuing a product, never schedules jobs/projects, never
// integrates with a CRM/ERP/QuickBooks/Xero, never processes invoicing or
// payroll, never derives inventory cost, and contains no AI/LLM/ML
// component. The caller supplies classification, attribution, and
// allocation policy; this package never infers any of the three from
// names, categories, or amounts — see docs/PROFITABILITY_ANALYTICS.md's
// "Explicit non-goals" section.
//
// # Relationship to sibling packages
//
// This package has no compile-time dependency on accounting/ledger,
// accounting/statements, accounting/labor, accounting/inventory, or
// analytics/* — integration with those packages (mapping ledger lines to
// profitability components, converting an accounting/labor direct-labor
// fact into a DIRECT_LABOR Fact, adapting an accounting/inventory
// authoritative item cost, converting accounting/statements totals into
// ControlTotals, and reusing analytics/concentration for revenue
// concentration) is demonstrated only via portable typed adapters and
// tests — see ledgeradapter.go, laboradapter.go, inventoryadapter.go,
// statementsadapter.go, and concentrationadapter.go.
//
// # No hidden current-date dependency
//
// Every calculation takes explicit periods/dates from caller input.
// Nothing in this package calls time.Now() — see determinism_test.go.
//
// # Purity, immutability, and determinism
//
// Every exported function is pure: no I/O, no mutation of caller-owned
// input, no package-global mutable state. Calculate can be called
// concurrently and repeatedly against identical input and always returns
// byte-for-byte identical JSON — see determinism_test.go and
// immutability_test.go.
//
// # Explicit non-goals
//
// This package contains no persistence, HTTP/API handlers, auth, UI,
// background jobs, CRM/ERP/QuickBooks/Xero integration, invoicing, job
// management, inventory costing, payroll processing, pricing
// recommendations, AI/LLM/ML, or customer-churn/employee-evaluation
// analytics — see docs/PROFITABILITY_ANALYTICS.md's "Explicit non-goals"
// section and the main README's [What this project intentionally does
// not contain](../../README.md#what-this-project-intentionally-does-not-contain).
package profitability

import (
	"math"
	"time"
)

// isNonFinite reports whether v is NaN or +/-Inf — the shared guard every
// money/weight/share-bearing field in this package is checked against.
func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}

// amountTolerance is the default floating-point comparison tolerance used
// throughout this package's reconciliation checks when Policy supplies no
// more specific tolerance — matches accounting/labor's and
// accounting/ar's/accounting/ap's identical money-comparison tolerance.
const amountTolerance = 0.005

// shareTolerance is the default tolerance for attribution-share-sum
// comparisons (e.g. "sum of shares <= 1") — see the task's section 11.
const shareTolerance = 0.0001

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown
// because a required input was absent." This package's own local copy of
// the convention every sibling package in this repository duplicates
// rather than importing another package's Value — see
// journaldiagnostics.Value's doc comment for the full rationale.
type Value struct {
	Available bool    `json:"available"`
	Amount    float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may
// legitimately be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// SourceRef is an opaque, caller-defined pointer back to the originating
// system record. This package never interprets it — mirrors
// labor.SourceRef/ar.SourceRef/ap.SourceRef's identical "opaque
// reference" convention. Not financial.SourceRef, which is a different
// shape for a different purpose.
type SourceRef struct {
	System string `json:"system,omitempty"`
	ID     string `json:"id,omitempty"`
}

// Dimension is one of the three fixed analytical lenses this package
// supports — task section 2. Not an open/extensible taxonomy: if
// SERVICE-level analysis is useful, model it as a PRODUCT entity or a
// PRODUCT Group, not a fourth Dimension value.
type Dimension string

const (
	DimensionCustomer Dimension = "CUSTOMER"
	DimensionJob      Dimension = "JOB"
	DimensionProduct  Dimension = "PRODUCT"
)

// dimensionOrder fixes Dimension declaration/processing order for
// deterministic output — task section 71 "dimensions CUSTOMER -> JOB ->
// PRODUCT".
var dimensionOrder = []Dimension{DimensionCustomer, DimensionJob, DimensionProduct}

func isRecognizedDimension(d Dimension) bool {
	switch d {
	case DimensionCustomer, DimensionJob, DimensionProduct:
		return true
	default:
		return false
	}
}

func dimensionRank(d Dimension) int {
	for i, dd := range dimensionOrder {
		if dd == d {
			return i
		}
	}
	return len(dimensionOrder)
}

// Applicability distinguishes a dimension the caller has deliberately
// opted out of from one that is simply missing data — task section 56.
// Never inferred from an empty entity/fact slice; a caller sets this
// explicitly via Policy.DimensionApplicability.
type Applicability string

const (
	// ApplicabilityEnabled means this Dimension should be analyzed
	// (default when Policy.DimensionApplicability omits an entry).
	ApplicabilityEnabled Applicability = "ENABLED"
	// ApplicabilityNotApplicable means this Dimension does not apply to
	// this business/analysis at all (e.g. a retailer with no JOB concept)
	// and its View is reported as ViewStatusNotApplicable regardless of
	// whether any entities/facts reference it.
	ApplicabilityNotApplicable Applicability = "NOT_APPLICABLE"
)

func isRecognizedApplicability(a Applicability) bool {
	switch a {
	case ApplicabilityEnabled, ApplicabilityNotApplicable:
		return true
	default:
		return false
	}
}

// Entity is one customer, job, or product/service master record — task
// section 3. Identity is the (Dimension, EntityID) pair. This package
// never infers Group/Category/hierarchy from Name — a caller classifies
// explicitly.
type Entity struct {
	Dimension Dimension `json:"dimension"`
	// EntityID uniquely identifies this entity within its Dimension.
	// Required; duplicates (same Dimension+EntityID) are flagged (see
	// IssueDuplicateEntity) and only the first occurrence (input order) is
	// used.
	EntityID string `json:"entity_id"`
	// Name is an optional human-readable label, carried through for
	// display only. Never used as an identity key or to infer Group/
	// Category.
	Name string `json:"name,omitempty"`
	// Group is a caller-defined grouping label (e.g. a customer segment, a
	// job type, a product line) — see GroupSummary. Never inferred from
	// Name.
	Group string `json:"group,omitempty"`
	// Category is a second, independent caller-defined classification
	// label, orthogonal to Group.
	Category string `json:"category,omitempty"`
	// Active is the caller's own current-status flag, never inferred by
	// this package.
	Active bool `json:"active"`
	// ParentEntityID optionally references another Entity.EntityID within
	// the same Dimension, for caller-declared hierarchy (e.g. a product
	// variant under a parent product). This package does not itself roll
	// up parent/child amounts anywhere — it is carried through for a
	// caller's own downstream use only.
	ParentEntityID string    `json:"parent_entity_id,omitempty"`
	SourceRef      SourceRef `json:"source_ref,omitempty"`
}

// PeriodInfo is one caller-supplied explicit period boundary. This
// package never infers a fiscal calendar and never calls time.Now() —
// task section 4.
type PeriodInfo struct {
	// Period is this period's label (e.g. "2025-06", "2025-Q2"), matching
	// Fact.Period's convention.
	Period    string    `json:"period"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	// Days is the caller-supplied period length in days, carried through
	// for display only.
	Days int `json:"days,omitempty"`
}

// Valid reports whether p's StartDate/EndDate are both set and consistent
// (StartDate <= EndDate).
func (p PeriodInfo) Valid() bool {
	if p.Period == "" || p.StartDate.IsZero() || p.EndDate.IsZero() {
		return false
	}
	return !p.EndDate.Before(p.StartDate)
}

// Component is the economic-fact taxonomy — task section 7. Kept to the
// categories that materially change the profitability bridge; finer
// subcategorization is the caller's own SourceType/SourceID/Dimensions,
// not a new Component.
type Component string

const (
	// Revenue components.
	ComponentGrossRevenue          Component = "GROSS_REVENUE"
	ComponentReturn                Component = "RETURN"
	ComponentDiscount              Component = "DISCOUNT"
	ComponentOtherRevenueReduction Component = "OTHER_REVENUE_REDUCTION"
	ComponentOtherRevenue          Component = "OTHER_REVENUE"

	// Direct-cost components.
	ComponentDirectMaterial      Component = "DIRECT_MATERIAL"
	ComponentDirectLabor         Component = "DIRECT_LABOR"
	ComponentDirectSubcontractor Component = "DIRECT_SUBCONTRACTOR"
	ComponentDirectFulfillment   Component = "DIRECT_FULFILLMENT"
	ComponentDirectOther         Component = "DIRECT_OTHER"

	// Variable-operating-cost components.
	ComponentVariableCommission Component = "VARIABLE_COMMISSION"
	ComponentVariablePaymentFee Component = "VARIABLE_PAYMENT_FEE"
	ComponentVariableOther      Component = "VARIABLE_OTHER"
)

// componentOrder fixes Component declaration order for deterministic
// bridge/breakdown iteration.
var componentOrder = []Component{
	ComponentGrossRevenue, ComponentReturn, ComponentDiscount, ComponentOtherRevenueReduction, ComponentOtherRevenue,
	ComponentDirectMaterial, ComponentDirectLabor, ComponentDirectSubcontractor, ComponentDirectFulfillment, ComponentDirectOther,
	ComponentVariableCommission, ComponentVariablePaymentFee, ComponentVariableOther,
}

func isRecognizedComponent(c Component) bool {
	for _, cc := range componentOrder {
		if cc == c {
			return true
		}
	}
	return false
}

func componentRank(c Component) int {
	for i, cc := range componentOrder {
		if cc == c {
			return i
		}
	}
	return len(componentOrder)
}

// componentClass distinguishes a component's contribution to the bridge —
// used internally to sum the right bucket without a giant switch
// repeated everywhere.
type componentClass int

const (
	classRevenueAdd componentClass = iota
	classRevenueReduce
	classDirectCost
	classVariableCost
)

var componentClassOf = map[Component]componentClass{
	ComponentGrossRevenue:          classRevenueAdd,
	ComponentOtherRevenue:          classRevenueAdd,
	ComponentReturn:                classRevenueReduce,
	ComponentDiscount:              classRevenueReduce,
	ComponentOtherRevenueReduction: classRevenueReduce,

	ComponentDirectMaterial:      classDirectCost,
	ComponentDirectLabor:         classDirectCost,
	ComponentDirectSubcontractor: classDirectCost,
	ComponentDirectFulfillment:   classDirectCost,
	ComponentDirectOther:         classDirectCost,

	ComponentVariableCommission: classVariableCost,
	ComponentVariablePaymentFee: classVariableCost,
	ComponentVariableOther:      classVariableCost,
}

// isRevenueComponent/isDirectCostComponent/isVariableCostComponent
// classify a Component for attribution/coverage bucketing — task section
// 13 "revenue/direct cost/variable cost" coverage buckets.
func isRevenueComponent(c Component) bool {
	cl, ok := componentClassOf[c]
	return ok && (cl == classRevenueAdd || cl == classRevenueReduce)
}

func isDirectCostComponent(c Component) bool {
	return componentClassOf[c] == classDirectCost
}

func isVariableCostComponent(c Component) bool {
	return componentClassOf[c] == classVariableCost
}

// Attribution assigns a Share (0-1) of one Fact to one Entity within one
// Dimension — task section 10. Shares within the same FactID+Dimension
// are evaluated independently per dimension; a Fact's CUSTOMER
// attributions never constrain its JOB or PRODUCT attributions.
type Attribution struct {
	Dimension Dimension `json:"dimension"`
	EntityID  string    `json:"entity_id"`
	// Share is this entity's fraction (0-1 inclusive) of the Fact's
	// Amount for Dimension. This package never renormalizes shares — see
	// the task's section 11.
	Share float64 `json:"share"`
}

// Fact is one economic-fact record — revenue, a return/discount, a direct
// cost, or a variable operating cost — recorded once and attributed
// explicitly to zero or more entities per dimension — task section 5.
type Fact struct {
	// FactID uniquely identifies this fact within one analysis. Required;
	// duplicates are flagged (see IssueDuplicateFact) and only the first
	// occurrence (input order) is used.
	FactID string `json:"fact_id"`
	// Period is the caller's label for the period this fact belongs to.
	// Required for period-level aggregation.
	Period string `json:"period"`
	// Date is this fact's optional transaction date, carried through for
	// display/provenance only.
	Date *time.Time `json:"date,omitempty"`
	// Component classifies this fact's economic meaning — see Component.
	Component Component `json:"component"`
	// Amount is this fact's non-negative economic magnitude — see the
	// task's section 6 "Amount semantics." Component determines whether
	// Amount adds to revenue, reduces revenue, or adds to cost.
	Amount float64 `json:"amount"`
	// Attributions is this fact's explicit per-dimension attribution —
	// see Attribution. A dimension with no Attribution entries at all
	// means 100% of this fact's amount is unattributed for that
	// dimension (see UnattributedRevenue/UnattributedDirectCost/
	// UnattributedVariableCost).
	Attributions []Attribution `json:"attributions,omitempty"`

	SourceType string    `json:"source_type,omitempty"`
	SourceID   string    `json:"source_id,omitempty"`
	Currency   string    `json:"currency"`
	SourceRef  SourceRef `json:"source_ref,omitempty"`
}

// DirectAttribution returns a single Attribution assigning 100% of a Fact
// to entityID within dimension — a pure helper for the common "this whole
// fact belongs to one entity" case, task section 12.
func DirectAttribution(dimension Dimension, entityID string) Attribution {
	return Attribution{Dimension: dimension, EntityID: entityID, Share: 1}
}

// DirectAttributions returns one 100%-share Attribution per (dimension,
// entityID) pair supplied, in the order given — a pure helper for the
// common case of attributing one fact 100% to one customer, one job, and
// one product simultaneously (each independently, per dimension).
func DirectAttributions(pairs ...[2]string) []Attribution {
	out := make([]Attribution, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, DirectAttribution(Dimension(p[0]), p[1]))
	}
	return out
}
