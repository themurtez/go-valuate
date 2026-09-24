package reconciliation

import (
	"strconv"
	"testing"
)

func TestMatcher_ExactReferenceAmountDate(t *testing.T) {
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow, Reference: "CHK-100"}}
	external := []ExternalItem{{ItemID: "E1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow, Reference: "chk100"}}
	policy := MatchingPolicy{ReferenceNormalization: ReferenceNormalization{CaseFold: true, RemovePunctuation: true}}
	state := newMatchState(1, 1)
	groups, ambiguous := runAutoMatching(book, external, policy, state)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d (ambiguous=%d)", len(groups), len(ambiguous))
	}
	if groups[0].MatchReason != ReasonExactReferenceAmountDate {
		t.Fatalf("expected reference+amount+date reason, got %s", groups[0].MatchReason)
	}
	if !groups[0].ReferenceMatched {
		t.Fatalf("expected ReferenceMatched true")
	}
}

func TestMatcher_ExactAmountSameDateFallback(t *testing.T) {
	// No reference at all - falls to stage 2.
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 250, Direction: DirectionInflow}}
	external := []ExternalItem{{ItemID: "E1", Date: "2025-01-10", Amount: 250, Direction: DirectionInflow}}
	state := newMatchState(1, 1)
	groups, _ := runAutoMatching(book, external, MatchingPolicy{}, state)
	if len(groups) != 1 || groups[0].MatchReason != ReasonExactAmountDate {
		t.Fatalf("expected 1 EXACT_AMOUNT_DATE group, got %+v", groups)
	}
}

func TestMatcher_DateWindowRequiresPolicyOptIn(t *testing.T) {
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 250, Direction: DirectionInflow}}
	external := []ExternalItem{{ItemID: "E1", Date: "2025-01-13", Amount: 250, Direction: DirectionInflow}}
	state := newMatchState(1, 1)
	groups, _ := runAutoMatching(book, external, MatchingPolicy{DateWindowDays: 0}, state)
	if len(groups) != 0 {
		t.Fatalf("expected no match without date_window_days configured, got %+v", groups)
	}

	state2 := newMatchState(1, 1)
	groups2, _ := runAutoMatching(book, external, MatchingPolicy{DateWindowDays: 5}, state2)
	if len(groups2) != 1 || groups2[0].MatchReason != ReasonExactAmountWithinWindow {
		t.Fatalf("expected 1 EXACT_AMOUNT_WITHIN_WINDOW group with window enabled, got %+v", groups2)
	}
	if groups2[0].DateDifferenceDays != 3 {
		t.Fatalf("expected date_difference_days=3, got %d", groups2[0].DateDifferenceDays)
	}
}

