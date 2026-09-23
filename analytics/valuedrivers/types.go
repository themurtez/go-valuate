// Package valuedrivers shows how deterministic, named changes to a
// business's underlying metrics and assumptions ("drivers") flow through
// to valuation — not by inventing a new valuation formula, but by
// mutating the same strongly-typed method Inputs valuation/orchestrator
// already accepts, re-running the caller's already-selected methods via
// orchestrator.Execute, and re-running valuation/consensus.Calculate with
// the caller's own Options/weights, exactly as if the caller had built
// that changed Request by hand.
//
// This package computes no valuation figure itself and defines no new
// pricing formula: every EquityValue/EnterpriseValue/AdjustedNetAssetValue
// in a ScenarioResult comes directly from the same valuation/sde, ebitda,
// capitalization, dcf, or netassets Calculate a caller would invoke
// directly, run against a mutated copy of the baseline Request. See
// apply.go for the one place a Driver's mutation is applied.
//
// Explicit linkage only. Every DriverType names exactly which method
// Input field(s) it mutates (see the DriverType constants' doc comments).
// A Driver never touches a method it has no documented linkage to: e.g. a
// DriverMultipleChange targeting valuation.CodeSDEMultiple never touches
// DCF's DiscountRate, and a caller who wants "recurring revenue
// percentage" to change a method's multiple must supply the resulting
// multiple explicitly via DriverMethodMultipleRule — this package never
// derives a multiple from a percentage on the caller's behalf (see
// DriverMethodMultipleRule's doc comment). Every ScenarioResult reports,
// per method, whether that method was actually touched (Linkage) so a
// caller/report never has to guess whether a "no effect" result reflects
// a real absence of causation or a driver that simply wasn't wired up to
// that method.
//
// Availability, not silence. A Driver that names a method with no Input
// supplied in the baseline Request, or whose targeted field does not
// exist for its DriverType/method combination, produces
// LinkageNotApplicable for that method (not an error) — the scenario
// still runs to completion for every other method it does apply to. A
// structurally invalid Driver (e.g. an empty DriverType, or a
// DriverMultipleChange with neither ChangePercent nor NewValue set) is
// recorded as a SeverityError Issue and that Driver contributes no
// mutation at all, rather than silently doing nothing.
//
// Determinism and no mutation of caller input. Calculate never mutates
// Input.BaselineRequest or any Driver/Scenario the caller supplied — every
// method Input is deep-copied before a Driver's mutation is applied (see
// cloneRequest in apply.go). Calculate is pure: no I/O, no package-global
// mutable state, and identical input always produces byte-for-byte
// identical JSON output (see determinism_test.go).
package valuedrivers

import (
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
)

// FormulaVersion identifies this package's fixed driver-application rules
// (see each DriverType constant's doc comment for the exact mutation each
// applies) and delta/percentage-delta formulas. Bump this whenever any of
// that changes in a way that could make a historical Result not reproduce
// identically under new code — see the repository README's
// versioning-strategy section.
const FormulaVersion = "1.0.0"

// DriverType is a closed set of deterministic, explicitly-linked ways a
// Driver can change the baseline Request before it is re-run. Every
// DriverType's doc comment states exactly which method(s) and Input
// field(s) it mutates — see the package doc comment on explicit linkage.
type DriverType string

