package inventory

import "sort"

// BucketDefinition is one caller-defined (or default) value-aging bucket
// boundary, expressed in days of age. Buckets partition [MinDays, MaxDays]
// ranges; the terminal bucket has HasMax == false (open-ended, e.g.
// "365+"). Mirrors ar.BucketDefinition/ap.BucketDefinition's identical
// shape and validation rules — this package's own independent copy, not a
// shared type, since AR/AP age "days past due" while this package ages
// "days since receipt/last movement," a different semantic axis entirely
// (task section 18: "unlike AR/AP, 'current' has no accounting meaning
// here").
type BucketDefinition struct {
	Code    string `json:"code"`
	Label   string `json:"label,omitempty"`
	MinDays int    `json:"min_days"`
	MaxDays int    `json:"max_days,omitempty"`
	HasMax  bool   `json:"has_max"`
}

// UnknownAgeBucketCode is the fixed, always-present bucket code for
// inventory lacking sufficient age evidence — task sections 18/50: "the
// UNKNOWN bucket must remain explicit," never folded into the youngest or
// oldest bucket.
const UnknownAgeBucketCode = "UNKNOWN"

// DefaultBuckets returns this package's default value-aging schema: 0-30,
// 31-60, 61-90, 91-180, 181-365, 365+ (task section 18's suggested
// default). UnknownAgeBucketCode is implicit and never appears in this
// slice — it is reported separately (see AgingSummary.UnknownAgeValue)
// rather than as a bucket among these.
func DefaultBuckets() []BucketDefinition {
	return []BucketDefinition{
		{Code: "0_30", Label: "0-30 Days", MinDays: 0, MaxDays: 30, HasMax: true},
		{Code: "31_60", Label: "31-60 Days", MinDays: 31, MaxDays: 60, HasMax: true},
		{Code: "61_90", Label: "61-90 Days", MinDays: 61, MaxDays: 90, HasMax: true},
		{Code: "91_180", Label: "91-180 Days", MinDays: 91, MaxDays: 180, HasMax: true},
		{Code: "181_365", Label: "181-365 Days", MinDays: 181, MaxDays: 365, HasMax: true},
		{Code: "365_PLUS", Label: "365+ Days", MinDays: 366, HasMax: false},
	}
}

// resolvedBuckets returns defs if non-empty, otherwise DefaultBuckets().
func resolvedBuckets(defs []BucketDefinition) []BucketDefinition {
	if len(defs) > 0 {
		return defs
	}
	return DefaultBuckets()
}

// sortedBucketsByMin returns a copy of defs sorted by MinDays ascending.
func sortedBucketsByMin(defs []BucketDefinition) []BucketDefinition {
	out := make([]BucketDefinition, len(defs))
	copy(out, defs)
	sort.SliceStable(out, func(i, j int) bool { return out[i].MinDays < out[j].MinDays })
	return out
}

// validateBucketDefinitions checks defs for the structural rules this
// package requires: no overlap, exactly one terminal open-ended bucket
// (which must be the last by MinDays), unique non-empty codes distinct
// from UnknownAgeBucketCode, and Min <= Max for bounded buckets. Gaps are
// always allowed (unlike ar/ap's due-date aging, a caller's custom
// value-aging schema may legitimately skip a range) — anything not
// covered by a caller-defined bucket set is simply reported as part of
// UnknownAgeValue's data-quality signal instead of forcing a rigid
// partition of every possible day count.
func validateBucketDefinitions(defs []BucketDefinition) []Issue {
	var issues []Issue
	if len(defs) == 0 {
		return []Issue{{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "aging bucket definitions must not be empty"}}
	}

	seenCodes := map[string]bool{}
	for _, d := range defs {
		if d.Code == "" {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "aging bucket missing code"})
			continue
		}
		if d.Code == UnknownAgeBucketCode {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "aging bucket code collides with reserved UNKNOWN bucket", BucketCode: d.Code})
		}
		if seenCodes[d.Code] {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "duplicate aging bucket code: " + d.Code, BucketCode: d.Code})
		}
		seenCodes[d.Code] = true
		if d.MinDays < 0 {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "aging bucket has negative MinDays", BucketCode: d.Code})
		}
		if d.HasMax && d.MaxDays < d.MinDays {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "aging bucket MaxDays is less than MinDays", BucketCode: d.Code})
		}
	}
	if len(issues) > 0 {
		return issues
	}

	sorted := sortedBucketsByMin(defs)
	terminalCount := 0
	for _, d := range sorted {
		if !d.HasMax {
			terminalCount++
		}
	}
	if terminalCount == 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "aging bucket definitions have no terminal open-ended bucket"})
	} else if terminalCount > 1 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "aging bucket definitions have more than one terminal open-ended bucket"})
	}
	for i, d := range sorted {
		if !d.HasMax && i != len(sorted)-1 {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "open-ended aging bucket is not the last bucket by MinDays", BucketCode: d.Code})
		}
	}
	for i := 1; i < len(sorted); i++ {
		prev, cur := sorted[i-1], sorted[i]
		if !prev.HasMax {
			continue
		}
		if cur.MinDays <= prev.MaxDays {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "aging buckets overlap: " + prev.Code + " and " + cur.Code, BucketCode: cur.Code})
		}
	}
	return issues
}

// bucketForDays returns the Code of the bucket in sorted (already sorted
// by sortedBucketsByMin) that ageDays falls into, and true if found.
// Returns ("", false) if defs is empty or ageDays falls into an
// unresolvable gap (a legal configuration for this package — see
// validateBucketDefinitions's doc comment — the caller (aging.go) reports
// that as UnknownAgeBucketCode).
func bucketForDays(sorted []BucketDefinition, ageDays int) (string, bool) {
	for _, d := range sorted {
		if ageDays < d.MinDays {
			continue
		}
		if !d.HasMax || ageDays <= d.MaxDays {
			return d.Code, true
		}
	}
	return "", false
}

// bucketDefByCode returns a pointer to the BucketDefinition with the given
// code within defs, or nil if not found.
func bucketDefByCode(defs []BucketDefinition, code string) *BucketDefinition {
	for i := range defs {
		if defs[i].Code == code {
			return &defs[i]
		}
	}
	return nil
}
