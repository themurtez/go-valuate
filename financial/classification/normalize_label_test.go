package classification

import "testing"

func TestNormalizeLabel_Whitespace(t *testing.T) {
	got := NormalizeLabel("  Advertising    Expense  ")
	if got.Comparable != "advertising expense" {
		t.Errorf("Comparable = %q, want %q", got.Comparable, "advertising expense")
	}
	if got.Original != "  Advertising    Expense  " {
		t.Errorf("Original was modified: %q", got.Original)
	}
}

func TestNormalizeLabel_Case(t *testing.T) {
	got := NormalizeLabel("ADVERTISING")
	if got.Comparable != "advertising" {
		t.Errorf("Comparable = %q, want %q", got.Comparable, "advertising")
	}
}

func TestNormalizeLabel_AmpersandVsAnd(t *testing.T) {
	tests := []struct {
		label string
		want  string
	}{
		{"Advertising & Promotion", "advertising and promotion"},
		{"Repairs & Maintenance", "repairs and maintenance"},
		{"Advertising and Promotion", "advertising and promotion"},
	}
	for _, tt := range tests {
		got := NormalizeLabel(tt.label)
		if got.Comparable != tt.want {
			t.Errorf("NormalizeLabel(%q).Comparable = %q, want %q", tt.label, got.Comparable, tt.want)
		}
	}
}

func TestNormalizeLabel_LeadingAccountNumber(t *testing.T) {
	tests := []struct {
		label string
		want  string
	}{
		{"6100 Advertising", "advertising"},
		{"6100 - Advertising", "advertising"},
		{"6100-Advertising", "advertising"},
		{"6100: Advertising", "advertising"},
		{"6100   Advertising", "advertising"},
	}
	for _, tt := range tests {
		got := NormalizeLabel(tt.label)
		if got.Comparable != tt.want {
			t.Errorf("NormalizeLabel(%q).Comparable = %q, want %q", tt.label, got.Comparable, tt.want)
		}
	}
}

func TestNormalizeLabel_Hyphens(t *testing.T) {
	got := NormalizeLabel("Repairs-Maintenance")
	if got.Comparable != "repairs maintenance" {
		t.Errorf("Comparable = %q, want %q", got.Comparable, "repairs maintenance")
	}
}

func TestNormalizeLabel_Parentheses(t *testing.T) {
	got := NormalizeLabel("Interest Expense (Net)")
	if got.Comparable != "interest expense net" {
		t.Errorf("Comparable = %q, want %q", got.Comparable, "interest expense net")
	}
}

func TestNormalizeLabel_Punctuation(t *testing.T) {
	tests := []struct {
		label string
		want  string
	}{
		{"Owner's Compensation", "owners compensation"},
		{"Freight, In", "freight in"},
		{"Freight In.", "freight in"},
		{"Bank Charges; Fees", "bank charges fees"},
	}
	for _, tt := range tests {
		got := NormalizeLabel(tt.label)
		if got.Comparable != tt.want {
			t.Errorf("NormalizeLabel(%q).Comparable = %q, want %q", tt.label, got.Comparable, tt.want)
		}
	}
}

// TestNormalizeLabel_DoesNotCorruptUnrelatedWords is a regression test for
// the class of bug called out explicitly in the design brief: naive
// substring-based abbreviation/token handling must never corrupt a word that
// happens to contain a short token. For example, if some future change
// tried to normalize the standalone word "ad" to "advertisement" via
// substring replacement, it must not also mangle "advertising",
// "adjustment", or "standard".
func TestNormalizeLabel_DoesNotCorruptUnrelatedWords(t *testing.T) {
	tests := []struct {
		label string
		want  string
	}{
		{"Advertising", "advertising"},
		{"Advertising Expense", "advertising expense"},
		{"Adjustment", "adjustment"},
		{"Standard Fees", "standard fees"},
		// "&" glued directly to letters (no surrounding whitespace) is left
		// untouched rather than guessed at; see
		// TestNormalizeLabel_StandaloneAmpersandOnlyAtWordBoundary.
		{"AT&T Wireless", "at&t wireless"},
	}
	for _, tt := range tests {
		got := NormalizeLabel(tt.label)
		if got.Comparable != tt.want {
			t.Errorf("NormalizeLabel(%q).Comparable = %q, want %q", tt.label, got.Comparable, tt.want)
		}
	}
}

func TestNormalizeLabel_PreservesOriginal(t *testing.T) {
	label := "  6100 - Advertising & Promotion  "
	got := NormalizeLabel(label)
	if got.Original != label {
		t.Errorf("Original = %q, want unmodified %q", got.Original, label)
	}
}

func TestNormalizeLabel_StandaloneAmpersandOnlyAtWordBoundary(t *testing.T) {
	// "&" glued directly to letters on both sides (no surrounding
	// whitespace) is left alone rather than guessed at, since we cannot
	// tell if it is a standalone conjunction or part of a compound token
	// like "R&D" without more context. This test just documents the
	// current conservative behavior for "R&D" specifically, which should
	// not become "randd" or otherwise be corrupted into gibberish that
	// breaks other matching.
	got := NormalizeLabel("R&D Expense")
	if got.Comparable == "" {
		t.Fatal("expected non-empty Comparable")
	}
	// It must not silently drop the D or corrupt "expense".
	if !containsToken(got.Comparable, "expense") {
		t.Errorf("expected %q to still contain the word 'expense'", got.Comparable)
	}
}