const (
	// DriverRevenueGrowth scales a revenue-driven earnings figure by
	// (1 + Driver.RevenueGrowth.GrowthPercent): MaintainableEBITDA
	// (valuation.CodeEBITDAMultiple), MaintainableSDE
	// (valuation.CodeSDEMultiple), MaintainableEarnings
	// (valuation.CodeCapitalizationOfEarnings), and every
	// ForecastPeriod.FreeCashFlow (valuation.CodeDCF) are each multiplied
	// by the same factor. Requires Driver.RevenueGrowth.
	DriverRevenueGrowth DriverType = "REVENUE_GROWTH"
	// DriverMarginChange adds Driver.MarginChange.MarginPointsDelta (in
	// decimal points, e.g. 0.02 = +2 points) times
	// Driver.MarginChange.RevenueBase to the maintainable-earnings figure
	// of valuation.CodeEBITDAMultiple, valuation.CodeSDEMultiple, and
	// valuation.CodeCapitalizationOfEarnings — a margin-point change is
	// only meaningful relative to a revenue base, which this package never
	// invents (see MarginChangeParams.RevenueBase). Requires
	// Driver.MarginChange.
	DriverMarginChange DriverType = "MARGIN_CHANGE"
	// DriverSDEChange changes MaintainableSDE directly (SDE multiple
	// method only), either by an additive dollar amount or a percentage,
	// per Driver.SDEChange. Use this instead of DriverRevenueGrowth when
	// the caller's SDE change does not move in lockstep with EBITDA/
	// earnings (e.g. an owner-specific SDE addback change that has no
	// EBITDA/earnings-base counterpart). Requires Driver.SDEChange.
	DriverSDEChange DriverType = "SDE_CHANGE"
	// DriverMultipleChange changes Input.Multiple for
	// valuation.CodeSDEMultiple and/or valuation.CodeEBITDAMultiple —
	// whichever of Driver.MultipleChange.Methods names — either by an
	// additive delta or an absolute replacement value, per
	// MultipleChangeParams. Requires Driver.MultipleChange.
	DriverMultipleChange DriverType = "MULTIPLE_CHANGE"
	// DriverCapRateChange changes Input.CapitalizationRate for
	// valuation.CodeCapitalizationOfEarnings, either by an additive delta
	// or an absolute replacement value, per CapRateChangeParams. Requires
	// Driver.CapRateChange.
	DriverCapRateChange DriverType = "CAP_RATE_CHANGE"
	// DriverDiscountRateChange changes Input.DiscountRate and/or
	// Input.TerminalGrowthRate for valuation.CodeDCF, either by an
	// additive delta or an absolute replacement value, per
	// DiscountRateChangeParams. Requires Driver.DiscountRateChange.
	DriverDiscountRateChange DriverType = "DISCOUNT_RATE_CHANGE"
	// DriverDebtChange changes the debt/cash bridge inputs
	// (ExcessCash/ShortTermDebt/LongTermDebt/OtherDebt) for
	// valuation.CodeSDEMultiple, valuation.CodeEBITDAMultiple, and
	// valuation.CodeDCF — every method whose EquityBridge.Requested is
	// true in the baseline Request — by an additive delta per method
	// field, per DebtChangeParams. A method whose baseline EquityBridge
	// was not Requested is left untouched (LinkageNotApplicable): this
	// driver adjusts an already-requested bridge, it never opts a method
	// into computing one it did not already ask for. Requires
	// Driver.DebtChange.
	DriverDebtChange DriverType = "DEBT_CHANGE"
	// DriverWorkingCapitalChange changes a caller-named debt-bridge field
	// (almost always OtherDebt, modeling an incremental working-capital
	// investment as a debt-like claim, or ExcessCash, modeling a working-
	// capital release) by an additive delta, for the same
	// EquityBridge.Requested-gated method set as DriverDebtChange. This
	// package has no native "net working capital" field on any method
	// Input (see analytics/workingcapital for NWC analysis itself); a
	// caller models its valuation effect exactly the way any other
	// bridge-affecting driver does, by naming which bridge field it moves
	// via WorkingCapitalChangeParams.BridgeField — never through an
	// invented separate mechanism. Requires Driver.WorkingCapitalChange.
	DriverWorkingCapitalChange DriverType = "WORKING_CAPITAL_CHANGE"
	// DriverOwnerCompensationAdjustment adds
	// Driver.OwnerCompensationAdjustment.Amount to MaintainableSDE
	// (valuation.CodeSDEMultiple), and additionally to MaintainableEBITDA
	// (valuation.CodeEBITDAMultiple) and MaintainableEarnings
	// (valuation.CodeCapitalizationOfEarnings) only if
	// Driver.OwnerCompensationAdjustment.AppliesBeyondSDE is true — an
	// owner-compensation addback conventionally affects SDE by
	// definition, but whether the same dollar addback also belongs in an
	// EBITDA/earnings base depends on how the caller's own normalization
	// already treated owner compensation (see
	// financial/adjustments.AdjustmentType for that upstream decision),
	// which this package does not re-derive. Requires
	// Driver.OwnerCompensationAdjustment.
	DriverOwnerCompensationAdjustment DriverType = "OWNER_COMPENSATION_ADJUSTMENT"
	// DriverCustomerLossImpact models the earnings effect of losing a
	// caller-identified slice of revenue: RevenueAtRiskAmount (or
	// RevenueBase * RevenueAtRiskPercent) times
	// EarningsMarginOnLostRevenue is subtracted from the maintainable-
	// earnings figure of valuation.CodeEBITDAMultiple,
	// valuation.CodeSDEMultiple, and valuation.CodeCapitalizationOfEarnings,
	// and the same dollar reduction is applied to the final forecast
	// period's FreeCashFlow for valuation.CodeDCF (mirroring
	// DriverRevenueGrowth's DCF treatment, but as a one-time level shift
	// applied to every forecast period rather than a scaling factor — see
	// CustomerLossImpactParams' doc comment). Requires
	// Driver.CustomerLossImpact.
	DriverCustomerLossImpact DriverType = "CUSTOMER_LOSS_IMPACT"
	// DriverMethodMultipleRule directly replaces Input.Multiple for a
	// caller-named method (valuation.CodeSDEMultiple or
	// valuation.CodeEBITDAMultiple) with a caller-supplied NewMultiple,
	// labeled with a caller-supplied TriggerLabel/TriggerValue purely for
	// explainability (e.g. "Recurring revenue crosses 60%" /
	// "62%"). This is the only mechanism this package provides for a
	// business-quality metric like recurring-revenue percentage to affect
	// a multiple: the package never computes NewMultiple itself from
	// TriggerValue or any other input — see the package doc comment and
	// MethodMultipleRuleParams' doc comment. Requires
	// Driver.MethodMultipleRule.
	DriverMethodMultipleRule DriverType = "METHOD_MULTIPLE_RULE"
)

