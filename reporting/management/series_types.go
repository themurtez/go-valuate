package management

import "github.com/themurtez/go-valuate/financial"

// HistoricalPeriod is one period's core income-statement/balance-sheet
// figures, the row shape HistoricalSeries.Periods iterates over — sourced
// from Input.Metrics.Snapshots (financial/metrics.Snapshot), the canonical
// per-period shape every sibling analytics package already reuses.
type HistoricalPeriod struct {
	Period             financial.Period `json:"period"`
	TotalRevenue       Value            `json:"total_revenue"`
	TotalCOGS          Value            `json:"total_cogs"`
	GrossProfit        Value            `json:"gross_profit"`
	GrossMargin        Value            `json:"gross_margin"`
	TotalOpex          Value            `json:"total_opex"`
	EBITDA             Value            `json:"ebitda"`
	EBITDAMargin       Value            `json:"ebitda_margin"`
	NetIncome          Value            `json:"net_income"`
	Cash               Value            `json:"cash"`
	AccountsReceivable Value            `json:"accounts_receivable"`
	Inventory          Value            `json:"inventory"`
	CurrentAssets      Value            `json:"current_assets"`
	AccountsPayable    Value            `json:"accounts_payable"`
	CurrentLiabilities Value            `json:"current_liabilities"`
	WorkingCapital     Value            `json:"working_capital"`
	TotalDebt          Value            `json:"total_debt"`
	NetDebt            Value            `json:"net_debt"`
}

// HistoricalSeries is the historical financial series section: one row per
// period, chronological (Input.PeriodMeta order when supplied, else
// Input.Metrics.Snapshots' own order), sourced entirely from
// Input.Metrics.Snapshots.
type HistoricalSeries struct {
	// Available is true when Input.Metrics.Snapshots was non-empty.
	Available bool `json:"available"`
	// FormulaVersion echoes Input.Metrics.FormulaVersion, for traceability
	// back to exactly which metrics formula version produced Periods.
	FormulaVersion string `json:"formula_version,omitempty"`
	// Periods is one HistoricalPeriod per Input.Metrics.Snapshots entry, in
	// chronological order.
	Periods []HistoricalPeriod `json:"periods,omitempty"`
}

// ProfitabilityPeriod is one period's margin figures — sourced primarily
// from Input.Ratios.History (analytics/ratios.PeriodRatios) when available,
// falling back to computing GrossMargin/EBITDAMargin directly from
// Input.Metrics.Snapshots when Ratios is unavailable.
type ProfitabilityPeriod struct {
	Period          financial.Period `json:"period"`
	GrossMargin     Value            `json:"gross_margin"`
	OperatingMargin Value            `json:"operating_margin"`
	EBITDAMargin    Value            `json:"ebitda_margin"`
	NetMargin       Value            `json:"net_margin"`
	ReturnOnAssets  Value            `json:"return_on_assets"`
	ReturnOnEquity  Value            `json:"return_on_equity"`
}

// ProfitabilitySeries is the profitability series section: one row per
// period, chronological.
type ProfitabilitySeries struct {
	// Available is true when at least one ProfitabilityPeriod entry has at
	// least one Available margin figure.
	Available bool `json:"available"`
	// FormulaVersion echoes Input.Ratios.FormulaVersion when Ratios was the
	// source, else Input.Metrics.FormulaVersion when the metrics-only
	// fallback was used.
	FormulaVersion string `json:"formula_version,omitempty"`
	// Source names which Input field populated Periods ("ratios" or
	// "metrics").
	Source  string                `json:"source,omitempty"`
	Periods []ProfitabilityPeriod `json:"periods,omitempty"`
}

