package management

import "github.com/themurtez/go-valuate/financial/metrics"

// buildHistoricalSeries populates HistoricalSeries from in.Metrics.Snapshots
// when supplied, reordered chronologically via orderedSnapshots. When
// in.Metrics.Snapshots is empty but in.Dataset carries items, it recomputes
// metrics.Calculate(in.Dataset, metrics.Options{PeriodMeta: in.PeriodMeta})
// itself — the same choice analytics/qoe, analytics/ratios, and
// analytics/cashflow already make for their own per-period figures (see
// qoe.Calculate's doc comment for the full rationale: metrics.Calculate
// performs no I/O and is cheap, pure arithmetic over an in-memory dataset,
// so a caller supplying only Dataset is not required to separately
// pre-compute and pass Metrics).
func buildHistoricalSeries(in Input) (HistoricalSeries, *Issue) {
	snapshots := in.Metrics.Snapshots
	formulaVersion := in.Metrics.FormulaVersion
	if len(snapshots) == 0 {
		if len(in.Dataset.Items) == 0 {
			return HistoricalSeries{}, nil
		}
		computed := metrics.Calculate(in.Dataset, metrics.Options{PeriodMeta: in.PeriodMeta})
		snapshots = computed.Snapshots
		formulaVersion = computed.FormulaVersion
		if len(snapshots) == 0 {
			return HistoricalSeries{}, nil
		}
	}
	snapshots, issue := orderedSnapshots(snapshots, in.PeriodMeta)

	periods := make([]HistoricalPeriod, 0, len(snapshots))
	for _, s := range snapshots {
		periods = append(periods, HistoricalPeriod{
			Period:             s.Period,
			TotalRevenue:       fromMetricValue(s.TotalRevenue),
			TotalCOGS:          fromMetricValue(s.TotalCOGS),
			GrossProfit:        fromMetricValue(s.GrossProfit),
			GrossMargin:        fromMetricValue(s.GrossMargin),
			TotalOpex:          fromMetricValue(s.TotalOpex),
			EBITDA:             fromMetricValue(s.EBITDA),
			EBITDAMargin:       fromMetricValue(s.EBITDAMargin),
			NetIncome:          fromMetricValue(s.NetIncome),
			Cash:               fromMetricValue(s.Cash),
			AccountsReceivable: fromMetricValue(s.AccountsReceivable),
			Inventory:          fromMetricValue(s.Inventory),
			CurrentAssets:      fromMetricValue(s.CurrentAssets),
			AccountsPayable:    fromMetricValue(s.AccountsPayable),
			CurrentLiabilities: fromMetricValue(s.CurrentLiabilities),
			WorkingCapital:     fromMetricValue(s.WorkingCapital),
			TotalDebt:          fromMetricValue(s.TotalDebt),
			NetDebt:            fromMetricValue(s.NetDebt),
		})
	}
	return HistoricalSeries{
		Available:      true,
		FormulaVersion: formulaVersion,
		Periods:        periods,
	}, issue
}

// buildProfitabilitySeries prefers in.Ratios.History (which already
// carries every margin ratio computed alongside ReturnOnAssets/
// ReturnOnEquity, unavailable from Metrics alone) and falls back to
// deriving GrossMargin/EBITDAMargin from hs (this Report's own
// already-built HistoricalSeries, itself already resolved from
// in.Metrics.Snapshots or a recomputed in.Dataset — see
// buildHistoricalSeries) when Ratios is unavailable — the same fallback
// relationship transactions/salereadiness's MarginTrend dimension already
// establishes between these two sibling packages. Deriving from hs rather
// than re-reading in.Metrics/in.Dataset directly guarantees this section
// never computes a second, independently-derived view of the same
// dataset — mirroring analytics/qoe.Calculate's identical
// single-computation discipline.
func buildProfitabilitySeries(in Input, hs HistoricalSeries) ProfitabilitySeries {
	if in.Ratios.Available && len(in.Ratios.History) > 0 {
		periods := make([]ProfitabilityPeriod, 0, len(in.Ratios.History))
		anyAvailable := false
		for _, pr := range in.Ratios.History {
			p := ProfitabilityPeriod{
				Period:          pr.Period,
				GrossMargin:     fromMetricValue(pr.GrossMargin.Value),
				OperatingMargin: fromMetricValue(pr.OperatingMargin.Value),
				EBITDAMargin:    fromMetricValue(pr.EBITDAMargin.Value),
				NetMargin:       fromMetricValue(pr.NetMargin.Value),
				ReturnOnAssets:  fromMetricValue(pr.ReturnOnAssets.Value),
				ReturnOnEquity:  fromMetricValue(pr.ReturnOnEquity.Value),
			}
			if p.GrossMargin.Available || p.OperatingMargin.Available || p.EBITDAMargin.Available ||
				p.NetMargin.Available || p.ReturnOnAssets.Available || p.ReturnOnEquity.Available {
				anyAvailable = true
			}
			periods = append(periods, p)
		}
		return ProfitabilitySeries{
			Available:      anyAvailable,
			FormulaVersion: in.Ratios.FormulaVersion,
			Source:         "ratios",
			Periods:        periods,
		}
	}

	if !hs.Available {
		return ProfitabilitySeries{}
	}
	periods := make([]ProfitabilityPeriod, 0, len(hs.Periods))
	anyAvailable := false
	for _, s := range hs.Periods {
		p := ProfitabilityPeriod{
			Period:       s.Period,
			GrossMargin:  s.GrossMargin,
			EBITDAMargin: s.EBITDAMargin,
		}
		if p.GrossMargin.Available || p.EBITDAMargin.Available {
			anyAvailable = true
		}
		periods = append(periods, p)
	}
	return ProfitabilitySeries{
		Available:      anyAvailable,
		FormulaVersion: hs.FormulaVersion,
		Source:         "metrics",
		Periods:        periods,
	}
}

