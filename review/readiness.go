package review

import "sort"

// ReadinessState is the outcome of EvaluateReadiness: a deterministic
// verdict on whether valuation may proceed given a set of (post-Apply)
// review items.
type ReadinessState string

const (
	// ReadinessReady means no unresolved item of any severity remains.
	ReadinessReady ReadinessState = "READY"
	// ReadinessReadyWithWarnings means no unresolved BLOCKING item remains,
	// but at least one unresolved WARNING or ERROR item does — valuation
	// may proceed, but the caller should surface these to a reviewer.
	ReadinessReadyWithWarnings ReadinessState = "READY_WITH_WARNINGS"
	// ReadinessNotReady means at least one unresolved BLOCKING item
	// remains. This is the ONLY condition that produces NOT_READY — see
	// EvaluateReadiness's doc comment: this is a pure function of
	// Severity + Status, never of any other field, so it can never be
	// swayed by an unresolved WARNING/ERROR item alone, no matter how many
	// there are.
	ReadinessNotReady ReadinessState = "NOT_READY"
)

// ReasonCode is a stable identifier for one ReadinessReason, analogous to
// every other Code-style identifier in this package.
type ReasonCode string

const (
	// ReasonUnresolvedBlocking means an unresolved item with
	// Severity == SeverityBlocking exists. The only ReasonCode that can
	// ever appear alongside ReadinessNotReady.
	ReasonUnresolvedBlocking ReasonCode = "UNRESOLVED_BLOCKING_ITEM"
	// ReasonUnresolvedWarning means an unresolved item with
	// Severity == SeverityWarning exists.
	ReasonUnresolvedWarning ReasonCode = "UNRESOLVED_WARNING_ITEM"
	// ReasonUnresolvedError means an unresolved item with
	// Severity == SeverityError exists.
	ReasonUnresolvedError ReasonCode = "UNRESOLVED_ERROR_ITEM"
	// ReasonNoUsablePeriods means the item set contains zero resolvable
	// financial periods at all (every KindPeriod item remains unresolved,
	// or none were supplied) — informational context only; this by itself
	// contributes a WARNING-level reason unless the corresponding items
	// are themselves BLOCKING, in which case ReasonUnresolvedBlocking
	// already covers it.
	ReasonNoUsablePeriods ReasonCode = "NO_USABLE_PERIODS"
)

// Reason is one structured explanation contributing to a ReadinessState,
// carrying a stable code plus enough context to point back at the specific
// ReviewItem responsible — never a bare string, per section 17's
// requirement.
type Reason struct {
	Code ReasonCode `json:"code"`
	// Message is a short human-readable explanation.
	Message string `json:"message"`
	// ItemID identifies the specific ReviewItem this reason is about, when
	// applicable.
	ItemID string `json:"item_id,omitempty"`
	// Kind is the responsible item's Kind, when ItemID is set.
	Kind Kind `json:"kind,omitempty"`
}

// Readiness is the output of EvaluateReadiness.
type Readiness struct {
	// State is the overall readiness verdict.
	State ReadinessState `json:"state"`
	// Reasons explains State, in a deterministic order (see
	// EvaluateReadiness). Empty when State == ReadinessReady.
	Reasons []Reason `json:"reasons,omitempty"`
}

// EvaluateReadiness is a deterministic valuation-readiness gate over items
// (typically ApplyResult.Items, the post-decision view — but any
// []ReviewItem works, including a Plan's own Items before any decisions
// exist, in which case every Required item is naturally still unresolved).
//
// EvaluateReadiness is a PURE function of each item's Severity and Status —
// never of Reason/Title/Message text (see Severity's own doc comment's
// "Readiness must not depend on parsing free-form text" requirement).
// ReadinessNotReady occurs IF AND ONLY IF at least one unresolved item has
// Severity == SeverityBlocking. An unresolved WARNING or ERROR item can
// never by itself produce NOT_READY — it surfaces in Reasons (contributing
// to ReadinessReadyWithWarnings) but never gates readiness on its own; only
// items whose Severity resolved to SeverityBlocking under the caller's
// Policy (e.g. a reconciliation failure only reaches SeverityBlocking when
// Policy.ReconciliationFailureBlocks is true — see
// buildReconciliationItems) actually gate readiness.
func EvaluateReadiness(items []ReviewItem) Readiness {
	var reasons []Reason
	blocking := false

	sorted := make([]ReviewItem, len(items))
	copy(sorted, items)
	sortItems(sorted)

	for _, it := range sorted {
		if !it.Status.IsUnresolved() {
			continue
		}
		switch it.Severity {
		case SeverityBlocking:
			blocking = true
			reasons = append(reasons, Reason{
				Code:    ReasonUnresolvedBlocking,
				Message: "unresolved blocking review item: " + it.Title,
				ItemID:  it.ID,
				Kind:    it.Kind,
			})
		case SeverityError:
			reasons = append(reasons, Reason{
				Code:    ReasonUnresolvedError,
				Message: "unresolved error-severity review item: " + it.Title,
				ItemID:  it.ID,
				Kind:    it.Kind,
			})
		case SeverityWarning:
			reasons = append(reasons, Reason{
				Code:    ReasonUnresolvedWarning,
				Message: "unresolved warning-severity review item: " + it.Title,
				ItemID:  it.ID,
				Kind:    it.Kind,
			})
		}
	}

	state := ReadinessReady
	switch {
	case blocking:
		state = ReadinessNotReady
	case len(reasons) > 0:
		state = ReadinessReadyWithWarnings
	}

	// Reasons is built by iterating `sorted` (Severity, then Kind, then
	// SourceRowID, then ID), so BLOCKING-derived reasons already precede
	// WARNING/ERROR-derived ones. This final stable sort groups reasons by
	// ReasonCode (so every ReasonUnresolvedBlocking reason is contiguous),
	// with ItemID as a deterministic tiebreaker within a code — a small
	// additional guarantee beyond severity ordering alone, for a caller
	// that wants to group Reasons by Code directly.
	sort.SliceStable(reasons, func(i, j int) bool {
		if reasons[i].Code != reasons[j].Code {
			return reasons[i].Code < reasons[j].Code
		}
		return reasons[i].ItemID < reasons[j].ItemID
	})

	return Readiness{State: state, Reasons: reasons}
}
