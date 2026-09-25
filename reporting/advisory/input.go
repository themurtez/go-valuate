package advisory

import (
	"github.com/themurtez/go-valuate/accounting/ap"
	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/accounting/cashforecast"
	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/inventory"
	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
	"github.com/themurtez/go-valuate/accounting/vendorspend"
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/consolidation"
	"github.com/themurtez/go-valuate/analytics/covenants"
	"github.com/themurtez/go-valuate/analytics/debt"
	analyticsdiagnostics "github.com/themurtez/go-valuate/analytics/diagnostics"
	"github.com/themurtez/go-valuate/analytics/forecast"
	"github.com/themurtez/go-valuate/analytics/kpi"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/analytics/valuedrivers"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial/metrics"
	portfoliodiagnostics "github.com/themurtez/go-valuate/portfolio/diagnostics"
	"github.com/themurtez/go-valuate/transactions/acquisition"
	"github.com/themurtez/go-valuate/transactions/dealstructure"
	"github.com/themurtez/go-valuate/transactions/salereadiness"
	"github.com/themurtez/go-valuate/valuation/consensus"
)

// EntityScope is metadata-only, describing what one [Result] represents —
// task section 50. Build never consolidates multiple entities itself; a
// COMPANY/PORTFOLIO/CONSOLIDATED distinction here is purely descriptive of
// what the caller already assembled upstream (via analytics/consolidation
// or portfolio/diagnostics) before calling Build.
type EntityScope string

const (
	ScopeCompany      EntityScope = "COMPANY"
	ScopeBusinessUnit EntityScope = "BUSINESS_UNIT"
	ScopePortfolio    EntityScope = "PORTFOLIO"
	ScopeConsolidated EntityScope = "CONSOLIDATED"
)

// CompanyContext is caller-supplied, opaque display/identity metadata —
// task section 4. Every field is passed through unchanged; Build performs
// no lookup and infers no industry, currency, or scope from any other
// Input field.
type CompanyContext struct {
	CompanyID         string      `json:"company_id,omitempty"`
	CompanyName       string      `json:"company_name,omitempty"`
	ReportingCurrency string      `json:"reporting_currency,omitempty"`
	CurrentPeriod     string      `json:"current_period,omitempty"`
	PriorPeriod       string      `json:"prior_period,omitempty"`
	FiscalYear        int         `json:"fiscal_year,omitempty"`
	IndustryLabel     string      `json:"industry_label,omitempty"`
	EntityScope       EntityScope `json:"entity_scope,omitempty"`
}

// PeriodInfo is one caller-supplied, ordered period — task section 49.
// Build never parses a period label to infer chronology; Sequence alone
// determines ordering among Periods entries, and a Metric/Insight's own
// Period string is matched against Periods by Code for
// IsCurrent/IsPrior classification, never guessed.
type PeriodInfo struct {
	// Code is this period's stable label, matched verbatim against every
	// composed Metric/Insight's own Period field (which in turn echoes
	// whatever period-label convention the source sibling package used,
	// e.g. financial.Period's string value).
	Code string `json:"code"`
	// Label is an optional, caller-supplied human-readable display label
	// (e.g. "Q1 FY2026"), distinct from Code. Falls back to Code when
	// empty.
	Label string `json:"label,omitempty"`
	// Sequence orders Periods entries; a lower Sequence is earlier. Ties
	// are broken by Periods' own slice order (stable).
	Sequence int `json:"sequence"`
	// IsCurrent/IsPrior mark this period as the pack's current/prior
	// period for current-vs-prior comparison — task section 10. At most
	// one Periods entry should set each; Build uses the first it
	// encounters (input order) if more than one does, and reports
	// IssueInvalidPeriod as advisory-only in that case.
	IsCurrent bool `json:"is_current,omitempty"`
	IsPrior   bool `json:"is_prior,omitempty"`
}

