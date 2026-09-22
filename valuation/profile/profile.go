// Package profile defines a minimal, domain-level description of the
// business being valued, used only to drive method-applicability scoring
// (see valuation/applicability). It is deliberately not a CRM/business
// entity model: it holds no name, address, contact, ownership, or
// account/client/tenant concept — every one of those belongs to a
// consuming application, not to this deterministic valuation core (see the
// repository README's list of intentionally-out-of-scope concerns).
//
// Every field is optional (pointers, or a zero value that is itself a
// legitimate "unknown/not applicable" signal — see each field's doc
// comment). A Profile with every field left unset is valid input:
// applicability rules degrade to their most conservative behavior rather
// than failing, since a caller may simply not have collected every
// indicator yet. This package performs no I/O and holds no state; it only
// defines the shape of the profile and a handful of trivial accessors.
package profile

// Industry is a coarse business-model category used by applicability
// rules to distinguish, e.g., an asset-heavy manufacturer from an
// asset-light services business. This is intentionally coarser than a
// full NAICS/SIC taxonomy — applicability rules only need to know the
// handful of structural traits captured by AssetIntensity and
// RecurringRevenuePercent below, not an exhaustive industry code list —
// but Industry is retained as a human-readable label and an extension
// point for future rules that do want finer-grained categorization.
type Industry string

const (
	IndustryUnknown        Industry = ""
	IndustryServices       Industry = "services"
	IndustryTrades         Industry = "trades"
	IndustryRetail         Industry = "retail"
	IndustryManufacturing  Industry = "manufacturing"
	IndustryDistribution   Industry = "distribution"
	IndustryTechnologySaaS Industry = "technology_saas"
	IndustryHealthcare     Industry = "healthcare"
	IndustryConstruction   Industry = "construction"
	IndustryHospitality    Industry = "hospitality"
	IndustryOther          Industry = "other"
)

// EarningsStability is a caller-supplied qualitative read on how
// predictable the business's historical earnings have been (e.g. derived
// from financial/metrics' trend/volatility output, or from an analyst's
// judgment) — this package does not compute it from raw financials itself.
type EarningsStability string

const (
	EarningsStabilityUnknown   EarningsStability = ""
	EarningsStabilityStable    EarningsStability = "stable"
	EarningsStabilityVariable  EarningsStability = "variable"
	EarningsStabilityVolatile  EarningsStability = "volatile"
	EarningsStabilityDeclining EarningsStability = "declining"
)

// Profitability is a caller-supplied qualitative read on current
// profitability, independent of the exact earnings figures a valuation
// method consumes separately.
type Profitability string

const (
	ProfitabilityUnknown      Profitability = ""
	ProfitabilityStrong       Profitability = "strong"
	ProfitabilityModerate     Profitability = "moderate"
	ProfitabilityMarginal     Profitability = "marginal"
	ProfitabilityUnprofitable Profitability = "unprofitable"
)

// DataAvailability records which inputs the caller actually has on hand
// for this business, independent of whether a given valuation method would
// otherwise be a good conceptual fit. Applicability rules use this to
// separate "this method suits this kind of business" from "this method
// cannot run because the necessary input was never supplied" — the two are
// different reasons for the same NOT_APPLICABLE/LOW outcome and are
// reported with different Reason text (see valuation/applicability).
type DataAvailability struct {
	// HasMultiYearFinancials indicates whether more than one period of
	// financial history is available (affects confidence in earnings-based
	// methods and trend-derived stability signals).
	HasMultiYearFinancials bool `json:"has_multi_year_financials,omitempty"`
	// HasBalanceSheet indicates whether balance-sheet data (assets/
	// liabilities) is available at all — required for the adjusted net
	// asset value method to be applicable, as opposed to merely
	// calculable from a caller-assembled item list.
	HasBalanceSheet bool `json:"has_balance_sheet,omitempty"`
	// HasForecast indicates whether the caller has (or intends to supply)
	// explicit forecast free cash flows for a DCF — this package never
	// generates a forecast (see valuation/dcf's package doc comment), so a
	// DCF is only applicable when this is true.
	HasForecast bool `json:"has_forecast,omitempty"`
	// HasAppraisedAssetValues indicates whether at least one balance-sheet
	// item has an independent fair-value/appraisal override available
	// (mirrors valuation/netassets.AssetItem.IsOverride), which strengthens
	// (but is not required for) net asset value applicability.
	HasAppraisedAssetValues bool `json:"has_appraised_asset_values,omitempty"`
}