// RevenueGrowthParams parameterizes DriverRevenueGrowth.
type RevenueGrowthParams struct {
	// GrowthPercent is the revenue growth rate applied to every linked
	// method's earnings/cash-flow figure, expressed as a decimal (0.10 =
	// +10%, -0.05 = -5%). The multiplicative factor is (1 +
	// GrowthPercent); every linked figure is multiplied by it.
	GrowthPercent float64 `json:"growth_percent"`
}

// MarginChangeParams parameterizes DriverMarginChange.
type MarginChangeParams struct {
	// MarginPointsDelta is the change in margin, in decimal points (0.02 =
	// +2 points, -0.015 = -1.5 points).
	MarginPointsDelta float64 `json:"margin_points_delta"`
	// RevenueBase is the revenue figure MarginPointsDelta is applied
	// against to derive a dollar earnings change: dollar delta =
	// MarginPointsDelta * RevenueBase. Required and must be > 0 — a margin
	// point change has no dollar meaning without a revenue base, and this
	// package never substitutes one from a method Input (no method Input
	// carries a revenue figure at all). See IssueMissingRevenueBase.
	RevenueBase float64 `json:"revenue_base"`
}

// SDEChangeParams parameterizes DriverSDEChange. Exactly one of
// AmountDelta/PercentDelta should be nonzero; if both are nonzero,
// AmountDelta and PercentDelta are both applied (dollar delta first, then
// the percentage factor on the result), which is almost never what a
// caller intends — see IssueBothDeltaKindsSet.
type SDEChangeParams struct {
	// AmountDelta is an additive dollar change to MaintainableSDE.
	AmountDelta float64 `json:"amount_delta,omitempty"`
	// PercentDelta is a multiplicative change to MaintainableSDE,
	// expressed as a decimal (0.10 = +10%): applied as (1 + PercentDelta).
	PercentDelta float64 `json:"percent_delta,omitempty"`
}

// MultipleChangeMethod names which method(s) a multiple-shaped driver
// (DriverMultipleChange) targets.
type MultipleChangeMethod string

const (
	MultipleChangeSDE    MultipleChangeMethod = "SDE"
	MultipleChangeEBITDA MultipleChangeMethod = "EBITDA"
)

// MultipleChangeParams parameterizes DriverMultipleChange. Exactly one of
// ChangeDelta/NewValue should be set; NewValue, if nonzero, takes
// precedence over ChangeDelta (an absolute replacement is unambiguous,
// unlike a delta, so there is no equivalent "both set" ambiguity to
// reject the way SDEChangeParams does) — see the Multiple field's doc
// comment on Linkage.AppliedValue always reporting the resulting
// absolute value either way.
type MultipleChangeParams struct {
	// Methods lists which method(s) this change applies to. Required;
	// empty produces IssueNoTargetMethod.
	Methods []MultipleChangeMethod `json:"methods"`
	// ChangeDelta is an additive change to Input.Multiple (e.g. +0.5 to go
	// from a 4.0x to a 4.5x multiple). Ignored if NewValue is nonzero.
	ChangeDelta float64 `json:"change_delta,omitempty"`
	// NewValue, if nonzero, replaces Input.Multiple outright rather than
	// adjusting it by a delta.
	NewValue float64 `json:"new_value,omitempty"`
}

// CapRateChangeParams parameterizes DriverCapRateChange. Same
// delta-or-replacement shape as MultipleChangeParams, applied to
// Input.CapitalizationRate.
type CapRateChangeParams struct {
	ChangeDelta float64 `json:"change_delta,omitempty"`
	NewValue    float64 `json:"new_value,omitempty"`
}

// DiscountRateChangeParams parameterizes DriverDiscountRateChange. At
// least one of the two rate changes must be set (both zero produces
// IssueNoRateChange); each is independently delta-or-replacement shaped.
type DiscountRateChangeParams struct {
	// DiscountRateDelta is an additive change to Input.DiscountRate.
	// Ignored if DiscountRateNewValue is nonzero.
	DiscountRateDelta float64 `json:"discount_rate_delta,omitempty"`
	// DiscountRateNewValue, if nonzero, replaces Input.DiscountRate
	// outright.
	DiscountRateNewValue float64 `json:"discount_rate_new_value,omitempty"`
	// TerminalGrowthRateDelta is an additive change to
	// Input.TerminalGrowthRate. Ignored if TerminalGrowthRateNewValue is
	// nonzero.
	TerminalGrowthRateDelta float64 `json:"terminal_growth_rate_delta,omitempty"`
	// TerminalGrowthRateNewValue, if nonzero, replaces
	// Input.TerminalGrowthRate outright.
	TerminalGrowthRateNewValue float64 `json:"terminal_growth_rate_new_value,omitempty"`
}

