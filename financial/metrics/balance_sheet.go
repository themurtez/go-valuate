package metrics

import "github.com/themurtez/go-valuate/financial"

var (
	currentAssetCodes = []financial.Code{
		financial.CodeBsCash,
		financial.CodeBsAccountsReceivable,
		financial.CodeBsInventory,
		financial.CodeBsPrepaid,
		financial.CodeBsCurrentAssetOther,
	}
	currentLiabilityCodes = []financial.Code{
		financial.CodeBsAccountsPayable,
		financial.CodeBsCurrentLiabilityOther,
		financial.CodeBsShortTermDebt,
	}
)

// currentAssets sums the current-asset codes for one period.
func currentAssets(idx codeIndex, period financial.Period) MetricResult {
	sum := sumCodes(idx, period, currentAssetCodes...)
	res := MetricResult{
		Metric:  MetricCurrentAssets,
		Period:  period,
		Formula: "Cash + Accounts Receivable + Inventory + Prepaid Expenses + Other Current Assets",
	}
	if sum.anyPresent {
		res.Value = AvailableValue(sum.total)
		res.Components = sum.components
	}
	return res
}

// currentLiabilities sums the current-liability codes for one period. Note
// this includes short-term debt, consistent with a standard current-assets
// vs. current-liabilities working-capital definition; TotalDebt/NetDebt (see
// debtMetrics) separately break out short-term debt as a debt figure.
func currentLiabilities(idx codeIndex, period financial.Period) MetricResult {
	sum := sumCodes(idx, period, currentLiabilityCodes...)
	res := MetricResult{
		Metric:  MetricCurrentLiabilities,
		Period:  period,
		Formula: "Accounts Payable + Other Current Liabilities + Short-Term Debt",
	}
	if sum.anyPresent {
		res.Value = AvailableValue(sum.total)
		res.Components = sum.components
	}
	return res
}

// workingCapital computes Current Assets - Current Liabilities. Available
// only if both inputs are available.
func workingCapital(ca, cl MetricResult, period financial.Period) MetricResult {
	res := MetricResult{
		Metric:  MetricWorkingCapital,
		Period:  period,
		Formula: "Current Assets - Current Liabilities",
	}
	if !ca.Value.Available || !cl.Value.Available {
		return res
	}
	res.Value = AvailableValue(ca.Value.Value - cl.Value.Value)
	res.Components = []Component{
		{Code: MetricCurrentAssets, Label: "Current Assets", Amount: ca.Value.Value},
		{Code: MetricCurrentLiabilities, Label: "Current Liabilities", Amount: -cl.Value.Value},
	}
	return res
}

// debtMetrics computes short-term debt, long-term debt, total debt, and net
// debt for one period.
//
// TotalDebt = Short-Term Debt + Long-Term Debt (each code absent from the
// dataset contributes 0; TotalDebt itself is Available only if at least one
// of the two debt codes is present, so a dataset with no debt codes at all
// reports Unavailable rather than a misleading $0).
//
// NetDebt = Total Debt - Cash. Available only if TotalDebt is available AND
// cash is present in the dataset — netting against an unknown cash position
// would misrepresent the result as more precise than the data supports.
func debtMetrics(idx codeIndex, period financial.Period) (shortTerm, longTerm, total, net MetricResult) {
	shortTerm = singleCodeMetric(idx, period, MetricShortTermDebt, "Short-Term Debt (as reported)", financial.CodeBsShortTermDebt)
	longTerm = singleCodeMetric(idx, period, MetricLongTermDebt, "Long-Term Debt (as reported)", financial.CodeBsLongTermDebt)

	sum := sumCodes(idx, period, financial.CodeBsShortTermDebt, financial.CodeBsLongTermDebt)
	total = MetricResult{
		Metric:  MetricTotalDebt,
		Period:  period,
		Formula: "Short-Term Debt + Long-Term Debt",
	}
	if sum.anyPresent {
		total.Value = AvailableValue(sum.total)
		total.Components = sum.components
	}

	net = MetricResult{
		Metric:  MetricNetDebt,
		Period:  period,
		Formula: "Total Debt - Cash",
	}
	cash, cashOK := idx.lookup(financial.CodeBsCash, period)
	if total.Value.Available && cashOK {
		net.Value = AvailableValue(total.Value.Value - cash)
		net.Components = []Component{
			{Code: MetricTotalDebt, Label: "Total Debt", Amount: total.Value.Value},
			{Code: string(financial.CodeBsCash), Label: "Cash", Amount: -cash},
		}
	}
	return
}

// tangibleAssetValue estimates tangible asset value as:
//
//	Fixed Assets - Accumulated Depreciation
//
// This deliberately excludes intangible assets and goodwill (the two
// taxonomy codes explicitly meant to hold non-tangible value:
// CodeBsIntangibleAssets, CodeBsGoodwill) since the point of a "tangible"
// figure is to isolate value backed by physical/depreciable property.
// Available only if fixed assets are present in the dataset; accumulated
// depreciation absent contributes 0 (some datasets report fixed assets net
// of depreciation already, with no separate accumulated-depreciation line).
func tangibleAssetValue(idx codeIndex, period financial.Period) MetricResult {
	res := MetricResult{
		Metric:  MetricTangibleAssetValue,
		Period:  period,
		Formula: "Fixed Assets - Accumulated Depreciation",
	}
	fixed, ok := idx.lookup(financial.CodeBsFixedAssets, period)
	if !ok {
		return res
	}
	accumDep, hasAccumDep := idx.lookup(financial.CodeBsAccumDepreciation, period)
	components := []Component{{Code: string(financial.CodeBsFixedAssets), Label: "Fixed Assets", Amount: fixed}}
	total := fixed
	if hasAccumDep {
		total -= accumDep
		components = append(components, Component{Code: string(financial.CodeBsAccumDepreciation), Label: "Accumulated Depreciation", Amount: -accumDep})
	}
	res.Value = AvailableValue(total)
	res.Components = components
	return res
}
