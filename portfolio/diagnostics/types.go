// Package diagnostics scans a portfolio of already-computed
// client/business analytics — one [BusinessSnapshot] per business, each a
// condensed summary of whichever sibling packages' results a caller has on
// hand — and surfaces ranked, deterministic findings that indicate an
// account needs attention or advisory follow-up.
//
// This package fetches nothing and persists nothing: no database, no
// client-data retrieval, no I/O. It is a pure aggregation/scanning layer
// exactly like [transactions/salereadiness], but one level up — where
// salereadiness assesses a single business from several sibling Results,
// diagnostics assesses many businesses (a "portfolio") from a much smaller
// per-business summary of those same sibling packages' output, ranking
// findings across the whole book rather than dimensions within one
// business.
//
// # Summaries, not full sibling Results
//
// [BusinessSnapshot] embeds condensed summary structs
// ([QoESummary], [RatioHealthSummary], [ConcentrationSummary],
// [CashFlowSummary], [ValuationSummary], [SaleReadinessSummary]) rather
// than the full sibling Result types those packages return. A caller
// already holding e.g. an [analytics/qoe.Result] populates [QoESummary]'s
// handful of fields from it (see each Summary type's doc comment for the
// exact source fields). This keeps a multi-hundred-business portfolio's
// JSON payload proportional to what diagnostics actually needs, rather
// than multiplying six sibling packages' full output by every business in
// the book. Every summary field is independently optional — see
// [BusinessSnapshot]'s doc comment.
//
// # Trend findings require a prior snapshot
//
// A finding like "margin deterioration" or "revenue decline" is a
// comparison, not a single-period reading. [BusinessSnapshot.Prior] is an
// optional pointer to that same business's previous-period snapshot; every
// change-based finding is skipped (not scored as "no change") when Prior
// is nil or the specific summary field being compared is unavailable on
// either side — see each detect* function in findings.go.
//
// # No sales pitches
//
// Every [Finding] states a fact about already-computed data plus a
// [ReviewCategory] pointing to which advisory conversation it belongs in
// (e.g. "cash flow", "valuation"). Findings never recommend a specific
// engagement, product, or service, and never claim a business will benefit
// from advisory follow-up in dollar terms — mirroring
// [transactions/salereadiness]'s identical no-promise discipline.
//
// # Determinism
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state, no wall-clock/randomness. Calculate can
// be called concurrently and repeatedly against identical input and always
// returns byte-for-byte identical JSON — see determinism_test.go.
package diagnostics

import "github.com/themurtez/go-valuate/financial"

// FormulaVersion identifies this package's fixed rule set: every detect*
// finding rule in findings.go, DefaultPolicy, and the priority-score
// formula in score.go. Bump whenever any of that changes in a way that
// could make a historical Result not reproduce identically under new code
// — see the repository README's versioning-strategy section, which this
// constant follows exactly (qoe.FormulaVersion, salereadiness.FormulaVersion,
// etc.). Echoed on every Result.
const FormulaVersion = "1.0.0"

