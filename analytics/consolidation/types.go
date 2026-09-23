// Package consolidation combines multiple entities' canonical
// financial.FinancialDatasets into a single consolidated
// financial.FinancialDataset, using only caller-supplied
// ownership/currency/elimination information.
//
// Unlike most analytics/ siblings, this package's primary output is itself
// a financial.FinancialDataset — the same shape Calculate takes as input,
// per entity — rather than a bespoke analysis Result. It is intentionally
// the one analytics package that produces a dataset rather than only
// reading one, so a caller can feed Consolidate's output straight into any
// other package in this repository (financial/metrics, analytics/qoe,
// valuation/orchestrator, ...) exactly as it would a single entity's
// dataset.
//
// Three things this package deliberately never does, each directly from
// the task brief:
//
//   - It never fetches FX rates. Currency.go converts only when
//     Input.CurrencyRates supplies a matching (FromCurrency, ToCurrency,
//     Period) rate; a currency mismatch with no matching rate is reported
//     as an IssueMissingCurrencyRate warning and that entity/period's items
//     are excluded from the consolidated total rather than silently
//     assumed to be 1:1.
//   - It never infers intercompany eliminations. Input.Eliminations is the
//     complete, explicit list of what to remove; Calculate performs no
//     analysis to detect an intercompany relationship on its own (e.g. it
//     never assumes two entities' matching revenue/expense codes in the
//     same period are intercompany trade — that judgment belongs to the
//     caller, who has visibility into its own corporate structure that
//     this package cannot infer from amounts alone).
//   - It never guesses which periods are "common." Input.Periods is the
//     caller-selected, explicit set of reporting periods to consolidate;
//     an entity dataset with data outside Input.Periods simply does not
//     contribute those periods, and a period present in Input.Periods but
//     missing from an entity is reported (IssuePeriodMissingForEntity) as
//     an advisory reconciliation issue, never treated as a zero
//     contribution.
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state. Calculate can be called concurrently
// and repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package consolidation

import "github.com/themurtez/go-valuate/financial"

// FormulaVersion identifies this package's fixed formula set: the
// full-consolidation summation rule, the ownership-weighting formula, the
// currency-conversion rule, the elimination-application rule, and every
// reconciliation-issue trigger. Bump this whenever any of that changes in
// a way that could make a historical Result not reproduce identically
// under new code — see the repository README's versioning-strategy
// section, which this constant follows exactly (financial.TaxonomyVersion,
// concentration.FormulaVersion, debt.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// Mode selects how EntityDataset amounts are combined into the
// consolidated dataset.
type Mode string

const (
	// ModeFullConsolidation sums each selected entity's full amount for
	// every (Code, Period), independent of OwnershipPercent — the standard
	// "consolidate 100% of every controlled entity" accounting convention
	// (majority/controlling-interest consolidation). This is the default:
	// the zero Mode resolves to ModeFullConsolidation (see resolveMode).
	ModeFullConsolidation Mode = "full_consolidation"
	// ModeOwnershipWeighted multiplies each selected entity's amount for
	// every (Code, Period) by that entity's OwnershipPercent before
	// summing — a proportional/equity-method-style view. Must be
	// explicitly selected (per the task brief: "simple ownership-weighted
	// mode if explicitly selected"); Calculate returns
	// IssueMissingOwnershipPercent for any selected entity with a nil
	// OwnershipPercent under this Mode, and that entity contributes
	// nothing rather than being silently treated as 100% or 0% owned.
	ModeOwnershipWeighted Mode = "ownership_weighted"
)

