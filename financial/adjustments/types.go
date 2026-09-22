// Package adjustments models explicit, human-supplied normalization
// adjustments (owner compensation normalization, personal expenses run
// through the business, one-time items, related-party rent, etc.) and
// applies them to financial/metrics output to produce transparent,
// auditable "normalized EBITDA" and "normalized SDE" bridges.
//
// This package does not decide which adjustments exist for a given
// business, does not store them, and does not know who created them or
// where they came from (SourceRef is an opaque caller-supplied string). It
// only models the adjustment shape, validates a supplied set for internal
// consistency, and applies that set to a metrics.Snapshot. Every function
// here is pure: same (snapshot, adjustments) in, same Result out, every
// time, with no I/O and no mutation of its inputs.
//
// Sign convention. Every Adjustment carries an unsigned Amount (a
// magnitude, never a signed delta) plus an explicit Effect
// (EffectIncrease or EffectDecrease) stating whether applying it makes the
// target metric larger or smaller. This is deliberate: a bare signed
// number ("-50000") forces every caller and reviewer to separately
// remember whether negative means "this is an expense being added back" or
// "this reduces earnings," and that convention is exactly the kind of
// thing that gets flipped by accident. Amount and Effect are validated and
// applied independently, so "remove non-operating income" is expressed as
// Amount: 75000, Effect: EffectDecrease — never as Amount: -75000.
package adjustments

import "github.com/themurtez/go-valuate/financial"

// ID is a caller-supplied stable identifier for a single Adjustment, unique
// within the set passed to Apply. This package does not generate IDs and
// does not care what scheme the caller uses (UUID, database primary key,
// sequential string); it only requires that IDs be non-empty and unique
// within a single Apply call — see Validate.
type ID string

// Effect states whether applying an Adjustment makes the target metric
// larger or smaller. Effect is always evaluated together with Amount (a
// non-negative magnitude — see Adjustment.Amount): there is no signed
// "delta" anywhere in this package's public API.
type Effect string

const (
	// EffectIncrease means Amount is added to the target metric.
	EffectIncrease Effect = "increase"
	// EffectDecrease means Amount is subtracted from the target metric.
	EffectDecrease Effect = "decrease"
)

// Target identifies which normalized metric bridge an Adjustment applies
// to. A single Adjustment may apply to more than one target (see
// Adjustment.Targets) since some items — most notably owner compensation —
// are treated differently in an EBITDA bridge than in an SDE bridge (see
// the package doc comment on OwnerCompensation handling, and Apply's doc
// comment for the exact double-counting rule).
type Target string

const (
	// TargetEBITDA means this adjustment participates in the normalized
	// EBITDA bridge (see BridgeEBITDA).
	TargetEBITDA Target = "ebitda"
	// TargetSDE means this adjustment participates in the normalized SDE
	// bridge (see BridgeSDE).
	TargetSDE Target = "sde"
)

// Type identifies the kind of normalization adjustment. Type is a plain
// string (like financial.Code) specifically so new types can be added
// later — including by a caller building on this package, via
// RegisterType — without breaking existing callers or requiring a change
// to this package's exported surface. Existing type values are a stable,
// durable contract: once published, a Type's string value must never
// change or be reused for a different meaning.
type Type string

