package reconciliation

import (
	"strconv"
	"testing"
)

// TestInvariant_MatchExclusivity locks task section 77: "an item belongs
// to at most one final match group."
func TestInvariant_MatchExclusivity(t *testing.T) {
	in := buildDeterminismInput()
	r := Calculate(in)

	seenBook := make(map[string]bool)
	seenExternal := make(map[string]bool)
	for _, g := range r.MatchedGroups {
		for _, id := range g.BookItemIDs {
			if seenBook[id] {
				t.Fatalf("book item %s appears in more than one match group", id)
			}
			seenBook[id] = true
		}
		for _, id := range g.ExternalItemIDs {
			if seenExternal[id] {
				t.Fatalf("external item %s appears in more than one match group", id)
			}
			seenExternal[id] = true
		}
	}
}

// TestInvariant_MatchArithmetic locks task section 77: "Book vs external
// group difference follows orientation/tolerance exactly" — every
// MatchGroup's Difference must equal BookAmount - ExternalAmount.
func TestInvariant_MatchArithmetic(t *testing.T) {
	in := buildDeterminismInput()
	r := Calculate(in)
	for _, g := range r.MatchedGroups {
		want := g.BookAmount - g.ExternalAmount
		if g.Difference != want {
			t.Errorf("match %s: Difference %v != BookAmount-ExternalAmount %v", g.MatchID, g.Difference, want)
		}
	}
}

// TestInvariant_SideRollforward locks task section 77: "Opening +
// Activity = ExpectedEnding."
func TestInvariant_SideRollforward(t *testing.T) {
	opening := 500.0
	items := []float64{10, 20, -5}
	bal, _ := resolveBalance(BalanceInput{OpeningBalance: &opening}, items, false, 0.01, 1)
	if !bal.ExpectedEndingAvailable {
		t.Fatalf("expected ExpectedEnding available")
	}
	if bal.ExpectedEnding != 525 {
		t.Fatalf("expected ExpectedEnding 525 (500+10+20-5), got %v", bal.ExpectedEnding)
	}
}

// TestInvariant_CrossSideDifference locks task section 77:
// "AdjustedBook - AdjustedExternal = Difference."
func TestInvariant_CrossSideDifference(t *testing.T) {
	in := buildDeterminismInput()
	r := Calculate(in)
	if !r.Equation.Available {
		t.Fatalf("expected equation available")
	}
	want := r.Equation.AdjustedBookBalance - r.Equation.AdjustedExternalBalance
	if r.Equation.Difference != want {
		t.Fatalf("expected Difference == AdjustedBookBalance - AdjustedExternalBalance, got %v != %v", r.Equation.Difference, want)
	}
}

// TestInvariant_ConfirmedPrecedenceNeverDisplaced (task section
// 77/81): a valid confirmed match is applied verbatim even when a
// "stronger" automatic candidate exists for the same items.
func TestInvariant_ConfirmedPrecedenceNeverDisplaced(t *testing.T) {
	in := Input{
		AccountID: "A",
		AsOfDate:  "2025-01-01",
		BookItems: []BookItem{
			{ItemID: "B1", Date: "2025-01-10", Amount: 500, Direction: DirectionOutflow},
		},
		ExternalItems: []ExternalItem{
			{ItemID: "E1", Date: "2025-01-10", Amount: 500, Direction: DirectionOutflow}, // exact same-date match
			{ItemID: "E2", Date: "2025-01-15", Amount: 500, Direction: DirectionOutflow}, // the confirmed target
		},
		ConfirmedMatches: []ConfirmedMatch{
			{MatchID: "MANUAL", BookItemIDs: []string{"B1"}, ExternalItemIDs: []string{"E2"}},
		},
		Policy: MatchingPolicy{DateWindowDays: 10},
	}
	r := Calculate(in)
	if len(r.MatchedGroups) != 1 {
		t.Fatalf("expected exactly 1 match group, got %d: %+v", len(r.MatchedGroups), r.MatchedGroups)
	}
	if r.MatchedGroups[0].ExternalItemIDs[0] != "E2" {
		t.Fatalf("expected confirmed match to E2 (never displaced by E1's stronger auto-candidate), got %v", r.MatchedGroups[0].ExternalItemIDs)
	}
	// E1 must remain genuinely unmatched, not silently absorbed.
	found := false
	for _, id := range r.UnmatchedExternalItems {
		if id == "E1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected E1 unmatched, got %v", r.UnmatchedExternalItems)
	}
}

