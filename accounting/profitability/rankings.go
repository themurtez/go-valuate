package profitability

import "sort"

// RankingEntry is one entity's position in a deterministic ranking — task
// section 44. No composite score: every ranking is a plain sort by one
// factual metric, ties broken by EntityID.
type RankingEntry struct {
	EntityID string  `json:"entity_id"`
	Value    float64 `json:"value"`
}

// Rankings bundles every deterministic entity ranking for one dimension
// (all-period basis) — task sections 44-46.
type Rankings struct {
	TopByNetRevenue         []RankingEntry `json:"top_by_net_revenue,omitempty"`
	TopByGrossProfit        []RankingEntry `json:"top_by_gross_profit,omitempty"`
	TopByContributionProfit []RankingEntry `json:"top_by_contribution_profit,omitempty"`

	// NegativeContribution/NegativeAllocatedProfit list every entity
	// (all-period) whose ContributionProfit/AllocatedProfit is negative
	// and material per Policy.NegativeContributionMateriality — factual
	// codes, not a broad "unprofitable" label (task section 46).
	NegativeContribution    []RankingEntry `json:"negative_contribution,omitempty"`
	NegativeAllocatedProfit []RankingEntry `json:"negative_allocated_profit,omitempty"`
}

func sortRankingDesc(entries []RankingEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Value != entries[j].Value {
			return entries[i].Value > entries[j].Value
		}
		return entries[i].EntityID < entries[j].EntityID
	})
}

func topN(entries []RankingEntry, n int) []RankingEntry {
	if n <= 0 || n >= len(entries) {
		return entries
	}
	return entries[:n]
}

// buildRankings computes Rankings from a dimension's all-period entity
// totals — task sections 44-46.
func buildRankings(entities []AllPeriodResult, topNLimit int, materiality MaterialityPolicy) Rankings {
	var byRevenue, byGross, byContribution, negContribution, negAllocated []RankingEntry
	for _, e := range entities {
		byRevenue = append(byRevenue, RankingEntry{EntityID: e.EntityID, Value: e.RevenueBridge.NetRevenue})
		byGross = append(byGross, RankingEntry{EntityID: e.EntityID, Value: e.GrossProfit})
		byContribution = append(byContribution, RankingEntry{EntityID: e.EntityID, Value: e.ContributionProfit})
		if e.ContributionProfit < 0 && materiality.isMaterial(e.ContributionProfit, e.RevenueBridge.NetRevenue) {
			negContribution = append(negContribution, RankingEntry{EntityID: e.EntityID, Value: e.ContributionProfit})
		}
		if e.AllocatedProfit < 0 && materiality.isMaterial(e.AllocatedProfit, e.RevenueBridge.NetRevenue) {
			negAllocated = append(negAllocated, RankingEntry{EntityID: e.EntityID, Value: e.AllocatedProfit})
		}
	}
	sortRankingDesc(byRevenue)
	sortRankingDesc(byGross)
	sortRankingDesc(byContribution)
	// Negative rankings sort worst-first (ascending value, i.e. most
	// negative first) since "most negative" is the more attention-worthy
	// row for these two lists.
	sort.SliceStable(negContribution, func(i, j int) bool {
		if negContribution[i].Value != negContribution[j].Value {
			return negContribution[i].Value < negContribution[j].Value
		}
		return negContribution[i].EntityID < negContribution[j].EntityID
	})
	sort.SliceStable(negAllocated, func(i, j int) bool {
		if negAllocated[i].Value != negAllocated[j].Value {
			return negAllocated[i].Value < negAllocated[j].Value
		}
		return negAllocated[i].EntityID < negAllocated[j].EntityID
	})

	return Rankings{
		TopByNetRevenue:         topN(byRevenue, topNLimit),
		TopByGrossProfit:        topN(byGross, topNLimit),
		TopByContributionProfit: topN(byContribution, topNLimit),
		NegativeContribution:    negContribution,
		NegativeAllocatedProfit: negAllocated,
	}
}

// MarginRankingEntry is one entity's position in a margin-based ranking —
// task section 45. Separate from RankingEntry so
// MinimumRevenueForMarginRanking exclusion is visible in the type (an
// entity below the threshold never appears here at all).
type MarginRankingEntry struct {
	EntityID string  `json:"entity_id"`
	Margin   float64 `json:"margin"`
	Revenue  float64 `json:"revenue"`
}

// buildMarginRanking ranks entities by ContributionMargin descending,
// excluding any entity whose NetRevenue is below minimumRevenue — task
// section 45 "prevent tiny-denominator entities from dominating margin
// rankings."
func buildMarginRanking(entities []AllPeriodResult, minimumRevenue float64) []MarginRankingEntry {
	var out []MarginRankingEntry
	for _, e := range entities {
		if !e.Margins.ContributionMargin.Available {
			continue
		}
		if minimumRevenue > 0 && e.RevenueBridge.NetRevenue < minimumRevenue {
			continue
		}
		out = append(out, MarginRankingEntry{EntityID: e.EntityID, Margin: e.Margins.ContributionMargin.Amount, Revenue: e.RevenueBridge.NetRevenue})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Margin != out[j].Margin {
			return out[i].Margin > out[j].Margin
		}
		return out[i].EntityID < out[j].EntityID
	})
	return out
}
