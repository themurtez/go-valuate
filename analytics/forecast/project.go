package forecast

import (
	"fmt"
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// projectScenario projects one Scenario forward from base across
// Input.Horizon periods, returning the full ScenarioResult plus every Issue
// produced (already scoped to scenario.Name — see Calculate).
func projectScenario(scenario Scenario, base PeriodPL, baseNWC ForecastValue, horizon int, labels []string, dscSource DebtServiceCoverageSource) (ScenarioResult, []Issue) {
	result := ScenarioResult{Name: scenario.Name, Type: scenario.Type}
	var issues []Issue

	a := scenario.Assumptions
	if len(a.Revenue) < horizon {
		issues = append(issues, Issue{
			Code: IssueMissingRevenueAssumption, Severity: SeverityWarning, Scenario: scenario.Name,
			Message: fmt.Sprintf("only %d of %d forecast periods have a revenue assumption; later periods' revenue is unavailable", len(a.Revenue), horizon),
		})
	}
	if len(a.COGS) < horizon {
		issues = append(issues, Issue{
			Code: IssueMissingCOGSAssumption, Severity: SeverityWarning, Scenario: scenario.Name,
			Message: fmt.Sprintf("only %d of %d forecast periods have a COGS assumption; later periods' COGS is unavailable", len(a.COGS), horizon),
		})
	}
	if len(a.Opex) < horizon {
		issues = append(issues, Issue{
			Code: IssueMissingOpexAssumption, Severity: SeverityWarning, Scenario: scenario.Name,
			Message: fmt.Sprintf("only %d of %d forecast periods have an opex assumption; later periods' opex is unavailable", len(a.Opex), horizon),
		})
	}

	periods := make([]PeriodPL, 0, horizon)
	wcPeriods := make([]WorkingCapitalPeriod, 0, horizon)
	cfPeriods := make([]CashFlowPeriod, 0, horizon)
	var trace []TraceStep

	prevPL := base
	prevNWC := baseNWC
	for i := 0; i < horizon; i++ {
		label := periodLabel(labels, i)
		periodNum := i + 1

		pl := PeriodPL{Period: label, PeriodNumber: periodNum}

		var revAssumption RevenuePeriodAssumption
		if i < len(a.Revenue) {
			revAssumption = a.Revenue[i]
		}
		revLines, totalRev, revIssues := projectRevenue(revAssumption, prevPL, scenario.Name, label)
		pl.RevenueLines = revLines
		pl.TotalRevenue = totalRev
		issues = append(issues, revIssues...)
		trace = append(trace, TraceStep{Period: label, Label: "Total Revenue", Value: totalRev.Value, Detail: revenueTraceDetail(revAssumption)})

		var cogsAssumption COGSPeriodAssumption
		if i < len(a.COGS) {
			cogsAssumption = a.COGS[i]
		}
		cogsLines, totalCOGS, cogsIssues := projectCOGS(cogsAssumption, prevPL, totalRev, scenario.Name, label)
		pl.COGSLines = cogsLines
		pl.TotalCOGS = totalCOGS
		issues = append(issues, cogsIssues...)
		trace = append(trace, TraceStep{Period: label, Label: "Total COGS", Value: totalCOGS.Value, Detail: cogsTraceDetail(cogsAssumption)})

		if pl.TotalRevenue.Available && pl.TotalCOGS.Available {
			pl.GrossProfit = AvailableValue(pl.TotalRevenue.Value - pl.TotalCOGS.Value)
			if pl.TotalRevenue.Value != 0 {
				pl.GrossMargin = AvailableValue(pl.GrossProfit.Value / pl.TotalRevenue.Value)
			}
			trace = append(trace, TraceStep{Period: label, Label: "Gross Profit", Value: pl.GrossProfit.Value, Detail: "Total Revenue - Total COGS"})
		}

		var opexAssumption OpexPeriodAssumption
		if i < len(a.Opex) {
			opexAssumption = a.Opex[i]
		}
		opexLines, totalOpex := projectOpex(opexAssumption, prevPL)
		pl.OpexLines = opexLines
		pl.TotalOpex = totalOpex
		for _, l := range opexLines {
			if l.Code == financial.CodeOpexOwnerComp {
				pl.OwnerCompensation = AvailableValue(l.Amount)
			}
		}
		trace = append(trace, TraceStep{Period: label, Label: "Total Opex", Value: totalOpex.Value, Detail: opexTraceDetail(opexAssumption)})

		if pl.GrossProfit.Available && pl.TotalOpex.Available {
			pl.EBIT = AvailableValue(pl.GrossProfit.Value - pl.TotalOpex.Value)
			trace = append(trace, TraceStep{Period: label, Label: "EBIT", Value: pl.EBIT.Value, Detail: "Gross Profit - Total Opex"})
		}

		var daAssumption DepreciationAmortizationAssumption
		if i < len(a.DepreciationAmortization) {
			daAssumption = a.DepreciationAmortization[i]
		}
		pl.Depreciation = daAssumption.Depreciation
		pl.Amortization = daAssumption.Amortization

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
			trace = append(trace, TraceStep{Period: label, Label: "EBITDA", Value: pl.EBITDA.Value, Detail: "EBIT + Depreciation + Amortization"})

			ownerComp := 0.0
			if pl.OwnerCompensation.Available {
				ownerComp = pl.OwnerCompensation.Value
			}
			pl.SDE = AvailableValue(pl.EBITDA.Value + ownerComp)
			if pl.TotalRevenue.Available && pl.TotalRevenue.Value != 0 {
				pl.SDEMargin = AvailableValue(pl.SDE.Value / pl.TotalRevenue.Value)
			}
			trace = append(trace, TraceStep{Period: label, Label: "SDE", Value: pl.SDE.Value, Detail: "EBITDA + Owner Compensation"})
		}

		var debtAssumption DebtServiceAssumption
		if i < len(a.DebtService) {
			debtAssumption = a.DebtService[i]
		}
		debtAssumption = resolveDebtServiceInterest(debtAssumption)
		pl.InterestExpense = debtAssumption.Interest
		pl.InterestIncome = Unavailable()

		if pl.EBIT.Available {
			interestExpense, interestIncome := 0.0, 0.0
			if pl.InterestExpense.Available {
				interestExpense = pl.InterestExpense.Value
			}
			if pl.InterestIncome.Available {
				interestIncome = pl.InterestIncome.Value
			}
			pl.PretaxIncome = AvailableValue(pl.EBIT.Value - interestExpense + interestIncome)
			trace = append(trace, TraceStep{Period: label, Label: "Pretax Income", Value: pl.PretaxIncome.Value, Detail: "EBIT - Interest Expense + Interest Income"})
		}

		var taxAssumption TaxPeriodAssumption
		if i < len(a.Tax) {
			taxAssumption = a.Tax[i]
		}
		pl.IncomeTax = projectTax(taxAssumption, pl.PretaxIncome)
		if pl.PretaxIncome.Available && pl.IncomeTax.Available {
			pl.NetIncome = AvailableValue(pl.PretaxIncome.Value - pl.IncomeTax.Value)
			trace = append(trace, TraceStep{Period: label, Label: "Net Income", Value: pl.NetIncome.Value, Detail: "Pretax Income - Income Tax"})
		}

		issues = append(issues, negativeValueIssues(pl, scenario.Name, label)...)

		periods = append(periods, pl)

		var wcAssumption WorkingCapitalPeriodAssumption
		hasWC := i < len(a.WorkingCapital)
		if hasWC {
			wcAssumption = a.WorkingCapital[i]
		}
		nwc := projectWorkingCapital(wcAssumption, hasWC, prevNWC, pl.TotalRevenue)
		wcPeriod := WorkingCapitalPeriod{Period: label, PeriodNumber: periodNum, NWC: nwc}
		if nwc.Available && prevNWC.Available {
			wcPeriod.ChangeInNWC = AvailableValue(nwc.Value - prevNWC.Value)
		}
		wcPeriods = append(wcPeriods, wcPeriod)

		var capexAssumption CapexAssumption
		if i < len(a.Capex) {
			capexAssumption = a.Capex[i]
		}
		cf := buildCashFlowPeriod(label, periodNum, pl, wcPeriod, capexAssumption, debtAssumption, dscSource)
		cfPeriods = append(cfPeriods, cf)

		prevPL = pl
		prevNWC = nwc
	}

	result.ProjectedPeriods = periods
	result.WorkingCapital = wcPeriods
	result.CashFlow = cfPeriods
	result.Trace = trace
	result.Warnings = issues
	return result, issues
}

