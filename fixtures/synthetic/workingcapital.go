package synthetic

import "github.com/themurtez/go-valuate/analytics/workingcapital"

// BuildWorkingCapitalInput returns analytics/workingcapital.Input for
// Meridian SaaS's full 5-year history, using the package's own
// DefaultInclusionPolicy() (left at the zero value) and AsOf set to the
// most recent period for PegComparison.
func BuildWorkingCapitalInput() workingcapital.Input {
	return workingcapital.Input{
		Dataset:    BuildDataset(),
		PeriodMeta: WorkingCapitalPeriodMeta(),
		AsOf:       "2025",
	}
}
