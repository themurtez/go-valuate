package cashforecast

import "time"

// AmountValue represents a single figure that may or may not be
// available, distinguishing "computed to be exactly 0" from "unknown
// because a required input was absent" — the same availability convention
// ar.AmountValue/ap.AmountValue/debt.Value all use, duplicated here as its
// own type per this repository's established convention (see
// debt.Value's doc comment) rather than importing one of those packages
// into a package that otherwise has no dependency on them.
type AmountValue struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

// UnavailableAmount is the canonical zero-information AmountValue.
func UnavailableAmount() AmountValue { return AmountValue{} }

// AvailableAmount reports an AmountValue for a known figure (which may
// legitimately be zero or negative).
func AvailableAmount(v float64) AmountValue { return AmountValue{Available: true, Value: v} }

// CashDirection is whether a CashFlowEvent is money coming in or going
// out. Direction, not the sign of Amount, encodes this — see
// CashFlowEvent.Amount's doc comment for why a negative Amount is always
// invalid regardless of Direction.
type CashDirection string

const (
	DirectionInflow  CashDirection = "INFLOW"
	DirectionOutflow CashDirection = "OUTFLOW"
)

func isRecognizedDirection(d CashDirection) bool {
	switch d {
	case DirectionInflow, DirectionOutflow:
		return true
	default:
		return false
	}
}

// CashCategory is a stable, high-level classification of one
// CashFlowEvent, sufficient for management reporting and category
// breakdowns. This is deliberately a small, closed taxonomy per the task's
// "do not create an unnecessarily huge taxonomy" instruction — a caller
// wanting finer-grained classification uses Subcategory.
type CashCategory string

// Inflow categories.
const (
	CategoryARCollection         CashCategory = "AR_COLLECTION"
	CategoryCashSale             CashCategory = "CASH_SALE"
	CategoryOtherOperatingInflow CashCategory = "OTHER_OPERATING_INFLOW"
	CategoryLoanProceeds         CashCategory = "LOAN_PROCEEDS"
	CategoryOwnerContribution    CashCategory = "OWNER_CONTRIBUTION"
	CategoryAssetSale            CashCategory = "ASSET_SALE"
	CategoryOtherFinancingInflow CashCategory = "OTHER_FINANCING_INFLOW"
	CategoryOtherInflow          CashCategory = "OTHER_INFLOW"
)

// Outflow categories.
const (
	CategoryAPPayment             CashCategory = "AP_PAYMENT"
	CategoryPayroll               CashCategory = "PAYROLL"
	CategoryPayrollTax            CashCategory = "PAYROLL_TAX"
	CategorySalesTax              CashCategory = "SALES_TAX"
	CategoryIncomeTax             CashCategory = "INCOME_TAX"
	CategoryRent                  CashCategory = "RENT"
	CategoryDebtService           CashCategory = "DEBT_SERVICE"
	CategoryCapex                 CashCategory = "CAPEX"
	CategoryOwnerDistribution     CashCategory = "OWNER_DISTRIBUTION"
	CategoryOtherOperatingOutflow CashCategory = "OTHER_OPERATING_OUTFLOW"
	CategoryOtherFinancingOutflow CashCategory = "OTHER_FINANCING_OUTFLOW"
	CategoryOtherOutflow          CashCategory = "OTHER_OUTFLOW"
)

// inflowCategories and outflowCategories fix each category's expected
// Direction for validation (IssueCategoryDirectionMismatch) and drive
// deterministic category enumeration order — see sort.go.
var inflowCategories = []CashCategory{
	CategoryARCollection,
	CategoryCashSale,
	CategoryOtherOperatingInflow,
	CategoryLoanProceeds,
	CategoryOwnerContribution,
	CategoryAssetSale,
	CategoryOtherFinancingInflow,
	CategoryOtherInflow,
}

