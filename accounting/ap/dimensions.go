package ap

import "sort"

// DimensionValueSummary is one dimension value's (e.g. one location, one
// business unit) aggregated aging figures.
type DimensionValueSummary struct {
	Value        string  `json:"value"`
	OpenAmount   float64 `json:"open_amount"`
	OverdueTotal float64 `json:"overdue_total"`
	BillCount    int     `json:"bill_count"`
}

// DimensionSummary is the optional one-dimension breakdown Options.Dimension
// selects. Available only if Options.Dimension was set to a non-empty key
// present on at least one included Payable.
type DimensionSummary struct {
	Available bool                    `json:"available"`
	Key       string                  `json:"key,omitempty"`
	Values    []DimensionValueSummary `json:"values,omitempty"`
}

func dimensionValue(dims []Dimension, key string) (string, bool) {
	for _, d := range dims {
		if d.Key == key {
			return d.Value, true
		}
	}
	return "", false
}

// buildDimensionSummary aggregates rows by the Dimension.Value matching
// key, sorted by OpenAmount descending then Value ascending
// (deterministic). This is a single caller-selected grouping, not
// arbitrary OLAP/cube logic.
func buildDimensionSummary(rows []payableAging, sortedBuckets []BucketDefinition, key string) DimensionSummary {
	if key == "" {
		return DimensionSummary{}
	}

	type acc struct {
		openAmount float64
		billCount  int
	}
	byValue := map[string]*acc{}
	var order []string

	for _, row := range rows {
		if !row.includedInAgg || row.isCredit {
			continue
		}
		v, ok := dimensionValue(row.p.Dimensions, key)
		if !ok {
			continue
		}
		a, exists := byValue[v]
		if !exists {
			a = &acc{}
			byValue[v] = a
			order = append(order, v)
		}
		a.openAmount += row.p.OpenAmount
		a.billCount++
	}

	if len(order) == 0 {
		return DimensionSummary{}
	}

	// overdue total requires bucket membership; recompute with a second
	// filtered pass keyed by value.
	overdueByValue := map[string]float64{}
	for _, row := range rows {
		if !row.includedInAgg || row.isCredit {
			continue
		}
		v, ok := dimensionValue(row.p.Dimensions, key)
		if !ok {
			continue
		}
		if !bucketIsCurrent(sortedBuckets, row.bucketCode) {
			overdueByValue[v] += row.p.OpenAmount
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		ai, aj := byValue[order[i]], byValue[order[j]]
		if ai.openAmount != aj.openAmount {
			return ai.openAmount > aj.openAmount
		}
		return order[i] < order[j]
	})

	values := make([]DimensionValueSummary, 0, len(order))
	for _, v := range order {
		a := byValue[v]
		values = append(values, DimensionValueSummary{
			Value:        v,
			OpenAmount:   a.openAmount,
			OverdueTotal: overdueByValue[v],
			BillCount:    a.billCount,
		})
	}

	return DimensionSummary{Available: true, Key: key, Values: values}
}
