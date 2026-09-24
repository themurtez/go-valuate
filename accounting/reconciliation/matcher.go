package reconciliation

import (
	"strings"
	"time"
)

// matchState tracks, across the whole matching pass, which book/external
// items have already been consumed by a MatchGroup — shared by every
// precedence stage so no item is ever double-consumed (task section 30's
// "duplicate handling cannot exclude the wrong first occurrence" and the
// package's own "match exclusivity" invariant).
type matchState struct {
	usedBook     map[string]bool
	usedExternal map[string]bool
}

func newMatchState(bookCount, externalCount int) *matchState {
	return &matchState{
		usedBook:     make(map[string]bool, bookCount),
		usedExternal: make(map[string]bool, externalCount),
	}
}

func (s *matchState) bookAvailable(id string) bool     { return !s.usedBook[id] }
func (s *matchState) externalAvailable(id string) bool { return !s.usedExternal[id] }
func (s *matchState) consumeBook(id string)            { s.usedBook[id] = true }
func (s *matchState) consumeExternal(id string)        { s.usedExternal[id] = true }

// runAutoMatching applies the matching-precedence stages (task section
// 13, stages 2-4; stage 1 CONFIRMED and stage 5 composite are applied by
// the caller separately — see calculate.go) against every book/external
// item not yet consumed in state: exact normalized reference + amount +
// compatible date, then exact amount + same date, then exact amount +
// date within window. Each stage only ever proposes a ONE_TO_ONE group.
// Within a stage, if an item has more than one equally valid candidate
// under that stage's own rule, it is left unresolved (ambiguous) rather
// than arbitrarily paired — task sections 22/46/47. Returns the accepted
// groups (unordered; final ordering is applied later — see sort.go) and
// updates state in place.
func runAutoMatching(book []BookItem, external []ExternalItem, policy MatchingPolicy, state *matchState) ([]MatchGroup, []Candidate) {
	var groups []MatchGroup
	var ambiguous []Candidate

	// Index both sides ONCE, before any stage runs — task section 71:
	// "Index once... Avoid full book x external pair scans where
	// possible." Every stage below looks candidates up through these
	// indexes (by normalized reference, or by amount bucket) rather than
	// re-scanning the opposite population for every item, which is what
	// made this package O(book x external) before this fix — confirmed
	// via profiling a 10,000/10,000 benchmark, where re-normalizing
	// every external item's reference once per book item dominated
	// runtime (see benchmark_test.go's scaling sweep, which caught this).
	bookIdx := buildItemIndex(len(book), func(i int) (ref string, amount float64) {
		b := book[i]
		r, _ := normalizedReference(b.Reference, policy.ReferenceNormalization)
		return r, b.SignedAmount()
	}, policy.AmountTolerance)
	externalIdx := buildItemIndex(len(external), func(i int) (ref string, amount float64) {
		e := external[i]
		r, _ := normalizedReference(e.Reference, policy.ReferenceNormalization)
		return r, e.OrientedSignedAmount(policy.orientation())
	}, policy.AmountTolerance)

	// Stage: exact normalized reference + amount + compatible date
	// (same date, since "compatible" with no window configured means
	// exact; if DateWindowDays > 0, reference-backed matches also accept
	// that same window — reference agreement is strong evidence, so it
	// reuses the window rather than requiring a stricter same-date-only
	// rule that amount-only matching doesn't get either).
	g, a := matchStage(book, external, bookIdx, externalIdx, policy, state,
		ReasonExactReferenceAmountDate, ConfidenceExact, true, true)
	groups = append(groups, g...)
	ambiguous = append(ambiguous, a...)

	// Stage: exact amount + same date (no reference requirement).
	g, a = matchStage(book, external, bookIdx, externalIdx, policy, state,
		ReasonExactAmountDate, ConfidenceStrong, false, false)
	groups = append(groups, g...)
	ambiguous = append(ambiguous, a...)

	// Stage: exact amount + date within window.
	if policy.DateWindowDays > 0 {
		g, a = matchStage(book, external, bookIdx, externalIdx, policy, state,
			ReasonExactAmountWithinWindow, ConfidenceReview, false, true)
		groups = append(groups, g...)
		ambiguous = append(ambiguous, a...)
	}

	return groups, ambiguous
}

