package advisory

import "github.com/themurtez/go-valuate/accounting/vendorspend"

// buildVendorSpendSection composes VENDOR_SPEND from
// Input.Operating.VendorSpend — task section 24.
func buildVendorSpendSection(in Input, policy Policy) Section {
	vs := in.Operating.VendorSpend
	if !vs.Available {
		return newUnavailableSection(SectionVendorSpend, StatusNotSupplied)
	}

	period := currentPeriodLabel(in)
	if len(vs.Periods) > 0 {
		period = vs.Periods[len(vs.Periods)-1]
	}

	var metricsOut []Metric
	metricsOut = append(metricsOut, newMetric("net_vendor_spend", "Net Vendor Spend", AvailableValue(vs.Bridge.NetSpend), UnitCurrency, period, "vendorspend", "bridge.net_spend"))

	if vs.Concentration.Available {
		if vs.Concentration.Top1.Available {
			metricsOut = append(metricsOut, newMetric("top_supplier_share", "Top Supplier Share", AvailableValue(vs.Concentration.Top1.Value), UnitPercent, period, "vendorspend", "concentration.top1"))
		}
		if vs.Concentration.Top5.Available {
			metricsOut = append(metricsOut, newMetric("top5_supplier_share", "Top 5 Supplier Share", AvailableValue(vs.Concentration.Top5.Value), UnitPercent, period, "vendorspend", "concentration.top5"))
		}
	}
	if vs.TailSpend.Available && vs.TailSpend.TailSpendPercent.Available {
		metricsOut = append(metricsOut, newMetric("tail_spend_percent", "Tail Spend %", AvailableValue(vs.TailSpend.TailSpendPercent.Value), UnitPercent, period, "vendorspend", "tail_spend.tail_spend_percent"))
	}

	var findings []Insight
	var actions []ActionItem
	sourceRef := []SourceRef{{Module: "vendorspend", Period: period}}

	for _, f := range vs.Flags {
		findings = append(findings, Insight{
			Code: string(f.Code), Category: string(SectionVendorSpend), Severity: severityFromVendorSpendFlag(f.Severity),
			Title: "Vendor spend flag", Statement: f.Message, Period: period, EntityRef: f.SupplierID,
			SourceModule: "vendorspend", SourceCode: string(f.Code),
			SourceRefs: []SourceRef{{Module: "vendorspend", Code: string(f.Code), Ref: f.SupplierID, Period: period}},
		})
		switch f.Code {
		case "HIGH_SUPPLIER_CONCENTRATION":
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewSupplierConcentration], PriorityMedium, "vendorspend", string(f.Code), sourceRef, f.SupplierID, period))
		case "UNIT_PRICE_INCREASE":
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewUnitPriceChange], PriorityMedium, "vendorspend", string(f.Code), []SourceRef{{Module: "vendorspend", Ref: f.SupplierID, Period: period}}, f.SupplierID, period))
		case "NON_PREFERRED_SUPPLIER_SPEND":
			actions = append(actions, newGeneratedAction(actionTemplates[ActionReviewNonPreferredSupplierSpend], PriorityLow, "vendorspend", string(f.Code), []SourceRef{{Module: "vendorspend", Ref: f.SupplierID, Period: period}}, f.SupplierID, period))
		}
	}

	return Section{
		Code: SectionVendorSpend, Availability: StatusAvailable,
		Metrics: metricsOut, Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(dedupeActions(actions), policy),
		Sources: sourceRef,
	}
}

func severityFromVendorSpendFlag(s vendorspend.FlagSeverity) Severity {
	switch s {
	case vendorspend.FlagSeverityCritical:
		return SeverityHigh
	case vendorspend.FlagSeverityWarning:
		return SeverityMedium
	default:
		return SeverityInfo
	}
}
