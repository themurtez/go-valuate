package cashforecast

// validSourceOpenAmountTotal sums OpenAmount across sources, excluding
// any non-finite or negative value — an ARReceivableSource/
// APPayableSource has no CashFlowEvent-style validation pass of its own
// (it is a lightweight coverage-percentage/over-scheduling input, not a
// forecasted cash movement), so this guard exists specifically to keep a
// single bad row (e.g. a credit memo mis-mapped as a receivable with a
// negative OpenAmount) from silently driving ScheduledPercent above 100%
// or negative.
func validSourceOpenAmountTotal(amounts []float64) (total float64, excludedCount int) {
	for _, a := range amounts {
		if isNonFinite(a) || a < 0 {
			excludedCount++
			continue
		}
		total += a
	}
	return total, excludedCount
}

// buildCoverage assembles Coverage from Input/Options and the
// already-computed UnscheduledAR/UnscheduledAP summaries.
func buildCoverage(in Input, opts Options, opening OpeningPosition, unscheduledAR, unscheduledAP UnscheduledSummary) (Coverage, []Issue) {
	var issues []Issue
	c := Coverage{
		OpeningCashSupplied: opening.TotalCash != 0 || in.OpeningCash.Currency != "" || len(in.CashAccounts) > 0,
	}

	c.AR = SourceCoverage{Supplied: len(in.ARSources) > 0 || len(in.ARCollections) > 0}
	if len(in.ARSources) > 0 {
		amounts := make([]float64, len(in.ARSources))
		for i, s := range in.ARSources {
			amounts[i] = s.OpenAmount
		}
		totalOpen, excluded := validSourceOpenAmountTotal(amounts)
		if excluded > 0 {
			issues = append(issues, Issue{Code: IssueNonFiniteAmount, Severity: SeverityWarning,
				Message: "one or more ARReceivableSource.OpenAmount values are negative or non-finite and were excluded from AR coverage"})
		}
		if totalOpen > 0 {
			scheduled := totalOpen - unscheduledAR.Amount
			c.AR.ScheduledPercent = AvailableAmount(scheduled / totalOpen)
		}
	}

	c.AP = SourceCoverage{Supplied: len(in.APSources) > 0 || len(in.APPlans) > 0}
	if len(in.APSources) > 0 {
		amounts := make([]float64, len(in.APSources))
		for i, s := range in.APSources {
			amounts[i] = s.OpenAmount
		}
		totalOpen, excluded := validSourceOpenAmountTotal(amounts)
		if excluded > 0 {
			issues = append(issues, Issue{Code: IssueNonFiniteAmount, Severity: SeverityWarning,
				Message: "one or more APPayableSource.OpenAmount values are negative or non-finite and were excluded from AP coverage"})
		}
		if totalOpen > 0 {
			scheduled := totalOpen - unscheduledAP.Amount
			c.AP.ScheduledPercent = AvailableAmount(scheduled / totalOpen)
		}
	}

	c.Payroll = SourceCoverage{Supplied: len(in.Payroll) > 0}
	c.DebtService = SourceCoverage{Supplied: len(in.DebtService) > 0}
	c.Tax = SourceCoverage{Supplied: len(in.Tax) > 0}
	c.RecurringOpex = SourceCoverage{Supplied: len(in.RecurringRules) > 0}
	c.Capex = SourceCoverage{Supplied: len(in.Capex) > 0}

	c.ARStaleness = buildStaleness(in.ARSnapshotDate, in.ForecastStartDate, opts.Staleness.MaxARAgeDays)
	c.APStaleness = buildStaleness(in.APSnapshotDate, in.ForecastStartDate, opts.Staleness.MaxAPAgeDays)
	c.CashStaleness = buildStaleness(in.OpeningCash.AsOfDate, in.ForecastStartDate, opts.Staleness.MaxCashAgeDays)

	req := opts.RequiredInputs
	if req.AR && !c.AR.Supplied {
		c.RequiredInputsMissing = append(c.RequiredInputsMissing, "AR")
	}
	if req.AP && !c.AP.Supplied {
		c.RequiredInputsMissing = append(c.RequiredInputsMissing, "AP")
	}
	if req.Payroll && !c.Payroll.Supplied {
		c.RequiredInputsMissing = append(c.RequiredInputsMissing, "PAYROLL")
	}
	if req.DebtService && !c.DebtService.Supplied {
		c.RequiredInputsMissing = append(c.RequiredInputsMissing, "DEBT_SERVICE")
	}
	if req.Tax && !c.Tax.Supplied {
		c.RequiredInputsMissing = append(c.RequiredInputsMissing, "TAX")
	}

	return c, issues
}
