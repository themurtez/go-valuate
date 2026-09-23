package diagnostics

import "sort"

// Calculate derives a full Result from in. It never mutates any
// caller-owned input and performs no I/O.
//
// Calculate mines whichever of Input's fifteen optional sibling Results
// are available for their own already-computed Flags/Signals/Anomalies/
// Status classifications and republishes them as Findings — it computes
// no financial figure of its own and reaches no legal, tax, or investment
// conclusion. See the package doc comment.
func Calculate(in Input) Result {
	coverage := buildCoverage(in)
	policy := in.Policy
	result := Result{FormulaVersion: FormulaVersion, Coverage: coverage, Policy: policy}

	if coverage.AvailableModules == 0 {
		result.Errors = []Issue{{
			Code:     IssueNoInputSupplied,
			Severity: IssueSeverityError,
			Message:  "no optional analytics/valuation module was supplied or available; nothing to diagnose",
		}}
		return result
	}
	result.Available = true

	var findings []Finding
	findings = append(findings, mineMetrics(in)...)
	findings = append(findings, mineRatios(in)...)
	findings = append(findings, mineQoE(in)...)
	findings = append(findings, mineWorkingCapital(in)...)
	findings = append(findings, mineCashFlow(in)...)
	findings = append(findings, mineRevenueQuality(in)...)
	findings = append(findings, mineConcentration(in)...)
	findings = append(findings, mineAnomalies(in)...)
	findings = append(findings, mineVariance(in)...)
	findings = append(findings, mineDebt(in)...)
	findings = append(findings, mineCovenants(in)...)
	findings = append(findings, mineBenchmarks(in)...)
	findings = append(findings, mineValueDrivers(in)...)

	srFindings, srStrengths, srOpportunities := mineSaleReadiness(in)
	findings = append(findings, srFindings...)

	sortFindings(findings)
	result.Findings = findings

	var strengths, concerns, opportunities []Finding
	for _, f := range findings {
		switch {
		case isOpportunityCode(f.Code):
			opportunities = append(opportunities, f)
		case f.Severity == SeverityInfo:
			strengths = append(strengths, f)
		default:
			concerns = append(concerns, f)
		}
	}
	strengths = append(strengths, srStrengths...)
	opportunities = append(opportunities, srOpportunities...)
	sortFindings(strengths)
	sortFindings(concerns)
	sortFindings(opportunities)

	result.Strengths = strengths
	result.Concerns = concerns
	result.Opportunities = opportunities

	result.MissingDataAreas = buildMissingDataAreas(coverage)

	if score, ok := computeHealthScore(findings, coverage, policy); ok {
		result.OverallHealthScore = &score
	}

	for _, m := range coverage.MissingModules {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueModuleUnavailable,
			Severity: IssueSeverityWarning,
			Message:  m + " was not supplied or was unavailable; findings for the categories it backs are narrowed",
			Module:   SourceModule(m),
		})
	}

	return result
}

// isOpportunityCode reports whether code is always framed as an
// improvement-area Opportunity rather than a Strength/Concern, regardless
// of the Finding's own Severity — currently only
// FindingSaleReadinessOpportunity (echoing
// transactions/salereadiness.Opportunity, which is itself never a Blocker/
// Risk/Strength).
func isOpportunityCode(code FindingCode) bool {
	return code == FindingSaleReadinessOpportunity
}

// findingCodeRank maps each FindingCode to its declaration-order index in
// findingSortOrder, used only as a sort tie-break.
var findingCodeRank = func() map[FindingCode]int {
	ranks := make(map[FindingCode]int, len(findingSortOrder))
	for i, c := range findingSortOrder {
		ranks[c] = i
	}
	return ranks
}()

// categoryRank maps each Category to its declaration-order index in
// categoryOrder, used only as a sort tie-break.
var categoryRank = func() map[Category]int {
	ranks := make(map[Category]int, len(categoryOrder))
	for i, c := range categoryOrder {
		ranks[c] = i
	}
	return ranks
}()

// severityRank orders Severity from most to least urgent for sorting.
var severityRank = map[Severity]int{
	SeverityCritical: 0,
	SeverityWarning:  1,
	SeverityInfo:     2,
}

// sortFindings orders findings by Category (categoryOrder), then Severity
// descending (critical first), then FindingCode declaration order, then
// Period ascending, then SourceCode ascending — a fixed, fully specified,
// deterministic tie-break chain that never depends on map iteration order
// or input slice order. Sorts in place; callers pass a slice they already
// own.
func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if ra, rb := categoryRank[a.Category], categoryRank[b.Category]; ra != rb {
			return ra < rb
		}
		if ra, rb := severityRank[a.Severity], severityRank[b.Severity]; ra != rb {
			return ra < rb
		}
		if ra, rb := findingCodeRank[a.Code], findingCodeRank[b.Code]; ra != rb {
			return ra < rb
		}
		if a.Period != b.Period {
			return a.Period < b.Period
		}
		return a.SourceCode < b.SourceCode
	})
}
