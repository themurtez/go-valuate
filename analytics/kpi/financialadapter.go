package kpi

import "github.com/themurtez/go-valuate/financial/metrics"

// This file bridges financial/metrics.Snapshot — the authoritative source
// for revenue/gross-profit/EBITDA/working-capital-style figures in this
// repository (see financial/metrics' own package doc: "the single place
// valuation methods should get EBITDA, ..." from) — into this package's
// MetricValue shape, under the "financial.*" namespace (doc.go's "Adapter
// metric namespaces" section). This adapter reimplements no formula: it
// only reads Snapshot's already-computed MetricValue fields and
// republishes them under this package's own MetricValue type, one-to-one
// — task section 32's "expose existing computed values, not reimplement
// formulas" instruction.
//
// financial/metrics.Snapshot has its own distinct MetricValue type
// (Available/Value, no Unit/Aggregation/Code/Period/Dimensions) — this
// adapter's job is exactly translating that into THIS package's richer
// MetricValue, adding the Unit/Aggregation/Code/Period this engine
// requires.

// FinancialMetricCode is the fixed "financial.*" namespace this adapter
// publishes. Only a representative subset of Snapshot's fields is
// exposed, matching task section 32's example list; a caller wanting a
// Snapshot field not listed here can trivially add one more
// FinancialMetricValues-style line using the same pattern.
const (
	FinancialMetricRevenue        = "financial.revenue"
	FinancialMetricGrossProfit    = "financial.gross_profit"
	FinancialMetricGrossMargin    = "financial.gross_margin"
	FinancialMetricEBITDA         = "financial.ebitda"
	FinancialMetricEBITDAMargin   = "financial.ebitda_margin"
	FinancialMetricNetIncome      = "financial.net_income"
	FinancialMetricWorkingCapital = "financial.working_capital"
)

// FinancialMetricValues converts one financial/metrics.Snapshot into this
// package's MetricValue slice for period, tagged with currency (a
// Snapshot itself carries no currency field — that lives on the
// financial.FinancialDataset it was computed from — so the caller
// supplies it explicitly here, the same "caller supplies what the source
// package does not carry" convention every *adapter.go file in this
// repository follows for a missing field).
func FinancialMetricValues(snap metrics.Snapshot, period string, currency string) []MetricValue {
	currencyUnit := Unit{Kind: UnitCurrency, CurrencyCode: currency}
	percentUnit := Unit{Kind: UnitPercent}

	return []MetricValue{
		financialMetric(FinancialMetricRevenue, period, snap.TotalRevenue, currencyUnit),
		financialMetric(FinancialMetricGrossProfit, period, snap.GrossProfit, currencyUnit),
		financialMetric(FinancialMetricGrossMargin, period, snap.GrossMargin, percentUnit),
		financialMetric(FinancialMetricEBITDA, period, snap.EBITDA, currencyUnit),
		financialMetric(FinancialMetricEBITDAMargin, period, snap.EBITDAMargin, percentUnit),
		financialMetric(FinancialMetricNetIncome, period, snap.NetIncome, currencyUnit),
		financialMetric(FinancialMetricWorkingCapital, period, snap.WorkingCapital, currencyUnit),
	}
}

func financialMetric(code, period string, mv metrics.MetricValue, unit Unit) MetricValue {
	agg := AggregationSum
	if unit.Kind == UnitPercent {
		agg = AggregationNotAggregatable
	}
	return MetricValue{
		Code: code, Period: period, Value: mv.Value, Available: mv.Available,
		Unit: unit, Aggregation: agg, Source: "financial/metrics",
	}
}
