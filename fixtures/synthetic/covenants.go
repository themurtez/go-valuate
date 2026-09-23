package synthetic

import "github.com/themurtez/go-valuate/analytics/covenants"

// BuildCovenantTests returns analytics/covenants.Input for Meridian SaaS's
// 2025 covenant package under its term loan (see BuildDebtInput). DSCR's
// Actual (14.77x) is the same figure BuildDebtInput's loan terms actually
// produce — computed independently here (not by calling analytics/debt),
// so a suite-level test can assert the two agree, per this task's
// cross-module invariant requirement. The other two tests use realistic,
// independently-set Actual figures rather than deriving from a sibling
// module, since current-ratio/minimum-EBITDA covenants are conventionally
// lender-specific figures a caller supplies directly.
func BuildCovenantTests() covenants.Input {
	return covenants.Input{
		Tests: []covenants.CovenantTest{
			{
				CovenantID:           "MIN_DSCR",
				Label:                "Minimum Debt Service Coverage Ratio",
				Metric:               covenants.MetricDSCR,
				Operator:             covenants.OperatorGTE,
				Threshold:            1.25,
				Actual:               covenants.AvailableValue(14.77),
				Period:               "2025",
				WarningBufferPercent: 0.10,
			},
			{
				CovenantID:           "MIN_CURRENT_RATIO",
				Label:                "Minimum Current Ratio",
				Metric:               covenants.MetricCurrentRatio,
				Operator:             covenants.OperatorGTE,
				Threshold:            1.20,
				Actual:               covenants.AvailableValue(17.04),
				Period:               "2025",
				WarningBufferPercent: 0.10,
			},
			{
				CovenantID:           "MAX_CAPEX",
				Label:                "Maximum Annual Capital Expenditures",
				Metric:               covenants.MetricMaximumCapex,
				Operator:             covenants.OperatorLTE,
				Threshold:            750_000,
				Actual:               covenants.AvailableValue(690_000),
				Period:               "2025",
				WarningBufferPercent: 0.10, // within 10% of the ceiling: a near-breach
			},
		},
	}
}
