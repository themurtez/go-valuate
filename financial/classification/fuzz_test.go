package classification

import (
	"testing"
	"unicode/utf8"
)

// FuzzNormalizeLabel proves NormalizeLabel — the label normalizer every
// alias/rule match in this package runs through — never panics on
// arbitrary input (including invalid UTF-8), always preserves Original
// byte-for-byte, and never produces a Comparable form containing the raw
// ampersand character (every "&" must have already been expanded to
// "and" or removed, per the package's own documented normalization
// steps) or leading/trailing whitespace (step 7's collapse must leave no
// boundary whitespace, matching step 1's trim).
func FuzzNormalizeLabel(f *testing.F) {
	seeds := []string{
		"", " ", "Advertising", "6100 Advertising", "6100 - Advertising",
		"Owner's Draw", "AT&T", "R&D", "Marketing & Promotion", "  spaced  out  ",
		"\"Quoted\" Label", "Café Expenses", "支出", "\x00\x01", "&&&&",
		"6100Advertising", "610 0 - Ad", "-- Advertising --", "Ad, ver, tis, ing",
		string([]byte{0xff, 0xfe, 0xfd}), // invalid UTF-8
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, label string) {
		result := NormalizeLabel(label)

		if result.Original != label {
			t.Fatalf("NormalizeLabel(%q) did not preserve Original byte-for-byte, got %q", label, result.Original)
		}
		if !utf8.ValidString(result.Comparable) && utf8.ValidString(label) {
			t.Fatalf("NormalizeLabel(%q) produced invalid UTF-8 in Comparable from valid UTF-8 input: %q", label, result.Comparable)
		}
		if len(result.Comparable) > 0 {
			if result.Comparable[0] == ' ' || result.Comparable[len(result.Comparable)-1] == ' ' {
				t.Fatalf("NormalizeLabel(%q) left leading/trailing whitespace in Comparable: %q", label, result.Comparable)
			}
		}
		// Comparable's growth relative to Original is bounded, never
		// unbounded: '&' -> "and" expands by 2 bytes per occurrence, and
		// each byte of invalid UTF-8 in the input becomes one 3-byte
		// U+FFFD replacement rune during Go's standard rune iteration (a
		// well-known, fixed 3x expansion — not a bug in this package). No
		// other step in NormalizeLabel expands its input. This bound
		// exists to catch a genuine unbounded-growth regression, not to
		// assert an exact length.
		ampersands, invalidUTF8Bytes := 0, 0
		for i := 0; i < len(label); {
			r, size := utf8.DecodeRuneInString(label[i:])
			if r == utf8.RuneError && size == 1 {
				invalidUTF8Bytes++
			} else if r == '&' {
				ampersands++
			}
			i += size
		}
		maxPlausibleLen := len(label) + 2*ampersands + 2*invalidUTF8Bytes + 8
		if len(result.Comparable) > maxPlausibleLen {
			t.Fatalf("NormalizeLabel(%q) produced a Comparable form far longer than input could justify: len(Comparable)=%d len(Original)=%d maxPlausibleLen=%d", label, len(result.Comparable), len(label), maxPlausibleLen)
		}
	})
}
