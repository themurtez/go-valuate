package management

// ChartPoint is one X/Y observation in a ChartSeries — X is a display
// label (a financial.Period's string value for a historical point, or a
// forecast period's own label for a projected point), never a typed date,
// since this package spans both historical (financial.Period, an opaque
// string with no guaranteed sort order of its own) and forecast (a plain
// display label) periods under one series — see ChartSeries' doc comment.
type ChartPoint struct {
	X string `json:"x"`
	// Y is this point's value. Unavailable points are omitted from
	// ChartSeries.Points entirely (never included with Y.Available ==
	// false), since a chart has no meaningful way to plot a gap other than
	// leaving the point out — a caller wanting to detect a gap compares
	// ChartSeries.Points' X labels against the series' own expected period
	// list.
	Y Value `json:"y"`
	// IsForecast is true when this point comes from Input.Forecast rather
	// than a historical series, letting a caller render the forecast
	// portion of a series distinctly (e.g. a dashed line) without
	// re-deriving which points are projected from X alone.
	IsForecast bool `json:"is_forecast,omitempty"`
}

// ChartSeries is one named, chart-ready line series: a label, a unit, and
// an ordered list of points spanning historical periods (chronological,
// per HistoricalSeries/ProfitabilitySeries/etc.'s own ordering) followed by
// forecast periods (in Input.Forecast.ForecastPeriods' order), when both
// are available for the same underlying figure.
type ChartSeries struct {
	// Label is a short, fixed, human-readable name for this series (e.g.
	// "Total Revenue", "EBITDA Margin").
	Label string `json:"label"`
	Unit  Unit   `json:"unit"`
	// Source names which Input field(s) this series was built from (e.g.
	// "metrics", "metrics+forecast"), for traceability.
	Source string       `json:"source,omitempty"`
	Points []ChartPoint `json:"points,omitempty"`
}

// ChartSeriesSection is the chart-ready series section: a fixed, ordered
// set of ChartSeries built from whichever of HistoricalSeries,
// ProfitabilitySeries, CashFlowSeries, WorkingCapitalSeries, and
// ForecastTables are Available — see buildChartSeries for the exact fixed
// list and which section each series is sourced from.
type ChartSeriesSection struct {
	// Available is true when at least one ChartSeries has at least one
	// point.
	Available bool          `json:"available"`
	Series    []ChartSeries `json:"series,omitempty"`
}
