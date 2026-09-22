package workingcapital

import (
	"fmt"
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// revenueCodes mirrors financial/metrics' own total-revenue definition
// (see metrics.totalRevenue): every canonical revenue code summed together.
// Duplicated here (rather than depending on financial/metrics) since this
// package needs only the total, not metrics' full Snapshot machinery — see
// the package doc comment on not forcing an unrelated dependency.
var revenueCodes = []financial.Code{
	financial.CodeRevProduct,
	financial.CodeRevService,
	financial.CodeRevRecurring,
	financial.CodeRevOther,
}

// Calculate derives a full Result from in under opts. It never mutates
// in.Dataset and performs no I/O.
func Calculate(in Input, opts Options) Result {
	policy := resolveInclusionPolicy(opts.InclusionPolicy)
	result := Result{FormulaVersion: FormulaVersion, InclusionPolicy: policy}

	periods := in.Dataset.Periods()
	if len(periods) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoPeriods,
			Severity: SeverityError,
			Message:  "dataset has no periods; working-capital analysis requires at least one",
		})
		return result
	}
	result.Available = true

	if len(policy.AssetCodes) == 0 || len(policy.LiabilityCodes) == 0 {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueEmptyInclusionPolicy,
			Severity: SeverityWarning,
			Message:  "InclusionPolicy has no codes on one or both sides; OperatingCurrentAssets or OperatingCurrentLiabilities will be unavailable for every period",
		})
	}

	orderedPeriods, orderIssue := chronologicalPeriods(periods, in.PeriodMeta)
	if orderIssue != nil {
		result.Warnings = append(result.Warnings, *orderIssue)
	}

	idx := buildIndex(in.Dataset)

	history := make([]PeriodNWC, 0, len(orderedPeriods))
	for _, p := range orderedPeriods {
		history = append(history, computePeriodNWC(idx, policy, p))
	}
	result.History = history

	if !hasAnyRevenue(history) {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueNoRevenueData,
			Severity: SeverityWarning,
			Message:  "no revenue codes present for any period; NWCPercentOfRevenue is unavailable throughout",
		})
	}

	result.ExcludedCodes = excludedCodes(in.Dataset, policy)

	result.NWCStatistics = calculateStatistics(nwcSeries(history))
	result.NWCPercentOfRevenueStatistics = calculateStatistics(nwcPercentSeries(history))
	result.Trend = calculateTrend(history)

	if len(in.PeriodMeta) > 0 {
		result.SeasonalProfile = calculateSeasonalProfile(history, in.PeriodMeta)
	}

	suggestedPeg, pegIssue := calculateSuggestedPeg(history, opts)
	result.SuggestedPeg = suggestedPeg
	if pegIssue != nil {
		result.Warnings = append(result.Warnings, *pegIssue)
	}

	currentNWC, currentIssue := resolveCurrentNWC(in, idx, policy)
	if currentIssue != nil {
		result.Warnings = append(result.Warnings, *currentIssue)
	}
	result.PegComparison = buildPegComparison(currentNWC, suggestedPeg.Value)

	return result
}

// codeIndex is a lookup from (code, period) to amount, built once per
// Calculate call so per-period computation doesn't repeatedly scan
// Dataset.Items.
type codeIndex map[financial.Code]map[financial.Period]float64

func buildIndex(ds financial.FinancialDataset) codeIndex {
	idx := make(codeIndex)
	for _, item := range ds.Items {
		byPeriod, ok := idx[item.Code]
		if !ok {
			byPeriod = make(map[financial.Period]float64)
			idx[item.Code] = byPeriod
		}
		byPeriod[item.Period] = item.Amount
	}
	return idx
}

func (idx codeIndex) lookup(code financial.Code, period financial.Period) (float64, bool) {
	byPeriod, ok := idx[code]
	if !ok {
		return 0, false
	}
	v, ok := byPeriod[period]
	return v, ok
}

// sumResult mirrors summing a set of codes for one period.
type sumResult struct {
	total      float64
	anyPresent bool
	components []Component
}

func sumCodes(idx codeIndex, period financial.Period, codes []financial.Code) sumResult {
	var res sumResult
	for _, code := range codes {
		amount, ok := idx.lookup(code, period)
		if !ok {
			continue
		}
		res.anyPresent = true
		res.total += amount
		meta, _ := financial.LookupCode(code)
		res.components = append(res.components, Component{Code: string(code), Label: meta.Label, Amount: amount})
	}
	return res
}

