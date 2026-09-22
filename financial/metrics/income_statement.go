package metrics

import "github.com/themurtez/go-valuate/financial"

// Sign convention: every amount in a financial.FinancialDataset is stored as
// a positive magnitude in its natural "as reported" sense — an expense code
// (COGS, OPEX, depreciation, interest expense, taxes) holds a positive
// number representing how much was spent, not a negated contribution to
// income. financial.Normalize performs no sign manipulation, so this
// convention flows through unchanged from however the caller's raw/mapped
// data represented it. Every formula in this package explicitly subtracts
// expense-side codes rather than assuming a pre-negated sign; see each
// function's doc comment for the exact arithmetic.

var (
	revenueCodes = []financial.Code{
		financial.CodeRevProduct,
		financial.CodeRevService,
		financial.CodeRevRecurring,
		financial.CodeRevOther,
	}
	cogsCodes = []financial.Code{
		financial.CodeCogsMaterial,
		financial.CodeCogsDirectLabor,
		financial.CodeCogsFreight,
		financial.CodeCogsOther,
	}
	opexCodes = []financial.Code{
		financial.CodeOpexPayroll,
		financial.CodeOpexOwnerComp,
		financial.CodeOpexRent,
		financial.CodeOpexMarketing,
		financial.CodeOpexInsurance,
		financial.CodeOpexUtilities,
		financial.CodeOpexSoftware,
		financial.CodeOpexProfessionalFees,
		financial.CodeOpexRepairs,
		financial.CodeOpexVehicle,
		financial.CodeOpexTravel,
		financial.CodeOpexOffice,
		financial.CodeOpexOther,
	}
)

// revenueBreakdown computes total revenue plus its four component lines for
// one period. TotalRevenue.Available is true if at least one revenue code is
// present in the dataset for period; each component line is independently
// Available based on whether its own code is present.
func revenueBreakdown(idx codeIndex, period financial.Period) (total MetricResult, product, service, recurring, other MetricResult) {
	sum := sumCodes(idx, period, revenueCodes...)
	total = MetricResult{
		Metric:  MetricTotalRevenue,
		Period:  period,
		Formula: "Product Revenue + Service Revenue + Recurring Revenue + Other Revenue",
	}
	if sum.anyPresent {
		total.Value = AvailableValue(sum.total)
		total.Components = sum.components
	}

	product = singleCodeMetric(idx, period, MetricProductRevenue, "Product Revenue (as reported)", financial.CodeRevProduct)
	service = singleCodeMetric(idx, period, MetricServiceRevenue, "Service Revenue (as reported)", financial.CodeRevService)
	recurring = singleCodeMetric(idx, period, MetricRecurringRevenue, "Recurring Revenue (as reported)", financial.CodeRevRecurring)
	other = singleCodeMetric(idx, period, MetricOtherRevenue, "Other Revenue (as reported)", financial.CodeRevOther)
	return
}

// totalCOGS sums every COGS code for one period.
func totalCOGS(idx codeIndex, period financial.Period) MetricResult {
	sum := sumCodes(idx, period, cogsCodes...)
	res := MetricResult{
		Metric:  MetricTotalCOGS,
		Period:  period,
		Formula: "Materials + Direct Labor + Freight + Other COGS",
	}
	if sum.anyPresent {
		res.Value = AvailableValue(sum.total)
		res.Components = sum.components
	}
	return res
}

// grossProfit computes Revenue - COGS. Available only if both TotalRevenue
// and TotalCOGS are themselves available; a business with revenue but no
// COGS data at all cannot have a trustworthy gross profit computed (as
// opposed to a business that legitimately has $0 COGS, which would still
// show TotalCOGS as Available with Value 0 — see sumCodes' anyPresent
// semantics).
func grossProfit(revenue, cogs MetricResult, period financial.Period) MetricResult {
	res := MetricResult{
		Metric:  MetricGrossProfit,
		Period:  period,
		Formula: "Total Revenue - Total COGS",
	}
	if !revenue.Value.Available || !cogs.Value.Available {
		return res
	}
	res.Value = AvailableValue(revenue.Value.Value - cogs.Value.Value)
	res.Components = []Component{
		{Code: MetricTotalRevenue, Label: "Total Revenue", Amount: revenue.Value.Value},
		{Code: MetricTotalCOGS, Label: "Total COGS", Amount: -cogs.Value.Value},
	}
	return res
}

