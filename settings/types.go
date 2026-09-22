// Package settings implements a generic, hierarchical valuation settings
// resolver with four scopes — system, account, client, and valuation — and
// strict override precedence between them.
//
// This package has no knowledge of what an "account" or "client" is; those
// are concepts owned by the consuming application. It only knows about four
// ordered layers of settings values and how to combine them. The consuming
// application is responsible for constructing the four Settings objects
// (e.g. by loading them from its own storage) and supplying them to Resolve.
//
// Settings are plain, JSON-compatible structs. This package does not persist
// anything; the Resolution returned by Resolve is meant to be used
// immediately and, if desired, snapshotted by the caller.
package settings

// Scope identifies one of the four settings layers, in increasing order of
// precedence.
type Scope string

const (
	ScopeSystem    Scope = "system"
	ScopeAccount   Scope = "account"
	ScopeClient    Scope = "client"
	ScopeValuation Scope = "valuation"
)

// scopeOrder lists scopes from lowest to highest precedence. Later entries
// override earlier ones. This is the single source of truth for precedence
// order used by the resolver.
var scopeOrder = []Scope{ScopeSystem, ScopeAccount, ScopeClient, ScopeValuation}

// Method identifies a valuation method that can be enabled or disabled via
// settings.
type Method string

const (
	MethodSDE                    Method = "sde"
	MethodEBITDA                 Method = "ebitda"
	MethodDCF                    Method = "dcf"
	MethodCapitalizationEarnings Method = "capitalization_of_earnings"
	MethodAdjustedNetAssetValue  Method = "adjusted_net_asset_value"
)

// allMethods lists every known Method, for deterministic iteration.
var allMethods = []Method{
	MethodSDE,
	MethodEBITDA,
	MethodDCF,
	MethodCapitalizationEarnings,
	MethodAdjustedNetAssetValue,
}

// Settings is a single layer of valuation settings at one Scope. Every
// numeric field is a pointer so that nil unambiguously means "unset /
// inherit from the next lower-precedence scope," while a non-nil pointer to
// 0 means "explicitly set to zero." The same distinction applies to method
// enablement flags via MethodEnabled.
//
// A Settings value with every field nil is valid and simply contributes
// nothing at its scope; resolution falls through to lower scopes for every
// field.
type Settings struct {
	// Scope identifies which layer this Settings value represents. Resolve
	// uses this to determine precedence, not the struct's position in any
	// particular argument.
	Scope Scope `json:"scope"`

	// SDEMultiple is the multiple applied to Seller's Discretionary
	// Earnings.
	SDEMultiple *float64 `json:"sde_multiple,omitempty"`
	// EBITDAMultiple is the multiple applied to EBITDA.
	EBITDAMultiple *float64 `json:"ebitda_multiple,omitempty"`
	// DiscountRate is the rate used to discount future cash flows (e.g. in
	// a DCF), expressed as a decimal (0.15 = 15%).
	DiscountRate *float64 `json:"discount_rate,omitempty"`
	// TerminalGrowthRate is the long-run growth rate used for terminal
	// value calculations, expressed as a decimal.
	TerminalGrowthRate *float64 `json:"terminal_growth_rate,omitempty"`
	// TaxRate is the effective tax rate used in valuation calculations,
	// expressed as a decimal.
	TaxRate *float64 `json:"tax_rate,omitempty"`
	// MarketabilityDiscount is the discount for lack of marketability,
	// expressed as a decimal.
	MarketabilityDiscount *float64 `json:"marketability_discount,omitempty"`
	// ControlPremium is the premium for a controlling interest, expressed
	// as a decimal.
	ControlPremium *float64 `json:"control_premium,omitempty"`

	// MethodEnabled maps a valuation Method to whether it is enabled at
	// this scope. A method absent from the map (or present with a nil
	// value) is unset/inherit; a present non-nil value is an explicit
	// enable (true) or disable (false).
	MethodEnabled map[Method]*bool `json:"method_enabled,omitempty"`
}

// numericFieldSpec describes one numeric settings field for generic
// resolution: how to read the field's pointer out of a Settings value, and
// the key it should be reported under in Resolution.Values/Sources.
type numericFieldSpec struct {
	key string
	get func(*Settings) *float64
}

// numericFields lists every numeric settings field. Defined once so
// Resolve, and anything else that needs to enumerate fields, stay in sync
// by construction.
var numericFields = []numericFieldSpec{
	{"sde_multiple", func(s *Settings) *float64 { return s.SDEMultiple }},
	{"ebitda_multiple", func(s *Settings) *float64 { return s.EBITDAMultiple }},
	{"discount_rate", func(s *Settings) *float64 { return s.DiscountRate }},
	{"terminal_growth_rate", func(s *Settings) *float64 { return s.TerminalGrowthRate }},
	{"tax_rate", func(s *Settings) *float64 { return s.TaxRate }},
	{"marketability_discount", func(s *Settings) *float64 { return s.MarketabilityDiscount }},
	{"control_premium", func(s *Settings) *float64 { return s.ControlPremium }},
}

// Float64 returns a pointer to v, for convenience when constructing
// Settings values with explicit numeric overrides (including explicit
// zero).
func Float64(v float64) *float64 { return &v }

// Bool returns a pointer to v, for convenience when constructing Settings
// values with explicit method-enable overrides (including explicit false).
func Bool(v bool) *bool { return &v }