// DebtChangeParams parameterizes DriverDebtChange: an additive dollar
// delta to apply to each named bridge component, for every method whose
// baseline EquityBridge.Requested was already true (see
// DriverDebtChange's doc comment). A zero field means "no change to that
// component," not "set to zero."
type DebtChangeParams struct {
	ExcessCashDelta    float64 `json:"excess_cash_delta,omitempty"`
	ShortTermDebtDelta float64 `json:"short_term_debt_delta,omitempty"`
	LongTermDebtDelta  float64 `json:"long_term_debt_delta,omitempty"`
	OtherDebtDelta     float64 `json:"other_debt_delta,omitempty"`
}

// BridgeField names a single debt/cash bridge component, used by
// WorkingCapitalChangeParams to state which one it is moving.
type BridgeField string

const (
	BridgeFieldExcessCash    BridgeField = "EXCESS_CASH"
	BridgeFieldShortTermDebt BridgeField = "SHORT_TERM_DEBT"
	BridgeFieldLongTermDebt  BridgeField = "LONG_TERM_DEBT"
	BridgeFieldOtherDebt     BridgeField = "OTHER_DEBT"
)

// WorkingCapitalChangeParams parameterizes DriverWorkingCapitalChange. See
// DriverWorkingCapitalChange's doc comment for why this reuses the debt
// bridge mechanism rather than a dedicated NWC field.
type WorkingCapitalChangeParams struct {
	// Field names which bridge component Amount is added to. Required; an
	// empty/unrecognized Field produces IssueInvalidBridgeField.
	Field BridgeField `json:"field"`
	// Amount is the additive dollar delta applied to Field, in the
	// direction that field's own sign convention already uses (e.g. a
	// positive Amount against BridgeFieldOtherDebt increases debt and
	// lowers equity value; a positive Amount against BridgeFieldExcessCash
	// increases cash and raises equity value).
	Amount float64 `json:"amount"`
}

// OwnerCompensationAdjustmentParams parameterizes
// DriverOwnerCompensationAdjustment.
type OwnerCompensationAdjustmentParams struct {
	// Amount is the additive dollar addback (or claw-back, if negative)
	// applied to MaintainableSDE, and — only if AppliesBeyondSDE — to
	// MaintainableEBITDA/MaintainableEarnings as well.
	Amount float64 `json:"amount"`
	// AppliesBeyondSDE opts into also applying Amount to
	// valuation.CodeEBITDAMultiple's MaintainableEBITDA and
	// valuation.CodeCapitalizationOfEarnings' MaintainableEarnings. false
	// (the default) applies the adjustment to SDE only — see
	// DriverOwnerCompensationAdjustment's doc comment.
	AppliesBeyondSDE bool `json:"applies_beyond_sde,omitempty"`
}

// CustomerLossImpactParams parameterizes DriverCustomerLossImpact. Exactly
// one of RevenueAtRiskAmount or (RevenueBase, RevenueAtRiskPercent) should
// be supplied; if RevenueAtRiskAmount is nonzero it takes precedence.
type CustomerLossImpactParams struct {
	// RevenueAtRiskAmount is the dollar amount of revenue at risk of loss.
	// Takes precedence over RevenueBase/RevenueAtRiskPercent when nonzero.
	RevenueAtRiskAmount float64 `json:"revenue_at_risk_amount,omitempty"`
	// RevenueBase and RevenueAtRiskPercent together derive the dollar
	// amount at risk (RevenueBase * RevenueAtRiskPercent) when
	// RevenueAtRiskAmount is left zero.
	RevenueBase          float64 `json:"revenue_base,omitempty"`
	RevenueAtRiskPercent float64 `json:"revenue_at_risk_percent,omitempty"`
	// EarningsMarginOnLostRevenue is the margin (decimal, e.g. 0.25 = 25%)
	// applied to the revenue-at-risk figure to derive the earnings
	// reduction: earnings delta = -(revenue at risk *
	// EarningsMarginOnLostRevenue). Required and must be > 0 — see
	// IssueMissingEarningsMargin.
	EarningsMarginOnLostRevenue float64 `json:"earnings_margin_on_lost_revenue"`
}

// MethodMultipleRuleParams parameterizes DriverMethodMultipleRule. See the
// package doc comment and DriverMethodMultipleRule's doc comment: this
// package applies NewMultiple verbatim and never derives it from
// TriggerValue or any other figure.
type MethodMultipleRuleParams struct {
	// Method names which method's Input.Multiple to replace. Required.
	Method MultipleChangeMethod `json:"method"`
	// NewMultiple is the resulting multiple, supplied entirely by the
	// caller. Must be > 0 — see IssueNonPositiveMultiple.
	NewMultiple float64 `json:"new_multiple"`
	// TriggerLabel and TriggerValue are purely descriptive metadata
	// explaining what caller-side rule produced NewMultiple (e.g.
	// TriggerLabel: "Recurring revenue percentage", TriggerValue: "62%").
	// Echoed on Linkage.Assumptions for explainability; never parsed or
	// used in any calculation.
	TriggerLabel string `json:"trigger_label,omitempty"`
	TriggerValue string `json:"trigger_value,omitempty"`
}

