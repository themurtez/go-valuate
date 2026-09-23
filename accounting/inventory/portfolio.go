package inventory

import "sort"

// CategoryValue/LocationValue are one category/location's aggregated
// on-hand value — task section 12.
type CategoryValue struct {
	Category string  `json:"category"`
	Value    float64 `json:"value"`
}

type LocationValue struct {
	Location string  `json:"location"`
	Value    float64 `json:"value"`
}

// TopItem is one item's rank in the top-N-by-value list — task section
// 12.
type TopItem struct {
	ItemID string  `json:"item_id"`
	Value  float64 `json:"value"`
	Rank   int     `json:"rank"`
}

// PortfolioSummary is the core as-of portfolio totals — task section 12.
// TotalUnits is deliberately absent as a single cross-item figure: task
// section 12 explicitly forbids a meaningless global unit total across
// incompatible units of measure (pieces + kilograms + liters). Per-item
// Qty already carries its own UnitOfMeasure; a caller wanting a
// compatible-UOM total groups items by UnitOfMeasure and sums those Qty
// values itself, or supplies UOMConversion rows so this package's
// itemstate-level aggregation can do it (see itemstate.go) — there is no
// portfolio-wide equivalent because a portfolio spans many items whose
// UOMs are not knowable to share compatibility without that same explicit
// conversion data.
type PortfolioSummary struct {
	TotalInventoryValue float64 `json:"total_inventory_value"`

	ActiveItemCount        int `json:"active_item_count"`
	ItemsWithPositiveStock int `json:"items_with_positive_stock"`
	ItemsWithNegativeStock int `json:"items_with_negative_stock"`
	ItemsWithZeroStock     int `json:"items_with_zero_stock"`

	InventoryByCategory []CategoryValue `json:"inventory_by_category,omitempty"`
	InventoryByLocation []LocationValue `json:"inventory_by_location,omitempty"`
	TopItemsByValue     []TopItem       `json:"top_items_by_value,omitempty"`
}

func buildPortfolioSummary(order []string, states map[string]*itemState, topN int) PortfolioSummary {
	p := PortfolioSummary{}
	byCategory := map[string]float64{}
	byLocation := map[string]float64{}
	var itemValues []TopItem

	for _, id := range order {
		st := states[id]
		if st.item.Active {
			p.ActiveItemCount++
		}
		if st.quantity.Available {
			switch {
			case st.quantity.Amount > 0:
				p.ItemsWithPositiveStock++
			case st.quantity.Amount < 0:
				p.ItemsWithNegativeStock++
			default:
				p.ItemsWithZeroStock++
			}
		}
		if !st.value.Available {
			continue
		}
		p.TotalInventoryValue += st.value.Amount
		if st.item.Category != "" {
			byCategory[st.item.Category] += st.value.Amount
		}
		itemValues = append(itemValues, TopItem{ItemID: id, Value: st.value.Amount})

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

	categories := make([]string, 0, len(byCategory))
	for c := range byCategory {
		categories = append(categories, c)
	}
	sortStrings(categories)
	for _, c := range categories {
		p.InventoryByCategory = append(p.InventoryByCategory, CategoryValue{Category: c, Value: byCategory[c]})
	}

	locations := make([]string, 0, len(byLocation))
	for l := range byLocation {
		locations = append(locations, l)
	}
	sortStrings(locations)
	for _, l := range locations {
		p.InventoryByLocation = append(p.InventoryByLocation, LocationValue{Location: l, Value: byLocation[l]})
	}

	sort.SliceStable(itemValues, func(i, j int) bool {
		if itemValues[i].Value != itemValues[j].Value {
			return itemValues[i].Value > itemValues[j].Value
		}
		return itemValues[i].ItemID < itemValues[j].ItemID
	})
	if topN > 0 && len(itemValues) > topN {
		itemValues = itemValues[:topN]
	}
	for i := range itemValues {
		itemValues[i].Rank = i + 1
	}
	p.TopItemsByValue = itemValues

	return p
}
