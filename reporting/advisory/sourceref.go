package advisory

// SourceRef is a pointer from a composed [Metric]/[Insight]/[ActionItem]
// back to the specific sibling-package fact it was built from — the
// mechanism behind task section 36's "a future user should be able to
// answer 'why is this action here?' without reverse engineering"
// requirement and section 57's "no recalculation drift" invariant.
// SourceRef never carries an opinion of its own; it is purely a provenance
// coordinate.
type SourceRef struct {
	// Module is a stable, fixed name for the sibling package this fact
	// came from (e.g. "ar", "cashforecast", "reconciliation",
	// "closechecklist") — matches SourceVersions' field naming.
	Module string `json:"module"`
	// Code is the source package's own stable identifier for this fact,
	// copied as a plain string so this package need not import a dozen
	// distinct code types for one field — mirrors
	// reporting/management.TopIssue.Code's identical "copy verbatim as a
	// plain string" convention. Examples: an ar.FlagCode's string value, a
	// reconciliation.FindingCode's string value, a closechecklist Task ID,
	// a covenants.TestResult.CovenantID.
	Code string `json:"code,omitempty"`
	// Ref is an opaque, caller/source-defined pointer to a more specific
	// record than Code alone identifies (e.g. one CustomerID, one
	// SupplierID, one EntityID, one AccountID) — carried through, never
	// interpreted.
	Ref string `json:"ref,omitempty"`
	// Period is the period this fact concerns, when the source fact is
	// period-specific. Left empty for a point-in-time source (e.g.
	// analytics/debt, accounting/reconciliation's as-of balances).
	Period string `json:"period,omitempty"`
}

// Evidence is one supporting data point behind an [Insight] — a small,
// typed fact (a value, a threshold, a count) that a caller can inspect
// without having to re-derive it from a Statement's rendered text. Task
// section 7's Insight.Evidence field.
type Evidence struct {
	Label string `json:"label"`
	Value Value  `json:"value"`
	Unit  Unit   `json:"unit,omitempty"`
}
