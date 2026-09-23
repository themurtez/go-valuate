package management

// Coverage summarizes how many of this package's optional Input fields
// were actually available, letting a caller see at a glance how complete
// the underlying pack is without inspecting every Section itself —
// mirrors portfolio/diagnostics.CoverageCounts' identical
// one-bool-per-sibling-package shape (closer fit than
// transactions/salereadiness.Coverage's abstract-dimension-count shape,
// since this package's sections map 1:1 to named sibling packages rather
// than to abstract classified dimensions).
type Coverage struct {
	// TotalModules is the fixed number of optional sibling modules this
	// package can draw from — always 12 (Metrics, Ratios, CashFlow,
	// WorkingCapital, QoE, Variance, Forecast, Anomalies, Concentration,
	// RevenueQuality, Debt, Covenants; Consensus is a 13th but tracked
	// separately since it is not itself a Section source — see
	// WithConsensus).
	TotalModules int `json:"total_modules"`
	// AvailableModules is how many of the WithX fields below are true.
	AvailableModules int `json:"available_modules"`
	// CoveragePercent is AvailableModules / TotalModules, as a decimal.
	CoveragePercent float64 `json:"coverage_percent"`

	WithMetrics        bool `json:"with_metrics"`
	WithRatios         bool `json:"with_ratios"`
	WithCashFlow       bool `json:"with_cash_flow"`
	WithWorkingCapital bool `json:"with_working_capital"`
	WithQoE            bool `json:"with_qoe"`
	WithVariance       bool `json:"with_variance"`
	WithForecast       bool `json:"with_forecast"`
	WithAnomalies      bool `json:"with_anomalies"`
	WithConcentration  bool `json:"with_concentration"`
	WithRevenueQuality bool `json:"with_revenue_quality"`
	WithDebt           bool `json:"with_debt"`
	WithCovenants      bool `json:"with_covenants"`
	// WithConsensus is tracked but excluded from TotalModules/
	// AvailableModules/CoveragePercent, since Consensus contributes only to
	// ExecutiveSummary (a handful of KPIs) rather than backing an entire
	// Section the way the other twelve modules each do.
	WithConsensus bool `json:"with_consensus"`

	// MissingModules lists the fixed name of every module (in Input's own
	// field declaration order) with WithX == false, so a caller need not
	// inspect every WithX field individually to know what to supply next.
	MissingModules []string `json:"missing_modules,omitempty"`
}

// ModuleVersion echoes one contributing sibling package's own version
// constant, so a persisted Report remains self-describing about exactly
// which upstream formula version produced each Section — mirroring
// portfolio/diagnostics's QoESummary.FormulaVersion (etc.) pattern of
// copying a sibling's version field verbatim rather than only versioning
// this package's own FormulaVersion.
type ModuleVersion struct {
	// Module is a stable, fixed name for the sibling package (e.g.
	// "metrics", "ratios", "qoe") — matches the lowercase field name used
	// in Coverage's WithX naming and Input's own field names.
	Module string `json:"module"`
	// Version is the sibling package's own FormulaVersion (or, for
	// financial/adjustments-style packages, SemanticsVersion) string, when
	// that module was Available. Empty when the module was unavailable —
	// there is then no version to echo.
	Version string `json:"version,omitempty"`
}

// ModuleVersions is the source/module versions section: this package's own
// FormulaVersion plus every contributing sibling's own version, in a fixed
// order matching Coverage's WithX field order.
type ModuleVersions struct {
	// FormulaVersion echoes this package's own FormulaVersion constant.
	FormulaVersion string `json:"formula_version"`
	// Modules is one ModuleVersion per contributing sibling package, in a
	// fixed order (metrics, ratios, cash_flow, working_capital, qoe,
	// variance, forecast, anomalies, concentration, revenue_quality, debt,
	// covenants, consensus) — always all 13 entries, regardless of
	// availability (an unavailable module's entry has an empty Version,
	// never omitted, so a caller can always range over a complete,
	// fixed-shape list).
	Modules []ModuleVersion `json:"modules"`
}

// Report is the output of Calculate: every fixed section of
// presentation-neutral management-reporting data, in sectionOrder, plus a
// data-coverage summary and source/module versions.
type Report struct {
	// FormulaVersion identifies which version of this package's fixed
	// section-building rule set produced this Report.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all —
	// every optional Input field was left at its zero value / unavailable,
	// so no section could be populated. Every other field is then
	// zero-value except FormulaVersion and Errors — mirroring
	// transactions/salereadiness.Result.Available's identical "every other
	// field is zero-value" convention. In every other case Calculate
	// populates whatever subset of sections the supplied Input supports
	// and reports gaps via Warnings/Coverage.MissingModules.
	Available bool `json:"available"`

	ExecutiveSummary        ExecutiveSummary        `json:"executive_summary"`
	HistoricalSeries        HistoricalSeries        `json:"historical_series"`
	ProfitabilitySeries     ProfitabilitySeries     `json:"profitability_series"`
	LiquidityLeverageSeries LiquidityLeverageSeries `json:"liquidity_leverage_series"`
	CashFlowSeries          CashFlowSeries          `json:"cash_flow_series"`
	WorkingCapitalSeries    WorkingCapitalSeries    `json:"working_capital_series"`
	VarianceTables          VarianceTables          `json:"variance_tables"`
	ForecastTables          ForecastTables          `json:"forecast_tables"`
	TopIssues               TopIssues               `json:"top_issues"`
	ChartSeries             ChartSeriesSection      `json:"chart_series"`

	// Coverage summarizes how many of the 12 section-backing modules were
	// actually available.
	Coverage Coverage `json:"coverage"`
	// Versions carries this package's own FormulaVersion plus every
	// contributing sibling's own version.
	Versions ModuleVersions `json:"versions"`

	// Warnings carries every Issue with IssueSeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with IssueSeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