// Driver is a single named, typed scenario input: exactly one of the
// parameter fields below should be populated, selected by Type. An
// unpopulated (zero-value) parameter struct for the selected Type is
// itself a valid, if unusual, Driver (e.g. RevenueGrowthParams{
// GrowthPercent: 0} is a legal, no-op driver) — Calculate does not reject
// a zero-effect Driver outright, it reports it as applied with a zero
// delta, distinct from a structurally invalid Driver (see
// IssueUnrecognizedDriverType and each Params type's own required-field
// Issues).
type Driver struct {
	// ID is a caller-assigned stable identifier for this driver (e.g.
	// "REVENUE_GROWTH_10PCT"), echoed on every Linkage/Issue that
	// references it and used as the map key for
	// ScenarioResult.DriverResults in a combined scenario. Required;
	// Calculate records IssueMissingDriverID and skips the driver
	// (contributes no mutation) if empty.
	ID string `json:"id"`
	// Label is a short human-readable description (e.g. "10% revenue
	// growth"), for display only.
	Label string `json:"label,omitempty"`
	// Type selects which of the parameter fields below is read. Required;
	// an empty or unrecognized Type produces IssueUnrecognizedDriverType.
	Type DriverType `json:"type"`

	RevenueGrowth               *RevenueGrowthParams               `json:"revenue_growth,omitempty"`
	MarginChange                *MarginChangeParams                `json:"margin_change,omitempty"`
	SDEChange                   *SDEChangeParams                   `json:"sde_change,omitempty"`
	MultipleChange              *MultipleChangeParams              `json:"multiple_change,omitempty"`
	CapRateChange               *CapRateChangeParams               `json:"cap_rate_change,omitempty"`
	DiscountRateChange          *DiscountRateChangeParams          `json:"discount_rate_change,omitempty"`
	DebtChange                  *DebtChangeParams                  `json:"debt_change,omitempty"`
	WorkingCapitalChange        *WorkingCapitalChangeParams        `json:"working_capital_change,omitempty"`
	OwnerCompensationAdjustment *OwnerCompensationAdjustmentParams `json:"owner_compensation_adjustment,omitempty"`
	CustomerLossImpact          *CustomerLossImpactParams          `json:"customer_loss_impact,omitempty"`
	MethodMultipleRule          *MethodMultipleRuleParams          `json:"method_multiple_rule,omitempty"`
}

// LinkageStatus classifies whether/how a Driver actually affected one
// method within a Scenario.
type LinkageStatus string

const (
	// LinkageApplied means this Driver mutated at least one field of this
	// method's Input (even if the resulting numeric delta happened to be
	// zero, e.g. a GrowthPercent of 0 — see Driver's doc comment).
	LinkageApplied LinkageStatus = "applied"
	// LinkageNotApplicable means this DriverType has no documented
	// mutation for this method at all (e.g. DriverCapRateChange never
	// touches valuation.CodeDCF), or the method-specific precondition for
	// applying it was not met (e.g. DriverDebtChange targeting a method
	// whose baseline EquityBridge.Requested was false). Never an error —
	// see the package doc comment on availability, not silence.
	LinkageNotApplicable LinkageStatus = "not_applicable"
	// LinkageMethodExcluded means this method had no Input at all in the
	// baseline Request (so it was OutcomeExcluded with ExclusionNoInput in
	// every scenario, identical to the baseline) — nothing to mutate, and
	// nothing to compare.
	LinkageMethodExcluded LinkageStatus = "method_excluded"
)

// Linkage records exactly what one Driver did (or did not do) to one
// method's Input within a Scenario — the explicit "did this driver
// actually cause this method's result to change" audit trail the package
// doc comment describes.
type Linkage struct {
	// Method is the valuation.Code this Linkage describes.
	Method valuation.Code `json:"method"`
	// Status classifies whether the driver applied to this method.
	Status LinkageStatus `json:"status"`
	// Detail is a short human-readable explanation, always populated (e.g.
	// "multiplied MaintainableEBITDA by 1.10" or "no EquityBridge was
	// requested for this method in the baseline; debt driver has nothing
	// to adjust").
	Detail string `json:"detail"`
}

// DriverIssueSeverity distinguishes a problem that made a Driver
// contribute no mutation at all (SeverityError) from one worth surfacing
// that did not block the driver (SeverityWarning) — the same two-severity
// model every sibling analytics package uses.
type DriverIssueSeverity string

const (
	DriverSeverityError   DriverIssueSeverity = "error"
	DriverSeverityWarning DriverIssueSeverity = "warning"
)

