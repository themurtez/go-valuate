// Package labor implements deterministic payroll / labor analytics for
// accountants, controllers, CFOs, and business-advisory workflows: total
// labor cost and its bridge (regular/overtime/bonus/commission/other pay,
// employer taxes/benefits/other burden, contractor spend), headcount and
// FTE, overtime and contractor mix, direct-vs-indirect labor,
// department/location/cost-center summaries, productivity metrics
// (revenue/gross-profit/EBITDA per employee or FTE), labor-cost-vs-revenue
// trend signals, workforce movement (hires/departures/turnover),
// worker-date consistency review, and payroll-register-to-GL
// reconciliation.
//
// # This is not payroll processing
//
// This package never calculates statutory payroll deductions, gross-to-net
// pay, tax withholding, employer tax liability, remittances, benefits
// administration, or T4/W-2 preparation. It has no time-clock system, no
// HR workflow, no recruiting logic, and produces no individual-employee
// performance score, productivity ranking, or termination recommendation.
// Every payroll dollar amount (regular pay, overtime pay, bonus,
// commission, employer taxes, benefits, other employer cost) is a fact the
// caller supplies; this package never derives or estimates one of these
// components from another. See docs/LABOR_ANALYTICS.md's "No payroll-law
// conclusions / Neutral language" section and safety_test.go for the
// regression test enforcing neutral, non-legal, non-decision language
// across every generated message.
//
// # Relationship to sibling packages
//
// This package is not accounting/ar or accounting/ap — a payroll register
// and a contractor labor ledger are not open-item receivables/payables,
// and this package never reuses ar.Receivable/ap.Payable shapes. It is not
// analytics/* — it performs no valuation, forecasting, or DCF math; its
// only cross-reference to a business's broader financial picture is the
// caller-supplied, intentionally narrow BusinessMetrics (Revenue,
// GrossProfit, EBITDA) used for productivity ratios. This package has no
// compile-time dependency on accounting/ledger, accounting/statements,
// accounting/ar, accounting/ap, accounting/cashforecast, or analytics/* —
// integration with those packages (mapping a payroll register to GL
// control balances, feeding known payroll cash obligations into a 13-week
// cash forecast, converting statement/financial-metrics output into
// BusinessMetrics) is demonstrated only via portable typed adapters and
// tests — see bridge_test.go's statement-reconciliation test,
// cashforecastadapter.go, and financialmetrics_test.go.
//
// # No hidden current-date dependency
//
// Every calculation takes explicit periods/dates from caller input.
// Nothing in this package calls time.Now() — this is essential for
// reproducibility and for building historical analyses from any date — see
// determinism_test.go.
//
// # Purity, immutability, and determinism
//
// Every exported function is pure: no I/O, no mutation of caller-owned
// input (Worker, PayrollRecord, ContractorLaborRecord, PeriodInfo,
// BusinessMetrics, GLPayrollControl, Policy, Dimension, and every
// slice/map they appear in are never modified in place — see
// immutability_test.go), no package-global mutable state. Calculate can be
// called concurrently and repeatedly against identical input and always
// returns byte-for-byte identical JSON — see determinism_test.go.
//
// # Explicit non-goals
//
// This package contains no persistence, HTTP/API handlers, auth, UI,
// background jobs, QuickBooks/Xero integration, payroll-provider API
// integration, payroll processing, tax tables, tax withholding
// calculations, remittance calculations, benefits administration,
// time-clock system, HR workflow, recruiting, individual-employee
// performance analytics, termination recommendations, payroll payment
// execution, HRIS synchronization, AI/LLM, or protected-class analysis —
// see docs/LABOR_ANALYTICS.md's "Explicit non-goals" section and the main
// README's [What this project intentionally does not
// contain](../../README.md#what-this-project-intentionally-does-not-contain).
package labor

import (
	"math"
	"time"
)

// isNonFinite reports whether v is NaN or +/-Inf — the shared guard every
// money/hours-bearing field in this package is checked against.
func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}

// amountTolerance is the floating-point comparison tolerance used
// throughout this package's amount validation and reconciliation,
// matching accounting/ar's and accounting/ap's identical
// money-comparison tolerance.
const amountTolerance = 0.005

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown because
// a required input was absent." This package's own local copy of the
// convention every sibling package in this repository duplicates rather
// than importing another package's Value — see
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
// system record (e.g. a payroll provider's pay-run ID or an HRIS employee
// key). This package never interprets it — mirrors ar.SourceRef/
// ap.SourceRef's identical "opaque reference" convention. Not
// financial.SourceRef, which is a different shape for a different
// purpose.
type SourceRef struct {
	System string `json:"system,omitempty"`
	ID     string `json:"id,omitempty"`
}

