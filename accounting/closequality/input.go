package closequality

import (
	"github.com/themurtez/go-valuate/accounting/ap"
	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
)

// Input holds every upstream result this package may compose. Following
// analytics/diagnostics's precedent (the closest existing multi-sibling
// aggregator in this repository), every sibling Result is a plain value,
// not a pointer — each sibling Result already carries its own
// availability signal (ar.Result.Available, ap.Result.Available,
// statements.Result's per-statement DatasetAvailability), and a caller
// with nothing for a module simply leaves that field at its zero value.
// ledger.Ledger has no such Available concept (it is raw domain input,
// not a computed Result), so its presence is instead signaled by
// LedgerProvided.
type Input struct {
	Period PeriodInfo `json:"period"`

	// Ledger and LedgerProvided: because ledger.Ledger{} (no accounts, no
	// entries) is itself a valid, meaningful value, this package cannot
	// use "zero value" to mean "not supplied" the way it does for the
	// sibling Results below. LedgerProvided is the explicit signal.
	Ledger         ledger.Ledger `json:"ledger,omitempty"`
	LedgerProvided bool          `json:"ledger_provided"`

	// LedgerValidation, if supplied, is used as-is (this package never
	// re-runs ledger.Ledger.Validate itself, per the "reuse, don't
	// recalculate" rule) — a caller who already validated Ledger
	// elsewhere can pass that result directly. If Ledger is provided but
	// LedgerValidation is nil, LedgerIntegrity is still assessed by
	// running ledger.Ledger.Validate(ledger.ValidateOptions{}) with
	// zero-value options, matching Calculate's documented default.
	LedgerValidation []ledger.Issue `json:"ledger_validation,omitempty"`

	Statements         statements.Result         `json:"statements"`
	AR                 ar.Result                 `json:"ar"`
	AP                 ap.Result                 `json:"ap"`
	JournalDiagnostics journaldiagnostics.Result `json:"journal_diagnostics"`

	Reconciliations []ReconciliationStatus `json:"reconciliations,omitempty"`
	CloseTasks      []CloseTaskStatus      `json:"close_tasks,omitempty"`

	// TotalAssets/TotalRevenue, if supplied, feed MaterialityPolicy's
	// PercentOfAssets/PercentOfRevenue bases. Optional — those
	// materiality legs are simply unavailable without them (see
	// isMaterial).
	TotalAssets  *float64 `json:"total_assets,omitempty"`
	TotalRevenue *float64 `json:"total_revenue,omitempty"`
}

func (in Input) statementsAvailable() bool {
	return in.Statements.DatasetAvailability != "" &&
		in.Statements.DatasetAvailability != statements.AvailabilityNotRequested
}

func (in Input) arAvailable() bool { return in.AR.Available }
func (in Input) apAvailable() bool { return in.AP.Available }

// journalDiagnosticsAvailable treats the presence of a non-empty Period
// as the availability signal, since journaldiagnostics.Result has no
// top-level Available field (see the survey: its Coverage/RuleAvailability
// serve that role for individual rules, but Period is always populated
// by a real Calculate call and never by a caller's zero-value Result{}).
func (in Input) journalDiagnosticsAvailable() bool {
	return in.JournalDiagnostics.Period != ""
}
