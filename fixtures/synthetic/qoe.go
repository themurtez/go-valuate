package synthetic

import "github.com/themurtez/go-valuate/analytics/qoe"

// BuildQoEInput returns analytics/qoe.Input for Meridian SaaS's full
// 5-year history plus its one confirmed adjustment (the 2023 rebranding
// one-time expense — see BuildConfirmedAdjustments).
func BuildQoEInput() qoe.Input {
	return qoe.Input{
		Dataset:     BuildDataset(),
		PeriodMeta:  BuildPeriodMeta(),
		Adjustments: BuildConfirmedAdjustments(),
	}
}
