package synthetic

import "github.com/themurtez/go-valuate/analytics/revenuequality"

// BuildRevenueQualityInput returns analytics/revenuequality.Input for
// Meridian SaaS's full 5-year history plus customer-level detail (see
// BuildCustomerRevenue), using the package's own DefaultPolicy() (left at
// the zero value).
func BuildRevenueQualityInput() revenuequality.Input {
	return revenuequality.Input{
		Dataset:         BuildDataset(),
		PeriodMeta:      RevenueQualityPeriodMeta(),
		CustomerRevenue: BuildCustomerRevenue(),
	}
}