// ScoreVersion identifies the exact formula computePriorityScore implements
// (score.go), versioned separately from FormulaVersion for the same reason
// salereadiness.ScoreVersion is distinct from salereadiness.FormulaVersion:
// a caller may reasonably want to change which findings are detected
// independently of how already-detected findings are ranked into one
// priority number.
const ScoreVersion = "1.0.0"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown because
// a required input was absent" — this package's own copy of the convention
// every sibling package duplicates locally (see
// transactions/salereadiness.Value's doc comment for the full rationale)
// rather than importing another package's Value.
type Value struct {
	Available bool    `json:"available"`
	Amount    float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may legitimately
// be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// Direction is a coarse, deterministic characterization of how a figure
// moved between a snapshot and its Prior — this package's own copy of the
// increasing/declining/stable/unavailable convention every sibling package
// with trend output duplicates locally (analytics/cashflow.TrendDirection,
// analytics/workingcapital.TrendDirection, analytics/ratios.RatioTrendDirection,
// analytics/concentration.TrendDirection) rather than importing any one of
// them, since diagnostics compares figures that originate from several
// different sibling packages under one uniform rule.
type Direction string

const (
	DirectionImproving     Direction = "improving"
	DirectionDeteriorating Direction = "deteriorating"
	DirectionStable        Direction = "stable"
	DirectionUnavailable   Direction = "unavailable"
)

// DirectionFlatBandPercent is the |change| / |prior value| band, as a
// decimal, within which a change is characterized DirectionStable rather
// than DirectionImproving/DirectionDeteriorating. Fixed (not
// caller-configurable) because it is part of this package's versioned
// FormulaVersion — mirroring workingcapital.TrendFlatBandPercent/
// ratios.TrendFlatBandPercent's identical rationale and value.
const DirectionFlatBandPercent = 0.05

// SelectedMetrics is a small, caller-chosen set of headline figures for one
// business/period — the "selected metrics" Prompt 33 calls for distinct
// from the more detailed sibling summaries below. A caller populates this
// from whichever financial/metrics.Snapshot or analytics/ratios.PeriodRatios
// fields it considers headline for its own book (e.g. revenue, EBITDA,
// EBITDA margin); diagnostics does not prescribe which metrics belong here
// beyond Revenue and EBITDA, since "selected" is a caller judgment call by
// design.
type SelectedMetrics struct {
	// Revenue is total revenue for the snapshot's Period.
	Revenue Value `json:"revenue"`
	// EBITDA is EBITDA for the snapshot's Period.
	EBITDA Value `json:"ebitda"`
	// EBITDAMargin is EBITDA / Revenue for the snapshot's Period, as a
	// decimal.
	EBITDAMargin Value `json:"ebitda_margin"`
	// Extra holds any additional caller-chosen figures beyond
	// Revenue/EBITDA/EBITDAMargin, keyed by a caller-defined stable metric
	// name (e.g. "NET_INCOME"), carried through JSON round-trips for a
	// caller's own downstream display/reporting. No detect* rule in this
	// package reads Extra — it is not an input to Calculate's finding
	// detection, only a pass-through convenience field, so its map key
	// order never affects Result's determinism (JSON map keys already
	// marshal in sorted order regardless).
	Extra map[string]Value `json:"extra,omitempty"`
}

// QoESummary condenses an analytics/qoe.Result: the earnings-quality
// signals most relevant to portfolio-level review, populated by a caller
// from its own already-computed qoe.Result. Every field is independently
// optional; the zero value means "not supplied."
type QoESummary struct {
	// Available mirrors qoe.Result.Available — false means this summary was
	// not populated (Calculate treats every other field as absent).
	Available bool `json:"available"`
	// FormulaVersion echoes qoe.Result.FormulaVersion, carried for
	// traceability back to exactly which qoe formula version produced this
	// summary.
	FormulaVersion string `json:"formula_version,omitempty"`
	// EBITDAVolatility is qoe.Result.EBITDAVolatility.Value.Value (a
	// decimal, e.g. 0.25 = 25%), when qoe.Result.EBITDAVolatility.Value.Available.
	EBITDAVolatility Value `json:"ebitda_volatility"`
	// AdjustmentToEBITDA is qoe.Result.Ratios.AdjustmentToEBITDA — how large
	// the add-back/adjustment burden is relative to reported EBITDA for the
	// most recent period, as a decimal.
	AdjustmentToEBITDA Value `json:"adjustment_to_ebitda"`
	// HasCriticalFlags is true when qoe.Result.Flags contains at least one
	// entry with qoe.FlagSeverityCritical — a caller-computed roll-up,
	// since diagnostics does not import analytics/qoe's Flag type directly.
	HasCriticalFlags bool `json:"has_critical_flags,omitempty"`
	// OpenFlagCount is len(qoe.Result.Flags), regardless of severity.
	OpenFlagCount int `json:"open_flag_count,omitempty"`
}

// RatioHealthSummary condenses an analytics/ratios.Result: the
// profitability/liquidity/leverage health signals most relevant to
// portfolio-level review, populated by a caller from its own
// already-computed ratios.Result. Every field is independently optional.
type RatioHealthSummary struct {
	// Available mirrors ratios.Result.Available.
	Available bool `json:"available"`
	// FormulaVersion echoes ratios.Result.FormulaVersion.
	FormulaVersion string `json:"formula_version,omitempty"`
	// EBITDAMargin is the most recent period's
	// PeriodRatios.EBITDAMargin.Value.Value, as a decimal.
	EBITDAMargin Value `json:"ebitda_margin"`
	// NetDebtToEBITDA is the most recent period's
	// PeriodRatios.NetDebtToEBITDA.Value.Value, as a decimal multiple.
	NetDebtToEBITDA Value `json:"net_debt_to_ebitda"`
	// CurrentRatio is the most recent period's
	// PeriodRatios.CurrentRatio.Value.Value.
	CurrentRatio Value `json:"current_ratio"`
	// HasRisingLeverageSignal is true when ratios.Result.Signals contains a
	// ratios.SignalRisingLeverage entry.
	HasRisingLeverageSignal bool `json:"has_rising_leverage_signal,omitempty"`
	// HasMarginCompressionSignal is true when ratios.Result.Signals
	// contains a ratios.SignalMarginCompression or
	// ratios.SignalDeterioratingProfitability entry.
	HasMarginCompressionSignal bool `json:"has_margin_compression_signal,omitempty"`
	// HasWeakeningLiquiditySignal is true when ratios.Result.Signals
	// contains a ratios.SignalWeakeningLiquidity entry.
	HasWeakeningLiquiditySignal bool `json:"has_weakening_liquidity_signal,omitempty"`
	// CriticalSignalCount is the number of ratios.Result.Signals entries
	// with ratios.SignalSeverityCritical.
	CriticalSignalCount int `json:"critical_signal_count,omitempty"`
}

// ConcentrationSummary condenses an analytics/concentration.Result:
// customer/counterparty concentration, populated by a caller from its own
// already-computed concentration.Result. Every field is independently
// optional.
type ConcentrationSummary struct {
	// Available mirrors concentration.Result.Available.
	Available bool `json:"available"`
	// FormulaVersion echoes concentration.Result.FormulaVersion.
	FormulaVersion string `json:"formula_version,omitempty"`
	// LargestEntityShare is the most recent period's
	// PeriodConcentration.LargestEntityShare.Value, as a decimal.
	LargestEntityShare Value `json:"largest_entity_share"`
	// HHI is the most recent period's PeriodConcentration.HHI.Value, on the
	// standard 0-10,000 Herfindahl-Hirschman scale.
	HHI Value `json:"hhi"`
	// LargestShareTrend is concentration.Result.LargestShareTrend.Direction,
	// translated to this package's own Direction
	// (concentration.TrendIncreasing -> DirectionDeteriorating, since a
	// larger largest-customer share is a worsening concentration signal;
	// concentration.TrendDeclining -> DirectionImproving;
	// concentration.TrendStable -> DirectionStable;
	// concentration.TrendUnavailable -> DirectionUnavailable).
	LargestShareTrend Direction `json:"largest_share_trend"`
}

// CashFlowSummary condenses an analytics/cashflow.Result: EBITDA-to-cash
// conversion and runway, populated by a caller from its own
// already-computed cashflow.Result. Every field is independently optional.
type CashFlowSummary struct {
	// Available mirrors cashflow.Result.Available.
	Available bool `json:"available"`
	// FormulaVersion echoes cashflow.Result.FormulaVersion.
	FormulaVersion string `json:"formula_version,omitempty"`
	// EBITDAToFreeCashFlow is the most recent period's
	// ConversionRatios.EBITDAToFreeCashFlow.Value, as a decimal.
	EBITDAToFreeCashFlow Value `json:"ebitda_to_free_cash_flow"`
	// ConversionTrend is cashflow.Result.ConversionTrend.Direction,
	// translated to this package's own Direction the same way
	// ConcentrationSummary.LargestShareTrend is (cashflow.TrendIncreasing
	// conversion -> DirectionImproving; cashflow.TrendDeclining ->
	// DirectionDeteriorating; cashflow.TrendStable -> DirectionStable;
	// cashflow.TrendUnavailable -> DirectionUnavailable).
	ConversionTrend Direction `json:"conversion_trend"`
	// HasDecliningConversionFlag is true when cashflow.Result.Flags
	// contains a cashflow.FlagDecliningConversionTrend or
	// cashflow.FlagWeakCashConversion entry.
	HasDecliningConversionFlag bool `json:"has_declining_conversion_flag,omitempty"`
	// MonthsOfRunway is cashflow.Result.CashRunway.MonthsOfRunway.Value,
	// when cashflow.Result.CashRunway.Available.
	MonthsOfRunway Value `json:"months_of_runway"`
}

// ValuationSummary condenses a valuation/consensus.Result: the indicated
// value point estimate and method agreement, populated by a caller from
// its own already-computed consensus.Result. Every field is independently
// optional.
type ValuationSummary struct {
	// Available mirrors consensus.Result.Available.
	Available bool `json:"available"`
	// FormulaVersion echoes consensus.Result.FormulaVersion.
	FormulaVersion string `json:"formula_version,omitempty"`
	// IndicatedValue is consensus.Result.Statistics.WeightedMean when
	// consensus.Result.WeightsValid, else Statistics.SimpleMean — this
	// package's single best-point-estimate figure for the business, on
	// consensus.Result.Basis.
	IndicatedValue Value `json:"indicated_value"`
	// RangeLow/RangeHigh are consensus.Result.Range.Min/Max.
	RangeLow  Value `json:"range_low"`
	RangeHigh Value `json:"range_high"`
	// MethodCount is consensus.Result.Statistics.Count — how many valuation
	// methods contributed to IndicatedValue.
	MethodCount int `json:"method_count,omitempty"`
	// DispersionLevel is consensus.Result.Dispersion.Level, copied
	// verbatim as a plain string so this package need not import
	// valuation/consensus's Level type for one field
	// ("HIGH_CONSENSUS"/"MODERATE_CONSENSUS"/"LOW_CONSENSUS").
	DispersionLevel string `json:"dispersion_level,omitempty"`
}

// SaleReadinessSummary condenses a transactions/salereadiness.Result,
// populated by a caller from its own already-computed salereadiness.Result.
// Every field is independently optional.
type SaleReadinessSummary struct {
	// Available mirrors salereadiness.Result.Available.
	Available bool `json:"available"`
	// FormulaVersion echoes salereadiness.Result.FormulaVersion.
	FormulaVersion string `json:"formula_version,omitempty"`
	// OverallScore is salereadiness.Result.OverallScore.Value, when
	// OverallScore is non-nil, on its native [0, 100] scale.
	OverallScore Value `json:"overall_score"`
	// BlockerCount is len(salereadiness.Result.Blockers).
	BlockerCount int `json:"blocker_count,omitempty"`
	// StrengthCount is len(salereadiness.Result.Strengths).
	StrengthCount int `json:"strength_count,omitempty"`
	// CoveragePercent is salereadiness.Result.Coverage.CoveragePercent, as
	// a decimal.
	CoveragePercent Value `json:"coverage_percent"`
}

// BusinessSnapshot is one business's condensed, already-computed
// analytical state at one point in time — the unit Calculate scans. Every
// field beyond ID and Period is optional; Calculate degrades gracefully
// (skipping whichever findings a field's absence makes undetectable) when
// a summary is left at its zero value / Available == false — see each
// detect* function in findings.go for exactly which fields drive which
// finding.
type BusinessSnapshot struct {
	// ID is an opaque, caller-defined business identifier (e.g. a client
	// record ID). Required; Calculate skips a snapshot with an empty ID and
	// reports IssueEmptyBusinessID.
	ID string `json:"id"`
	// Label is an optional human-readable display name, carried through to
	// Finding output for display purposes only — never parsed or matched
	// on by this package's own logic (ID is the stable key).
	Label string `json:"label,omitempty"`
	// Period is the reporting period this snapshot describes.
	Period financial.Period `json:"period"`

	Metrics       SelectedMetrics      `json:"metrics"`
	QoE           QoESummary           `json:"qoe"`
	RatioHealth   RatioHealthSummary   `json:"ratio_health"`
	Concentration ConcentrationSummary `json:"concentration"`
	CashFlow      CashFlowSummary      `json:"cash_flow"`
	Valuation     ValuationSummary     `json:"valuation"`
	SaleReadiness SaleReadinessSummary `json:"sale_readiness"`

	// Prior is this same business's snapshot for the immediately preceding
	// period being compared against, when the caller has one — the sole
	// source for every change-based finding (margin deterioration, revenue
	// decline, cash-conversion weakening, leverage increase, concentration
	// increase, valuation movement). nil means no prior snapshot is
	// available; every change-based finding is then skipped for this
	// business rather than scored as "no change" — see
	// CoverageCounts.WithPrior.
	//
	// Prior is itself a *BusinessSnapshot (not a further-nested Prior) —
	// Calculate only ever compares a snapshot to its immediate Prior, never
	// walks a chain, so a caller wanting a longer history simply calls
	// Calculate once per adjacent pair it cares about.
	Prior *BusinessSnapshot `json:"prior,omitempty"`
}

// Policy is the caller-supplied threshold/weighting configuration driving
// every change-based finding's materiality bar and computePriorityScore's
// weights. Every threshold's zero value falls back to this package's fixed
// DefaultPolicy value for that field — distinct from
// transactions/salereadiness.Policy's "zero means skip the check" model,
// because Prompt 33 requires the priority formula to be
// "deterministic and configurable" (not optional-per-field): a portfolio
// scan with no Policy supplied should still rank every business, using
// sensible fixed defaults, rather than silently disabling ranking.
type Policy struct {
	// MaterialChangePercent is the minimum |change| / |prior value| a
	// change-based finding (margin, revenue, cash conversion, leverage,
	// concentration, valuation) requires to be reported at all — the same
	// role as Direction's flat band, but caller-configurable per portfolio
	// rather than fixed like DirectionFlatBandPercent, since what counts as
	// "material enough to flag to an accountant" reasonably varies by
	// portfolio. Zero falls back to DefaultPolicy.MaterialChangePercent
	// (0.05, matching DirectionFlatBandPercent).
	MaterialChangePercent float64 `json:"material_change_percent,omitempty"`
	// SeverityWeights maps each Severity to its contribution to
	// computePriorityScore (score.go). Nil/missing entries fall back to
	// DefaultPolicy.SeverityWeights' value for that Severity.
	SeverityWeights map[Severity]float64 `json:"severity_weights,omitempty"`
}

// DefaultPolicy is the fixed threshold/weighting set Calculate uses for any
// Policy field left at its zero value, and the Policy an empty
// Input.Policy resolves to entirely. Part of this package's versioned
// FormulaVersion — a change to any value here requires a FormulaVersion
// bump.
var DefaultPolicy = Policy{
	MaterialChangePercent: DirectionFlatBandPercent,
	SeverityWeights: map[Severity]float64{
		SeverityCritical: 10,
		SeverityWarning:  5,
		SeverityInfo:     1,
	},
}

// resolvePolicy returns p with every zero-value field replaced by
// DefaultPolicy's corresponding value, never mutating p or DefaultPolicy.
func resolvePolicy(p Policy) Policy {
	resolved := p
	if resolved.MaterialChangePercent == 0 {
		resolved.MaterialChangePercent = DefaultPolicy.MaterialChangePercent
	}
	if resolved.SeverityWeights == nil {
		resolved.SeverityWeights = DefaultPolicy.SeverityWeights
	} else {
		merged := make(map[Severity]float64, len(DefaultPolicy.SeverityWeights))
		for k, v := range DefaultPolicy.SeverityWeights {
			merged[k] = v
		}
		for k, v := range resolved.SeverityWeights {
			merged[k] = v
		}
		resolved.SeverityWeights = merged
	}
	return resolved
}

// Input bundles everything Calculate needs: the portfolio to scan plus the
// Policy to scan it under.
type Input struct {
	// Portfolio is every business snapshot to scan, in caller-supplied
	// order. Order does not affect Result.Findings' order (Findings is
	// always sorted by priority — see Result.Findings' doc comment) or
	// Result.Coverage (a set of counts, not an ordered list).
	Portfolio []BusinessSnapshot `json:"portfolio"`
	// Policy is the caller-supplied threshold/weighting configuration. The
	// zero value resolves entirely to DefaultPolicy — see Policy's doc
	// comment.
	Policy Policy `json:"policy"`
}

// Severity distinguishes how urgently a Finding should be addressed —
// mirroring transactions/salereadiness.Severity's identical three-level
// scale and this package's own copy of it (see that type's doc comment for
// why every sibling package with a severity concept duplicates its own
// rather than sharing one).
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// FindingCode is a stable identifier for one kind of portfolio diagnostic
// finding. Declaration order below is the tie-break order two Findings with
// equal PriorityScore fall back to — see sortFindings (score.go).
type FindingCode string

const (
	// FindingMarginDeterioration means EBITDAMargin (from RatioHealth, or
	// Metrics as a fallback) declined by at least Policy.MaterialChangePercent
	// between Prior and the current snapshot, or RatioHealth carries an
	// active margin-compression/deteriorating-profitability signal.
	FindingMarginDeterioration FindingCode = "MARGIN_DETERIORATION"
	// FindingRevenueDecline means Metrics.Revenue declined by at least
	// Policy.MaterialChangePercent between Prior and the current snapshot.
	FindingRevenueDecline FindingCode = "REVENUE_DECLINE"
	// FindingCashConversionWeakening means CashFlow.ConversionTrend is
	// DirectionDeteriorating, EBITDAToFreeCashFlow declined by at least
	// Policy.MaterialChangePercent versus Prior, or CashFlow carries an
	// active declining-conversion flag.
	FindingCashConversionWeakening FindingCode = "CASH_CONVERSION_WEAKENING"
	// FindingLeverageIncrease means RatioHealth.NetDebtToEBITDA increased by
	// at least Policy.MaterialChangePercent versus Prior, or RatioHealth
	// carries an active rising-leverage signal.
	FindingLeverageIncrease FindingCode = "LEVERAGE_INCREASE"
	// FindingConcentrationIncrease means Concentration.LargestEntityShare
	// increased by at least Policy.MaterialChangePercent versus Prior, or
	// Concentration.LargestShareTrend is DirectionDeteriorating.
	FindingConcentrationIncrease FindingCode = "CONCENTRATION_INCREASE"
	// FindingValuationMovement means Valuation.IndicatedValue changed
	// (either direction) by at least Policy.MaterialChangePercent versus
	// Prior — the one finding code that fires on either a decline or a
	// rise, since a large valuation swing in either direction is itself the
	// signal worth an advisory conversation, not just a decline.
	FindingValuationMovement FindingCode = "VALUATION_MOVEMENT"
	// FindingUnresolvedFinancialQuality means QoE.HasCriticalFlags is true,
	// QoE.AdjustmentToEBITDA is unusually large (see findings.go's fixed
	// threshold), or RatioHealth.CriticalSignalCount > 0 — a single-period
	// reading, not a change, so it fires even with no Prior.
	FindingUnresolvedFinancialQuality FindingCode = "UNRESOLVED_FINANCIAL_QUALITY"
	// FindingSaleReadinessOpportunity means SaleReadiness.BlockerCount > 0
	// or SaleReadiness.OverallScore is below a fixed threshold — a
	// single-period reading pointing at an advisory opportunity, framed as
	// a factual observation about already-computed data (see the package
	// doc comment's no-sales-pitch rule), never a recommendation to pursue
	// a sale.
	FindingSaleReadinessOpportunity FindingCode = "SALE_READINESS_OPPORTUNITY"
)

// findingOrder is FindingCode's fixed declaration order, used as the
// tie-break when two Findings for the same business have equal
// PriorityScore — see sortFindings (score.go).
var findingOrder = []FindingCode{
	FindingMarginDeterioration,
	FindingRevenueDecline,
	FindingCashConversionWeakening,
	FindingLeverageIncrease,
	FindingConcentrationIncrease,
	FindingValuationMovement,
	FindingUnresolvedFinancialQuality,
	FindingSaleReadinessOpportunity,
}

// ReviewCategory is a stable identifier for which advisory conversation a
// Finding belongs in — deliberately coarser than FindingCode, since several
// finding codes can point a preparer toward the same category of
// follow-up conversation with a client.
type ReviewCategory string

const (
	ReviewCategoryProfitability        ReviewCategory = "PROFITABILITY"
	ReviewCategoryCashFlow             ReviewCategory = "CASH_FLOW"
	ReviewCategoryRisk                 ReviewCategory = "RISK"
	ReviewCategoryValuation            ReviewCategory = "VALUATION"
	ReviewCategoryFinancialQuality     ReviewCategory = "FINANCIAL_QUALITY"
	ReviewCategoryTransactionReadiness ReviewCategory = "TRANSACTION_READINESS"
)

// Finding is one deterministic diagnostic observation about one business —
// the unit Result.Findings ranks.
type Finding struct {
	// BusinessID echoes the BusinessSnapshot.ID this Finding concerns.
	BusinessID string `json:"business_id"`
	// BusinessLabel echoes BusinessSnapshot.Label, for display only.
	BusinessLabel string `json:"business_label,omitempty"`
	// Code identifies which kind of finding this is.
	Code FindingCode `json:"code"`
	// Severity is this Finding's structured urgency.
	Severity Severity `json:"severity"`
	// Metric names the underlying figure this Finding was derived from
	// (e.g. "EBITDA_MARGIN", "LARGEST_ENTITY_SHARE"), for traceability —
	// never parsed for meaning distinct from Code.
	Metric string `json:"metric"`
	// CurrentValue is Metric's value on the current snapshot.
	CurrentValue Value `json:"current_value"`
	// PriorValue is Metric's value on BusinessSnapshot.Prior, when Prior
	// was supplied and this Finding is change-based. Unavailable for a
	// single-period finding (FindingUnresolvedFinancialQuality,
	// FindingSaleReadinessOpportunity).
	PriorValue Value `json:"prior_value"`
	// Change is (CurrentValue - PriorValue) / |PriorValue|, as a decimal,
	// when both CurrentValue and PriorValue are Available and PriorValue is
	// nonzero. Unavailable under the same conditions as PriorValue, or when
	// PriorValue.Amount is exactly 0.
	Change Value `json:"change"`
	// Reason is a short, fixed-form human-readable statement of why this
	// Finding was raised, built entirely from already-computed fields —
	// never free text a caller must parse for meaning distinct from Code.
	Reason string `json:"reason"`
	// SuggestedReviewCategory names which advisory conversation this
	// Finding points toward.
	SuggestedReviewCategory ReviewCategory `json:"suggested_review_category"`
	// PriorityScore is this Finding's individual contribution to ranking —
	// see score.go's computePriorityScore. Higher means more urgent.
	PriorityScore float64 `json:"priority_score"`
}

// CoverageCounts summarizes how many businesses in the scanned portfolio
// actually had each optional summary available, letting a caller see at a
// glance how complete the underlying data was without recomputing it from
// Input.Portfolio.
type CoverageCounts struct {
	// TotalBusinesses is len(Input.Portfolio).
	TotalBusinesses int `json:"total_businesses"`
	// WithQoE, WithRatioHealth, WithConcentration, WithCashFlow,
	// WithValuation, WithSaleReadiness count snapshots whose corresponding
	// summary has Available == true.
	WithQoE           int `json:"with_qoe"`
	WithRatioHealth   int `json:"with_ratio_health"`
	WithConcentration int `json:"with_concentration"`
	WithCashFlow      int `json:"with_cash_flow"`
	WithValuation     int `json:"with_valuation"`
	WithSaleReadiness int `json:"with_sale_readiness"`
	// WithPrior counts snapshots with a non-nil Prior, the precondition for
	// every change-based Finding.
	WithPrior int `json:"with_prior"`
}

// PortfolioCounts summarizes Result.Findings by Severity and FindingCode —
// the structured counterpart to scanning Result.Findings by hand.
type PortfolioCounts struct {
	// TotalFindings is len(Result.Findings).
	TotalFindings int `json:"total_findings"`
	// BusinessesWithFindings is the number of distinct BusinessID values
	// present in Result.Findings.
	BusinessesWithFindings int `json:"businesses_with_findings"`
	// BySeverity maps each Severity present in Result.Findings to its
	// count. Always has an entry for every Severity value that appears at
	// least once; never a zero-count entry for a Severity that did not
	// occur.
	BySeverity map[Severity]int `json:"by_severity,omitempty"`
	// ByCode maps each FindingCode present in Result.Findings to its
	// count, same zero-omission rule as BySeverity.
	ByCode map[FindingCode]int `json:"by_code,omitempty"`
}

// IssueSeverity distinguishes an input problem Calculate could not proceed
// past for some portion of the analysis (IssueSeverityError) from one that
// is advisory only (IssueSeverityWarning) — this package's own two-severity
// model, matching every sibling package's identical convention.
type IssueSeverity string

const (
	IssueSeverityError   IssueSeverity = "error"
	IssueSeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent with
// every other package in this repository.
type IssueCode string

const (
	// IssueEmptyPortfolio means Input.Portfolio had zero entries.
	IssueEmptyPortfolio IssueCode = "EMPTY_PORTFOLIO"
	// IssueEmptyBusinessID means a BusinessSnapshot in Input.Portfolio had
	// an empty ID; that snapshot is skipped entirely (not scanned, not
	// counted in Coverage).
	IssueEmptyBusinessID IssueCode = "EMPTY_BUSINESS_ID"
	// IssueDuplicateBusinessID means two or more BusinessSnapshot entries
	// in Input.Portfolio shared the same ID; every entry after the first is
	// skipped, mirroring map-key-collision handling elsewhere in this
	// repository (last-write-wins is never used, since silently dropping a
	// business's findings without saying so would violate this package's
	// coverage-transparency rule).
	IssueDuplicateBusinessID IssueCode = "DUPLICATE_BUSINESS_ID"
)

// Issue is a single Calculate-time input finding.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// HasErrors reports whether any Issue in issues has IssueSeverityError.
//
// Intentionally duplicated from every sibling package's identical
// HasErrors rather than shared — see
// transactions/salereadiness.HasErrors's doc comment for the full
// rationale (each package's Issue is a distinct Go type with no common
// interface worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == IssueSeverityError {
			return true
		}
	}
	return false
}

