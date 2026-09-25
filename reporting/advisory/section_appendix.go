package advisory

// buildAppendixSection composes APPENDIX from every SOURCE_CONFLICT (and
// other advisory-only) Issue Build accumulated while composing the other
// sections — a single place a caller can see every source-disagreement
// surfaced during composition, each with its own REVIEW_SOURCE_CONFLICT
// action (task section 19/46/59).
func buildAppendixSection(issues []Issue, policy Policy) Section {
	var findings []Insight
	var actions []ActionItem
	for _, iss := range issues {
		if iss.Code != IssueSourceConflict {
			continue
		}
		findings = append(findings, Insight{
			Code: string(iss.Code), Category: string(SectionAppendix), Severity: SeverityLow,
			Title: "Source conflict", Statement: iss.Message, Period: iss.Period,
			SourceModule: iss.SourceModule, SourceCode: iss.SourceCode,
			SourceRefs: []SourceRef{{Module: iss.SourceModule, Code: iss.SourceCode, Period: iss.Period}},
		})
		actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewSourceConflict], PriorityLow, iss.SourceModule, iss.SourceCode, []SourceRef{{Module: iss.SourceModule, Code: iss.SourceCode, Period: iss.Period}}, "", iss.Period))
	}

	if len(findings) == 0 {
		return newUnavailableSection(SectionAppendix, StatusNotApplicable)
	}

	return Section{
		Code: SectionAppendix, Availability: StatusAvailable,
		Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(dedupeActions(actions), policy),
	}
}