// TestInvariant_AmbiguityNeverResolvedArbitrarily locks task section 78:
// "Two equally valid candidates must remain unresolved/ambiguous."
func TestInvariant_AmbiguityNeverResolvedArbitrarily(t *testing.T) {
	in := Input{
		AccountID: "A",
		AsOfDate:  "2025-01-01",
		BookItems: []BookItem{
			{ItemID: "B1", Date: "2025-01-10", Amount: 75, Direction: DirectionOutflow},
		},
		ExternalItems: []ExternalItem{
			{ItemID: "E1", Date: "2025-01-10", Amount: 75, Direction: DirectionOutflow},
			{ItemID: "E2", Date: "2025-01-10", Amount: 75, Direction: DirectionOutflow},
		},
	}
	r := Calculate(in)
	if len(r.MatchedGroups) != 0 {
		t.Fatalf("expected no arbitrary resolution, got %+v", r.MatchedGroups)
	}
	if len(r.AmbiguousCandidates) == 0 {
		t.Fatalf("expected ambiguous candidates reported")
	}
}

// TestInvariant_RepeatedAmountNoO2Explosion locks task section 79 at
// Calculate's full-pipeline level (matcher_test.go already locks it at
// the matchStage level) — 150 identical amounts on each side must
// complete quickly and produce ambiguity, not a crash or timeout.
func TestInvariant_RepeatedAmountNoO2Explosion(t *testing.T) {
	const n = 150
	var book []BookItem
	var external []ExternalItem
	for i := 0; i < n; i++ {
		book = append(book, BookItem{ItemID: "B" + strconv.Itoa(i), Date: "2025-01-10", Amount: 10, Direction: DirectionOutflow})
		external = append(external, ExternalItem{ItemID: "E" + strconv.Itoa(i), Date: "2025-01-10", Amount: 10, Direction: DirectionOutflow})
	}
	r := Calculate(Input{AccountID: "A", AsOfDate: "2025-01-01", BookItems: book, ExternalItems: external})
	if len(r.MatchedGroups) != 0 {
		t.Fatalf("expected 0 groups for fully indistinguishable repeated amounts, got %d", len(r.MatchedGroups))
	}
}

// TestInvariant_CompositeUniqueVsAmbiguous locks task section 80's exact
// worked examples at the top-level Calculate.
func TestInvariant_CompositeUniqueVsAmbiguous(t *testing.T) {
	uniquePolicy := MatchingPolicy{
		EnableCompositeMatching: true, MaxCompositeGroupSize: 5,
		MaxCandidatesPerItem: 20, MaxCompositeSearchCombinations: 1000, DateWindowDays: 5,
	}

	unique := Calculate(Input{
		AccountID: "A", AsOfDate: "2025-01-01",
		BookItems: []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 300, Direction: DirectionOutflow}},
		ExternalItems: []ExternalItem{
			{ItemID: "E1", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
			{ItemID: "E2", Date: "2025-01-10", Amount: 200, Direction: DirectionOutflow},
		},
		Policy: uniquePolicy,
	})
	if len(unique.MatchedGroups) != 1 {
		t.Fatalf("expected unique composite sum to auto-match, got %+v", unique.MatchedGroups)
	}

	ambiguous := Calculate(Input{
		AccountID: "A", AsOfDate: "2025-01-01",
		BookItems: []BookItem{{ItemID: "B1", Date: "2025-01-10", Amount: 300, Direction: DirectionOutflow}},
		ExternalItems: []ExternalItem{
			{ItemID: "E1", Date: "2025-01-10", Amount: 50, Direction: DirectionOutflow},
			{ItemID: "E2", Date: "2025-01-10", Amount: 250, Direction: DirectionOutflow},
			{ItemID: "E3", Date: "2025-01-10", Amount: 100, Direction: DirectionOutflow},
			{ItemID: "E4", Date: "2025-01-10", Amount: 200, Direction: DirectionOutflow},
		},
		Policy: uniquePolicy,
	})
	if len(ambiguous.MatchedGroups) != 0 {
		t.Fatalf("expected ambiguous composite sums to remain unresolved, got %+v", ambiguous.MatchedGroups)
	}
}