// DriverIssueCode is a stable identifier for one kind of Driver/Scenario
// validation problem.
type DriverIssueCode string

const (
	// IssueMissingDriverID means Driver.ID was empty.
	IssueMissingDriverID DriverIssueCode = "MISSING_DRIVER_ID"
	// IssueUnrecognizedDriverType means Driver.Type was empty or not one
	// of the DriverType constants.
	IssueUnrecognizedDriverType DriverIssueCode = "UNRECOGNIZED_DRIVER_TYPE"
	// IssueMissingDriverParams means Driver.Type named a params field that
	// was left nil.
	IssueMissingDriverParams DriverIssueCode = "MISSING_DRIVER_PARAMS"
	// IssueMissingRevenueBase means MarginChangeParams.RevenueBase was <=
	// 0.
	IssueMissingRevenueBase DriverIssueCode = "MISSING_REVENUE_BASE"
	// IssueBothDeltaKindsSet is a warning noting that both
	// SDEChangeParams.AmountDelta and PercentDelta were nonzero.
	IssueBothDeltaKindsSet DriverIssueCode = "BOTH_DELTA_KINDS_SET"
	// IssueNoTargetMethod means MultipleChangeParams.Methods was empty, or
	// MethodMultipleRuleParams.Method was empty or not one of the
	// MultipleChangeMethod constants.
	IssueNoTargetMethod DriverIssueCode = "NO_TARGET_METHOD"
	// IssueNoRateChange means every field of DiscountRateChangeParams was
	// zero.
	IssueNoRateChange DriverIssueCode = "NO_RATE_CHANGE"
	// IssueInvalidBridgeField means
	// WorkingCapitalChangeParams.Field was empty or unrecognized.
	IssueInvalidBridgeField DriverIssueCode = "INVALID_BRIDGE_FIELD"
	// IssueMissingEarningsMargin means
	// CustomerLossImpactParams.EarningsMarginOnLostRevenue was <= 0.
	IssueMissingEarningsMargin DriverIssueCode = "MISSING_EARNINGS_MARGIN"
	// IssueNonPositiveMultiple means MethodMultipleRuleParams.NewMultiple
	// was <= 0.
	IssueNonPositiveMultiple DriverIssueCode = "NON_POSITIVE_MULTIPLE"
	// IssueNoLinkedMethod is a warning noting that, after applying every
	// per-method precondition, a Driver ended up with LinkageApplied for
	// zero methods (every targeted method was either excluded or not
	// applicable) — the driver ran without error but changed nothing.
	IssueNoLinkedMethod DriverIssueCode = "NO_LINKED_METHOD"
	// IssueDuplicateDriverID is a warning noting that two or more Drivers
	// in the same Scenario share a non-empty ID.
	IssueDuplicateDriverID DriverIssueCode = "DUPLICATE_DRIVER_ID"
	// IssueMissingScenarioID means Scenario.ID was empty. The
	// ScenarioResult is Available == false.
	IssueMissingScenarioID DriverIssueCode = "MISSING_SCENARIO_ID"
	// IssueEmptyScenario means Scenario.Drivers was empty. The
	// ScenarioResult is Available == false.
	IssueEmptyScenario DriverIssueCode = "EMPTY_SCENARIO"
)

// DriverIssue is a single Driver/Scenario-time validation finding.
type DriverIssue struct {
	Code     DriverIssueCode     `json:"code"`
	Severity DriverIssueSeverity `json:"severity"`
	Message  string              `json:"message"`
	// DriverID identifies which Driver.ID this DriverIssue relates to.
	// Empty when the issue is scenario-level rather than driver-specific.
	DriverID string `json:"driver_id,omitempty"`
}

// HasDriverErrors reports whether any DriverIssue in issues has
// DriverSeverityError.
//
// Intentionally duplicated from every sibling analytics package's own
// HasErrors rather than shared — see
// valuation.HasErrors's doc comment for the full rationale (each
// package's Issue is a distinct Go type with no common interface worth
// introducing for one boolean function).
func HasDriverErrors(issues []DriverIssue) bool {
	for _, i := range issues {
		if i.Severity == DriverSeverityError {
			return true
		}
	}
	return false
}

// Scenario is a named group of one or more Drivers applied together
// against the same baseline Request in a single re-run — the "combined
// scenario" the package brief calls for. A Scenario with exactly one
// Driver is how Calculate also represents each entry of its
// one-factor-at-a-time analysis (see Result.OneFactorAtATime), so both
// modes share one ScenarioResult shape.
type Scenario struct {
	// ID is a caller-assigned stable identifier for this scenario.
	// Required; Calculate records a scenario-level DriverIssue and
	// produces an unavailable ScenarioResult if empty.
	ID string `json:"id"`
	// Label is a short human-readable description (e.g. "Downside case:
	// revenue decline + margin compression"), for display only.
	Label string `json:"label,omitempty"`
	// Drivers is every Driver applied together in this Scenario, in
	// order. Applied in this order to the same cloned Request, so a later
	// Driver in the list sees an earlier Driver's already-mutated fields
	// (e.g. a DriverMultipleChange after a DriverRevenueGrowth still only
	// touches Multiple, but the earnings figure it multiplies against
	// already reflects the growth driver — deltas compound the way a
	// caller manually building the same combined Request by hand would
	// expect). Required; empty produces an unavailable ScenarioResult.
	Drivers []Driver `json:"drivers"`
}

