package ar

import "sort"

// AgingBasis controls which date DaysPastDue is measured from. Default is
// AgingByDueDate — see the package doc comment's "aging basis" section for
// why this, not invoice date, is the default: due-date aging measures
// actual delinquency relative to the contractual payment obligation,
// while invoice-date aging conflates long payment terms with lateness.
type AgingBasis string

const (
	// AgingByDueDate ages AsOfDate - DueDate. Default.
	AgingByDueDate AgingBasis = "AGING_BY_DUE_DATE"
	// AgingByInvoiceDate ages AsOfDate - InvoiceDate, for a caller who
	// specifically wants invoice-age rather than delinquency.
	AgingByInvoiceDate AgingBasis = "AGING_BY_INVOICE_DATE"
)

// resolvedAgingBasis returns b if recognized, otherwise AgingByDueDate.
func resolvedAgingBasis(b AgingBasis) AgingBasis {
	if b == AgingByInvoiceDate {
		return b
	}
	return AgingByDueDate
}

// BucketDefinition is one caller-defined (or default) aging bucket
// boundary, expressed in days past due. Buckets partition
// [MinDaysPastDue, MaxDaysPastDue] ranges; the terminal bucket has
// HasMax == false (open-ended, e.g. "91+").
type BucketDefinition struct {
	// Code is a stable identifier for this bucket (e.g. "CURRENT",
	// "1_30"), used in JSON output and flag/issue references. Not a UI
	// label — see the package doc's "do not hard-code UI labels as
	// semantic identifiers" instruction. Required, unique within a
	// BucketDefinition slice.
	Code string `json:"code"`
	// Label is an optional human-readable display label (e.g. "1-30 Days
	// Past Due"). Purely cosmetic; never used for matching/comparison.
	Label string `json:"label,omitempty"`
	// MinDaysPastDue is this bucket's inclusive lower bound in days past
	// due. 0 for the CURRENT bucket (not-yet-due or due today).
	MinDaysPastDue int `json:"min_days_past_due"`
	// MaxDaysPastDue is this bucket's inclusive upper bound in days past
	// due. Ignored (open-ended) when HasMax is false.
	MaxDaysPastDue int `json:"max_days_past_due,omitempty"`
	// HasMax is false for exactly one bucket in a valid BucketDefinition
	// slice: the terminal, open-ended bucket (e.g. "91+"). See
	// validateBucketDefinitions.
	HasMax bool `json:"has_max"`
}

// DefaultBuckets returns this package's default five-bucket aging schema:
// CURRENT, 1-30, 31-60, 61-90, 91+ — the buckets the task specifies as the
// default.
func DefaultBuckets() []BucketDefinition {
	return []BucketDefinition{
		{Code: "CURRENT", Label: "Current", MinDaysPastDue: 0, MaxDaysPastDue: 0, HasMax: true},
		{Code: "1_30", Label: "1-30 Days Past Due", MinDaysPastDue: 1, MaxDaysPastDue: 30, HasMax: true},
		{Code: "31_60", Label: "31-60 Days Past Due", MinDaysPastDue: 31, MaxDaysPastDue: 60, HasMax: true},
		{Code: "61_90", Label: "61-90 Days Past Due", MinDaysPastDue: 61, MaxDaysPastDue: 90, HasMax: true},
		{Code: "91_PLUS", Label: "91+ Days Past Due", MinDaysPastDue: 91, HasMax: false},
	}
}

// resolvedBuckets returns defs if non-empty, otherwise DefaultBuckets().
func resolvedBuckets(defs []BucketDefinition) []BucketDefinition {
	if len(defs) > 0 {
		return defs
	}
	return DefaultBuckets()
}

// sortedBucketsByMin returns a copy of defs sorted by MinDaysPastDue
// ascending — the deterministic evaluation/output order every aging
// computation and CustomerSummary bucket breakdown uses, independent of
// caller input order.
func sortedBucketsByMin(defs []BucketDefinition) []BucketDefinition {
	out := make([]BucketDefinition, len(defs))
	copy(out, defs)
	sort.SliceStable(out, func(i, j int) bool { return out[i].MinDaysPastDue < out[j].MinDaysPastDue })
	return out
}

