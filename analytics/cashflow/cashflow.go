package cashflow

import (
	"fmt"
	"sort"

	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// Calculate derives a full Result from in under opts. It never mutates any
// caller-owned input and performs no I/O.
//
// Calculate recomputes financial/metrics.Snapshot (for EBITDA) and
// analytics/workingcapital.Result (for the period-over-period NWC series)
// itself, directly from in.Dataset/in.PeriodMeta, rather than requiring the
// caller to pre-run either package — the same choice analytics/qoe and
// analytics/ratios already make for metrics.Calculate (see qoe.Calculate's
// doc comment for the full rationale: this guarantees History's EBITDA and
// working-capital figures are always derived from exactly one computation,
// never two independently-computed views that could silently drift apart).
func Calculate(in Input, opts Options) Result {
	thresholds := resolveThresholds(opts.Thresholds)
	result := Result{FormulaVersion: FormulaVersion, Thresholds: thresholds}

	periods := in.Dataset.Periods()
	if len(periods) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoPeriods,
			Severity: SeverityError,
			Message:  "dataset has no periods; cash-flow analysis requires at least one",
		})
		return result
	}
	result.Available = true

	if len(in.PeriodMeta) == 0 {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityWarning,
			Message:  "no PeriodMeta supplied; chronological ordering, change in working capital, trends, and cash runway are unavailable",
		})
	}

	orderedPeriods, orderIssue := chronologicalPeriods(periods, in.PeriodMeta)
	if orderIssue != nil {
		result.Warnings = append(result.Warnings, *orderIssue)
	}

	metricsResult := metrics.Calculate(in.Dataset, metrics.Options{PeriodMeta: in.PeriodMeta})
	nwcResult := workingcapital.Calculate(
		workingcapital.Input{Dataset: in.Dataset, PeriodMeta: workingCapitalPeriodMeta(in.PeriodMeta)},
		workingcapital.Options{InclusionPolicy: in.Policy},
	)

	if !hasAnyReportedOCF(in.OperatingCashFlow) {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueNoCashFlowStatement,
			Severity: SeverityWarning,
			Message:  "no reported Input.OperatingCashFlow supplied for any period; operating cash flow and everything downstream are unavailable unless Options.AllowEBITDAEstimate is set",
		})
	}

	nwcByPeriod := indexPeriodNWC(nwcResult.History)

	history := make([]Bridge, 0, len(orderedPeriods))
	var usedEstimate bool
	for i, p := range orderedPeriods {
		snap, _ := metricsResult.SnapshotFor(p)
		nwc := nwcByPeriod[p]

		var prevNWC *workingcapital.PeriodNWC
		if i > 0 {
			if pv, ok := nwcByPeriod[orderedPeriods[i-1]]; ok {
				prevNWC = &pv
			}
		}

		bridge := buildBridge(in, p, snap, nwc, prevNWC, opts.AllowEBITDAEstimate)
		if bridge.OperatingCashFlow.IsEstimate {
			usedEstimate = true
		}
		history = append(history, bridge)
	}
	result.History = history

	if usedEstimate {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueEstimatedFromEBITDA,
			Severity: SeverityWarning,
			Message:  "one or more periods' operating cash flow (and dependent free cash flow) was estimated from EBITDA minus change in net working capital rather than reported; see each Bridge.OperatingCashFlow.IsEstimate",
		})
	}

	result.Conversion = buildConversionRatios(history)

	result.OperatingCashFlowTrend = calculateTrend(extractOCF(history))
	result.FreeCashFlowTrend = calculateTrend(extractFCF(history))
	result.ConversionTrend = calculateTrend(extractConversion(result.Conversion))

	result.RecurringDrains = buildRecurringDrains(history)

	result.CashRunway = calculateCashRunway(in, orderedPeriods, history)

	result.Flags = buildFlags(result, thresholds)

	return result
}

