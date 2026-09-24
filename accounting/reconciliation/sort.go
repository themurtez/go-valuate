package reconciliation

import (
	"fmt"
	"sort"
)

// Deterministic output ordering (task section 65):
//
//	confirmed matches, then auto matches by precedence/date/BookItemID,
//	ambiguous groups by source ID, unmatched items by date/ID, findings
//	by severity/code/source, issues by code/source.
//
// No slice here is ever ordered by ranging a Go map — every sort key
// below is drawn from already-materialized struct fields.

var matchReasonRank = map[MatchReason]int{
	ReasonConfirmed:                0,
	ReasonExactReferenceAmountDate: 1,
	ReasonExactReferenceAmount:     2,
	ReasonExactAmountDate:          3,
	ReasonExactAmountWithinWindow:  4,
	ReasonCompositeSumMatch:        5,
}

// sortMatchGroups orders groups per task section 65: confirmed first,
// then by matching-precedence rank, then by DateDifferenceDays, then by
// the group's first BookItemID (or, if it has none, first
// ExternalItemID) for a fully deterministic tiebreak.
func sortMatchGroups(groups []MatchGroup) {
	sort.SliceStable(groups, func(i, j int) bool {
		gi, gj := groups[i], groups[j]
		ri, rj := matchReasonRank[gi.MatchReason], matchReasonRank[gj.MatchReason]
		if ri != rj {
			return ri < rj
		}
		if gi.DateDifferenceDays != gj.DateDifferenceDays {
			return gi.DateDifferenceDays < gj.DateDifferenceDays
		}
		return groupKey(gi) < groupKey(gj)
	})
}

func groupKey(g MatchGroup) string {
	if len(g.BookItemIDs) > 0 {
		return g.BookItemIDs[0]
	}
	if len(g.ExternalItemIDs) > 0 {
		return g.ExternalItemIDs[0]
	}
	return ""
}

// assignMatchIDs assigns a stable "M" + zero-padded-ordinal MatchID to
// every group in groups' current (already-sorted) order, skipping groups
// that already carry a caller-supplied MatchID (from a ConfirmedMatch —
// see confirmed.go).
func assignMatchIDs(groups []MatchGroup) {
	for i := range groups {
		if groups[i].MatchID == "" {
			groups[i].MatchID = matchIDFor(i)
		}
	}
}

func matchIDFor(i int) string {
	return fmt.Sprintf("M%03d", i+1)
}

// sortByID sorts a []string of item IDs lexically, for unmatched-item ID
// lists.
func sortByID(ids []string) {
	sort.Strings(ids)
}

// sortAgedItems orders AgedItem slices by DaysOutstanding descending
// (most stale first), then by ItemID for a deterministic tiebreak.
func sortAgedItems(items []AgedItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].DaysOutstanding != items[j].DaysOutstanding {
			return items[i].DaysOutstanding > items[j].DaysOutstanding
		}
		return items[i].ItemID < items[j].ItemID
	})
}

var severityRank = map[Severity]int{
	SeverityWarning: 0,
	SeverityInfo:    1,
}

// sortFindings orders findings by severity, then FindingCode, then the
// first ItemID — task section 65.
func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		fi, fj := findings[i], findings[j]
		si, sj := severityRank[fi.Severity], severityRank[fj.Severity]
		if si != sj {
			return si < sj
		}
		if fi.Code != fj.Code {
			return fi.Code < fj.Code
		}
		return findingKey(fi) < findingKey(fj)
	})
}

func findingKey(f Finding) string {
	if len(f.ItemIDs) > 0 {
		return f.ItemIDs[0]
	}
	return ""
}

var issueSeverityRank = map[IssueSeverity]int{
	IssueSeverityError:   0,
	IssueSeverityWarning: 1,
}

// sortIssues orders issues by severity, then IssueCode, then ItemID —
// task section 65.
func sortIssues(issues []Issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		ii, ij := issues[i], issues[j]
		si, sj := issueSeverityRank[ii.Severity], issueSeverityRank[ij.Severity]
		if si != sj {
			return si < sj
		}
		if ii.Code != ij.Code {
			return ii.Code < ij.Code
		}
		return ii.ItemID < ij.ItemID
	})
}
