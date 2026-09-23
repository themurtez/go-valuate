package inventory

// NegativeInventoryHandling controls how a negative QuantityOnHand or
// InventoryValue is treated — task section 9. Negative inventory can
// legitimately result from timing/system issues (a shipment recorded
// before its matching receipt), so this package never auto-corrects it to
// zero; the only choice is whether to reject the row (exclude it, with an
// error Issue) or allow it through with a prominent warning-level
// finding.
type NegativeInventoryHandling string

const (
	// NegativeInventoryAllowWithWarning includes the row in every
	// calculation and reports FlagNegativeInventory. Default.
	NegativeInventoryAllowWithWarning NegativeInventoryHandling = "ALLOW_WITH_WARNING"
	// NegativeInventoryReject excludes the row from computation and
	// reports IssueInvalidQuantity/IssueInvalidCost as an error.
	NegativeInventoryReject NegativeInventoryHandling = "REJECT"
)

// resolvedNegativeInventoryHandling returns h if recognized, otherwise
// NegativeInventoryAllowWithWarning.
func resolvedNegativeInventoryHandling(h NegativeInventoryHandling) NegativeInventoryHandling {
	if h == NegativeInventoryReject {
		return h
	}
	return NegativeInventoryAllowWithWarning
}

// ReconciliationTolerance configures how a subledger figure is compared
// against its corresponding GL/rollforward figure — mirrors
// labor.ReconciliationTolerance/cashforecast's identical tolerance-policy
// shape. A difference is Reconciled when abs(Difference) <=
// AbsoluteTolerance, or (if PercentTolerance > 0) when abs(Difference) <=
// PercentTolerance * abs(the comparison's reference amount).
type ReconciliationTolerance struct {
	AbsoluteTolerance float64 `json:"absolute_tolerance,omitempty"`
	PercentTolerance  float64 `json:"percent_tolerance,omitempty"`
}