// MethodValue is one method's headline figure plus its value type — the
// portable (basis, amount) pair this package uses wherever it needs to
// name a single method's result, mirroring
// valuation/report.headlineValue's return shape but as a named, JSON-safe
// struct rather than a private multi-return helper.
type MethodValue struct {
	// Available echoes the method Result's own Available field — a
	// changed Request can make a previously-available method unavailable
	// (e.g. a multiple driver pushing Multiple to a non-positive value)
	// just as validly as the reverse.
	Available bool `json:"available"`
	// ValueType is the method's value basis (enterprise/equity/asset).
	ValueType valuation.ValueType `json:"value_type,omitempty"`
	// Value is the method's headline figure on ValueType's basis.
	// Meaningful only when Available is true.
	Value float64 `json:"value,omitempty"`
}

// MethodDelta compares one method's baseline and scenario MethodValue.
type MethodDelta struct {
	// Method is the valuation.Code this MethodDelta describes.
	Method valuation.Code `json:"method"`
	// Baseline is this method's MethodValue under Result.Baseline.
	Baseline MethodValue `json:"baseline"`
	// Scenario is this method's MethodValue under this ScenarioResult.
	Scenario MethodValue `json:"scenario"`
	// ValueDelta is Scenario.Value - Baseline.Value. Meaningful only when
	// both Baseline.Available and Scenario.Available are true.
	ValueDelta float64 `json:"value_delta,omitempty"`
	// PercentDelta is ValueDelta / |Baseline.Value|, as a decimal. Zero
	// (not NaN/Inf) when Baseline.Value is zero — see percentOf.
	PercentDelta float64 `json:"percent_delta,omitempty"`
	// DeltaAvailable is true only when both Baseline.Available and
	// Scenario.Available are true, gating ValueDelta/PercentDelta — a
	// method that was available at baseline but became unavailable under
	// the scenario (or vice versa) has a meaningless numeric delta, which
	// this package never reports as a misleading zero.
	DeltaAvailable bool `json:"delta_available"`
	// Linkages records, per Driver.ID that was part of this Scenario,
	// whether/how it touched this method — see Linkage's doc comment. For
	// a single-driver Scenario (the one-factor-at-a-time case) this always
	// has exactly one entry.
	Linkages []Linkage `json:"linkages,omitempty"`
}

// ScenarioResult is the full outcome of re-running the baseline Request
// under one Scenario's Drivers.
type ScenarioResult struct {
	// ScenarioID/Label echo the source Scenario.
	ScenarioID string `json:"scenario_id"`
	Label      string `json:"label,omitempty"`
	// Available is false only if the Scenario itself was structurally
	// invalid (empty ID or empty Drivers) — every other field is then
	// zero-value. A Scenario whose Drivers were individually invalid but
	// present is still Available; the invalid Driver(s) simply contribute
	// no mutation (see Issues).
	Available bool `json:"available"`
	// ChangedInputs is a flattened, caller-facing list of every method
	// Input field this Scenario's Drivers actually changed, one entry per
	// (method, field) pair touched by at least one Driver, in method-then-
	// driver-then-field order. Purely descriptive (mirrors valuation.Step/
	// Assumption's role in other packages) — every figure here is also
	// implicitly reflected in Run/Consensus.
	ChangedInputs []ChangedInput `json:"changed_inputs,omitempty"`
	// Run is the full orchestrator.Run produced by re-executing the
	// mutated Request — every method's recalculated Result, in
	// orchestrator.Execute's fixed order, exactly as a caller would get
	// from calling orchestrator.Execute directly.
	Run orchestrator.Run `json:"run"`
	// Consensus is valuation/consensus.Calculate re-run over Run's
	// successful methods, using Result.ConsensusOptions/Weights — the
	// identical inputs used to compute Result.Baseline.Consensus, so the
	// two are directly comparable.
	Consensus consensus.Result `json:"consensus"`
	// MethodDeltas compares every method present in either the baseline
	// or this scenario's Run, in orchestrator's fixed method order (SDE,
	// EBITDA, Capitalization, DCF, NetAssets).
	MethodDeltas []MethodDelta `json:"method_deltas"`
	// ConsensusValueDelta is Consensus.Statistics.SimpleMean -
	// Result.Baseline.Consensus.Statistics.SimpleMean. Meaningful only
	// when ConsensusDeltaAvailable is true.
	ConsensusValueDelta float64 `json:"consensus_value_delta,omitempty"`
	// ConsensusPercentDelta is ConsensusValueDelta /
	// |Result.Baseline.Consensus.Statistics.SimpleMean|, as a decimal.
	ConsensusPercentDelta float64 `json:"consensus_percent_delta,omitempty"`
	// ConsensusDeltaAvailable is true only when both
	// Result.Baseline.Consensus.Available and Consensus.Available are
	// true.
	ConsensusDeltaAvailable bool `json:"consensus_delta_available"`
	// Assumptions is a flattened, human-readable summary of every Driver
	// applied in this Scenario (one entry per driver, echoing its
	// Label/Type and key parameter(s)), for direct display without a
	// caller re-deriving prose from typed Driver params itself.
	Assumptions []Assumption `json:"assumptions,omitempty"`
	// Issues carries every DriverIssue raised while applying this
	// Scenario's Drivers (both driver-level and scenario-level).
	Issues []DriverIssue `json:"issues,omitempty"`
}

