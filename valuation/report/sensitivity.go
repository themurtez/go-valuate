package report

// SensitivityData is the report's structured sensitivity section:
// rows/matrices only, reshaped from valuation/sensitivity output (see
// build.go) into this package's plain, presentation-neutral form. No
// chart is rendered here — a future UI/export decides how (or whether) to
// visualize these rows.
type SensitivityData struct {
	// MultipleSensitivity lists one row per analyzed multiple (see
	// valuation/sensitivity.MultipleSensitivity), when supplied.
	MultipleSensitivity []SensitivityRow `json:"multiple_sensitivity,omitempty"`
	// EarningsMultipleMatrix is the flattened earnings-scenario x multiple
	// grid (see valuation/sensitivity.Matrix), when supplied — one row per
	// (scenario, multiple) cell rather than a nested structure, so this
	// section stays uniform with MultipleSensitivity/DCFSensitivity and
	// serializes as a single flat table a CSV export can emit directly.
	EarningsMultipleMatrix []MatrixRow `json:"earnings_multiple_matrix,omitempty"`
	// DCFSensitivity is the flattened discount-rate x terminal-growth-rate
	// grid (see valuation/sensitivity.DCFGrid), when supplied.
	DCFSensitivity []DCFSensitivityRow `json:"dcf_sensitivity,omitempty"`
}

// SensitivityRow is one multiple's result in a flattened multiple
// sensitivity table.
type SensitivityRow struct {
	Multiple float64 `json:"multiple"`
	Value    float64 `json:"value"`
	Valid    bool    `json:"valid"`
	Reason   string  `json:"reason,omitempty"`
}

// MatrixRow is one (earnings scenario, multiple) cell in a flattened
// earnings x multiple matrix table.
type MatrixRow struct {
	EarningsLabel string  `json:"earnings_label"`
	Earnings      float64 `json:"earnings"`
	Multiple      float64 `json:"multiple"`
	Value         float64 `json:"value"`
	Valid         bool    `json:"valid"`
	Reason        string  `json:"reason,omitempty"`
}

// DCFSensitivityRow is one (discount rate, terminal growth rate) cell in a
// flattened DCF sensitivity table.
type DCFSensitivityRow struct {
	DiscountRate       float64 `json:"discount_rate"`
	TerminalGrowthRate float64 `json:"terminal_growth_rate"`
	Valid              bool    `json:"valid"`
	EnterpriseValue    float64 `json:"enterprise_value"`
}
