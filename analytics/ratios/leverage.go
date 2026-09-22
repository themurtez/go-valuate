package ratios

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// debtToEquity computes Total Debt / Total Equity, read from Snapshot's
// TotalDebt (financial/metrics already computes this) and this package's
// own TotalEquity (see components.go). Available only if both are
// available and total equity is nonzero. A negative Total Equity is common
// (accumulated deficit) and produces a negative ratio — allowed through
// rather than suppressed, mirroring returnOnEquity's identical rationale
// (profitability.go): this package reports the arithmetic result and lets
// the caller interpret an unusual figure, rather than silently hiding it.
func debtToEquity(s metrics.Snapshot, totalEquity metrics.MetricValue, period financial.Period) Ratio {
	return marginRatio(RatioDebtToEquity, "Total Debt / Total Equity", s.TotalDebt, totalEquity, metrics.MetricTotalDebt, "Total Debt", "TOTAL_EQUITY", "Total Equity", period)
}

// debtToAssets computes Total Debt / Total Assets. Available only if both
// are available and total assets is nonzero.
func debtToAssets(s metrics.Snapshot, totalAssets metrics.MetricValue, period financial.Period) Ratio {
	return marginRatio(RatioDebtToAssets, "Total Debt / Total Assets", s.TotalDebt, totalAssets, metrics.MetricTotalDebt, "Total Debt", "TOTAL_ASSETS", "Total Assets", period)
}

// debtToEBITDA computes Total Debt / EBITDA — the standard leverage
// multiple ("X times EBITDA"). Available only if both are available and
// EBITDA is nonzero. A negative EBITDA is allowed through the formula
// (producing a negative or otherwise not-conventionally-interpretable
// multiple) rather than suppressed — see debtToEquity's identical
// rationale for why this package does not second-guess an unusual result
// arithmetically derived from available data.
func debtToEBITDA(s metrics.Snapshot, period financial.Period) Ratio {
	return marginRatio(RatioDebtToEBITDA, "Total Debt / EBITDA", s.TotalDebt, s.EBITDA, metrics.MetricTotalDebt, "Total Debt", metrics.MetricEBITDA, "EBITDA", period)
}

// netDebtToEBITDA computes Net Debt / EBITDA, using Snapshot's NetDebt
// (Total Debt - Cash, available only when cash is present per
// metrics.debtMetrics' own rule). Available only if both are available and
// EBITDA is nonzero.
func netDebtToEBITDA(s metrics.Snapshot, period financial.Period) Ratio {
	return marginRatio(RatioNetDebtToEBITDA, "Net Debt / EBITDA", s.NetDebt, s.EBITDA, metrics.MetricNetDebt, "Net Debt", metrics.MetricEBITDA, "EBITDA", period)
}

// interestCoverage computes EBIT / Interest Expense — how many times
// operating earnings cover the interest expense obligation. Uses EBIT
// (not EBITDA) as the conventional numerator for this specific ratio,
// consistent with the standard "times interest earned" definition.
// Available only if EBIT is available, interest expense is present in the
// dataset, and interest expense is nonzero (a business with $0 interest
// expense has no meaningful coverage ratio to compute — not "infinite
// coverage" reported as a number, and not a division-by-zero result).
func interestCoverage(idx codeIndex, s metrics.Snapshot, period financial.Period) Ratio {
	r := Ratio{Metric: RatioInterestCoverage, Period: period, Formula: "EBIT / Interest Expense"}
	interestExpense, ok := idx.lookup(financial.CodeInterestExpense, period)
	if !s.EBIT.Available || !ok || interestExpense == 0 {
		return r
	}
	r.Value = metrics.AvailableValue(s.EBIT.Value / interestExpense)
	r.Components = []Component{
		{Code: metrics.MetricEBIT, Label: "EBIT", Amount: s.EBIT.Value},
		{Code: string(financial.CodeInterestExpense), Label: "Interest Expense", Amount: interestExpense},
	}
	return r
}
