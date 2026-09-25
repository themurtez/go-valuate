package advisory

import "github.com/themurtez/go-valuate/financial/metrics"

// buildFinancialPerformanceSection composes FINANCIAL_PERFORMANCE from
// Input.Financial.Metrics — task section 16. financial/metrics.Snapshot
// is read verbatim, matching every sibling composition package's
// identical precedent (reporting/management.HistoricalPeriod). Growth/
// variance figures, when supplied, are read from analytics/variance via
// synthesis.go's cross-references rather than recomputed here.
func buildFinancialPerformanceSection(in Input, policy Policy) Section {
	snaps := in.Financial.Metrics.Snapshots
	if len(snaps) == 0 {
		return newUnavailableSection(SectionFinancialPerf, StatusNotSupplied)
	}

	cur, hasCur := latestSnapshot(snaps, currentPeriodLabel(in))
	if !hasCur {
		return newUnavailableSection(SectionFinancialPerf, StatusUnavailable)
	}
	prior, hasPrior := latestSnapshot(snaps, priorPeriodLabel(in))

	const src = "metrics"
	mk := func(code, label string, cur2 metrics.MetricValue, unit Unit) Metric {
		curV := Value{Available: cur2.Available, Amount: cur2.Value}
		if hasPrior {
			priorV := metricValueByField(prior, code)
			return newMetricWithPrior(code, label, curV, priorV, unit, string(cur.Period), src, code)
		}
		return newMetric(code, label, curV, unit, string(cur.Period), src, code)
	}

	metricsOut := []Metric{
		mk(metricCodeRevenue, "Total Revenue", cur.TotalRevenue, UnitCurrency),
		mk("gross_profit", "Gross Profit", cur.GrossProfit, UnitCurrency),
		mk(metricCodeGrossMargin, "Gross Margin", cur.GrossMargin, UnitPercent),
		mk(metricCodeEBITDA, "EBITDA", cur.EBITDA, UnitCurrency),
		mk("ebitda_margin", "EBITDA Margin", cur.EBITDAMargin, UnitPercent),
		mk("net_income", "Net Income", cur.NetIncome, UnitCurrency),
	}

	sourceRef := []SourceRef{{Module: src, Period: string(cur.Period)}}
	return Section{
		Code: SectionFinancialPerf, Availability: StatusAvailable,
		Metrics: metricsOut,
		Sources: sourceRef,
	}
}

// latestSnapshot returns the Snapshot in snaps whose Period matches label,
// or (if label is empty) the last entry in snaps — this package never
// guesses chronology from Snapshots' own order (financial/metrics.Result's
// own doc comment warns Snapshots is not guaranteed chronological), so an
// empty label falls back to input order only as a last resort, matching
// reporting/management's identical documented limitation for its
// ExecutiveSummary fallback.
func latestSnapshot(snaps []metrics.Snapshot, label string) (metrics.Snapshot, bool) {
	if label != "" {
		for _, s := range snaps {
			if string(s.Period) == label {
				return s, true
			}
		}
		return metrics.Snapshot{}, false
	}
	if len(snaps) == 0 {
		return metrics.Snapshot{}, false
	}
	return snaps[len(snaps)-1], true
}

// metricValueByField reads one named metric field off a Snapshot by the
// same code mk uses to build its Metric — a small closed switch, not
// reflection, matching this repository's "no reflection" discipline
// (task section 48's KPI-engine precedent explicitly forbids reflection).
func metricValueByField(s metrics.Snapshot, code string) Value {
	var mv metrics.MetricValue
	switch code {
	case metricCodeRevenue:
		mv = s.TotalRevenue
	case "gross_profit":
		mv = s.GrossProfit
	case metricCodeGrossMargin:
		mv = s.GrossMargin
	case metricCodeEBITDA:
		mv = s.EBITDA
	case "ebitda_margin":
		mv = s.EBITDAMargin
	case "net_income":
		mv = s.NetIncome
	default:
		return Unavailable()
	}
	return Value{Available: mv.Available, Amount: mv.Value}
}