var outflowCategories = []CashCategory{
	CategoryAPPayment,
	CategoryPayroll,
	CategoryPayrollTax,
	CategorySalesTax,
	CategoryIncomeTax,
	CategoryRent,
	CategoryDebtService,
	CategoryCapex,
	CategoryOwnerDistribution,
	CategoryOtherOperatingOutflow,
	CategoryOtherFinancingOutflow,
	CategoryOtherOutflow,
}

// allCategoriesOrder is every recognized category in one fixed enumeration
// order (inflows then outflows, each in declaration order), used for
// deterministic category-breakdown output — see sort.go.
var allCategoriesOrder = append(append([]CashCategory{}, inflowCategories...), outflowCategories...)

// categoryDirection returns the expected CashDirection for c, and false if
// c is not a recognized category.
func categoryDirection(c CashCategory) (CashDirection, bool) {
	for _, ic := range inflowCategories {
		if ic == c {
			return DirectionInflow, true
		}
	}
	for _, oc := range outflowCategories {
		if oc == c {
			return DirectionOutflow, true
		}
	}
	return "", false
}

// CashFlowClass is the operating/investing/financing classification a
// CashCategory maps to, echoed on WeeklyForecast when the category
// taxonomy cleanly supports it — see the task's "do not force
// cash-flow-statement accounting if semantics are ambiguous" instruction.
// Every category defined above maps to exactly one class; there is no
// "ambiguous" case in practice given the fixed taxonomy.
type CashFlowClass string

const (
	ClassOperating CashFlowClass = "OPERATING"
	ClassInvesting CashFlowClass = "INVESTING"
	ClassFinancing CashFlowClass = "FINANCING"
)

// categoryClass returns c's CashFlowClass.
func categoryClass(c CashCategory) CashFlowClass {
	switch c {
	case CategoryCapex, CategoryAssetSale:
		return ClassInvesting
	case CategoryLoanProceeds, CategoryOwnerContribution, CategoryOwnerDistribution,
		CategoryOtherFinancingInflow, CategoryOtherFinancingOutflow, CategoryDebtService:
		return ClassFinancing
	default:
		return ClassOperating
	}
}

// CashBasis distinguishes how firmly grounded one CashFlowEvent's amount
// and date are, per the task's "do not call assumed amounts known"
// instruction. This is caller-supplied descriptive metadata; this package
// never infers or upgrades/downgrades a Basis value.
type CashBasis string

const (
	// BasisKnown means the event already exists as a firm obligation or
	// receivable with a known date and amount (e.g. an open AP bill's due
	// date and amount).
	BasisKnown CashBasis = "KNOWN"
	// BasisScheduled means the event is a caller-committed schedule item
	// not yet a firm bill/invoice (e.g. payroll on the next pay date).
	BasisScheduled CashBasis = "SCHEDULED"
	// BasisAssumed means the event is the caller's own estimate (e.g. an
	// estimated AR receipt date/amount not yet firmly scheduled).
	BasisAssumed CashBasis = "ASSUMED"
	// BasisScenario means the event exists only under a specific named
	// Scenario (e.g. a delayed collection in a downside case), never in
	// the base case.
	BasisScenario CashBasis = "SCENARIO"
)

func isRecognizedBasis(b CashBasis) bool {
	switch b {
	case BasisKnown, BasisScheduled, BasisAssumed, BasisScenario:
		return true
	default:
		return false
	}
}

// Certainty is an optional, small caller-supplied confidence label on one
// CashFlowEvent. This is descriptive metadata only — this package never
// converts it into a probability or otherwise uses it in any calculation.
// A caller wanting probability-weighted forecasting supplies its own
// pre-weighted Amount instead; that is out of scope for V1 (see the task's
// "no probabilistic forecasting is needed in V1" instruction).
type Certainty string

const (
	CertaintyCommitted Certainty = "COMMITTED"
	CertaintyHigh      Certainty = "HIGH"
	CertaintyMedium    Certainty = "MEDIUM"
	CertaintyLow       Certainty = "LOW"
	CertaintyUnknown   Certainty = "UNKNOWN"
)

func isRecognizedCertainty(c Certainty) bool {
	switch c {
	case "", CertaintyCommitted, CertaintyHigh, CertaintyMedium, CertaintyLow, CertaintyUnknown:
		return true
	default:
		return false
	}
}