// itemIndex is a one-time index built over one side's items (book or
// external), keyed by normalized reference and by amount bucket — task
// section 71. It stores only indexes into the caller's own slice, never
// a copy of the items themselves.
type itemIndex struct {
	byReference map[string][]int // normalized reference -> item indexes
	byAmount    map[int64][]int  // amount-bucket key -> item indexes
	bucketWidth float64
}

// buildItemIndex indexes n items (accessed via at, which returns each
// item's normalized reference — "" if it has none — and its already-
// oriented signed amount) by reference and by amount bucket.
func buildItemIndex(n int, at func(i int) (ref string, amount float64), tolerance float64) *itemIndex {
	idx := &itemIndex{
		byReference: make(map[string][]int, n),
		byAmount:    make(map[int64][]int, n),
		bucketWidth: resolvedBucketWidth(tolerance),
	}
	for i := 0; i < n; i++ {
		ref, amount := at(i)
		if ref != "" {
			idx.byReference[ref] = append(idx.byReference[ref], i)
		}
		key := amountBucketKey(amount, idx.bucketWidth)
		idx.byAmount[key] = append(idx.byAmount[key], i)
	}
	return idx
}

// candidatesByReference returns the indexes of every item sharing ref
// exactly (already-normalized), or nil if ref is empty.
func (idx *itemIndex) candidatesByReference(ref string) []int {
	if ref == "" {
		return nil
	}
	return idx.byReference[ref]
}

// candidatesByAmount returns the indexes of every item whose amount
// bucket is within one neighboring bucket of amount — always wide
// enough to cover the full tolerance band, since bucketWidth is derived
// from that same tolerance (see resolvedBucketWidth/amountBucketKey).
// The result may contain duplicates across neighboring-bucket overlap
// and is not itself deduplicated — matchStage's own seen-ItemID map
// deduplicates at the point of use.
func (idx *itemIndex) candidatesByAmount(amount float64) []int {
	key := amountBucketKey(amount, idx.bucketWidth)
	var out []int
	for _, k := range [3]int64{key - 1, key, key + 1} {
		out = append(out, idx.byAmount[k]...)
	}
	return out
}

// amountBucketKey buckets amount into a fixed-width bucket sized to
// bucketWidth, so any two amounts within tolerance of each other land in
// the same or an immediately-adjacent bucket (see candidatesByAmount).
func amountBucketKey(amount, bucketWidth float64) int64 {
	if bucketWidth <= 0 {
		bucketWidth = 0.01
	}
	return int64(amount / bucketWidth)
}

func resolvedBucketWidth(tolerance float64) float64 {
	if tolerance > 0 {
		return tolerance
	}
	return 0.01
}

