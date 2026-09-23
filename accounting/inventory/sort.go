package inventory

import "sort"

// sortStrings sorts s ascending in place — a thin, named wrapper around
// sort.Strings used at every "sort map keys before floating-point
// accumulation" call site in this package (task section 70), so each
// call site reads as an explicit determinism step rather than a bare
// standard-library call easy to overlook during review.
func sortStrings(s []string) {
	sort.Strings(s)
}