// grossMargin computes Gross Profit / Total Revenue. Available only if
// GrossProfit is available and TotalRevenue is nonzero (a zero-revenue
// business has no meaningful margin ratio, so this deliberately returns
// Unavailable rather than a division-by-zero result or a misleading 0/Inf).
func grossMargin(gp, revenue MetricResult, period financial.Period) MetricResult {
	res := MetricResult{
		Metric:  MetricGrossMargin,
		Period:  period,
		Formula: "Gross Profit / Total Revenue",
	}
	if !gp.Value.Available || !revenue.Value.Available || revenue.Value.Value == 0 {
		return res
	}
	res.Value = AvailableValue(gp.Value.Value / revenue.Value.Value)
	return res
}

// totalOpex sums every operating-expense code for one period. Owner
// compensation is included here (it is a real cash operating expense) and
// separately exposed via OwnerCompensation for SDE purposes.
func totalOpex(idx codeIndex, period financial.Period) MetricResult {
	sum := sumCodes(idx, period, opexCodes...)
	res := MetricResult{
		Metric:  MetricTotalOpex,
		Period:  period,
		Formula: "sum of all OPEX_* codes (including owner compensation)",
	}
	if sum.anyPresent {
		res.Value = AvailableValue(sum.total)
		res.Components = sum.components
	}
	return res
}

// ebit computes Gross Profit - Total Operating Expenses (operating earnings
// before interest and taxes). Available only if both inputs are available.
func ebit(gp, opex MetricResult, period financial.Period) MetricResult {
	res := MetricResult{
		Metric:  MetricEBIT,
		Period:  period,
		Formula: "Gross Profit - Total Operating Expenses",
	}
	if !gp.Value.Available || !opex.Value.Available {
		return res
	}
	res.Value = AvailableValue(gp.Value.Value - opex.Value.Value)
	res.Components = []Component{
		{Code: MetricGrossProfit, Label: "Gross Profit", Amount: gp.Value.Value},
		{Code: MetricTotalOpex, Label: "Total Operating Expenses", Amount: -opex.Value.Value},
	}
	return res
}

// ebitda computes EBIT + Depreciation + Amortization. This is the exact
// formula used throughout this package: EBITDA is always derived from EBIT
// plus D&A found in the dataset, never taken as an externally reported
// figure (reconciliation.Run is the place that compares this reconstructed
// value against a caller-supplied reported EBITDA, if one exists).
//
// Available only if EBIT is available; Depreciation and Amortization that
// are absent from the dataset contribute 0 rather than making the whole
// figure unavailable, since many statements combine D&A into COGS/OPEX
// lines without breaking it out separately, and EBIT already reflects
// whatever was reported.
func ebitda(ebitRes, depreciation, amortization MetricResult, period financial.Period) MetricResult {
	res := MetricResult{
		Metric:  MetricEBITDA,
		Period:  period,
		Formula: "EBIT + Depreciation + Amortization",
	}
	if !ebitRes.Value.Available {
		return res
	}
	dep := 0.0
	amort := 0.0
	components := []Component{
		{Code: MetricEBIT, Label: "EBIT", Amount: ebitRes.Value.Value},
	}
	if depreciation.Value.Available {
		dep = depreciation.Value.Value
		components = append(components, Component{Code: string(financial.CodeDepreciation), Label: "Depreciation", Amount: dep})
	}
	if amortization.Value.Available {
		amort = amortization.Value.Value
		components = append(components, Component{Code: string(financial.CodeAmortization), Label: "Amortization", Amount: amort})
	}
	res.Value = AvailableValue(ebitRes.Value.Value + dep + amort)
	res.Components = components
	return res
}

// ebitdaMargin computes EBITDA / Total Revenue, with the same zero-revenue
// guard as grossMargin.
func ebitdaMargin(ebitdaRes, revenue MetricResult, period financial.Period) MetricResult {
	res := MetricResult{
		Metric:  MetricEBITDAMargin,
		Period:  period,
		Formula: "EBITDA / Total Revenue",
	}
	if !ebitdaRes.Value.Available || !revenue.Value.Available || revenue.Value.Value == 0 {
		return res
	}
	res.Value = AvailableValue(ebitdaRes.Value.Value / revenue.Value.Value)
	return res
}

