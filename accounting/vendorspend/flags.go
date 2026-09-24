package vendorspend

import "sort"

// FlagCode is a stable identifier for one deterministic vendor-spend
// signal. This package never derives a hidden supplier-risk or
// fraud-likelihood model; every flag is a simple, documented threshold
// comparison the caller can fully see and override via Thresholds — the
// same convention every analytics sibling package's FlagCode uses. Only
// codes this package actually emits are defined.
type FlagCode string

const (
	FlagHighSupplierConcentration       FlagCode = "HIGH_SUPPLIER_CONCENTRATION"
	FlagSupplierConcentrationIncreasing FlagCode = "SUPPLIER_CONCENTRATION_INCREASING"
	FlagNewSupplierActivity             FlagCode = "NEW_SUPPLIER_ACTIVITY"
	FlagSupplierSpendDiscontinued       FlagCode = "SUPPLIER_SPEND_DISCONTINUED"
	FlagUnitPriceIncrease               FlagCode = "UNIT_PRICE_INCREASE"
	FlagObservedSingleSourceProduct     FlagCode = "OBSERVED_SINGLE_SOURCE_PRODUCT"
	FlagObservedRepeatedSpend           FlagCode = "OBSERVED_REPEATED_SPEND"
	FlagPossibleDuplicateSpend          FlagCode = "POSSIBLE_DUPLICATE_SPEND"
	FlagMaterialUncategorizedSpend      FlagCode = "MATERIAL_UNCATEGORIZED_SPEND"
	FlagLowProductAttributionCoverage   FlagCode = "LOW_PRODUCT_ATTRIBUTION_COVERAGE"
	FlagSpendWithNonPreferredSupplier   FlagCode = "SPEND_WITH_NON_PREFERRED_SUPPLIER"
	FlagSpendOutsideContractedSuppliers FlagCode = "SPEND_OUTSIDE_CONTRACTED_SUPPLIERS"
	FlagHighTailSpend                   FlagCode = "HIGH_TAIL_SPEND"
	FlagControlTotalMismatch            FlagCode = "CONTROL_TOTAL_MISMATCH"
)

// flagCodeOrder fixes FlagCode declaration order for deterministic Flags
// sorting — task section 41's "flags severity/code/supplier/product"
// rule.
var flagCodeOrder = []FlagCode{
	FlagHighSupplierConcentration,
	FlagSupplierConcentrationIncreasing,
	FlagNewSupplierActivity,
	FlagSupplierSpendDiscontinued,
	FlagUnitPriceIncrease,
	FlagObservedSingleSourceProduct,
	FlagObservedRepeatedSpend,
	FlagPossibleDuplicateSpend,
	FlagMaterialUncategorizedSpend,
	FlagLowProductAttributionCoverage,
	FlagSpendWithNonPreferredSupplier,
	FlagSpendOutsideContractedSuppliers,
	FlagHighTailSpend,
	FlagControlTotalMismatch,
}

func flagRank(c FlagCode) int {
	for i, fc := range flagCodeOrder {
		if fc == c {
			return i
		}
	}
	return len(flagCodeOrder)
}

// FlagSeverity mirrors every analytics sibling package's identical role: a
// structured, matchable urgency signal, never inferred from Message text.
type FlagSeverity string

const (
	FlagSeverityInfo     FlagSeverity = "info"
	FlagSeverityWarning  FlagSeverity = "warning"
	FlagSeverityCritical FlagSeverity = "critical"
)

// Flag is one deterministic, explainable vendor-spend signal — always
// rule-based against Thresholds, never AI-scored, and always phrased in
// neutral, factual language (see neutral_language_test.go's permanent
// regression coverage: no "overcharging," "bad vendor," "replace
// vendor," "negotiate harder," "risky," or "fraud" wording anywhere in
// this package).
type Flag struct {
	Code       FlagCode     `json:"code"`
	Severity   FlagSeverity `json:"severity"`
	Message    string       `json:"message"`
	SupplierID string       `json:"supplier_id,omitempty"`
	ProductID  string       `json:"product_id,omitempty"`
	Period     string       `json:"period,omitempty"`
	Value      float64      `json:"value,omitempty"`
	Threshold  float64      `json:"threshold,omitempty"`
}

