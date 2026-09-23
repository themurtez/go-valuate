package management

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// buildExecutiveSummary selects a small, fixed set of headline KPIs from
// whichever Input fields are available, each for the most recent period
// its own contributing source has (sources are not reconciled to a common
// period — see KPI.Period's doc comment). KPIs is built in a fixed
// declaration order (never Go map order): revenue/EBITDA/margin from
// metrics, current ratio/net-debt-to-EBITDA from ratios, free-cash-flow
// conversion from cash flow, NWC from working capital, largest-customer
// share from concentration, recurring-revenue percent from revenue
// quality, and indicated value from consensus. A KPI whose underlying
// figure is entirely unavailable is omitted from KPIs rather than
// included unavailable — see ExecutiveSummary's doc comment.
func buildExecutiveSummary(in Input, report Report) ExecutiveSummary {
	var kpis []KPI
	var mostRecent financial.Period

	recordPeriod := func(p financial.Period) {
		if p == "" {
			return
		}
		if mostRecent == "" || periodIsAfter(p, mostRecent, in.PeriodMeta) {
			mostRecent = p
		}
	}

	if len(report.HistoricalSeries.Periods) > 0 {
		periods := report.HistoricalSeries.Periods
		cur := periods[len(periods)-1]
		var prior HistoricalPeriod
		hasPrior := len(periods) >= 2
		if hasPrior {
			prior = periods[len(periods)-2]
		}

		if k, ok := kpiFromValue("Total Revenue", cur.TotalRevenue, prior.TotalRevenue, hasPrior, UnitCurrency, cur.Period, "metrics"); ok {
			kpis = append(kpis, k)
			recordPeriod(k.Period)
		}
		if k, ok := kpiFromValue("Gross Profit", cur.GrossProfit, prior.GrossProfit, hasPrior, UnitCurrency, cur.Period, "metrics"); ok {
			kpis = append(kpis, k)
			recordPeriod(k.Period)
		}
		if k, ok := kpiFromValue("EBITDA", cur.EBITDA, prior.EBITDA, hasPrior, UnitCurrency, cur.Period, "metrics"); ok {
			kpis = append(kpis, k)
			recordPeriod(k.Period)
		}
		if k, ok := kpiFromValue("Net Income", cur.NetIncome, prior.NetIncome, hasPrior, UnitCurrency, cur.Period, "metrics"); ok {
			kpis = append(kpis, k)
			recordPeriod(k.Period)
		}
	}

	if len(report.ProfitabilitySeries.Periods) > 0 {
		periods := report.ProfitabilitySeries.Periods
		cur := periods[len(periods)-1]
		var prior ProfitabilityPeriod
		hasPrior := len(periods) >= 2
		if hasPrior {
			prior = periods[len(periods)-2]
		}
		if k, ok := kpiFromValue("EBITDA Margin", cur.EBITDAMargin, prior.EBITDAMargin, hasPrior, UnitPercent, cur.Period, report.ProfitabilitySeries.Source); ok {
			kpis = append(kpis, k)
			recordPeriod(k.Period)
		}
	}

	if len(report.LiquidityLeverageSeries.Periods) > 0 {
		periods := report.LiquidityLeverageSeries.Periods
		cur := periods[len(periods)-1]
		var prior LiquidityLeveragePeriod
		hasPrior := len(periods) >= 2
		if hasPrior {
			prior = periods[len(periods)-2]
		}
		if k, ok := kpiFromValue("Current Ratio", cur.CurrentRatio, prior.CurrentRatio, hasPrior, UnitMultiple, cur.Period, "ratios"); ok {
			kpis = append(kpis, k)
			recordPeriod(k.Period)
		}
		if k, ok := kpiFromValue("Net Debt / EBITDA", cur.NetDebtToEBITDA, prior.NetDebtToEBITDA, hasPrior, UnitMultiple, cur.Period, "ratios"); ok {
			kpis = append(kpis, k)
			recordPeriod(k.Period)
		}
	}

	if len(report.CashFlowSeries.Periods) > 0 {
		periods := report.CashFlowSeries.Periods
		cur := periods[len(periods)-1]
		var prior CashFlowPeriod
		hasPrior := len(periods) >= 2
		if hasPrior {
			prior = periods[len(periods)-2]
		}
		if k, ok := kpiFromValue("Free Cash Flow", cur.FreeCashFlow, prior.FreeCashFlow, hasPrior, UnitCurrency, cur.Period, "cash_flow"); ok {
			kpis = append(kpis, k)
			recordPeriod(k.Period)
		}
		if k, ok := kpiFromValue("EBITDA to FCF Conversion", cur.EBITDAToFreeCashFlow, prior.EBITDAToFreeCashFlow, hasPrior, UnitPercent, cur.Period, "cash_flow"); ok {
			kpis = append(kpis, k)
			recordPeriod(k.Period)
		}
	}

	if len(report.WorkingCapitalSeries.Periods) > 0 {
		periods := report.WorkingCapitalSeries.Periods
		cur := periods[len(periods)-1]
		var prior WorkingCapitalPeriod
		hasPrior := len(periods) >= 2
		if hasPrior {
			prior = periods[len(periods)-2]
		}
		if k, ok := kpiFromValue("Net Working Capital", cur.NWC, prior.NWC, hasPrior, UnitCurrency, cur.Period, "working_capital"); ok {
			kpis = append(kpis, k)
			recordPeriod(k.Period)
		}
	}

	if in.Concentration.Available && len(in.Concentration.History) > 0 {
		cur := in.Concentration.History[len(in.Concentration.History)-1]
		v := Value{Available: cur.LargestEntityShare.Available, Amount: cur.LargestEntityShare.Value}
		if v.Available {
			kpis = append(kpis, KPI{Label: "Largest Customer Share", Value: v, Unit: UnitPercent, Period: cur.Period, Source: "concentration"})
			recordPeriod(cur.Period)
		}
	}

	if in.RevenueQuality.Available && len(in.RevenueQuality.TotalRevenueHistory) > 0 {
		cur := in.RevenueQuality.TotalRevenueHistory[len(in.RevenueQuality.TotalRevenueHistory)-1]
		v := Value{Available: cur.RecurringPercent.Available, Amount: cur.RecurringPercent.Value}
		if v.Available {
			kpis = append(kpis, KPI{Label: "Recurring Revenue %", Value: v, Unit: UnitPercent, Period: cur.Period, Source: "revenue_quality"})
			recordPeriod(cur.Period)
		}
	}

	if in.Consensus.Available && in.Consensus.Statistics.Count > 0 {
		amount := in.Consensus.Statistics.WeightedMean
		if !in.Consensus.WeightsValid {
			amount = in.Consensus.Statistics.SimpleMean
		}
		kpis = append(kpis, KPI{Label: "Indicated Value", Value: AvailableValue(amount), Unit: UnitCurrency, Source: "consensus"})
	}

	return ExecutiveSummary{
		Available: len(kpis) > 0,
		Period:    mostRecent,
		KPIs:      kpis,
	}
}

