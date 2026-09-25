package advisory

import "github.com/themurtez/go-valuate/analytics/kpi"

// buildKPISection composes KPI from Input.KPIValues — task section 30.
// Every kpi.KPIResult is read verbatim (value/target/band/trend/
// availability); no KPI formula is re-evaluated — task section 30's "do
// not reinterpret caller band labels" rule.
func buildKPISection(in Input, policy Policy) Section {
	if len(in.KPIValues) == 0 {
		return newUnavailableSection(SectionKPI, StatusNotSupplied)
	}

	var metricsOut []Metric
	var findings []Insight
	for _, k := range in.KPIValues {
		m := Metric{
			Code: k.Code, Label: k.Code, Period: k.Period,
			Value:        Value{Available: k.Value.Available, Amount: k.Value.Amount},
			Unit:         kpiUnitToAdvisoryUnit(k.Unit),
			SourceModule: "kpi", SourceCode: k.Code,
		}
		if k.Change.Prior.Available {
			m.Prior = Value{Available: k.Change.Prior.Available, Amount: k.Change.Prior.Amount}
			m.Change = Change{
				Current:               m.Value,
				Prior:                 m.Prior,
				AbsoluteChange:        Value{Available: k.Change.AbsoluteChange.Available, Amount: k.Change.AbsoluteChange.Amount},
				PercentChange:         Value{Available: k.Change.RelativeChangePercent.Available, Amount: k.Change.RelativeChangePercent.Amount},
				PercentagePointChange: Value{Available: k.Change.PercentagePointChange.Available, Amount: k.Change.PercentagePointChange.Amount},
			}
		}
		metricsOut = append(metricsOut, m)

		if k.TargetEvaluation.TargetAvailable && !k.TargetEvaluation.TargetMet {
			findings = append(findings, Insight{
				Code: "KPI_TARGET_NOT_MET", Category: string(SectionKPI), Severity: SeverityMedium,
				Title: "KPI target not met: " + k.Code, Statement: "KPI " + k.Code + " did not meet its configured target.",
				Period: k.Period, Current: m.Value,
				SourceModule: "kpi", SourceCode: k.Code,
				SourceRefs: []SourceRef{{Module: "kpi", Code: k.Code, Period: k.Period}},
			})
		}
		if !k.Value.Available {
			findings = append(findings, Insight{
				Code: "KPI_VALUE_UNAVAILABLE", Category: string(SectionKPI), Severity: SeverityInfo,
				Title: "KPI value unavailable: " + k.Code, Statement: "KPI " + k.Code + " could not be evaluated: " + string(k.Value.Reason) + ".",
				Period:       k.Period,
				SourceModule: "kpi", SourceCode: k.Code,
				SourceRefs: []SourceRef{{Module: "kpi", Code: k.Code, Period: k.Period}},
			})
		}
	}

	return Section{
		Code: SectionKPI, Availability: StatusAvailable,
		Metrics: metricsOut, Findings: capInsightsForSection(findings, policy),
		Sources: []SourceRef{{Module: "kpi"}},
	}
}

func kpiUnitToAdvisoryUnit(u kpi.Unit) Unit {
	switch u.Kind {
	case kpi.UnitCurrency:
		return UnitCurrency
	case kpi.UnitPercent:
		return UnitPercent
	case kpi.UnitDays:
		return UnitDays
	case kpi.UnitRatio:
		return UnitRatio
	case kpi.UnitCount, kpi.UnitQuantity:
		return UnitCount
	default:
		return UnitCount
	}
}
