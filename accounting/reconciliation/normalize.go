package reconciliation

import "strings"

// NormalizeReference applies only the safe, caller-configurable
// transformations n declares, in a fixed order: trim, case-fold, remove
// punctuation, remove spaces, strip leading zeros. No fuzzy/NLP
// transformation is ever applied — task section 14. Exported so a caller
// (or a test) can reproduce exactly what evidence-generation used.
func NormalizeReference(ref string, n ReferenceNormalization) string {
	s := ref
	if n.Trim {
		s = strings.TrimSpace(s)
	}
	if n.CaseFold {
		s = strings.ToLower(s)
	}
	if n.RemovePunctuation {
		var b strings.Builder
		b.Grow(len(s))
		for _, r := range s {
			if isPunctuation(r) {
				continue
			}
			b.WriteRune(r)
		}
		s = b.String()
	}
	if n.RemoveSpaces {
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "\t", "")
	}
	if n.StripLeadingZeros {
		s = stripLeadingZeros(s)
	}
	return s
}

// isPunctuation reports whether r is one of a fixed, small set of ASCII
// punctuation characters commonly present in check/reference numbers
// (e.g. "CHK-0042" vs "CHK0042"). Deliberately narrow and explicit —
// never a Unicode-category-based classifier, to keep behavior fully
// predictable and testable.
func isPunctuation(r rune) bool {
	switch r {
	case '-', '_', '.', ',', '#', '/', '\\', ':', ';', '\'', '"', '(', ')', '[', ']':
		return true
	default:
		return false
	}
}

// stripLeadingZeros removes leading '0' characters, but never reduces a
// string to empty when it was originally all zeros (e.g. "000" -> "0",
// not "") — this preserves "the reference existed and was zero" as
// distinct from "the reference was empty."
func stripLeadingZeros(s string) string {
	i := 0
	for i < len(s)-1 && s[i] == '0' {
		i++
	}
	return s[i:]
}

// normalizedReference resolves an item's Reference under n, returning
// ("", false) if the raw Reference was empty (an empty reference never
// participates in reference-based matching, regardless of
// normalization).
func normalizedReference(raw string, n ReferenceNormalization) (string, bool) {
	if raw == "" {
		return "", false
	}
	norm := NormalizeReference(raw, n)
	if norm == "" {
		return "", false
	}
	return norm, true
}