// matchStage runs one precedence stage: for every not-yet-consumed book
// item, find not-yet-consumed external items whose OrientedSignedAmount
// matches the book item's SignedAmount within tolerance, whose date
// satisfies the stage's date rule, and — if requireReference is true —
// whose normalized reference equals the book item's. If
// policy.DescriptionExactMatchEnabled is true and BOTH items carry a
// non-empty Description, normalized-description equality is also
// required as an additional narrowing filter; per task section 15,
// description is never SOLE matching evidence — it only ever narrows a
// candidate set that amount/date (and, in stage 1, reference) already
// established, and an item with no Description at all is never excluded
// on that basis alone. A book item with exactly one valid external
// candidate (and vice versa, to guarantee mutual uniqueness — see the
// note below) is matched; a book item with zero or 2+ candidates is left
// for the next stage/left unmatched.
//
// useWindow selects whether the date rule is "within policy.DateWindowDays"
// (true) or "same date" (false, i.e. window of 0 regardless of policy).
// Both directions of candidate lookup go through bookIdx/externalIdx
// (built once by runAutoMatching) rather than scanning the opposite
// population in full — task section 71.
func matchStage(
	book []BookItem, external []ExternalItem, bookIdx, externalIdx *itemIndex, policy MatchingPolicy, state *matchState,
	reason MatchReason, confidence MatchConfidence, requireReference, useWindow bool,
) ([]MatchGroup, []Candidate) {
	window := 0
	if useWindow {
		window = policy.DateWindowDays
	}

	// Deterministic iteration: book items are walked in caller-supplied
	// (input) order — the slice itself is the deterministic order, so no
	// separate sort is needed here.
	var groups []MatchGroup
	var ambiguous []Candidate
	for _, b := range book {
		if !state.bookAvailable(b.ItemID) {
			continue
		}
		bookRef, haveBookRef := normalizedReference(b.Reference, policy.ReferenceNormalization)
		if requireReference && !haveBookRef {
			continue
		}
		bookDate, haveBookDate := parseDate(b.Date)
		bookAmount := b.SignedAmount()

		var externalCandidateIdxs []int
		if requireReference {
			externalCandidateIdxs = externalIdx.candidatesByReference(bookRef)
		} else {
			externalCandidateIdxs = externalIdx.candidatesByAmount(bookAmount)
		}

		var candidates []ExternalItem
		seenExternal := make(map[string]bool, len(externalCandidateIdxs))
		for _, ci := range externalCandidateIdxs {
			e := external[ci]
			if seenExternal[e.ItemID] {
				continue // neighboring-bucket overlap can repeat an index
			}
			seenExternal[e.ItemID] = true
			if !state.externalAvailable(e.ItemID) {
				continue
			}
			if requireReference {
				extRef, haveExtRef := normalizedReference(e.Reference, policy.ReferenceNormalization)
				if !haveExtRef || extRef != bookRef {
					continue
				}
			}
			if b.Currency != "" && e.Currency != "" && b.Currency != e.Currency {
				continue
			}
			if !amountsMatch(bookAmount, e.OrientedSignedAmount(policy.orientation()), policy.AmountTolerance, policy.RelativeTolerance) {
				continue
			}
			extDate, haveExtDate := parseDate(e.Date)
			if !haveBookDate || !haveExtDate {
				continue
			}
			days := dayDiff(bookDate, extDate)
			if days > window {
				continue
			}
			if !descriptionsCompatible(b.Description, e.Description, policy) {
				continue
			}
			candidates = append(candidates, e)
		}

		if len(candidates) > 1 {
			// 2+ equally valid candidates under this stage's own rule —
			// report every pairing as an ambiguous Candidate rather than
			// picking one (task sections 22/46/47).
			for _, e := range candidates {
				ambiguous = append(ambiguous, ambiguousCandidate(b, e, reason, policy, "AMBIGUOUS_MULTIPLE_CANDIDATES"))
			}
			continue
		}
		if len(candidates) == 0 {
			continue // unmatched at this stage; may still match at a later stage
		}
		e := candidates[0]
		eAmount := e.OrientedSignedAmount(policy.orientation())

		// Mutual-uniqueness check: this book item must also be the
		// unique candidate for e under the same rule, or this pairing is
		// ambiguous from e's perspective (e.g. two book items both
		// amount-match one external item; picking either arbitrarily
		// would violate the ambiguity invariant). Looked up through
		// bookIdx the same way — never a full scan of book.
		var bookCandidateIdxs []int
		if requireReference {
			extRef, _ := normalizedReference(e.Reference, policy.ReferenceNormalization)
			bookCandidateIdxs = bookIdx.candidatesByReference(extRef)
		} else {
			bookCandidateIdxs = bookIdx.candidatesByAmount(eAmount)
		}
		eDate, _ := parseDate(e.Date)

		var reverseMatches []BookItem
		seenBook := make(map[string]bool, len(bookCandidateIdxs))
		for _, bi := range bookCandidateIdxs {
			b2 := book[bi]
			if seenBook[b2.ItemID] {
				continue
			}
			seenBook[b2.ItemID] = true
			if b2.ItemID != b.ItemID && !state.bookAvailable(b2.ItemID) {
				continue
			}
			if requireReference {
				ref2, have2 := normalizedReference(b2.Reference, policy.ReferenceNormalization)
				extRef, haveExtRef := normalizedReference(e.Reference, policy.ReferenceNormalization)
				if !have2 || !haveExtRef || ref2 != extRef {
					continue
				}
			}
			if b2.Currency != "" && e.Currency != "" && b2.Currency != e.Currency {
				continue
			}
			if !amountsMatch(b2.SignedAmount(), eAmount, policy.AmountTolerance, policy.RelativeTolerance) {
				continue
			}
			d2, ok2 := parseDate(b2.Date)
			if !ok2 {
				continue
			}
			if dayDiff(d2, eDate) > window {
				continue
			}
			if !descriptionsCompatible(b2.Description, e.Description, policy) {
				continue
			}
			reverseMatches = append(reverseMatches, b2)
		}
		if len(reverseMatches) > 1 {
			// Ambiguous from e's own perspective: e itself has more than
			// one equally valid book-side candidate, even though b's own
			// forward search happened to see only e. Record every such
			// pairing as ambiguous rather than silently dropping it — this
			// is the exact scenario task section 22/46/47 calls out (two
			// book items both amount-matching one external item).
			for _, b2 := range reverseMatches {
				ambiguous = append(ambiguous, ambiguousCandidate(b2, e, reason, policy, "AMBIGUOUS_MULTIPLE_CANDIDATES"))
			}
			continue
		}
		if len(reverseMatches) == 0 {
			continue // defensive; e always reverse-matches at least b itself
		}

		state.consumeBook(b.ItemID)
		state.consumeExternal(e.ItemID)

		extDate, _ := parseDate(e.Date)
		days := dayDiff(bookDate, extDate)

		refMatched := requireReference
		normRef := ""
		if refMatched {
			normRef = bookRef
		}

		groups = append(groups, MatchGroup{
			BookItemIDs:         []string{b.ItemID},
			ExternalItemIDs:     []string{e.ItemID},
			BookAmount:          bookAmount,
			ExternalAmount:      eAmount,
			Difference:          bookAmount - eAmount,
			MatchType:           MatchOneToOne,
			MatchReason:         reason,
			Confidence:          confidence,
			DateDifferenceDays:  days,
			ReferenceMatched:    refMatched,
			NormalizedReference: normRef,
		})
	}
	return groups, ambiguous
}

