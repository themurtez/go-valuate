package synthetic

import (
	"github.com/themurtez/go-valuate/analytics/variance"
	"github.com/themurtez/go-valuate/financial"
)

// BuildVarianceInput returns analytics/variance.Input comparing Meridian
// SaaS's 2025 actual figures (from BuildDataset) against a 2025 budget set
// a few months before close: revenue slightly beat budget (favorable),
// opex ran slightly over (unfavorable) — a realistic, mixed result rather
// than every line beating or missing uniformly.
func BuildVarianceInput() variance.Input {
	return variance.Input{
		Lines: []variance.LineObservation{
			{
				AccountCode:  financial.CodeRevRecurring,
				Label:        "Recurring Revenue",
				Period:       "2025",
				Actual:       8_610_000,
				Baseline:     8_300_000,
				BaselineType: variance.BaselineTypeBudget,
			},
			{
				AccountCode:  financial.CodeOpexPayroll,
				Label:        "Payroll",
				Period:       "2025",
				Actual:       3_620_000,
				Baseline:     3_480_000,
				BaselineType: variance.BaselineTypeBudget,
			},
			{
				AccountCode:  financial.CodeOpexMarketing,
				Label:        "Marketing",
				Period:       "2025",
				Actual:       592_000,
				Baseline:     640_000,
				BaselineType: variance.BaselineTypeBudget,
			},
			{
				AccountCode:  financial.CodeCogsOther,
				Label:        "Hosting & COGS",
				Period:       "2025",
				Actual:       1_210_000,
				Baseline:     1_150_000,
				BaselineType: variance.BaselineTypeBudget,
			},
		},
		PeriodMeta: VariancePeriodMeta(),
	}
}
