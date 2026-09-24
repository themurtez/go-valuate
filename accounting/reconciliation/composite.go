package reconciliation

import "time"

// runCompositeMatching attempts bounded one-book-to-many-external and
// many-book-to-one-external matches for items not consumed by
// runAutoMatching, when policy.EnableCompositeMatching is true (task
// sections 20/21/80). For each remaining anchor item (the "one" side), a
// candidate pool is built from remaining opposite-side items within
// policy.DateWindowDays of the anchor, capped at policy.MaxCandidatesPerItem
// (task section 21's "max candidate count"); if the pool already exceeds
// the cap, search for that anchor is skipped and
// IssueCompositeSearchLimitReached is reported — the pool is never
// silently truncated to an arbitrary subset, since doing so could hide a
// real combination. Within the (possibly capped) pool, every subset of
// size 2..MaxCompositeGroupSize is examined up to
// MaxCompositeSearchCombinations total combinations per anchor; if that
// budget would be exceeded before the search completes,
// IssueCompositeSearchLimitReached is reported and only whatever was
// found before the budget ran out is kept — never an unbounded/unconstrained
// subset-sum search (task section 20).
//
// If exactly one subset sums to the anchor's amount within tolerance,
// it is matched. If two or more equally-valid subsets exist, the anchor
// is left unresolved (ambiguous) — task section 48: "no stronger
// evidence distinguishes them, do not auto-match." Runs book-anchored
// search first, then external-anchored search over whatever remains.
func runCompositeMatching(book []BookItem, external []ExternalItem, policy MatchingPolicy, state *matchState) ([]MatchGroup, []Candidate, []Issue) {
	if !policy.EnableCompositeMatching {
		return nil, nil, nil
	}

	var groups []MatchGroup
	var ambiguous []Candidate
	var issues []Issue

	bookAnchored, ambB, issuesB := compositeSearch(book, external, policy, state, true)
	groups = append(groups, bookAnchored...)
	ambiguous = append(ambiguous, ambB...)
	issues = append(issues, issuesB...)

	externalAnchored, ambE, issuesE := compositeSearch(external, book, policy, state, false)
	groups = append(groups, externalAnchored...)
	ambiguous = append(ambiguous, ambE...)
	issues = append(issues, issuesE...)

	return groups, ambiguous, issues
}

// anchorItem is the minimal shape compositeSearch needs from either
// BookItem or ExternalItem, so the same search logic runs in both
// directions without duplicating it.
type anchorItem struct {
	id           string
	signedAmount float64
	date         time.Time
	haveDate     bool
	currency     string
}

func bookAnchor(b BookItem) anchorItem {
	d, ok := parseDate(b.Date)
	return anchorItem{id: b.ItemID, signedAmount: b.SignedAmount(), date: d, haveDate: ok, currency: b.Currency}
}

func externalAnchor(e ExternalItem, o Orientation) anchorItem {
	d, ok := parseDate(e.Date)
	return anchorItem{id: e.ItemID, signedAmount: e.OrientedSignedAmount(o), date: d, haveDate: ok, currency: e.Currency}
}