// Dimension is one lightweight, optional analysis tag on a record — an
// open string key, never a closed taxonomy, mirroring ar.Dimension/
// ap.Dimension/cashforecast.Dimension's identical rationale.
type Dimension struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Conventional Dimension.Key values. These are suggestions, not a closed
// set.
const (
	DimensionLocation     string = "location"
	DimensionDepartment   string = "department"
	DimensionCostCenter   string = "cost_center"
	DimensionJobTitle     string = "job_title"
	DimensionPayGroup     string = "pay_group"
	DimensionBusinessUnit string = "business_unit"
)

// WorkerType classifies a Worker's engagement type. Only categories that
// materially improve analytics are included — see the task's section 3.
type WorkerType string

const (
	WorkerTypeEmployee      WorkerType = "EMPLOYEE"
	WorkerTypeContractor    WorkerType = "CONTRACTOR"
	WorkerTypeOwnerEmployee WorkerType = "OWNER_EMPLOYEE"
	WorkerTypeTemporary     WorkerType = "TEMPORARY"
	WorkerTypeOther         WorkerType = "OTHER"
)

func isRecognizedWorkerType(w WorkerType) bool {
	switch w {
	case WorkerTypeEmployee, WorkerTypeContractor, WorkerTypeOwnerEmployee, WorkerTypeTemporary, WorkerTypeOther:
		return true
	default:
		return false
	}
}

// LaborClass distinguishes direct from indirect labor at the worker/
// record/cost-center level — the task's section 17. Never inferred from
// department name; a caller must classify explicitly. Zero value
// (LaborClassUnclassified) is the safe default for a caller who has not
// supplied a classification.
type LaborClass string

const (
	LaborClassUnclassified LaborClass = "UNCLASSIFIED"
	LaborClassDirect       LaborClass = "DIRECT"
	LaborClassIndirect     LaborClass = "INDIRECT"
)

func resolvedLaborClass(c LaborClass) LaborClass {
	switch c {
	case LaborClassDirect, LaborClassIndirect:
		return c
	default:
		return LaborClassUnclassified
	}
}

// RecordType distinguishes an ordinary payroll record from a correction —
// the task's section 47 explicit-typed-adjustment guidance. Zero value is
// treated as RecordTypeNormal.
type RecordType string

const (
	RecordTypeNormal     RecordType = "NORMAL"
	RecordTypeReversal   RecordType = "REVERSAL"
	RecordTypeAdjustment RecordType = "ADJUSTMENT"
)

func resolvedRecordType(t RecordType) RecordType {
	switch t {
	case RecordTypeReversal, RecordTypeAdjustment:
		return t
	default:
		return RecordTypeNormal
	}
}

// Worker is one portable worker/workforce-roster identity record — opaque
// identifiers only. This package never stores or analyzes race, religion,
// disability, health, political affiliation, sexual orientation, or other
// protected/sensitive demographic attributes; Worker deliberately carries
// no field for any of these — see the task's section 3.
type Worker struct {
	// WorkerID uniquely identifies this worker within one analysis.
	// Required; duplicates are flagged (see IssueDuplicateWorker) and only
	// the first occurrence (input order) is used.
	WorkerID string `json:"worker_id"`
	// WorkerType classifies this worker's engagement — see WorkerType.
	WorkerType WorkerType `json:"worker_type"`
	// Active is the caller's own current-status flag for this worker,
	// never inferred from HireDate/TerminationDate by this package.
	Active bool `json:"active"`
	// HireDate/TerminationDate are optional; TerminationDate nil means
	// still employed (or unknown), not "never terminated will be assumed."
	HireDate        *time.Time `json:"hire_date,omitempty"`
	TerminationDate *time.Time `json:"termination_date,omitempty"`

	Department string `json:"department,omitempty"`
	Location   string `json:"location,omitempty"`
	CostCenter string `json:"cost_center,omitempty"`

	// KeyWorker optionally marks this worker as a caller-declared key
	// role, used only for KeyWorkerLaborCostShare — see keyworker.go. Never
	// inferred by this package.
	KeyWorker bool `json:"key_worker,omitempty"`

	Dimensions []Dimension `json:"dimensions,omitempty"`
	SourceRef  SourceRef   `json:"source_ref,omitempty"`
}

