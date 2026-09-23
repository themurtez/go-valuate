package inventory

// MaterialityAssessment reports whether a dollar amount is material under
// Policy's resolved absolute-threshold and percent-of-total-inventory
// rules — task section 56. This is a review-attention threshold, never
// "audit materiality" in the professional-standards sense (task section
// 56: "do not call it audit materiality"). Mirrors accounting/ap's
// identical OR-semantics rule: either the absolute threshold OR the
// percent-of-total threshold is sufficient to be material.
type MaterialityAssessment struct {
	Amount            float64 `json:"amount"`
	AbsoluteThreshold float64 `json:"absolute_threshold"`
	PercentThreshold  float64 `json:"percent_threshold"`
	// PercentOfTotalInventory is Amount / total inventory value.
	// Unavailable if total inventory value is zero.
	PercentOfTotalInventory Value `json:"percent_of_total_inventory"`
	Material                bool  `json:"material"`
}

// assessMateriality evaluates amount against the resolved absolute and
// percent-of-inventory thresholds.
func assessMateriality(amount, absoluteThreshold, percentThreshold, totalInventoryValue float64) MaterialityAssessment {
	m := MaterialityAssessment{Amount: amount, AbsoluteThreshold: absoluteThreshold, PercentThreshold: percentThreshold}
	if totalInventoryValue != 0 {
		m.PercentOfTotalInventory = AvailableValue(amount / totalInventoryValue)
	}
	if absoluteThreshold > 0 && amount > absoluteThreshold {
		m.Material = true
	}
	if m.PercentOfTotalInventory.Available && percentThreshold > 0 && m.PercentOfTotalInventory.Amount > percentThreshold {
		m.Material = true
	}
	return m
}