// compositeSearch runs the anchored composite search in one direction.
// bookIsAnchorSide selects which side is the single "1" and which is the
// "many" pool searched for a summing subset: true means each BookItem is
// tried as the "1" anchor against a pool of ExternalItems (producing
// MatchOneBookToManyExternal groups); false means each ExternalItem is
// the anchor against a pool of BookItems (producing
// MatchManyBookToOneExternal groups). anchorSide/poolSide are passed as
// interface{} purely so this one function body serves both directions
// without duplicating the search/budget logic — each branch immediately
// type-asserts back to the concrete slice type it expects.
func compositeSearch(anchorSide, poolSide interface{}, policy MatchingPolicy, state *matchState, bookIsAnchorSide bool) ([]MatchGroup, []Candidate, []Issue) {
	var groups []MatchGroup
	var ambiguous []Candidate
	var issues []Issue

	if bookIsAnchorSide {
		anchors := anchorSide.([]BookItem)
		pool := poolSide.([]ExternalItem)
		for _, b := range anchors {
			if !state.bookAvailable(b.ItemID) {
				continue
			}
			a := bookAnchor(b)
			var candidates []anchorItem
			var candidateExternal []ExternalItem
			for _, e := range pool {
				if !state.externalAvailable(e.ItemID) {
					continue
				}
				if a.currency != "" && e.Currency != "" && a.currency != e.Currency {
					continue
				}
				ea := externalAnchor(e, policy.orientation())
				if !withinDateWindow(a, ea, policy.DateWindowDays) {
					continue
				}
				candidates = append(candidates, ea)
				candidateExternal = append(candidateExternal, e)
			}
			if len(candidates) > policy.MaxCandidatesPerItem {
				issues = append(issues, Issue{
					Code: IssueCompositeSearchLimitReached, Severity: IssueSeverityWarning,
					Message: "composite candidate pool exceeded max_candidates_per_item; search skipped for this item", ItemID: b.ItemID,
				})
				continue
			}
			subsets, limitHit := findSubsetsSummingTo(candidates, a.signedAmount, policy)
			if limitHit {
				issues = append(issues, Issue{
					Code: IssueCompositeSearchLimitReached, Severity: IssueSeverityWarning,
					Message: "composite search combination budget reached for this item", ItemID: b.ItemID,
				})
			}
			if len(subsets) > 1 {
				for _, idxs := range subsets {
					var extIDs []string
					for _, idx := range idxs {
						extIDs = append(extIDs, candidateExternal[idx].ItemID)
					}
					ambiguous = append(ambiguous, Candidate{
						BookItemID:      b.ItemID,
						ExternalItemIDs: extIDs,
						Rule:            ReasonCompositeSumMatch,
						Reason:          "AMBIGUOUS_MULTIPLE_COMPOSITE_SUMS",
					})
				}
				continue
			}
			if len(subsets) == 0 {
				continue
			}
			idxs := subsets[0]
			var extIDs []string
			var extAmount float64
			maxDays := 0
			for _, idx := range idxs {
				extIDs = append(extIDs, candidateExternal[idx].ItemID)
				extAmount += candidates[idx].signedAmount
				if a.haveDate && candidates[idx].haveDate {
					d := dayDiff(a.date, candidates[idx].date)
					if d > maxDays {
						maxDays = d
					}
				}
			}
			state.consumeBook(b.ItemID)
			for _, id := range extIDs {
				state.consumeExternal(id)
			}
			groups = append(groups, MatchGroup{
				BookItemIDs:        []string{b.ItemID},
				ExternalItemIDs:    extIDs,
				BookAmount:         a.signedAmount,
				ExternalAmount:     extAmount,
				Difference:         a.signedAmount - extAmount,
				MatchType:          MatchOneBookToManyExternal,
				MatchReason:        ReasonCompositeSumMatch,
				Confidence:         ConfidenceReview,
				DateDifferenceDays: maxDays,
			})
		}
		return groups, ambiguous, issues
	}

	// External-anchored: one external item matched by many book items.
	anchors := anchorSide.([]ExternalItem)
	pool := poolSide.([]BookItem)
	for _, e := range anchors {
		if !state.externalAvailable(e.ItemID) {
			continue
		}
		a := externalAnchor(e, policy.orientation())
		var candidates []anchorItem
		var candidateBook []BookItem
		for _, b := range pool {
			if !state.bookAvailable(b.ItemID) {
				continue
			}
			if a.currency != "" && b.Currency != "" && a.currency != b.Currency {
				continue
			}
			ba := bookAnchor(b)
			if !withinDateWindow(a, ba, policy.DateWindowDays) {
				continue
			}
			candidates = append(candidates, ba)
			candidateBook = append(candidateBook, b)
		}
		if len(candidates) > policy.MaxCandidatesPerItem {
			issues = append(issues, Issue{
				Code: IssueCompositeSearchLimitReached, Severity: IssueSeverityWarning,
				Message: "composite candidate pool exceeded max_candidates_per_item; search skipped for this item", ItemID: e.ItemID,
			})
			continue
		}
		subsets, limitHit := findSubsetsSummingTo(candidates, a.signedAmount, policy)
		if limitHit {
			issues = append(issues, Issue{
				Code: IssueCompositeSearchLimitReached, Severity: IssueSeverityWarning,
				Message: "composite search combination budget reached for this item", ItemID: e.ItemID,
			})
		}
		if len(subsets) > 1 {
			for _, idxs := range subsets {
				var bookIDs []string
				for _, idx := range idxs {
					bookIDs = append(bookIDs, candidateBook[idx].ItemID)
				}
				ambiguous = append(ambiguous, Candidate{
					BookItemIDs:    bookIDs,
					ExternalItemID: e.ItemID,
					Rule:           ReasonCompositeSumMatch,
					Reason:         "AMBIGUOUS_MULTIPLE_COMPOSITE_SUMS",
				})
			}
			continue
		}
		if len(subsets) == 0 {
			continue
		}
		idxs := subsets[0]
		var bookIDs []string
		var bookAmount float64
		maxDays := 0
		for _, idx := range idxs {
			bookIDs = append(bookIDs, candidateBook[idx].ItemID)
			bookAmount += candidates[idx].signedAmount
			if a.haveDate && candidates[idx].haveDate {
				d := dayDiff(a.date, candidates[idx].date)
				if d > maxDays {
					maxDays = d
				}
			}
		}
		state.consumeExternal(e.ItemID)
		for _, id := range bookIDs {
			state.consumeBook(id)
		}
		groups = append(groups, MatchGroup{
			BookItemIDs:        bookIDs,
			ExternalItemIDs:    []string{e.ItemID},
			BookAmount:         bookAmount,
			ExternalAmount:     a.signedAmount,
			Difference:         bookAmount - a.signedAmount,
			MatchType:          MatchManyBookToOneExternal,
			MatchReason:        ReasonCompositeSumMatch,
			Confidence:         ConfidenceReview,
			DateDifferenceDays: maxDays,
		})
	}
	return groups, ambiguous, issues
}