func TestMatcher_AmbiguousMultipleCandidatesLeftUnresolved(t *testing.T) {
	// Two book items, two external items, all same amount/date - no
	// reference to disambiguate. Task sections 22/46/47: never guess.
	book := []BookItem{
		{ItemID: "B1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
		{ItemID: "B2", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
	}
	external := []ExternalItem{
		{ItemID: "E1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
		{ItemID: "E2", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
	}
	state := newMatchState(2, 2)
	groups, ambiguous := runAutoMatching(book, external, MatchingPolicy{}, state)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups (fully ambiguous), got %+v", groups)
	}
	if len(ambiguous) == 0 {
		t.Fatalf("expected ambiguous candidates to be reported")
	}
	// Every item must remain available (never consumed) since nothing
	// was actually resolved.
	for _, id := range []string{"B1", "B2"} {
		if !state.bookAvailable(id) {
			t.Fatalf("expected %s to remain available (unmatched), it was consumed", id)
		}
	}
	for _, id := range []string{"E1", "E2"} {
		if !state.externalAvailable(id) {
			t.Fatalf("expected %s to remain available (unmatched), it was consumed", id)
		}
	}
}

func TestMatcher_RepeatedAmountsNoArbitraryPairing(t *testing.T) {
	// Task section 79: 100+ identical amounts with no distinguishing
	// evidence must not produce arbitrary pairings.
	const n = 120
	var book []BookItem
	var external []ExternalItem
	for i := 0; i < n; i++ {
		book = append(book, BookItem{ItemID: "B" + strconv.Itoa(i), Date: "2025-01-10", Amount: 50, Direction: DirectionOutflow})
		external = append(external, ExternalItem{ItemID: "E" + strconv.Itoa(i), Date: "2025-01-10", Amount: 50, Direction: DirectionOutflow})
	}
	state := newMatchState(n, n)
	groups, ambiguous := runAutoMatching(book, external, MatchingPolicy{}, state)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups for indistinguishable repeated amounts, got %d", len(groups))
	}
	if len(ambiguous) == 0 {
		t.Fatalf("expected ambiguous candidates for repeated amounts")
	}
}

func TestMatcher_MutualUniquenessFromExternalSide(t *testing.T) {
	// Two book items both amount-match one external item (only one
	// external candidate exists, but it is ambiguous FROM the external
	// item's perspective) - must not arbitrarily pick either.
	book := []BookItem{
		{ItemID: "B1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
		{ItemID: "B2", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
	}
	external := []ExternalItem{
		{ItemID: "E1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
	}
	state := newMatchState(2, 1)
	groups, ambiguous := runAutoMatching(book, external, MatchingPolicy{}, state)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups (ambiguous from external side), got %+v", groups)
	}
	if len(ambiguous) == 0 {
		t.Fatalf("expected ambiguous candidates")
	}
}

func TestMatcher_CurrencyMismatchExcludesPair(t *testing.T) {
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow, Currency: "USD"}}
	external := []ExternalItem{{ItemID: "E1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow, Currency: "EUR"}}
	state := newMatchState(1, 1)
	groups, ambiguous := runAutoMatching(book, external, MatchingPolicy{}, state)
	if len(groups) != 0 || len(ambiguous) != 0 {
		t.Fatalf("expected no match and no ambiguous candidate for mismatched currency, got groups=%+v ambiguous=%+v", groups, ambiguous)
	}
}

func TestMatcher_ToleranceAppliesToAmountComparison(t *testing.T) {
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 100.00, Direction: DirectionOutflow}}
	external := []ExternalItem{{ItemID: "E1", Date: "2025-01-10", Amount: 100.02, Direction: DirectionOutflow}}
	state := newMatchState(1, 1)
	groups, _ := runAutoMatching(book, external, MatchingPolicy{AmountTolerance: 0.05}, state)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group within 0.05 tolerance, got %+v", groups)
	}
}

func TestMatcher_DescriptionExactMatchNarrowsNotSole(t *testing.T) {
	// Two same-amount/date external items; only one shares an exact
	// normalized description with the book item.
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow, Description: "Office Supplies"}}
	external := []ExternalItem{
		{ItemID: "E1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow, Description: "office   supplies"},
		{ItemID: "E2", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow, Description: "Travel"},
	}
	state := newMatchState(1, 2)
	groups, ambiguous := runAutoMatching(book, external, MatchingPolicy{DescriptionExactMatchEnabled: true}, state)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group narrowed by description, got %+v (ambiguous=%+v)", groups, ambiguous)
	}
	if groups[0].ExternalItemIDs[0] != "E1" {
		t.Fatalf("expected match to E1, got %v", groups[0].ExternalItemIDs)
	}
}

func TestMatcher_MissingDescriptionNeverExcludesOnItsOwn(t *testing.T) {
	// One side has no description at all - descriptionsCompatible must
	// still allow the match when DescriptionExactMatchEnabled is true.
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow}}
	external := []ExternalItem{{ItemID: "E1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow, Description: "Wire transfer"}}
	state := newMatchState(1, 1)
	groups, _ := runAutoMatching(book, external, MatchingPolicy{DescriptionExactMatchEnabled: true}, state)
	if len(groups) != 1 {
		t.Fatalf("expected match despite missing description on one side, got %+v", groups)
	}
}
