package reconciliation

// Severity is a Finding's review-priority level. Deliberately a simple
// two-level scale (unlike closequality's three-level BLOCKING/WARNING/INFO)
// because this package renders no readiness verdict of its own — Status
// (see status.go) already carries the "is this reconciled" answer, so a
// Finding's Severity only needs to say how much review attention it
// warrants, not whether it blocks anything.
type Severity string

const (
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// FindingCode is a stable identifier for a reconciliation condition
// needing review — as opposed to IssueCode, which identifies a problem
// with the Input/MatchingPolicy itself. Only the codes below are ever
// emitted — this is a closed, stable set (task section 55: "only define
// emitted codes").
type FindingCode string

const (
	FindingBalanceMismatch               FindingCode = "BALANCE_MISMATCH"
	FindingMaterialUnmatchedBookItem     FindingCode = "MATERIAL_UNMATCHED_BOOK_ITEM"
	FindingMaterialUnmatchedExternalItem FindingCode = "MATERIAL_UNMATCHED_EXTERNAL_ITEM"
	FindingAmbiguousMatch                FindingCode = "AMBIGUOUS_MATCH"
	FindingStaleUnmatchedItem            FindingCode = "STALE_UNMATCHED_ITEM"
	FindingStaleReconcilingItem          FindingCode = "STALE_RECONCILING_ITEM"
	FindingBookRollforwardMismatch       FindingCode = "BOOK_ROLLFORWARD_MISMATCH"
	FindingExternalRollforwardMismatch   FindingCode = "EXTERNAL_ROLLFORWARD_MISMATCH"
	FindingMaterialReconcilingItems      FindingCode = "MATERIAL_RECONCILING_ITEMS"
)

// Finding is one reconciliation condition this package surfaces for
// caller review — a factual observation, never a workflow assignment,
// journal-entry suggestion, or fraud conclusion (see the package doc
// comment's "Neutral language" requirement, locked by
// neutral_language_test.go).
type Finding struct {
	Code     FindingCode `json:"code"`
	Severity Severity    `json:"severity"`
	Message  string      `json:"message"`
	// ItemIDs, when set, are the BookItem/ExternalItem/ReconcilingItem IDs
	// this finding concerns.
	ItemIDs []string `json:"item_ids,omitempty"`
	// Amount is the dollar figure most relevant to this finding (an
	// unmatched item's amount, a balance difference, a reconciling-item
	// total) — always the plain signed or absolute figure the specific
	// FindingCode's doc comment above describes; never a computed score.
	Amount float64 `json:"amount,omitempty"`
	// DaysOutstanding/DaysStale, when applicable, is the age computed
	// against Input.AsOfDate — see aging.go.
	DaysOutstanding int `json:"days_outstanding,omitempty"`
}
