package report

// FinancialSummary is the report's historical financial overview, built
// from a caller-supplied series of financial/metrics.Snapshot values (see
// build.go) plus the normalized-earnings bridges from
// financial/adjustments and financial/earnings.
type FinancialSummary struct {
	// Periods lists every period included in this summary, in the order
	// supplied to Build (expected chronological, oldest first, matching
	// every other package in this repository's period-ordering
	// convention).
	Periods []FinancialPeriod `json:"periods"`
	// NormalizedEBITDA is the maintainable normalized EBITDA figure used as
	// a valuation input (e.g. financial/earnings.Result.Value over a
	// financial/adjustments EBITDA bridge series), when supplied.
	NormalizedEBITDA FinancialFigure `json:"normalized_ebitda"`
	// NormalizedSDE is the maintainable normalized SDE figure used as a
	// valuation input, when supplied.
	NormalizedSDE FinancialFigure `json:"normalized_sde"`
	// GrowthMetrics carries the caller-supplied historical growth/CAGR/
	// volatility figures (e.g. from financial/metrics.Trend), reshaped into
	// this package's plain (label, value) form rather than requiring a
	// consumer to know metrics.Trend's schema — see build.go.
	GrowthMetrics []Assumption `json:"growth_metrics,omitempty"`
}

// FinancialPeriod is one period's headline figures in the financial
// summary table.
type FinancialPeriod struct {
	// Period is a display label for this period (e.g. "2025", "FY2024").
	Period string `json:"period"`
	// Revenue is this period's total revenue, when available.
	Revenue FinancialFigure `json:"revenue"`
	// EBITDA is this period's (unadjusted, as-reported/computed) EBITDA.
	EBITDA FinancialFigure `json:"ebitda"`
	// EBITDAMargin is EBITDA / Revenue for this period, when available.
	EBITDAMargin FinancialFigure `json:"ebitda_margin"`
	// SDE is this period's (unadjusted) SDE.
	SDE FinancialFigure `json:"sde"`
	// GrossMargin is this period's gross margin, when available.
	GrossMargin FinancialFigure `json:"gross_margin"`
}

// FinancialFigure is a single labeled figure that may or may not be
// available, mirroring financial/metrics.MetricValue's
// available/unavailable distinction so a report never confuses "computed
// to be exactly 0" with "not available in the supplied data."
type FinancialFigure struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}