// workingCapitalPeriodMeta converts in.PeriodMeta into
// analytics/workingcapital's own (structurally identical but distinctly
// named) PeriodInfo type, since that package defines its own type rather
// than aliasing financial/metrics' (see workingcapital's PeriodInfo doc
// comment).
func workingCapitalPeriodMeta(meta map[financial.Period]metrics.PeriodInfo) map[financial.Period]workingcapital.PeriodInfo {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[financial.Period]workingcapital.PeriodInfo, len(meta))
	for p, info := range meta {
		out[p] = workingcapital.PeriodInfo{
			Type:           workingcapital.PeriodType(info.Type),
			FiscalYear:     info.FiscalYear,
			SequenceInYear: info.SequenceInYear,
		}
	}
	return out
}

// hasAnyReportedOCF reports whether ocf has at least one Available entry.
func hasAnyReportedOCF(ocf map[financial.Period]CashFlowValue) bool {
	for _, v := range ocf {
		if v.Available {
			return true
		}
	}
	return false
}

// indexPeriodNWC builds a period -> PeriodNWC lookup from
// workingcapital.Result.History, so buildBridge can fetch the current and
// preceding period's NWC by key rather than by re-scanning the slice per
// period.
func indexPeriodNWC(history []workingcapital.PeriodNWC) map[financial.Period]workingcapital.PeriodNWC {
	idx := make(map[financial.Period]workingcapital.PeriodNWC, len(history))
	for _, p := range history {
		idx[p.Period] = p
	}
	return idx
}

// buildBridge computes one period's full Bridge.
func buildBridge(in Input, period financial.Period, snap metrics.Snapshot, nwc workingcapital.PeriodNWC, prevNWC *workingcapital.PeriodNWC, allowEstimate bool) Bridge {
	b := Bridge{
		Period: period,
		EBITDA: snap.EBITDA,
		Capex:  in.Capex[period],
	}

	if prevNWC != nil && nwc.NWC.Available && prevNWC.NWC.Available {
		b.ChangeInNWC = metrics.AvailableValue(nwc.NWC.Value - prevNWC.NWC.Value)
	}

	b.OperatingCashFlow = resolveOperatingCashFlow(in.OperatingCashFlow[period], b.EBITDA, b.ChangeInNWC, allowEstimate)
	b.FreeCashFlow = calculateFreeCashFlow(b.OperatingCashFlow, b.Capex)

	b.DebtService = in.DebtService[period]
	b.FreeCashFlowToFirm = freeCashFlowToFirm(b.FreeCashFlow, b.DebtService)
	b.FreeCashFlowToOwner = freeCashFlowToOwner(b.FreeCashFlow, b.DebtService)

	b.OwnerDistributions = in.OwnerDistributions[period]
	b.CashTaxesPaid = in.CashTaxesPaid[period]

	return b
}

// resolveOperatingCashFlow returns reported when Available, otherwise an
// EBITDA-based estimate (EBITDA - change in NWC) when allowEstimate is set
// and both ebitda and changeInNWC are available, otherwise Unavailable().
// This is the only place this package ever derives a cash-flow figure from
// EBITDA — see the package doc comment and Options.AllowEBITDAEstimate.
func resolveOperatingCashFlow(reported CashFlowValue, ebitda, changeInNWC metrics.MetricValue, allowEstimate bool) CashFlowValue {
	if reported.Available {
		return reported
	}
	if !allowEstimate || !ebitda.Available || !changeInNWC.Available {
		return Unavailable()
	}
	return Estimated(
		ebitda.Value-changeInNWC.Value,
		"EBITDA - change in net working capital; no reported operating cash flow supplied for this period",
	)
}

// calculateFreeCashFlow is OperatingCashFlow.Value - Capex.Value. Available
// only when OperatingCashFlow is available. If Capex is unavailable, capex
// is treated as zero and the result is marked as an estimate noting capex
// was assumed zero, so a caller can never mistake "capex not supplied" for
// "capex confirmed to be zero."
func calculateFreeCashFlow(ocf, capex CashFlowValue) CashFlowValue {
	if !ocf.Available {
		return Unavailable()
	}
	if capex.Available {
		fcf := CashFlowValue{Available: true, Value: ocf.Value - capex.Value}
		if ocf.IsEstimate {
			fcf.IsEstimate = true
			fcf.EstimateBasis = "operating cash flow component is estimated; see Bridge.OperatingCashFlow.EstimateBasis"
		}
		return fcf
	}
	return CashFlowValue{
		Available:     true,
		Value:         ocf.Value,
		IsEstimate:    true,
		EstimateBasis: "capex not supplied for this period; assumed zero",
	}
}