// LiquidityLeveragePeriod is one period's liquidity and leverage ratios —
// sourced entirely from Input.Ratios.History
// (analytics/ratios.PeriodRatios). This section has no Input.Debt
// fallback: analytics/debt.Result's coverage figures (BaseCase,
// ExistingOnlyCase) are single point-in-time results with no Period field
// of their own, so there is no period to attach a Debt-sourced
// NetDebtToEBITDA to here — see LiquidityLeverageSeries' doc comment.
type LiquidityLeveragePeriod struct {
	Period           financial.Period `json:"period"`
	CurrentRatio     Value            `json:"current_ratio"`
	QuickRatio       Value            `json:"quick_ratio"`
	CashRatio        Value            `json:"cash_ratio"`
	DebtToEquity     Value            `json:"debt_to_equity"`
	DebtToAssets     Value            `json:"debt_to_assets"`
	DebtToEBITDA     Value            `json:"debt_to_ebitda"`
	NetDebtToEBITDA  Value            `json:"net_debt_to_ebitda"`
	InterestCoverage Value            `json:"interest_coverage"`
}

// LiquidityLeverageSeries is the liquidity/leverage series section: one row
// per period, chronological, sourced from Input.Ratios.History.
type LiquidityLeverageSeries struct {
	// Available is true when Input.Ratios.Available was true and History
	// was non-empty.
	Available bool `json:"available"`
	// FormulaVersion echoes Input.Ratios.FormulaVersion.
	FormulaVersion string                    `json:"formula_version,omitempty"`
	Periods        []LiquidityLeveragePeriod `json:"periods,omitempty"`
}

// CashFlowPeriod is one period's cash-flow bridge figures — sourced from
// Input.CashFlow.History (analytics/cashflow.Bridge) and
// Input.CashFlow.Conversion (analytics/cashflow.ConversionRatios) for the
// same period.
type CashFlowPeriod struct {
	Period                    financial.Period `json:"period"`
	EBITDA                    Value            `json:"ebitda"`
	OperatingCashFlow         Value            `json:"operating_cash_flow"`
	Capex                     Value            `json:"capex"`
	FreeCashFlow              Value            `json:"free_cash_flow"`
	EBITDAToOperatingCashFlow Value            `json:"ebitda_to_operating_cash_flow"`
	EBITDAToFreeCashFlow      Value            `json:"ebitda_to_free_cash_flow"`
}

// CashFlowSeries is the cash-flow series section: one row per period,
// chronological, sourced from Input.CashFlow, plus the most recent
// CashRunway reading when available.
type CashFlowSeries struct {
	// Available is true when Input.CashFlow.Available was true.
	Available bool `json:"available"`
	// FormulaVersion echoes Input.CashFlow.FormulaVersion.
	FormulaVersion string           `json:"formula_version,omitempty"`
	Periods        []CashFlowPeriod `json:"periods,omitempty"`
	// MonthsOfRunway is Input.CashFlow.CashRunway.MonthsOfRunway, when
	// Input.CashFlow.CashRunway.Available.
	MonthsOfRunway Value `json:"months_of_runway"`
}

// WorkingCapitalPeriod is one period's net-working-capital figures —
// sourced from Input.WorkingCapital.History (analytics/workingcapital.PeriodNWC).
type WorkingCapitalPeriod struct {
	Period              financial.Period `json:"period"`
	NWC                 Value            `json:"nwc"`
	NWCPercentOfRevenue Value            `json:"nwc_percent_of_revenue"`
}

// WorkingCapitalSeries is the working-capital series section: one row per
// period, chronological, sourced from Input.WorkingCapital.History, plus
// summary statistics and the suggested peg when available.
type WorkingCapitalSeries struct {
	// Available is true when Input.WorkingCapital.Available was true.
	Available bool `json:"available"`
	// FormulaVersion echoes Input.WorkingCapital.FormulaVersion.
	FormulaVersion string                 `json:"formula_version,omitempty"`
	Periods        []WorkingCapitalPeriod `json:"periods,omitempty"`
	// AverageNWC is Input.WorkingCapital.NWCStatistics.Average.
	AverageNWC Value `json:"average_nwc"`
	// SuggestedPeg is Input.WorkingCapital.SuggestedPeg.Value, when
	// Input.WorkingCapital.SuggestedPeg.Value.Available.
	SuggestedPeg Value `json:"suggested_peg"`
}