// Built-in adjustment types. See TypeMeta and DefaultTypeMeta for each
// type's default Targets/Effect/label; every one of these defaults can be
// overridden per-Adjustment (see Adjustment.Targets/Effect).
const (
	// TypeOwnerCompensationNormalization adjusts recorded owner
	// compensation up or down to a market-rate replacement-owner salary.
	// Applies to SDE only by default: EBITDA's baseline calculation
	// (financial/metrics) never adds back owner compensation in the first
	// place, so there is nothing to normalize in an EBITDA bridge — see
	// Apply's doc comment.
	TypeOwnerCompensationNormalization Type = "owner_compensation_normalization"
	// TypeOwnerDiscretionaryExpense is a personal or discretionary expense
	// run through the business by an owner (club dues, personal insurance,
	// etc.) that is not a legitimate cost of operating the business.
	// Increases both EBITDA and SDE by default (adding it back).
	TypeOwnerDiscretionaryExpense Type = "owner_discretionary_expense"
	// TypePersonalVehicle is a personal-use vehicle expense run through the
	// business. Increases both EBITDA and SDE by default.
	TypePersonalVehicle Type = "personal_vehicle"
	// TypePersonalTravel is personal travel expense run through the
	// business. Increases both EBITDA and SDE by default.
	TypePersonalTravel Type = "personal_travel"
	// TypeOneTimeExpense is a non-recurring expense (e.g. a one-time
	// equipment repair, a legal settlement) that should not be projected
	// forward. Increases both EBITDA and SDE by default.
	TypeOneTimeExpense Type = "one_time_expense"
	// TypeNonRecurringProfessionalFees is a non-recurring legal,
	// accounting, or consulting fee (e.g. one-time transaction or
	// litigation costs). Increases both EBITDA and SDE by default.
	TypeNonRecurringProfessionalFees Type = "non_recurring_professional_fees"
	// TypeRelatedPartyRentAdjustment adjusts rent paid to a related party
	// (an owner-affiliated landlord) to fair-market rent. Unlike the other
	// expense-side types, this one has no fixed default Effect: replacing
	// below-market rent with fair-market rent decreases earnings, while
	// replacing above-market rent increases them. The caller must supply
	// Effect explicitly for this type — see Validate.
	TypeRelatedPartyRentAdjustment Type = "related_party_rent_adjustment"
	// TypeNonOperatingIncome is income unrelated to core operations (e.g. a
	// one-time asset sale gain, investment income). Decreases both EBITDA
	// and SDE by default: removing non-operating income from normalized
	// earnings lowers the normalized figure, which is why this package
	// treats sign explicitness as a hard requirement (see the package doc
	// comment) rather than letting callers reach for a "negative add-back."
	TypeNonOperatingIncome Type = "non_operating_income"
	// TypeUnusualGain is a one-time, non-operating gain (e.g. litigation
	// proceeds, an insurance settlement in excess of loss). Decreases both
	// EBITDA and SDE by default, for the same reason as
	// TypeNonOperatingIncome.
	TypeUnusualGain Type = "unusual_gain"
	// TypeUnusualLoss is a one-time, non-operating loss. Increases both
	// EBITDA and SDE by default (adding back a loss that depressed
	// reported earnings but will not recur).
	TypeUnusualLoss Type = "unusual_loss"
	// TypeCustom is a caller-defined adjustment with no built-in default
	// Targets or Effect; both must be supplied explicitly on the
	// Adjustment. Use this for anything not covered by a more specific
	// type, rather than misusing an existing type for a different meaning.
	TypeCustom Type = "custom"
)

// TypeMeta is descriptive/default metadata about an adjustment Type,
// mirroring financial.CodeMeta's role for financial.Code: the Type's
// string value is the stable identifier, while label and defaults are
// separate metadata that can be extended without touching existing types.
type TypeMeta struct {
	Type Type `json:"type"`
	// Label is a short human-readable description of the type.
	Label string `json:"label"`
	// DefaultTargets lists which Target(s) this type applies to when an
	// Adjustment of this Type does not explicitly set Targets. Empty for
	// TypeCustom and TypeRelatedPartyRentAdjustment, which have no safe
	// default and require the caller to set Targets explicitly.
	DefaultTargets []Target `json:"default_targets,omitempty"`
	// DefaultEffect is the Effect an Adjustment of this Type uses when it
	// does not explicitly set Effect. Empty for TypeCustom and
	// TypeRelatedPartyRentAdjustment, which have no safe default and
	// require the caller to set Effect explicitly (see Validate).
	DefaultEffect Effect `json:"default_effect,omitempty"`
}

