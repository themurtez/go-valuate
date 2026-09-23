// Package closequality provides deterministic bookkeeping-quality and
// period-close-readiness diagnostics for a single accounting period. It
// sits above [github.com/themurtez/go-valuate/accounting/ledger],
// [github.com/themurtez/go-valuate/accounting/statements],
// [github.com/themurtez/go-valuate/accounting/ar],
// [github.com/themurtez/go-valuate/accounting/ap], and
// [github.com/themurtez/go-valuate/accounting/journaldiagnostics],
// composing their already-computed outputs into one structured answer to
// five questions: are the books structurally sound, are key balances
// plausible, are there unresolved bookkeeping issues, is the period ready
// for controller/accountant review, and what specifically still needs
// attention before close.
//
// # Not an audit opinion, not a reconciliation engine
//
// This package renders no audit opinion, performs no tax calculation, and
// does not implement fraud detection. It also does not implement account
// reconciliation (bank, credit-card, loan, or otherwise) — it only
// consumes externally- or manually-computed [ReconciliationStatus] values
// a caller supplies. A dedicated reconciliation-matching engine is a
// separate, later package; this package's [ReconciliationStatus] and
// [CloseTaskStatus] inputs are deliberately lightweight status carriers,
// not workflow or matching engines.
//
// # Composition, not recalculation
//
// This package computes no AR/AP aging math, no journal-entry anomaly
// detection, no financial-statement mapping, and no ledger validation of
// its own — it reuses the typed outputs of accounting/ledger,
// accounting/statements, accounting/ar, accounting/ap, and
// accounting/journaldiagnostics, translating their existing
// issues/findings/reconciliations into close-quality [Finding]s that
// preserve [Finding.SourceModule]/[Finding.SourceCode] provenance. See
// [Input] for how each upstream module is optional and how missing
// modules become [DimensionUnassessed] rather than automatically bad.
//
// # Determinism
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no wall-clock reads (time.Now is never called — see [PeriodInfo] and
// [PeriodLockState]), no package-global mutable state. [Calculate] can be
// called concurrently and repeatedly against identical input and always
// returns byte-for-byte identical JSON — see determinism_test.go.
package closequality

import "time"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown
// because a required input was absent." This package's own local copy of
// the convention every sibling package duplicates rather than importing
// (see journaldiagnostics.Value's doc comment for the full rationale);
// this package follows journaldiagnostics.Value's Amount naming since it
// is the closest sibling this package composes.
type Value struct {
	Available bool    `json:"available"`
	Amount    float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may
// legitimately be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// Severity is a Finding's review-priority level, mirroring the
// three-level BLOCKING/WARNING/INFO scale requested for close-quality
// findings — distinct in name from journaldiagnostics.Severity's
// INFO/WARNING/HIGH scale because "blocking" (prevents close readiness)
// is a different axis than "high" (review priority within an already-open
// period).
type Severity string

const (
	SeverityBlocking Severity = "BLOCKING"
	SeverityWarning  Severity = "WARNING"
	SeverityInfo     Severity = "INFO"
)

var severityRank = map[Severity]int{
	SeverityBlocking: 0,
	SeverityWarning:  1,
	SeverityInfo:     2,
}

// SourceModule identifies which upstream package a Finding or an Issue's
// evidence was translated from, or "" when the finding/issue originates
// in this package itself (e.g. a reconciliation-coverage or close-task
// finding that has no upstream sibling equivalent).
type SourceModule string

const (
	SourceLedger             SourceModule = "accounting/ledger"
	SourceStatements         SourceModule = "accounting/statements"
	SourceAR                 SourceModule = "accounting/ar"
	SourceAP                 SourceModule = "accounting/ap"
	SourceJournalDiagnostics SourceModule = "accounting/journaldiagnostics"
	SourceCloseQuality       SourceModule = "accounting/closequality"
)

// PeriodInfo identifies the accounting period this Result assesses.
// Callers must supply explicit dates; this package never calls
// time.Now() and never infers a fiscal period from data.
type PeriodInfo struct {
	// Period is the caller's own period label (e.g. "2025-06" or "FY2025-Q2").
	Period string `json:"period"`
	// StartDate and EndDate bound the period, inclusive, "YYYY-MM-DD".
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	// CloseDate, if supplied, is the date the period was (or is intended
	// to be) closed, "YYYY-MM-DD". Required for post-close-activity
	// detection (see journaldiagnostics.PeriodWindow.CloseDate, which this
	// package derives from the same value when composing journal
	// diagnostics itself, or accepts directly when the caller already ran
	// journaldiagnostics separately).
	CloseDate string `json:"close_date,omitempty"`
	// Lock is the caller-supplied period lock state. This package never
	// enforces locking; it only uses Lock to decide how severely to treat
	// post-close activity (see Policy.TreatLockedPostCloseAsBlocking).
	Lock PeriodLockState `json:"lock,omitempty"`
}

// PeriodLockState is a caller-supplied declaration of whether a period is
// still open for posting. This package implements no locking mechanism;
// it only reads this value to calibrate severity of post-close findings.
type PeriodLockState string

const (
	PeriodOpen       PeriodLockState = "OPEN"
	PeriodSoftClosed PeriodLockState = "SOFT_CLOSED"
	PeriodClosed     PeriodLockState = "CLOSED"
	PeriodLocked     PeriodLockState = "LOCKED"
)

func (p PeriodInfo) valid() bool {
	if p.Period == "" || p.StartDate == "" || p.EndDate == "" {
		return false
	}
	start, err := time.Parse("2006-01-02", p.StartDate)
	if err != nil {
		return false
	}
	end, err := time.Parse("2006-01-02", p.EndDate)
	if err != nil {
		return false
	}
	if end.Before(start) {
		return false
	}
	if p.CloseDate != "" {
		if _, err := time.Parse("2006-01-02", p.CloseDate); err != nil {
			return false
		}
	}
	switch p.Lock {
	case "", PeriodOpen, PeriodSoftClosed, PeriodClosed, PeriodLocked:
	default:
		return false
	}
	return true
}

// isLocked reports whether p's lock state means posting is not expected
// to still be occurring (CLOSED or LOCKED).
func (p PeriodInfo) isLocked() bool {
	return p.Lock == PeriodClosed || p.Lock == PeriodLocked
}
