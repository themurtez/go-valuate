package synthetic

import "github.com/themurtez/go-valuate/analytics/ratios"

// BuildRatiosInput returns analytics/ratios.Input for Meridian SaaS's full
// 5-year history.
func BuildRatiosInput() ratios.Input {
	return ratios.Input{
		Dataset:    BuildDataset(),
		PeriodMeta: RatiosPeriodMeta(),
	}
}