// periodIsAfter reports whether a is chronologically after b, using meta
// (Input.PeriodMeta) when both a and b have an entry in it. When meta does
// not cover both periods, it falls back to a plain string comparison —
// this package's best-effort answer for ExecutiveSummary.Period across
// several independently-sourced KPI series, since ExecutiveSummary has no
// single sibling Result to defer chronological ordering to the way
// HistoricalSeries defers to orderedSnapshots. A caller wanting a
// guaranteed-correct comparison should supply PeriodMeta covering every
// period its Input's sections use.
func periodIsAfter(a, b financial.Period, meta map[financial.Period]metrics.PeriodInfo) bool {
	if meta != nil {
		infoA, okA := meta[a]
		infoB, okB := meta[b]
		if okA && okB {
			if infoA.FiscalYear != infoB.FiscalYear {
				return infoA.FiscalYear > infoB.FiscalYear
			}
			return infoA.SequenceInYear > infoB.SequenceInYear
		}
	}
	return a > b
}

// kpiFromValue builds a KPI from a current/prior Value pair, computing
// Change when both are available and prior is nonzero. Returns ok == false
// when cur itself is unavailable — such a KPI is omitted entirely by the
// caller, per ExecutiveSummary.KPIs' doc comment.
func kpiFromValue(label string, cur, prior Value, hasPrior bool, unit Unit, period financial.Period, source string) (KPI, bool) {
	if !cur.Available {
		return KPI{}, false
	}
	k := KPI{Label: label, Value: cur, Unit: unit, Period: period, Source: source}
	if hasPrior && prior.Available {
		k.PriorValue = prior
		if prior.Amount != 0 {
			k.Change = AvailableValue((cur.Amount - prior.Amount) / absFloat(prior.Amount))
		}
	}
	return k, true
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
