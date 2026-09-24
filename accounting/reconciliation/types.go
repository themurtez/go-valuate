// Package reconciliation implements deterministic account reconciliation
// between book-side records (a caller's own ledger/subledger) and
// external/control-side records (a bank statement, lender statement,
// GL control account, intercompany counterparty, or any other
// independent source of truth for the same account).
//
// This package performs matching and reconciliation arithmetic only. It
// does not fetch bank data, parse OFX/CSV bank files, call a bank or
// accounting-system API, post journal entries, generate adjustments,
// execute payments, run OCR, manage workflow/approvals, apply AI/LLM or
// fuzzy NLP matching, or draw fraud conclusions — see "Explicit
// non-goals" in docs/ACCOUNT_RECONCILIATION.md.
//
// # Reconciliation types are metadata only
//
// [ReconciliationType] classifies what a reconciliation represents (bank,
// credit card, loan, AR/AP control, inventory control, payroll clearing,
// intercompany, suspense/clearing, or generic) purely for caller-facing
// labeling and for choosing an [Orientation]. There is exactly one
// matching/arithmetic engine underneath every type — this package never
// hard-codes an account name, a bank's file format, or type-specific
// business logic. See docs/ACCOUNT_RECONCILIATION.md's
// "One engine, nine labels" section.
//
// # Book side vs external side
//
// [BookItem] and [ExternalItem] are the two populations being reconciled.
// "Book" is always the caller's own record (a ledger, a subledger, an
// internally-maintained balance); "External" is always the independent
// control-side record (a bank statement line, a lender statement, a GL
// control balance, a counterparty's reciprocal balance). Which side is
// "book" and which is "external" is a caller choice, not something this
// package infers — see [Input].
//
// # Signed comparison convention
//
// Every [BookItem] and [ExternalItem] carries a non-negative Amount plus
// an explicit [Direction] (INFLOW or OUTFLOW), rather than a single
// signed amount. Internally, this package derives one canonical signed
// figure — SignedAmount() — as:
//
//	INFLOW  -> +Amount
//	OUTFLOW -> -Amount
//
// every arithmetic comparison (match-group differences, balance
// rollforwards, the reconciliation equation) uses this signed figure.
// The original Direction/Amount pair is always preserved and returned
// alongside it — see [BookItem.SignedAmount] and
// [ExternalItem.SignedAmount].
//
// # Orientation
//
// [Orientation] (SAME or REVERSED) declares whether the external side's
// signed convention runs in the same direction as the book side's, or in
// the opposite direction, before any comparison happens. This is
// critical for liability-type reconciliations (credit cards, loans): a
// payment that is an OUTFLOW on the book side (cash going out) is
// typically a balance-reducing INFLOW-equivalent movement on a card
// issuer's own statement. This package never assumes an asset-account
// orientation for every reconciliation type — Orientation is always an
// explicit [MatchingPolicy] field. See [MatchingPolicy.Orientation] and
// docs/ACCOUNT_RECONCILIATION.md's orientation section.
//
// # Determinism, immutability, concurrency
//
// Every exported function in this package is pure: no I/O, no mutation
// of caller-owned input (BookItem, ExternalItem, [ConfirmedMatch],
// [ReconcilingItem], [MatchingPolicy], and everything they contain are
// never modified in place — see immutability_test.go), no package-global
// mutable state, and no time.Now() call anywhere (every date this
// package reasons about — AsOfDate, item dates, staleness — is supplied
// explicitly by the caller). [Calculate] can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go and concurrency_test.go.
//
// # Currency
//
// V1 assumes one reporting currency, or values the caller has already
// converted to one reporting currency before calling this package. There
// is no FX fetching and no silent cross-currency matching — a currency
// mismatch between compared items is reported as [IssueMixedCurrency] and
// the pair is excluded from matching, never coerced.
package reconciliation

import "math"

// ReconciliationType classifies what a reconciliation represents, for
// caller-facing labeling and for choosing a sensible default
// [Orientation]/policy — it changes no matching arithmetic. This package
// implements exactly one engine; ReconciliationType never triggers a
// parallel special-case code path (see docs/ACCOUNT_RECONCILIATION.md's
// "One engine, nine labels" section, and the credit-card/loan/bank
// non-goal notes in the package doc comment).
type ReconciliationType string

