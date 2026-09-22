package ratios

import "github.com/themurtez/go-valuate/financial"

// totalAssetCodes sums to this package's own Total Assets figure: every
// current-asset code plus fixed assets (net of accumulated depreciation,
// mirroring metrics.tangibleAssetValue's identical net-of-depreciation
// treatment) plus intangible assets and goodwill. financial/metrics does
// not itself compute a Total Assets figure (see that package's Snapshot —
// CurrentAssets is the closest existing figure, but stops short of the
// full balance sheet), so this package derives it directly from the
// taxonomy's balance-sheet codes rather than assuming Assets = Liabilities
// + Equity (a plug that would silently mask a source statement that does
// not actually balance — this package never assumes that identity holds).
var totalAssetCodes = []financial.Code{
	financial.CodeBsCash,
	financial.CodeBsAccountsReceivable,
	financial.CodeBsInventory,
	financial.CodeBsPrepaid,
	financial.CodeBsCurrentAssetOther,
	financial.CodeBsFixedAssets,
	financial.CodeBsIntangibleAssets,
	financial.CodeBsGoodwill,
}

// totalEquityCodes sums to this package's own Total Equity figure:
// retained earnings plus owner/shareholder equity — the two taxonomy codes
// that represent residual equity (see financial.CodeBsRetainedEarnings/
// CodeBsOwnerEquity's doc comments in financial/taxonomy.go).
var totalEquityCodes = []financial.Code{
	financial.CodeBsRetainedEarnings,
	financial.CodeBsOwnerEquity,
}

// quickAssetCodes sums to "quick assets" for QuickRatio: current assets
// excluding inventory and prepaid expenses (neither is readily convertible
// to cash), the standard acid-test definition.
var quickAssetCodes = []financial.Code{
	financial.CodeBsCash,
	financial.CodeBsAccountsReceivable,
	financial.CodeBsCurrentAssetOther,
}

// totalDebtCodes mirrors metrics.debtMetrics' own TotalDebt definition
// (short-term + long-term debt), duplicated here rather than depended-on
// since this package needs it as a plain sum-with-components result (see
// sumCodes below), not metrics' MetricResult-returning debtMetrics
// function — the same "duplicated, not depended-on" convention
// workingcapital.revenueCodes documents for its own identical situation.
var totalDebtCodes = []financial.Code{
	financial.CodeBsShortTermDebt,
	financial.CodeBsLongTermDebt,
}

// sumResult mirrors summing a set of codes for one period, matching
// workingcapital.sumResult/metrics.sumResult's identical shape.
type sumResult struct {
	total      float64
	anyPresent bool
	components []Component
}

// sumCodes sums the amounts for the given codes in period from idx, using
// the exact tolerant-per-code/anyPresent semantics documented on
// metrics.sumCodes: a code absent from the dataset simply does not
// contribute (not zero-and-thus-disqualifying), while anyPresent tells the
// caller whether at least one requested code existed at all for period.
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