// EntityDataset is one entity's canonical financial data plus the
// consolidation-specific metadata Calculate needs about it.
type EntityDataset struct {
	// EntityID is a caller-assigned identifier unique within Input.Entities
	// (e.g. an internal legal-entity code). Required; used as the join key
	// for Policy.SelectedEntityIDs, Input.Eliminations, and
	// Input.CurrencyRates, and as EntityContribution.EntityID/
	// financial.SourceRef.RowID's provenance link back to this entity in
	// Result.Consolidated.
	EntityID string `json:"entity_id"`
	// EntityLabel is a human-readable display name for EntityID, carried
	// through to EntityContribution.EntityLabel purely for display/audit —
	// never used as a lookup key (EntityID is).
	EntityLabel string `json:"entity_label,omitempty"`
	// Dataset is this entity's own normalized financial.FinancialDataset —
	// typically the output of financial.Normalize for that entity's own
	// source data. Calculate never mutates it.
	Dataset financial.FinancialDataset `json:"dataset"`
	// OwnershipPercent is the caller-supplied ownership stake in this
	// entity, as a decimal (0.8 means 80%). Nil means "not supplied."
	// Required only under ModeOwnershipWeighted (see Mode's doc comment);
	// ignored entirely under ModeFullConsolidation, where every selected
	// entity always contributes its full amount regardless of this field —
	// mirroring workingcapital.Result's "supplying an unused field never
	// changes behavior silently" discipline: a caller cannot accidentally
	// under-consolidate a majority-owned subsidiary by forgetting Mode.
	OwnershipPercent *float64 `json:"ownership_percent,omitempty"`
}

// CurrencyRate is a caller-supplied exchange rate for converting amounts
// denominated in FromCurrency into ToCurrency, for one specific Period.
// This package never fetches or infers a rate; a rate is used only when
// FromCurrency, ToCurrency, and Period all match an entity's Dataset.Currency
// and the period being consolidated.
type CurrencyRate struct {
	// FromCurrency is the ISO 4217 code an entity's Dataset.Currency is
	// denominated in.
	FromCurrency string `json:"from_currency"`
	// ToCurrency is the ISO 4217 code to convert into — must equal
	// Policy.TargetCurrency for this rate to ever be selected (see
	// resolveRate).
	ToCurrency string `json:"to_currency"`
	// Period is the specific reporting period this rate applies to. Rates
	// are never assumed constant across periods — a caller with a single
	// blended annual rate supplies the same Rate value once per Period it
	// covers.
	Period financial.Period `json:"period"`
	// Rate is the multiplier: amount in ToCurrency = amount in FromCurrency
	// * Rate. Must be finite and > 0; a non-positive or non-finite Rate is
	// rejected (IssueInvalidCurrencyRate) and treated as if absent.
	Rate float64 `json:"rate"`
}

// Elimination is one caller-declared intercompany entry to remove from one
// entity's contribution before it is summed into the consolidated dataset
// — e.g. intercompany revenue one subsidiary recognized from a sale to
// another. Calculate performs no analysis to detect these on its own (see
// the package doc comment); Input.Eliminations is the complete, explicit
// list.
type Elimination struct {
	// EntityID identifies which EntityDataset this elimination applies to.
	// Must match an EntityID in Input.Entities; an Elimination naming an
	// unknown EntityID produces IssueUnknownEliminationEntity and is
	// skipped.
	EntityID string `json:"entity_id"`
	// Code is the canonical taxonomy code this elimination applies to.
	Code financial.Code `json:"code"`
	// Period is the specific period this elimination applies to.
	Period financial.Period `json:"period"`
	// Amount is the amount to remove from EntityID's contribution at
	// (Code, Period), in EntityID's native Dataset.Currency (i.e. removed
	// before any currency conversion, mirroring the order a real
	// consolidation eliminates intercompany balances before translating
	// the net figure). A positive Amount reduces the contribution by that
	// much; Calculate does not require |Amount| to be <= the entity's raw
	// amount at that (Code, Period) — an elimination can drive a
	// contribution negative, which is preserved rather than clamped, since
	// clamping would silently understate the elimination.
	Amount float64 `json:"amount"`
	// Description documents why this elimination applies, for audit
	// purposes only (e.g. "Q2 intercompany management fee, Sub A -> Sub
	// B"). Never parsed.
	Description string `json:"description,omitempty"`
}

