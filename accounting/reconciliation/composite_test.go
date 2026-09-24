package reconciliation

import (
	"strconv"
	"testing"
)

func defaultCompositePolicy() MatchingPolicy {
	return MatchingPolicy{
		EnableCompositeMatching:        true,
		MaxCompositeGroupSize:          5,
		MaxCandidatesPerItem:           50,
		MaxCompositeSearchCombinations: 10000,
		DateWindowDays:                 5,
	}
}

func TestComposite_UniqueSumMatches(t *testing.T) {
	// Task section 80's "unique case": Book 300, External 100+200.
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 300, Direction: DirectionOutflow}}
	external := []ExternalItem{
		{ItemID: "E1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
		{ItemID: "E2", Date: "2025-01-10", Amount: 200, Direction: DirectionOutflow},
	}
	state := newMatchState(1, 2)
	groups, ambiguous, issues := runCompositeMatching(book, external, defaultCompositePolicy(), state)
	if len(groups) != 1 {
		t.Fatalf("expected 1 composite group, got %d (ambiguous=%d issues=%+v)", len(groups), len(ambiguous), issues)
	}
	g := groups[0]
	if g.MatchType != MatchOneBookToManyExternal {
		t.Fatalf("expected ONE_BOOK_TO_MANY_EXTERNAL, got %s", g.MatchType)
	}
	if len(g.ExternalItemIDs) != 2 {
		t.Fatalf("expected 2 external items in the group, got %v", g.ExternalItemIDs)
	}
}

func TestComposite_AmbiguousSumsNeverAutoMatch(t *testing.T) {
	// Task section 80's ambiguous case: 50+250 vs 100+200, both sum to
	// book's 300, no stronger evidence distinguishes them.
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 300, Direction: DirectionOutflow}}
	external := []ExternalItem{
		{ItemID: "E1", Date: "2025-01-10", Amount: 50, Direction: DirectionOutflow},
		{ItemID: "E2", Date: "2025-01-10", Amount: 250, Direction: DirectionOutflow},
		{ItemID: "E3", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
		{ItemID: "E4", Date: "2025-01-10", Amount: 200, Direction: DirectionOutflow},
	}
	state := newMatchState(1, 4)
	groups, ambiguous, _ := runCompositeMatching(book, external, defaultCompositePolicy(), state)
	if len(groups) != 0 {
		t.Fatalf("expected 0 composite matches for ambiguous sums, got %+v", groups)
	}
	if len(ambiguous) < 2 {
		t.Fatalf("expected 2+ ambiguous composite candidates, got %d", len(ambiguous))
	}
	// Neither B1 nor any external item should be consumed.
	if !state.bookAvailable("B1") {
		t.Fatalf("B1 should remain available (ambiguous, not matched)")
	}
	for _, id := range []string{"E1", "E2", "E3", "E4"} {
		if !state.externalAvailable(id) {
			t.Fatalf("%s should remain available (ambiguous, not matched)", id)
		}
	}
}

func TestComposite_OffByDefault(t *testing.T) {
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 300, Direction: DirectionOutflow}}
	external := []ExternalItem{
		{ItemID: "E1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
		{ItemID: "E2", Date: "2025-01-10", Amount: 200, Direction: DirectionOutflow},
	}
	state := newMatchState(1, 2)
	groups, ambiguous, issues := runCompositeMatching(book, external, MatchingPolicy{}, state)
	if groups != nil || ambiguous != nil || issues != nil {
		t.Fatalf("expected no composite activity when disabled, got groups=%v ambiguous=%v issues=%v", groups, ambiguous, issues)
	}
}

func TestComposite_CandidatePoolCapReportsIssue(t *testing.T) {
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 1000, Direction: DirectionOutflow}}
	var external []ExternalItem
	for i := 0; i < 10; i++ {
		external = append(external, ExternalItem{ItemID: "E" + strconv.Itoa(i), Date: "2025-01-10", Amount: 10, Direction: DirectionOutflow})
	}
	policy := defaultCompositePolicy()
	policy.MaxCandidatesPerItem = 5 // fewer than the 10 candidates available
	state := newMatchState(1, 10)
	groups, _, issues := runCompositeMatching(book, external, policy, state)
	if len(groups) != 0 {
		t.Fatalf("expected no match when candidate pool exceeds cap, got %+v", groups)
	}
	found := false
	for _, iss := range issues {
		if iss.Code == IssueCompositeSearchLimitReached {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueCompositeSearchLimitReached, got %+v", issues)
	}
}

func TestComposite_SearchStaysBoundedUnderCombinationExplosion(t *testing.T) {
	// 30 same-window candidates with MaxCompositeGroupSize=5 would be
	// C(30,2)+C(30,3)+C(30,4)+C(30,5) ~ 174k combinations - budget this
	// tightly and confirm the search stops rather than running away.
	book := []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 999999, Direction: DirectionOutflow}} // unreachable sum
	var external []ExternalItem
	for i := 0; i < 30; i++ {
		external = append(external, ExternalItem{ItemID: "E" + strconv.Itoa(i), Date: "2025-01-10", Amount: float64(i + 1), Direction: DirectionOutflow})
	}
	policy := MatchingPolicy{
		EnableCompositeMatching:        true,
		MaxCompositeGroupSize:          5,
		MaxCandidatesPerItem:           30,
		MaxCompositeSearchCombinations: 500, // deliberately small budget
		DateWindowDays:                 5,
	}
	state := newMatchState(1, 30)
	groups, _, issues := runCompositeMatching(book, external, policy, state)
	if len(groups) != 0 {
		t.Fatalf("expected no match (sum unreachable), got %+v", groups)
	}
	found := false
	for _, iss := range issues {
		if iss.Code == IssueCompositeSearchLimitReached {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueCompositeSearchLimitReached when budget of 500 is exceeded by a 30-candidate/size-5 search, got %+v", issues)
	}
}

func TestComposite_ManyToOneDirectionSymmetric(t *testing.T) {
	// External 300 <- Book 100+200 (external-anchored direction).
	book := []BookItem{
		{ItemID: "B1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
		{ItemID: "B2", Date: "2025-01-10", Amount: 200, Direction: DirectionOutflow},
	}
	external := []ExternalItem{{ItemID: "E1", Date: "2025-01-10", Amount: 300, Direction: DirectionOutflow}}
	state := newMatchState(2, 1)
	groups, ambiguous, _ := runCompositeMatching(book, external, defaultCompositePolicy(), state)
	if len(groups) != 1 {
		t.Fatalf("expected 1 many-book-to-one-external group, got %d (ambiguous=%d)", len(groups), len(ambiguous))
	}
	if groups[0].MatchType != MatchManyBookToOneExternal {
		t.Fatalf("expected MANY_BOOK_TO_ONE_EXTERNAL, got %s", groups[0].MatchType)
	}
}