func withinDateWindow(a, b anchorItem, windowDays int) bool {
	if !a.haveDate || !b.haveDate {
		return false
	}
	return dayDiff(a.date, b.date) <= windowDays
}

// findSubsetsSummingTo enumerates subsets of candidates (size 2..
// policy.MaxCompositeGroupSize) whose summed signedAmount matches target
// within tolerance, stopping and reporting limitHit=true if more than
// policy.MaxCompositeSearchCombinations subsets would need to be
// examined. Returns every matching subset found (as index slices into
// candidates) up to the point the budget was reached — task section 21:
// "if cap is hit, return a structured issue, not runaway compute." A
// single matching subset of size 1 is intentionally never produced here
// (size-1 "composite" matches are just ordinary one-to-one matches,
// already handled by matchStage) — the loop starts subset size at 2.
func findSubsetsSummingTo(candidates []anchorItem, target float64, policy MatchingPolicy) (subsets [][]int, limitHit bool) {
	n := len(candidates)
	maxSize := policy.MaxCompositeGroupSize
	if maxSize > n {
		maxSize = n
	}
	budget := policy.MaxCompositeSearchCombinations
	combos := 0

	// Recursive combination enumeration, bounded overall by
	// MaxCandidatesPerItem on the caller side (n is never large) and by
	// budget here. chosen is deliberately copied (via append to a fresh
	// slice, never appended to in place) before each recursive descent —
	// reusing one growable slice across sibling loop iterations would let
	// a deeper call's append write into a still-referenced earlier
	// slice's backing array once capacity allowed in-place growth, which
	// would corrupt an already-recorded subset. This copy is the fix.
	var combo func(start int, chosen []int, sum float64, size int) bool // returns false if budget exhausted
	combo = func(start int, chosen []int, sum float64, size int) bool {
		if len(chosen) == size {
			combos++
			if combos > budget {
				return false
			}
			if amountsMatch(sum, target, policy.AmountTolerance, policy.RelativeTolerance) {
				subsets = append(subsets, append([]int(nil), chosen...))
			}
			return true
		}
		for i := start; i < n; i++ {
			next := make([]int, len(chosen)+1)
			copy(next, chosen)
			next[len(chosen)] = i
			if !combo(i+1, next, sum+candidates[i].signedAmount, size) {
				return false
			}
		}
		return true
	}

	for size := 2; size <= maxSize; size++ {
		if !combo(0, nil, 0, size) {
			return subsets, true
		}
	}
	return subsets, false
}