// PayrollRecord is one portable payroll-register row — one worker's pay
// for one pay period/pay date. Every dollar/hour component is a fact the
// caller supplies; this package never calculates one component from
// another (e.g. never derives EmployerTaxes from RegularPay) — see the
// task's section 4.
type PayrollRecord struct {
	// ID uniquely identifies this payroll record within one analysis.
	// Required; duplicates are flagged (see IssueDuplicatePayrollRecord)
	// and only the first occurrence (input order) is used.
	ID string `json:"id"`
	// WorkerID identifies the worker this record pays. Required. May
	// reference a Worker not present in the supplied roster (roster is
	// optional) — see IssueUnknownWorker, which is only emitted when a
	// roster was actually supplied.
	WorkerID string `json:"worker_id"`

	// Period is the caller's label for the pay/accrual period this record
	// belongs to (e.g. "2025-06", "2025-W24"). Required for period-level
	// aggregation.
	Period string `json:"period"`
	// PayDate is when this pay was actually (or will be) disbursed —
	// distinct from Period, which is the accrual/expense period. Used for
	// cash-basis views (see basis.go) and pay-schedule analytics. Required.
	PayDate time.Time `json:"pay_date"`

	RegularPay    float64 `json:"regular_pay"`
	OvertimePay   float64 `json:"overtime_pay"`
	BonusPay      float64 `json:"bonus_pay"`
	CommissionPay float64 `json:"commission_pay"`
	OtherPay      float64 `json:"other_pay"`

	EmployerTaxes     float64 `json:"employer_taxes"`
	BenefitsCost      float64 `json:"benefits_cost"`
	OtherEmployerCost float64 `json:"other_employer_cost"`

	// HoursRegular/HoursOvertime are optional — hours-dependent analytics
	// (FTE, overtime share) report themselves unavailable without these.
	HoursRegular  Value `json:"hours_regular"`
	HoursOvertime Value `json:"hours_overtime"`

	Department string `json:"department,omitempty"`
	Location   string `json:"location,omitempty"`
	CostCenter string `json:"cost_center,omitempty"`

	// LaborClass is this record's direct/indirect classification — see
	// LaborClass. Overrides the owning Worker's own classification (if
	// any roster-level classification convention is added by a caller) for
	// this specific record; this package has no Worker-level LaborClass
	// field, so record-level is the only classification surface.
	LaborClass LaborClass `json:"labor_class,omitempty"`

	// RecordType distinguishes a normal record from a reversal/correction
	// — see RecordType. A caller needing to record a correction uses this
	// rather than an unexplained negative amount — see the task's section
	// 47.
	RecordType RecordType `json:"record_type,omitempty"`

	Currency string `json:"currency"`

	Dimensions []Dimension `json:"dimensions,omitempty"`
	SourceRef  SourceRef   `json:"source_ref,omitempty"`
}

// ContractorLaborRecord is one portable contractor-labor spend row. A
// caller must explicitly identify contractor labor here — this package
// never automatically classifies vendor/AP spend as contractor labor, per
// the task's section 5.
type ContractorLaborRecord struct {
	// ID uniquely identifies this record within one analysis. Required;
	// duplicates are flagged (see IssueDuplicateContractorRecord) and only
	// the first occurrence (input order) is used.
	ID string `json:"id"`
	// ContractorID is an opaque identifier for the contractor/vendor.
	// Required.
	ContractorID string `json:"contractor_id"`

	Period string    `json:"period"`
	Date   time.Time `json:"date"`

	Amount float64 `json:"amount"`
	Hours  Value   `json:"hours"`

	Department string `json:"department,omitempty"`
	Location   string `json:"location,omitempty"`
	CostCenter string `json:"cost_center,omitempty"`

	Currency string `json:"currency"`

	Dimensions []Dimension `json:"dimensions,omitempty"`
	SourceRef  SourceRef   `json:"source_ref,omitempty"`
}

// PeriodInfo is one caller-supplied explicit period boundary. This package
// never infers a fiscal calendar and never calls time.Now() — see the
// task's section 6.
type PeriodInfo struct {
	// Period is this period's label (e.g. "2025-06", "2025-Q2"), matching
	// PayrollRecord.Period/ContractorLaborRecord.Period's convention.
	Period    string    `json:"period"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	// Days is the caller-supplied period length in days, used only for
	// display/rate calculations this package does not otherwise need to
	// derive itself.
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

// BusinessMetrics is the narrow, portable set of caller-supplied financial
// figures used only for productivity ratios (revenue/gross-profit/EBITDA
// per employee or FTE). This package deliberately does not require a
// financial.FinancialDataset — see the task's section 18. An
// adapter/test may populate this from existing financial analytics; this
// package itself never imports financial or analytics/*.
type BusinessMetrics struct {
	Period      string `json:"period"`
	Revenue     Value  `json:"revenue"`
	GrossProfit Value  `json:"gross_profit"`
	EBITDA      Value  `json:"ebitda"`
}

// GLPayrollControl is one period's caller-supplied GL control-account
// balances for payroll-related accounts, used only for payroll-register-
// to-GL reconciliation (see reconcile.go). This package never posts
// adjustments and never infers which GL accounts are payroll-related — a
// caller (or a typed adapter — see ledgeradapter.go) supplies these
// balances explicitly.
type GLPayrollControl struct {
	Period string `json:"period"`

	GrossWages      Value `json:"gross_wages"`
	EmployerTaxes   Value `json:"employer_taxes"`
	Benefits        Value `json:"benefits"`
	ContractorLabor Value `json:"contractor_labor"`
	OtherLabor      Value `json:"other_labor"`
	TotalLaborCost  Value `json:"total_labor_cost"`
}
