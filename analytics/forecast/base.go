package forecast

import (
	"sort"

	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
)

// codeIndex is a (code, period) -> amount index built once per Calculate
// call, mirroring financial/metrics.codeIndex and analytics/workingcapital.
// codeIndex exactly (see either for the identical rationale).
type codeIndex struct {
	amounts map[financial.Code]map[financial.Period]float64
}

func buildCodeIndex(ds financial.FinancialDataset) codeIndex {
	idx := codeIndex{amounts: make(map[financial.Code]map[financial.Period]float64)}
	for _, item := range ds.Items {
		periods, ok := idx.amounts[item.Code]
		if !ok {
			periods = make(map[financial.Period]float64)
			idx.amounts[item.Code] = periods
		}
		periods[item.Period] = item.Amount
	}
	return idx
}

func (idx codeIndex) lookup(code financial.Code, period financial.Period) (float64, bool) {
	periods, ok := idx.amounts[code]
	if !ok {
		return 0, false
	}
	v, ok := periods[period]
	return v, ok
}

// lineSum is the outcome of summing a set of codes for one period: the
// total, whether at least one code was present, and the individual
// LineItems found — mirroring financial/metrics.sumResult exactly (see its
// doc comment for the anyPresent-vs-zero rationale).
type lineSum struct {
	total      float64
	anyPresent bool
	lines      []LineItem
}

func sumCodesForPeriod(idx codeIndex, period financial.Period, codes []financial.Code) lineSum {
	var res lineSum
	for _, code := range codes {
		amount, ok := idx.lookup(code, period)
		if !ok {
			continue
		}
		res.anyPresent = true
		res.total += amount
		meta, _ := financial.LookupCode(code)
		res.lines = append(res.lines, LineItem{Code: code, Label: meta.Label, Amount: amount})
	}
	sort.Slice(res.lines, func(i, j int) bool { return res.lines[i].Code < res.lines[j].Code })
	return res
}

// chronologicallyLastPeriod returns the period in periods with the latest
// (FiscalYear, granularity-rank, SequenceInYear) ordering under meta. It
// requires every period in periods to have a meta entry; the caller
// (resolveBasePeriod) is responsible for having already validated that.
func chronologicallyLastPeriod(periods []financial.Period, meta map[financial.Period]PeriodInfo) financial.Period {
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
	last := periods[0]
	for _, p := range periods[1:] {
		a, b := meta[last], meta[p]
		switch {
		case b.FiscalYear != a.FiscalYear:
			if b.FiscalYear > a.FiscalYear {
				last = p
			}
		case rank(b.Type) != rank(a.Type):
			if rank(b.Type) > rank(a.Type) {
				last = p
			}
		case b.SequenceInYear > a.SequenceInYear:
			last = p
		}
	}
	return last
}

// resolveBasePeriod validates that meta covers every period in periods and
// returns the chronologically last one. Returns an error Issue (never a
// zero financial.Period) if periods is empty or meta is incomplete —
// Calculate treats either as blocking, per Input.PeriodMeta's doc comment.
func resolveBasePeriod(periods []financial.Period, meta map[financial.Period]PeriodInfo) (financial.Period, *Issue) {
	if len(meta) == 0 {
		return "", &Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityError,
			Message:  "no PeriodMeta supplied; the historical base period cannot be identified",
		}
	}
	var missing []financial.Period
	for _, p := range periods {
		if _, ok := meta[p]; !ok {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		return "", &Issue{
			Code:     IssueNoPeriodMeta,
			Severity: SeverityError,
			Message:  "one or more dataset periods have no PeriodMeta entry; the historical base period cannot be reliably identified",
		}
	}
	return chronologicallyLastPeriod(periods, meta), nil
}

