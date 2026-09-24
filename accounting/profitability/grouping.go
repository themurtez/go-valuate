package profitability

import "sort"

// GroupSummary is one caller-provided Group or Category label's all-
// period rollup for one dimension — task section 34.
type GroupSummary struct {
	Dimension Dimension `json:"dimension"`
	// GroupKey is the raw Entity.Group or Entity.Category value this
	// summary was grouped by.
	GroupKey    string `json:"group_key"`
	EntityCount int    `json:"entity_count"`

	NetRevenue         float64 `json:"net_revenue"`
	GrossProfit        float64 `json:"gross_profit"`
	ContributionProfit float64 `json:"contribution_profit"`
	AllocatedProfit    float64 `json:"allocated_profit"`

	Margins Margins `json:"margins"`
}

// buildGroupSummaries rolls up entities's all-period totals by
// keyFunc(entity) (Entity.Group or Entity.Category), skipping entities
// with an empty key. Deterministic ascending-normalized-name order.
func buildGroupSummaries(dim Dimension, entities []AllPeriodResult, keyOf map[string]string) []GroupSummary {
	byKey := map[string]*GroupSummary{}
	for _, e := range entities {
		key := keyOf[e.EntityID]
		if key == "" {
			continue
		}
		g, ok := byKey[key]
		if !ok {
			g = &GroupSummary{Dimension: dim, GroupKey: key}
			byKey[key] = g
		}
		g.EntityCount++
		g.NetRevenue += e.RevenueBridge.NetRevenue
		g.GrossProfit += e.GrossProfit
		g.ContributionProfit += e.ContributionProfit
		g.AllocatedProfit += e.AllocatedProfit
	}
	keys := sortedStringKeys(byKey)
	out := make([]GroupSummary, 0, len(keys))
	for _, k := range keys {
		g := byKey[k]
		g.Margins = Margins{
			GrossMargin:        marginValue(g.GrossProfit, g.NetRevenue),
			ContributionMargin: marginValue(g.ContributionProfit, g.NetRevenue),
			AllocatedMargin:    marginValue(g.AllocatedProfit, g.NetRevenue),
		}
		out = append(out, *g)
	}
	return out
}

func sortEntityPeriodResultsForDisplay(rs []EntityPeriodResult) {
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].ContributionProfit != rs[j].ContributionProfit {
			return rs[i].ContributionProfit > rs[j].ContributionProfit
		}
		return rs[i].EntityID < rs[j].EntityID
	})
}

func sortAllPeriodResultsForDisplay(rs []AllPeriodResult) {
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].ContributionProfit != rs[j].ContributionProfit {
			return rs[i].ContributionProfit > rs[j].ContributionProfit
		}
		return rs[i].EntityID < rs[j].EntityID
	})
}