// FinancialInputs bundles the core financial-performance/liquidity/
// working-capital/revenue/debt sibling results — task section 3's
// grouping. Every field is independently optional.
type FinancialInputs struct {
	Metrics        metrics.Result        `json:"metrics"`
	Ratios         ratios.Result         `json:"ratios"`
	WorkingCapital workingcapital.Result `json:"working_capital"`
	RevenueQuality revenuequality.Result `json:"revenue_quality"`
	Concentration  concentration.Result  `json:"concentration"`
	Debt           debt.Result           `json:"debt"`
	Covenants      covenants.Result      `json:"covenants"`
	ValueDrivers   valuedrivers.Result   `json:"value_drivers"`
	Consolidation  consolidation.Result  `json:"consolidation"`
	// Forecast is the already-computed analytics/forecast.Result — the
	// financial/operating forecast, distinct from
	// Operating.CashForecast's 13-week liquidity forecast (task section
	// 27's explicit "do not merge them into one model" rule; see
	// section_forecast.go).
	Forecast forecast.Result `json:"forecast"`
	// Diagnostics is the already-computed analytics/diagnostics.Result
	// (which itself mines up to 15 other optional sibling Results' own
	// Flags/Signals/Anomalies/Status into Findings) — a cross-cutting
	// meta-analysis source, distinct from portfolio/diagnostics (see
	// OperatingInputs.Portfolio) which scans across multiple businesses
	// rather than mining one business's own sibling Results.
	Diagnostics analyticsdiagnostics.Result `json:"diagnostics"`
}

// OperatingInputs bundles the operational sibling results — AR/AP/
// inventory/labor/profitability/vendor spend/cash forecast — task section
// 3's grouping.
type OperatingInputs struct {
	AR            ar.Result            `json:"ar"`
	AP            ap.Result            `json:"ap"`
	Inventory     inventory.Result     `json:"inventory"`
	Labor         labor.Result         `json:"labor"`
	Profitability profitability.Result `json:"profitability"`
	VendorSpend   vendorspend.Result   `json:"vendor_spend"`
	CashForecast  cashforecast.Result  `json:"cash_forecast"`

	// Portfolio is the optional portfolio/diagnostics.Result supplied when
	// this pack's EntityScope is ScopePortfolio — task section 51. Build
	// never inspects individual portfolio companies beyond what this
	// already-computed Result exposes.
	Portfolio portfoliodiagnostics.Result `json:"portfolio"`
}

// CloseInputs bundles the accounting/close sibling results — task section
// 3's grouping.
type CloseInputs struct {
	Reconciliation reconciliation.Result `json:"reconciliation"`
	CloseQuality   closequality.Result   `json:"close_quality"`
	CloseChecklist closechecklist.Result `json:"close_checklist"`
}

// TransactionInputs bundles the transaction-readiness sibling results —
// task section 3's grouping. Every field is optional; task section 29's
// "do not infer that a company should sell/acquire" rule means Build only
// ever reports these Results' own facts.
type TransactionInputs struct {
	SaleReadiness salereadiness.Result `json:"sale_readiness"`
	Acquisition   acquisition.Result   `json:"acquisition"`
	DealStructure dealstructure.Result `json:"deal_structure"`
}

// ValuationInputs bundles the already-computed valuation sibling results —
// task section 3's grouping. Build never reruns a valuation method; see
// section_valuation.go.
type ValuationInputs struct {
	Consensus consensus.Result `json:"consensus"`
}

// Input bundles every optional section this pack can be built from — task
// section 3. Every field beyond Periods/Company is independently
// optional; Build degrades gracefully when a field is left at its zero
// value, exactly like every sibling composition package in this
// repository (reporting/management.Input, transactions/salereadiness.Input,
// portfolio/diagnostics.Input). Build never mutates Input or any of its
// nested Results.
type Input struct {
	Periods []PeriodInfo `json:"periods,omitempty"`

	Company     CompanyContext    `json:"company"`
	Financial   FinancialInputs   `json:"financial"`
	Operating   OperatingInputs   `json:"operating"`
	Close       CloseInputs       `json:"close"`
	Transaction TransactionInputs `json:"transaction"`
	Valuation   ValuationInputs   `json:"valuation"`

	KPIValues []kpi.KPIResult `json:"kpi_values,omitempty"`

	// CallerActions is every caller-supplied custom action item — task
	// section 38. Marked ActionOriginCallerSupplied automatically by
	// Build; wording is never altered.
	CallerActions []ActionItem `json:"caller_actions,omitempty"`

	// Prior is a previously built Result from an earlier call to Build,
	// used for current/prior insight/action/metric comparison — task
	// section 41/102. Optional; nil means no prior-comparison sections are
	// populated.
	Prior *Result `json:"prior,omitempty"`
}

// currentAndPriorPeriod returns the Code of the Periods entry marked
// IsCurrent/IsPrior respectively (first-encountered in input order — see
// PeriodInfo's doc comment), or "" if none is marked.
func currentAndPriorPeriod(periods []PeriodInfo) (current, prior string) {
	for _, p := range periods {
		if p.IsCurrent && current == "" {
			current = p.Code
		}
		if p.IsPrior && prior == "" {
			prior = p.Code
		}
	}
	return current, prior
}
