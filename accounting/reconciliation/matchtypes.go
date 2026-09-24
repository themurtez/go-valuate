package reconciliation

// MatchType classifies the cardinality of one MatchGroup — how many book
// items matched how many external items.
type MatchType string

const (
	MatchOneToOne              MatchType = "ONE_TO_ONE"
	MatchOneBookToManyExternal MatchType = "ONE_BOOK_TO_MANY_EXTERNAL"
	MatchManyBookToOneExternal MatchType = "MANY_BOOK_TO_ONE_EXTERNAL"
	MatchManualGroup           MatchType = "MANUAL_GROUP"
)

// MatchConfidence is a deterministic, descriptive classification of how a
// MatchGroup was established — never a probability, never a score (task
// section 10: "Confidence is deterministic/descriptive... not a
// probability").
type MatchConfidence string

const (
	// ConfidenceExact: a caller-confirmed match, or an automatic match on
	// exact normalized reference + exact amount + compatible date.
	ConfidenceExact MatchConfidence = "EXACT"
	// ConfidenceStrong: an automatic match on exact amount + same date, or
	// exact amount + reference (without date compatibility having been
	// separately confirmed).
	ConfidenceStrong MatchConfidence = "STRONG"
	// ConfidenceReview: an automatic match established with weaker
	// evidence (amount within the caller's date window, or a bounded
	// composite-sum match) — matched, but worth a caller's review pass.
	ConfidenceReview MatchConfidence = "REVIEW"
	// ConfidenceManual: a caller-confirmed ConfirmedMatch that this
	// package's own evidence rules would not have produced on their own
	// (e.g. amounts differ beyond MatchingPolicy tolerance, explicitly
	// accepted by the caller as an explained difference).
	ConfidenceManual MatchConfidence = "MANUAL"
)

// MatchReason is a stable, closed identifier for exactly which rule
// established a MatchGroup — no message parsing required (task section
// 24).
type MatchReason string

const (
	ReasonConfirmed                MatchReason = "CONFIRMED"
	ReasonExactReferenceAmountDate MatchReason = "EXACT_REFERENCE_AMOUNT_DATE"
	ReasonExactReferenceAmount     MatchReason = "EXACT_REFERENCE_AMOUNT"
	ReasonExactAmountDate          MatchReason = "EXACT_AMOUNT_DATE"
	ReasonExactAmountWithinWindow  MatchReason = "EXACT_AMOUNT_WITHIN_WINDOW"
	ReasonCompositeSumMatch        MatchReason = "COMPOSITE_SUM_MATCH"
)

// MatchGroup is one resolved set of book items and external items
// determined to correspond to the same underlying economic event(s).
type MatchGroup struct {
	// MatchID is a stable, deterministically-assigned identifier —
	// "M" + a zero-padded ordinal in this Result's own deterministic
	// output order (see sort.go). Not stable across different Inputs;
	// stable and reproducible for the identical Input (see
	// determinism_test.go).
	MatchID string `json:"match_id"`

	BookItemIDs     []string `json:"book_item_ids"`
	ExternalItemIDs []string `json:"external_item_ids"`

	// BookAmount/ExternalAmount are the summed SignedAmount()/
	// OrientedSignedAmount() of the group's member items respectively.
	BookAmount     float64 `json:"book_amount"`
	ExternalAmount float64 `json:"external_amount"`
	// Difference is BookAmount - ExternalAmount for this group.
	Difference float64 `json:"difference"`

	MatchType   MatchType       `json:"match_type"`
	MatchReason MatchReason     `json:"match_reason"`
	Confidence  MatchConfidence `json:"confidence"`

	// DateDifferenceDays is the absolute day difference between the
	// (single) book item's date and the (single) external item's date,
	// for a ONE_TO_ONE group; for a composite/multi-item group, the
	// largest pairwise absolute day difference among the group's book
	// item(s) and external item(s), so it always represents the group's
	// worst-case date spread rather than an arbitrary single pair.
	DateDifferenceDays int `json:"date_difference_days"`
	// ReferenceMatched reports whether normalized-reference equality
	// contributed to establishing this group (see normalize.go). False
	// for amount/date-only matches and for CONFIRMED groups where
	// reference was not part of the caller's stated evidence.
	ReferenceMatched bool `json:"reference_matched"`
	// NormalizedReference is the normalized reference string used, when
	// ReferenceMatched is true — task section 14: "Return normalized
	// reference in evidence when used."
	NormalizedReference string `json:"normalized_reference,omitempty"`
}

