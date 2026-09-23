package management

import (
	"sort"

	"github.com/themurtez/go-valuate/analytics/anomalies"
	"github.com/themurtez/go-valuate/analytics/covenants"
	"github.com/themurtez/go-valuate/analytics/debt"
	"github.com/themurtez/go-valuate/analytics/qoe"
)

// severityRank orders TopIssueSeverity for sorting: critical first, then
// warning, then info.
func severityRank(s TopIssueSeverity) int {
	switch s {
	case TopIssueSeverityCritical:
		return 0
	case TopIssueSeverityWarning:
		return 1
	default:
		return 2
	}
}

// sourceRank orders TopIssueSource for sorting, matching TopIssueSource's
// own declaration order (anomalies, qoe, debt, covenants).
func sourceRank(s TopIssueSource) int {
	switch s {
	case TopIssueSourceAnomalies:
		return 0
	case TopIssueSourceQoE:
		return 1
	case TopIssueSourceDebt:
		return 2
	case TopIssueSourceCovenants:
		return 3
	default:
		return 4
	}
}

// buildTopIssues merges already-detected findings from in.Anomalies,
// in.QoE.Flags, in.Debt.Flags, and breached/near-breached
// in.Covenants.Tests entries onto this section's own TopIssueSeverity
// scale — it detects nothing of its own; every TopIssue traces back to a
// specific sibling finding.
func buildTopIssues(in Input) TopIssues {
	var out []TopIssue

	if in.Anomalies.Available {
		for _, a := range in.Anomalies.Anomalies {
			out = append(out, TopIssue{
				Source:   TopIssueSourceAnomalies,
				Code:     string(a.Code),
				Severity: anomalySeverityToTopIssue(a.Severity),
				Period:   a.Period,
				Message:  a.Explanation,
			})
		}
	}

	if in.QoE.Available {
		for _, f := range in.QoE.Flags {
			out = append(out, TopIssue{
				Source:   TopIssueSourceQoE,
				Code:     string(f.Code),
				Severity: qoeFlagSeverityToTopIssue(f.Severity),
				Period:   f.Period,
				Message:  f.Message,
			})
		}
	}

	if in.Debt.Available {
		for _, f := range in.Debt.Flags {
			out = append(out, TopIssue{
				Source:   TopIssueSourceDebt,
				Code:     string(f.Code),
				Severity: debtFlagSeverityToTopIssue(f.Severity),
				Message:  f.Message,
			})
		}
	}

	if in.Covenants.Available {
		for _, t := range in.Covenants.Tests {
			if t.Status != covenants.StatusFail && t.WarningBufferStatus != covenants.WarningBufferWithinBuffer {
				continue
			}
			severity := TopIssueSeverityWarning
			if t.Status == covenants.StatusFail {
				severity = TopIssueSeverityCritical
			}
			out = append(out, TopIssue{
				Source:   TopIssueSourceCovenants,
				Code:     t.CovenantID,
				Severity: severity,
				Period:   t.Period,
				Message:  t.Explanation,
			})
		}
	}

	if len(out) == 0 {
		return TopIssues{Available: in.Anomalies.Available || in.QoE.Available || in.Debt.Available || in.Covenants.Available}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if severityRank(out[i].Severity) != severityRank(out[j].Severity) {
			return severityRank(out[i].Severity) < severityRank(out[j].Severity)
		}
		return sourceRank(out[i].Source) < sourceRank(out[j].Source)
	})

	summary := TopIssuesSummary{
		Total:             len(out),
		ReviewRecommended: true,
		BySeverity:        map[TopIssueSeverity]int{},
		BySource:          map[TopIssueSource]int{},
	}
	for _, issue := range out {
		summary.BySeverity[issue.Severity]++
		summary.BySource[issue.Source]++
	}

	return TopIssues{
		Available: true,
		Issues:    out,
		Summary:   summary,
	}
}

// anomalySeverityToTopIssue translates analytics/anomalies.AnomalySeverity
// onto TopIssueSeverity — both packages use an identical
// info/warning/critical scale, so this is a direct, lossless mapping.
func anomalySeverityToTopIssue(s anomalies.AnomalySeverity) TopIssueSeverity {
	switch s {
	case anomalies.AnomalySeverityCritical:
		return TopIssueSeverityCritical
	case anomalies.AnomalySeverityWarning:
		return TopIssueSeverityWarning
	default:
		return TopIssueSeverityInfo
	}
}

// qoeFlagSeverityToTopIssue translates analytics/qoe.FlagSeverity onto
// TopIssueSeverity — both packages use an identical info/warning/critical
// scale, so this is a direct, lossless mapping.
func qoeFlagSeverityToTopIssue(s qoe.FlagSeverity) TopIssueSeverity {
	switch s {
	case qoe.FlagSeverityCritical:
		return TopIssueSeverityCritical
	case qoe.FlagSeverityWarning:
		return TopIssueSeverityWarning
	default:
		return TopIssueSeverityInfo
	}
}

// debtFlagSeverityToTopIssue translates analytics/debt.FlagSeverity onto
// TopIssueSeverity — both packages use an identical info/warning/critical
// scale, so this is a direct, lossless mapping.
func debtFlagSeverityToTopIssue(s debt.FlagSeverity) TopIssueSeverity {
	switch s {
	case debt.FlagSeverityCritical:
		return TopIssueSeverityCritical
	case debt.FlagSeverityWarning:
		return TopIssueSeverityWarning
	default:
		return TopIssueSeverityInfo
	}
}
