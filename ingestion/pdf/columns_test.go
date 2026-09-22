package pdf

import (
	"reflect"
	"testing"
)

func mkLine(page int, y float64, words ...word) line {
	return line{pageIndex: page, y: y, words: words}
}

func mkWord(x0, x1 float64, text string) word {
	return word{x0: x0, x1: x1, fontSize: 10, text: text}
}

func TestBuildGrid_LabelAndValueColumns(t *testing.T) {
	lines := []line{
		mkLine(0, 680, mkWord(72, 100, "Account"), mkWord(300, 330, "FY2025")),
		mkLine(0, 655, mkWord(72, 100, "Revenue"), mkWord(300, 330, "850,000")),
		mkLine(0, 630, mkWord(72, 150, "Cost of Goods Sold"), mkWord(300, 330, "320,000")),
	}
	grid := buildGrid(lines)
	if len(grid) != 3 {
		t.Fatalf("got %d grid rows, want 3", len(grid))
	}
	if grid[1][0] != "Revenue" {
		t.Errorf("grid[1][0] = %q, want Revenue", grid[1][0])
	}
	if grid[1][1] != "850,000" {
		t.Errorf("grid[1][1] = %q, want 850,000", grid[1][1])
	}
}

func TestBuildGrid_MultiWordLabelMergesIntoOneCell(t *testing.T) {
	// "Total Operating Expenses" reconstructed as three separate words
	// (ordinary inter-word spacing splits them at the word-grouping
	// stage) must still land in ONE grid cell, since all three fall
	// within the label column's X region.
	lines := []line{
		mkLine(0, 680, mkWord(72, 100, "Account"), mkWord(300, 330, "FY2025")),
		mkLine(0, 600, mkWord(72, 95, "Total"), mkWord(100, 160, "Operating"), mkWord(165, 230, "Expenses"), mkWord(300, 330, "540,000")),
	}
	grid := buildGrid(lines)
	if grid[1][0] != "Total Operating Expenses" {
		t.Errorf("grid[1][0] = %q, want %q", grid[1][0], "Total Operating Expenses")
	}
}

func TestBuildGrid_HeaderLineAnchorsColumnBoundaries(t *testing.T) {
	// Two period columns at X=260 and X=340 — the header row's own word
	// positions should become the column boundaries even without any
	// clustering signal from the body rows (only one body row here).
	lines := []line{
		mkLine(0, 665, mkWord(72, 100, "Account"), mkWord(260, 290, "2024"), mkWord(340, 370, "2025")),
		mkLine(0, 640, mkWord(72, 100, "Revenue"), mkWord(260, 300, "1,450,000"), mkWord(340, 380, "1,780,000")),
	}
	grid := buildGrid(lines)
	if len(grid[0]) != 3 {
		t.Fatalf("got %d columns, want 3 (label + 2 periods): %+v", len(grid[0]), grid[0])
	}
	if grid[1][1] != "1,450,000" || grid[1][2] != "1,780,000" {
		t.Errorf("grid[1] = %+v, want [Revenue-ish, 1,450,000, 1,780,000]", grid[1])
	}
}

func TestBuildGrid_EmptyLinesReturnsNil(t *testing.T) {
	grid := buildGrid(nil)
	if grid != nil {
		t.Errorf("got %+v, want nil", grid)
	}
}

func TestClusterColumnXs_SeparatesDistantColumns(t *testing.T) {
	lines := []line{
		mkLine(0, 650, mkWord(72, 100, "Revenue"), mkWord(300, 330, "850,000")),
		mkLine(0, 630, mkWord(72, 100, "Expenses"), mkWord(300, 330, "210,000")),
	}
	bounds := clusterColumnXs(lines, 10)
	if len(bounds) != 2 {
		t.Fatalf("got %d boundaries, want 2: %+v", len(bounds), bounds)
	}
	if bounds[0].x != 72 || bounds[1].x != 300 {
		t.Errorf("boundaries = %+v, want [{72} {300}]", bounds)
	}
}

func TestClusterColumnXs_GroupsNearbyRightAlignedNumbers(t *testing.T) {
	// Right-aligned numbers of different digit counts start at slightly
	// different X (a 6-digit number starts further left than a 3-digit
	// one at the same right edge) — these must still cluster into ONE
	// column, not split into several.
	lines := []line{
		mkLine(0, 650, mkWord(72, 100, "A"), mkWord(298, 330, "850,000")),
		mkLine(0, 630, mkWord(72, 100, "B"), mkWord(320, 330, "10")), // starts further right (fewer digits)
	}
	bounds := clusterColumnXs(lines, 10)
	if len(bounds) != 2 {
		t.Fatalf("got %d boundaries, want 2 (label + one value column): %+v", len(bounds), bounds)
	}
}

func TestColumnFor_AssignsToCorrectRegion(t *testing.T) {
	bounds := []columnBoundary{{x: 72}, {x: 300}}
	cases := []struct {
		x    float64
		want int
	}{
		{72, 0},
		{150, 0},
		{299, 0},
		{300, 1},
		{500, 1},
	}
	for _, c := range cases {
		got := columnFor(c.x, bounds)
		if got != c.want {
			t.Errorf("columnFor(%v) = %d, want %d", c.x, got, c.want)
		}
	}
}

func TestFindHeaderLine_RecognizesPeriodHeader(t *testing.T) {
	lines := []line{
		mkLine(0, 700, mkWord(72, 200, "Some Company Title")),
		mkLine(0, 665, mkWord(72, 100, "Account"), mkWord(260, 290, "2024"), mkWord(340, 370, "2025")),
		mkLine(0, 640, mkWord(72, 100, "Revenue")),
	}
	header, ok := findHeaderLine(lines)
	if !ok {
		t.Fatal("expected to find a header line")
	}
	if len(header.words) != 3 {
		t.Errorf("header line has %d words, want 3", len(header.words))
	}
}

func TestFindHeaderLine_NoneFound(t *testing.T) {
	lines := []line{
		mkLine(0, 700, mkWord(72, 200, "Just a title")),
		mkLine(0, 680, mkWord(72, 100, "Some label")),
	}
	_, ok := findHeaderLine(lines)
	if ok {
		t.Error("expected no header line to be found")
	}
}

func TestDominantFontSize_PicksMostCommon(t *testing.T) {
	lines := []line{
		{words: []word{{fontSize: 14}, {fontSize: 10}, {fontSize: 10}}},
		{words: []word{{fontSize: 10}}},
	}
	if got := dominantFontSize(lines); got != 10 {
		t.Errorf("dominantFontSize = %v, want 10", got)
	}
}

func TestAssignLineToColumns_EmptyColumnsAreEmptyStrings(t *testing.T) {
	bounds := []columnBoundary{{x: 72}, {x: 300}, {x: 400}}
	ln := mkLine(0, 650, mkWord(72, 100, "Revenue"), mkWord(300, 330, "850,000"))
	row := assignLineToColumns(ln, bounds)
	want := []string{"Revenue", "850,000", ""}
	if !reflect.DeepEqual(row, want) {
		t.Errorf("row = %+v, want %+v", row, want)
	}
}
