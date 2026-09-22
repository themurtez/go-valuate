package pdf

import "testing"

func TestGroupWords_MergesCloseGlyphsIntoOneWord(t *testing.T) {
	// Simulates "Revenue" drawn as 7 separate glyph fragments, each
	// tightly packed (as real per-character PDF extraction produces —
	// see the package doc comment's spike discussion).
	frags := []fragment{
		{pageIndex: 0, x: 72.0, y: 650, fontSize: 10, w: 7.22, s: "R"},
		{pageIndex: 0, x: 79.22, y: 650, fontSize: 10, w: 5.56, s: "e"},
		{pageIndex: 0, x: 84.78, y: 650, fontSize: 10, w: 5.00, s: "v"},
		{pageIndex: 0, x: 89.78, y: 650, fontSize: 10, w: 5.56, s: "e"},
		{pageIndex: 0, x: 95.34, y: 650, fontSize: 10, w: 5.56, s: "n"},
		{pageIndex: 0, x: 100.90, y: 650, fontSize: 10, w: 5.56, s: "u"},
		{pageIndex: 0, x: 106.46, y: 650, fontSize: 10, w: 5.56, s: "e"},
	}
	words := groupWords(frags, defaultRowYTolerance)
	if len(words) != 1 {
		t.Fatalf("got %d words, want 1: %+v", len(words), words)
	}
	if words[0].text != "Revenue" {
		t.Errorf("text = %q, want %q", words[0].text, "Revenue")
	}
}

func TestGroupWords_SplitsOnLargeGap(t *testing.T) {
	// "Revenue" (ends ~112) then a value column starting at X=300: the
	// gap (~188pt) is far larger than any word-internal gap, so these
	// must split into two separate words.
	frags := []fragment{
		{pageIndex: 0, x: 72.0, y: 650, fontSize: 10, w: 40, s: "Revenue"},
		{pageIndex: 0, x: 300.0, y: 650, fontSize: 10, w: 30, s: "850,000"},
	}
	words := groupWords(frags, defaultRowYTolerance)
	if len(words) != 2 {
		t.Fatalf("got %d words, want 2: %+v", len(words), words)
	}
	if words[0].text != "Revenue" || words[1].text != "850,000" {
		t.Errorf("words = %+v, want [Revenue, 850,000]", words)
	}
}

func TestGroupWords_EmptyFragmentsDropped(t *testing.T) {
	frags := []fragment{
		{pageIndex: 0, x: 72, y: 650, fontSize: 10, s: ""},
		{pageIndex: 0, x: 80, y: 650, fontSize: 10, s: "X"},
	}
	words := groupWords(frags, defaultRowYTolerance)
	if len(words) != 1 {
		t.Fatalf("got %d words, want 1: %+v", len(words), words)
	}
}

func TestGroupWords_SortsOutOfExtractionOrder(t *testing.T) {
	// Fragments arrive in an order that does NOT match visual reading
	// order (the value drawn before the label — some producers do this):
	// groupWords must still reconstruct left-to-right order within the
	// line.
	frags := []fragment{
		{pageIndex: 0, x: 300.0, y: 650, fontSize: 10, w: 30, s: "850,000"},
		{pageIndex: 0, x: 72.0, y: 650, fontSize: 10, w: 40, s: "Revenue"},
	}
	words := groupWords(frags, defaultRowYTolerance)
	if len(words) != 2 {
		t.Fatalf("got %d words, want 2", len(words))
	}
	if words[0].text != "Revenue" {
		t.Errorf("words[0] = %q, want Revenue (leftmost first, regardless of extraction order)", words[0].text)
	}
}

func TestGroupRows_ToleratesSmallYJitter(t *testing.T) {
	words := []word{
		{pageIndex: 0, x0: 72, x1: 110, y: 650.0, text: "Revenue"},
		{pageIndex: 0, x0: 300, x1: 340, y: 650.4, text: "850,000"}, // 0.4pt jitter
	}
	lines := groupRows(words, defaultRowYTolerance)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (small Y jitter must merge): %+v", len(lines), lines)
	}
	if len(lines[0].words) != 2 {
		t.Fatalf("got %d words in the line, want 2", len(lines[0].words))
	}
}

func TestGroupRows_SeparatesGenuinelyDifferentRows(t *testing.T) {
	words := []word{
		{pageIndex: 0, x0: 72, x1: 110, y: 650, text: "Revenue"},
		{pageIndex: 0, x0: 72, x1: 130, y: 630, text: "Expenses"}, // 20pt lower: a real different row
	}
	lines := groupRows(words, defaultRowYTolerance)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %+v", len(lines), lines)
	}
}

func TestGroupRows_PreservesLeftToRightOrderWithinLine(t *testing.T) {
	words := []word{
		{pageIndex: 0, x0: 300, x1: 340, y: 650, text: "850,000"},
		{pageIndex: 0, x0: 72, x1: 110, y: 650, text: "Revenue"},
	}
	lines := groupRows(words, defaultRowYTolerance)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if lines[0].words[0].text != "Revenue" || lines[0].words[1].text != "850,000" {
		t.Errorf("line words = %+v, want [Revenue, 850,000]", lines[0].words)
	}
}

func TestGroupRows_SeparatesDifferentPages(t *testing.T) {
	words := []word{
		{pageIndex: 0, x0: 72, x1: 110, y: 650, text: "Revenue"},
		{pageIndex: 1, x0: 72, x1: 110, y: 650, text: "Revenue"}, // same Y, different page
	}
	lines := groupRows(words, defaultRowYTolerance)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (same Y on different pages must not merge)", len(lines))
	}
}
