package management

import "github.com/themurtez/go-valuate/financial"

// VarianceLine is one line item's actual-vs-baseline comparison — a
// condensed view of analytics/variance.LineVariance, dropping fields (e.g.
// TaxonomyCategory, Materiality) not needed for a presentation table.
type VarianceLine struct {
	AccountCode       financial.Code   `json:"account_code"`
	Label             string           `json:"label,omitempty"`
	Period            financial.Period `json:"period"`
	Actual            float64          `json:"actual"`
	BaselineAvailable bool             `json:"baseline_available"`
	Baseline          float64          `json:"baseline,omitempty"`
	AbsoluteVariance  Value            `json:"absolute_variance"`
	PercentVariance   Value            `json:"percent_variance"`
	// Favorability echoes analytics/variance.Favorability's string value
	// verbatim ("favorable"/"unfavorable"/"neutral"/"unknown"), copied as a
	// plain string so this package need not import variance's Favorability
	// type for one field.
	Favorability string `json:"favorability,omitempty"`
}

// VarianceTable is one grouping of VarianceLines — either the full line-item
// table or a top-N favorable/unfavorable subset, mirroring
// analytics/variance.Result's own TopFavorable/TopUnfavorable split.
type VarianceTable struct {
	// Label names this table (e.g. "All Line Items", "Top Favorable
	// Variances", "Top Unfavorable Variances", "Material Exceptions").
	Label string         `json:"label"`
	Lines []VarianceLine `json:"lines,omitempty"`
}

// VarianceTables is the variance tables section: sourced entirely from
// Input.Variance.
type VarianceTables struct {
	// Available is true when Input.Variance.Available was true.
	Available bool `json:"available"`
	// FormulaVersion echoes Input.Variance.FormulaVersion.
	FormulaVersion string `json:"formula_version,omitempty"`
	// Tables is a fixed, ordered set of tables: "All Line Items", "Top
	// Favorable Variances", "Top Unfavorable Variances", "Material
	// Exceptions" — a table is omitted entirely when its source slice is
	// empty (e.g. no material exceptions), never included empty.
	Tables []VarianceTable `json:"tables,omitempty"`
}

// ForecastPeriod is one projected period's headline P&L/cash-flow figures
// for one scenario — a condensed view of analytics/forecast.PeriodPL plus
// its corresponding analytics/forecast.CashFlowPeriod for the same
// forecast period label.
type ForecastPeriod struct {
	// Period is the forecast period's display label (analytics/forecast
	// periods are plain labels, not financial.Period — see
	// analytics/forecast.PeriodPL.Period's doc comment).
	Period       string `json:"period"`
	PeriodNumber int    `json:"period_number"`
	TotalRevenue Value  `json:"total_revenue"`
	GrossProfit  Value  `json:"gross_profit"`
	EBITDA       Value  `json:"ebitda"`
	NetIncome    Value  `json:"net_income"`
	FreeCashFlow Value  `json:"free_cash_flow"`
}

// ForecastScenarioTable is one scenario's full projected-period series.
type ForecastScenarioTable struct {
	Name string `json:"name"`
	// Type echoes analytics/forecast.ScenarioType's string value verbatim,
	// copied as a plain string so this package need not import forecast's
	// ScenarioType for one field.
	Type    string           `json:"type,omitempty"`
	Periods []ForecastPeriod `json:"periods,omitempty"`
}

// ForecastTables is the forecast tables section: one table per
// Input.Forecast.ScenarioResults entry, in that slice's own order (forecast
// does not otherwise define a canonical scenario ordering beyond
// caller-supplied Scenario slice order, which ScenarioResults preserves).
type ForecastTables struct {
	// Available is true when Input.Forecast.Available was true.
	Available bool `json:"available"`
	// FormulaVersion echoes Input.Forecast.FormulaVersion.
	FormulaVersion string `json:"formula_version,omitempty"`
	// Horizon echoes Input.Forecast.Horizon.
	Horizon   int                     `json:"horizon,omitempty"`
	Scenarios []ForecastScenarioTable `json:"scenarios,omitempty"`
}
