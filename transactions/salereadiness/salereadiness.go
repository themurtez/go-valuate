package salereadiness

// Calculate derives a full Result from in. It never mutates any
// caller-owned input and performs no I/O.
//
// Calculate never claims a business is guaranteed to sell, will sell at
// any particular price, or will sell within any particular timeframe —
// see the package doc comment. Every Blocker, Risk, and Strength is a
// factual, traceable observation about the supplied input, and every
// Opportunity is framed as an action, never a promise about outcome.
func Calculate(in Input) Result {
	result := Result{FormulaVersion: FormulaVersion, Policy: in.Policy}

	var issues []Issue

	hasAnyInput := in.QoE.Available || in.WorkingCapital.Available || in.Concentration.Available ||
		in.RevenueQuality.Available || in.Consensus.Available || len(in.Metrics.Snapshots) > 0 ||
		len(in.Dataset.Items) > 0 || in.Profile != (profileZeroValue) || in.DataQuality != (DataQuality{})
	if !hasAnyInput {
		issues = append(issues, Issue{
			Code:     IssueNoInputSupplied,
			Severity: IssueSeverityError,
			Message:  "no financial, analytical, profile, or data-quality input supplied; nothing to assess",
		})
		result.Errors = issues
		return result
	}
	result.Available = true

	if !in.QoE.Available {
		issues = append(issues, Issue{Code: IssueQoEUnavailable, Severity: IssueSeverityWarning,
			Message: "QoE.Available is false; earnings-stability and normalization-burden assessment is reduced"})
	}
	if !in.WorkingCapital.Available {
		issues = append(issues, Issue{Code: IssueWorkingCapitalUnavailable, Severity: IssueSeverityWarning,
			Message: "WorkingCapital.Available is false; working-capital-stability assessment is unavailable"})
	}
	if !in.Concentration.Available {
		issues = append(issues, Issue{Code: IssueConcentrationUnavailable, Severity: IssueSeverityWarning,
			Message: "Concentration.Available is false; customer-concentration assessment is unavailable"})
	}
	if !in.RevenueQuality.Available {
		issues = append(issues, Issue{Code: IssueRevenueQualityUnavailable, Severity: IssueSeverityWarning,
			Message: "RevenueQuality.Available is false; recurring-revenue assessment falls back to Profile only"})
	}
	if !in.Consensus.Available {
		issues = append(issues, Issue{Code: IssueConsensusUnavailable, Severity: IssueSeverityWarning,
			Message: "Consensus.Available is false; valuation-method-consensus assessment is unavailable"})
	}
	if len(in.Metrics.Snapshots) == 0 {
		issues = append(issues, Issue{Code: IssueMetricsUnavailable, Severity: IssueSeverityWarning,
			Message: "Metrics.Snapshots is empty; margin-trend and debt-leverage assessment may fall back to QoE or be unavailable"})
	}
	if in.Policy == (Policy{}) {
		issues = append(issues, Issue{Code: IssueNoPolicyThresholds, Severity: IssueSeverityWarning,
			Message: "Policy is the zero value; every threshold-graded dimension still assesses but cannot distinguish Strong/Acceptable/Weak/Concerning by threshold"})
	}

	dims := buildDimensions(in)
	result.Dimensions = dims

	assessed := 0
	for _, d := range dims {
		if d.Status != StatusUnassessed {
			assessed++
		}
	}
	result.Coverage = Coverage{
		TotalDimensions:    len(dimensionOrder),
		AssessedDimensions: assessed,
		CoveragePercent:    float64(assessed) / float64(len(dimensionOrder)),
	}

	blockers, risks, strengths, missing, opportunities := buildFindings(dims)
	if b, ok := negativeEarningsBlocker(in); ok {
		blockers = append([]Blocker{b}, blockers...)
	}
	result.Blockers = blockers
	result.Risks = risks
	result.Strengths = strengths
	result.MissingInformation = missing
	result.Opportunities = opportunities

	if score, ok := computeOverallScore(dims); ok {
		result.OverallScore = &score
	}

	for i := range issues {
		if issues[i].Severity == IssueSeverityError {
			result.Errors = append(result.Errors, issues[i])
		} else {
			result.Warnings = append(result.Warnings, issues[i])
		}
	}

	return result
}

// profileZeroValue is the zero value of profile.Profile, named here so
// Calculate's hasAnyInput check reads as an explicit comparison rather than
// a repeated composite literal.
var profileZeroValue = Input{}.Profile
