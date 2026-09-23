// Package ar implements deterministic accounts-receivable aging and
// collections analytics from a portable open-receivables model.
//
// This package is application-independent, like every analytics-style
// package in this repository: it does not require accounting/ledger or
// accounting/statements input at all. A caller can populate it directly
// from a QuickBooks/Xero export, a homegrown billing system, or a synthetic
// fixture — see accounting/ar/fixtures. A future statements/ledger-aware
// adapter (not part of this package) may bridge accounting/ledger accounts
// receivable balances into Receivable values, but this package itself never
// imports accounting/ledger.
//
// This package contains no persistence, HTTP/API handlers, auth, UI,
// background jobs, QuickBooks/Xero integration, email/collections
// automation, AI/LLM, tax logic, or credit-bureau logic — see the package
// doc's "Explicit non-goals" section in docs/AR_AGING.md.
//
// # No hidden current-date dependency
//
// Every aging calculation takes an explicit AsOfDate from caller input.
// Nothing in this package calls time.Now() — this is essential for
// reproducibility and for building historical snapshots (see
// determinism_test.go).
//
// Every exported function is pure: no I/O, no mutation of caller-owned
// input (Receivable, Payment, BucketDefinition, and every slice/map they
// appear in are never modified in place — see immutability_test.go), no
// package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package ar

import "time"

// ReceivableStatus is the lifecycle state of one Receivable. Only
// OPEN/PARTIALLY_PAID receivables are economically outstanding; the others
// exist so a caller's full receivables export (including history) can be
// passed through without pre-filtering, while this package still applies
// its own default inclusion policy — see AgingOptions.IncludeStatuses and
// defaultAgingStatuses.
type ReceivableStatus string

const (
	StatusOpen          ReceivableStatus = "OPEN"
	StatusPartiallyPaid ReceivableStatus = "PARTIALLY_PAID"
	StatusPaid          ReceivableStatus = "PAID"
	StatusVoided        ReceivableStatus = "VOIDED"
	StatusWrittenOff    ReceivableStatus = "WRITTEN_OFF"
	StatusDisputed      ReceivableStatus = "DISPUTED"
)

// isRecognizedStatus reports whether s is one of the fixed ReceivableStatus
// values.
func isRecognizedStatus(s ReceivableStatus) bool {
	switch s {
	case StatusOpen, StatusPartiallyPaid, StatusPaid, StatusVoided, StatusWrittenOff, StatusDisputed:
		return true
	default:
		return false
	}
}

// defaultAgingStatuses returns the statuses that are "economically
// relevant open receivables" by default per the task's instruction: OPEN,
// PARTIALLY_PAID, and DISPUTED (a disputed invoice still ages — see the
// package doc's disputed-receivables section). PAID, VOIDED, and
// WRITTEN_OFF are excluded by default since they no longer represent
// outstanding exposure, though WRITTEN_OFF items remain visible via
// WriteOffSummary when supplied.
func defaultAgingStatuses() []ReceivableStatus {
	return []ReceivableStatus{StatusOpen, StatusPartiallyPaid, StatusDisputed}
}

// DocumentType distinguishes an ordinary invoice from a credit memo, per
// the task's explicit "do not treat every negative AR balance as invalid"
// instruction. A credit memo's OpenAmount is expected to be <= 0 (a credit
// owed to the customer); an invoice's OpenAmount is expected to be >= 0.
// Zero value ("") is treated as DocumentTypeInvoice — see
// resolvedDocumentType.
type DocumentType string

const (
	DocumentTypeInvoice    DocumentType = "INVOICE"
	DocumentTypeCreditMemo DocumentType = "CREDIT_MEMO"
)

// resolvedDocumentType returns d if recognized, otherwise
// DocumentTypeInvoice — the safe default for a caller's existing invoice
// data that predates this field.
func resolvedDocumentType(d DocumentType) DocumentType {
	if d == DocumentTypeCreditMemo {
		return d
	}
	return DocumentTypeInvoice
}

// SourceRef is an opaque, caller-defined pointer back to the originating
// system record (e.g. a QuickBooks transaction ID or an ERP invoice key).
// This package never interprets it — mirrors ledger.JournalLine.SourceRef's
// identical "opaque reference" convention.
type SourceRef struct {
	System string `json:"system,omitempty"`
	ID     string `json:"id,omitempty"`
}

