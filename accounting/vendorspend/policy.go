package vendorspend

import "math"

// amountTolerance is the floating-point comparison tolerance used
// throughout this package for reconciliation/balance checks — mirrors
// accounting/ap.amountTolerance/accounting/inventory's identical constant.
const amountTolerance = 0.005

func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}

// MaterialityPolicy configures the absolute/percent-of-total-spend
// materiality test used by flag triggers and control reconciliation —
// task section 32. Deliberately never called "audit materiality" (task
// section 32's explicit naming instruction); it is a caller-adjustable
// analysis threshold, not an attestation concept.
type MaterialityPolicy struct {
	// AbsoluteAmount is a fixed dollar threshold. If zero, only
	// PercentOfTotalSpend applies (if also zero, materiality resolves to
	// DefaultPolicy's baseline — see resolvePolicy).
	AbsoluteAmount float64 `json:"absolute_amount,omitempty"`
	// PercentOfTotalSpend is a decimal (0.01 means 1% of total net spend
	// for the scope being tested).
	PercentOfTotalSpend float64 `json:"percent_of_total_spend,omitempty"`
}

// resolvedThreshold returns the greater of AbsoluteAmount and
// PercentOfTotalSpend * totalSpend — a record is material if it clears
// either test. If totalSpend is zero or negative, only AbsoluteAmount
// applies.
func (m MaterialityPolicy) resolvedThreshold(totalSpend float64) float64 {
	pctAmt := 0.0
	if totalSpend > 0 {
		pctAmt = m.PercentOfTotalSpend * totalSpend
	}
	if m.AbsoluteAmount > pctAmt {
		return m.AbsoluteAmount
	}
	return pctAmt
}

// TailSpendDefinitionKind selects which of the two tail-spend policy
// shapes (task section 21) is active. A caller sets exactly one of
// TailSpendPolicy's two definitional fields; this package never invents
// a default tail-spend cutoff of its own that was not explicitly
// requested — see TailSpend.Available.
type TailSpendDefinitionKind string

const (
	TailSpendUndefined   TailSpendDefinitionKind = ""
	TailSpendBelowAmount TailSpendDefinitionKind = "BELOW_AMOUNT"
	TailSpendOutsideTopN TailSpendDefinitionKind = "OUTSIDE_TOP_N"
)

// TailSpendPolicy configures the caller-defined tail-spend cutoff — task
// section 21. Exactly one of BelowAmount/OutsideTopN should be set
// (non-zero); if both are set, BelowAmount takes precedence — documented
// here, not silent, and echoed on TailSpend.Definition so a caller can
// always see which definition was actually applied. The zero value (both
// fields 0) means "no tail-spend policy configured," not an error — see
// TailSpend.Available.
type TailSpendPolicy struct {
	// BelowAmount, if > 0, defines tail spend as every supplier whose
	// total NetSpend for the scope is below this absolute dollar amount.
	BelowAmount float64 `json:"below_amount,omitempty"`
	// OutsideTopN, if > 0, defines tail spend as every supplier outside
	// the top N suppliers by NetSpend for the scope.
	OutsideTopN int `json:"outside_top_n,omitempty"`
}

func (t TailSpendPolicy) kind() TailSpendDefinitionKind {
	switch {
	case t.BelowAmount > 0:
		return TailSpendBelowAmount
	case t.OutsideTopN > 0:
		return TailSpendOutsideTopN
	default:
		return TailSpendUndefined
	}
}