// Policy configures Calculate's caller-adjustable, non-formula behavior:
// which entities participate and what currency the consolidated dataset
// is denominated in.
type Policy struct {
	// SelectedEntityIDs narrows consolidation to just these EntityIDs. If
	// empty, every EntityDataset in Input.Entities is included — mirroring
	// every analytics sibling package's zero-value-means-"use everything
	// supplied" convention (e.g. concentration.Policy's TopN defaulting,
	// applied here to entity selection instead of a numeric cutoff). An ID
	// listed here that does not match any Input.Entities[i].EntityID
	// produces IssueUnknownSelectedEntity and is otherwise ignored.
	SelectedEntityIDs []string `json:"selected_entity_ids,omitempty"`
	// TargetCurrency is the ISO 4217 code Result.Consolidated.Currency is
	// denominated in. Required whenever any selected entity's
	// Dataset.Currency differs from it (Calculate returns Available ==
	// false with IssueMissingTargetCurrency if so and this is empty); if
	// every selected entity already shares one currency and TargetCurrency
	// is left empty, that shared currency is used and no conversion is
	// attempted.
	TargetCurrency string `json:"target_currency,omitempty"`
}

// resolvePolicy returns p with SelectedEntityIDs defaulted to every
// EntityID in entities when p.SelectedEntityIDs is empty — the same
// zero-value-means-defaults rule every analytics sibling package's
// resolvePolicy/resolveInclusionPolicy uses.
func resolvePolicy(p Policy, entities []EntityDataset) Policy {
	if len(p.SelectedEntityIDs) == 0 {
		ids := make([]string, 0, len(entities))
		for _, e := range entities {
			ids = append(ids, e.EntityID)
		}
		p.SelectedEntityIDs = ids
	}
	return p
}

// resolveMode returns m if non-empty, otherwise ModeFullConsolidation —
// see Mode's doc comment.
func resolveMode(m Mode) Mode {
	if m == "" {
		return ModeFullConsolidation
	}
	return m
}

