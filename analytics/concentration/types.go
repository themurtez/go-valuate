// Package concentration analyzes concentration risk — how dependent a
// business is on its largest customers, suppliers, or any other
// counterparty relationship — from caller-supplied revenue/spend
// observations.
//
// This package is deliberately independent of financial.FinancialDataset
// and the financial.Code taxonomy. Unlike analytics/revenuequality (which
// reads customer revenue as an optional enrichment of a normalized
// financial dataset), concentration analysis is frequently performed on
// data a financial dataset never carries at all: per-customer invoicing
// exports, per-vendor accounts-payable detail, or per-referral-source
// revenue. So this package defines its own minimal, fully portable
// Observation{EntityKey, Period, Amount, Category} tuple and never requires
// a financial.FinancialDataset, a financial.Code, or any other
// dataset-shaped input. A caller who also has a financial.FinancialDataset
// is free to reconcile this package's totals against it externally; this
// package has no opinion on that reconciliation.
//
// Basis labels the observations' meaning (customer revenue, supplier
// spend, or an caller-defined "other" concentration basis) purely for
// display/audit purposes — every metric in this package (shares, HHI,
// ranking, scenarios) is computed identically regardless of Basis, since
// "concentration among counterparties by dollar amount" is the same
// arithmetic whether the counterparties are customers, suppliers, or
// referral sources.
//
// Privacy: EntityKey is an opaque, caller-assigned string. This package
// never requires, stores, or infers any real name, email, address, or other
// PII — the same convention analytics/revenuequality.CustomerPeriodRevenue
// established for customer-level detail.
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state. Calculate can be called concurrently and
// repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package concentration

import "github.com/themurtez/go-valuate/financial"

// FormulaVersion identifies this package's fixed formula set: the
// top-N-share/HHI formulas, the ranking/dependency-change methodology, the
// lost-entity and top-N-loss scenario formulas (including the optional
// earnings-impact conversion), and every flag-trigger rule in Thresholds.
// Bump this whenever any of that changes in a way that could make a
// historical Result not reproduce identically under new code — see the
// repository README's versioning-strategy section, which this constant
// follows exactly (financial.TaxonomyVersion, metrics.FormulaVersion,
// workingcapital.FormulaVersion, cashflow.FormulaVersion,
// revenuequality.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// PeriodType mirrors metrics.PeriodType/workingcapital.PeriodType/
// revenuequality.PeriodType (fiscal year, YTD, quarter, month), duplicated
// here rather than aliased so this package's own doc comments apply
// directly at the call site — the same choice every analytics sibling
// package already made for its own PeriodType.
type PeriodType string

const (
	PeriodTypeFiscalYear PeriodType = "fiscal_year"
	PeriodTypeYTD        PeriodType = "ytd"
	PeriodTypeQuarter    PeriodType = "quarter"
	PeriodTypeMonth      PeriodType = "month"
)

// PeriodInfo is caller-supplied metadata about a single financial.Period,
// used to determine chronological order for Trend, DependencyChanges, and
// scenario selection ("most recent period"). financial.Period is
// intentionally just a string with no guaranteed sort order; this package
// requires PeriodInfo rather than inferring order or granularity from the
// string, mirroring every analytics sibling package's identical
// no-guessing rule.
type PeriodInfo struct {
	// Type is this period's granularity.
	Type PeriodType `json:"type"`
	// FiscalYear is the fiscal year this period falls within. Required for
	// chronological ordering.
	FiscalYear int `json:"fiscal_year"`
	// SequenceInYear orders periods that share the same FiscalYear and Type
	// (e.g. Q1=1..Q4=4, or month 1-12). Ignored for PeriodTypeFiscalYear and
	// PeriodTypeYTD.
	SequenceInYear int `json:"sequence_in_year,omitempty"`
}

// ConcentrationValue represents a single concentration figure that may or
// may not be calculable, mirroring metrics.MetricValue/
// revenuequality.RevenueValue's identical availability convention:
// Available distinguishes "computed to be exactly $0/0%" from "cannot be
// computed because a required input is absent."
type ConcentrationValue struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

