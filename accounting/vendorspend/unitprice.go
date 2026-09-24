package vendorspend

import "sort"

// unitPriceKey groups records eligible for unit-price analytics: same
// supplier, same ProductID (may be empty), same UnitOfMeasure. Two
// records are only ever compared/aggregated if they share this full key
// — task section 14's "never compare or aggregate unit prices across
// incompatible UOM" rule, extended here to "or across different
// products," which the task's per-supplier unit-price section (15)
// implicitly assumes by scoping to "same supplier/product/UOM."
type unitPriceKey struct {
	SupplierID    string
	ProductID     string
	UnitOfMeasure string
}

// eligibleUnitPriceRows returns every record with both Quantity and
// UnitPrice Available (positive, finite Quantity), grouped by
// unitPriceKey.
func eligibleUnitPriceRows(records []SpendRecord) map[unitPriceKey][]SpendRecord {
	out := map[unitPriceKey][]SpendRecord{}
	for _, r := range records {
		if !r.Quantity.Available || !r.UnitPrice.Available || r.UnitOfMeasure == "" {
			continue
		}
		if r.Quantity.Value <= 0 || isNonFinite(r.Quantity.Value) || isNonFinite(r.UnitPrice.Value) || r.UnitPrice.Value < 0 {
			continue
		}
		k := unitPriceKey{SupplierID: r.SupplierID, ProductID: r.ProductID, UnitOfMeasure: r.UnitOfMeasure}
		out[k] = append(out[k], r)
	}
	return out
}

// UnitPricePoint is one supplier/product/UOM group's unit-price
// statistics across every included record for that group — task section
// 15.
type UnitPricePoint struct {
	SupplierID    string `json:"supplier_id"`
	ProductID     string `json:"product_id,omitempty"`
	UnitOfMeasure string `json:"unit_of_measure"`

	WeightedAverageUnitPrice Value `json:"weighted_average_unit_price"`
	MinUnitPrice             Value `json:"min_unit_price"`
	MaxUnitPrice             Value `json:"max_unit_price"`
	MedianUnitPrice          Value `json:"median_unit_price"`

	// FirstUnitPrice/LastUnitPrice are the unit prices of the
	// chronologically first/last record in the group (by Date), used for
	// UnitPriceChange.
	FirstUnitPrice Value `json:"first_unit_price"`
	LastUnitPrice  Value `json:"last_unit_price"`
	// UnitPriceChange/UnitPriceChangePercent are LastUnitPrice -
	// FirstUnitPrice and that change as a percent of FirstUnitPrice.
	// Unavailable if fewer than two distinct dates exist in the group, or
	// FirstUnitPrice is zero (for the percent).
	UnitPriceChange        Value `json:"unit_price_change"`
	UnitPriceChangePercent Value `json:"unit_price_change_percent"`

	RecordCount int `json:"record_count"`
}

// computeUnitPricePoints computes one UnitPricePoint per unitPriceKey
// group, sorted by SupplierID then ProductID then UnitOfMeasure.
func computeUnitPricePoints(records []SpendRecord) []UnitPricePoint {
	groups := eligibleUnitPriceRows(records)
	if len(groups) == 0 {
		return nil
	}
	keys := make([]unitPriceKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].SupplierID != keys[j].SupplierID {
			return keys[i].SupplierID < keys[j].SupplierID
		}
		if keys[i].ProductID != keys[j].ProductID {
			return keys[i].ProductID < keys[j].ProductID
		}
		return keys[i].UnitOfMeasure < keys[j].UnitOfMeasure
	})

	out := make([]UnitPricePoint, 0, len(keys))
	for _, k := range keys {
		rows := make([]SpendRecord, len(groups[k]))
		copy(rows, groups[k])
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Date.Before(rows[j].Date) })

		p := UnitPricePoint{SupplierID: k.SupplierID, ProductID: k.ProductID, UnitOfMeasure: k.UnitOfMeasure, RecordCount: len(rows)}

		var qtySum, spendSum float64
		prices := make([]float64, 0, len(rows))
		for _, r := range rows {
			qtySum += r.Quantity.Value
			spendSum += r.Quantity.Value * r.UnitPrice.Value
			prices = append(prices, r.UnitPrice.Value)
		}
		p.WeightedAverageUnitPrice = AvailableValue(spendSum / qtySum)

		minP, maxP := prices[0], prices[0]
		for _, pr := range prices {
			if pr < minP {
				minP = pr
			}
			if pr > maxP {
				maxP = pr
			}
		}
		p.MinUnitPrice = AvailableValue(minP)
		p.MaxUnitPrice = AvailableValue(maxP)
		p.MedianUnitPrice = AvailableValue(medianFloat(prices))

		p.FirstUnitPrice = AvailableValue(rows[0].UnitPrice.Value)
		p.LastUnitPrice = AvailableValue(rows[len(rows)-1].UnitPrice.Value)
		if len(rows) > 1 {
			change := rows[len(rows)-1].UnitPrice.Value - rows[0].UnitPrice.Value
			p.UnitPriceChange = AvailableValue(change)
			if rows[0].UnitPrice.Value != 0 {
				p.UnitPriceChangePercent = AvailableValue(change / rows[0].UnitPrice.Value)
			}
		}

		out = append(out, p)
	}
	return out
}