// periodLabel returns labels[i] if present and non-empty, otherwise a
// generated "Period N" fallback (1-based) — mirroring dcf.ForecastPeriod's
// "label is display-only, never parsed" convention.
func periodLabel(labels []string, i int) string {
	if i < len(labels) && labels[i] != "" {
		return labels[i]
	}
	return fmt.Sprintf("Period %d", i+1)
}

// lineItemsByCode indexes a PeriodPL's line items (of one section — revenue,
// COGS, or opex) by financial.Code for base-period lookup during growth-rate
// compounding.
func lineItemsByCode(items []LineItem) map[financial.Code]float64 {
	m := make(map[financial.Code]float64, len(items))
	for _, it := range items {
		m[it.Code] = it.Amount
	}
	return m
}

// projectRevenue derives one period's revenue LineItems and TotalRevenue
// from assumption and the prior period's PeriodPL.
//
// Every revenue financial.Code present in prior.RevenueLines is projected
// individually: a CodeOverrides entry for that code takes precedence,
// otherwise the aggregate assumption.Method/GrowthRate applies. Under
// RevenueMethodFixedAmount, the aggregate FixedAmount is spread across
// codes without their own override, proportionally to those codes' share
// of the prior period's total revenue among non-overridden codes — so a
// caller supplying one total-revenue figure still gets a sensible
// code-level breakdown rather than dumping the whole amount onto a single
// synthetic line. A code newly introduced via CodeOverrides that was not
// present in prior.RevenueLines is projected using FixedAmount directly (no
// prior-period base to grow from) or skipped for GrowthRate (there is
// nothing to compound; this is reported as an advisory Issue).
func projectRevenue(assumption RevenuePeriodAssumption, prior PeriodPL, scenarioName, label string) ([]LineItem, ForecastValue, []Issue) {
	priorByCode := lineItemsByCode(prior.RevenueLines)
	overrides := revenueOverrideIndex(assumption.CodeOverrides)

	if len(priorByCode) == 0 && len(overrides) == 0 {
		return nil, Unavailable(), []Issue{{
			Code: IssueNoBaseRevenue, Severity: SeverityWarning, Scenario: scenarioName, Period: label,
			Message: "no base-period revenue and no CodeOverrides supplied; revenue is unavailable for this period",
		}}
	}

	var issues []Issue
	var lines []LineItem
	total := 0.0

	// Codes carried forward from the prior period, using either their
	// override or the aggregate rate.
	nonOverriddenPriorTotal := 0.0
	for _, code := range sortedCodesByAmount(priorByCode) {
		if _, overridden := overrides[code]; overridden {
			continue
		}
		nonOverriddenPriorTotal += priorByCode[code]
	}
	for _, code := range sortedCodesByAmount(priorByCode) {
		priorAmount := priorByCode[code]
		var amount float64
		if ov, ok := overrides[code]; ok {
			amount = applyRevenueMethod(ov.Method, ov.GrowthRate, ov.FixedAmount, priorAmount)
		} else {
			amount = aggregateRevenueAmount(assumption, priorAmount, nonOverriddenPriorTotal)
		}
		meta, _ := financial.LookupCode(code)
		lines = append(lines, LineItem{Code: code, Label: meta.Label, Amount: amount})
		total += amount
	}

	// Override-only codes not present in the prior period at all.
	for _, code := range sortedOverrideCodesNotIn(overrides, priorByCode) {
		ov := overrides[code]
		if ov.Method == RevenueMethodFixedAmount {
			meta, _ := financial.LookupCode(code)
			amount := ov.FixedAmount
			lines = append(lines, LineItem{Code: code, Label: meta.Label, Amount: amount})
			total += amount
		} else {
			issues = append(issues, Issue{
				Code: IssueNoBaseRevenue, Severity: SeverityWarning, Scenario: scenarioName, Period: label,
				Message: fmt.Sprintf("CodeOverrides entry for %s has no base-period amount to grow from and Method is not fixed_amount; skipped", code),
			})
		}
	}

	sort.Slice(lines, func(i, j int) bool { return lines[i].Code < lines[j].Code })
	return lines, AvailableValue(total), issues
}