// Unavailable is the canonical zero-information ConcentrationValue.
func Unavailable() ConcentrationValue { return ConcentrationValue{} }

// AvailableValue reports a ConcentrationValue for a successfully computed
// figure.
func AvailableValue(v float64) ConcentrationValue {
	return ConcentrationValue{Available: true, Value: v}
}

// Basis labels what an Observation's Amount represents, for display/audit
// purposes only — see the package doc comment for why every metric in this
// package is computed identically regardless of Basis.
type Basis string

const (
	BasisCustomerRevenue Basis = "customer_revenue"
	BasisSupplierSpend   Basis = "supplier_spend"
	BasisOther           Basis = "other"
)

// Observation is one entity's amount in one period — the portable input
// tuple this package requires for every computation (see the package doc
// comment). A caller assembles a slice of these from whatever source system
// it has (a billing export, an AP ledger, a CRM); this package has no
// opinion on where it came from.
type Observation struct {
	// EntityKey is an opaque, caller-assigned identifier for the customer,
	// supplier, or other counterparty. Any stable string the caller chooses
	// (an internal ID, a hash) — this package never requires it to be a real
	// name or other PII, and treats it purely as an equality-comparable key.
	EntityKey string `json:"entity_key"`
	// Period is the reporting period this amount was observed in. Must be a
	// period present in Input.PeriodMeta for this row to be included in any
	// ordering-dependent output (Trend, DependencyChanges, scenario
	// selection of "the most recent period"); a row for a period absent from
	// PeriodMeta is still included in that period's own PeriodConcentration
	// (computed in Input's own encounter order when chronology is
	// unavailable — see Calculate) but is excluded from every
	// cross-period output. See IssuePeriodMissingFromMeta.
	Period financial.Period `json:"period"`
	// Amount is this entity's revenue or spend for Period, in the caller's
	// native currency. Must be >= 0; a negative or non-finite Amount makes
	// its row excluded from every computation and recorded as
	// IssueInvalidObservation — see validateObservations.
	Amount float64 `json:"amount"`
	// Category, when non-empty, is a caller-assigned grouping label (e.g. a
	// product line, industry vertical, or geography) used only for
	// PeriodConcentration's optional by-category breakdown. Plain string
	// (mirroring financial.Code/revenuequality.CustomerPeriodRevenue.Segment)
	// so this package never hard-codes a category taxonomy.
	Category string `json:"category,omitempty"`
}

