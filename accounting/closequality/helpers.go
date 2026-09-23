package closequality

import (
	"sort"
	"strings"
)

// normalizeSpaceLower is the one normalization step ExpectedPeriodEntry
// description matching applies: trim + collapse-internal-whitespace +
// case-fold. Deliberately not fuzzy — no edit distance, no tokenization,
// no stemming. See ExpectedPeriodEntry's doc comment.
func normalizeSpaceLower(s string) string {
	fields := strings.Fields(s)
	return strings.ToLower(strings.Join(fields, " "))
}

// sortedStrings returns a sorted copy of ss, never mutating ss.
func sortedStrings(ss []string) []string {
	if len(ss) == 0 {
		return nil
	}
	out := make([]string, len(ss))
	copy(out, ss)
	sort.Strings(out)
	return out
}

// uniqueSortedStrings returns a deduplicated, sorted copy of ss.
func uniqueSortedStrings(ss []string) []string {
	if len(ss) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
