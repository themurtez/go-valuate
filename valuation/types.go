// Package valuation defines the common result envelope, value-type
// vocabulary, and enterprise-value-to-equity-value bridge shared by every
// individual valuation method (financial/../valuation/sde, ebitda,
// capitalization, dcf, netassets).
//
// This package holds no calculation logic of its own — each method package
// computes its own figure under its own strongly-typed Input, and reports it
// through the shared Result/Step/Issue/Bridge types defined here, so a
// caller (or a future orchestrator/consensus engine) can display, compare,
// and eventually persist results from every method uniformly without each
// method reinventing its own ad hoc result shape.
//
// Like every other package in this repository, valuation performs no I/O,
// holds no package-global mutable state, and never mutates its inputs.
// Nothing here knows about a database, an HTTP handler, a UI, or an
// application's concept of an "account" or "client" — see the repository
// README for the full list of intentionally-out-of-scope concerns.
package valuation

// Code is a stable identifier for one individual valuation method, meant to
// be matched on by callers (e.g. an orchestrator selecting which methods to
// run, or a persistence layer recording which method produced a historical
// valuation) independent of any human-readable label. Once published, a
// Code's string value must never change or be reused for a different
// method.
type Code string

const (
	// CodeSDEMultiple identifies the Seller's Discretionary Earnings
	// multiple method (valuation/sde).
	CodeSDEMultiple Code = "SDE_MULTIPLE"
	// CodeEBITDAMultiple identifies the EBITDA multiple method
	// (valuation/ebitda).
	CodeEBITDAMultiple Code = "EBITDA_MULTIPLE"
	// CodeCapitalizationOfEarnings identifies the capitalization-of-earnings
	// method (valuation/capitalization).
	CodeCapitalizationOfEarnings Code = "CAPITALIZATION_OF_EARNINGS"
	// CodeDCF identifies the discounted-cash-flow method (valuation/dcf).
	CodeDCF Code = "DCF"
	// CodeAdjustedNetAssetValue identifies the adjusted-net-asset-value
	// method (valuation/netassets).
	CodeAdjustedNetAssetValue Code = "ADJUSTED_NET_ASSET_VALUE"
)

// ValueType states which kind of value a method's calculated figure
// represents. Every method result carries exactly one ValueType, and
// methods never mix value types silently — where an enterprise-value method
// can be bridged to an equity value (see Bridge), that conversion is a
// separate, explicit, inspectable step, never folded invisibly into the
// headline number.
type ValueType string

const (
	// ValueTypeEnterprise means the calculated value represents Enterprise
	// Value: the value of the business's operations, capital-structure
	// neutral (before subtracting debt or adding back excess cash/
	// non-operating assets).
	ValueTypeEnterprise ValueType = "enterprise_value"
	// ValueTypeEquity means the calculated value represents Equity Value:
	// the value attributable to the owners of the business after accounting
	// for debt and cash (Enterprise Value + Excess Cash - Debt, or a
	// method's own direct equity calculation — see each method's doc
	// comment for which convention it uses).
	ValueTypeEquity ValueType = "equity_value"
	// ValueTypeAsset means the calculated value represents a net asset
	// value: adjusted assets minus adjusted liabilities. Distinct from both
	// Enterprise and Equity value — see valuation/netassets' doc comment for
	// how a net asset value relates to (but is not necessarily equal to) an
	// equity value.
	ValueTypeAsset ValueType = "asset_value"
)

// Step is one labeled figure in a method's calculation trace: an
// intermediate or final number a reviewer would want to see on the way from
// raw inputs to the headline result, in the order the calculation actually
// produced them. Every method appends every step of its calculation here —
// nothing material is computed and then discarded before reaching Result.
type Step struct {
	// Label is a short, fixed, human-readable description of what this step
	// computed (e.g. "Enterprise Value = EBITDA x Multiple"). Stable across
	// calls with the same method version; not meant to be templated with
	// caller data (that belongs in Detail).
	Label string `json:"label"`
	// Value is the numeric result of this step.
	Value float64 `json:"value"`
	// Detail optionally elaborates on Value with caller-specific context
	// (e.g. "8,500,000 x 3.20"). Omitted when Label alone is sufficient.
	Detail string `json:"detail,omitempty"`
}

