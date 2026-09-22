package ratios

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// grossMargin computes Gross Profit / Total Revenue. Available only if both
// values are available from Snapshot and revenue is nonzero — a zero-
// revenue business has no meaningful margin ratio, so this deliberately
// returns Unavailable rather than a division-by-zero result. Snapshot's own
// GrossMargin field already computes this identical ratio
// (financial/metrics.grossMargin); this package recomputes it directly from
// Snapshot's GrossProfit/TotalRevenue (rather than merely copying
// Snapshot.GrossMargin) purely so every Ratio here carries this package's
// own Components/Formula shape, exactly as metrics.grossMargin itself does
// not populate Components for a plain division.
func grossMargin(s metrics.Snapshot, period financial.Period) Ratio {
	return marginRatio(RatioGrossMargin, "Gross Profit / Total Revenue", s.GrossProfit, s.TotalRevenue, metrics.MetricGrossProfit, "Gross Profit", metrics.MetricTotalRevenue, "Total Revenue", period)
}

// operatingMargin computes EBIT / Total Revenue — "operating margin" in the
// conventional sense (earnings before interest and taxes, over revenue).
func operatingMargin(s metrics.Snapshot, period financial.Period) Ratio {
	return marginRatio(RatioOperatingMargin, "EBIT / Total Revenue", s.EBIT, s.TotalRevenue, metrics.MetricEBIT, "EBIT", metrics.MetricTotalRevenue, "Total Revenue", period)
}

// ebitdaMargin computes EBITDA / Total Revenue.
func ebitdaMargin(s metrics.Snapshot, period financial.Period) Ratio {
	return marginRatio(RatioEBITDAMargin, "EBITDA / Total Revenue", s.EBITDA, s.TotalRevenue, metrics.MetricEBITDA, "EBITDA", metrics.MetricTotalRevenue, "Total Revenue", period)
}

// netMargin computes Net Income / Total Revenue.
func netMargin(s metrics.Snapshot, period financial.Period) Ratio {
	return marginRatio(RatioNetMargin, "Net Income / Total Revenue", s.NetIncome, s.TotalRevenue, metrics.MetricNetIncome, "Net Income", metrics.MetricTotalRevenue, "Total Revenue", period)
}

// marginRatio is the shared numerator/denominator division pattern most
// ratios in this package follow (margins here, plus liquidity.go's
// CurrentRatio/CashRatio and any other plain division): Available only if
// both numerator and denominator are available and the denominator is
// nonzero.
func marginRatio(name, formula string, numerator, denominator metrics.MetricValue, numCode, numLabel, denomCode, denomLabel string, period financial.Period) Ratio {
	r := Ratio{Metric: name, Period: period, Formula: formula}
	if !numerator.Available || !denominator.Available || denominator.Value == 0 {
		return r
	}
	r.Value = metrics.AvailableValue(numerator.Value / denominator.Value)
	r.Components = []Component{
		{Code: numCode, Label: numLabel, Amount: numerator.Value},
		{Code: denomCode, Label: denomLabel, Amount: denominator.Value},
	}
	return r
}

// returnOnAssets computes Net Income / Total Assets. Available only if net
// income is available, total assets is available, and total assets is
// nonzero. A negative Total Assets figure (not ordinarily expected, but not
// impossible given this package's sum-of-codes derivation — see
// components.go) is allowed through the formula rather than treated as
// invalid: ROA is well-defined arithmetically for any nonzero denominator,
// and this package leaves the interpretation of an unusual result to the
// caller rather than second-guessing the input data.
func returnOnAssets(s metrics.Snapshot, totalAssets metrics.MetricValue, period financial.Period) Ratio {
	return marginRatio(RatioReturnOnAssets, "Net Income / Total Assets", s.NetIncome, totalAssets, metrics.MetricNetIncome, "Net Income", "TOTAL_ASSETS", "Total Assets", period)
}

// returnOnEquity computes Net Income / Total Equity. Available only if net
// income is available, total equity is available, and total equity is
// nonzero. A negative Total Equity is common (accumulated deficit) and is
// allowed through the formula — ROE against negative equity is a real,
// if unusual, figure this package reports rather than suppresses; a caller
// building a report can decide how to flag it.
func returnOnEquity(s metrics.Snapshot, totalEquity metrics.MetricValue, period financial.Period) Ratio {
	return marginRatio(RatioReturnOnEquity, "Net Income / Total Equity", s.NetIncome, totalEquity, metrics.MetricNetIncome, "Net Income", "TOTAL_EQUITY", "Total Equity", period)
}
