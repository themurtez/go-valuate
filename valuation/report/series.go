package report

// ChartSeries holds every chart-ready data series a future presentation
// layer might plot, as plain (label, value) points with no dependency on
// any specific charting library — see the package doc comment. Building a
// chart, choosing colors, or picking a chart type is entirely out of
// scope here; this package only shapes the numbers.
type ChartSeries struct {
	// ValuationByMethod is one point per included method: {Label: method
	// code, Value: headline figure} — the classic "bar chart of every
	// method's result" series.
	ValuationByMethod []SeriesPoint `json:"valuation_by_method,omitempty"`
	// RevenueHistory is one point per period: {Label: period, Value:
	// revenue}.
	RevenueHistory []SeriesPoint `json:"revenue_history,omitempty"`
	// EBITDAHistory is one point per period: {Label: period, Value:
	// EBITDA}.
	EBITDAHistory []SeriesPoint `json:"ebitda_history,omitempty"`
	// SDEHistory is one point per period: {Label: period, Value: SDE}.
	SDEHistory []SeriesPoint `json:"sde_history,omitempty"`
	// MarginHistory is one point per period: {Label: period, Value: EBITDA
	// margin} — a single representative margin series (EBITDA margin,
	// mirroring FinancialPeriod.EBITDAMargin) rather than every possible
	// margin, since a caller wanting gross margin or another margin series
	// charted can build it directly from FinancialSummary.Periods.
	MarginHistory []SeriesPoint `json:"margin_history,omitempty"`
	// ValuationHistory is a placeholder series for a future multi-valuation-
	// date history (e.g. this business's consensus value at several past
	// valuation dates) — empty for a single-valuation-date Report built by
	// this phase, since no persistence/history mechanism exists yet (see
	// the repository README's out-of-scope list). Shaped identically to
	// every other series here so a future addition needs no schema change,
	// only populated data.
	ValuationHistory []SeriesPoint `json:"valuation_history,omitempty"`
}

// SeriesPoint is a single presentation-neutral (label, value) point usable
// directly by any charting library the caller chooses.
type SeriesPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}
