package anomalies

import (
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// Calculate derives a full Result from in under opts. It never mutates
// in.Dataset, in.AccountGroups, or in.DiscretionaryCodes and performs no
// I/O.
func Calculate(in Input, opts Options) Result {
	thresholds := resolveThresholds(opts.Thresholds)
	result := Result{FormulaVersion: FormulaVersion, Thresholds: thresholds}

	periods := in.Dataset.Periods()
	if len(periods) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoPeriods,
			Severity: SeverityError,
			Message:  "dataset has no periods; anomaly detection requires at least one",
		})
		return result
	}
	result.Available = true

	orderedPeriods, chronological, orderIssue := chronologicalPeriods(periods, in.PeriodMeta)
	if orderIssue != nil {
		result.Warnings = append(result.Warnings, *orderIssue)
	}

	idx := buildIndex(in.Dataset)
	groups := groupIndex(in.AccountGroups)

	var anomalies []Anomaly

	// Single-period/order-independent rules: need no chronological order.
	anomalies = append(anomalies, detectUnexpectedNegativeAmounts(idx, periods, thresholds)...)
	anomalies = append(anomalies, detectRepeatedUnusualValues(idx, periods, thresholds)...)
	anomalies = append(anomalies, detectDuplicateLikeAmounts(idx, periods, thresholds)...)

	if chronological {
		anomalies = append(anomalies, detectAbsoluteAmountSpikes(idx, orderedPeriods, thresholds)...)
		anomalies = append(anomalies, detectPercentageChangeSpikes(idx, orderedPeriods, thresholds)...)
		anomalies = append(anomalies, detectSignFlips(idx, orderedPeriods, thresholds)...)
		anomalies = append(anomalies, detectExpenseOutpacingRevenue(idx, orderedPeriods, thresholds)...)
		anomalies = append(anomalies, detectMarginDeterioration(idx, orderedPeriods, thresholds)...)
		anomalies = append(anomalies, detectNewMaterialExpenseCategories(idx, orderedPeriods, thresholds)...)
		anomalies = append(anomalies, detectAccountDisappearedReappeared(idx, orderedPeriods)...)
		anomalies = append(anomalies, detectHighOwnerDiscretionaryShare(idx, orderedPeriods, in.DiscretionaryCodes, thresholds)...)
	} else {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityWarning,
			Message:  "chronological period order unavailable; every period-over-period and most-recent-period rule was skipped (spike, percentage-change spike, sign flip, expense-outpacing-revenue, margin deterioration, new material expense category, account disappeared/reappeared, high owner/discretionary share)",
		})
	}

	for i := range anomalies {
		labelGroup(&anomalies[i], groups)
	}
	sortAnomalies(anomalies, in.PeriodMeta)
	result.Anomalies = anomalies
	result.Summary = buildSummary(anomalies)

	return result
}

// codeIndex is a lookup from (code, period) to amount, built once per
// Calculate call so per-rule computation doesn't repeatedly scan
// Dataset.Items — mirrors revenuequality.codeIndex/workingcapital.codeIndex
// exactly.
type codeIndex map[financial.Code]map[financial.Period]financial.NormalizedItem

func buildIndex(ds financial.FinancialDataset) codeIndex {
	idx := make(codeIndex)
	for _, item := range ds.Items {
		byPeriod, ok := idx[item.Code]
		if !ok {
			byPeriod = make(map[financial.Period]financial.NormalizedItem)
			idx[item.Code] = byPeriod
		}
		byPeriod[item.Period] = item
	}
	return idx
}

// lookup returns the NormalizedItem for code/period, and whether one was
// reported at all — distinct from "reported as zero," which returns
// ok == true with Amount == 0.
func (idx codeIndex) lookup(code financial.Code, period financial.Period) (financial.NormalizedItem, bool) {
	byPeriod, ok := idx[code]
	if !ok {
		return financial.NormalizedItem{}, false
	}
	item, ok := byPeriod[period]
	return item, ok
}

