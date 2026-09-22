package classification

import (
	"regexp"
	"strings"
)

// NormalizedLabel is the result of normalizing a raw account label for
// matching purposes. It preserves the original label separately so callers
// and rules that need the untouched source text (e.g. for a Reason message)
// never lose it.
type NormalizedLabel struct {
	// Original is the label exactly as supplied, unmodified.
	Original string
	// Comparable is the normalized form used for alias and rule matching:
	// lowercase, whitespace-collapsed, punctuation-normalized, with "&"
	// expanded to "and" and a leading account number stripped, if present.
	Comparable string
}

// leadingAccountNumber matches a leading account-number prefix such as
// "6100 ", "6100 - ", "6100-", or "6100: " at the start of a label, so it can
// be stripped before comparison. It requires at least two digits so short
// numeric tokens that are actually part of a real label (unlikely, but
// possible) are less likely to be mistaken for an account number, and it
// requires the digits to be followed by whitespace or a "-"/":" separator
// (itself optionally followed by more whitespace), never directly by a
// letter or another digit, so a label like "24hr Support" is never touched.
var leadingAccountNumber = regexp.MustCompile(`^[0-9]{2,}(\s+|\s*[-:]\s*)`)

// repeatedWhitespace matches any run of one or more whitespace characters,
// collapsed to a single space during normalization.
var repeatedWhitespace = regexp.MustCompile(`\s+`)

// standaloneAmpersand matches a "&" that is its own token (surrounded by
// whitespace or string boundaries), so it can be safely expanded to "and"
// without touching a "&" that is part of a larger token (e.g. "AT&T").
var standaloneAmpersand = regexp.MustCompile(`(^|\s)&(\s|$)`)

// NormalizeLabel produces a NormalizedLabel from a raw account label. It is
// deliberately conservative: every transformation operates on whitespace
// boundaries or well-defined token positions, never on raw substrings, so
// that a short token never corrupts a longer word it happens to appear
// inside of (e.g. normalizing "&" must never touch "advertising" or
// "standard").
//
// Normalization steps, in order:
//  1. trim leading/trailing whitespace
//  2. strip a leading account-number prefix, if present ("6100 Advertising"
//     or "6100 - Advertising" both become "Advertising")
//  3. lowercase
//  4. expand a standalone "&" to "and"
//  5. normalize common punctuation variants (curly quotes, em/en dashes) to
//     plain ASCII equivalents
//  6. remove punctuation that carries no comparison meaning (commas,
//     periods, parentheses, colons, semicolons, quotes), replacing each with
//     a space so words never get glued together
//  7. collapse repeated whitespace to a single space
//
// The Original field always retains the untouched input.
func NormalizeLabel(label string) NormalizedLabel {
	original := label

	trimmed := strings.TrimSpace(label)
	trimmed = leadingAccountNumber.ReplaceAllString(trimmed, "")
	trimmed = strings.TrimSpace(trimmed)

	lower := strings.ToLower(trimmed)

	lower = standaloneAmpersand.ReplaceAllString(lower, "${1}and${2}")

	lower = normalizePunctuation(lower)

	lower = repeatedWhitespace.ReplaceAllString(lower, " ")
	lower = strings.TrimSpace(lower)

	return NormalizedLabel{
		Original:   original,
		Comparable: lower,
	}
}

// punctuationReplacer normalizes punctuation variants that commonly appear
// in exported financial statements. Quote/apostrophe characters (including
// curly variants) are dropped entirely (mapped to "") so a possessive like
// "Owner's" becomes "owners" rather than "owner s" — gluing the two
// fragments the apostrophe used to separate back together. Every other
// punctuation mark carries no comparison meaning and is mapped to a space
// instead, so unrelated words never get glued together. Every substitution
// is a whole-character replacement, never a substring/word replacement, so
// no letter sequence is ever at risk of corruption.
var punctuationReplacer = strings.NewReplacer(
	"‘", "", // left single quote / apostrophe
	"’", "", // right single quote / apostrophe
	"“", "", // left double quote
	"”", "", // right double quote
	"'", "",
	"\"", "",
	"–", "-", // en dash
	"—", "-", // em dash
	",", " ",
	".", " ",
	"(", " ",
	")", " ",
	":", " ",
	";", " ",
	"_", " ",
	"/", " ",
	"-", " ",
)

func normalizePunctuation(s string) string {
	return punctuationReplacer.Replace(s)
}