// freeCashFlowToFirm is FreeCashFlow.Value + DebtService.Interest.Value,
// available only when both are available.
func freeCashFlowToFirm(fcf CashFlowValue, ds DebtServiceFigure) metrics.MetricValue {
	if !fcf.Available || !ds.Interest.Available {
		return metrics.Unavailable()
	}
	return metrics.AvailableValue(fcf.Value + ds.Interest.Value)
}

// freeCashFlowToOwner is FreeCashFlow.Value - DebtService.Total().Value,
// available only when both are available.
func freeCashFlowToOwner(fcf CashFlowValue, ds DebtServiceFigure) metrics.MetricValue {
	total := ds.Total()
	if !fcf.Available || !total.Available {
		return metrics.Unavailable()
	}
	return metrics.AvailableValue(fcf.Value - total.Value)
}

// buildConversionRatios computes one ConversionRatios per Bridge in
// history, in the same order.
func buildConversionRatios(history []Bridge) []ConversionRatios {
	out := make([]ConversionRatios, 0, len(history))
	for _, b := range history {
		cr := ConversionRatios{Period: b.Period}
		if b.EBITDA.Available && b.EBITDA.Value != 0 {
			if b.OperatingCashFlow.Available {
				cr.EBITDAToOperatingCashFlow = metrics.AvailableValue(b.OperatingCashFlow.Value / b.EBITDA.Value)
			}
			if b.FreeCashFlow.Available {
				cr.EBITDAToFreeCashFlow = metrics.AvailableValue(b.FreeCashFlow.Value / b.EBITDA.Value)
			}
		}
		out = append(out, cr)
	}
	return out
}

// chronologicalPeriods sorts periods by meta's FiscalYear/Type/
// SequenceInYear ordering, mirroring workingcapital.chronologicalPeriods
// exactly (see that function's doc comment for the full no-guessing
// rationale).
func chronologicalPeriods(periods []financial.Period, meta map[financial.Period]metrics.PeriodInfo) ([]financial.Period, *Issue) {
	if len(meta) == 0 {
		return periods, &Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityWarning,
			Message:  "no PeriodMeta supplied; History uses dataset lexical order and ordering-dependent outputs are unavailable",
		}
	}

	type entry struct {
		period financial.Period
		info   metrics.PeriodInfo
	}
	entries := make([]entry, 0, len(periods))
	var missing []financial.Period
	for _, p := range periods {
		info, ok := meta[p]
		if !ok {
			missing = append(missing, p)
			continue
		}
		entries = append(entries, entry{period: p, info: info})
	}
	if len(missing) > 0 {
		return periods, &Issue{
			Code:     IssuePeriodMissingFromMeta,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("one or more periods have no PeriodMeta entry: %v; History uses dataset lexical order and ordering-dependent outputs are unavailable", missing),
		}
	}

	rank := func(t metrics.PeriodType) int {
		switch t {
		case metrics.PeriodTypeFiscalYear, metrics.PeriodTypeYTD:
			return 0
		case metrics.PeriodTypeQuarter:
			return 1
		case metrics.PeriodTypeMonth:
			return 2
		default:
			return 3
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].info, entries[j].info
		if a.FiscalYear != b.FiscalYear {
			return a.FiscalYear < b.FiscalYear
		}
		if rank(a.Type) != rank(b.Type) {
			return rank(a.Type) < rank(b.Type)
		}
		return a.SequenceInYear < b.SequenceInYear
	})

	ordered := make([]financial.Period, len(entries))
	for i, e := range entries {
		ordered[i] = e.period
	}
	return ordered, nil
}