// buildLiquidityLeverageSeries populates LiquidityLeverageSeries entirely
// from in.Ratios.History — this section has no metrics-only fallback,
// since current/quick/cash ratios and every leverage ratio here require
// balance-sheet-derived figures analytics/ratios already computes and
// financial/metrics.Snapshot alone does not restate as ratios.
func buildLiquidityLeverageSeries(in Input) LiquidityLeverageSeries {
	if !in.Ratios.Available || len(in.Ratios.History) == 0 {
		return LiquidityLeverageSeries{}
	}
	periods := make([]LiquidityLeveragePeriod, 0, len(in.Ratios.History))
	for _, pr := range in.Ratios.History {
		periods = append(periods, LiquidityLeveragePeriod{
			Period:           pr.Period,
			CurrentRatio:     fromMetricValue(pr.CurrentRatio.Value),
			QuickRatio:       fromMetricValue(pr.QuickRatio.Value),
			CashRatio:        fromMetricValue(pr.CashRatio.Value),
			DebtToEquity:     fromMetricValue(pr.DebtToEquity.Value),
			DebtToAssets:     fromMetricValue(pr.DebtToAssets.Value),
			DebtToEBITDA:     fromMetricValue(pr.DebtToEBITDA.Value),
			NetDebtToEBITDA:  fromMetricValue(pr.NetDebtToEBITDA.Value),
			InterestCoverage: fromMetricValue(pr.InterestCoverage.Value),
		})
	}
	return LiquidityLeverageSeries{
		Available:      true,
		FormulaVersion: in.Ratios.FormulaVersion,
		Periods:        periods,
	}
}

// buildCashFlowSeries populates CashFlowSeries from in.CashFlow.History
// (for EBITDA/OperatingCashFlow/Capex/FreeCashFlow) joined by Period
// against in.CashFlow.Conversion (for the two conversion ratios), plus the
// most recent CashRunway reading.
func buildCashFlowSeries(in Input) CashFlowSeries {
	if !in.CashFlow.Available || len(in.CashFlow.History) == 0 {
		return CashFlowSeries{}
	}
	conversionByPeriod := make(map[string]cashflowConversion, len(in.CashFlow.Conversion))
	for _, c := range in.CashFlow.Conversion {
		conversionByPeriod[string(c.Period)] = cashflowConversion{
			toOCF: fromMetricValue(c.EBITDAToOperatingCashFlow),
			toFCF: fromMetricValue(c.EBITDAToFreeCashFlow),
		}
	}

	periods := make([]CashFlowPeriod, 0, len(in.CashFlow.History))
	for _, b := range in.CashFlow.History {
		conv := conversionByPeriod[string(b.Period)]
		periods = append(periods, CashFlowPeriod{
			Period:                    b.Period,
			EBITDA:                    fromMetricValue(b.EBITDA),
			OperatingCashFlow:         Value{Available: b.OperatingCashFlow.Available, Amount: b.OperatingCashFlow.Value},
			Capex:                     Value{Available: b.Capex.Available, Amount: b.Capex.Value},
			FreeCashFlow:              Value{Available: b.FreeCashFlow.Available, Amount: b.FreeCashFlow.Value},
			EBITDAToOperatingCashFlow: conv.toOCF,
			EBITDAToFreeCashFlow:      conv.toFCF,
		})
	}

	result := CashFlowSeries{
		Available:      true,
		FormulaVersion: in.CashFlow.FormulaVersion,
		Periods:        periods,
	}
	if in.CashFlow.CashRunway.Available {
		result.MonthsOfRunway = fromMetricValue(in.CashFlow.CashRunway.MonthsOfRunway)
	}
	return result
}

// cashflowConversion holds the two conversion ratios for one period, keyed
// by period string in buildCashFlowSeries's join map.
type cashflowConversion struct {
	toOCF Value
	toFCF Value
}

// buildWorkingCapitalSeries populates WorkingCapitalSeries entirely from
// in.WorkingCapital.
func buildWorkingCapitalSeries(in Input) WorkingCapitalSeries {
	if !in.WorkingCapital.Available || len(in.WorkingCapital.History) == 0 {
		return WorkingCapitalSeries{}
	}
	periods := make([]WorkingCapitalPeriod, 0, len(in.WorkingCapital.History))
	for _, pn := range in.WorkingCapital.History {
		periods = append(periods, WorkingCapitalPeriod{
			Period:              pn.Period,
			NWC:                 Value{Available: pn.NWC.Available, Amount: pn.NWC.Value},
			NWCPercentOfRevenue: Value{Available: pn.NWCPercentOfRevenue.Available, Amount: pn.NWCPercentOfRevenue.Value},
		})
	}
	result := WorkingCapitalSeries{
		Available:      true,
		FormulaVersion: in.WorkingCapital.FormulaVersion,
		Periods:        periods,
		AverageNWC:     Value{Available: in.WorkingCapital.NWCStatistics.Average.Available, Amount: in.WorkingCapital.NWCStatistics.Average.Value},
	}
	if in.WorkingCapital.SuggestedPeg.Value.Available {
		result.SuggestedPeg = Value{Available: true, Amount: in.WorkingCapital.SuggestedPeg.Value.Value}
	}
	return result
}
