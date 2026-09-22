package tabular

import "testing"

func TestSelectSheetByRequestedName(t *testing.T) {
	candidates := []SheetCandidate{
		{Index: 0, Name: "Notes"},
		{Index: 1, Name: "P&L"},
	}
	got := SelectSheet(candidates, "P&L")
	if got.SelectedIndex != 1 {
		t.Errorf("SelectedIndex = %d, want 1", got.SelectedIndex)
	}
}

func TestSelectSheetByRequestedNameNotFound(t *testing.T) {
	candidates := []SheetCandidate{{Index: 0, Name: "Notes"}}
	got := SelectSheet(candidates, "Nonexistent")
	if got.SelectedIndex != -1 {
		t.Errorf("SelectedIndex = %d, want -1", got.SelectedIndex)
	}
}

func TestSelectSheetSingleNonEmpty(t *testing.T) {
	candidates := []SheetCandidate{
		{Index: 0, Name: "Empty", Empty: true},
		{Index: 1, Name: "Data", PlausibilityScore: 5},
	}
	got := SelectSheet(candidates, "")
	if got.SelectedIndex != 1 {
		t.Errorf("SelectedIndex = %d, want 1", got.SelectedIndex)
	}
}

func TestSelectSheetHighestScoreWins(t *testing.T) {
	candidates := []SheetCandidate{
		{Index: 0, Name: "Notes", PlausibilityScore: 2},
		{Index: 1, Name: "P&L", PlausibilityScore: 10},
	}
	got := SelectSheet(candidates, "")
	if got.SelectedIndex != 1 {
		t.Errorf("SelectedIndex = %d, want 1", got.SelectedIndex)
	}
}

func TestSelectSheetAmbiguousTie(t *testing.T) {
	candidates := []SheetCandidate{
		{Index: 0, Name: "Sheet1", PlausibilityScore: 5},
		{Index: 1, Name: "Sheet2", PlausibilityScore: 5},
	}
	got := SelectSheet(candidates, "")
	if !got.Ambiguous {
		t.Error("expected Ambiguous=true for a tie")
	}
	if got.SelectedIndex != -1 {
		t.Errorf("SelectedIndex = %d, want -1 on ambiguous tie", got.SelectedIndex)
	}
}

func TestSelectSheetNoNonEmptySheets(t *testing.T) {
	candidates := []SheetCandidate{
		{Index: 0, Name: "Empty1", Empty: true},
		{Index: 1, Name: "Empty2", Empty: true},
	}
	got := SelectSheet(candidates, "")
	if got.SelectedIndex != -1 {
		t.Errorf("SelectedIndex = %d, want -1", got.SelectedIndex)
	}
	if got.Ambiguous {
		t.Error("expected Ambiguous=false when there are no non-empty sheets at all")
	}
}

func TestScoreSheetName(t *testing.T) {
	if ScoreSheetName("P&L") <= 0 {
		t.Error("expected positive score for P&L")
	}
	if ScoreSheetName("Notes") >= 0 {
		t.Error("expected negative score for Notes")
	}
	if ScoreSheetName("Random Sheet Name") != 0 {
		t.Error("expected neutral score for unrecognized name")
	}
}