// Result is the output of Calculate: every ranked Finding across the
// scanned portfolio, portfolio-level counts, and a coverage/missing-data
// summary.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed rule
	// set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// ScoreVersion identifies which version of computePriorityScore
	// produced every Finding.PriorityScore.
	ScoreVersion string `json:"score_version"`
	// Available is false only if Calculate could not proceed at all (every
	// BusinessSnapshot in Input.Portfolio was skipped — empty ID or empty
	// Input.Portfolio) — every other field is then zero-value, mirroring
	// every sibling package's identical Available convention.
	Available bool `json:"available"`

	// Findings is every diagnostic finding across the whole portfolio,
	// ordered by PriorityScore descending, then by findingOrder's
	// declaration order, then by BusinessID ascending — see sortFindings in
	// score.go for the exact, tested comparator.
	Findings []Finding `json:"findings,omitempty"`

	// Counts summarizes Findings by Severity and FindingCode.
	Counts PortfolioCounts `json:"counts"`
	// Coverage summarizes how many businesses had each optional summary
	// available.
	Coverage CoverageCounts `json:"coverage"`

	// Policy echoes the resolved Policy (after DefaultPolicy substitution)
	// this Result was computed under.
	Policy Policy `json:"policy"`

	// Warnings carries every Issue with IssueSeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with IssueSeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