// Commitment distinguishes an outflow the business must make from one it
// could defer or skip, per the task's "allow caller to mark outflows
// COMMITTED/DISCRETIONARY" instruction. This package never infers
// Commitment from CashCategory — a caller must set it explicitly for it to
// be meaningful; the zero value (CommitmentUnspecified) is excluded from
// every discretionary-vs-committed breakdown rather than defaulting to
// either extreme.
type Commitment string

const (
	CommitmentUnspecified   Commitment = ""
	CommitmentRequired      Commitment = "REQUIRED"
	CommitmentDeferrable    Commitment = "DEFERRABLE"
	CommitmentDiscretionary Commitment = "DISCRETIONARY"
)

func isRecognizedCommitment(c Commitment) bool {
	switch c {
	case CommitmentUnspecified, CommitmentRequired, CommitmentDeferrable, CommitmentDiscretionary:
		return true
	default:
		return false
	}
}

// Priority is an optional caller-supplied payment priority label, metadata
// only — this package never uses it to automatically choose which bills to
// pay in the base calculation. A scenario helper may use it if the caller
// explicitly requests a priority-based deferral policy — see
// DeferByPriority.
type Priority string

const (
	PriorityUnspecified Priority = ""
	PriorityCritical    Priority = "CRITICAL"
	PriorityHigh        Priority = "HIGH"
	PriorityNormal      Priority = "NORMAL"
	PriorityLow         Priority = "LOW"
)

func isRecognizedPriority(p Priority) bool {
	switch p {
	case PriorityUnspecified, PriorityCritical, PriorityHigh, PriorityNormal, PriorityLow:
		return true
	default:
		return false
	}
}

// Dimension is one lightweight, optional analysis tag on a CashFlowEvent,
// mirroring accounting/ar.Dimension and accounting/ap.Dimension's
// identical open-string-key rationale: this package never hard-codes a
// dimension taxonomy.
type Dimension struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SourceType identifies what kind of upstream record or process produced
// one CashFlowEvent, distinct from CashCategory (SourceType is about
// provenance/mechanism, CashCategory is about cash-flow-statement-style
// classification).
type SourceType string

const (
	SourceManual            SourceType = "MANUAL"
	SourceARReceivable      SourceType = "AR_RECEIVABLE"
	SourceAPPayable         SourceType = "AP_PAYABLE"
	SourcePayrollSchedule   SourceType = "PAYROLL_SCHEDULE"
	SourceTaxSchedule       SourceType = "TAX_SCHEDULE"
	SourceDebtSchedule      SourceType = "DEBT_SCHEDULE"
	SourceRecurringRule     SourceType = "RECURRING_RULE"
	SourceCapexPlan         SourceType = "CAPEX_PLAN"
	SourceFinancingPlan     SourceType = "FINANCING_PLAN"
	SourceScenarioTransform SourceType = "SCENARIO_TRANSFORM"
)