// Thresholds configures every flag's trigger point — task section 32/37.
// All caller-adjustable; the zero value resolves to DefaultThresholds.
// Kept separate from Policy (flags.go vs. policy.go): Policy changes what
// is reported, Thresholds changes only whether an already-computed
// figure crosses a caller-adjustable line into a Flag — mirrors
// analytics/concentration's identical Policy vs. Thresholds separation.
//
// Two flags are deliberate exceptions to that split:
// FlagMaterialUncategorizedSpend and FlagLowProductAttributionCoverage
// trigger directly off Policy.CategoryCoverageThreshold/
// ProductCoverageThreshold (see flagInputs' categoryCoverageThreshold/
// productCoverageThreshold fields and computeFlags), not a Thresholds
// field — "how much attribution coverage counts as adequate" is
// inseparable from the coverage measure itself, not an independent
// trigger tunable on top of it.
type Thresholds struct {
	// HighSupplierConcentrationShare triggers
	// FlagHighSupplierConcentration when SpendConcentration.Top1 is at or
	// above this fraction. Default 0.25.
	HighSupplierConcentrationShare float64 `json:"high_supplier_concentration_share,omitempty"`
	// HighTailSpendPercent triggers FlagHighTailSpend when
	// TailSpend.TailSpendPercent is at or above this fraction. Default
	// 0.2 (20%).
	HighTailSpendPercent float64 `json:"high_tail_spend_percent,omitempty"`
}

// DefaultThresholds returns this package's baseline flag trigger points.
func DefaultThresholds() Thresholds {
	return Thresholds{
		HighSupplierConcentrationShare: 0.25,
		HighTailSpendPercent:           0.2,
	}
}

// resolveThresholds merges t over DefaultThresholds field by field (a
// zero-valued field takes the default).
func resolveThresholds(t Thresholds) Thresholds {
	d := DefaultThresholds()
	if t.HighSupplierConcentrationShare == 0 {
		t.HighSupplierConcentrationShare = d.HighSupplierConcentrationShare
	}
	if t.HighTailSpendPercent == 0 {
		t.HighTailSpendPercent = d.HighTailSpendPercent
	}
	return t
}

// flagInputs bundles everything computeFlags needs, kept as one struct so
// the function signature stays readable — mirrors accounting/ap's
// identical flagInputs convention.
type flagInputs struct {
	concentration               SpendConcentration
	concentrationHistory        []SpendConcentration // chronological, for the increasing-concentration trend check
	supplierShareIncreasePoints float64
	newLost                     NewLostSuppliers
	newMaterial                 float64
	lostMaterial                float64
	unitPricePoints             []UnitPricePoint
	unitPriceIncreasePercent    float64
	productSpend                []ProductSpend
	observedRecurring           []ObservedRecurringGroup
	duplicateGroups             []DuplicateLikeGroup
	categoryProductCoverage     CategoryProductCoverage
	categoryCoverageThreshold   float64
	productCoverageThreshold    float64
	nonPreferred                NonPreferredSpend
	outsideContracted           OutsideContractedSpend
	tailSpend                   TailSpend
	controlReconciliation       ControlReconciliation
	thresholds                  Thresholds
}