// Candidate is one bounded, factual match possibility returned for an
// item this package could not resolve to a MatchGroup — evidence only,
// no UI, no ranking beyond the deterministic Rule/amount/date facts
// themselves (task section 23).
type Candidate struct {
	// BookItemID/ExternalItemID identify the specific pair (or, for a
	// composite candidate, ExternalItemIDs holds every external item in
	// the proposed sum against one BookItemID, and vice versa for the
	// reverse direction — exactly one of ExternalItemID/ExternalItemIDs
	// and BookItemID/BookItemIDs is populated per Candidate, matching
	// whichever side is the single anchor item).
	BookItemID      string   `json:"book_item_id,omitempty"`
	BookItemIDs     []string `json:"book_item_ids,omitempty"`
	ExternalItemID  string   `json:"external_item_id,omitempty"`
	ExternalItemIDs []string `json:"external_item_ids,omitempty"`

	Rule MatchReason `json:"rule"`

	AmountDifference   float64 `json:"amount_difference"`
	DateDifferenceDays int     `json:"date_difference_days"`

	ReferenceEvidence string `json:"reference_evidence,omitempty"`

	// Reason further identifies why this candidate is unresolved rather
	// than an accepted match — e.g. "AMBIGUOUS_WITH_OTHER_CANDIDATE" when
	// two+ equally valid candidates exist at the same precedence rule.
	// Open string for factual context only; not a new closed enum, since
	// this field is descriptive evidence text, never a driver of
	// downstream logic.
	Reason string `json:"reason,omitempty"`
}

// ReconcilingItemType classifies one caller-provided explicit reconciling
// item — never inferred, never auto-generated by this package.
type ReconcilingItemType string

const (
	ReconcilingOutstandingCheck   ReconcilingItemType = "OUTSTANDING_CHECK"
	ReconcilingDepositInTransit   ReconcilingItemType = "DEPOSIT_IN_TRANSIT"
	ReconcilingUnrecordedFee      ReconcilingItemType = "UNRECORDED_FEE"
	ReconcilingUnrecordedInterest ReconcilingItemType = "UNRECORDED_INTEREST"
	ReconcilingTimingDifference   ReconcilingItemType = "TIMING_DIFFERENCE"
	ReconcilingBookAdjustment     ReconcilingItemType = "BOOK_ADJUSTMENT"
	ReconcilingExternalAdjustment ReconcilingItemType = "EXTERNAL_ADJUSTMENT"
	ReconcilingOther              ReconcilingItemType = "OTHER"
)

func isRecognizedReconcilingType(t ReconcilingItemType) bool {
	switch t {
	case ReconcilingOutstandingCheck, ReconcilingDepositInTransit, ReconcilingUnrecordedFee,
		ReconcilingUnrecordedInterest, ReconcilingTimingDifference, ReconcilingBookAdjustment,
		ReconcilingExternalAdjustment, ReconcilingOther:
		return true
	default:
		return false
	}
}

// ReconcilingSide is which balance a ReconcilingItem adjusts — the book
// side or the external side. See equation.go's reconciliation-equation
// formula.
type ReconcilingSide string

const (
	ReconcilingSideBook     ReconcilingSide = "BOOK"
	ReconcilingSideExternal ReconcilingSide = "EXTERNAL"
)

// ReconcilingItem is one caller-provided, explicit item that explains
// part of the difference between the book and external balances (e.g. an
// outstanding check, a deposit in transit, a bank fee not yet recorded on
// the books). This package never creates, infers, or auto-generates a
// ReconcilingItem, and never posts a journal entry for one — see the
// package doc comment.
type ReconcilingItem struct {
	// ItemID uniquely identifies this reconciling item within one Input.
	// Required; duplicates are flagged (see IssueDuplicateReconcilingItem
	// via IssueInvalidReconcilingItem) and only the first occurrence is
	// used.
	ItemID string              `json:"item_id"`
	Type   ReconcilingItemType `json:"type"`
	// Side is which balance this item adjusts — see ReconcilingSide.
	Side ReconcilingSide `json:"side"`
	// Amount is this item's signed effect on Side's adjusted balance
	// (already signed correctly for direct addition — see equation.go;
	// this package never re-derives or flips its sign).
	Amount      float64 `json:"amount"`
	Date        string  `json:"date,omitempty"`
	Description string  `json:"description,omitempty"`
	// RelatedItemID, if set, points to the BookItem/ExternalItem this
	// reconciling item explains the absence/timing of (e.g. an
	// OUTSTANDING_CHECK's RelatedItemID is the BookItem that has not yet
	// cleared externally). Optional and never validated against a
	// specific side's item list — a caller may reference either side.
	RelatedItemID string `json:"related_item_id,omitempty"`
}
