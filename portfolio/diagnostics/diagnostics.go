package diagnostics

// Calculate scans in.Portfolio and returns a full Result: every diagnostic
// Finding across the portfolio, ranked, plus portfolio-level counts and a
// coverage/missing-data summary. It never mutates any caller-owned input
// and performs no I/O.
//
// A BusinessSnapshot with an empty ID is skipped entirely (see
// IssueEmptyBusinessID); a duplicate ID keeps only the first occurrence
// (see IssueDuplicateBusinessID). Every other snapshot is scanned
// regardless of how many of its optional summaries are Available — an
// absent summary simply narrows which detect* rules can fire for that
// business (see Coverage) rather than failing the whole scan.
func Calculate(in Input) Result {
	policy := resolvePolicy(in.Policy)
	result := Result{FormulaVersion: FormulaVersion, ScoreVersion: ScoreVersion, Policy: policy}

	var issues []Issue
	if len(in.Portfolio) == 0 {
		issues = append(issues, Issue{
			Code:     IssueEmptyPortfolio,
			Severity: IssueSeverityError,
			Message:  "portfolio has no businesses; nothing to scan",
		})
		result.Errors = issues
		return result
	}

	seen := make(map[string]bool, len(in.Portfolio))
	var scanned []BusinessSnapshot
	for _, b := range in.Portfolio {
		if b.ID == "" {
			issues = append(issues, Issue{
				Code:     IssueEmptyBusinessID,
				Severity: IssueSeverityWarning,
				Message:  "a business snapshot with an empty ID was skipped",
			})
			continue
		}
		if seen[b.ID] {
			issues = append(issues, Issue{
				Code:     IssueDuplicateBusinessID,
				Severity: IssueSeverityWarning,
				Message:  "business ID \"" + b.ID + "\" appeared more than once; only the first occurrence was scanned",
			})
			continue
		}
		seen[b.ID] = true
		scanned = append(scanned, b)
	}

	if len(scanned) == 0 {
		result.Errors = issues
		return result
	}
	result.Available = true

	result.Coverage = buildCoverage(scanned)

	var findings []Finding
	for _, b := range scanned {
		bf := detectFindings(b, policy)
		for i := range bf {
			bf[i].PriorityScore = computePriorityScore(bf[i], policy)
		}
		findings = append(findings, bf...)
	}
	sortFindings(findings)
	result.Findings = findings

	result.Counts = buildCounts(findings)

	for i := range issues {
		if issues[i].Severity == IssueSeverityError {
			result.Errors = append(result.Errors, issues[i])
		} else {
			result.Warnings = append(result.Warnings, issues[i])
		}
	}

	return result
}

// buildCoverage summarizes how many of scanned had each optional summary
// Available and a non-nil Prior.
func buildCoverage(scanned []BusinessSnapshot) CoverageCounts {
	c := CoverageCounts{TotalBusinesses: len(scanned)}
	for _, b := range scanned {
		if b.QoE.Available {
			c.WithQoE++
		}
		if b.RatioHealth.Available {
			c.WithRatioHealth++
		}
		if b.Concentration.Available {
			c.WithConcentration++
		}
		if b.CashFlow.Available {
			c.WithCashFlow++
		}
		if b.Valuation.Available {
			c.WithValuation++
		}
		if b.SaleReadiness.Available {
			c.WithSaleReadiness++
		}
		if b.Prior != nil {
			c.WithPrior++
		}
	}
	return c
}

// buildCounts summarizes findings by Severity and FindingCode, and counts
// the distinct businesses represented.
func buildCounts(findings []Finding) PortfolioCounts {
	counts := PortfolioCounts{TotalFindings: len(findings)}
	if len(findings) == 0 {
		return counts
	}

	bySeverity := make(map[Severity]int)
	byCode := make(map[FindingCode]int)
	businesses := make(map[string]bool)
	for _, f := range findings {
		bySeverity[f.Severity]++
		byCode[f.Code]++
		businesses[f.BusinessID] = true
	}
	counts.BySeverity = bySeverity
	counts.ByCode = byCode
	counts.BusinessesWithFindings = len(businesses)
	return counts
}