// typeRegistry is the canonical source of built-in Type metadata, mirroring
// financial's codeRegistry. Built once at init time and never mutated.
var typeRegistry = buildTypeRegistry()

func buildTypeRegistry() map[Type]TypeMeta {
	entries := []TypeMeta{
		{TypeOwnerCompensationNormalization, "Owner Compensation Normalization", []Target{TargetSDE}, ""},
		{TypeOwnerDiscretionaryExpense, "Owner Discretionary Expense", []Target{TargetEBITDA, TargetSDE}, EffectIncrease},
		{TypePersonalVehicle, "Personal Vehicle Expense", []Target{TargetEBITDA, TargetSDE}, EffectIncrease},
		{TypePersonalTravel, "Personal Travel Expense", []Target{TargetEBITDA, TargetSDE}, EffectIncrease},
		{TypeOneTimeExpense, "One-Time Expense", []Target{TargetEBITDA, TargetSDE}, EffectIncrease},
		{TypeNonRecurringProfessionalFees, "Non-Recurring Professional Fees", []Target{TargetEBITDA, TargetSDE}, EffectIncrease},
		{TypeRelatedPartyRentAdjustment, "Related-Party Rent Adjustment", nil, ""},
		{TypeNonOperatingIncome, "Non-Operating Income Removal", []Target{TargetEBITDA, TargetSDE}, EffectDecrease},
		{TypeUnusualGain, "Unusual Gain", []Target{TargetEBITDA, TargetSDE}, EffectDecrease},
		{TypeUnusualLoss, "Unusual Loss", []Target{TargetEBITDA, TargetSDE}, EffectIncrease},
		{TypeCustom, "Custom Adjustment", nil, ""},
	}
	registry := make(map[Type]TypeMeta, len(entries))
	for _, e := range entries {
		registry[e.Type] = e
	}
	return registry
}

// LookupType returns metadata for an adjustment Type and whether it is
// known to this package. A Type not found here is not necessarily
// invalid — a caller-defined type used consistently is legal (see the
// package doc comment on Type) — but Validate requires an unknown Type to
// carry explicit Targets and Effect, since no default is available.
func LookupType(t Type) (TypeMeta, bool) {
	meta, ok := typeRegistry[t]
	return meta, ok
}

// AllTypes returns metadata for every built-in Type, in a fixed
// declaration order (not sorted — the order groups related types the way
// a UI would want to present them).
func AllTypes() []TypeMeta {
	order := []Type{
		TypeOwnerCompensationNormalization,
		TypeOwnerDiscretionaryExpense,
		TypePersonalVehicle,
		TypePersonalTravel,
		TypeOneTimeExpense,
		TypeNonRecurringProfessionalFees,
		TypeRelatedPartyRentAdjustment,
		TypeNonOperatingIncome,
		TypeUnusualGain,
		TypeUnusualLoss,
		TypeCustom,
	}
	metas := make([]TypeMeta, 0, len(order))
	for _, t := range order {
		metas = append(metas, typeRegistry[t])
	}
	return metas
}

