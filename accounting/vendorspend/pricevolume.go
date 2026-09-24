package vendorspend

import "sort"

// priceVolumeKey groups records for price-volume decomposition: same
// supplier, same product, same UOM — mirrors unitPriceKey's identical
// rationale (a distinct type since the two keys are used in different
// files/functions and Go has no cheap type aliasing benefit here).
type priceVolumeKey struct{ supplierID, productID, uom string }

// PriceVolumeDecomposition is one homogeneous supplier/product/UOM
// group's adjacent-period-pair spend-change decomposition — task section
// 18. Locked identity (see pricevolume_invariant_test.go):
//
//	PriceEffect + VolumeEffect + Interaction = TotalSpendChange
//
// using the documented formulas:
//
//	PriceEffect  = (P1 - P0) * Q0
//	VolumeEffect = (Q1 - Q0) * P0
//	Interaction  = (P1 - P0) * (Q1 - Q0)
//
// Only ever computed for a single supplier/product/UOM pair with
// eligible (positive quantity, non-negative price, matching UOM) data in
// both periods — task section 18's "do not apply across heterogeneous
// mixes" rule. A caller wanting a portfolio-level price/volume story
// composes several of these itself; this package never aggregates across
// incompatible groups on its own.
type PriceVolumeDecomposition struct {
	SupplierID    string `json:"supplier_id"`
	ProductID     string `json:"product_id"`
	UnitOfMeasure string `json:"unit_of_measure"`
	FromPeriod    string `json:"from_period"`
	ToPeriod      string `json:"to_period"`

	P0 float64 `json:"p0"`
	P1 float64 `json:"p1"`
	Q0 float64 `json:"q0"`
	Q1 float64 `json:"q1"`

	PriceEffect      float64 `json:"price_effect"`
	VolumeEffect     float64 `json:"volume_effect"`
	Interaction      float64 `json:"interaction"`
	TotalSpendChange float64 `json:"total_spend_change"`
}

// computePriceVolumeDecomposition computes the decomposition for one
// group given its period-0 and period-1 quantity-weighted-average price
// and total quantity.
func computePriceVolumeDecomposition(supplierID, productID, uom, fromPeriod, toPeriod string, p0, q0, p1, q1 float64) PriceVolumeDecomposition {
	priceEffect := (p1 - p0) * q0
	volumeEffect := (q1 - q0) * p0
	interaction := (p1 - p0) * (q1 - q0)
	return PriceVolumeDecomposition{
		SupplierID: supplierID, ProductID: productID, UnitOfMeasure: uom,
		FromPeriod: fromPeriod, ToPeriod: toPeriod,
		P0: p0, P1: p1, Q0: q0, Q1: q1,
		PriceEffect: priceEffect, VolumeEffect: volumeEffect, Interaction: interaction,
		TotalSpendChange: priceEffect + volumeEffect + interaction,
	}
}

// computePriceVolumeDecompositions computes one PriceVolumeDecomposition
// per (SupplierID, ProductID, UnitOfMeasure) group present with eligible
// quantity+price data in both periods of every chronologically adjacent
// pair.
func computePriceVolumeDecompositions(orderedPeriods []string, byPeriod map[string][]SpendRecord) []PriceVolumeDecomposition {
	if len(orderedPeriods) < 2 {
		return nil
	}

	// periodGroupTotals[period][key] = (qtySum, spendSum)
	periodGroupTotals := make(map[string]map[priceVolumeKey][2]float64, len(orderedPeriods))
	for _, p := range orderedPeriods {
		totals := map[priceVolumeKey][2]float64{}
		for _, r := range byPeriod[p] {
			if r.ProductID == "" || !r.Quantity.Available || !r.UnitPrice.Available || r.UnitOfMeasure == "" {
				continue
			}
			if r.Quantity.Value <= 0 || isNonFinite(r.Quantity.Value) || isNonFinite(r.UnitPrice.Value) || r.UnitPrice.Value < 0 {
				continue
			}
			k := priceVolumeKey{r.SupplierID, r.ProductID, r.UnitOfMeasure}
			cur := totals[k]
			cur[0] += r.Quantity.Value
			cur[1] += r.Quantity.Value * r.UnitPrice.Value
			totals[k] = cur
		}
		periodGroupTotals[p] = totals
	}

	var out []PriceVolumeDecomposition
	for i := 1; i < len(orderedPeriods); i++ {
		from, to := orderedPeriods[i-1], orderedPeriods[i]
		fromTotals, toTotals := periodGroupTotals[from], periodGroupTotals[to]

		keys := make([]priceVolumeKey, 0, len(fromTotals))
		for k := range fromTotals {
			if _, ok := toTotals[k]; ok {
				keys = append(keys, k)
			}
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].supplierID != keys[j].supplierID {
				return keys[i].supplierID < keys[j].supplierID
			}
			if keys[i].productID != keys[j].productID {
				return keys[i].productID < keys[j].productID
			}
			return keys[i].uom < keys[j].uom
		})

		for _, k := range keys {
			fq, fs := fromTotals[k][0], fromTotals[k][1]
			tq, ts := toTotals[k][0], toTotals[k][1]
			if fq == 0 || tq == 0 {
				continue
			}
			p0, q0 := fs/fq, fq
			p1, q1 := ts/tq, tq
			out = append(out, computePriceVolumeDecomposition(k.supplierID, k.productID, k.uom, from, to, p0, q0, p1, q1))
		}
	}
	return out
}