// IssueSeverity distinguishes a problem that prevents a method from
// producing a usable value (SeverityError) from one worth a reviewer's
// attention that does not by itself invalidate the result (SeverityWarning)
// — mirroring financial/adjustments.IssueSeverity and
// financial/reconciliation.Status's error/warning split rather than
// collapsing every problem into a single boolean.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of validation problem, in
// the same spirit as financial/adjustments.IssueCode and
// financial/reconciliation.CheckCode: durable and meant to be matched on,
// independent of Issue.Message's wording. Each method package defines its
// own IssueCode constants (e.g. sde.IssueNonPositiveMultiple) since the set
// of things that can go wrong is method-specific; this package only defines
// the handful of problems common to every method's Input.
type IssueCode string

const (
	// IssueNonFiniteInput means a numeric input field was NaN or +/-Inf.
	// Common to every method that takes caller-supplied numeric inputs.
	IssueNonFiniteInput IssueCode = "NON_FINITE_INPUT"
)

// Issue is a single validation finding produced while checking a method's
// Input, before or during Calculate.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// Errors filters issues down to just those with SeverityError, for callers
// that want to check "did anything block this result" without scanning the
// full mixed list themselves.
func Errors(issues []Issue) []Issue {
	var out []Issue
	for _, iss := range issues {
		if iss.Severity == SeverityError {
			out = append(out, iss)
		}
	}
	return out
}

// Warnings filters issues down to just those with SeverityWarning.
func Warnings(issues []Issue) []Issue {
	var out []Issue
	for _, iss := range issues {
		if iss.Severity == SeverityWarning {
			out = append(out, iss)
		}
	}
	return out
}

// HasErrors reports whether any Issue in issues has SeverityError.
func HasErrors(issues []Issue) bool {
	for _, iss := range issues {
		if iss.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Bridge is an explicit, itemized walk from an Enterprise Value to an
// Equity Value:
//
//	Equity Value = Enterprise Value + Excess Cash - Total Debt
//
// This is the one convention every method in this repository uses to move
// between the two value types (see each method's doc comment for exactly
// which Snapshot/caller-supplied fields feed ExcessCash and TotalDebt).
// Debt and cash are never netted into an unexplained final number: a Bridge
// is always returned as its own inspectable value alongside the Enterprise
// Value it was computed from, never silently substituted for it.
//
// A caller that disagrees with this convention (e.g. wants to net only a
// portion of cash as "excess," or use a different debt definition) supplies
// different ExcessCash/TotalDebt figures into the method's Input rather
// than this package offering a second convention — see each method's
// EquityBridgeInput-shaped field.
type Bridge struct {
	// Available is false if a caller did not supply enough information to
	// compute an equity value (e.g. no debt/cash figures were given).
	// EnterpriseValue-typed methods remain fully valid with Available ==
	// false: the bridge is optional, not a requirement for the method's
	// headline result to be usable.
	Available bool `json:"available"`
	// EnterpriseValue is the value the bridge starts from. Meaningful only
	// when Available is true; equal to the method's own EnterpriseValue
	// result field.
	EnterpriseValue float64 `json:"enterprise_value"`
	// ExcessCash is the cash/cash-equivalents figure added back. Zero if the
	// caller supplied no cash adjustment (a documented, non-silent zero, not
	// a missing value — see each method's Input doc comment on how a bridge
	// becomes Available in the first place).
	ExcessCash float64 `json:"excess_cash"`
	// TotalDebt is the debt figure subtracted. Zero if the caller supplied
	// no debt.
	TotalDebt float64 `json:"total_debt"`
	// DebtComponents itemizes what made up TotalDebt (e.g. short-term debt,
	// long-term debt, other supplied debt), in the order supplied, so the
	// bridge is never a single opaque number — see each method's Input for
	// which components it accepts.
	DebtComponents []Component `json:"debt_components,omitempty"`
	// EquityValue is EnterpriseValue + ExcessCash - TotalDebt. Meaningful
	// only when Available is true.
	EquityValue float64 `json:"equity_value"`
}

// Component is one named figure that contributed to a Bridge or other
// composite calculation, mirroring financial/metrics.Component's role:
// carried so callers can explain how a total was produced without this
// package implementing a generic expression engine.
type Component struct {
	// Label is a short human-readable description of the component (e.g.
	// "Short-Term Debt").
	Label string `json:"label"`
	// Amount is the value contributed by this component, in the sign it
	// contributes with (e.g. always non-negative for a debt component that
	// is always subtracted by the caller-facing formula).
	Amount float64 `json:"amount"`
}
