package inventory

import (
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/financial"
)

// ConcentrationSummary bundles inventory VALUE concentration by item,
// category, and location — task section 28. Reuses analytics/concentration
// directly for share/HHI/top-N math, the same way accounting/ar and
// accounting/ap already reuse it, since "concentration among entities by
// dollar amount" is identical arithmetic regardless of what the entity
// represents.
//
// Important semantic rule (task section 28): this is inventory VALUE
// concentration (capital tied up in a small number of items/categories/
// locations), never supplier or customer dependency — Label says so
// explicitly, mirroring ap.ConcentrationSummary's identical disclaimer
// pattern for AP-balance-vs-vendor-dependency.
type ConcentrationSummary struct {
	ByItem     concentration.Result `json:"by_item"`
	ByCategory concentration.Result `json:"by_category"`
	ByLocation concentration.Result `json:"by_location"`

	Label string `json:"label"`
}

const inventoryConcentrationLabel = "inventory value concentration by item/category/location; not a measure of supplier or customer dependency"

// inventoryConcentrationPeriod is the synthetic financial.Period this
// package uses when calling analytics/concentration.Calculate, since
// inventory value concentration is a point-in-time (AsOfDate) analysis
// rather than a multi-period series — mirrors ap.apPeriod's identical
// adapter pattern.
const inventoryConcentrationPeriod financial.Period = "AS_OF"

// buildValueConcentration adapts entityKey -> value pairs into
// analytics/concentration.Observation rows and reuses that package's
// share/HHI/top-N calculation.
func buildValueConcentration(byEntity map[string]float64, policy concentration.Policy) concentration.Result {
	if len(byEntity) == 0 {
		return concentration.Result{}
	}
	keys := make([]string, 0, len(byEntity))
	for k := range byEntity {
		keys = append(keys, k)
	}
	sortStrings(keys)

	obs := make([]concentration.Observation, 0, len(keys))
	for _, k := range keys {
		v := byEntity[k]
		if v <= 0 {
			continue
		}
		obs = append(obs, concentration.Observation{EntityKey: k, Period: inventoryConcentrationPeriod, Amount: v})
	}
	if len(obs) == 0 {
		return concentration.Result{}
	}
	in := concentration.Input{
		Basis:        concentration.BasisOther,
		Observations: obs,
		PeriodMeta: map[financial.Period]concentration.PeriodInfo{
			inventoryConcentrationPeriod: {Type: concentration.PeriodTypeFiscalYear, FiscalYear: 0},
		},
		Policy: policy,
	}
	return concentration.Calculate(in, concentration.Options{})
}

func buildConcentrationSummary(order []string, states map[string]*itemState, policy concentration.Policy) ConcentrationSummary {
	byItem := map[string]float64{}
	byCategory := map[string]float64{}
	byLocation := map[string]float64{}

	for _, id := range order {
		st := states[id]
		if st.value.Available {
			byItem[id] += st.value.Amount
			if st.item.Category != "" {
				byCategory[st.item.Category] += st.value.Amount
			}
		}
		// Location totals are built purely from each contributing
		// snapshot's own Location — an item's on-hand value can be split
		// across multiple locations at once (one InventorySnapshot per
		// (location, lot)), so summing st.value.Amount (the item's
		// cross-location total) into a single location bucket here would
		// double-count whenever more than one location is involved.
		// Item.Location (a display/default field) is never used for this
		// grouping.
		for _, snap := range st.latestByLocationLot {
			if snap.Location == "" {
				continue
			}
			v, _ := resolveSnapshotValue(snap)
			if v.Available {
				byLocation[snap.Location] += v.Amount
			}
		}
	}

	return ConcentrationSummary{
		ByItem:     buildValueConcentration(byItem, policy),
		ByCategory: buildValueConcentration(byCategory, policy),
		ByLocation: buildValueConcentration(byLocation, policy),
		Label:      inventoryConcentrationLabel,
	}
}