// codes returns every distinct financial.Code present in idx, sorted for
// deterministic iteration order — never Go map order.
func (idx codeIndex) codes() []financial.Code {
	out := make([]financial.Code, 0, len(idx))
	for c := range idx {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// orderedPeriod pairs a financial.Period with its resolved PeriodInfo, used
// by every period-over-period rule once chronological order has been
// established.
type orderedPeriod struct {
	period financial.Period
	info   PeriodInfo
}

// chronologicalPeriods sorts periods by meta's FiscalYear/Type/
// SequenceInYear ordering, mirroring qoe.chronologicalPeriods/
// revenuequality.chronologicalPeriods exactly. chronological is false, and
// ordered is nil, if meta is empty or any period in periods lacks an entry —
// mirroring workingcapital/qoe's all-or-nothing rule (a partially-ordered
// series is not trustworthy for "which period is the baseline").
func chronologicalPeriods(periods []financial.Period, meta map[financial.Period]PeriodInfo) (ordered []orderedPeriod, chronological bool, issue *Issue) {
	if len(meta) == 0 {
		return nil, false, &Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityWarning,
			Message:  "no PeriodMeta supplied; chronological ordering unavailable",
		}
	}

	entries := make([]orderedPeriod, 0, len(periods))
	var missing []financial.Period
	for _, p := range periods {
		info, ok := meta[p]
		if !ok {
			missing = append(missing, p)
			continue
		}
		entries = append(entries, orderedPeriod{period: p, info: info})
	}
	if len(missing) > 0 {
		return nil, false, &Issue{
			Code:     IssuePeriodMissingFromMeta,
			Severity: SeverityWarning,
			Message:  "one or more periods have no PeriodMeta entry; chronological ordering unavailable",
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].info, entries[j].info
		if a.FiscalYear != b.FiscalYear {
			return a.FiscalYear < b.FiscalYear
		}
		if periodTypeRank(a.Type) != periodTypeRank(b.Type) {
			return periodTypeRank(a.Type) < periodTypeRank(b.Type)
		}
		return a.SequenceInYear < b.SequenceInYear
	})

	return entries, true, nil
}

// periodTypeRank places coarser period granularities before finer ones
// within the same fiscal year (fiscal year/YTD, then quarter, then month) —
// the single shared definition chronologicalPeriods and sortAnomalies both
// sort by, so a future PeriodType addition only needs to change one place.
func periodTypeRank(t PeriodType) int {
	switch t {
	case PeriodTypeFiscalYear, PeriodTypeYTD:
		return 0
	case PeriodTypeQuarter:
		return 1
	case PeriodTypeMonth:
		return 2
	default:
		return 3
	}
}

// groupIndex maps each financial.Code to the Name of the first AccountGroup
// in groups that lists it — see AccountGroup's doc comment on precedence.
func groupIndex(groups []AccountGroup) map[financial.Code]string {
	idx := make(map[financial.Code]string)
	for _, g := range groups {
		if g.Name == "" {
			continue
		}
		for _, c := range g.Codes {
			if _, ok := idx[c]; !ok {
				idx[c] = g.Name
			}
		}
	}
	return idx
}

// labelGroup sets a.Group from groups when a.Account is covered, and is a
// no-op otherwise (including when Account is empty, e.g. a synthetic-metric
// anomaly).
func labelGroup(a *Anomaly, groups map[financial.Code]string) {
	if a.Account == "" {
		return
	}
	if name, ok := groups[a.Account]; ok {
		a.Group = name
	}
}

// ruleOrder gives each RuleCode a fixed rank matching its declaration order
// in types.go, for deterministic sorting — never Go map order and never
// dependent on which rule happened to append its findings first in
// Calculate.
var ruleOrder = map[RuleCode]int{
	RuleAbsoluteAmountSpike:          0,
	RulePercentageChangeSpike:        1,
	RuleExpenseOutpacingRevenue:      2,
	RuleMarginDeterioration:          3,
	RuleNewMaterialExpenseCategory:   4,
	RuleAccountDisappearedReappeared: 5,
	RuleRepeatedUnusualValue:         6,
	RuleSignFlip:                     7,
	RuleDuplicateLikeAmounts:         8,
	RuleHighOwnerDiscretionaryShare:  9,
	RuleUnexpectedNegativeAmount:     10,
}

