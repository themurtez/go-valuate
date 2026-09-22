package pdf

import "testing"

func TestHasUsableTextLayer_NoFragments(t *testing.T) {
	ok, reason := hasUsableTextLayer(extractResult{pageCount: 1})
	if ok {
		t.Error("expected ok=false for zero fragments")
	}
	if reason == "" {
		t.Error("expected a non-empty reason")
	}
}

func TestHasUsableTextLayer_BelowMinimum(t *testing.T) {
	frags := make([]fragment, minTextFragmentsForUsableDocument-1)
	ok, _ := hasUsableTextLayer(extractResult{pageCount: 1, fragments: frags})
	if ok {
		t.Error("expected ok=false below the minimum fragment count")
	}
}

func TestHasUsableTextLayer_SparseAcrossManyPages(t *testing.T) {
	// Enough total fragments to pass the absolute minimum, but spread
	// across so many pages that the per-page average is too low.
	frags := make([]fragment, minTextFragmentsForUsableDocument+10)
	ok, _ := hasUsableTextLayer(extractResult{pageCount: 100, fragments: frags})
	if ok {
		t.Error("expected ok=false when average fragments/page is too low")
	}
}

func TestHasUsableTextLayer_RealDocumentPasses(t *testing.T) {
	frags := make([]fragment, 500)
	ok, reason := hasUsableTextLayer(extractResult{pageCount: 1, fragments: frags})
	if !ok {
		t.Errorf("expected ok=true for a realistic fragment count, got reason=%q", reason)
	}
}

func TestPagesWithNoText_IdentifiesSilentPages(t *testing.T) {
	extracted := extractResult{
		pageCount: 3,
		fragments: []fragment{
			{pageIndex: 0, s: "a"},
			{pageIndex: 2, s: "b"},
		},
	}
	pages := pagesWithNoText(extracted)
	if len(pages) != 1 || pages[0] != 1 {
		t.Errorf("pagesWithNoText = %+v, want [1]", pages)
	}
}

func TestFindTitleSignals_MatchesShortTitleLine(t *testing.T) {
	lines := []line{
		mkLine(0, 730, mkWord(72, 200, "Some Company Inc.")),
		mkLine(0, 712, mkWord(72, 200, "Income"), mkWord(210, 280, "Statement")),
		mkLine(0, 680, mkWord(72, 100, "Revenue"), mkWord(300, 330, "850,000")),
	}
	signals := findTitleSignals(lines)
	if len(signals) == 0 {
		t.Fatal("expected at least one title signal")
	}
}

func TestFindTitleSignals_IgnoresLongLines(t *testing.T) {
	// A long line that happens to contain "income statement" as a
	// substring must not be treated as a title signal — only genuinely
	// short, title-shaped lines are considered.
	longWords := []word{}
	longText := "This is a very long line of running text that happens to mention an income statement somewhere in its middle for testing purposes only"
	pos := 72.0
	for _, w := range []rune(longText) {
		_ = w
		longWords = append(longWords, mkWord(pos, pos+5, "x"))
		pos += 6
	}
	lines := []line{{pageIndex: 0, y: 650, words: longWords}}
	signals := findTitleSignals(lines)
	if len(signals) != 0 {
		t.Errorf("got %d signals for a long line, want 0", len(signals))
	}
}

func TestSplitSections_SingleStatementNoSplit(t *testing.T) {
	lines := []line{
		mkLine(0, 730, mkWord(72, 200, "Some Company Inc.")),
		mkLine(0, 712, mkWord(72, 200, "Income"), mkWord(210, 280, "Statement")),
		mkLine(0, 680, mkWord(72, 100, "Account"), mkWord(300, 330, "FY2025")),
		mkLine(0, 655, mkWord(72, 100, "Revenue"), mkWord(300, 330, "850,000")),
	}
	sections, warnings := splitSections(lines, defaultRowYTolerance)
	if len(sections) != 1 {
		t.Fatalf("got %d sections, want 1: %+v", len(sections), sections)
	}
	if len(sections[0].lines) != len(lines) {
		t.Errorf("section has %d lines, want all %d folded in", len(sections[0].lines), len(lines))
	}
	if len(warnings) != 0 {
		t.Errorf("got %d warnings for a single statement, want 0: %+v", len(warnings), warnings)
	}
}

func TestSplitSections_TwoDifferentStatementTypesSplit(t *testing.T) {
	lines := []line{
		mkLine(0, 712, mkWord(72, 200, "Income"), mkWord(210, 280, "Statement")),
		mkLine(0, 680, mkWord(72, 100, "Account"), mkWord(300, 330, "FY2025")),
		mkLine(0, 655, mkWord(72, 100, "Revenue"), mkWord(300, 330, "850,000")),
		mkLine(1, 712, mkWord(72, 200, "Balance"), mkWord(210, 280, "Sheet")),
		mkLine(1, 680, mkWord(72, 100, "Account"), mkWord(300, 330, "2025")),
		mkLine(1, 655, mkWord(72, 100, "Cash"), mkWord(300, 330, "410,000")),
	}
	sections, warnings := splitSections(lines, defaultRowYTolerance)
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2: %+v", len(sections), sections)
	}
	foundMultiple := false
	for _, w := range warnings {
		if w.Code == "MULTIPLE_STATEMENTS_DETECTED" {
			foundMultiple = true
		}
	}
	if !foundMultiple {
		t.Error("expected MULTIPLE_STATEMENTS_DETECTED warning")
	}
}

func TestSplitSections_RepeatedSameTypeSignalDoesNotSplit(t *testing.T) {
	// Two "Income Statement" title occurrences (e.g. reprinted on page 2)
	// must NOT be treated as two separate statements.
	lines := []line{
		mkLine(0, 712, mkWord(72, 200, "Income"), mkWord(210, 280, "Statement")),
		mkLine(0, 655, mkWord(72, 100, "Revenue"), mkWord(300, 330, "850,000")),
		mkLine(1, 712, mkWord(72, 200, "Income"), mkWord(210, 280, "Statement")),
		mkLine(1, 655, mkWord(72, 100, "Expenses"), mkWord(300, 330, "210,000")),
	}
	sections, _ := splitSections(lines, defaultRowYTolerance)
	if len(sections) != 1 {
		t.Fatalf("got %d sections, want 1 (repeated same-type signal must not split): %+v", len(sections), sections)
	}
}

func TestLargeVerticalGapBoundaries_DetectsBigGap(t *testing.T) {
	lines := []line{
		mkLine(0, 700, mkWord(72, 100, "a")),
		mkLine(0, 680, mkWord(72, 100, "b")), // normal 20pt gap
		mkLine(0, 500, mkWord(72, 100, "c")), // huge 180pt gap
	}
	boundaries := largeVerticalGapBoundaries(lines, defaultRowYTolerance)
	if len(boundaries) != 1 || boundaries[0] != 2 {
		t.Errorf("boundaries = %+v, want [2]", boundaries)
	}
}

func TestLargeVerticalGapBoundaries_IgnoresPageBreaks(t *testing.T) {
	lines := []line{
		mkLine(0, 100, mkWord(72, 100, "a")), // near bottom of page 0
		mkLine(1, 700, mkWord(72, 100, "b")), // near top of page 1 — a big Y "jump" but a different page
	}
	boundaries := largeVerticalGapBoundaries(lines, defaultRowYTolerance)
	if len(boundaries) != 0 {
		t.Errorf("boundaries = %+v, want none (page breaks are not gap boundaries)", boundaries)
	}
}
