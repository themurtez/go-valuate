package vendorspend

import "sort"

// CategorySummary is one category's overall (all-period) spend, share,
// and supplier-count picture — task section 13. Category growth is
// reported separately via Result.Trends.CategorySpendGrowth (keyed
// identically by Category), rather than duplicated here, since growth is
// inherently an adjacent-period-pair figure and SpendTrends already owns
// that machinery for every other dimension.
type CategorySummary struct {
	Category      string      `json:"category"`
	Bridge        SpendBridge `json:"bridge"`
	SharePercent  Value       `json:"share_percent"`
	SupplierCount int         `json:"supplier_count"`
}

// computeCategorySummaries aggregates every included record (all periods)
// into one CategorySummary per non-empty Category.
func computeCategorySummaries(records []SpendRecord, netTotal float64) []CategorySummary {
	type acc struct {
		bridge    SpendBridge
		suppliers map[string]bool
	}
	byCategory := map[string]*acc{}
	for _, r := range records {
		if r.Category == "" {
			continue
		}
		a, ok := byCategory[r.Category]
		if !ok {
			a = &acc{suppliers: map[string]bool{}}
			byCategory[r.Category] = a
		}
		a.bridge.addRecord(r)
		a.suppliers[r.SupplierID] = true
	}
	if len(byCategory) == 0 {
		return nil
	}
	cats := make([]string, 0, len(byCategory))
	for c := range byCategory {
		cats = append(cats, c)
	}
	sort.Strings(cats)

	out := make([]CategorySummary, 0, len(cats))
	for _, c := range cats {
		a := byCategory[c]
		cs := CategorySummary{Category: c, Bridge: a.bridge, SupplierCount: len(a.suppliers)}
		if netTotal != 0 {
			cs.SharePercent = AvailableValue(a.bridge.NetSpend / netTotal)
		}
		out = append(out, cs)
	}
	return out
}