// TestInvariant_ManualMatchOverridesStrongerAutomatic locks task section
// 81's exact scenario name: "Caller-confirmed B1/E4 remains even if E3
// looks stronger automatically, provided confirmed match validates."
func TestInvariant_ManualMatchOverridesStrongerAutomatic(t *testing.T) {
	in := Input{
		AccountID: "A",
		AsOfDate:  "2025-01-01",
		BookItems: []BookItem{
			{ItemID: "B1", Date: "2025-01-10", Amount: 400, Direction: DirectionOutflow, Reference: "REF1"},
		},
		ExternalItems: []ExternalItem{
			{ItemID: "E3", Date: "2025-01-10", Amount: 400, Direction: DirectionOutflow, Reference: "REF1"}, // reference+amount+date exact
			{ItemID: "E4", Date: "2025-01-14", Amount: 400, Direction: DirectionOutflow},                    // the confirmed target, weaker evidence
		},
		ConfirmedMatches: []ConfirmedMatch{
			{MatchID: "B1E4", BookItemIDs: []string{"B1"}, ExternalItemIDs: []string{"E4"}},
		},
		Policy: MatchingPolicy{DateWindowDays: 10, ReferenceNormalization: ReferenceNormalization{Trim: true}},
	}
	r := Calculate(in)
	if len(r.MatchedGroups) != 1 || r.MatchedGroups[0].ExternalItemIDs[0] != "E4" {
		t.Fatalf("expected B1 confirmed to E4 despite E3's stronger reference+amount+date evidence, got %+v", r.MatchedGroups)
	}
}

// TestInvariant_StatusCoverage locks task section 82: RECONCILED,
// RECONCILED_WITH_ITEMS, UNRECONCILED, INCOMPLETE, INVALID all reachable
// through the full Calculate pipeline (not just deriveStatus directly,
// which equation_test.go already covers).
func TestInvariant_StatusCoverage(t *testing.T) {
	book := 100.0
	external := 100.0
	reconciled := Calculate(Input{
		AccountID: "A", AsOfDate: "2025-01-01",
		BookBalance: BalanceInput{EndingBalance: &book}, ExternalBalance: BalanceInput{EndingBalance: &external},
		Policy: MatchingPolicy{AmountTolerance: 0.01},
	})
	if reconciled.Status != StatusReconciled {
		t.Errorf("expected RECONCILED, got %s", reconciled.Status)
	}

	withItems := Calculate(Input{
		AccountID: "A", AsOfDate: "2025-01-01",
		BookBalance: BalanceInput{EndingBalance: &book}, ExternalBalance: BalanceInput{EndingBalance: &external},
		ReconcilingItems: []ReconcilingItem{{ItemID: "R1", Type: ReconcilingOther, Side: ReconcilingSideBook, Amount: 0}},
		Policy:           MatchingPolicy{AmountTolerance: 0.01},
	})
	if withItems.Status != StatusReconciledWithItems {
		t.Errorf("expected RECONCILED_WITH_ITEMS, got %s", withItems.Status)
	}

	mismatched := 200.0
	unreconciled := Calculate(Input{
		AccountID: "A", AsOfDate: "2025-01-01",
		BookBalance: BalanceInput{EndingBalance: &book}, ExternalBalance: BalanceInput{EndingBalance: &mismatched},
		Policy: MatchingPolicy{AmountTolerance: 0.01},
	})
	if unreconciled.Status != StatusUnreconciled {
		t.Errorf("expected UNRECONCILED, got %s", unreconciled.Status)
	}

	incomplete := Calculate(Input{AccountID: "A", AsOfDate: "2025-01-01"})
	if incomplete.Status != StatusIncomplete {
		t.Errorf("expected INCOMPLETE, got %s", incomplete.Status)
	}

	invalid := Calculate(Input{})
	if invalid.Status != StatusInvalid {
		t.Errorf("expected INVALID, got %s", invalid.Status)
	}
}
