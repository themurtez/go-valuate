package advisory

import "github.com/themurtez/go-valuate/transactions/salereadiness"

// buildTransactionReadinessSection composes TRANSACTION_READINESS from
// Input.Transaction.SaleReadiness/Acquisition/DealStructure — task
// section 29. Never infers a should-sell/should-acquire conclusion;
// reports each source's own facts only.
func buildTransactionReadinessSection(in Input, policy Policy) Section {
	sr := in.Transaction.SaleReadiness
	acq := in.Transaction.Acquisition
	if !sr.Available && !acq.Available {
		return newUnavailableSection(SectionTransactionReady, StatusNotSupplied)
	}

	period := currentPeriodLabel(in)
	var metricsOut []Metric
	var findings []Insight
	usedModules := map[string]bool{}

	if sr.Available {
		usedModules["sale_readiness"] = true
		if sr.OverallScore != nil {
			metricsOut = append(metricsOut, newMetric("sale_readiness_score", "Sale-Readiness Score", AvailableValue(sr.OverallScore.Value), UnitCount, period, "sale_readiness", "overall_score.value"))
		}
		for _, b := range sr.Blockers {
			findings = append(findings, Insight{
				Code: string(b.Dimension), Category: string(SectionTransactionReady), Severity: severityFromSaleReadiness(b.Severity),
				Title: "Sale-readiness blocker", Statement: b.Message, Period: period,
				SourceModule: "sale_readiness", SourceCode: string(b.Dimension),
				SourceRefs: []SourceRef{{Module: "sale_readiness", Code: string(b.Dimension), Period: period}},
			})
		}
	}

	if acq.Available {
		usedModules["acquisition"] = true
		if acq.Multiples.PriceToEBITDA.Available {
			metricsOut = append(metricsOut, newMetric("price_to_ebitda", "Price / EBITDA", AvailableValue(acq.Multiples.PriceToEBITDA.Amount), UnitMultiple, period, "acquisition", "multiples.price_to_ebitda"))
		}
		if acq.Consensus.Premium.Available {
			metricsOut = append(metricsOut, newMetric("premium_to_consensus", "Premium to Consensus", AvailableValue(acq.Consensus.PremiumPercent.Amount), UnitPercent, period, "acquisition", "consensus.premium_percent"))
		}
		if acq.Returns.CashOnCashReturn.Available {
			metricsOut = append(metricsOut, newMetric("cash_on_cash_return", "Cash-on-Cash Return", AvailableValue(acq.Returns.CashOnCashReturn.Amount), UnitPercent, period, "acquisition", "returns.cash_on_cash_return"))
		}
		for _, f := range acq.Flags {
			findings = append(findings, Insight{
				Code: string(f.Code), Category: string(SectionTransactionReady), Severity: SeverityMedium,
				Title: "Acquisition screening flag", Statement: f.Message, Period: period,
				SourceModule: "acquisition", SourceCode: string(f.Code),
				SourceRefs: []SourceRef{{Module: "acquisition", Code: string(f.Code), Period: period}},
			})
		}
	}

	sources := make([]SourceRef, 0, len(usedModules))
	for m := range usedModules {
		sources = append(sources, SourceRef{Module: m, Period: period})
	}

	return Section{
		Code: SectionTransactionReady, Availability: StatusAvailable,
		Metrics: metricsOut, Findings: capInsightsForSection(findings, policy),
		Sources: sortedSourceRefs(sources),
	}
}

func severityFromSaleReadiness(s salereadiness.Severity) Severity {
	switch s {
	case salereadiness.SeverityCritical:
		return SeverityHigh
	case salereadiness.SeverityWarning:
		return SeverityMedium
	default:
		return SeverityInfo
	}
}
