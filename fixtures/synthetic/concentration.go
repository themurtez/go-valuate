package synthetic

import "github.com/themurtez/go-valuate/analytics/concentration"

// BuildConcentrationInput returns analytics/concentration.Input for
// Meridian SaaS's customer-revenue concentration (see
// BuildConcentrationObservations), using the package's own DefaultPolicy()
// (left at the zero value) for TopN/ScenarioTopN cutoffs.
func BuildConcentrationInput() concentration.Input {
	return concentration.Input{
		Basis:        concentration.BasisCustomerRevenue,
		Observations: BuildConcentrationObservations(),
		PeriodMeta:   ConcentrationPeriodMeta(),
	}
}
