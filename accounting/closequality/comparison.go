package closequality

// Trend summarizes a concrete current-vs-prior change direction. It is
// never a numeric score — only IMPROVING/STABLE/DETERIORATING/
// UNAVAILABLE, derived strictly from countable blocker/warning deltas
// (see ComparisonResult.deriveTrend). If that becomes ambiguous (equal
// counts of new and resolved issues at different severities), Trend is
// STABLE rather than a judgment call, and the caller can always inspect
// the explicit delta lists directly instead.
type Trend string

const (
	TrendImproving     Trend = "IMPROVING"
	TrendStable        Trend = "STABLE"
	TrendDeteriorating Trend = "DETERIORATING"
	TrendUnavailable   Trend = "UNAVAILABLE"
)

// ComparisonResult is a pure comparison against a prior Result — it
// never changes current-period calculations, only reports deltas.
type ComparisonResult struct {
	PriorStatus Status `json:"prior_status"`

	NewBlockers      []Finding `json:"new_blockers,omitempty"`
	ResolvedBlockers []Finding `json:"resolved_blockers,omitempty"`
	NewWarnings      []Finding `json:"new_warnings,omitempty"`
	ResolvedWarnings []Finding `json:"resolved_warnings,omitempty"`

	DimensionStatusChanges []DimensionStatusChange `json:"dimension_status_changes,omitempty"`
	CoverageChange         float64                 `json:"coverage_change"`

	Trend Trend `json:"trend"`
}

// DimensionStatusChange reports one dimension's status change between
// the prior and current Result.
type DimensionStatusChange struct {
	Dimension     Dimension       `json:"dimension"`
	PriorStatus   DimensionStatus `json:"prior_status"`
	CurrentStatus DimensionStatus `json:"current_status"`
}

func compareToPrior(current Result, prior Result) ComparisonResult {
	cr := ComparisonResult{PriorStatus: prior.Status}

	cr.NewBlockers = findingsNotIn(current.Blockers, prior.Blockers)
	cr.ResolvedBlockers = findingsNotIn(prior.Blockers, current.Blockers)
	cr.NewWarnings = findingsNotIn(current.Warnings, prior.Warnings)
	cr.ResolvedWarnings = findingsNotIn(prior.Warnings, current.Warnings)

	priorByDim := make(map[Dimension]DimensionStatus, len(prior.Dimensions))
	for _, d := range prior.Dimensions {
		priorByDim[d.Dimension] = d.Status
	}
	for _, d := range current.Dimensions {
		if ps, ok := priorByDim[d.Dimension]; ok && ps != d.Status {
			cr.DimensionStatusChanges = append(cr.DimensionStatusChanges, DimensionStatusChange{
				Dimension:     d.Dimension,
				PriorStatus:   ps,
				CurrentStatus: d.Status,
			})
		}
	}

	cr.CoverageChange = current.DimensionCoverage.CoveragePercent - prior.DimensionCoverage.CoveragePercent
	cr.Trend = deriveTrend(cr)
	return cr
}

// findingsNotIn returns the findings in a whose dedupKey does not appear
// in b, preserving a's order.
func findingsNotIn(a, b []Finding) []Finding {
	present := make(map[dedupKey]bool, len(b))
	for _, f := range b {
		present[keyFor(f)] = true
	}
	var out []Finding
	for _, f := range a {
		if !present[keyFor(f)] {
			out = append(out, f)
		}
	}
	return out
}

func deriveTrend(cr ComparisonResult) Trend {
	newCount := len(cr.NewBlockers) + len(cr.NewWarnings)
	resolvedCount := len(cr.ResolvedBlockers) + len(cr.ResolvedWarnings)
	switch {
	case newCount == 0 && resolvedCount == 0:
		return TrendStable
	case len(cr.NewBlockers) > 0 && len(cr.ResolvedBlockers) == 0:
		return TrendDeteriorating
	case resolvedCount > newCount:
		return TrendImproving
	case newCount > resolvedCount:
		return TrendDeteriorating
	default:
		return TrendStable
	}
}