// buildBasePL computes the historical base period's PeriodPL directly from
// idx, using the exact same formulas metrics/income_statement.go and
// metrics/balance_sheet.go use for EBIT/EBITDA/SDE/margins/net income — see
// those files for the canonical formula definitions this function mirrors
// (duplicated rather than depending on financial/metrics' full Snapshot
// machinery, mirroring analytics/workingcapital's identical "duplicate the
// formula, don't force the dependency" choice, now applied consistently
// across this package's own projection formulas in project.go).
func buildBasePL(idx codeIndex, period financial.Period) PeriodPL {
	pl := PeriodPL{Period: string(period), PeriodNumber: 0}

	rev := sumCodesForPeriod(idx, period, revenueCodes)
	pl.RevenueLines = rev.lines
	if rev.anyPresent {
		pl.TotalRevenue = AvailableValue(rev.total)
	}

	cogs := sumCodesForPeriod(idx, period, cogsCodes)
	pl.COGSLines = cogs.lines
	if cogs.anyPresent {
		pl.TotalCOGS = AvailableValue(cogs.total)
	}

	if pl.TotalRevenue.Available && pl.TotalCOGS.Available {
		pl.GrossProfit = AvailableValue(pl.TotalRevenue.Value - pl.TotalCOGS.Value)
		if pl.TotalRevenue.Value != 0 {
			pl.GrossMargin = AvailableValue(pl.GrossProfit.Value / pl.TotalRevenue.Value)
		}
	}

	opex := sumCodesForPeriod(idx, period, opexCodes)
	pl.OpexLines = opex.lines
	if opex.anyPresent {
		pl.TotalOpex = AvailableValue(opex.total)
	}
	if v, ok := idx.lookup(financial.CodeOpexOwnerComp, period); ok {
		pl.OwnerCompensation = AvailableValue(v)
	}

	if pl.GrossProfit.Available && pl.TotalOpex.Available {
		pl.EBIT = AvailableValue(pl.GrossProfit.Value - pl.TotalOpex.Value)
	}

	if v, ok := idx.lookup(financial.CodeDepreciation, period); ok {
		pl.Depreciation = AvailableValue(v)
	}
	if v, ok := idx.lookup(financial.CodeAmortization, period); ok {
		pl.Amortization = AvailableValue(v)
	}

	if pl.EBIT.Available {
		dep, amort := 0.0, 0.0
		if pl.Depreciation.Available {
			dep = pl.Depreciation.Value
		}
		if pl.Amortization.Available {
			amort = pl.Amortization.Value
		}
		pl.EBITDA = AvailableValue(pl.EBIT.Value + dep + amort)
		if pl.TotalRevenue.Available && pl.TotalRevenue.Value != 0 {
			pl.EBITDAMargin = AvailableValue(pl.EBITDA.Value / pl.TotalRevenue.Value)
		}

		ownerComp := 0.0
		if pl.OwnerCompensation.Available {
			ownerComp = pl.OwnerCompensation.Value
		}
		pl.SDE = AvailableValue(pl.EBITDA.Value + ownerComp)
		if pl.TotalRevenue.Available && pl.TotalRevenue.Value != 0 {
			pl.SDEMargin = AvailableValue(pl.SDE.Value / pl.TotalRevenue.Value)
		}
	}

	if v, ok := idx.lookup(financial.CodeInterestExpense, period); ok {
		pl.InterestExpense = AvailableValue(v)
	}
	if v, ok := idx.lookup(financial.CodeInterestIncome, period); ok {
		pl.InterestIncome = AvailableValue(v)
	}

	if pl.EBIT.Available {
		interestExpense, interestIncome := 0.0, 0.0
		if pl.InterestExpense.Available {
			interestExpense = pl.InterestExpense.Value
		}
		if pl.InterestIncome.Available {
			interestIncome = pl.InterestIncome.Value
		}
		pl.PretaxIncome = AvailableValue(pl.EBIT.Value - interestExpense + interestIncome)
	}

	if v, ok := idx.lookup(financial.CodeIncomeTax, period); ok {
		pl.IncomeTax = AvailableValue(v)
	}
	if pl.PretaxIncome.Available && pl.IncomeTax.Available {
		pl.NetIncome = AvailableValue(pl.PretaxIncome.Value - pl.IncomeTax.Value)
	}

	return pl
}

// buildBaseWorkingCapital sums policy.AssetCodes and policy.LiabilityCodes
// for period and returns their difference, available only if at least one
// code on each side was present in idx — mirroring
// workingcapital.computePeriodNWC's identical availability rule.
func buildBaseWorkingCapital(idx codeIndex, period financial.Period, policy workingcapital.InclusionPolicy) ForecastValue {
	assets := sumCodesForPeriod(idx, period, policy.AssetCodes)
	liabilities := sumCodesForPeriod(idx, period, policy.LiabilityCodes)
	if !assets.anyPresent || !liabilities.anyPresent {
		return Unavailable()
	}
	return AvailableValue(assets.total - liabilities.total)
}
