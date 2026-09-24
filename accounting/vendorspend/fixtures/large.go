package fixtures

import (
	"strconv"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

func itoa(v int) string { return strconv.Itoa(v) }

// LargePopulation generates a synthetic large-scale dataset for
// benchmarking — task section 46: numPeriods chronological monthly
// periods, numSuppliers suppliers, numProducts distinct ProductIDs
// (round-robin assigned across records so cross-supplier product
// comparisons and observed-single-source detection both have real work
// to do), and approximately numRecords SpendRecords spread evenly
// across suppliers/periods/products.
//
// Every slice is pre-allocated to its known final length, and every
// record is built from small, pre-formatted ID slices (never
// strconv.Itoa'd inside the innermost loop) — mirrors
// accounting/profitability/fixtures.LargePopulation's identical
// discipline, adopted here specifically because that package's doc
// comment documents catching a real >100-second GENERATION cost (not a
// Calculate cost) at large scale from exactly this mistake.
func LargePopulation(numRecords, numSuppliers, numProducts, numPeriods int) (
	periods []vendorspend.Period,
	suppliers []vendorspend.Supplier,
	records []vendorspend.SpendRecord,
) {
	if numRecords <= 0 {
		numRecords = 1
	}
	if numSuppliers <= 0 {
		numSuppliers = 1
	}
	if numProducts <= 0 {
		numProducts = 1
	}
	if numPeriods <= 0 {
		numPeriods = 1
	}

	periods = make([]vendorspend.Period, 0, numPeriods)
	start := date("2020-01-01")
	periodLabels := make([]string, numPeriods)
	for i := 0; i < numPeriods; i++ {
		s := start.AddDate(0, i, 0)
		e := s.AddDate(0, 1, -1)
		label := s.Format("2006-01")
		periodLabels[i] = label
		periods = append(periods, vendorspend.Period{Period: label, StartDate: s, EndDate: e, SequenceInYear: i + 1})
	}

	supplierIDs := make([]string, numSuppliers)
	suppliers = make([]vendorspend.Supplier, 0, numSuppliers)
	for i := range supplierIDs {
		id := "SUP" + itoa(i)
		supplierIDs[i] = id
		suppliers = append(suppliers, vendorspend.Supplier{SupplierID: id, Name: "Supplier " + itoa(i), Category: "CAT" + itoa(i%20), Active: true})
	}

	productIDs := make([]string, numProducts)
	for i := range productIDs {
		productIDs[i] = "PROD" + itoa(i)
	}

	records = make([]vendorspend.SpendRecord, 0, numRecords)
	for i := 0; i < numRecords; i++ {
		sIdx := i % numSuppliers
		pIdx := i % numProducts
		periodIdx := i % numPeriods
		date := periods[periodIdx].StartDate.AddDate(0, 0, i%28)

		records = append(records, vendorspend.SpendRecord{
			SpendID: "SP" + itoa(i), SupplierID: supplierIDs[sIdx], Period: periodLabels[periodIdx], Date: date,
			Amount: 100 + float64(i%5000), Currency: "USD", Category: "CAT" + itoa(sIdx%20),
			Quantity: vendorspend.AvailableValue(float64(1 + i%50)), UnitPrice: vendorspend.AvailableValue(10 + float64(i%100)), UnitOfMeasure: "EA",
			Effect: vendorspend.EffectNormal, SpendType: vendorspend.SpendTypeGoods, Basis: vendorspend.BasisAccrual,
			ProductID: productIDs[pIdx], Department: "DEPT" + itoa(i%10),
		})
	}

	return periods, suppliers, records
}