// ChangedInput is one (method, field) pair a Scenario's Drivers changed,
// with the before/after value.
type ChangedInput struct {
	Method   valuation.Code `json:"method"`
	Field    string         `json:"field"`
	Before   float64        `json:"before"`
	After    float64        `json:"after"`
	DriverID string         `json:"driver_id"`
}

// Assumption is a single labeled (label, value) pair, mirroring
// valuation/report.Assumption's role — a small, presentation-neutral
// summary line.
type Assumption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Baseline bundles the unmutated baseline Run/Consensus Calculate
// computed from Input.BaselineRequest, so a caller can inspect exactly
// what every ScenarioResult's deltas are measured against without
// separately re-running orchestrator/consensus itself.
type Baseline struct {
	// Run is orchestrator.Execute(Input.BaselineRequest), unmutated.
	Run orchestrator.Run `json:"run"`
	// Consensus is consensus.Calculate over Run's successful methods,
	// using Input.ConsensusOptions/Input.Weights.
	Consensus consensus.Result `json:"consensus"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// BaselineRequest is the unmutated baseline orchestrator.Request —
	// selected methods, their Inputs, settings/applicability filtering —
	// that every Driver/Scenario mutates a deep copy of. Required.
	// Calculate never mutates this value (see the package doc comment).
	BaselineRequest orchestrator.Request `json:"baseline_request"`
	// ConsensusOptions is passed verbatim to every consensus.Calculate
	// call this package makes (baseline and every scenario), so every
	// consensus figure in Result shares one value basis — see
	// valuation/consensus's package doc comment on value basis.
	ConsensusOptions consensus.Options `json:"consensus_options"`
	// Weights is passed verbatim to
	// valuation/report.BuildConsensusInputs-equivalent construction for
	// every consensus.Calculate call this package makes, keyed by
	// valuation.Code, mirroring
	// valuation/report.BuildConsensusInputs(run, weights)'s own
	// signature.
	Weights map[valuation.Code]float64 `json:"weights,omitempty"`
	// Drivers is the full set of individually-named drivers Calculate
	// analyzes one-factor-at-a-time (see Result.OneFactorAtATime) — each
	// entry becomes its own single-driver Scenario, run independently
	// against a fresh clone of BaselineRequest. Optional; a caller
	// wanting only combined-scenario analysis leaves this empty.
	Drivers []Driver `json:"drivers,omitempty"`
	// Scenarios is the full set of caller-named combined scenarios
	// Calculate analyzes (see Result.Scenarios) — each Scenario's Drivers
	// are applied together against a fresh clone of BaselineRequest.
	// Optional; a caller wanting only one-factor-at-a-time analysis leaves
	// this empty.
	Scenarios []Scenario `json:"scenarios,omitempty"`
}

// Result is the output of Calculate.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// driver-application/delta formulas produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Input.BaselineRequest could not be run at
	// all — in practice this never occurs (orchestrator.Execute never
	// itself fails; every method independently succeeds/is
	// unavailable/is excluded), so this exists purely as a defensive
	// regression guard, mirroring every sibling package's Available
	// convention despite this package having no realistic way to trip it.
	Available bool `json:"available"`
	// Baseline is the unmutated baseline Run/Consensus every delta in this
	// Result is measured against.
	Baseline Baseline `json:"baseline"`
	// OneFactorAtATime is one ScenarioResult per Input.Drivers entry, in
	// the same order, each representing that single Driver applied alone
	// against a fresh clone of Input.BaselineRequest.
	OneFactorAtATime []ScenarioResult `json:"one_factor_at_a_time,omitempty"`
	// Scenarios is one ScenarioResult per Input.Scenarios entry, in the
	// same order, each representing that Scenario's full Drivers list
	// applied together against a fresh clone of Input.BaselineRequest.
	Scenarios []ScenarioResult `json:"scenarios,omitempty"`
	// Issues carries every top-level DriverIssue not already attached to
	// a specific ScenarioResult (e.g. an empty BaselineRequest warning).
	Issues []DriverIssue `json:"issues,omitempty"`
}
