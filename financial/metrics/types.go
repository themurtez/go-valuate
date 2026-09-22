// Package metrics computes derived financial metrics (revenue totals, gross
// profit, EBITDA, SDE, working capital, debt, historical growth/volatility,
// etc.) from a normalized financial.FinancialDataset.
//
// This package is the single place valuation methods should get EBITDA,
// SDE, and similar figures from, rather than each method recomputing them
// independently. It performs no I/O, holds no package-global mutable state,
// and never mutates the financial.FinancialDataset it is given.
//
// Missing data is not zero. Every computed figure is a MetricValue, which
// distinguishes "calculated to be exactly 0" from "cannot be calculated
// because a required input is absent from the dataset." Callers must check
// MetricValue.Available before trusting MetricValue.Value.
package metrics

import "github.com/themurtez/go-valuate/financial"

// MetricValue represents a single computed figure that may or may not be
// calculable from the supplied data. Available is false when one or more
// required inputs were absent from the dataset; in that case Value is
// always 0 and must not be interpreted as a calculated result.
//
// This type exists specifically so callers can distinguish "EBITDA is
// $0" from "EBITDA cannot be calculated from the supplied data" — collapsing
// both to a bare 0.0 would silently misrepresent missing data as a real
// calculated value.
type MetricValue struct {
	// Available is true only if every input required to compute this metric
	// was present in the dataset.
	Available bool `json:"available"`
	// Value is the computed figure. Meaningful only when Available is true;
	// always 0 when Available is false.
	Value float64 `json:"value"`
}

// Unavailable is the canonical zero-information MetricValue, returned when a
// metric cannot be computed. Prefer this over a bare MetricValue{} literal
// for readability at call sites.
func Unavailable() MetricValue { return MetricValue{} }

// Available reports a MetricValue for a successfully computed figure.
func AvailableValue(v float64) MetricValue { return MetricValue{Available: true, Value: v} }

// Component is one named input that contributed to a computed metric,
// carried so callers can explain how a figure was produced (e.g. for a
// future report) without this package implementing a generic expression
// engine. Amount is the value actually used in the calculation, after
// dataset lookup, in the dataset's native sign convention (see the
// package's sign-convention note in metrics.go).
type Component struct {
	// Code identifies the canonical financial.Code or synthetic metric name
	// this component came from (e.g. "COGS_MATERIAL", or a synthetic
	// identifier like "TOTAL_REVENUE" for a component that is itself a
	// computed subtotal).
	Code string `json:"code"`
	// Label is a short human-readable description of the component.
	Label string `json:"label,omitempty"`
	// Amount is the value contributed by this component.
	Amount float64 `json:"amount"`
}

// MetricResult is a single computed metric for a single period, with enough
// metadata to explain how the value was derived. Not every metric collects
// Components; simple pass-through lookups (e.g. cash) may omit them.
type MetricResult struct {
	// Metric is the stable name of the computed metric (e.g. "EBITDA"). See
	// the Metric* constants.
	Metric string `json:"metric"`
	// Period is the reporting period this result applies to.
	Period financial.Period `json:"period"`
	// Value is the computed figure and its availability.
	Value MetricValue `json:"value"`
	// Formula is a short, fixed, human-readable description of exactly how
	// Value was computed (e.g. "Revenue - COGS"), independent of which
	// Components happened to be present. Always populated, even when Value
	// is unavailable, so callers can explain why a metric is defined the way
	// it is regardless of data availability.
	Formula string `json:"formula"`
	// Components lists the named inputs that were summed/combined to
	// produce Value, when Value.Available is true and the metric is a
	// composite of other figures. Omitted for simple lookups or when the
	// metric is unavailable.
	Components []Component `json:"components,omitempty"`
}

