package pdf

import "testing"

func TestSuppressRepeatedContent_RemovesExactRepeat(t *testing.T) {
	lines := []line{
		mkLine(0, 712, mkWord(72, 200, "Income"), mkWord(210, 280, "Statement")),
		mkLine(0, 655, mkWord(72, 100, "Revenue"), mkWord(300, 330, "850,000")),
		mkLine(1, 712, mkWord(72, 200, "Income"), mkWord(210, 280, "Statement")), // repeated
		mkLine(1, 655, mkWord(72, 100, "Expenses"), mkWord(300, 330, "210,000")),
	}
	extents := computePageExtents(lines)
	filtered, hits := suppressRepeatedContent(lines, extents)

	if len(filtered) != 3 {
		t.Fatalf("got %d filtered lines, want 3 (one repeat removed): %+v", len(filtered), filtered)
	}
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1: %+v", len(hits), hits)
	}
	if hits[0].pageIndex != 1 {
		t.Errorf("hit pageIndex = %d, want 1", hits[0].pageIndex)
	}
}

func TestSuppressRepeatedContent_DoesNotRemoveDistinctLines(t *testing.T) {
	lines := []line{
		mkLine(0, 655, mkWord(72, 100, "Revenue"), mkWord(300, 330, "850,000")),
		mkLine(1, 655, mkWord(72, 100, "Expenses"), mkWord(300, 330, "210,000")),
	}
	extents := computePageExtents(lines)
	filtered, hits := suppressRepeatedContent(lines, extents)
	if len(filtered) != 2 {
		t.Errorf("got %d filtered lines, want 2 (nothing repeated)", len(filtered))
	}
	if len(hits) != 0 {
		t.Errorf("got %d hits, want 0", len(hits))
	}
}

func TestSuppressRepeatedContent_CaseAndWhitespaceInsensitive(t *testing.T) {
	lines := []line{
		mkLine(0, 712, mkWord(72, 200, "Income"), mkWord(210, 280, "Statement")),
		mkLine(1, 712, mkWord(72, 200, "INCOME"), mkWord(210, 280, "STATEMENT")), // same text, different case
	}
	extents := computePageExtents(lines)
	filtered, hits := suppressRepeatedContent(lines, extents)
	if len(filtered) != 1 {
		t.Errorf("got %d filtered lines, want 1", len(filtered))
	}
	if len(hits) != 1 {
		t.Errorf("got %d hits, want 1", len(hits))
	}
}

func TestLooksLikePageFooter_BarePageNumberAtBottom(t *testing.T) {
	lines := []line{
		mkLine(0, 700, mkWord(72, 100, "Revenue")),
		mkLine(0, 72, mkWord(290, 300, "3")), // near the bottom
	}
	extents := computePageExtents(lines)
	if !looksLikePageFooter(lines[1], extents) {
		t.Error("expected a bare number near the page bottom to look like a footer")
	}
}

func TestLooksLikePageFooter_PageOfNFormat(t *testing.T) {
	lines := []line{
		mkLine(0, 700, mkWord(72, 100, "Revenue")),
		mkLine(0, 72, mkWord(250, 320, "Page"), mkWord(325, 340, "3"), mkWord(345, 360, "of"), mkWord(365, 380, "12")),
	}
	extents := computePageExtents(lines)
	if !looksLikePageFooter(lines[1], extents) {
		t.Error("expected \"Page 3 of 12\" near the page bottom to look like a footer")
	}
}

func TestLooksLikePageFooter_RealDataRowNeverSuppressed(t *testing.T) {
	// A real financial line ("Net Income  320,000") must never be
	// mistaken for a footer even if it happens to sit near the bottom of
	// its page.
	lines := []line{
		mkLine(0, 700, mkWord(72, 100, "Revenue")),
		mkLine(0, 72, mkWord(72, 130, "Net Income"), mkWord(300, 330, "320,000")),
	}
	extents := computePageExtents(lines)
	if looksLikePageFooter(lines[1], extents) {
		t.Error("a real financial data row must never be treated as a footer")
	}
}

func TestLooksLikePageFooter_NumberNotNearBottomIsNotFooter(t *testing.T) {
	// A bare number shape, but positioned near the TOP of the page (not
	// footer position) — position must matter, not just text shape.
	lines := []line{
		mkLine(0, 700, mkWord(290, 300, "3")),
		mkLine(0, 72, mkWord(72, 100, "Revenue")),
	}
	extents := computePageExtents(lines)
	if looksLikePageFooter(lines[0], extents) {
		t.Error("a page-number-shaped line near the TOP of the page should not be treated as a footer")
	}
}

func TestComputePageExtents_TracksPerPageRange(t *testing.T) {
	lines := []line{
		mkLine(0, 700, mkWord(72, 100, "a")),
		mkLine(0, 100, mkWord(72, 100, "b")),
		mkLine(1, 500, mkWord(72, 100, "c")),
	}
	extents := computePageExtents(lines)
	if extents[0].minY != 100 || extents[0].maxY != 700 {
		t.Errorf("page 0 extent = %+v, want {100 700}", extents[0])
	}
	if extents[1].minY != 500 || extents[1].maxY != 500 {
		t.Errorf("page 1 extent = %+v, want {500 500}", extents[1])
	}
}