// Policy is every caller-configurable threshold this package uses,
// following labor/closequality/journaldiagnostics's single-monolithic-
// struct convention. Nothing that materially changes a Flag or a
// reconciliation determination is a hidden code constant. Stock
// minimum/maximum/target/reorder values are never invented here — task
// section 59: those come only from caller-supplied StockPolicy per item.
type Policy struct {
	// SlowMovingDays triggers slow-moving classification when
	// DaysSinceOutbound (or DaysSinceAnyMovement if no outbound evidence
	// exists) is at least this many days — task section 19. This package
	// never invents a default obsolescence period; zero means "not
	// configured," and slow/non-moving classification is simply
	// unavailable until the caller supplies both thresholds.
	SlowMovingDays int `json:"slow_moving_days,omitempty"`
	// NonMovingDays triggers non-moving classification; must be >=
	// SlowMovingDays when both are set (see validatePolicy) — task
	// section 20.
	NonMovingDays int `json:"non_moving_days,omitempty"`

	// ExpiryWarningDays triggers FlagExpiringInventory when a lot's
	// ExpiryDate is within this many days of AsOfDate (and not yet
	// expired) — task section 48. Zero means expiry-warning review is
	// disabled (expired-lot review, an unambiguous fact once ExpiryDate
	// is known, remains available regardless).
	ExpiryWarningDays int `json:"expiry_warning_days,omitempty"`

	// Materiality is the absolute-dollar threshold used by
	// MaterialityAssessment (issues.go/materiality-style flags) — combined
	// with MaterialityPercent via OR semantics (either alone is
	// sufficient), matching accounting/ap's identical rule (task section
	// 56: "use repository-consistent OR semantics").
	Materiality        float64 `json:"materiality,omitempty"`
	MaterialityPercent float64 `json:"materiality_percent,omitempty"`

	// NegativeInventoryHandling — see its own doc comment. Zero value
	// resolves to NegativeInventoryAllowWithWarning.
	NegativeInventoryHandling NegativeInventoryHandling `json:"negative_inventory_handling,omitempty"`

	// AdjustmentRateThreshold triggers FlagHighInventoryAdjustmentRate
	// when AbsoluteAdjustmentValue / AverageInventory exceeds this
	// fraction for a period. Default 0.02 (2%).
	AdjustmentRateThreshold float64 `json:"adjustment_rate_threshold,omitempty"`
	// LargeWriteOffThreshold triggers FlagLargeWriteOff for a single
	// write-off movement (or period write-off total) exceeding this
	// absolute dollar amount. Default resolves from Materiality if unset
	// — see resolvePolicy.
	LargeWriteOffThreshold float64 `json:"large_write_off_threshold,omitempty"`
	// RepeatedItemAdjustmentCount triggers FlagRepeatedItemAdjustments
	// for an item with at least this many adjustment/write-off movements
	// within the analyzed window. Default 3.
	RepeatedItemAdjustmentCount int `json:"repeated_item_adjustment_count,omitempty"`
	// PeriodEndAdjustmentWindowDays flags an adjustment dated within this
	// many days of a period's EndDate as a period-end adjustment
	// (FlagPeriodEndInventoryAdjustment) — a neutral timing observation,
	// not a fraud inference. Default 3.
	PeriodEndAdjustmentWindowDays int `json:"period_end_adjustment_window_days,omitempty"`

	// PurchaseVsUsageThreshold triggers FlagPurchasesOutpaceUsage when
	// period PurchaseValue exceeds period OutboundUsageValue by more than
	// this fraction of usage (e.g. 0.20 means purchases running 20%+
	// ahead of usage value). Default 0.20.
	PurchaseVsUsageThreshold float64 `json:"purchase_vs_usage_threshold,omitempty"`
	// InventoryBuildGrowthGap triggers
	// FlagInventoryBuildWithoutMatchingCOGSGrowth when
	// (InventoryGrowthPercent - COGSGrowthPercent) exceeds this fraction,
	// first-vs-last across supplied PeriodFinancials. Default 0.15.
	InventoryBuildGrowthGap float64 `json:"inventory_build_growth_gap,omitempty"`

	// ReconciliationTolerance is the tolerance applied to subledger-vs-GL
	// comparisons (reconcile.go).
	ReconciliationTolerance ReconciliationTolerance `json:"reconciliation_tolerance"`
	// RollforwardTolerance is the tolerance applied to quantity/value
	// rollforward comparisons (reconcile.go). Quantity rollforward uses
	// this as an absolute-units tolerance (PercentTolerance is ignored
	// for quantity, since "percent of a quantity" mixes incompatible
	// units across items).
	RollforwardTolerance ReconciliationTolerance `json:"rollforward_tolerance"`

	// TopN is the top-N cutoff for concentration ranking. Default 10.
	TopN int `json:"top_n,omitempty"`

	// ReportingCurrency, when set, resolves the single currency an
	// analysis proceeds under; otherwise the most common currency among
	// included rows is used — mirrors ar/ap's identical resolution rule.
	ReportingCurrency string `json:"reporting_currency,omitempty"`

	// UOMConversions is the explicit, optional set of unit-of-measure
	// conversion factors — see UOMConversion. Empty means no cross-UOM
	// aggregation is ever attempted (safe default).
	UOMConversions []UOMConversion `json:"uom_conversions,omitempty"`

	// Buckets is the caller-defined value-aging bucket schema — task
	// section 18: "support caller-defined buckets." Empty resolves to
	// DefaultBuckets().
	Buckets []BucketDefinition `json:"buckets,omitempty"`
}

const (
	defaultAdjustmentRateThreshold       = 0.02
	defaultRepeatedItemAdjustmentCount   = 3
	defaultPeriodEndAdjustmentWindowDays = 3
	defaultPurchaseVsUsageThreshold      = 0.20
	defaultInventoryBuildGrowthGap       = 0.15
	defaultTopN                          = 10
)

// DefaultPolicy returns this package's baseline, broadly-safe Policy.
// SlowMovingDays/NonMovingDays/ExpiryWarningDays are deliberately left at
// zero (disabled) — task sections 19/48: this package never invents a
// default obsolescence period or shelf-life warning window; a caller must
// supply these explicitly to opt into that analysis.
func DefaultPolicy() Policy {
	return Policy{
		AdjustmentRateThreshold:       defaultAdjustmentRateThreshold,
		RepeatedItemAdjustmentCount:   defaultRepeatedItemAdjustmentCount,
		PeriodEndAdjustmentWindowDays: defaultPeriodEndAdjustmentWindowDays,
		PurchaseVsUsageThreshold:      defaultPurchaseVsUsageThreshold,
		InventoryBuildGrowthGap:       defaultInventoryBuildGrowthGap,
		TopN:                          defaultTopN,
	}
}