func dayDiff(a, b time.Time) int {
	d := int(b.Sub(a).Hours() / 24)
	if d < 0 {
		d = -d
	}
	return d
}

// normalizedDescription resolves a description under a fixed trim +
// case-fold + collapse-internal-whitespace rule — used only when
// MatchingPolicy.DescriptionExactMatchEnabled opts in to it.
func normalizedDescription(s string) (string, bool) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", false
	}
	return strings.ToLower(strings.Join(fields, " ")), true
}

// descriptionsCompatible reports whether a and b's descriptions are
// compatible under policy — always true when
// DescriptionExactMatchEnabled is off, or when either side has no
// (normalizable) Description at all, since this package never treats a
// missing description as a mismatch (task section 15: description
// narrows, it never independently excludes). When enabled and both sides
// have one, exact normalized equality is required.
func descriptionsCompatible(a, b string, policy MatchingPolicy) bool {
	if !policy.DescriptionExactMatchEnabled {
		return true
	}
	na, haveA := normalizedDescription(a)
	nb, haveB := normalizedDescription(b)
	if !haveA || !haveB {
		return true
	}
	return na == nb
}

// ambiguousCandidate builds one factual Candidate record for an
// unresolved (book, external) pairing — evidence only, no ranking (task
// section 23).
func ambiguousCandidate(b BookItem, e ExternalItem, reason MatchReason, policy MatchingPolicy, why string) Candidate {
	amountDiff := b.SignedAmount() - e.OrientedSignedAmount(policy.orientation())
	days := 0
	if bd, ok1 := parseDate(b.Date); ok1 {
		if ed, ok2 := parseDate(e.Date); ok2 {
			days = dayDiff(bd, ed)
		}
	}
	var refEvidence string
	if bookRef, ok := normalizedReference(b.Reference, policy.ReferenceNormalization); ok {
		refEvidence = bookRef
	}
	return Candidate{
		BookItemID:         b.ItemID,
		ExternalItemID:     e.ItemID,
		Rule:               reason,
		AmountDifference:   amountDiff,
		DateDifferenceDays: days,
		ReferenceEvidence:  refEvidence,
		Reason:             why,
	}
}
