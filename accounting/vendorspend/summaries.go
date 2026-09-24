package vendorspend

import "sort"

// GroupedSpend is one named group's (type/category/department/location/
// cost center) spend bridge and share of the period total — task section
// 7's "grouped spend where supplied" requirement.
type GroupedSpend struct {
	Key              string      `json:"key"`
	Bridge           SpendBridge `json:"bridge"`
	Share            Value       `json:"share"`
	TransactionCount int         `json:"transaction_count"`
}

// PeriodSummary is one period's overall spend picture — task section 7.
type PeriodSummary struct {
	Period string      `json:"period"`
	Bridge SpendBridge `json:"bridge"`

	SupplierCount          int   `json:"supplier_count"`
	TransactionCount       int   `json:"transaction_count"`
	AverageTransactionSize Value `json:"average_transaction_size"`
	MedianTransactionSize  Value `json:"median_transaction_size"`

	SpendByType       []GroupedSpend `json:"spend_by_type,omitempty"`
	SpendByCategory   []GroupedSpend `json:"spend_by_category,omitempty"`
	SpendByDepartment []GroupedSpend `json:"spend_by_department,omitempty"`
	SpendByLocation   []GroupedSpend `json:"spend_by_location,omitempty"`
	SpendByCostCenter []GroupedSpend `json:"spend_by_cost_center,omitempty"`
}

// groupBy buckets records by keyFn (skipping records where keyFn returns
// "") into a deterministically-ordered ([]GroupedSpend, sorted by Key
// ascending) slice, with Share computed against netTotal.
func groupBy(records []SpendRecord, netTotal float64, keyFn func(SpendRecord) string) []GroupedSpend {
	type acc struct {
		bridge SpendBridge
		count  int
	}
	byKey := map[string]*acc{}
	for _, r := range records {
		k := keyFn(r)
		if k == "" {
			continue
		}
		a, ok := byKey[k]
		if !ok {
			a = &acc{}
			byKey[k] = a
		}
		a.bridge.addRecord(r)
		a.count++
	}
	if len(byKey) == 0 {
		return nil
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]GroupedSpend, 0, len(keys))
	for _, k := range keys {
		a := byKey[k]
		gs := GroupedSpend{Key: k, Bridge: a.bridge, TransactionCount: a.count}
		if netTotal != 0 {
			gs.Share = AvailableValue(a.bridge.NetSpend / netTotal)
		}
		out = append(out, gs)
	}
	return out
}

// computePeriodSummary computes one PeriodSummary from the records
// belonging to a single period (already filtered by caller).
func computePeriodSummary(period string, records []SpendRecord) PeriodSummary {
	ps := PeriodSummary{Period: period, Bridge: computeBridge(records), TransactionCount: len(records)}

	suppliers := map[string]bool{}
	var amounts []float64
	for _, r := range records {
		suppliers[r.SupplierID] = true
		amounts = append(amounts, r.Amount)
	}
	ps.SupplierCount = len(suppliers)

	if len(amounts) > 0 {
		sum := 0.0
		for _, a := range amounts {
			sum += a
		}
		ps.AverageTransactionSize = AvailableValue(sum / float64(len(amounts)))
		ps.MedianTransactionSize = AvailableValue(medianFloat(amounts))
	}

	ps.SpendByType = groupBy(records, ps.Bridge.NetSpend, func(r SpendRecord) string { return string(r.resolvedSpendType()) })
	ps.SpendByCategory = groupBy(records, ps.Bridge.NetSpend, func(r SpendRecord) string { return r.Category })
	ps.SpendByDepartment = groupBy(records, ps.Bridge.NetSpend, func(r SpendRecord) string { return r.Department })
	ps.SpendByLocation = groupBy(records, ps.Bridge.NetSpend, func(r SpendRecord) string { return r.Location })
	ps.SpendByCostCenter = groupBy(records, ps.Bridge.NetSpend, func(r SpendRecord) string { return r.CostCenter })

	return ps
}