// Profile is the minimal set of business-level indicators used by
// valuation/applicability's deterministic rules. Every numeric field is a
// pointer so nil unambiguously means "unknown / not supplied," mirroring
// settings.Settings' convention: applicability rules must be able to tell
// "this business has zero recurring revenue" apart from "the caller never
// answered this question," since the two warrant different Reason text
// (see valuation/applicability).
type Profile struct {
	// Industry is a coarse business-model category. IndustryUnknown (the
	// zero value) is a legitimate "not supplied" state.
	Industry Industry `json:"industry,omitempty"`
	// OwnerOperated indicates whether the business is run day-to-day by
	// its owner, whose compensation/involvement is a primary driver of
	// reported earnings (the central SDE-vs-EBITDA distinction). nil means
	// unknown.
	OwnerOperated *bool `json:"owner_operated,omitempty"`
	// AnnualRevenue is the most recent full-year revenue figure, used only
	// as a coarse size signal for applicability (e.g. distinguishing a
	// main-street business from a lower-middle-market one) — not itself a
	// valuation input. nil means unknown.
	AnnualRevenue *float64 `json:"annual_revenue,omitempty"`
	// EmployeeCount is the approximate headcount, another coarse size/
	// owner-dependency signal. nil means unknown.
	EmployeeCount *int `json:"employee_count,omitempty"`
	// AssetIntensity is the approximate ratio of tangible operating assets
	// to revenue (or another caller-chosen consistent basis), expressed as
	// a decimal (e.g. 0.6 = tangible assets worth 60% of annual revenue).
	// This package does not compute it from a metrics.Snapshot itself —
	// the caller supplies whatever ratio its own basis produces. Higher
	// values push applicability toward asset-based/EBITDA methods and away
	// from SDE. nil means unknown.
	AssetIntensity *float64 `json:"asset_intensity,omitempty"`
	// RecurringRevenuePercent is the share of revenue that is contractually
	// recurring/subscription-like, expressed as a decimal (0.0-1.0+; not
	// clamped by this package — see Validate). Higher values are commonly
	// associated with growth/SaaS-style businesses where a DCF (given an
	// explicit forecast) is a better conceptual fit than a single-period
	// multiple. nil means unknown.
	RecurringRevenuePercent *float64 `json:"recurring_revenue_percent,omitempty"`
	// HistoricalGrowthRate is the caller-supplied recent revenue or
	// earnings growth rate, expressed as a decimal (e.g.
	// financial/metrics.CAGRResult.Value from a trend calculation this
	// package does not perform itself). nil means unknown.
	HistoricalGrowthRate *float64 `json:"historical_growth_rate,omitempty"`
	// EarningsStability is a qualitative read on earnings predictability.
	// EarningsStabilityUnknown (the zero value) is a legitimate "not
	// supplied" state.
	EarningsStability EarningsStability `json:"earnings_stability,omitempty"`
	// Profitability is a qualitative read on current profitability.
	// ProfitabilityUnknown (the zero value) is a legitimate "not supplied"
	// state.
	Profitability Profitability `json:"profitability,omitempty"`
	// YearsInOperation is how long the business has operated. nil means
	// unknown.
	YearsInOperation *int `json:"years_in_operation,omitempty"`
	// DataAvailability records which underlying inputs the caller actually
	// has on hand. The zero value (every flag false) is a legitimate,
	// conservative "nothing confirmed available" state.
	DataAvailability DataAvailability `json:"data_availability,omitempty"`
}

// IsOwnerOperated reports Profile.OwnerOperated's value, treating an
// unset (nil) field as unknown-false: applicability rules that special-case
// owner-operated businesses only fire on an explicit true, never assume it.
func (p Profile) IsOwnerOperated() bool {
	return p.OwnerOperated != nil && *p.OwnerOperated
}