// CashFlowEvent is one portable, dated cash inflow or outflow — the
// primary unit every WeeklyForecast bucket is built from. A caller
// assembles these directly, or this package's adapters/generators produce
// them deterministically from a recurring rule, AR/AP scheduling
// assumption, or scenario transformation; either way, every CashFlowEvent
// in a Result's DetailedSchedule is traceable to a concrete origin.
type CashFlowEvent struct {
	// ID uniquely identifies this event within one Input.Events (plus any
	// generated events — see the ID-namespacing note on each generator).
	// Required; duplicates are flagged (IssueDuplicateEvent) and only the
	// first occurrence (input order) is used.
	ID string `json:"id"`
	// Date is the expected cash date — when the money actually moves, not
	// an invoice/bill/due date. Required.
	Date time.Time `json:"date"`
	// Amount is always >= 0; Direction determines the sign. A negative
	// Amount is invalid regardless of Direction (IssueNegativeAmount) —
	// see the task's explicit "do not allow Direction=OUTFLOW +
	// Amount=-100" instruction. A refund/credit uses Direction/Category
	// explicitly (e.g. an AP vendor credit expected as cash back is
	// DirectionInflow, CategoryOtherOperatingInflow), never a negative
	// Amount.
	Amount float64 `json:"amount"`
	// Direction is INFLOW or OUTFLOW. Required.
	Direction CashDirection `json:"direction"`
	// Category is this event's cash-flow-statement-style classification.
	// Required; must be one of the fixed CashCategory constants and must
	// match Direction (e.g. CategoryAPPayment must have
	// Direction=OUTFLOW) — see IssueCategoryDirectionMismatch.
	Category CashCategory `json:"category"`
	// Subcategory is an optional caller-defined label/subcategory within
	// Category, carried through for display only and never validated
	// against a fixed set.
	Subcategory string `json:"subcategory,omitempty"`
	// Description is an optional human-readable label.
	Description string `json:"description,omitempty"`
	// CounterpartyID is an optional opaque identifier for the customer,
	// supplier, employee group, lender, or other counterparty.
	CounterpartyID string `json:"counterparty_id,omitempty"`

	// SourceType identifies what produced this event.
	SourceType SourceType `json:"source_type,omitempty"`
	// SourceID is an opaque pointer back to the originating record (e.g. a
	// Receivable.ID, Payable.ID, or recurring rule ID). Optional but
	// strongly recommended for provenance — see the task's "every forecast
	// amount must trace to source events" requirement.
	SourceID string `json:"source_id,omitempty"`

	// Basis distinguishes known/scheduled/assumed/scenario grounding — see
	// CashBasis. Required.
	Basis CashBasis `json:"basis"`
	// Certainty is an optional confidence label — see Certainty. Metadata
	// only.
	Certainty Certainty `json:"certainty,omitempty"`
	// Commitment optionally marks this outflow as required/deferrable/
	// discretionary — see Commitment. Ignored for inflows.
	Commitment Commitment `json:"commitment,omitempty"`
	// Priority is an optional payment-priority label — see Priority.
	// Metadata only unless a scenario helper is explicitly asked to use it
	// — see DeferByPriority.
	Priority Priority `json:"priority,omitempty"`

	// ScenarioTags marks this event as belonging only to the named
	// scenario(s) — empty means the event applies to every scenario
	// (including base). This is set by scenario transformation helpers
	// (AddEvent) for an event a caller adds only under a specific
	// scenario; a caller rarely sets this directly on a base Input.Events
	// entry.
	ScenarioTags []string `json:"scenario_tags,omitempty"`

	// Dimensions supports optional grouping.
	Dimensions []Dimension `json:"dimensions,omitempty"`
}

// cloneEvent returns a deep copy of e (its own slices copied), so
// generators and scenario transforms never share backing arrays with
// caller-owned or previously generated events — see immutability_test.go.
func cloneEvent(e CashFlowEvent) CashFlowEvent {
	out := e
	if len(e.ScenarioTags) > 0 {
		out.ScenarioTags = append([]string{}, e.ScenarioTags...)
	} else {
		out.ScenarioTags = nil
	}
	if len(e.Dimensions) > 0 {
		out.Dimensions = append([]Dimension{}, e.Dimensions...)
	} else {
		out.Dimensions = nil
	}
	return out
}

func cloneEvents(events []CashFlowEvent) []CashFlowEvent {
	if len(events) == 0 {
		return nil
	}
	out := make([]CashFlowEvent, len(events))
	for i, e := range events {
		out[i] = cloneEvent(e)
	}
	return out
}

// appliesToScenario reports whether e applies under scenarioLabel: true if
// e.ScenarioTags is empty (applies everywhere), or scenarioLabel is
// present in e.ScenarioTags.
func appliesToScenario(e CashFlowEvent, scenarioLabel string) bool {
	if len(e.ScenarioTags) == 0 {
		return true
	}
	for _, tag := range e.ScenarioTags {
		if tag == scenarioLabel {
			return true
		}
	}
	return false
}
