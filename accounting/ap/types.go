// Package ap implements deterministic accounts-payable aging and supplier
// payment analytics from a portable open-payables model.
//
// This package is application-independent, like every analytics-style
// package in this repository: it does not require accounting/ledger or
// accounting/statements input at all. A caller can populate it directly
// from a QuickBooks/Xero export, a homegrown AP system, or a synthetic
// fixture — see accounting/ap/fixtures. A future statements/ledger-aware
// adapter (not part of this package) may bridge accounting/ledger accounts
// payable balances into Payable values, but this package itself never
// imports accounting/ledger.
//
// accounting/ap mirrors accounting/ar's shape and conventions closely (both
// are open-item aging/analytics engines), but is not a generic
// generalization of it — the two are independent packages with independent
// domain models (Payable vs. Receivable, supplier vs. customer,
// DPO vs. DSO, due-date payment schedule and payment-pressure vs.
// collections), sharing only ideas and naming discipline, never code.
//
// This package contains no persistence, HTTP/API handlers, auth, UI,
// background jobs, QuickBooks/Xero integration, payment execution, bank
// integration, AI/LLM, or tax logic — see the package doc's "Explicit
// non-goals" section in docs/AP_AGING.md.
//
// # No hidden current-date dependency
//
// Every aging calculation takes an explicit AsOfDate from caller input.
// Nothing in this package calls time.Now() — this is essential for
// reproducibility and for building historical snapshots (see
// determinism_test.go).
//
// Every exported function is pure: no I/O, no mutation of caller-owned
// input (Payable, SupplierPayment, BucketDefinition, and every slice/map
// they appear in are never modified in place — see immutability_test.go),
// no package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package ap

import "time"

// PayableStatus is the lifecycle state of one Payable. Only OPEN/
// PARTIALLY_PAID payables are economically outstanding; the others exist
// so a caller's full payables export (including history) can be passed
// through without pre-filtering, while this package still applies its own
// default inclusion policy — see AgingOptions.IncludeStatuses and
// defaultAgingStatuses.
type PayableStatus string

const (
	StatusOpen          PayableStatus = "OPEN"
	StatusPartiallyPaid PayableStatus = "PARTIALLY_PAID"
	StatusPaid          PayableStatus = "PAID"
	StatusVoided        PayableStatus = "VOIDED"
	StatusDisputed      PayableStatus = "DISPUTED"
)

// isRecognizedStatus reports whether s is one of the fixed PayableStatus
// values.
func isRecognizedStatus(s PayableStatus) bool {
	switch s {
	case StatusOpen, StatusPartiallyPaid, StatusPaid, StatusVoided, StatusDisputed:
		return true
	default:
		return false
	}
}

// defaultAgingStatuses returns the statuses that are "economically
// relevant open payables" by default: OPEN, PARTIALLY_PAID, and DISPUTED
// (a disputed bill still ages — see the package doc's disputed-payables
// section, and section 14's explicit "do not automatically exclude
// disputed bills from aging" rule). PAID and VOIDED are excluded by
// default since they no longer represent outstanding exposure.
//
// The task's section 1 invites "another adjustment/writeoff-like status
// only if semantically justified." Unlike accounts-receivable
// write-offs (a lender-side credit-loss decision), a payable a business no
// longer intends to pay is simply VOIDED (the bill is canceled/reversed,
// often via a vendor credit) — there is no separate AP write-off concept
// this package's task describes, so no additional status is added.
func defaultAgingStatuses() []PayableStatus {
	return []PayableStatus{StatusOpen, StatusPartiallyPaid, StatusDisputed}
}

// DocumentType distinguishes an ordinary vendor bill from a vendor credit.
// A bill's OpenAmount is expected to be >= 0 (an amount owed to the
// supplier); a vendor credit's OpenAmount is expected to be <= 0 (a credit
// owed BY the supplier, reducing what this business owes). Zero value ("")
// is treated as DocumentTypeBill — see resolvedDocumentType.
type DocumentType string

const (
	DocumentTypeBill         DocumentType = "BILL"
	DocumentTypeVendorCredit DocumentType = "VENDOR_CREDIT"
)

// resolvedDocumentType returns d if recognized, otherwise
// DocumentTypeBill — the safe default for a caller's existing bill data
// that predates this field.
func resolvedDocumentType(d DocumentType) DocumentType {
	if d == DocumentTypeVendorCredit {
		return d
	}
	return DocumentTypeBill
}