// validateBucketDefinitions checks defs for the structural rules the task
// requires: no gaps (unless AllowBucketGaps), no overlap, exactly one
// terminal open-ended bucket, unique codes, non-negative bounds, and
// Min <= Max for bounded buckets. Returns issues; an empty slice means
// defs is structurally valid.
func validateBucketDefinitions(defs []BucketDefinition, allowGaps bool) []Issue {
	var issues []Issue
	if len(defs) == 0 {
		return []Issue{{Code: IssueInvalidBucketConfiguration, Severity: SeverityError, Message: "bucket definitions must not be empty"}}
	}

	seenCodes := map[string]bool{}
	for _, d := range defs {
		if d.Code == "" {
			issues = append(issues, Issue{Code: IssueInvalidBucketConfiguration, Severity: SeverityError, Message: "bucket definition missing code"})
			continue
		}
		if seenCodes[d.Code] {
			issues = append(issues, Issue{Code: IssueInvalidBucketConfiguration, Severity: SeverityError, Message: "duplicate bucket code: " + d.Code, BucketCode: d.Code})
		}
		seenCodes[d.Code] = true
		if d.MinDaysPastDue < 0 {
			issues = append(issues, Issue{Code: IssueInvalidBucketConfiguration, Severity: SeverityError, Message: "bucket has negative MinDaysPastDue", BucketCode: d.Code})
		}
		if d.HasMax && d.MaxDaysPastDue < d.MinDaysPastDue {
			issues = append(issues, Issue{Code: IssueInvalidBucketConfiguration, Severity: SeverityError, Message: "bucket MaxDaysPastDue is less than MinDaysPastDue", BucketCode: d.Code})
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
		issues = append(issues, Issue{Code: IssueInvalidBucketConfiguration, Severity: SeverityError, Message: "bucket definitions have no terminal open-ended bucket"})
	} else if terminalCount > 1 {
		issues = append(issues, Issue{Code: IssueInvalidBucketConfiguration, Severity: SeverityError, Message: "bucket definitions have more than one terminal open-ended bucket"})
	}
	// Only the last (highest MinDaysPastDue) bucket may be open-ended.
	for i, d := range sorted {
		if !d.HasMax && i != len(sorted)-1 {
			issues = append(issues, Issue{Code: IssueInvalidBucketConfiguration, Severity: SeverityError, Message: "open-ended bucket is not the last bucket by MinDaysPastDue", BucketCode: d.Code})
		}
	}

	for i := 1; i < len(sorted); i++ {
		prev, cur := sorted[i-1], sorted[i]
		if !prev.HasMax {
			continue // already flagged above as misplaced; skip range checks against it.
		}
		switch {
		case cur.MinDaysPastDue <= prev.MaxDaysPastDue:
			issues = append(issues, Issue{Code: IssueInvalidBucketConfiguration, Severity: SeverityError, Message: "buckets overlap: " + prev.Code + " and " + cur.Code, BucketCode: cur.Code})
		case cur.MinDaysPastDue > prev.MaxDaysPastDue+1 && !allowGaps:
			issues = append(issues, Issue{Code: IssueInvalidBucketConfiguration, Severity: SeverityError, Message: "gap between buckets: " + prev.Code + " and " + cur.Code, BucketCode: cur.Code})
		}
	}

	return issues
}

// bucketForDays returns the Code of the bucket in sorted (already sorted
// by sortedBucketsByMin) that daysPastDue falls into, and true if found. A
// negative daysPastDue (future-dated item, clamped to 0 by the caller
// before this is invoked — see agingForReceivable) always matches the
// first bucket. Returns ("", false) only if defs is empty or daysPastDue
// falls in an unresolvable gap (AllowBucketGaps was used and no bucket
// covers this value).
func bucketForDays(sorted []BucketDefinition, daysPastDue int) (string, bool) {
	for _, d := range sorted {
		if daysPastDue < d.MinDaysPastDue {
			continue
		}
		if !d.HasMax || daysPastDue <= d.MaxDaysPastDue {
			return d.Code, true
		}
	}
	return "", false
}
