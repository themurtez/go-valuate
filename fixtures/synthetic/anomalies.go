package synthetic

import "github.com/themurtez/go-valuate/analytics/anomalies"

// BuildAnomaliesInput returns analytics/anomalies.Input for Meridian
// SaaS's full 5-year history. The 2023 rebranding-cost spike in
// OPEX_OTHER (see meridianYears' doc comment) is deliberately built in so
// this package's expense-spike/margin-deterioration rules have a real
// signal to detect.
func BuildAnomaliesInput() anomalies.Input {
	return anomalies.Input{
		Dataset:    BuildDataset(),
		PeriodMeta: AnomaliesPeriodMeta(),
	}
}
