package ap

// MaterialityAssessment reports whether a dollar amount is material under
// Options' resolved absolute-threshold and percent-of-total-AP rules. This
// package imposes no accounting materiality standard (e.g. no fixed "5% of
// net income" rule); it is a simple, caller-configured deterministic
// comparison, used to distinguish a minor overdue item from a material one.
type MaterialityAssessment struct {
	Amount            float64 `json:"amount"`
	AbsoluteThreshold float64 `json:"absolute_threshold"`
	PercentThreshold  float64 `json:"percent_threshold"`
	// PercentOfTotalAP is Amount / total open AP. Unavailable if total
	// open AP is zero.
	PercentOfTotalAP AmountValue `json:"percent_of_total_ap"`
	// Material is true if Amount exceeds AbsoluteThreshold (when > 0) OR
	// PercentOfTotalAP exceeds PercentThreshold (when available) — either
	// rule alone is sufficient to be material.
	Material bool `json:"material"`
}

// assessMateriality evaluates amount against the resolved absolute and
// percent-of-AP thresholds.
func assessMateriality(amount, absoluteThreshold, percentThreshold, totalOpenAP float64) MaterialityAssessment {
	m := MaterialityAssessment{Amount: amount, AbsoluteThreshold: absoluteThreshold, PercentThreshold: percentThreshold}
	if totalOpenAP != 0 {
		m.PercentOfTotalAP = AvailableAmount(amount / totalOpenAP)
	}
	if absoluteThreshold > 0 && amount > absoluteThreshold {
		m.Material = true
	}
	if m.PercentOfTotalAP.Available && percentThreshold > 0 && m.PercentOfTotalAP.Value > percentThreshold {
		m.Material = true
	}
	return m
}

// resolveMaterialityAbsolute converts Options' materiality inputs into a
// single absolute-dollar figure used by resolveThresholds' balance-flag
// defaults: the explicit MaterialityThreshold if supplied, otherwise
// MaterialityPercentOfAP * totalOpenAP.
func resolveMaterialityAbsolute(explicitThreshold, percentOfAP, totalOpenAP float64) float64 {
	if explicitThreshold > 0 {
		return explicitThreshold
	}
	return resolvedMaterialityPercent(percentOfAP) * totalOpenAP
}