// Input bundles everything Calculate needs.
type Input struct {
	// Entities is the full set of entity datasets available to consolidate.
	// Required; Calculate returns Available == false if empty. Every
	// EntityID must be unique across this slice (IssueDuplicateEntityID
	// otherwise). Policy.SelectedEntityIDs (or all of Entities, if that is
	// empty) determines which of these actually participate in
	// Result.Consolidated.
	Entities []EntityDataset `json:"entities"`
	// Mode selects full vs. ownership-weighted consolidation. If empty,
	// ModeFullConsolidation is used.
	Mode Mode `json:"mode,omitempty"`
	// Periods is the caller-selected, explicit set of common reporting
	// periods to consolidate. Required; Calculate returns Available ==
	// false if empty — this package never infers "the periods every entity
	// happens to share" from the entities' own data, since two entities'
	// same-looking financial.Period string (e.g. "2025") is a caller
	// convention this package cannot verify actually denotes the same
	// fiscal period without being told so explicitly.
	Periods []financial.Period `json:"periods"`
	// CurrencyRates supplies every exchange rate Calculate is allowed to
	// use for currency conversion. This package never fetches or infers a
	// rate — see the package doc comment. May be empty if every selected
	// entity already shares Policy.TargetCurrency (or shares one currency
	// with TargetCurrency left empty).
	CurrencyRates []CurrencyRate `json:"currency_rates,omitempty"`
	// Eliminations is the complete, explicit list of intercompany entries
	// to remove before summing. This package never infers eliminations —
	// see the package doc comment. May be empty.
	Eliminations []Elimination `json:"eliminations,omitempty"`
	// Policy configures entity selection and target currency. If the zero
	// value, every entity in Entities is selected and the target currency
	// is resolved from the selected entities' own currencies (see
	// Policy.TargetCurrency's doc comment).
	Policy Policy `json:"policy"`
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model every
// analytics sibling package uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input or
// reconciliation problem. This package defines its own separate taxonomy,
// consistent with every other package in this repository, rather than
// reusing one of theirs — a consolidation input/reconciliation problem is
// a distinct problem domain.
type IssueCode string

const (
	// IssueNoEntities means Input.Entities was empty; Calculate returns
	// Available == false.
	IssueNoEntities IssueCode = "NO_ENTITIES"
	// IssueNoPeriods means Input.Periods was empty; Calculate returns
	// Available == false, since "common reporting periods" is a required,
	// explicit caller decision this package never infers.
	IssueNoPeriods IssueCode = "NO_PERIODS"
	// IssueDuplicateEntityID means two or more entries in Input.Entities
	// share the same EntityID; Calculate returns Available == false, since
	// every downstream join (Policy.SelectedEntityIDs, Eliminations,
	// provenance) depends on EntityID being unique.
	IssueDuplicateEntityID IssueCode = "DUPLICATE_ENTITY_ID"
	// IssueUnknownSelectedEntity means Policy.SelectedEntityIDs named an ID
	// with no matching Input.Entities entry. Advisory only; that ID is
	// otherwise ignored.
	IssueUnknownSelectedEntity IssueCode = "UNKNOWN_SELECTED_ENTITY"
	// IssueMissingOwnershipPercent means Mode is ModeOwnershipWeighted and
	// a selected entity's OwnershipPercent is nil. That entity contributes
	// nothing to Result.Consolidated (see Mode's doc comment).
	IssueMissingOwnershipPercent IssueCode = "MISSING_OWNERSHIP_PERCENT"
	// IssueInvalidOwnershipPercent means a selected entity's
	// OwnershipPercent is non-finite or outside [0, 1]. That entity
	// contributes nothing to Result.Consolidated, mirroring
	// IssueMissingOwnershipPercent's treatment — an out-of-range weight is
	// exactly as unusable as a missing one, never clamped.
	IssueInvalidOwnershipPercent IssueCode = "INVALID_OWNERSHIP_PERCENT"
	// IssueMissingTargetCurrency means selected entities use more than one
	// Dataset.Currency and Policy.TargetCurrency was left empty. Calculate
	// returns Available == false, since this package refuses to guess
	// which of several currencies a multi-currency consolidation should
	// resolve to.
	IssueMissingTargetCurrency IssueCode = "MISSING_TARGET_CURRENCY"
	// IssueMissingCurrencyRate means a selected entity's Dataset.Currency
	// differs from the resolved target currency and no Input.CurrencyRates
	// entry matches (FromCurrency, ToCurrency, Period) for at least one
	// period the entity contributes to. That entity's items for the
	// affected period(s) are excluded from Result.Consolidated rather than
	// summed unconverted — see the package doc comment.
	IssueMissingCurrencyRate IssueCode = "MISSING_CURRENCY_RATE"
	// IssueInvalidCurrencyRate means an Input.CurrencyRates entry's Rate is
	// non-finite or <= 0; that entry is ignored exactly as if it were
	// absent (see IssueMissingCurrencyRate).
	IssueInvalidCurrencyRate IssueCode = "INVALID_CURRENCY_RATE"
	// IssueUnknownEliminationEntity means an Input.Eliminations entry named
	// an EntityID absent from Input.Entities. That entry is skipped.
	IssueUnknownEliminationEntity IssueCode = "UNKNOWN_ELIMINATION_ENTITY"
	// IssueEliminationEntityNotSelected means an Input.Eliminations entry
	// named a real EntityID that Policy.SelectedEntityIDs did not select.
	// Advisory only: an elimination against an unselected (excluded)
	// entity has nothing to remove anything from, so it is skipped, but
	// this is worth surfacing since it often indicates a stale
	// elimination list after narrowing Policy.SelectedEntityIDs.
	IssueEliminationEntityNotSelected IssueCode = "ELIMINATION_ENTITY_NOT_SELECTED"
	// IssueEliminationTargetNotFound means an Input.Eliminations entry's
	// (Code, Period) has no matching item at all in that entity's own
	// Dataset.Items. Advisory only: the elimination is still recorded in
	// Result.EliminationsApplied (it does not name an unknown entity), but
	// it had nothing to subtract from, which usually means a typo in Code
	// or Period, or an elimination left over from a since-changed source
	// dataset.
	IssueEliminationTargetNotFound IssueCode = "ELIMINATION_TARGET_NOT_FOUND"
	// IssueEliminationPeriodOutOfScope means an Input.Eliminations entry's
	// Period is not among Input.Periods. Advisory only: the elimination is
	// still recorded in Result.EliminationsApplied (it names a known,
	// selected entity), but buildEntityContributions only ever consults
	// elimByEntity for items whose period is in scope (see
	// validatePeriodCoverage's identical period-filtering rule), so an
	// out-of-scope elimination can never actually be subtracted from
	// anything — this issue exists so that fact is never silent.
	IssueEliminationPeriodOutOfScope IssueCode = "ELIMINATION_PERIOD_OUT_OF_SCOPE"
	// IssuePeriodMissingForEntity means a period in Input.Periods has no
	// data at all in a selected entity's Dataset. Advisory only — that
	// entity simply contributes nothing for that period (distinct from
	// contributing a zero), mirroring every analytics sibling package's
	// "missing is not zero" availability discipline applied here at the
	// entity/period level.
	IssuePeriodMissingForEntity IssueCode = "PERIOD_MISSING_FOR_ENTITY"
	// IssueEntityCurrencyEmpty means a selected entity's Dataset.Currency
	// is the empty string, which financial.FinancialDataset's own contract
	// treats as never valid. That entity is excluded entirely from
	// Result.Consolidated.
	IssueEntityCurrencyEmpty IssueCode = "ENTITY_CURRENCY_EMPTY"
)

// Issue is a single Calculate-time input or reconciliation finding,
// mirroring every analytics sibling package's identical Issue shape.
type Issue struct {
	Code     IssueCode        `json:"code"`
	Severity IssueSeverity    `json:"severity"`
	EntityID string           `json:"entity_id,omitempty"`
	Period   financial.Period `json:"period,omitempty"`
	Message  string           `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from adjustments.HasErrors/qoe.HasErrors/
// concentration.HasErrors/... rather than shared — see
// adjustments.HasErrors's doc comment for the full rationale (each
// package's Issue is a distinct Go type with no common interface worth
// introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// EntityCodeContribution is one entity's contribution to a single
// (Code, Period) cell of the consolidated dataset, after eliminations and
// currency conversion but before ownership weighting is applied for
// display purposes — RawAmount and ConvertedAmount are always the
// eliminated-but-unweighted figures, while WeightedAmount additionally
// reflects OwnershipPercent under ModeOwnershipWeighted so a caller can
// see both the entity's true reported figure and what it contributed to
// the consolidated total.
type EntityCodeContribution struct {
	Code   financial.Code   `json:"code"`
	Period financial.Period `json:"period"`
	// RawAmount is this entity's amount at (Code, Period) from its own
	// Dataset, after eliminations are applied, in the entity's native
	// currency.
	RawAmount float64 `json:"raw_amount"`
	// ConvertedAmount is RawAmount converted to the consolidated dataset's
	// currency. Equal to RawAmount when no conversion was needed.
	ConvertedAmount float64 `json:"converted_amount"`
	// WeightedAmount is ConvertedAmount, multiplied by OwnershipPercent
	// under ModeOwnershipWeighted (equal to ConvertedAmount under
	// ModeFullConsolidation). This is the figure actually summed into
	// Result.Consolidated for this entity at this cell.
	WeightedAmount float64 `json:"weighted_amount"`
}

// EntityContribution summarizes one selected entity's full participation
// in Result.Consolidated: its identity, resolved weighting, and every
// (Code, Period) cell it contributed to.
type EntityContribution struct {
	EntityID    string `json:"entity_id"`
	EntityLabel string `json:"entity_label,omitempty"`
	// Currency is the entity's own native Dataset.Currency, echoed for
	// audit purposes.
	Currency string `json:"currency"`
	// Mode echoes the Result-level Mode this contribution was computed
	// under (every entity in one Result shares the same Mode).
	Mode Mode `json:"mode"`
	// OwnershipPercent echoes EntityDataset.OwnershipPercent as supplied.
	// Nil under ModeFullConsolidation unless the caller happened to supply
	// one anyway (harmless — see Mode's doc comment).
	OwnershipPercent *float64 `json:"ownership_percent,omitempty"`
	// Items is one EntityCodeContribution per (Code, Period) this entity
	// contributed to the consolidated total — sorted by Code, then Period.
	// A cell excluded by a missing currency rate, an invalid ownership
	// percent, or falling outside Input.Periods is simply absent here,
	// never present with a zero WeightedAmount.
	Items []EntityCodeContribution `json:"items,omitempty"`
	// TotalWeightedContribution is the sum of every Items[i].WeightedAmount
	// — a single summary figure for this entity's overall pull on the
	// consolidated total, across every code and period.
	TotalWeightedContribution float64 `json:"total_weighted_contribution"`
}

// CurrencyConversion records one (EntityID, FromCurrency, ToCurrency,
// Period) conversion actually applied — one entry per distinct
// combination, not one per line item, since a single CurrencyRate applies
// uniformly to every code an entity reports in that period.
type CurrencyConversion struct {
	EntityID     string           `json:"entity_id"`
	FromCurrency string           `json:"from_currency"`
	ToCurrency   string           `json:"to_currency"`
	Period       financial.Period `json:"period"`
	Rate         float64          `json:"rate"`
}

// Result is the output of Calculate: the consolidated dataset itself,
// full per-entity provenance, applied eliminations/conversions, and
// reconciliation issues.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoEntities, IssueNoPeriods, IssueDuplicateEntityID,
	// IssueMissingTargetCurrency) — every other field is then zero-value.
	Available bool `json:"available"`
	// Mode echoes the resolved Mode (after ModeFullConsolidation
	// substitution) this Result was computed under.
	Mode Mode `json:"mode"`
	// Policy echoes the resolved Input.Policy (after SelectedEntityIDs
	// defaulting) this Result was computed under.
	Policy Policy `json:"policy"`

	// Consolidated is the combined financial.FinancialDataset: one
	// NormalizedItem per (Code, Period) present in Input.Periods across
	// every selected entity, summed per EntityCodeContribution.
	// WeightedAmount, with Sources carrying one financial.SourceRef per
	// contributing entity (SourceRef.RowID is the EntityID, SourceRef.Label
	// is the EntityLabel, SourceRef.Period is the item's period, and
	// SourceRef.Amount is that entity's WeightedAmount contribution) — the
	// provenance link back to source entities the task brief requires,
	// expressed through financial.FinancialDataset's own existing
	// provenance mechanism rather than a new one.
	Consolidated financial.FinancialDataset `json:"consolidated"`

	// EntityContributions is one EntityContribution per selected entity
	// that contributed at least one item, sorted by EntityID ascending.
	EntityContributions []EntityContribution `json:"entity_contributions,omitempty"`

	// EliminationsApplied is every Input.Eliminations entry that was
	// actually applied (a known, selected EntityID), sorted by EntityID,
	// then Code, then Period. An Elimination naming an unknown or
	// unselected entity is not repeated here — see
	// IssueUnknownEliminationEntity/IssueEliminationEntityNotSelected.
	EliminationsApplied []Elimination `json:"eliminations_applied,omitempty"`

	// CurrencyConversions is every distinct (EntityID, FromCurrency,
	// ToCurrency, Period) conversion actually applied, sorted by EntityID,
	// then Period.
	CurrencyConversions []CurrencyConversion `json:"currency_conversions,omitempty"`

	// ReconciliationIssues carries every Issue raised while validating
	// period alignment, currency consistency, ownership weighting, and
	// elimination applicability — the full set regardless of Severity
	// (Warnings/Errors below are the same Issues split by Severity, for a
	// caller that wants only one or the other).
	ReconciliationIssues []Issue `json:"reconciliation_issues,omitempty"`
	// Warnings carries every ReconciliationIssues entry with
	// SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every ReconciliationIssues entry with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