// SourceRef is an opaque, caller-defined pointer back to the originating
// system record (e.g. a QuickBooks transaction ID or an ERP bill key).
// This package never interprets it — mirrors ledger.JournalLine.SourceRef's
// identical "opaque reference" convention.
type SourceRef struct {
	System string `json:"system,omitempty"`
	ID     string `json:"id,omitempty"`
}

// Dimension is one lightweight, optional analysis tag on a Payable —
// location, department, business unit, supplier category, or region. Key
// is an open string rather than a closed enum, mirroring
// ledger.Dimension's identical rationale: this package never hard-codes a
// dimension taxonomy.
type Dimension struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Conventional Dimension.Key values. These are suggestions, not a closed
// set.
const (
	DimensionLocation         string = "location"
	DimensionDepartment       string = "department"
	DimensionBusinessUnit     string = "business_unit"
	DimensionSupplierCategory string = "supplier_category"
	DimensionRegion           string = "region"
)

// Payable is one portable open-item / bill-level record — the primary
// input to this package. A caller assembles a slice of these from
// whatever AP/vendor-bill source it has; this package has no opinion on
// where it came from.
type Payable struct {
	// ID uniquely identifies this payable within one analysis. Required;
	// duplicates are flagged (see IssueDuplicatePayable) and only the
	// first occurrence (in input order) is used.
	ID string `json:"id"`
	// SupplierID is an opaque, caller-assigned identifier for the
	// supplier. Required — see SupplierName's doc comment for why this
	// package treats SupplierID, not SupplierName, as the identity key.
	SupplierID string `json:"supplier_id"`
	// SupplierName is an optional human-readable label, carried through
	// for display only. Never used as an identity key: two Payables with
	// the same SupplierID but different SupplierName strings are still
	// the same supplier, and a caller who only has opaque IDs (no names at
	// all) gets a fully functional analysis.
	SupplierName string `json:"supplier_name,omitempty"`
	// BillNumber is an optional human-readable bill label, carried through
	// for display only.
	BillNumber string `json:"bill_number,omitempty"`

	// DocumentType distinguishes a bill from a vendor credit — see
	// DocumentType.
	DocumentType DocumentType `json:"document_type,omitempty"`

	BillDate time.Time `json:"bill_date"`
	DueDate  time.Time `json:"due_date"`

	// OriginalAmount is the payable's original face amount at issuance.
	// Expected >= 0 for a bill, <= 0 for a vendor credit (see
	// DocumentType) — a sign mismatch is flagged (see IssueInvalidAmount)
	// but does not by itself exclude the row.
	OriginalAmount float64 `json:"original_amount"`
	// OpenAmount is the amount still outstanding as of the data snapshot
	// this Payable represents. This is the amount aging is always
	// computed from (never OriginalAmount) — see the package doc
	// comment's "amount semantics" section. Status is never inferred from
	// OpenAmount — see the task's explicit "do not infer status from
	// OpenAmount" instruction.
	OpenAmount float64 `json:"open_amount"`

	// Currency is this payable's ISO 4217-style currency code (e.g.
	// "USD"). Required; a mix of currencies within one analysis is
	// flagged unless resolved — see Options.ReportingCurrency and
	// IssueMixedCurrency.
	Currency string `json:"currency"`

	Status PayableStatus `json:"status"`

	// TermsDays is the contractual payment terms in days (e.g. 30 for
	// "net 30"), used by TermsAnalysis. Optional.
	TermsDays int `json:"terms_days,omitempty"`
	// PaidDate is when this payable was fully paid, if it has been.
	// Optional; used only for payment-timing analysis when
	// SupplierPayments are not separately supplied.
	PaidDate *time.Time `json:"paid_date,omitempty"`

	// Dimensions supports optional grouping (location, department,
	// business unit, supplier category, region, ...) — see Dimension.
	Dimensions []Dimension `json:"dimensions,omitempty"`
	// SourceRef is an opaque pointer back to the originating system
	// record.
	SourceRef SourceRef `json:"source_ref,omitempty"`
}

// SupplierPayment is one portable payment event against a Payable —
// optional input used for average-days-to-pay and payment-timing
// analysis. This package does not perform payment execution/application
// accounting beyond reading these events for analytics — see the package
// doc's non-goals.
type SupplierPayment struct {
	ID         string    `json:"id"`
	PayableID  string    `json:"payable_id"`
	SupplierID string    `json:"supplier_id"`
	Date       time.Time `json:"date"`
	Amount     float64   `json:"amount"`
	SourceRef  SourceRef `json:"source_ref,omitempty"`
}