const (
	TypeBank             ReconciliationType = "BANK"
	TypeCreditCard       ReconciliationType = "CREDIT_CARD"
	TypeLoan             ReconciliationType = "LOAN"
	TypeARControl        ReconciliationType = "AR_CONTROL"
	TypeAPControl        ReconciliationType = "AP_CONTROL"
	TypeInventoryControl ReconciliationType = "INVENTORY_CONTROL"
	TypePayrollClearing  ReconciliationType = "PAYROLL_CLEARING"
	TypeIntercompany     ReconciliationType = "INTERCOMPANY"
	TypeSuspenseClearing ReconciliationType = "SUSPENSE_CLEARING"
	TypeGeneric          ReconciliationType = "GENERIC"
)

// isRecognizedType reports whether t is one of the fixed
// ReconciliationType values (or empty, which this package treats as
// TypeGeneric — see [Input.effectiveType]).
func isRecognizedType(t ReconciliationType) bool {
	switch t {
	case "", TypeBank, TypeCreditCard, TypeLoan, TypeARControl, TypeAPControl,
		TypeInventoryControl, TypePayrollClearing, TypeIntercompany,
		TypeSuspenseClearing, TypeGeneric:
		return true
	default:
		return false
	}
}

// Direction is the economic direction of one BookItem/ExternalItem,
// carried separately from its non-negative Amount — see the package doc
// comment's "Signed comparison convention" section.
type Direction string

const (
	DirectionInflow  Direction = "INFLOW"
	DirectionOutflow Direction = "OUTFLOW"
)

func isRecognizedDirection(d Direction) bool {
	return d == DirectionInflow || d == DirectionOutflow
}

// signOf returns +1 for DirectionInflow, -1 for DirectionOutflow, and 0
// for an unrecognized Direction (the caller is expected to have already
// validated Direction — see [validateItems] — so this is a defensive
// fallback, never silently treated as INFLOW).
func signOf(d Direction) float64 {
	switch d {
	case DirectionInflow:
		return 1
	case DirectionOutflow:
		return -1
	default:
		return 0
	}
}

// Orientation declares whether the external side's signed convention
// runs the same way as the book side's (SAME) or in the opposite
// direction (REVERSED) — see the package doc comment's "Orientation"
// section. Applied only to the external side's SignedAmount before any
// cross-side comparison; the book side's own sign is never flipped.
type Orientation string

const (
	OrientationSame     Orientation = "SAME"
	OrientationReversed Orientation = "REVERSED"
)

func isRecognizedOrientation(o Orientation) bool {
	return o == "" || o == OrientationSame || o == OrientationReversed
}

// orientationMultiplier returns -1 for OrientationReversed, +1 otherwise
// (including the zero value, which this package treats as
// OrientationSame — the safe default for an asset-style account).
func orientationMultiplier(o Orientation) float64 {
	if o == OrientationReversed {
		return -1
	}
	return 1
}

// SourceRef is an opaque, caller-defined pointer back to the originating
// system record (e.g. a bank's own transaction ID, a lender statement
// line reference, an ERP journal-line key). This package never
// interprets it — mirrors ledger.JournalLine.SourceRef's identical
// "opaque reference" convention used throughout this repository's
// accounting/* packages.
type SourceRef struct {
	System string `json:"system,omitempty"`
	ID     string `json:"id,omitempty"`
}