// Dimension is one lightweight, optional analysis tag on a Receivable —
// location, department, sales rep, customer segment, or region. Key is an
// open string rather than a closed enum, mirroring ledger.Dimension's
// identical rationale: this package never hard-codes a dimension taxonomy.
type Dimension struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Conventional Dimension.Key values. These are suggestions, not a closed
// set.
const (
	DimensionLocation   string = "location"
	DimensionDepartment string = "department"
	DimensionSalesRep   string = "sales_rep"
	DimensionSegment    string = "segment"
	DimensionRegion     string = "region"
)

// Receivable is one portable open-item / invoice-level record — the
// primary input to this package. A caller assembles a slice of these from
// whatever billing/AR source it has; this package has no opinion on where
// it came from.
type Receivable struct {
	// ID uniquely identifies this receivable within one analysis. Required;
	// duplicates are flagged (see IssueDuplicateReceivable) and only the
	// first occurrence (in input order) is used.
	ID string `json:"id"`
	// CustomerID is an opaque, caller-assigned identifier for the customer.
	// Required — see CustomerName's doc comment for why this package treats
	// CustomerID, not CustomerName, as the identity key.
	CustomerID string `json:"customer_id"`
	// CustomerName is an optional human-readable label, carried through for
	// display only. Never used as an identity key: two Receivables with the
	// same CustomerID but different CustomerName strings are still the same
	// customer, and a caller who only has opaque IDs (no names at all) gets
	// a fully functional analysis.
	CustomerName string `json:"customer_name,omitempty"`
	// InvoiceNumber is an optional human-readable invoice label, carried
	// through for display only.
	InvoiceNumber string `json:"invoice_number,omitempty"`

	// DocumentType distinguishes an invoice from a credit memo — see
	// DocumentType.
	DocumentType DocumentType `json:"document_type,omitempty"`

	InvoiceDate time.Time `json:"invoice_date"`
	DueDate     time.Time `json:"due_date"`

	// OriginalAmount is the receivable's original face amount at issuance.
	// Expected >= 0 for an invoice, <= 0 for a credit memo (see
	// DocumentType) — a sign mismatch is flagged (see IssueInvalidAmount)
	// but does not by itself exclude the row.
	OriginalAmount float64 `json:"original_amount"`
	// OpenAmount is the amount still outstanding as of the data snapshot
	// this Receivable represents. This is the amount aging is always
	// computed from (never OriginalAmount) — see the package doc comment's
	// "open balance semantics" section.
	OpenAmount float64 `json:"open_amount"`

	// Currency is this receivable's ISO 4217-style currency code (e.g.
	// "USD"). Required; a mix of currencies within one analysis is flagged
	// unless resolved — see Options.ReportingCurrency and
	// IssueMixedCurrency.
	Currency string `json:"currency"`

	Status ReceivableStatus `json:"status"`

	// TermsDays is the contractual payment terms in days (e.g. 30 for
	// "net 30"), used by TermsAnalysis. Optional.
	TermsDays int `json:"terms_days,omitempty"`
	// PaidDate is when this receivable was fully paid, if it has been.
	// Optional; used only for payment-timing analysis when Payments are not
	// separately supplied.
	PaidDate *time.Time `json:"paid_date,omitempty"`

	// Dimensions supports optional grouping (location, department, sales
	// rep, segment, region, ...) — see Dimension.
	Dimensions []Dimension `json:"dimensions,omitempty"`
	// SourceRef is an opaque pointer back to the originating system record.
	SourceRef SourceRef `json:"source_ref,omitempty"`
}

// Payment is one portable payment event against a Receivable — optional
// input used for average-days-to-pay and collection-timing analysis. This
// package does not perform payment application/accounting logic beyond
// reading these events for analytics (see the package doc's non-goals).
type Payment struct {
	ID           string    `json:"id"`
	ReceivableID string    `json:"receivable_id"`
	CustomerID   string    `json:"customer_id"`
	Date         time.Time `json:"date"`
	Amount       float64   `json:"amount"`
	SourceRef    SourceRef `json:"source_ref,omitempty"`
}
