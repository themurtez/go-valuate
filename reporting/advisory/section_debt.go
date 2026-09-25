package advisory

import (
	"github.com/themurtez/go-valuate/analytics/covenants"
	"github.com/themurtez/go-valuate/analytics/debt"
)

// buildDebtSection composes DEBT_AND_COVENANTS from Input.Financial.Debt/
// Covenants — task section 25. analytics/debt.Result is point-in-time (no
// Period field, confirmed by research); analytics/covenants.TestResult
// carries Period per-test and uses Explanation (not Message) for its
// finding text, with no Flag type of its own — findings are surfaced via
// Status/WarningBufferStatus directly, per the researched contract.
func buildDebtSection(in Input, policy Policy) Section {
	d := in.Financial.Debt
	cov := in.Financial.Covenants
	if !d.Available && !cov.Available {
		return newUnavailableSection(SectionDebtAndCovenants, StatusNotSupplied)
	}

	period := currentPeriodLabel(in)
	var metricsOut []Metric
	var findings []Insight
	var actions []ActionItem
	usedModules := map[string]bool{}

	if d.Available {
		usedModules["debt"] = true
		if d.BaseCase.TotalDebtBalance.Available {
			metricsOut = append(metricsOut, newMetric(metricCodeTotalDebt, "Total Debt Balance", AvailableValue(d.BaseCase.TotalDebtBalance.Amount), UnitCurrency, period, "debt", "base_case.total_debt_balance"))
		}
		if d.BaseCase.DSCR.Available {
			metricsOut = append(metricsOut, newMetric("dscr", "DSCR", AvailableValue(d.BaseCase.DSCR.Amount), UnitMultiple, period, "debt", "base_case.dscr"))
		}
		if d.BaseCase.NetDebtToEBITDA.Available {
			metricsOut = append(metricsOut, newMetric("net_debt_to_ebitda", "Net Debt / EBITDA", AvailableValue(d.BaseCase.NetDebtToEBITDA.Amount), UnitMultiple, period, "debt", "base_case.net_debt_to_ebitda"))
		}
		if d.Capacity.Headroom.Available {
			metricsOut = append(metricsOut, newMetric("debt_capacity_headroom", "Debt Capacity Headroom", AvailableValue(d.Capacity.Headroom.Amount), UnitCurrency, period, "debt", "capacity.headroom"))
		}
		sourceRef := []SourceRef{{Module: "debt", Period: period}}
		for _, f := range d.Flags {
			if f.Scenario != "" {
				continue // base-case facts only for this section's headline findings
			}
			findings = append(findings, Insight{
				Code: string(f.Code), Category: string(SectionDebtAndCovenants), Severity: severityFromDebtFlag(f.Severity),
				Title: "Debt flag", Statement: f.Message, Period: period,
				SourceModule: "debt", SourceCode: string(f.Code), SourceRefs: sourceRef,
			})
			if f.Code == "APPROACHING_MAXIMUM_LEVERAGE" || f.Code == "LOW_DSCR_HEADROOM" {
				actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewDebtServiceRequirement], PriorityMedium, "debt", string(f.Code), sourceRef, "", period))
			}
		}
	}

	if cov.Available {
		usedModules["covenants"] = true
		for _, t := range cov.Tests {
			testPeriod := string(t.Period)
			if testPeriod == "" {
				testPeriod = period
			}
			sourceRef := []SourceRef{{Module: "covenants", Code: t.CovenantID, Period: testPeriod}}
			if t.Headroom.Available {
				metricsOut = append(metricsOut, newMetric("covenant_headroom_"+t.CovenantID, "Covenant Headroom: "+t.CovenantID, AvailableValue(t.Headroom.Amount), UnitRatio, testPeriod, "covenants", "test_result.headroom"))
			}
			switch t.Status {
			case covenants.StatusFail:
				findings = append(findings, Insight{
					Code: string(StatementCovenantFailed), Category: string(SectionDebtAndCovenants), Severity: SeverityBlocking,
					Title: "Covenant failed", Statement: statementCovenantFailed(t.CovenantID, t.Explanation),
					Period: testPeriod, Current: Value{Available: t.Actual.Available, Amount: t.Actual.Amount}, EntityRef: t.CovenantID,
					SourceModule: "covenants", SourceCode: t.CovenantID, SourceRefs: sourceRef,
				})
				actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewCovenantFailure], PriorityCritical, "covenants", t.CovenantID, sourceRef, t.CovenantID, testPeriod))
			default:
				if t.WarningBufferStatus == covenants.WarningBufferWithinBuffer {
					findings = append(findings, Insight{
						Code: string(StatementCovenantNearBreach), Category: string(SectionDebtAndCovenants), Severity: SeverityMedium,
						Title: "Covenant approaching threshold", Statement: t.Explanation,
						Period: testPeriod, EntityRef: t.CovenantID,
						SourceModule: "covenants", SourceCode: t.CovenantID, SourceRefs: sourceRef,
					})
					actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewCovenantHeadroom], PriorityHigh, "covenants", t.CovenantID, sourceRef, t.CovenantID, testPeriod))
				}
			}
		}
	}

	sources := make([]SourceRef, 0, len(usedModules))
	for m := range usedModules {
		sources = append(sources, SourceRef{Module: m, Period: period})
	}

	return Section{
		Code: SectionDebtAndCovenants, Availability: StatusAvailable,
		Metrics: metricsOut, Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(dedupeActions(actions), policy),
		Sources: sortedSourceRefs(sources),
	}
}

func severityFromDebtFlag(s debt.FlagSeverity) Severity {
	switch s {
	case debt.FlagSeverityCritical:
		return SeverityHigh
	case debt.FlagSeverityWarning:
		return SeverityMedium
	default:
		return SeverityInfo
	}
}
