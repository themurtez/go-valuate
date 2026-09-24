package vendorspend

import "sort"

// DependencyClassSummary is one DependencyClass's factual spend measures
// — task section 10. This package never derives a hidden dependency
// score; DependencyClass itself is always caller-supplied (see
// Supplier.Dependency), and this summary only totals spend by that
// caller-supplied label.
type DependencyClassSummary struct {
	Class         DependencyClass `json:"class"`
	SupplierCount int             `json:"supplier_count"`
	Spend         float64         `json:"spend"`
	SpendPercent  Value           `json:"spend_percent"`
}

// DependencySummary reports factual spend measures grouped by each
// supplier's caller-declared DependencyClass, for the analysis's overall
// (all-period) net spend — task section 10. CriticalSupplierSpend/
// CriticalSupplierSpendPercent are called out by name (mirroring the
// task's explicit field names) since CRITICAL is the dependency label
// most often needed at a glance; the full by-class breakdown is in
// ByClass.
type DependencySummary struct {
	ByClass []DependencyClassSummary `json:"by_class,omitempty"`

	CriticalSupplierSpend        float64 `json:"critical_supplier_spend"`
	CriticalSupplierSpendPercent Value   `json:"critical_supplier_spend_percent"`
}

// computeDependencySummary totals each supplier's overall NetSpend
// (across every period) grouped by its resolved DependencyClass.
func computeDependencySummary(records []SpendRecord, suppliersByID map[string]Supplier) DependencySummary {
	bySupplier := map[string]float64{}
	for _, r := range records {
		bySupplier[r.SupplierID] += netSpendOf(r)
	}

	byClass := map[DependencyClass]*struct {
		count int
		spend float64
	}{}
	var totalSpend float64
	for sid, spend := range bySupplier {
		totalSpend += spend
		cls := suppliersByID[sid].resolvedDependency()
		a, ok := byClass[cls]
		if !ok {
			a = &struct {
				count int
				spend float64
			}{}
			byClass[cls] = a
		}
		a.count++
		a.spend += spend
	}

	classes := make([]DependencyClass, 0, len(byClass))
	for c := range byClass {
		classes = append(classes, c)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i] < classes[j] })

	ds := DependencySummary{}
	for _, c := range classes {
		a := byClass[c]
		cs := DependencyClassSummary{Class: c, SupplierCount: a.count, Spend: a.spend}
		if totalSpend != 0 {
			cs.SpendPercent = AvailableValue(a.spend / totalSpend)
		}
		ds.ByClass = append(ds.ByClass, cs)
		if c == DependencyCritical {
			ds.CriticalSupplierSpend = a.spend
			if totalSpend != 0 {
				ds.CriticalSupplierSpendPercent = AvailableValue(a.spend / totalSpend)
			}
		}
	}
	return ds
}

// netSpendOf returns r's contribution to net spend (positive for
// EffectNormal, negative for a reducing effect) — a one-record
// convenience used where a full SpendBridge is unnecessary.
func netSpendOf(r SpendRecord) float64 {
	if isReducingEffect(r.resolvedEffect()) {
		return -absFloat(r.Amount)
	}
	return r.Amount
}