func aggregateRevenueAmount(a RevenuePeriodAssumption, priorAmount, nonOverriddenPriorTotal float64) float64 {
	if a.Method == RevenueMethodFixedAmount {
		if nonOverriddenPriorTotal == 0 {
			return 0
		}
		share := priorAmount / nonOverriddenPriorTotal
		return a.FixedAmount * share
	}
	return priorAmount * (1 + a.GrowthRate)
}

func applyRevenueMethod(method RevenueMethod, growthRate, fixedAmount, priorAmount float64) float64 {
	if method == RevenueMethodFixedAmount {
		return fixedAmount
	}
	return priorAmount * (1 + growthRate)
}

func revenueOverrideIndex(overrides []RevenueCodeAssumption) map[financial.Code]RevenueCodeAssumption {
	idx := make(map[financial.Code]RevenueCodeAssumption, len(overrides))
	for _, o := range overrides {
		if _, exists := idx[o.Code]; exists {
			continue // first entry wins — deterministic, caller-controlled precedence.
		}
		idx[o.Code] = o
	}
	return idx
}

// sortedCodesByAmount returns m's keys sorted ascending, for deterministic
// LineItem iteration order regardless of Go map order. Used across revenue,
// COGS, and opex projection alike.
func sortedCodesByAmount(m map[financial.Code]float64) []financial.Code {
	codes := make([]financial.Code, 0, len(m))
	for c := range m {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	return codes
}

func sortedOverrideCodesNotIn(overrides map[financial.Code]RevenueCodeAssumption, prior map[financial.Code]float64) []financial.Code {
	var codes []financial.Code
	for c := range overrides {
		if _, ok := prior[c]; !ok {
			codes = append(codes, c)
		}
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	return codes
}

func revenueTraceDetail(a RevenuePeriodAssumption) string {
	if a.Method == RevenueMethodFixedAmount {
		return fmt.Sprintf("fixed amount %v, spread across revenue codes proportionally", a.FixedAmount)
	}
	return fmt.Sprintf("prior period revenue x (1 + %v)", a.GrowthRate)
}

// projectCOGS derives one period's COGS LineItems and TotalCOGS.
//
// Unlike revenue/opex, COGS has no per-financial.Code CodeOverrides (see
// COGSPeriodAssumption's doc comment): the aggregate figure — however
// derived (gross-margin target, own growth rate, or fixed amount) — is
// always spread across the prior period's COGS codes proportionally to
// their share of prior total COGS, exactly mirroring
// aggregateRevenueAmount's fixed-amount proportional split. A period with
// no prior-period COGS lines and a nonzero target still produces a single
// synthetic CodeCogsOther line, since gross margin must be attributable to
// *some* code for the per-code LineItems to sum to TotalCOGS.
func projectCOGS(assumption COGSPeriodAssumption, prior PeriodPL, totalRevenue ForecastValue, scenarioName, label string) ([]LineItem, ForecastValue, []Issue) {
	target, ok := cogsTarget(assumption, prior, totalRevenue)
	if !ok {
		return nil, Unavailable(), []Issue{{
			Code: IssueMissingCOGSAssumption, Severity: SeverityWarning, Scenario: scenarioName, Period: label,
			Message: "COGS could not be derived for this period (gross-margin method requires available total revenue; growth-rate method requires prior-period COGS)",
		}}
	}

	var issues []Issue
	if assumption.Method == COGSMethodGrossMarginPercent && assumption.GrossMarginPercent == 0 {
		issues = append(issues, Issue{
			Code: IssueImpliedZeroGrossMargin, Severity: SeverityWarning, Scenario: scenarioName, Period: label,
			Message: "gross_margin_percent is 0 (or unset), implying COGS consumes 100% of revenue",
		})
	}

	// Sum priorTotal directly over prior.COGSLines (already in a fixed,
	// sorted order — see lines 385/476's identical sort) rather than over
	// the priorByCode map: floating-point addition is not associative, so
	// re-summing a map in Go's randomized iteration order would make
	// priorTotal (and therefore every target*share line below) vary in its
	// last bit from run to run on identical input — see
	// determinism_test.go's TestCalculate_DeterministicAcrossMapOrdering,
	// which caught exactly this before this fix.
	priorByCode := lineItemsByCode(prior.COGSLines)
	priorTotal := 0.0
	for _, l := range prior.COGSLines {
		priorTotal += l.Amount
	}

	var lines []LineItem
	if priorTotal == 0 {
		meta, _ := financial.LookupCode(financial.CodeCogsOther)
		lines = []LineItem{{Code: financial.CodeCogsOther, Label: meta.Label, Amount: target}}
	} else {
		for _, code := range sortedCodesByAmount(priorByCode) {
			share := priorByCode[code] / priorTotal
			meta, _ := financial.LookupCode(code)
			lines = append(lines, LineItem{Code: code, Label: meta.Label, Amount: target * share})
		}
	}

	return lines, AvailableValue(target), issues
}

// cogsTarget resolves the aggregate COGS figure for the period, and whether
// it could be derived at all.
func cogsTarget(a COGSPeriodAssumption, prior PeriodPL, totalRevenue ForecastValue) (float64, bool) {
	switch a.Method {
	case COGSMethodGrowthRate:
		if !prior.TotalCOGS.Available {
			return 0, false
		}
		return prior.TotalCOGS.Value * (1 + a.GrowthRate), true
	case COGSMethodFixedAmount:
		return a.FixedAmount, true
	default: // COGSMethodGrossMarginPercent, including the zero-value default.
		if !totalRevenue.Available {
			return 0, false
		}
		return totalRevenue.Value * (1 - a.GrossMarginPercent), true
	}
}

func cogsTraceDetail(a COGSPeriodAssumption) string {
	switch a.Method {
	case COGSMethodGrowthRate:
		return fmt.Sprintf("prior period COGS x (1 + %v)", a.GrowthRate)
	case COGSMethodFixedAmount:
		return fmt.Sprintf("fixed amount %v", a.FixedAmount)
	default:
		return fmt.Sprintf("Total Revenue x (1 - %v gross margin target)", a.GrossMarginPercent)
	}
}

// projectOpex derives one period's opex LineItems and TotalOpex, under the
// same aggregate-plus-per-code-override rule projectRevenue uses for
// revenue (see OpexPeriodAssumption's doc comment). Unlike revenue, opex
// projection never fails outright: an opex code with no prior-period amount
// and no override simply does not appear (opex, unlike revenue, has no
// "no base to grow from" failure mode worth reporting — a business
// legitimately may have zero of a given opex category).
func projectOpex(assumption OpexPeriodAssumption, prior PeriodPL) ([]LineItem, ForecastValue) {
	priorByCode := lineItemsByCode(prior.OpexLines)
	overrides := opexOverrideIndex(assumption.CodeOverrides)

	if len(priorByCode) == 0 && len(overrides) == 0 {
		return nil, Unavailable()
	}

	var lines []LineItem
	total := 0.0

	nonOverriddenPriorTotal := 0.0
	for _, code := range sortedCodesByAmount(priorByCode) {
		if _, overridden := overrides[code]; overridden {
			continue
		}
		nonOverriddenPriorTotal += priorByCode[code]
	}
	for _, code := range sortedCodesByAmount(priorByCode) {
		priorAmount := priorByCode[code]
		var amount float64
		if ov, ok := overrides[code]; ok {
			amount = applyOpexMethod(ov, priorAmount)
		} else {
			amount = aggregateOpexAmount(assumption, priorAmount, nonOverriddenPriorTotal)
		}
		meta, _ := financial.LookupCode(code)
		lines = append(lines, LineItem{Code: code, Label: meta.Label, Amount: amount})
		total += amount
	}

	for _, code := range sortedOpexOverrideCodesNotIn(overrides, priorByCode) {
		ov := overrides[code]
		if ov.Method == OpexMethodFixedAmount {
			meta, _ := financial.LookupCode(code)
			amount := ov.FixedAmount
			lines = append(lines, LineItem{Code: code, Label: meta.Label, Amount: amount})
			total += amount
		}
		// A growth-rate override with no prior-period base simply contributes
		// nothing — mirroring this function's "no failure mode for opex"
		// doc-comment rule, unlike revenue's IssueNoBaseRevenue.
	}

	sort.Slice(lines, func(i, j int) bool { return lines[i].Code < lines[j].Code })
	return lines, AvailableValue(total)
}

func aggregateOpexAmount(a OpexPeriodAssumption, priorAmount, nonOverriddenPriorTotal float64) float64 {
	if a.Method == OpexMethodFixedAmount {
		if nonOverriddenPriorTotal == 0 {
			return 0
		}
		share := priorAmount / nonOverriddenPriorTotal
		return a.FixedAmount * share
	}
	return priorAmount * (1 + a.GrowthRate)
}

func applyOpexMethod(ov OpexCodeAssumption, priorAmount float64) float64 {
	switch ov.Method {
	case OpexMethodFixedAmount:
		return ov.FixedAmount
	case OpexMethodExcludeAmount:
		return (priorAmount - ov.ExcludeFromBase) * (1 + ov.GrowthRate)
	default: // OpexMethodGrowthRate, including the zero-value default.
		return priorAmount * (1 + ov.GrowthRate)
	}
}

func opexOverrideIndex(overrides []OpexCodeAssumption) map[financial.Code]OpexCodeAssumption {
	idx := make(map[financial.Code]OpexCodeAssumption, len(overrides))
	for _, o := range overrides {
		if _, exists := idx[o.Code]; exists {
			continue
		}
		idx[o.Code] = o
	}
	return idx
}

func sortedOpexOverrideCodesNotIn(overrides map[financial.Code]OpexCodeAssumption, prior map[financial.Code]float64) []financial.Code {
	var codes []financial.Code
	for c := range overrides {
		if _, ok := prior[c]; !ok {
			codes = append(codes, c)
		}
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	return codes
}

func opexTraceDetail(a OpexPeriodAssumption) string {
	if a.Method == OpexMethodFixedAmount {
		return fmt.Sprintf("fixed amount %v, spread across opex codes proportionally", a.FixedAmount)
	}
	return fmt.Sprintf("prior period opex x (1 + %v)", a.GrowthRate)
}

// projectTax derives one period's income tax expense.
//
// TaxMethodPercentOfPretaxIncome floors at $0: this package never reports a
// negative tax expense (a "tax benefit") from a projected pretax loss,
// since modeling a realizable tax benefit requires assumptions (carryback
// availability, valuation allowances) far beyond a flat effective rate —
// a projected loss simply produces $0 projected tax under this method,
// leaving NetIncome equal to PretaxIncome (an aggressive-optimistic
// approximation the caller is expected to know) rather than a fabricated
// tax credit.
func projectTax(a TaxPeriodAssumption, pretaxIncome ForecastValue) ForecastValue {
	switch a.Method {
	case TaxMethodFixedAmount:
		return AvailableValue(a.FixedAmount)
	default: // TaxMethodPercentOfPretaxIncome, including the zero-value default.
		if !pretaxIncome.Available {
			return Unavailable()
		}
		if a.TaxRate == 0 {
			return Unavailable()
		}
		tax := pretaxIncome.Value * a.TaxRate
		if tax < 0 {
			tax = 0
		}
		return AvailableValue(tax)
	}
}

// projectWorkingCapital derives one period's operating NWC. hasAssumption
// distinguishes "no WorkingCapitalPeriodAssumption supplied for this
// period" (NWC left Unavailable, per Assumptions.WorkingCapital's doc
// comment) from "an assumption was supplied, using the zero-value
// WorkingCapitalMethodPercentOfRevenue default" (0% of revenue — a
// deliberate, if unusual, assumption, matching this package's general
// "the zero value is a legal input, not a missing one" convention for
// every other Method type here).
func projectWorkingCapital(a WorkingCapitalPeriodAssumption, hasAssumption bool, priorNWC, totalRevenue ForecastValue) ForecastValue {
	if !hasAssumption {
		return Unavailable()
	}
	switch a.Method {
	case WorkingCapitalMethodFixedAmount:
		return AvailableValue(a.FixedAmount)
	case WorkingCapitalMethodHeldFlat:
		return priorNWC
	default: // WorkingCapitalMethodPercentOfRevenue, including the zero-value default.
		if !totalRevenue.Available {
			return Unavailable()
		}
		return AvailableValue(totalRevenue.Value * a.PercentOfRevenue)
	}
}

// resolveDebtServiceInterest returns a with Interest recomputed as
// BeginningBalance x InterestRate whenever both are available, per
// DebtServiceAssumption's doc comment; otherwise a is returned unchanged
// (Interest stays exactly as supplied, including Unavailable).
func resolveDebtServiceInterest(a DebtServiceAssumption) DebtServiceAssumption {
	if a.InterestRate.Available && a.BeginningBalance.Available {
		a.Interest = AvailableValue(a.BeginningBalance.Value * a.InterestRate.Value)
	}
	return a
}

// buildCashFlowPeriod derives one period's CashFlowPeriod from its already-
// computed PeriodPL, WorkingCapitalPeriod, and capex/debt-service
// assumptions.
func buildCashFlowPeriod(label string, periodNum int, pl PeriodPL, wc WorkingCapitalPeriod, capex CapexAssumption, debt DebtServiceAssumption, dscSource DebtServiceCoverageSource) CashFlowPeriod {
	cf := CashFlowPeriod{Period: label, PeriodNumber: periodNum, Capex: capex.Capex, DebtService: debt}

	if pl.EBITDA.Available && wc.ChangeInNWC.Available {
		tax := 0.0
		if pl.IncomeTax.Available {
			tax = pl.IncomeTax.Value
		}
		cf.OperatingCashFlow = AvailableValue(pl.EBITDA.Value - wc.ChangeInNWC.Value - tax)
	}

	if cf.OperatingCashFlow.Available {
		capexAmount := 0.0
		if capex.Capex.Available {
			capexAmount = capex.Capex.Value
		}
		cf.FreeCashFlow = AvailableValue(cf.OperatingCashFlow.Value - capexAmount)
	}

	total := debt.Total()
	if cf.FreeCashFlow.Available && total.Available {
		cf.FreeCashFlowToOwner = AvailableValue(cf.FreeCashFlow.Value - total.Value)
	}

	if total.Available && total.Value != 0 {
		var numerator ForecastValue
		if dscSource == DSCSourceEBITDA {
			numerator = pl.EBITDA
		} else {
			numerator = cf.OperatingCashFlow
		}
		if numerator.Available {
			cf.DebtServiceCoverage = AvailableValue(numerator.Value / total.Value)
		}
	}

	return cf
}

// negativeValueIssues reports IssueNegativeProjectedValue for any of
// TotalRevenue/TotalCOGS/TotalOpex that projected negative — see that
// Issue's doc comment for why this is advisory, never clamped.
func negativeValueIssues(pl PeriodPL, scenarioName, label string) []Issue {
	var issues []Issue
	check := func(name string, v ForecastValue) {
		if v.Available && v.Value < 0 {
			issues = append(issues, Issue{
				Code: IssueNegativeProjectedValue, Severity: SeverityWarning, Scenario: scenarioName, Period: label,
				Message: fmt.Sprintf("%s projected negative (%v)", name, v.Value),
			})
		}
	}
	check("total revenue", pl.TotalRevenue)
	check("total COGS", pl.TotalCOGS)
	check("total opex", pl.TotalOpex)
	return issues
}