// EntityImpactAssumption supplies a caller-known contribution-margin (or
// other earnings-impact) rate for one specific EntityKey, overriding
// Policy.DefaultImpactMarginRate for that entity alone in every Scenario
// impact calculation. A caller with per-customer margin data (e.g. a
// low-margin reseller account vs. a high-margin direct account) supplies
// this; a caller with only a single blended assumption for the whole book
// leaves it empty and relies on Policy.DefaultImpactMarginRate alone.
type EntityImpactAssumption struct {
	EntityKey  string  `json:"entity_key"`
	MarginRate float64 `json:"margin_rate"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// Basis labels what Observations' Amount represents. Required only for
	// display (echoed on Result.Basis); if empty, BasisOther is assumed.
	Basis Basis `json:"basis"`
	// Observations is the full set of entity/period/amount rows to analyze.
	// Required; Calculate returns a Result with Available == false and an
	// Errors entry if empty.
	Observations []Observation `json:"observations"`
	// PeriodMeta supplies chronological ordering for Observations' periods.
	// Required for Trend, DependencyChanges, and every Scenario (which is
	// always computed for the chronologically most recent period); if nil,
	// or a period present in Observations has no entry, those specific
	// outputs are left unavailable rather than the whole Result failing —
	// mirroring analytics/workingcapital.Input.PeriodMeta's identical
	// convention. Per-period PeriodConcentration entries are still computed
	// for every period, ordered by first appearance in Observations, when
	// PeriodMeta is incomplete.
	PeriodMeta map[financial.Period]PeriodInfo

	// Policy configures top-N cutoffs and the optional earnings-impact
	// assumptions used by Scenarios. If the zero value, DefaultPolicy() is
	// used.
	Policy Policy
}

// Policy configures Calculate's optional, caller-adjustable behavior that
// is not a fixed part of FormulaVersion (unlike Thresholds, which governs
// flag triggers) — mirroring revenuequality.Policy/cashflow.Options.Thresholds's
// identical separation of "what is reported" from "what triggers a flag."
type Policy struct {
	// TopN lists each entity-count cutoff PeriodConcentration.TopNShares
	// reports a share for (e.g. []int{1, 3, 5, 10} for "top 1 entity," "top 3
	// entities," etc. — see the prompt's requested 1/3/5/10 cutoffs). If
	// empty, DefaultPolicy's []int{1, 3, 5, 10} is used.
	TopN []int `json:"top_n"`
	// ScenarioTopN lists each entity-count cutoff Scenarios' top-N-loss
	// scenarios are computed for (e.g. []int{3, 5} for "lose the top 3
	// entities," "lose the top 5 entities"). If empty, DefaultPolicy's
	// []int{3, 5} is used. Kept separate from TopN since a caller may want a
	// finer set of reporting cutoffs than loss scenarios.
	ScenarioTopN []int `json:"scenario_top_n"`
	// DefaultImpactMarginRate, when non-nil, is the contribution-margin (or
	// other earnings-impact) rate applied to every entity's lost revenue in
	// every Scenario's earnings-impact calculation, except entities covered
	// by EntityImpactAssumptions. A decimal (0.35 means 35%). If nil, no
	// default applies and only entities covered by EntityImpactAssumptions
	// get an earnings-impact figure; entities covered by neither leave
	// Scenario.EarningsImpact/EntityImpact[i].EarningsImpact Unavailable —
	// this package never invents a margin assumption a caller did not
	// supply.
	DefaultImpactMarginRate *float64 `json:"default_impact_margin_rate,omitempty"`
	// EntityImpactAssumptions supplies per-entity margin-rate overrides —
	// see EntityImpactAssumption.
	EntityImpactAssumptions []EntityImpactAssumption `json:"entity_impact_assumptions,omitempty"`
}

// DefaultPolicy returns this package's baseline concentration cutoffs: top
// 1/3/5/10 entities for reporting (the cutoffs the prompt specifies) and
// top 3/5 for loss scenarios, with no earnings-impact assumption (a caller
// must opt in explicitly).
func DefaultPolicy() Policy {
	return Policy{
		TopN:         []int{1, 3, 5, 10},
		ScenarioTopN: []int{3, 5},
	}
}

// resolvePolicy returns p with any empty cutoff list replaced by
// DefaultPolicy's corresponding list — the same zero-value-means-defaults
// rule workingcapital.resolveInclusionPolicy/revenuequality.resolvePolicy
// use, applied field-by-field here since DefaultImpactMarginRate/
// EntityImpactAssumptions are meaningful even when TopN/ScenarioTopN need
// defaulting.
func resolvePolicy(p Policy) Policy {
	d := DefaultPolicy()
	if len(p.TopN) == 0 {
		p.TopN = d.TopN
	}
	if len(p.ScenarioTopN) == 0 {
		p.ScenarioTopN = d.ScenarioTopN
	}
	return p
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

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository, rather than reusing one of
// theirs — a concentration-analysis input problem is a distinct problem
// domain.
type IssueCode string

const (
	// IssueNoObservations means Input.Observations was empty; Calculate
	// returns Available == false.
	IssueNoObservations IssueCode = "NO_OBSERVATIONS"
	// IssueInvalidObservation means at least one Observation had a negative
	// or non-finite Amount; that row is excluded from every computation but
	// does not error the whole calculation.
	IssueInvalidObservation IssueCode = "INVALID_OBSERVATION"
	// IssueNoPeriodMeta means Input.PeriodMeta was nil or empty, so
	// chronological ordering, Trend, DependencyChanges, and Scenarios could
	// not be computed. Advisory only: per-period PeriodConcentration figures
	// are still fully computed, ordered by first appearance in Observations.
	IssueNoPeriodMeta IssueCode = "NO_PERIOD_META"
	// IssuePeriodMissingFromMeta means at least one period present in
	// Observations has no PeriodMeta entry, so every ordering-dependent
	// output is unavailable — mirroring
	// workingcapital.IssuePeriodMissingFromMeta's identical rule.
	IssuePeriodMissingFromMeta IssueCode = "PERIOD_MISSING_FROM_META"
	// IssueNoImpactAssumption means at least one entity removed by a
	// Scenario lacks both a matching EntityImpactAssumption and
	// Policy.DefaultImpactMarginRate, so that Scenario's EarningsImpact
	// fields are left Unavailable. Advisory only, and scoped only to
	// entities Scenarios actually remove — a caller who supplied margin
	// data only for its largest entities does not get this warning if no
	// configured Scenario reaches beyond them.
	IssueNoImpactAssumption IssueCode = "NO_IMPACT_ASSUMPTION"
)

// Issue is a single Calculate-time input finding, mirroring every analytics
// sibling package's identical Issue shape.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from adjustments.HasErrors/qoe.HasErrors/
// workingcapital.HasErrors/cashflow.HasErrors/revenuequality.HasErrors
// rather than shared — see adjustments.HasErrors's doc comment for the full
// rationale (each package's Issue is a distinct Go type with no common
// interface worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// RankedEntity is one entity's rank, amount, and share within a single
// period, sorted descending by Amount (ties broken by EntityKey ascending
// for deterministic output — see rankEntities).
type RankedEntity struct {
	// Rank is this entity's 1-based position by Amount, descending.
	Rank int `json:"rank"`
	// EntityKey is the opaque entity identifier (see Observation.EntityKey).
	EntityKey string `json:"entity_key"`
	// Amount is this entity's summed Amount for the period.
	Amount float64 `json:"amount"`
	// Share is Amount / period total. Available only if the period total is
	// available and nonzero.
	Share ConcentrationValue `json:"share"`
}

// TopNShare is the share of a period's total amount contributed by its top
// N entities by amount.
type TopNShare struct {
	// N is the entity-count cutoff (see Policy.TopN).
	N int `json:"n"`
	// Amount is the summed amount of the top N entities (or every entity, if
	// fewer than N were present).
	Amount ConcentrationValue `json:"amount"`
	// Share is Amount.Value / period total. Available only if both are
	// available and the total is nonzero.
	Share ConcentrationValue `json:"share"`
}

// CategoryShare is one Category's share of a period's total amount,
// populated only for rows in Input.Observations that supplied a non-empty
// Category.
type CategoryShare struct {
	Category string             `json:"category"`
	Amount   ConcentrationValue `json:"amount"`
	// Share is Amount.Value / period total (across every entity, not just
	// categorized ones). Available only if both are available and the total
	// is nonzero.
	Share ConcentrationValue `json:"share"`
}

// PeriodConcentration is one period's full concentration picture: entity
// count, total amount, ranked entities, top-N shares, HHI, and (when
// supplied) category breakdown.
type PeriodConcentration struct {
	// Period is the period this snapshot applies to.
	Period financial.Period `json:"period"`
	// EntityCount is the number of distinct entities with a valid
	// Observation in Period.
	EntityCount int `json:"entity_count"`
	// TotalAmount is the sum of every valid Observation.Amount in Period.
	TotalAmount ConcentrationValue `json:"total_amount"`
	// RankedEntities is every entity active in Period, ranked descending by
	// Amount — see RankedEntity.
	RankedEntities []RankedEntity `json:"ranked_entities,omitempty"`
	// LargestEntityShare is RankedEntities[0].Share, duplicated here as its
	// own named field since "largest single entity's share" is the single
	// most commonly cited concentration figure in diligence — mirrors
	// TopNShares' N=1 entry exactly when Policy.TopN includes 1, but
	// available even if it does not.
	LargestEntityShare ConcentrationValue `json:"largest_entity_share"`
	// TopNShares is one TopNShare per Policy.TopN cutoff, sorted ascending by
	// N.
	TopNShares []TopNShare `json:"top_n_shares,omitempty"`
	// HHI is the Herfindahl-Hirschman Index of entity shares for Period: the
	// sum of each entity's (share)^2, expressed on the conventional
	// 0-10,000 scale (a share of 1.0 contributes 10,000; a share of 0.1
	// contributes 100). Higher means more concentrated among fewer entities.
	// Available only when EntityCount > 0 and TotalAmount is available and
	// nonzero.
	HHI ConcentrationValue `json:"hhi"`
	// Categories is one CategoryShare per distinct non-empty Category value
	// present in Period's observations, sorted by Category ascending. Empty
	// if no row for Period supplied a Category.
	Categories []CategoryShare `json:"categories,omitempty"`
}

// TrendDirection is a coarse, deterministic characterization of a series'
// overall direction, computed from its first vs. last available
// observation — never inferred from a Message string. Mirrors every
// analytics sibling package's identical TrendDirection.
type TrendDirection string

const (
	TrendIncreasing  TrendDirection = "increasing"
	TrendDeclining   TrendDirection = "declining"
	TrendStable      TrendDirection = "stable"
	TrendUnavailable TrendDirection = "unavailable"
)

// TrendFlatBandPercent is the |change| / |first value| band, as a decimal,
// within which a series is characterized TrendStable rather than
// TrendIncreasing/TrendDeclining. Fixed (not caller-configurable) because it
// is part of this package's versioned FormulaVersion — the same fixed ±5%
// band every analytics sibling package's TrendFlatBandPercent uses.
const TrendFlatBandPercent = 0.05

// Trend is a first-vs-last-observation direction characterization of a
// series (see TrendDirection), plus the underlying comparison values for
// display. This package reports Trend for both LargestEntityShare and HHI
// across PeriodConcentrations — see Result.LargestShareTrend/HHITrend.
type Trend struct {
	Direction TrendDirection `json:"direction"`
	// FirstPeriod/LastPeriod are the chronologically first/last periods with
	// an available observation, when Direction is not TrendUnavailable.
	FirstPeriod financial.Period `json:"first_period,omitempty"`
	LastPeriod  financial.Period `json:"last_period,omitempty"`
	// FirstValue/LastValue are the corresponding observation values.
	FirstValue ConcentrationValue `json:"first_value"`
	LastValue  ConcentrationValue `json:"last_value"`
	// PercentChange is (LastValue - FirstValue) / |FirstValue|. Available
	// only if both values are available and FirstValue.Value != 0.
	PercentChange ConcentrationValue `json:"percent_change"`
}

// DependencyChange compares one entity's share of total amount between two
// chronologically adjacent periods — how concentrated the business's
// dependency on that specific entity became or eased. Computed once per
// chronologically adjacent period pair, for every entity active in either
// period (mirroring revenuequality.CustomerTransition's identical
// adjacent-period-only convention, and computeCustomerTransition's
// new/lost/retained classification, adapted here to a single flat list
// scoped to one entity+period-pair per element rather than pre-aggregated
// categories, since a caller most often wants dependency changes sorted or
// filtered per entity rather than pre-summed).
type DependencyChange struct {
	// FromPeriod/ToPeriod are the two chronologically adjacent periods this
	// change compares.
	FromPeriod financial.Period `json:"from_period"`
	ToPeriod   financial.Period `json:"to_period"`
	// EntityKey is the opaque entity identifier.
	EntityKey string `json:"entity_key"`
	// FromAmount/ToAmount are this entity's amount in FromPeriod/ToPeriod.
	// Available == false with Value == 0 for a period in which the entity
	// had no Observation at all (distinct from an Observation with Amount ==
	// 0).
	FromAmount ConcentrationValue `json:"from_amount"`
	ToAmount   ConcentrationValue `json:"to_amount"`
	// FromShare/ToShare are this entity's share of that period's TotalAmount.
	// Available only under the same rules as PeriodConcentration.HHI.
	FromShare ConcentrationValue `json:"from_share"`
	ToShare   ConcentrationValue `json:"to_share"`
	// ShareChange is ToShare.Value - FromShare.Value, in raw decimal points
	// (0.05 means 5 percentage points). Available only if both are
	// available.
	ShareChange ConcentrationValue `json:"share_change"`
}

// EntityImpact is one entity's contribution to a Scenario's total revenue
// and (when an impact-margin assumption applies) earnings loss.
type EntityImpact struct {
	EntityKey string `json:"entity_key"`
	// RevenueImpact is this entity's amount in Scenario's base Period — the
	// revenue/spend that would be lost.
	RevenueImpact ConcentrationValue `json:"revenue_impact"`
	// EarningsImpact is RevenueImpact.Value * the applicable margin rate
	// (EntityImpactAssumption if one covers EntityKey, else
	// Policy.DefaultImpactMarginRate). Available only if a margin rate
	// applies to this entity — see IssueNoImpactAssumption.
	EarningsImpact ConcentrationValue `json:"earnings_impact"`
	// MarginRateUsed is the margin rate applied to compute EarningsImpact,
	// echoed for auditability. Zero and meaningless when EarningsImpact is
	// unavailable.
	MarginRateUsed float64 `json:"margin_rate_used,omitempty"`
}

// ScenarioKind identifies which hypothetical loss a Scenario models.
type ScenarioKind string

const (
	// ScenarioLostLargestEntity models losing the single largest entity by
	// amount in the base Period.
	ScenarioLostLargestEntity ScenarioKind = "lost_largest_entity"
	// ScenarioTopNLoss models losing the top N entities by amount in the
	// base Period, for each N in Policy.ScenarioTopN — see Scenario.N.
	ScenarioTopNLoss ScenarioKind = "top_n_loss"
)

// Scenario is one deterministic hypothetical-loss calculation: the revenue
// (and, when a margin assumption applies, earnings) impact of losing one or
// more of a period's largest entities, plus the resulting remaining-revenue
// concentration profile. Always computed against the chronologically most
// recent period with available data (concentration risk is a point-in-time
// diligence question, not a trend — mirroring
// revenuequality.ConcentrationSummary's identical "most recent period"
// convention).
type Scenario struct {
	Kind ScenarioKind `json:"kind"`
	// N is the number of entities this scenario removes. 1 for
	// ScenarioLostLargestEntity (redundant with EntityImpacts' length but
	// included for uniform filtering across both Kinds); the corresponding
	// Policy.ScenarioTopN cutoff for ScenarioTopNLoss.
	N int `json:"n"`
	// Period is the base period this scenario was computed against.
	Period financial.Period `json:"period"`
	// EntityImpacts is one EntityImpact per entity removed by this scenario,
	// ordered by Amount descending (the same order RankedEntities uses).
	EntityImpacts []EntityImpact `json:"entity_impacts"`
	// TotalRevenueImpact is the sum of every EntityImpacts[i].RevenueImpact.
	TotalRevenueImpact ConcentrationValue `json:"total_revenue_impact"`
	// TotalEarningsImpact is the sum of every available
	// EntityImpacts[i].EarningsImpact. Available only if every removed
	// entity had an applicable margin rate; a partial sum would understate
	// the true earnings impact without saying so, so this package requires
	// full coverage among the removed entities rather than silently summing
	// a subset.
	TotalEarningsImpact ConcentrationValue `json:"total_earnings_impact"`
	// RemainingRevenue is Period's TotalAmount minus TotalRevenueImpact.
	// Available only if both are available.
	RemainingRevenue ConcentrationValue `json:"remaining_revenue"`
	// RevenueImpactPercent is TotalRevenueImpact.Value / Period's
	// TotalAmount. Available only if both are available and the total is
	// nonzero.
	RevenueImpactPercent ConcentrationValue `json:"revenue_impact_percent"`
}

// Thresholds configures the trigger points for Result.Flags. All ratio
// fields are decimals (a ratio of 0.5 means 50%) unless noted otherwise.
// Kept separate from Policy (which configures reporting cutoffs): Policy
// changes what is reported, Thresholds changes only whether an
// already-computed figure crosses a caller-adjustable line into a Flag —
// mirroring cashflow's identical Options.Thresholds vs. Input.Policy
// separation.
type Thresholds struct {
	// HighLargestEntityShareRatio is LargestEntityShare at or above which
	// FlagHighLargestEntityConcentration triggers.
	HighLargestEntityShareRatio float64
	// HighTop5ShareRatio is the top-5 TopNShare.Share at or above which
	// FlagHighTop5Concentration triggers. If Policy.TopN does not include 5,
	// this flag never triggers (no synthetic top-5 figure is computed solely
	// for flag evaluation).
	HighTop5ShareRatio float64
	// HighHHI is the HHI value (0-10,000 scale) at or above which
	// FlagHighHHI triggers. 2500 is the U.S. DOJ/FTC "highly concentrated"
	// merger-guidelines threshold, used as this package's own default (see
	// DefaultThresholds) purely as a familiar reference point, not because
	// this package endorses antitrust guidelines as a concentration-risk
	// standard.
	HighHHI float64
	// IncreasingLargestShareTrendPoints is the number of percentage points
	// (raw decimal, e.g. 0.1 for 10 points) LargestShareTrend is allowed to
	// rise from its first to its last available observation before
	// FlagIncreasingConcentration triggers.
	IncreasingLargestShareTrendPoints float64
	// HighScenarioRevenueImpactRatio is a Scenario's RevenueImpactPercent at
	// or above which FlagHighScenarioImpact triggers, evaluated against the
	// smallest N present in Policy.ScenarioTopN (the least-severe top-N-loss
	// scenario configured) plus ScenarioLostLargestEntity.
	HighScenarioRevenueImpactRatio float64
}

// DefaultThresholds returns this package's baseline SMB-advisory trigger
// points. A caller with different risk tolerance supplies its own
// Thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		HighLargestEntityShareRatio:       0.25,
		HighTop5ShareRatio:                0.5,
		HighHHI:                           2500,
		IncreasingLargestShareTrendPoints: 0.1,
		HighScenarioRevenueImpactRatio:    0.3,
	}
}

// resolveThresholds returns t if it is non-zero, otherwise
// DefaultThresholds() — the same zero-value-means-defaults rule every
// analytics sibling package's resolveThresholds uses.
func resolveThresholds(t Thresholds) Thresholds {
	if t == (Thresholds{}) {
		return DefaultThresholds()
	}
	return t
}

// Options controls Calculate's optional flag-trigger behavior. The zero
// Options is valid: DefaultThresholds is used.
type Options struct {
	// Thresholds configures every deterministic flag trigger point (see
	// Thresholds). If the zero value, DefaultThresholds() is used.
	Thresholds Thresholds
}

// FlagCode is a stable identifier for one kind of deterministic
// concentration-risk signal, analogous to qoe.FlagCode/cashflow.FlagCode/
// revenuequality.FlagCode.
type FlagCode string

const (
	// FlagHighLargestEntityConcentration means the most recent period's
	// LargestEntityShare is at or above Thresholds.HighLargestEntityShareRatio.
	FlagHighLargestEntityConcentration FlagCode = "HIGH_LARGEST_ENTITY_CONCENTRATION"
	// FlagHighTop5Concentration means the most recent period's top-5
	// TopNShare.Share is at or above Thresholds.HighTop5ShareRatio.
	FlagHighTop5Concentration FlagCode = "HIGH_TOP_5_CONCENTRATION"
	// FlagHighHHI means the most recent period's HHI is at or above
	// Thresholds.HighHHI.
	FlagHighHHI FlagCode = "HIGH_HHI"
	// FlagIncreasingConcentration means LargestShareTrend rose by at least
	// Thresholds.IncreasingLargestShareTrendPoints (raw percentage points)
	// from its first to its last available observation.
	FlagIncreasingConcentration FlagCode = "INCREASING_CONCENTRATION"
	// FlagHighScenarioImpact means ScenarioLostLargestEntity's, or the
	// smallest-N ScenarioTopNLoss's, RevenueImpactPercent is at or above
	// Thresholds.HighScenarioRevenueImpactRatio.
	FlagHighScenarioImpact FlagCode = "HIGH_SCENARIO_IMPACT"
)

// FlagSeverity mirrors every analytics sibling package's identical role: a
// structured, matchable urgency signal, never inferred from Message text.
type FlagSeverity string

const (
	FlagSeverityInfo     FlagSeverity = "info"
	FlagSeverityWarning  FlagSeverity = "warning"
	FlagSeverityCritical FlagSeverity = "critical"
)

// Flag is a single deterministic, explainable concentration-risk signal.
// Every Flag here is rule-based against Thresholds, never AI-scored —
// mirroring qoe.Flag/cashflow.Flag/revenuequality.Flag's identical design.
type Flag struct {
	Code      FlagCode         `json:"code"`
	Severity  FlagSeverity     `json:"severity"`
	Period    financial.Period `json:"period,omitempty"`
	Message   string           `json:"message"`
	Value     float64          `json:"value"`
	Threshold float64          `json:"threshold"`
}

// Result is the output of Calculate: the full per-period concentration
// history, largest-share/HHI trends, entity-level dependency changes,
// lost-entity/top-N-loss scenarios, deterministic flags, and formula
// version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoObservations) — every other field is then zero-value.
	Available bool `json:"available"`
	// Basis echoes Input.Basis (BasisOther if it was empty).
	Basis Basis `json:"basis"`

	// History is one PeriodConcentration per period present in
	// Input.Observations, ordered chronologically when Input.PeriodMeta was
	// supplied and covers every period; otherwise in order of first
	// appearance in Input.Observations.
	History []PeriodConcentration `json:"history,omitempty"`

	// LargestShareTrend characterizes LargestEntityShare's overall
	// first-vs-last direction across History.
	LargestShareTrend Trend `json:"largest_share_trend"`
	// HHITrend characterizes HHI's overall first-vs-last direction across
	// History.
	HHITrend Trend `json:"hhi_trend"`

	// DependencyChanges is one DependencyChange per entity active in either
	// side of every chronologically adjacent period pair in History. Empty
	// if chronological order was unavailable (see IssueNoPeriodMeta/
	// IssuePeriodMissingFromMeta).
	DependencyChanges []DependencyChange `json:"dependency_changes,omitempty"`

	// Scenarios is one Scenario per configured cutoff: always
	// ScenarioLostLargestEntity, plus one ScenarioTopNLoss per
	// Policy.ScenarioTopN entry, all computed against the chronologically
	// most recent period in History. Empty if chronological order was
	// unavailable.
	Scenarios []Scenario `json:"scenarios,omitempty"`

	// Flags is every deterministic signal Calculate triggered, ordered by
	// FlagCode's declaration order, then by Period.
	Flags []Flag `json:"flags,omitempty"`

	// Thresholds echoes the resolved Options.Thresholds (after
	// DefaultThresholds substitution) this Result was computed under.
	Thresholds Thresholds `json:"thresholds"`
	// Policy echoes the resolved Input.Policy (after DefaultPolicy
	// substitution) this Result was computed under.
	Policy Policy `json:"policy"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