// Stable metric name constants, used as MetricResult.Metric and as map keys
// in Snapshot lookups. Names are part of this package's durable API surface:
// once published, a name's string value should not change.
const (
	MetricTotalRevenue       = "TOTAL_REVENUE"
	MetricProductRevenue     = "PRODUCT_REVENUE"
	MetricServiceRevenue     = "SERVICE_REVENUE"
	MetricRecurringRevenue   = "RECURRING_REVENUE"
	MetricOtherRevenue       = "OTHER_REVENUE"
	MetricTotalCOGS          = "TOTAL_COGS"
	MetricGrossProfit        = "GROSS_PROFIT"
	MetricGrossMargin        = "GROSS_MARGIN"
	MetricTotalOpex          = "TOTAL_OPEX"
	MetricEBIT               = "EBIT"
	MetricDepreciation       = "DEPRECIATION"
	MetricAmortization       = "AMORTIZATION"
	MetricEBITDA             = "EBITDA"
	MetricEBITDAMargin       = "EBITDA_MARGIN"
	MetricOwnerCompensation  = "OWNER_COMPENSATION"
	MetricSDE                = "SDE"
	MetricNetIncome          = "NET_INCOME"
	MetricCash               = "CASH"
	MetricAccountsReceivable = "ACCOUNTS_RECEIVABLE"
	MetricInventory          = "INVENTORY"
	MetricCurrentAssets      = "CURRENT_ASSETS"
	MetricAccountsPayable    = "ACCOUNTS_PAYABLE"
	MetricCurrentLiabilities = "CURRENT_LIABILITIES"
	MetricWorkingCapital     = "WORKING_CAPITAL"
	MetricShortTermDebt      = "SHORT_TERM_DEBT"
	MetricLongTermDebt       = "LONG_TERM_DEBT"
	MetricTotalDebt          = "TOTAL_DEBT"
	MetricNetDebt            = "NET_DEBT"
	MetricTangibleAssetValue = "TANGIBLE_ASSET_VALUE"
)

// Snapshot holds every per-period metric computed for one financial.Period.
// Each field is independently Available; a business missing a balance sheet
// (income-statement-only data) will simply have Available == false on every
// balance-sheet-derived field rather than failing the whole calculation.
type Snapshot struct {
	Period financial.Period `json:"period"`

	// Income statement metrics.
	TotalRevenue      MetricValue `json:"total_revenue"`
	ProductRevenue    MetricValue `json:"product_revenue"`
	ServiceRevenue    MetricValue `json:"service_revenue"`
	RecurringRevenue  MetricValue `json:"recurring_revenue"`
	OtherRevenue      MetricValue `json:"other_revenue"`
	TotalCOGS         MetricValue `json:"total_cogs"`
	GrossProfit       MetricValue `json:"gross_profit"`
	GrossMargin       MetricValue `json:"gross_margin"`
	TotalOpex         MetricValue `json:"total_opex"`
	EBIT              MetricValue `json:"ebit"`
	Depreciation      MetricValue `json:"depreciation"`
	Amortization      MetricValue `json:"amortization"`
	EBITDA            MetricValue `json:"ebitda"`
	EBITDAMargin      MetricValue `json:"ebitda_margin"`
	OwnerCompensation MetricValue `json:"owner_compensation"`
	SDE               MetricValue `json:"sde"`
	NetIncome         MetricValue `json:"net_income"`

	// Balance sheet metrics.
	Cash               MetricValue `json:"cash"`
	AccountsReceivable MetricValue `json:"accounts_receivable"`
	Inventory          MetricValue `json:"inventory"`
	CurrentAssets      MetricValue `json:"current_assets"`
	AccountsPayable    MetricValue `json:"accounts_payable"`
	CurrentLiabilities MetricValue `json:"current_liabilities"`
	WorkingCapital     MetricValue `json:"working_capital"`
	ShortTermDebt      MetricValue `json:"short_term_debt"`
	LongTermDebt       MetricValue `json:"long_term_debt"`
	TotalDebt          MetricValue `json:"total_debt"`
	NetDebt            MetricValue `json:"net_debt"`
	TangibleAssetValue MetricValue `json:"tangible_asset_value"`

	// Results carries every computed MetricResult for this period (same
	// values as the typed fields above, plus Formula/Components), keyed by
	// metric name, for callers that want explainability without knowing
	// each field name up front.
	Results map[string]MetricResult `json:"results"`
}
