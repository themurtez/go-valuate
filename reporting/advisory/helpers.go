package advisory

// currentPeriodLabel returns Input.Company.CurrentPeriod, the caller's own
// declared current-period label — task section 4/49. Sections use this as
// the Period stamped on a point-in-time or "most recent" Metric/Insight
// when the source sibling Result itself has no more specific period of
// its own (e.g. accounting/cashforecast, analytics/debt, which are
// point-in-time by design). Never inferred from any source Result's own
// data.
func currentPeriodLabel(in Input) string { return in.Company.CurrentPeriod }

// priorPeriodLabel returns Input.Company.PriorPeriod.
func priorPeriodLabel(in Input) string { return in.Company.PriorPeriod }

// capInsightsForSection caps in at policy.MaxSectionHighlights, after
// sorting by insightSortKey using DefaultCategoryOrder (a section's own
// Findings/Highlights are ordered the same way regardless of
// Policy.CategoryOrder, since CategoryOrder governs section-to-section
// order, not within-section order).
func capInsightsForSection(in []Insight, policy Policy) []Insight {
	sortInsights(in, sectionOrder)
	if policy.MaxSectionHighlights <= 0 || len(in) <= policy.MaxSectionHighlights {
		return in
	}
	return in[:policy.MaxSectionHighlights]
}

// capActionsForSection caps in at policy.MaxActions, after sorting by
// actionSortKey.
func capActionsForSection(in []ActionItem, policy Policy) []ActionItem {
	sortActions(in, sectionOrder)
	if policy.MaxActions <= 0 || len(in) <= policy.MaxActions {
		return in
	}
	return in[:policy.MaxActions]
}
