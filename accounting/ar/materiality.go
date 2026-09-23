package ar

// MaterialityAssessment reports whether a dollar amount is material under
// Options' resolved absolute-threshold and percent-of-total-AR rules —
// section 22. This package imposes no accounting materiality standard
// (e.g. no fixed "5% of net income" rule); it is a simple, caller-
// configured deterministic comparison, used to distinguish a minor
// overdue item from a material one.
type MaterialityAssessment struct {
	Amount            float64 `json:"amount"`
	AbsoluteThreshold float64 `json:"absolute_threshold"`
	PercentThreshold  float64 `json:"percent_threshold"`
	// PercentOfTotalAR is Amount / total open AR. Unavailable if total
	// open AR is zero.
	PercentOfTotalAR AmountValue `json:"percent_of_total_ar"`
	// Material is true if Amount exceeds AbsoluteThreshold (when > 0) OR
	// PercentOfTotalAR exceeds PercentThreshold (when available) — either
	// rule alone is sufficient to be material.
	Material bool `json:"material"`
}

// assessMateriality evaluates amount against the resolved absolute and
// percent-of-AR thresholds.
func assessMateriality(amount, absoluteThreshold, percentThreshold, totalOpenAR float64) MaterialityAssessment {
	m := MaterialityAssessment{Amount: amount, AbsoluteThreshold: absoluteThreshold, PercentThreshold: percentThreshold}
	if totalOpenAR != 0 {
		m.PercentOfTotalAR = AvailableAmount(amount / totalOpenAR)
	}
	if absoluteThreshold > 0 && amount > absoluteThreshold {
		m.Material = true
	}
	if m.PercentOfTotalAR.Available && percentThreshold > 0 && m.PercentOfTotalAR.Value > percentThreshold {
		m.Material = true
	}
	return m
}

// resolveMaterialityAbsolute converts Options' materiality inputs into a
// single absolute-dollar figure used by resolveThresholds' balance-flag
// defaults: the explicit MaterialityThreshold if supplied, otherwise
// MaterialityPercentOfAR * totalOpenAR.
func resolveMaterialityAbsolute(explicitThreshold, percentOfAR, totalOpenAR float64) float64 {
	if explicitThreshold > 0 {
		return explicitThreshold
	}
	return resolvedMaterialityPercent(percentOfAR) * totalOpenAR
}