// Adjustment is a single explicit normalization adjustment supplied by a
// caller (an accountant, an analyst, or a UI backed by human review). This
// package has no idea who created it or where it is persisted — Adjustment
// carries only the domain data needed to validate and apply it.
type Adjustment struct {
	// ID is a caller-supplied stable identifier, unique within the set
	// passed to a single Apply/Validate call. Required.
	ID ID `json:"id"`
	// Period is the reporting period this adjustment applies to. Required,
	// and must be a period present in the metrics.Snapshot it is applied
	// against — see Validate.
	Period financial.Period `json:"period"`
	// Type identifies the kind of adjustment. Required. See the Type
	// constants and LookupType.
	Type Type `json:"type"`
	// Amount is a non-negative magnitude — never a signed delta. Whether
	// applying this adjustment increases or decreases the target metric is
	// controlled entirely by Effect, never by Amount's sign. See the
	// package doc comment.
	Amount float64 `json:"amount"`
	// Effect states whether Amount increases or decreases the target
	// metric. If empty, Validate/Apply fall back to Type's DefaultEffect
	// (see TypeMeta); if Type has no default (TypeCustom,
	// TypeRelatedPartyRentAdjustment, or an unrecognized Type), Effect must
	// be supplied explicitly or validation fails.
	Effect Effect `json:"effect,omitempty"`
	// Targets lists which bridge(s) (EBITDA, SDE, or both) this adjustment
	// participates in. If empty, Validate/Apply fall back to Type's
	// DefaultTargets; if Type has no default, Targets must be supplied
	// explicitly or validation fails.
	Targets []Target `json:"targets,omitempty"`
	// Reason is a short human-readable description of why this adjustment
	// exists (e.g. "Replace owner salary with market-rate GM
	// compensation"). Required — an adjustment with no stated reason is not
	// auditable.
	Reason string `json:"reason"`
	// SourceCode optionally names the canonical financial.Code this
	// adjustment relates to or is derived from (e.g.
	// financial.CodeOpexOwnerComp for an owner compensation normalization,
	// financial.CodeOpexVehicle for a personal-vehicle add-back). Purely
	// informational/traceability; Apply does not require the code to be
	// present in the dataset unless the adjustment's Type specifically
	// needs a base amount from it (see BaseMetric/RequiresBaseMetric
	// below).
	SourceCode financial.Code `json:"source_code,omitempty"`
	// SourceRef is an opaque, caller-defined provenance reference (e.g. a
	// document ID, a row ID, a ticket number). This package never
	// interprets it — see the package doc comment on not knowing who
	// created an adjustment or where it is stored.
	SourceRef string `json:"source_ref,omitempty"`
	// Notes is optional freeform commentary beyond Reason.
	Notes string `json:"notes,omitempty"`
	// Included controls whether this adjustment participates in Apply's
	// bridges. false means the adjustment is recorded but intentionally
	// excluded (e.g. proposed but not yet confirmed by the accountant) —
	// Apply reports it under Result.Skipped with reason "not included"
	// rather than silently dropping it. The zero value is false, so
	// callers must explicitly opt an adjustment in.
	Included bool `json:"included"`
}

// resolvedEffect returns adj.Effect if set, else Type's DefaultEffect via
// LookupType. The bool result is false if neither is available.
func (adj Adjustment) resolvedEffect() (Effect, bool) {
	if adj.Effect != "" {
		return adj.Effect, true
	}
	meta, ok := LookupType(adj.Type)
	if !ok || meta.DefaultEffect == "" {
		return "", false
	}
	return meta.DefaultEffect, true
}

// resolvedTargets returns adj.Targets if set, else Type's DefaultTargets
// via LookupType.
func (adj Adjustment) resolvedTargets() []Target {
	if len(adj.Targets) > 0 {
		return adj.Targets
	}
	meta, ok := LookupType(adj.Type)
	if !ok {
		return nil
	}
	return meta.DefaultTargets
}

// appliesTo reports whether this adjustment participates in the bridge for
// target, after resolving defaults.
func (adj Adjustment) appliesTo(target Target) bool {
	for _, t := range adj.resolvedTargets() {
		if t == target {
			return true
		}
	}
	return false
}

// signedAmount returns Amount with the sign implied by Effect: positive
// for EffectIncrease, negative for EffectDecrease. ok is false if Effect
// could not be resolved. This is the one place in the package a signed
// number is derived from Amount+Effect — everywhere else in the public API
// stays unsigned-magnitude-plus-explicit-direction.
func (adj Adjustment) signedAmount() (value float64, ok bool) {
	effect, ok := adj.resolvedEffect()
	if !ok {
		return 0, false
	}
	if effect == EffectDecrease {
		return -adj.Amount, true
	}
	return adj.Amount, true
}