// computeFlags evaluates every FlagCode rule and returns triggered flags
// sorted by FlagCode declaration order, then SupplierID, then ProductID.
func computeFlags(in flagInputs) []Flag {
	var flags []Flag
	t := in.thresholds

	if in.concentration.Available && in.concentration.Top1.Available && in.concentration.Top1.Value >= t.HighSupplierConcentrationShare {
		flags = append(flags, Flag{Code: FlagHighSupplierConcentration, Severity: FlagSeverityWarning,
			Value: in.concentration.Top1.Value, Threshold: t.HighSupplierConcentrationShare,
			Message: "largest supplier's share of total net spend is at or above threshold"})
	}

	if len(in.concentrationHistory) >= 2 {
		first, last := in.concentrationHistory[0], in.concentrationHistory[len(in.concentrationHistory)-1]
		if first.Top1.Available && last.Top1.Available {
			change := last.Top1.Value - first.Top1.Value
			if change >= in.supplierShareIncreasePoints {
				flags = append(flags, Flag{Code: FlagSupplierConcentrationIncreasing, Severity: FlagSeverityInfo,
					Value: change, Threshold: in.supplierShareIncreasePoints,
					Message: "largest supplier's spend share increased from the first to the most recent period with data by at least the configured number of points"})
			}
		}
	}

	for _, c := range in.newLost.Changes {
		if !c.Material {
			continue
		}
		switch c.Kind {
		case ChangeNewSupplierActivity:
			flags = append(flags, Flag{Code: FlagNewSupplierActivity, Severity: FlagSeverityInfo,
				SupplierID: c.SupplierID, Period: c.ToPeriod, Value: c.Amount, Threshold: in.newMaterial,
				Message: "supplier shows new material spend activity versus the prior period"})
		case ChangeSupplierSpendDiscontinued:
			flags = append(flags, Flag{Code: FlagSupplierSpendDiscontinued, Severity: FlagSeverityInfo,
				SupplierID: c.SupplierID, Period: c.FromPeriod, Value: c.Amount, Threshold: in.lostMaterial,
				Message: "supplier's material spend activity stopped versus the prior period"})
		}
	}

	for _, p := range in.unitPricePoints {
		if !p.UnitPriceChangePercent.Available {
			continue
		}
		if p.UnitPriceChangePercent.Value >= in.unitPriceIncreasePercent {
			flags = append(flags, Flag{Code: FlagUnitPriceIncrease, Severity: FlagSeverityWarning,
				SupplierID: p.SupplierID, ProductID: p.ProductID, Value: p.UnitPriceChangePercent.Value, Threshold: in.unitPriceIncreasePercent,
				Message: "unit price increased from first to last observed record at or above threshold percent"})
		}
	}

	for _, p := range in.productSpend {
		if p.ObservedSingleSource {
			flags = append(flags, Flag{Code: FlagObservedSingleSourceProduct, Severity: FlagSeverityInfo,
				ProductID: p.ProductID, Value: p.Spend,
				Message: "only one supplier observed in the supplied data for this product"})
		}
	}

	for _, g := range in.observedRecurring {
		flags = append(flags, Flag{Code: FlagObservedRepeatedSpend, Severity: FlagSeverityInfo,
			SupplierID: g.SupplierID, Value: g.AverageAmount,
			Message: "supplier/category shows a repeated comparable-amount spend pattern across multiple periods"})
	}

	for _, g := range in.duplicateGroups {
		flags = append(flags, Flag{Code: FlagPossibleDuplicateSpend, Severity: FlagSeverityWarning,
			SupplierID: g.SupplierID, Value: g.Amount,
			Message: "multiple spend records share a normalized signature within the duplicate-detection window"})
	}

	if in.categoryProductCoverage.CategoryCoveragePercent.Available &&
		in.categoryProductCoverage.CategoryCoveragePercent.Value < in.categoryCoverageThreshold {
		flags = append(flags, Flag{Code: FlagMaterialUncategorizedSpend, Severity: FlagSeverityWarning,
			Value: in.categoryProductCoverage.CategoryCoveragePercent.Value, Threshold: in.categoryCoverageThreshold,
			Message: "categorized share of total net spend is below threshold"})
	}

	if in.categoryProductCoverage.ProductCoveragePercent.Available &&
		in.categoryProductCoverage.ProductCoveragePercent.Value < in.productCoverageThreshold {
		flags = append(flags, Flag{Code: FlagLowProductAttributionCoverage, Severity: FlagSeverityInfo,
			Value: in.categoryProductCoverage.ProductCoveragePercent.Value, Threshold: in.productCoverageThreshold,
			Message: "product-attributed share of total net spend is below threshold"})
	}

	// Triggered on presence (len(Suppliers) > 0), not TotalSpend > 0: a
	// non-preferred/non-contracted supplier whose activity nets to zero
	// or negative (e.g. normal purchases largely offset by a credit) is
	// still real non-preferred/non-contracted activity that occurred —
	// TotalSpend > 0 would silently hide it purely because netting made
	// the signed total non-positive.
	if in.nonPreferred.Available && len(in.nonPreferred.Suppliers) > 0 {
		flags = append(flags, Flag{Code: FlagSpendWithNonPreferredSupplier, Severity: FlagSeverityInfo,
			Value: in.nonPreferred.TotalSpend, Message: "spend exists with suppliers not marked as preferred"})
	}
	if in.outsideContracted.Available && len(in.outsideContracted.Suppliers) > 0 {
		flags = append(flags, Flag{Code: FlagSpendOutsideContractedSuppliers, Severity: FlagSeverityInfo,
			Value: in.outsideContracted.TotalSpend, Message: "spend exists with suppliers not marked as contracted"})
	}

	if in.tailSpend.Available && in.tailSpend.TailSpendPercent.Available && in.tailSpend.TailSpendPercent.Value >= t.HighTailSpendPercent {
		flags = append(flags, Flag{Code: FlagHighTailSpend, Severity: FlagSeverityInfo,
			Value: in.tailSpend.TailSpendPercent.Value, Threshold: t.HighTailSpendPercent,
			Message: "tail spend share of total net spend is at or above threshold"})
	}

	for _, cr := range []ComponentReconciliation{
		in.controlReconciliation.Purchases, in.controlReconciliation.ExpenseSpend, in.controlReconciliation.CapexSpend,
		in.controlReconciliation.InventoryPurchases, in.controlReconciliation.ContractorSpend,
	} {
		if cr.Available && !cr.Reconciled {
			flags = append(flags, Flag{Code: FlagControlTotalMismatch, Severity: FlagSeverityWarning,
				Value: cr.Difference, Threshold: cr.Tolerance, Message: "vendor spend does not match a supplied control total within tolerance"})
		}
	}

	sort.SliceStable(flags, func(i, j int) bool {
		ri, rj := flagRank(flags[i].Code), flagRank(flags[j].Code)
		if ri != rj {
			return ri < rj
		}
		if flags[i].SupplierID != flags[j].SupplierID {
			return flags[i].SupplierID < flags[j].SupplierID
		}
		return flags[i].ProductID < flags[j].ProductID
	})
	return flags
}