// resolvePolicy merges p over DefaultPolicy field by field: a zero-valued
// field takes the default. Fields with a legitimate zero default
// (SlowMovingDays, NonMovingDays, ExpiryWarningDays, Materiality,
// MaterialityPercent, ReconciliationTolerance, RollforwardTolerance,
// LargeWriteOffThreshold before materiality fallback, ReportingCurrency,
// UOMConversions) are never defaulted away from zero.
func resolvePolicy(p Policy) Policy {
	d := DefaultPolicy()
	if p.AdjustmentRateThreshold == 0 {
		p.AdjustmentRateThreshold = d.AdjustmentRateThreshold
	}
	if p.RepeatedItemAdjustmentCount == 0 {
		p.RepeatedItemAdjustmentCount = d.RepeatedItemAdjustmentCount
	}
	if p.PeriodEndAdjustmentWindowDays == 0 {
		p.PeriodEndAdjustmentWindowDays = d.PeriodEndAdjustmentWindowDays
	}
	if p.PurchaseVsUsageThreshold == 0 {
		p.PurchaseVsUsageThreshold = d.PurchaseVsUsageThreshold
	}
	if p.InventoryBuildGrowthGap == 0 {
		p.InventoryBuildGrowthGap = d.InventoryBuildGrowthGap
	}
	if p.TopN == 0 {
		p.TopN = d.TopN
	}
	if p.LargeWriteOffThreshold == 0 {
		p.LargeWriteOffThreshold = resolveMaterialityAbsolute(p.Materiality, p.MaterialityPercent, 0)
	}
	p.NegativeInventoryHandling = resolvedNegativeInventoryHandling(p.NegativeInventoryHandling)
	return p
}

// validatePolicy checks Policy for structurally invalid configuration.
func validatePolicy(p Policy) []Issue {
	var issues []Issue
	checkFinite := func(v float64, label string) {
		if isNonFinite(v) {
			issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "policy threshold " + label + " is non-finite"})
		}
	}
	checkFinite(p.Materiality, "materiality")
	checkFinite(p.MaterialityPercent, "materiality_percent")
	checkFinite(p.AdjustmentRateThreshold, "adjustment_rate_threshold")
	checkFinite(p.LargeWriteOffThreshold, "large_write_off_threshold")
	checkFinite(p.PurchaseVsUsageThreshold, "purchase_vs_usage_threshold")
	checkFinite(p.InventoryBuildGrowthGap, "inventory_build_growth_gap")
	checkFinite(p.ReconciliationTolerance.AbsoluteTolerance, "reconciliation_tolerance.absolute_tolerance")
	checkFinite(p.ReconciliationTolerance.PercentTolerance, "reconciliation_tolerance.percent_tolerance")
	checkFinite(p.RollforwardTolerance.AbsoluteTolerance, "rollforward_tolerance.absolute_tolerance")
	checkFinite(p.RollforwardTolerance.PercentTolerance, "rollforward_tolerance.percent_tolerance")

	if p.SlowMovingDays < 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "policy slow_moving_days must not be negative"})
	}
	if p.NonMovingDays < 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "policy non_moving_days must not be negative"})
	}
	if p.SlowMovingDays > 0 && p.NonMovingDays > 0 && p.NonMovingDays < p.SlowMovingDays {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "policy non_moving_days must be >= slow_moving_days"})
	}
	if p.ExpiryWarningDays < 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "policy expiry_warning_days must not be negative"})
	}
	if p.RepeatedItemAdjustmentCount < 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "policy repeated_item_adjustment_count must not be negative"})
	}
	if p.PeriodEndAdjustmentWindowDays < 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "policy period_end_adjustment_window_days must not be negative"})
	}
	if p.TopN < 0 {
		issues = append(issues, Issue{Code: IssueInvalidPolicy, Severity: SeverityError, Message: "policy top_n must not be negative"})
	}
	for _, c := range p.UOMConversions {
		if c.From == "" || c.To == "" {
			issues = append(issues, Issue{Code: IssueInvalidUOM, Severity: SeverityError, Message: "uom conversion missing From/To"})
		}
		if isNonFinite(c.Factor) || c.Factor <= 0 {
			issues = append(issues, Issue{Code: IssueInvalidUOM, Severity: SeverityError, Message: "uom conversion factor must be a positive finite number"})
		}
	}
	return issues
}

// resolveMaterialityAbsolute converts explicit materiality inputs into a
// single absolute-dollar figure: the explicit threshold if supplied,
// otherwise percentOfTotal * totalBase. Mirrors accounting/ap's identical
// helper.
func resolveMaterialityAbsolute(explicitThreshold, percentOfTotal, totalBase float64) float64 {
	if explicitThreshold > 0 {
		return explicitThreshold
	}
	return percentOfTotal * totalBase
}
