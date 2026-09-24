package vendorspend

import "github.com/themurtez/go-valuate/accounting/labor"

// SpendRecordsFromContractorLabor converts accounting/labor
// ContractorLaborRecords into SpendRecords under SpendTypeSubcontractor,
// using contractorSupplierID to map each record's ContractorID to this
// package's SupplierID — task section 27's "only under explicit caller
// identity mapping" rule; a ContractorID absent from contractorSupplierID
// is skipped (never guessed to be its own SupplierID, which would
// silently assume the two identifier spaces are the same).
//
// Double-counting: if the same contractor spend is ALSO already present
// in the caller's own Input.SpendRecords (e.g. because it was entered
// directly as a SUBCONTRACTOR-type record from AP data), converting it
// again from accounting/labor and appending both to one Input.SpendRecords
// slice double-counts it. This function does not itself detect that — a
// caller combining both sources supplies exactly one of them per
// contractor engagement, or assigns each source a disjoint SpendID
// namespace and only ever calls Calculate with the union it intends
// (mirrors accounting/profitability.DirectLaborFactFromPayrollRecord's
// identical "the caller owns de-duplication across sources" rule).
func SpendRecordsFromContractorLabor(records []labor.ContractorLaborRecord, contractorSupplierID map[string]string) []SpendRecord {
	out := make([]SpendRecord, 0, len(records))
	for _, r := range records {
		supplierID, ok := contractorSupplierID[r.ContractorID]
		if !ok || supplierID == "" {
			continue
		}
		out = append(out, SpendRecord{
			SpendID:    r.ID,
			SupplierID: supplierID,
			Period:     r.Period,
			Date:       r.Date,
			Amount:     r.Amount,
			Currency:   r.Currency,
			Effect:     EffectNormal,
			SpendType:  SpendTypeSubcontractor,
			Department: r.Department,
			Location:   r.Location,
			CostCenter: r.CostCenter,
		})
	}
	return out
}
