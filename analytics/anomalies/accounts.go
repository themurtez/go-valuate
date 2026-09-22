package anomalies

import "github.com/themurtez/go-valuate/financial"

// revenueCodes mirrors financial/metrics' own total-revenue definition
// (every financial.CategoryRevenue code) — see
// workingcapital.revenueCodes/revenuequality.revenueCodes for the same
// duplicated-rather-than-imported convention. Unlike those two packages,
// this package derives the list from financial.CodesByCategory at package
// init rather than hand-listing each Code, since every rule here (expense-
// outpacing-revenue, margin deterioration, new-category materiality,
// owner-discretionary share, unexpected-negative) needs the same three
// category-derived sets (revenue/COGS/OPEX) and none of them needs a finer
// split (e.g. recurring vs. non-recurring) the way revenuequality does —
// deriving from the taxonomy directly means this package never drifts out
// of sync if a future taxonomy version adds a new revenue/COGS/OPEX code.
var (
	revenueCodes = categoryCodes(financial.CategoryRevenue)
	cogsCodes    = categoryCodes(financial.CategoryCogs)
	opexCodes    = categoryCodes(financial.CategoryOpex)
)

// expenseCodes is cogsCodes plus opexCodes — every account
// RuleExpenseOutpacingRevenue/RuleNewMaterialExpenseCategory/
// RuleUnexpectedNegativeAmount treat as "an expense."
var expenseCodes = append(append([]financial.Code{}, cogsCodes...), opexCodes...)

// categoryCodes returns every financial.Code in category, sorted (via
// financial.CodesByCategory, which is itself already sorted by Code).
func categoryCodes(category financial.CodeCategory) []financial.Code {
	metas := financial.CodesByCategory(category)
	out := make([]financial.Code, 0, len(metas))
	for _, m := range metas {
		out = append(out, m.Code)
	}
	return out
}

// sumCodes sums idx's amount for every code in codes at period. anyPresent
// is true if at least one of codes had a NormalizedItem at period (even if
// its Amount was 0) — mirrors revenuequality.sumCodes/workingcapital.sumCodes's
// identical "presence, not just nonzero" convention.
func sumCodes(idx codeIndex, period financial.Period, codes []financial.Code) (total float64, anyPresent bool) {
	for _, code := range codes {
		item, ok := idx.lookup(code, period)
		if !ok {
			continue
		}
		anyPresent = true
		total += item.Amount
	}
	return total, anyPresent
}

// totalRevenue returns Total Revenue for period: the sum of every
// revenueCodes item present, with anyPresent mirroring sumCodes.
func totalRevenue(idx codeIndex, period financial.Period) (float64, bool) {
	return sumCodes(idx, period, revenueCodes)
}

// totalCogs returns Total COGS for period.
func totalCogs(idx codeIndex, period financial.Period) (float64, bool) {
	return sumCodes(idx, period, cogsCodes)
}

// totalOpex returns Total OPEX for period.
func totalOpex(idx codeIndex, period financial.Period) (float64, bool) {
	return sumCodes(idx, period, opexCodes)
}

// grossMargin returns (revenue - cogs) / revenue for period. Available only
// if revenue is present and nonzero and cogs is present (cogs being $0 is a
// legitimate, available figure — e.g. a pure-service business — so only
// "no COGS codes reported at all" makes this unavailable, matching
// sumCodes' presence convention).
func grossMargin(idx codeIndex, period financial.Period) (float64, bool) {
	revenue, revenueOK := totalRevenue(idx, period)
	cogs, cogsOK := totalCogs(idx, period)
	if !revenueOK || !cogsOK || revenue == 0 {
		return 0, false
	}
	return (revenue - cogs) / revenue, true
}

// operatingMargin returns (revenue - cogs - opex) / revenue for period.
// Available only under the same rules as grossMargin, additionally
// requiring opex to be present.
func operatingMargin(idx codeIndex, period financial.Period) (float64, bool) {
	revenue, revenueOK := totalRevenue(idx, period)
	cogs, cogsOK := totalCogs(idx, period)
	opex, opexOK := totalOpex(idx, period)
	if !revenueOK || !cogsOK || !opexOK || revenue == 0 {
		return 0, false
	}
	return (revenue - cogs - opex) / revenue, true
}