// medianFloat returns the median of vs without mutating vs — package-
// local, mirroring every analytics sibling package's own unshared median
// helper (see accounting/journaldiagnostics/amounts.go's identical
// convention and rationale).
func medianFloat(vs []float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	cp := make([]float64, len(vs))
	copy(cp, vs)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

// SupplierPeriodSummary is one supplier's spend summary for one period —
// task section 8.
type SupplierPeriodSummary struct {
	SupplierID   string `json:"supplier_id"`
	SupplierName string `json:"supplier_name,omitempty"`
	Period       string `json:"period"`

	Bridge SpendBridge `json:"bridge"`

	TransactionCount       int   `json:"transaction_count"`
	AverageTransactionSize Value `json:"average_transaction_size"`
	MedianTransactionSize  Value `json:"median_transaction_size"`
	SpendShare             Value `json:"spend_share"`

	CategoryMix []GroupedSpend `json:"category_mix,omitempty"`

	FirstSpendDate string `json:"first_spend_date,omitempty"`
	LastSpendDate  string `json:"last_spend_date,omitempty"`

	// TotalQuantity/WeightedAverageUnitPrice are available only when at
	// least one record has compatible Quantity+UnitOfMeasure (task
	// section 8's "when compatible quantity/UOM exists" rule). When
	// multiple UnitOfMeasure values are present for this supplier/period,
	// these remain Unavailable rather than silently mixing units — see
	// UnitPriceAnalysis for the UOM-scoped breakdown.
	TotalQuantity            Value `json:"total_quantity"`
	WeightedAverageUnitPrice Value `json:"weighted_average_unit_price"`
}

// computeSupplierPeriodSummaries aggregates records (already
// period-filtered) into one SupplierPeriodSummary per SupplierID, sorted
// by NetSpend descending then SupplierID ascending — task section 41's
// documented default order.
func computeSupplierPeriodSummaries(period string, records []SpendRecord, suppliersByID map[string]Supplier, periodNetTotal float64) []SupplierPeriodSummary {
	type acc struct {
		bridge  SpendBridge
		count   int
		amounts []float64
		first   SpendRecord
		last    SpendRecord
		hasDate bool
		rows    []SpendRecord
		uoms    map[string]bool
	}
	bySupplier := map[string]*acc{}
	var order []string

	for _, r := range records {
		a, ok := bySupplier[r.SupplierID]
		if !ok {
			a = &acc{uoms: map[string]bool{}}
			bySupplier[r.SupplierID] = a
			order = append(order, r.SupplierID)
		}
		a.bridge.addRecord(r)
		a.count++
		a.amounts = append(a.amounts, r.Amount)
		a.rows = append(a.rows, r)
		if !a.hasDate || r.Date.Before(a.first.Date) {
			a.first = r
		}
		if !a.hasDate || r.Date.After(a.last.Date) {
			a.last = r
		}
		a.hasDate = true
		if r.UnitOfMeasure != "" {
			a.uoms[r.UnitOfMeasure] = true
		}
	}

	out := make([]SupplierPeriodSummary, 0, len(order))
	for _, sid := range order {
		a := bySupplier[sid]
		sup := suppliersByID[sid]
		s := SupplierPeriodSummary{
			SupplierID:       sid,
			SupplierName:     sup.Name,
			Period:           period,
			Bridge:           a.bridge,
			TransactionCount: a.count,
		}
		if len(a.amounts) > 0 {
			sum := 0.0
			for _, amt := range a.amounts {
				sum += amt
			}
			s.AverageTransactionSize = AvailableValue(sum / float64(len(a.amounts)))
			s.MedianTransactionSize = AvailableValue(medianFloat(a.amounts))
		}
		if periodNetTotal != 0 {
			s.SpendShare = AvailableValue(a.bridge.NetSpend / periodNetTotal)
		}
		s.CategoryMix = groupBy(a.rows, a.bridge.NetSpend, func(r SpendRecord) string { return r.Category })
		if a.hasDate {
			s.FirstSpendDate = a.first.Date.Format("2006-01-02")
			s.LastSpendDate = a.last.Date.Format("2006-01-02")
		}

		if len(a.uoms) == 1 {
			var uom string
			for u := range a.uoms {
				uom = u
			}
			qty, wavg, ok := weightedAverageUnitPrice(a.rows, uom)
			if ok {
				s.TotalQuantity = AvailableValue(qty)
				s.WeightedAverageUnitPrice = AvailableValue(wavg)
			}
		}

		out = append(out, s)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Bridge.NetSpend != out[j].Bridge.NetSpend {
			return out[i].Bridge.NetSpend > out[j].Bridge.NetSpend
		}
		return out[i].SupplierID < out[j].SupplierID
	})
	return out
}

// weightedAverageUnitPrice computes total quantity and the quantity-
// weighted-average unit price across rows sharing UnitOfMeasure uom, for
// rows with both Quantity and UnitPrice Available. Returns ok == false if
// no eligible row exists.
func weightedAverageUnitPrice(rows []SpendRecord, uom string) (totalQty, wavg float64, ok bool) {
	var qtySum, spendSum float64
	found := false
	for _, r := range rows {
		if r.UnitOfMeasure != uom || !r.Quantity.Available || !r.UnitPrice.Available {
			continue
		}
		if r.Quantity.Value <= 0 {
			continue
		}
		qtySum += r.Quantity.Value
		spendSum += r.Quantity.Value * r.UnitPrice.Value
		found = true
	}
	if !found || qtySum == 0 {
		return 0, 0, false
	}
	return qtySum, spendSum / qtySum, true
}
