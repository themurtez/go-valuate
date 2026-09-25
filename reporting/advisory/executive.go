package advisory

import "sort"

// ExecutiveSummary is the concise, deterministic top-level summary — task
// section 11. Not prose generation: every field is a bounded selection
// over Insights/ActionItems this package already built for its Sections,
// picked by the explicit rule in selectExecutiveSummary (executive.go).
type ExecutiveSummary struct {
	KeyHighlights []Insight    `json:"key_highlights,omitempty"`
	KeyRisks      []Insight    `json:"key_risks,omitempty"`
	KeyActions    []ActionItem `json:"key_actions,omitempty"`

	LiquidityStatus AvailabilityStatus `json:"liquidity_status"`
	CloseStatus     AvailabilityStatus `json:"close_status"`
}

// selectExecutiveSummary builds ExecutiveSummary from every Section's own
// Highlights/Findings/Actions — task section 11's "selection logic must be
// explicit" rule. KeyHighlights draws from Highlights across every
// section (capped at policy.MaxExecutiveHighlights, sorted by
// insightSortKey); KeyRisks draws from Findings whose Severity is
// SeverityBlocking/SeverityHigh (capped identically); KeyActions draws
// from every section's Actions (capped at policy.MaxActions, sorted by
// actionSortKey). LiquidityStatus/CloseStatus echo the LIQUIDITY/
// ACCOUNTING_AND_CLOSE sections' own Availability verbatim — never
// recomputed. Never mutates sections.
func selectExecutiveSummary(sections []Section, order []SectionCode, policy Policy) ExecutiveSummary {
	summary := ExecutiveSummary{}

	var highlights, risks []Insight
	var actions []ActionItem
	for _, s := range sections {
		highlights = append(highlights, s.Highlights...)
		for _, f := range s.Findings {
			if f.Severity == SeverityBlocking || f.Severity == SeverityHigh {
				risks = append(risks, f)
			}
		}
		actions = append(actions, s.Actions...)

		switch s.Code {
		case SectionLiquidity:
			summary.LiquidityStatus = s.Availability
		case SectionAccountingAndClose:
			summary.CloseStatus = s.Availability
		}
	}

	sortInsights(highlights, order)
	sortInsights(risks, order)
	sortActions(actions, order)

	summary.KeyHighlights = capInsights(highlights, policy.MaxExecutiveHighlights)
	summary.KeyRisks = capInsights(risks, policy.MaxExecutiveHighlights)
	summary.KeyActions = capActions(actions, policy.MaxActions)

	return summary
}

func sortInsights(in []Insight, order []SectionCode) {
	sort.SliceStable(in, func(i, j int) bool {
		return lessInsightSortKey(sortKeyForInsight(in[i], order), sortKeyForInsight(in[j], order))
	})
}

func sortActions(in []ActionItem, order []SectionCode) {
	sort.SliceStable(in, func(i, j int) bool {
		return lessActionSortKey(sortKeyForAction(in[i], order), sortKeyForAction(in[j], order))
	})
}

func capInsights(in []Insight, max int) []Insight {
	if max <= 0 || len(in) <= max {
		return in
	}
	return in[:max]
}

func capActions(in []ActionItem, max int) []ActionItem {
	if max <= 0 || len(in) <= max {
		return in
	}
	return in[:max]
}