// sde computes Seller's Discretionary Earnings.
//
// Exact formula: SDE = EBITDA + Owner Compensation
//
// Owner compensation is added back because EBITDA already deducted it as an
// operating expense (see totalOpex), but SDE is defined as the total
// financial benefit available to a single working owner-operator, which
// includes their own compensation. This is the standard small-business
// valuation definition of SDE built from the components this package
// currently has available.
//
// This function does NOT add back any other discretionary items (personal
// vehicle expenses run through the business, one-time legal settlements,
// above-market rent to a related party, etc.) — per this module's scope,
// discretionary add-backs beyond financial.CodeOpexOwnerComp are explicitly
// deferred to a later adjustments module. Callers should treat this SDE as
// a baseline, not a final number.
//
// Available only if EBITDA is available; owner compensation absent from the
// dataset contributes 0 (many businesses, e.g. non-owner-operated ones,
// legitimately have none).
func sde(ebitdaRes, ownerComp MetricResult, period financial.Period) MetricResult {
	res := MetricResult{
		Metric:  MetricSDE,
		Period:  period,
		Formula: "EBITDA + Owner Compensation (no further discretionary add-backs)",
	}
	if !ebitdaRes.Value.Available {
		return res
	}
	comp := 0.0
	components := []Component{
		{Code: MetricEBITDA, Label: "EBITDA", Amount: ebitdaRes.Value.Value},
	}
	if ownerComp.Value.Available {
		comp = ownerComp.Value.Value
		components = append(components, Component{Code: string(financial.CodeOpexOwnerComp), Label: "Owner Compensation", Amount: comp})
	}
	res.Value = AvailableValue(ebitdaRes.Value.Value + comp)
	res.Components = components
	return res
}

// netIncome computes:
//
//	Operating Income (EBIT)
//	+ Other Income
//	+ Interest Income
//	- Interest Expense
//	- Other Expense
//	- Taxes
//	= Net Income
//
// Available only if EBIT is available. Every other line (other income,
// interest income/expense, other expense, taxes) that is absent from the
// dataset contributes 0 rather than making the whole figure unavailable —
// most small-business statements below the operating-income line are
// sparse, and treating any single missing below-the-line item as
// disqualifying would make NetIncome nearly always Unavailable in practice.
// This means the reconstructed NetIncome may differ from a below-the-line
// reported figure that includes items this dataset does not model (e.g.
// extraordinary items); reconciliation.Run is the place a caller can compare
// against an explicitly reported net income if greater rigor is needed.
func netIncome(idx codeIndex, ebitRes MetricResult, period financial.Period) MetricResult {
	res := MetricResult{
		Metric:  MetricNetIncome,
		Period:  period,
		Formula: "EBIT + Other Income + Interest Income - Interest Expense - Other Expense - Taxes",
	}
	if !ebitRes.Value.Available {
		return res
	}

	components := []Component{{Code: MetricEBIT, Label: "EBIT", Amount: ebitRes.Value.Value}}
	total := ebitRes.Value.Value

	add := func(code financial.Code, label string, negate bool) {
		amount, ok := idx.lookup(code, period)
		if !ok {
			return
		}
		signed := amount
		if negate {
			signed = -amount
		}
		total += signed
		components = append(components, Component{Code: string(code), Label: label, Amount: signed})
	}
	add(financial.CodeOtherIncome, "Other Income", false)
	add(financial.CodeInterestIncome, "Interest Income", false)
	add(financial.CodeInterestExpense, "Interest Expense", true)
	add(financial.CodeOtherExpense, "Other Expense", true)
	add(financial.CodeIncomeTax, "Income Tax", true)

	res.Value = AvailableValue(total)
	res.Components = components
	return res
}

// singleCodeMetric builds a MetricResult that is a direct pass-through
// lookup of one canonical code, with no aggregation.
func singleCodeMetric(idx codeIndex, period financial.Period, metric, formula string, code financial.Code) MetricResult {
	res := MetricResult{Metric: metric, Period: period, Formula: formula}
	amount, ok := idx.lookup(code, period)
	if !ok {
		return res
	}
	res.Value = AvailableValue(amount)
	return res
}