// BookItem is one portable, normalized book-side transaction (a ledger
// line, a subledger entry, an internally-recorded movement). Amount is
// expected to be >= 0; Direction carries the sign — see the package doc
// comment's "Signed comparison convention" section.
type BookItem struct {
	// ItemID uniquely identifies this item within one Input. Required;
	// duplicates are flagged (see IssueDuplicateBookItem) and only the
	// first occurrence (in input order) is used.
	ItemID string `json:"item_id"`
	// Date is this item's transaction date, "YYYY-MM-DD".
	Date string `json:"date"`
	// Amount is this item's magnitude. Expected >= 0; Direction supplies
	// the sign (see SignedAmount). A negative Amount is flagged (see
	// IssueInvalidAmount) but does not by itself exclude the item — the
	// item is still matched using its literal signed value so a caller's
	// pre-signed data is never silently altered, only warned about.
	Amount float64 `json:"amount"`
	// Direction is this item's economic direction — see Direction.
	Direction Direction `json:"direction"`
	// Reference is this item's own reference/check-number/transaction
	// code, used for reference-based matching — see NormalizeReference.
	Reference string `json:"reference,omitempty"`
	// Description is a human-readable label, used for matching only when
	// MatchingPolicy.DescriptionExactMatchEnabled is true — see the
	// package doc comment's "Description boundary" (docs section 15).
	Description string `json:"description,omitempty"`
	// AccountID is the book-side account this item belongs to. Carried
	// through for display/provenance; Input.AccountID is the
	// reconciliation's own identity key, not this field.
	AccountID string `json:"account_id,omitempty"`
	// Currency is this item's ISO 4217-style currency code. Required for
	// cross-currency-mismatch detection — see IssueMixedCurrency.
	Currency string `json:"currency,omitempty"`
	// SourceType identifies what produced this item (e.g. "ledger", "ar",
	// "ap", "manual"). Open string, caller-defined.
	SourceType string `json:"source_type,omitempty"`
	// SourceID is an opaque pointer to this item's origin record within
	// SourceType (e.g. a ledger.JournalEntry.ID). Never interpreted.
	SourceID string `json:"source_id,omitempty"`
	// SourceRef is a structured opaque provenance pointer, for callers
	// that want a (system, id) pair rather than two separate strings —
	// both SourceID and SourceRef may be populated; neither is required.
	SourceRef SourceRef `json:"source_ref,omitempty"`
}

// SignedAmount returns b's canonical signed figure: +Amount for
// DirectionInflow, -Amount for DirectionOutflow — see the package doc
// comment's "Signed comparison convention" section. Returns 0 for an
// unrecognized Direction (defensive; validation flags this separately).
func (b BookItem) SignedAmount() float64 {
	return signOf(b.Direction) * b.Amount
}

// ExternalItem is one portable, normalized external/control-side
// transaction (a bank statement line, a lender statement line, a GL
// control-account posting, a counterparty's reciprocal record). Shape
// mirrors BookItem exactly — see the package doc comment's "Book side vs
// external side" section for why they are two distinct types rather than
// one shared type with a "side" flag: keeping them distinct lets every
// matching function's signature state, unambiguously, which population
// an argument belongs to.
type ExternalItem struct {
	ItemID      string    `json:"item_id"`
	Date        string    `json:"date"`
	Amount      float64   `json:"amount"`
	Direction   Direction `json:"direction"`
	Reference   string    `json:"reference,omitempty"`
	Description string    `json:"description,omitempty"`
	AccountID   string    `json:"account_id,omitempty"`
	Currency    string    `json:"currency,omitempty"`
	SourceType  string    `json:"source_type,omitempty"`
	SourceID    string    `json:"source_id,omitempty"`
	SourceRef   SourceRef `json:"source_ref,omitempty"`
}

// SignedAmount returns e's canonical signed figure, before any
// Orientation adjustment — see BookItem.SignedAmount and the package doc
// comment's "Orientation" section for where Orientation is applied
// (always at the point of cross-side comparison, never mutating the item
// itself).
func (e ExternalItem) SignedAmount() float64 {
	return signOf(e.Direction) * e.Amount
}

// OrientedSignedAmount returns e.SignedAmount() adjusted by o — the
// figure actually used in every cross-side comparison.
func (e ExternalItem) OrientedSignedAmount(o Orientation) float64 {
	return e.SignedAmount() * orientationMultiplier(o)
}

// isNonFinite reports whether v is NaN or +/-Inf — the shared guard
// every money-bearing field in this package is checked against, per this
// repository's "reject NaN/Inf" money-safety convention (see e.g.
// ledger.isNonFinite).
func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