// ProductPriceComparison is one ProductID's cross-supplier unit-price
// comparison — task section 16. Only populated for a ProductID with
// eligible unit-price data from more than one supplier under a single
// shared UnitOfMeasure (task section 14's "no inferred UOM conversion"
// rule applies here too: suppliers quoting a product in incompatible
// UOMs are never compared). Explicitly performs no supplier-switching
// recommendation — task section 16's "do not recommend switching
// suppliers" rule.
type ProductPriceComparison struct {
	ProductID     string `json:"product_id"`
	UnitOfMeasure string `json:"unit_of_measure"`

	ProductMedianPrice  Value `json:"product_median_price"`
	LowestObservedPrice Value `json:"lowest_observed_price"`

	Suppliers []SupplierProductPrice `json:"suppliers"`
}

// SupplierProductPrice is one supplier's price position within a
// ProductPriceComparison.
type SupplierProductPrice struct {
	SupplierID                   string `json:"supplier_id"`
	SupplierWeightedAveragePrice Value  `json:"supplier_weighted_average_price"`
	DifferenceVsProductMedian    Value  `json:"difference_vs_product_median"`
	DifferenceVsLowestObserved   Value  `json:"difference_vs_lowest_observed"`
}

// computeProductPriceComparisons builds one ProductPriceComparison per
// (ProductID, UnitOfMeasure) pair with eligible data from 2+ distinct
// suppliers.
func computeProductPriceComparisons(points []UnitPricePoint) []ProductPriceComparison {
	type key struct{ productID, uom string }
	byKey := map[key][]UnitPricePoint{}
	for _, p := range points {
		if p.ProductID == "" || !p.WeightedAverageUnitPrice.Available {
			continue
		}
		k := key{p.ProductID, p.UnitOfMeasure}
		byKey[k] = append(byKey[k], p)
	}

	keys := make([]key, 0, len(byKey))
	for k, pts := range byKey {
		suppliers := map[string]bool{}
		for _, p := range pts {
			suppliers[p.SupplierID] = true
		}
		if len(suppliers) < 2 {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].productID != keys[j].productID {
			return keys[i].productID < keys[j].productID
		}
		return keys[i].uom < keys[j].uom
	})

	out := make([]ProductPriceComparison, 0, len(keys))
	for _, k := range keys {
		pts := byKey[k]
		sort.SliceStable(pts, func(i, j int) bool { return pts[i].SupplierID < pts[j].SupplierID })

		prices := make([]float64, 0, len(pts))
		for _, p := range pts {
			prices = append(prices, p.WeightedAverageUnitPrice.Value)
		}
		productMedian := medianFloat(prices)
		lowest := prices[0]
		for _, pr := range prices {
			if pr < lowest {
				lowest = pr
			}
		}

		pc := ProductPriceComparison{
			ProductID: k.productID, UnitOfMeasure: k.uom,
			ProductMedianPrice: AvailableValue(productMedian), LowestObservedPrice: AvailableValue(lowest),
		}
		for _, p := range pts {
			pc.Suppliers = append(pc.Suppliers, SupplierProductPrice{
				SupplierID:                   p.SupplierID,
				SupplierWeightedAveragePrice: p.WeightedAverageUnitPrice,
				DifferenceVsProductMedian:    AvailableValue(p.WeightedAverageUnitPrice.Value - productMedian),
				DifferenceVsLowestObserved:   AvailableValue(p.WeightedAverageUnitPrice.Value - lowest),
			})
		}
		out = append(out, pc)
	}
	return out
}

// ProductSpend is one ProductID's overall spend/quantity/supplier
// picture — task section 17.
type ProductSpend struct {
	ProductID            string  `json:"product_id"`
	Spend                float64 `json:"spend"`
	Quantity             Value   `json:"quantity"`
	SupplierCount        int     `json:"supplier_count"`
	LargestSupplierShare Value   `json:"largest_supplier_share"`
	// ObservedSingleSource is true when exactly one SupplierID has any
	// spend for this ProductID in the supplied data. This means only
	// "one observed supplier in supplied data" — task section 17's
	// explicit "do not infer impossibility of alternate sourcing" rule;
	// it is never evidence that no other supplier could provide the
	// product.
	ObservedSingleSource bool `json:"observed_single_source"`
}

// computeProductSpend aggregates every included record with a non-empty
// ProductID into one ProductSpend per ProductID, sorted by ProductID
// ascending.
func computeProductSpend(records []SpendRecord) []ProductSpend {
	type acc struct {
		spend      float64
		hasQty     bool
		qty        float64
		bySupplier map[string]float64
	}
	byProduct := map[string]*acc{}
	for _, r := range records {
		if r.ProductID == "" {
			continue
		}
		a, ok := byProduct[r.ProductID]
		if !ok {
			a = &acc{bySupplier: map[string]float64{}}
			byProduct[r.ProductID] = a
		}
		net := netSpendOf(r)
		a.spend += net
		a.bySupplier[r.SupplierID] += net
		if r.Quantity.Available && !isNonFinite(r.Quantity.Value) {
			a.hasQty = true
			a.qty += r.Quantity.Value
		}
	}
	if len(byProduct) == 0 {
		return nil
	}
	products := make([]string, 0, len(byProduct))
	for p := range byProduct {
		products = append(products, p)
	}
	sort.Strings(products)

	out := make([]ProductSpend, 0, len(products))
	for _, pid := range products {
		a := byProduct[pid]
		ps := ProductSpend{ProductID: pid, Spend: a.spend, SupplierCount: len(a.bySupplier)}
		if a.hasQty {
			ps.Quantity = AvailableValue(a.qty)
		}
		var largest float64
		for _, s := range a.bySupplier {
			if s > largest {
				largest = s
			}
		}
		if a.spend != 0 {
			ps.LargestSupplierShare = AvailableValue(largest / a.spend)
		}
		ps.ObservedSingleSource = len(a.bySupplier) == 1
		out = append(out, ps)
	}
	return out
}