// computePeriodNWC computes one period's full PeriodNWC under policy.
func computePeriodNWC(idx codeIndex, policy InclusionPolicy, period financial.Period) PeriodNWC {
	pnwc := PeriodNWC{Period: period}

	assets := sumCodes(idx, period, policy.AssetCodes)
	if assets.anyPresent {
		pnwc.OperatingCurrentAssets = AvailableValue(assets.total)
		pnwc.AssetComponents = assets.components
	}

	liabilities := sumCodes(idx, period, policy.LiabilityCodes)
	if liabilities.anyPresent {
		pnwc.OperatingCurrentLiabilities = AvailableValue(liabilities.total)
		pnwc.LiabilityComponents = liabilities.components
	}

	if pnwc.OperatingCurrentAssets.Available && pnwc.OperatingCurrentLiabilities.Available {
		pnwc.NWC = AvailableValue(pnwc.OperatingCurrentAssets.Value - pnwc.OperatingCurrentLiabilities.Value)
	}

	revenue := sumCodes(idx, period, revenueCodes)
	if revenue.anyPresent {
		pnwc.Revenue = AvailableValue(revenue.total)
	}

	if pnwc.NWC.Available && pnwc.Revenue.Available && pnwc.Revenue.Value != 0 {
		pnwc.NWCPercentOfRevenue = AvailableValue(pnwc.NWC.Value / pnwc.Revenue.Value)
	}

	return pnwc
}

// hasAnyRevenue reports whether at least one period in history has an
// available Revenue figure.
func hasAnyRevenue(history []PeriodNWC) bool {
	for _, p := range history {
		if p.Revenue.Available {
			return true
		}
	}
	return false
}

// excludedCodes lists every balance-sheet financial.Code present in ds that
// policy assigns to neither AssetCodes nor LiabilityCodes, sorted for
// determinism.
func excludedCodes(ds financial.FinancialDataset, policy InclusionPolicy) []financial.Code {
	included := codeSet(policy.AssetCodes)
	for c := range codeSet(policy.LiabilityCodes) {
		included[c] = struct{}{}
	}

	seen := make(map[financial.Code]struct{})
	var excluded []financial.Code
	for _, item := range ds.Items {
		meta, ok := financial.LookupCode(item.Code)
		if !ok || meta.StatementType != financial.StatementBalanceSheet {
			continue
		}
		if _, isIncluded := included[item.Code]; isIncluded {
			continue
		}
		if _, alreadySeen := seen[item.Code]; alreadySeen {
			continue
		}
		seen[item.Code] = struct{}{}
		excluded = append(excluded, item.Code)
	}
	sort.Slice(excluded, func(i, j int) bool { return excluded[i] < excluded[j] })
	return excluded
}

// chronologicalPeriods sorts periods by meta's FiscalYear/Type/
// SequenceInYear ordering, mirroring analytics/qoe.chronologicalPeriods and
// financial/metrics.sortOrderedPeriods. If meta is empty or any period is
// missing an entry, it returns periods in their original (lexical) order
// plus a descriptive warning Issue — this package never guesses
// chronological order from financial.Period's string value, but a missing
// order does not prevent per-period History from being computed (only
// Trend/SeasonalProfile depend on true chronological order).
func chronologicalPeriods(periods []financial.Period, meta map[financial.Period]PeriodInfo) ([]financial.Period, *Issue) {
	if len(meta) == 0 {
		return periods, &Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityWarning,
			Message:  "no PeriodMeta supplied; History uses dataset lexical order and Trend/SeasonalProfile are unavailable",
		}
	}

	type entry struct {
		period financial.Period
		info   PeriodInfo
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
			Message:  fmt.Sprintf("one or more periods have no PeriodMeta entry: %v; History uses dataset lexical order and Trend/SeasonalProfile are unavailable", missing),
		}
	}

	rank := func(t PeriodType) int {
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

// resolveCurrentNWC returns Input.CurrentNWC if Available, else the NWC
// derived from Dataset at Input.AsOf, else Unavailable with an explanatory
// Issue.
func resolveCurrentNWC(in Input, idx codeIndex, policy InclusionPolicy) (NWCValue, *Issue) {
	if in.CurrentNWC.Available {
		return in.CurrentNWC, nil
	}
	if in.AsOf != "" {
		pnwc := computePeriodNWC(idx, policy, in.AsOf)
		if pnwc.NWC.Available {
			return pnwc.NWC, nil
		}
	}
	return Unavailable(), &Issue{
		Code:     IssueCurrentNWCUnavailable,
		Severity: SeverityWarning,
		Message:  "no Input.CurrentNWC supplied and no derivable NWC at Input.AsOf; PegComparison is unavailable",
	}
}

// buildPegComparison computes ExcessDeficit = current - peg, available only
// if both sides are available.
func buildPegComparison(current, peg NWCValue) PegComparison {
	cmp := PegComparison{CurrentNWC: current, Peg: peg}
	if current.Available && peg.Available {
		cmp.ExcessDeficit = AvailableValue(current.Value - peg.Value)
	}
	return cmp
}
