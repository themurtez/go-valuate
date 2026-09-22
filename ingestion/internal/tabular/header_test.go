package tabular

import "testing"

func TestDetectHeaderRow(t *testing.T) {
	grid := Grid{
		{"Acme Corp", "", ""},
		{"Income Statement", "", ""},
		{"Account", "2023", "2024"},
		{"Revenue", "1000", "1200"},
	}
	got := DetectHeaderRow(grid, 0)
	if got.RowIndex != 2 {
		t.Fatalf("RowIndex = %d, want 2", got.RowIndex)
	}
	if len(got.Columns) != 2 {
		t.Fatalf("len(Columns) = %d, want 2", len(got.Columns))
	}
	if got.Columns[0].Period.CanonicalID != "2023" || got.Columns[1].Period.CanonicalID != "2024" {
		t.Errorf("unexpected columns: %+v", got.Columns)
	}
}

func TestDetectHeaderRowMultipleCandidates(t *testing.T) {
	// Two rows both look like plausible headers: a "Current Year / Prior
	// Year" row (weak-confidence period signal), then the real header row
	// with two full years. CandidateRows should include both, but
	// RowIndex should pick the row with more period-like columns.
	grid := Grid{
		{"Account", "Current Year", ""},
		{"Account", "2023", "2024"},
	}
	got := DetectHeaderRow(grid, 0)
	if len(got.CandidateRows) < 2 {
		t.Errorf("expected multiple candidate rows, got %v", got.CandidateRows)
	}
	if got.RowIndex != 1 {
		t.Errorf("RowIndex = %d, want 1 (more period columns)", got.RowIndex)
	}
}

func TestDetectHeaderRowNoPeriods(t *testing.T) {
	grid := Grid{
		{"Just some notes", "nothing here"},
		{"more text", "still nothing"},
	}
	got := DetectHeaderRow(grid, 0)
	if got.RowIndex != -1 {
		t.Errorf("RowIndex = %d, want -1 for no detectable periods", got.RowIndex)
	}
}

func TestDetectLabelColumn(t *testing.T) {
	grid := Grid{
		{"Account", "2023", "2024"},
		{"Revenue", "1000", "1200"},
		{"COGS", "500", "600"},
	}
	got := DetectLabelColumn(grid)
	if got != 0 {
		t.Errorf("DetectLabelColumn = %d, want 0", got)
	}
}

func TestDetectLabelColumnNotFirst(t *testing.T) {
	// Label column is column 1; column 0 holds account numbers (numeric).
	grid := Grid{
		{"Code", "Account", "2023"},
		{"4000", "Revenue", "1000"},
		{"5000", "COGS", "500"},
	}
	got := DetectLabelColumn(grid)
	if got != 1 {
		t.Errorf("DetectLabelColumn = %d, want 1", got)
	}
}