// Policy configures Calculate's caller-adjustable behavior that is not a
// fixed part of FormulaVersion — task section 33. Mirrors
// analytics/concentration.Policy vs. Thresholds' identical separation of
// "what is reported" from "what triggers a flag" (Thresholds lives in
// flags.go).
type Policy struct {
	// Materiality configures the materiality test used by flag triggers
	// and control reconciliation. Zero value resolves to
	// DefaultPolicy's baseline.
	Materiality MaterialityPolicy `json:"materiality"`
	// TopN lists each supplier-count cutoff SpendConcentration's Top1/
	// Top3/Top5/Top10 fields can report a share for — a cutoff absent
	// from TopN leaves its corresponding field Unavailable (Top1 is the
	// exception: it is always available from the underlying
	// analytics/concentration LargestEntityShare figure regardless of
	// TopN). If empty, DefaultPolicy's []int{1, 3, 5, 10} is used (task
	// section 9's requested 1/3/5/10 cutoffs).
	TopN []int `json:"top_n,omitempty"`
	// SupplierShareIncreasePoints is the number of raw decimal percentage
	// points (0.05 means 5 points) a supplier's period-over-period spend
	// share is allowed to rise before FlagSupplierConcentrationIncreasing
	// triggers.
	SupplierShareIncreasePoints float64 `json:"supplier_share_increase_points,omitempty"`
	// UnitPriceIncreasePercent is the decimal period-over-period unit-
	// price increase (0.1 means 10%) at or above which
	// FlagUnitPriceIncrease triggers.
	UnitPriceIncreasePercent float64 `json:"unit_price_increase_percent,omitempty"`
	// NewSupplierMaterialAmount is the minimum NetSpend a newly-active
	// supplier must reach in its first active period for
	// FlagNewSupplierActivity to trigger. Records below this amount still
	// appear in NewLostSuppliers, just without the flag.
	NewSupplierMaterialAmount float64 `json:"new_supplier_material_amount,omitempty"`
	// LostSupplierMaterialAmount is the minimum prior-period NetSpend a
	// discontinued supplier must have had for
	// FlagSupplierSpendDiscontinued to trigger.
	LostSupplierMaterialAmount float64 `json:"lost_supplier_material_amount,omitempty"`
	// TailSpend configures the tail-spend definition — see
	// TailSpendPolicy. Zero value means TailSpend analysis is
	// unavailable (task section 21's "never report tail spend without
	// the definition used" rule — an undefined policy means no
	// computation, not a silently invented one).
	TailSpend TailSpendPolicy `json:"tail_spend,omitempty"`
	// DuplicateWindowDays bounds the possible-duplicate-spend scan to
	// record pairs within this many days of each other (task section 22's
	// "bounded indexed approach rather than O(N^2)" instruction). If
	// zero, DefaultPolicy's 3-day window is used.
	DuplicateWindowDays int `json:"duplicate_window_days,omitempty"`
	// ObservedRecurringMinPeriods is the minimum number of distinct
	// periods a supplier/category combination must show comparable
	// spend in for FlagObservedRepeatedSpend to trigger (task section
	// 19). If zero, DefaultPolicy's 3 is used.
	ObservedRecurringMinPeriods int `json:"observed_recurring_min_periods,omitempty"`
	// ObservedRecurringAmountTolerance is the decimal tolerance (0.2
	// means 20%) within which consecutive per-period amounts are
	// considered "comparable" for observed-recurring detection. If zero,
	// DefaultPolicy's 0.2 is used.
	ObservedRecurringAmountTolerance float64 `json:"observed_recurring_amount_tolerance,omitempty"`
	// CategoryCoverageThreshold/ProductCoverageThreshold are the decimal
	// coverage ratios below which
	// FlagMaterialUncategorizedSpend/FlagLowProductAttributionCoverage
	// trigger. If zero, DefaultPolicy's 0.9/0.5 are used respectively.
	CategoryCoverageThreshold float64 `json:"category_coverage_threshold,omitempty"`
	ProductCoverageThreshold  float64 `json:"product_coverage_threshold,omitempty"`
	// ControlTolerance is the absolute-dollar tolerance for
	// ControlReconciliation's Reconciled test. If zero, DefaultPolicy's
	// $0.01 is used.
	ControlTolerance float64 `json:"control_tolerance,omitempty"`
	// RequireSingleBasis, if true, makes mixed SpendBasis values among
	// included records emit IssueMixedSpendBasis at SeverityError rather
	// than the default advisory treatment (task section 6: "if mixed
	// basis is intentionally allowed, summarize separately" — the default
	// is to allow it and rely on Result.SpendByBasis for transparency).
	RequireSingleBasis bool `json:"require_single_basis,omitempty"`
	// ReportingCurrency, if non-empty, is the single currency an analysis
	// proceeds under; records in another currency are excluded from every
	// aggregate total and flagged (IssueMixedCurrency). If empty, the
	// most common currency among included records is used automatically.
	ReportingCurrency string `json:"reporting_currency,omitempty"`
}

// DefaultPolicy returns this package's baseline configuration.
func DefaultPolicy() Policy {
	return Policy{
		TopN:                             []int{1, 3, 5, 10},
		SupplierShareIncreasePoints:      0.1,
		UnitPriceIncreasePercent:         0.1,
		DuplicateWindowDays:              3,
		ObservedRecurringMinPeriods:      3,
		ObservedRecurringAmountTolerance: 0.2,
		CategoryCoverageThreshold:        0.9,
		ProductCoverageThreshold:         0.5,
		ControlTolerance:                 0.01,
	}
}

// resolvePolicy returns p with every empty/zero field replaced by
// DefaultPolicy's corresponding field — the same zero-value-means-
// defaults rule every analytics sibling package's resolvePolicy uses.
// Materiality, TailSpend, RequireSingleBasis, and ReportingCurrency are
// deliberately NOT defaulted here: a zero MaterialityPolicy/TailSpendPolicy
// is a meaningful "not configured" state (see their own doc comments),
// and RequireSingleBasis/ReportingCurrency default to false/"" correctly
// as Go zero values.
func resolvePolicy(p Policy) Policy {
	d := DefaultPolicy()
	if len(p.TopN) == 0 {
		p.TopN = d.TopN
	}
	if p.SupplierShareIncreasePoints == 0 {
		p.SupplierShareIncreasePoints = d.SupplierShareIncreasePoints
	}
	if p.UnitPriceIncreasePercent == 0 {
		p.UnitPriceIncreasePercent = d.UnitPriceIncreasePercent
	}
	if p.DuplicateWindowDays == 0 {
		p.DuplicateWindowDays = d.DuplicateWindowDays
	}
	if p.ObservedRecurringMinPeriods == 0 {
		p.ObservedRecurringMinPeriods = d.ObservedRecurringMinPeriods
	}
	if p.ObservedRecurringAmountTolerance == 0 {
		p.ObservedRecurringAmountTolerance = d.ObservedRecurringAmountTolerance
	}
	if p.CategoryCoverageThreshold == 0 {
		p.CategoryCoverageThreshold = d.CategoryCoverageThreshold
	}
	if p.ProductCoverageThreshold == 0 {
		p.ProductCoverageThreshold = d.ProductCoverageThreshold
	}
	if p.ControlTolerance == 0 {
		p.ControlTolerance = d.ControlTolerance
	}
	return p
}
