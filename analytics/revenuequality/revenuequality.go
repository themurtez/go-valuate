package revenuequality

import (
	"fmt"
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// recurringCode is the single financial.Code this package treats as
// recurring revenue — see the package doc comment.
const recurringCode = financial.CodeRevRecurring

// nonRecurringCodes are the three financial.Code values this package sums
// into NonRecurringRevenue — see the package doc comment.
var nonRecurringCodes = []financial.Code{
	financial.CodeRevProduct,
	financial.CodeRevService,
	financial.CodeRevOther,
}

// revenueCodes is nonRecurringCodes plus recurringCode — this package's own
// total-revenue definition, mirroring financial/metrics' revenueBreakdown
// and workingcapital.revenueCodes exactly (see either for why this is
// duplicated rather than importing financial/metrics).
var revenueCodes = []financial.Code{
	financial.CodeRevProduct,
	financial.CodeRevService,
	financial.CodeRevRecurring,
	financial.CodeRevOther,
}

// Calculate derives a full Result from in under opts. It never mutates
// in.Dataset or in.CustomerRevenue and performs no I/O.
func Calculate(in Input, opts Options) Result {
	policy := resolvePolicy(in.Policy)
	thresholds := resolveThresholds(opts.Thresholds)
	result := Result{FormulaVersion: FormulaVersion, Policy: policy, Thresholds: thresholds}

	periods := in.Dataset.Periods()
	if len(periods) == 0 {
		result.Errors = append(result.Errors, Issue{
			Code:     IssueNoPeriods,
			Severity: SeverityError,
			Message:  "dataset has no periods; revenue-quality analysis requires at least one",
		})
		return result
	}
	result.Available = true

	orderedPeriods, orderIssue := chronologicalPeriods(periods, in.PeriodMeta)
	if orderIssue != nil {
		result.Warnings = append(result.Warnings, *orderIssue)
	}
	chronological := orderIssue == nil

	idx := buildIndex(in.Dataset)

	history := make([]PeriodRevenue, 0, len(orderedPeriods))
	for _, p := range orderedPeriods {
		history = append(history, computePeriodRevenue(idx, p))
	}
	result.TotalRevenueHistory = history

	if !hasAnyRevenue(history) {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueNoRevenueData,
			Severity: SeverityWarning,
			Message:  "no revenue codes present for any period; TotalRevenueHistory figures are unavailable throughout",
		})
	}

	result.RevenueStatistics = calculateStatistics(totalRevenueSeries(history))

	if chronological {
		result.RevenueTrend = calculateTrend(history)
		result.RevenueGrowth = calculateGrowthSeries(history)
		result.RevenueCAGR = calculateCAGR(history, in.PeriodMeta)
	} else {
		result.RevenueTrend = Trend{Direction: TrendUnavailable}
	}
	result.RevenueVolatility = calculateVolatility(history)

	if len(in.CustomerRevenue) == 0 {
		result.Warnings = append(result.Warnings, Issue{
			Code:     IssueNoCustomerData,
			Severity: SeverityWarning,
			Message:  "no CustomerRevenue supplied; every customer-level output is unavailable",
		})
	} else {
		customerIssues := validateCustomerRevenue(in.CustomerRevenue, in.Dataset)
		result.Warnings = append(result.Warnings, customerIssues...)

		validRows := filterValidCustomerRevenue(in.CustomerRevenue, in.Dataset)
		customerByPeriod := groupCustomerRevenueByPeriod(validRows)

		customerHistory := make([]CustomerPeriodTotal, 0, len(orderedPeriods))
		for _, p := range orderedPeriods {
			if rows, ok := customerByPeriod[p]; ok {
				customerHistory = append(customerHistory, computeCustomerPeriodTotal(p, rows))
			}
		}
		result.CustomerHistory = customerHistory

		if chronological {
			result.CustomerTransitions = calculateCustomerTransitions(orderedPeriods, customerByPeriod)
		}

		result.ConcentrationSummary = calculateConcentrationSummary(orderedPeriods, customerByPeriod, history, policy)
	}

	result.Flags = buildFlags(result, thresholds)

	return result
}

// codeIndex is a lookup from (code, period) to amount, built once per
// Calculate call so per-period computation doesn't repeatedly scan
// Dataset.Items — mirrors workingcapital.codeIndex exactly.
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

// computePeriodRevenue computes one period's full PeriodRevenue.
func computePeriodRevenue(idx codeIndex, period financial.Period) PeriodRevenue {
	pr := PeriodRevenue{Period: period}

	total := sumCodes(idx, period, revenueCodes)
	if total.anyPresent {
		pr.TotalRevenue = AvailableValue(total.total)
		pr.Components = total.components
	}

	recurring := sumCodes(idx, period, []financial.Code{recurringCode})
	if recurring.anyPresent {
		pr.RecurringRevenue = AvailableValue(recurring.total)
	}

	nonRecurring := sumCodes(idx, period, nonRecurringCodes)
	if nonRecurring.anyPresent {
		pr.NonRecurringRevenue = AvailableValue(nonRecurring.total)
	}

	if pr.RecurringRevenue.Available && pr.NonRecurringRevenue.Available && pr.TotalRevenue.Available && pr.TotalRevenue.Value != 0 {
		pr.RecurringPercent = AvailableValue(pr.RecurringRevenue.Value / pr.TotalRevenue.Value)
		pr.NonRecurringPercent = AvailableValue(pr.NonRecurringRevenue.Value / pr.TotalRevenue.Value)
	}

	return pr
}

// hasAnyRevenue reports whether at least one period in history has an
// available TotalRevenue figure.
func hasAnyRevenue(history []PeriodRevenue) bool {
	for _, p := range history {
		if p.TotalRevenue.Available {
			return true
		}
	}
	return false
}

// chronologicalPeriods sorts periods by meta's FiscalYear/Type/
// SequenceInYear ordering, mirroring workingcapital.chronologicalPeriods/
// analytics/qoe.chronologicalPeriods exactly. If meta is empty or any
// period is missing an entry, it returns periods in their original
// (lexical) order plus a descriptive warning Issue.
func chronologicalPeriods(periods []financial.Period, meta map[financial.Period]PeriodInfo) ([]financial.Period, *Issue) {
	if len(meta) == 0 {
		return periods, &Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityWarning,
			Message:  "no PeriodMeta supplied; history uses dataset lexical order and every ordering-dependent output is unavailable",
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
			Message:  fmt.Sprintf("one or more periods have no PeriodMeta entry: %v; history uses dataset lexical order and every ordering-dependent output is unavailable", missing),
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
