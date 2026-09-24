package reconciliation

// daysOutstanding computes the age in days of dateStr relative to
// asOfDate, returning (0, false) if either date fails to parse or dateStr
// is after asOfDate (an item dated in the future relative to AsOfDate has
// no meaningful "days outstanding" — this package never reports a
// negative age).
func daysOutstanding(dateStr, asOfDate string) (int, bool) {
	d, ok1 := parseDate(dateStr)
	asOf, ok2 := parseDate(asOfDate)
	if !ok1 || !ok2 {
		return 0, false
	}
	days := int(asOf.Sub(d).Hours() / 24)
	if days < 0 {
		return 0, false
	}
	return days, true
}

// AgingBucket is one caller-configured aging band for unmatched-item
// aging (task section 52: "Optional caller-configured aging buckets" —
// this is generic reconciliation aging, distinct from ar/ap's own
// domain-specific aging).
type AgingBucket struct {
	Label   string `json:"label"`
	MinDays int    `json:"min_days"`
	MaxDays int    `json:"max_days,omitempty"` // 0 means unbounded (open-ended final bucket)
}

// AgedItem is one unmatched or reconciling item's computed age as of
// Input.AsOfDate.
type AgedItem struct {
	ItemID          string `json:"item_id"`
	DaysOutstanding int    `json:"days_outstanding"`
	Bucket          string `json:"bucket,omitempty"`
}

func bucketFor(days int, buckets []AgingBucket) string {
	for _, b := range buckets {
		if days < b.MinDays {
			continue
		}
		if b.MaxDays > 0 && days > b.MaxDays {
			continue
		}
		return b.Label
	}
	return ""
}

// staleFindings produces FindingStaleUnmatchedItem/FindingStaleReconcilingItem
// for items whose DaysOutstanding meets or exceeds
// policy.StaleDaysThreshold — task section 53. A zero threshold means
// staleness is never assessed (no universal default is invented).
func staleUnmatchedFindings(agedBook, agedExternal []AgedItem, threshold int) []Finding {
	if threshold <= 0 {
		return nil
	}
	var findings []Finding
	for _, a := range agedBook {
		if a.DaysOutstanding >= threshold {
			findings = append(findings, Finding{
				Code: FindingStaleUnmatchedItem, Severity: SeverityWarning,
				Message:         "unmatched book item has been outstanding at or beyond the stale-days threshold",
				ItemIDs:         []string{a.ItemID},
				DaysOutstanding: a.DaysOutstanding,
			})
		}
	}
	for _, a := range agedExternal {
		if a.DaysOutstanding >= threshold {
			findings = append(findings, Finding{
				Code: FindingStaleUnmatchedItem, Severity: SeverityWarning,
				Message:         "unmatched external item has been outstanding at or beyond the stale-days threshold",
				ItemIDs:         []string{a.ItemID},
				DaysOutstanding: a.DaysOutstanding,
			})
		}
	}
	return findings
}

func staleReconcilingFindings(agedReconciling []AgedItem, threshold int) []Finding {
	if threshold <= 0 {
		return nil
	}
	var findings []Finding
	for _, a := range agedReconciling {
		if a.DaysOutstanding >= threshold {
			findings = append(findings, Finding{
				Code: FindingStaleReconcilingItem, Severity: SeverityWarning,
				Message:         "reconciling item has been outstanding at or beyond the stale-days threshold",
				ItemIDs:         []string{a.ItemID},
				DaysOutstanding: a.DaysOutstanding,
			})
		}
	}
	return findings
}
