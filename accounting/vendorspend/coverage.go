package vendorspend

// CategoryProductCoverage reports category/product attribution coverage
// by spend dollars — task section 30. Never inferred: a record with an
// empty Category/ProductID is counted as uncategorized/unattributed, full
// stop.
//
// CategorizedSpend/UncategorizedSpend (and their product-attribution
// counterparts) are on the same signed net-spend basis as every other
// dollar figure in this package, so CategorizedSpend + UncategorizedSpend
// always equals the analysis's total NetSpend (see task section 40's
// locked invariant and invariants_test.go). CategoryCoveragePercent/
// ProductCoveragePercent are DELIBERATELY computed from a separate,
// unsigned-magnitude denominator (sum of |Amount| grouped by
// category/product presence) rather than from CategorizedSpend/
// UncategorizedSpend directly: a ratio built on signed net dollars
// degenerates when an uncategorized credit largely offsets categorized
// purchases — e.g. $100,000 categorized net spend against a $99,999
// uncategorized net credit gives a signed total of $1 and an
// arithmetically correct but meaningless 10,000,000% "coverage" figure,
// even though the underlying transaction volume is almost entirely
// categorized. The unsigned-magnitude basis reports coverage as "share
// of transactional dollar volume that is categorized," which stays
// sensible regardless of credits/refunds mixed into either side.
type CategoryProductCoverage struct {
	CategorizedSpend        float64 `json:"categorized_spend"`
	UncategorizedSpend      float64 `json:"uncategorized_spend"`
	CategoryCoveragePercent Value   `json:"category_coverage_percent"`

	ProductAttributedSpend   float64 `json:"product_attributed_spend"`
	UnattributedProductSpend float64 `json:"unattributed_product_spend"`
	ProductCoveragePercent   Value   `json:"product_coverage_percent"`
}

// computeCategoryProductCoverage measures categorized/uncategorized and
// product-attributed/unattributed net spend across records, plus the
// unsigned-magnitude-basis coverage percentages — see
// CategoryProductCoverage's doc comment for why the two bases differ.
func computeCategoryProductCoverage(records []SpendRecord) CategoryProductCoverage {
	var c CategoryProductCoverage
	var categorizedMagnitude, uncategorizedMagnitude float64
	var attributedMagnitude, unattributedMagnitude float64
	for _, r := range records {
		net := netSpendOf(r)
		magnitude := absFloat(r.Amount)
		if r.Category != "" {
			c.CategorizedSpend += net
			categorizedMagnitude += magnitude
		} else {
			c.UncategorizedSpend += net
			uncategorizedMagnitude += magnitude
		}
		if r.ProductID != "" {
			c.ProductAttributedSpend += net
			attributedMagnitude += magnitude
		} else {
			c.UnattributedProductSpend += net
			unattributedMagnitude += magnitude
		}
	}
	if magnitudeTotal := categorizedMagnitude + uncategorizedMagnitude; magnitudeTotal != 0 {
		c.CategoryCoveragePercent = AvailableValue(categorizedMagnitude / magnitudeTotal)
	}
	if magnitudeTotal := attributedMagnitude + unattributedMagnitude; magnitudeTotal != 0 {
		c.ProductCoveragePercent = AvailableValue(attributedMagnitude / magnitudeTotal)
	}
	return c
}

// MetadataCoverage reports factual field-population coverage across
// records, as a ratio of record count (not dollars — orthogonal to
// CategoryProductCoverage's dollar-based measure, since a caller
// evaluating "is my data clean enough" usually cares about row
// completeness, while "how much spend is unclassified" cares about
// dollars) — task section 31. No opaque composite score: every field is
// its own named ratio.
type MetadataCoverage struct {
	CategoryCoverage   Value `json:"category_coverage"`
	ProductCoverage    Value `json:"product_coverage"`
	QuantityCoverage   Value `json:"quantity_coverage"`
	UnitPriceCoverage  Value `json:"unit_price_coverage"`
	DepartmentCoverage Value `json:"department_coverage"`
	LocationCoverage   Value `json:"location_coverage"`
	CostCenterCoverage Value `json:"cost_center_coverage"`
	RecurrenceCoverage Value `json:"recurrence_coverage"`
	CommitmentCoverage Value `json:"commitment_coverage"`
	// ControlCoverage is the fraction of the five ControlTotals fields
	// (Purchases/ExpenseSpend/CapexSpend/InventoryPurchases/
	// ContractorSpend) that were supplied (non-nil), not a spend-based
	// ratio — control totals are analysis-wide, not per-record.
	ControlCoverage Value `json:"control_coverage"`
}

// computeMetadataCoverage computes every per-record field-population
// ratio plus the analysis-wide ControlCoverage.
func computeMetadataCoverage(records []SpendRecord, controls ControlTotals) MetadataCoverage {
	n := len(records)
	if n == 0 {
		return MetadataCoverage{}
	}
	var category, product, quantity, unitPrice, department, location, costCenter, recurrence, commitment int
	for _, r := range records {
		if r.Category != "" {
			category++
		}
		if r.ProductID != "" {
			product++
		}
		if r.Quantity.Available {
			quantity++
		}
		if r.UnitPrice.Available {
			unitPrice++
		}
		if r.Department != "" {
			department++
		}
		if r.Location != "" {
			location++
		}
		if r.CostCenter != "" {
			costCenter++
		}
		if r.Recurrence != "" {
			recurrence++
		}
		if r.Commitment != "" {
			commitment++
		}
	}

	nf := float64(n)
	mc := MetadataCoverage{
		CategoryCoverage:   AvailableValue(float64(category) / nf),
		ProductCoverage:    AvailableValue(float64(product) / nf),
		QuantityCoverage:   AvailableValue(float64(quantity) / nf),
		UnitPriceCoverage:  AvailableValue(float64(unitPrice) / nf),
		DepartmentCoverage: AvailableValue(float64(department) / nf),
		LocationCoverage:   AvailableValue(float64(location) / nf),
		CostCenterCoverage: AvailableValue(float64(costCenter) / nf),
		RecurrenceCoverage: AvailableValue(float64(recurrence) / nf),
		CommitmentCoverage: AvailableValue(float64(commitment) / nf),
	}

	controlsSupplied := 0
	if controls.Purchases != nil {
		controlsSupplied++
	}
	if controls.ExpenseSpend != nil {
		controlsSupplied++
	}
	if controls.CapexSpend != nil {
		controlsSupplied++
	}
	if controls.InventoryPurchases != nil {
		controlsSupplied++
	}
	if controls.ContractorSpend != nil {
		controlsSupplied++
	}
	mc.ControlCoverage = AvailableValue(float64(controlsSupplied) / 5.0)

	return mc
}
