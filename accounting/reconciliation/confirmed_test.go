package reconciliation

import "testing"

func testBookByID(items ...BookItem) map[string]BookItem {
	m := make(map[string]BookItem, len(items))
	for _, it := range items {
		m[it.ItemID] = it
	}
	return m
}

func testExternalByID(items ...ExternalItem) map[string]ExternalItem {
	m := make(map[string]ExternalItem, len(items))
	for _, it := range items {
		m[it.ItemID] = it
	}
	return m
}

func TestConfirmed_ValidMatchAccepted(t *testing.T) {
	book := testBookByID(BookItem{ItemID: "B1", Amount: 100, Direction: DirectionOutflow})
	external := testExternalByID(ExternalItem{ItemID: "E1", Amount: 100, Direction: DirectionOutflow})
	m := ConfirmedMatch{MatchID: "M1", BookItemIDs: []string{"B1"}, ExternalItemIDs: []string{"E1"}}
	accepted, issues := validateConfirmedMatches([]ConfirmedMatch{m}, book, external, MatchingPolicy{})
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %+v", issues)
	}
	if len(accepted) != 1 {
		t.Fatalf("expected 1 accepted match, got %d", len(accepted))
	}
}

func TestConfirmed_UnknownItemRejected(t *testing.T) {
	book := testBookByID(BookItem{ItemID: "B1", Amount: 100, Direction: DirectionOutflow})
	external := testExternalByID(ExternalItem{ItemID: "E1", Amount: 100, Direction: DirectionOutflow})
	m := ConfirmedMatch{BookItemIDs: []string{"B1"}, ExternalItemIDs: []string{"E999"}}
	accepted, issues := validateConfirmedMatches([]ConfirmedMatch{m}, book, external, MatchingPolicy{})
	if len(accepted) != 0 {
		t.Fatalf("expected match rejected, got %+v", accepted)
	}
	found := false
	for _, i := range issues {
		if i.Code == IssueUnknownMatchItem {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueUnknownMatchItem, got %+v", issues)
	}
}

func TestConfirmed_ItemInMultipleMatchesRejectsBoth(t *testing.T) {
	book := testBookByID(
		BookItem{ItemID: "B1", Amount: 100, Direction: DirectionOutflow},
		BookItem{ItemID: "B2", Amount: 200, Direction: DirectionOutflow},
	)
	external := testExternalByID(ExternalItem{ItemID: "E1", Amount: 100, Direction: DirectionOutflow})
	m1 := ConfirmedMatch{MatchID: "M1", BookItemIDs: []string{"B1"}, ExternalItemIDs: []string{"E1"}}
	m2 := ConfirmedMatch{MatchID: "M2", BookItemIDs: []string{"B2"}, ExternalItemIDs: []string{"E1"}}
	accepted, issues := validateConfirmedMatches([]ConfirmedMatch{m1, m2}, book, external, MatchingPolicy{})
	if len(accepted) != 0 {
		t.Fatalf("expected both matches rejected since E1 is reused, got %+v", accepted)
	}
	found := false
	for _, i := range issues {
		if i.Code == IssueItemUsedInMultipleMatches {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueItemUsedInMultipleMatches, got %+v", issues)
	}
}

func TestConfirmed_AmountMismatchRejectedWithoutExplanation(t *testing.T) {
	book := testBookByID(BookItem{ItemID: "B1", Amount: 100, Direction: DirectionOutflow})
	external := testExternalByID(ExternalItem{ItemID: "E1", Amount: 150, Direction: DirectionOutflow})
	m := ConfirmedMatch{BookItemIDs: []string{"B1"}, ExternalItemIDs: []string{"E1"}}
	accepted, issues := validateConfirmedMatches([]ConfirmedMatch{m}, book, external, MatchingPolicy{})
	if len(accepted) != 0 {
		t.Fatalf("expected mismatch rejected, got %+v", accepted)
	}
	if len(issues) == 0 || issues[0].Code != IssueInvalidConfirmedMatch {
		t.Fatalf("expected IssueInvalidConfirmedMatch, got %+v", issues)
	}
}

func TestConfirmed_ExplainedDifferenceAllowsMismatch(t *testing.T) {
	book := testBookByID(BookItem{ItemID: "B1", Amount: 100, Direction: DirectionOutflow})
	external := testExternalByID(ExternalItem{ItemID: "E1", Amount: 150, Direction: DirectionOutflow})
	explained := 50.0
	m := ConfirmedMatch{BookItemIDs: []string{"B1"}, ExternalItemIDs: []string{"E1"}, ExplainedDifference: &explained}
	accepted, issues := validateConfirmedMatches([]ConfirmedMatch{m}, book, external, MatchingPolicy{})
	if len(issues) != 0 {
		t.Fatalf("expected no issues with explained difference, got %+v", issues)
	}
	if len(accepted) != 1 {
		t.Fatalf("expected match accepted with explained difference, got %+v", accepted)
	}
}

func TestConfirmed_MixedCurrencyRejected(t *testing.T) {
	book := testBookByID(BookItem{ItemID: "B1", Amount: 100, Direction: DirectionOutflow, Currency: "USD"})
	external := testExternalByID(ExternalItem{ItemID: "E1", Amount: 100, Direction: DirectionOutflow, Currency: "EUR"})
	m := ConfirmedMatch{BookItemIDs: []string{"B1"}, ExternalItemIDs: []string{"E1"}}
	accepted, issues := validateConfirmedMatches([]ConfirmedMatch{m}, book, external, MatchingPolicy{})
	if len(accepted) != 0 {
		t.Fatalf("expected rejection for mixed currency, got %+v", accepted)
	}
	if len(issues) == 0 || issues[0].Code != IssueMixedCurrency {
		t.Fatalf("expected IssueMixedCurrency, got %+v", issues)
	}
}

func TestConfirmed_ManyToManyOnlyViaConfirmed(t *testing.T) {
	// Many-to-many is only reachable through ConfirmedMatch (task section
	// 11/20) - verify it validates successfully when both sides have 2+
	// items and amounts tie.
	book := testBookByID(
		BookItem{ItemID: "B1", Amount: 100, Direction: DirectionOutflow},
		BookItem{ItemID: "B2", Amount: 200, Direction: DirectionOutflow},
	)
	external := testExternalByID(
		ExternalItem{ItemID: "E1", Amount: 150, Direction: DirectionOutflow},
		ExternalItem{ItemID: "E2", Amount: 150, Direction: DirectionOutflow},
	)
	m := ConfirmedMatch{BookItemIDs: []string{"B1", "B2"}, ExternalItemIDs: []string{"E1", "E2"}}
	accepted, issues := validateConfirmedMatches([]ConfirmedMatch{m}, book, external, MatchingPolicy{})
	if len(issues) != 0 {
		t.Fatalf("expected valid many-to-many confirmed match, got issues=%+v", issues)
	}
	if len(accepted) != 1 {
		t.Fatalf("expected 1 accepted many-to-many match")
	}
}