// sortAnomalies sorts anomalies by Period (chronologically when meta covers
// every listed period, else dataset lexical order), then by RuleCode's
// declaration order, then by Account string — see Result.Anomalies' doc
// comment. Sorts in place.
func sortAnomalies(anomalies []Anomaly, meta map[financial.Period]PeriodInfo) {
	periodLess := func(a, b financial.Period) bool {
		infoA, okA := meta[a]
		infoB, okB := meta[b]
		if okA && okB {
			if infoA.FiscalYear != infoB.FiscalYear {
				return infoA.FiscalYear < infoB.FiscalYear
			}
			if periodTypeRank(infoA.Type) != periodTypeRank(infoB.Type) {
				return periodTypeRank(infoA.Type) < periodTypeRank(infoB.Type)
			}
			if infoA.SequenceInYear != infoB.SequenceInYear {
				return infoA.SequenceInYear < infoB.SequenceInYear
			}
			return a < b
		}
		return a < b
	}

	sort.SliceStable(anomalies, func(i, j int) bool {
		ai, aj := anomalies[i], anomalies[j]
		if ai.Period != aj.Period {
			return periodLess(ai.Period, aj.Period)
		}
		if ruleOrder[ai.Code] != ruleOrder[aj.Code] {
			return ruleOrder[ai.Code] < ruleOrder[aj.Code]
		}
		if ai.Account != aj.Account {
			return ai.Account < aj.Account
		}
		return ai.MetricLabel < aj.MetricLabel
	})
}

// anomalySeverityOrder gives each AnomalySeverity a fixed rank (info,
// warning, critical) for Summary.BySeverity's documented order.
var anomalySeverityOrder = map[AnomalySeverity]int{
	AnomalySeverityInfo:     0,
	AnomalySeverityWarning:  1,
	AnomalySeverityCritical: 2,
}

// buildSummary rolls up anomalies into a Summary — see Summary's doc
// comment for the exact ordering of ByRule/BySeverity.
func buildSummary(anomalies []Anomaly) Summary {
	s := Summary{Total: len(anomalies), ReviewRecommended: len(anomalies) > 0}

	ruleCounts := make(map[RuleCode]int)
	severityCounts := make(map[AnomalySeverity]int)
	for _, a := range anomalies {
		ruleCounts[a.Code]++
		severityCounts[a.Severity]++
	}

	ruleCodes := make([]RuleCode, 0, len(ruleCounts))
	for c := range ruleCounts {
		ruleCodes = append(ruleCodes, c)
	}
	sort.Slice(ruleCodes, func(i, j int) bool { return ruleOrder[ruleCodes[i]] < ruleOrder[ruleCodes[j]] })
	for _, c := range ruleCodes {
		s.ByRule = append(s.ByRule, RuleCount{Code: c, Count: ruleCounts[c]})
	}

	severities := make([]AnomalySeverity, 0, len(severityCounts))
	for sev := range severityCounts {
		severities = append(severities, sev)
	}
	sort.Slice(severities, func(i, j int) bool { return anomalySeverityOrder[severities[i]] < anomalySeverityOrder[severities[j]] })
	for _, sev := range severities {
		s.BySeverity = append(s.BySeverity, SeverityCount{Severity: sev, Count: severityCounts[sev]})
	}

	return s
}

// provenanceFor builds a Provenance from an observed NormalizedItem and an
// optional baseline NormalizedItem (baselineOK false when there is no
// single-period baseline for this rule). Returns the zero Provenance
// (omitted from JSON) when neither item carried any Sources.
func provenanceFor(observed financial.NormalizedItem, baseline financial.NormalizedItem, baselineOK bool) Provenance {
	var p Provenance
	if len(observed.Sources) > 0 {
		p.ObservedSources = observed.Sources
	}
	if baselineOK && len(baseline.Sources) > 0 {
		p.BaselineSources = baseline.Sources
	}
	return p
}
