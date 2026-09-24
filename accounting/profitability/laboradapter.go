package profitability

import "github.com/themurtez/go-valuate/accounting/labor"

// DirectLaborFactFromPayrollRecord converts one accounting/labor
// PayrollRecord's employee-cost total into a single DIRECT_LABOR Fact,
// explicitly attributed to attributions — task section 40. This is a
// typed adapter, not a compile-time dependency baked into Calculate: this
// package's core types never import accounting/labor, and a caller with
// no labor package usage never needs this function.
//
// The caller supplies attributions explicitly — this adapter never
// infers a customer/job/product from PayrollRecord.Department/
// Location/CostCenter or any other field, and it computes no
// employee-performance, productivity, or evaluation metric of any kind
// (task section 40's explicit "no employee-performance semantics"
// instruction). The fact's Amount is PayrollRecord's full loaded cost
// (regular + overtime + bonus + commission + other pay, plus employer
// taxes/benefits/other employer cost) — the same "gross employee cost"
// total accounting/labor.LaborCostBridge.GrossEmployeePay +
// EmployerBurden computes for a whole population, applied here to one
// record.
func DirectLaborFactFromPayrollRecord(record labor.PayrollRecord, attributions []Attribution) Fact {
	amount := record.RegularPay + record.OvertimePay + record.BonusPay + record.CommissionPay + record.OtherPay +
		record.EmployerTaxes + record.BenefitsCost + record.OtherEmployerCost

	return Fact{
		FactID:       "LABOR-" + record.ID,
		Period:       record.Period,
		Date:         &record.PayDate,
		Component:    ComponentDirectLabor,
		Amount:       amount,
		Attributions: attributions,
		SourceType:   "accounting/labor.PayrollRecord",
		SourceID:     record.ID,
		Currency:     record.Currency,
	}
}

// DirectLaborFactFromContractorRecord converts one accounting/labor
// ContractorLaborRecord into a single DIRECT_SUBCONTRACTOR Fact,
// explicitly attributed to attributions — the contractor-labor
// counterpart to DirectLaborFactFromPayrollRecord. A caller may prefer
// DIRECT_LABOR instead for a contractor functionally indistinguishable
// from direct labor in their business; this adapter defaults to
// DIRECT_SUBCONTRACTOR since accounting/labor itself already
// distinguishes PayrollRecord (employee) from ContractorLaborRecord
// (contractor).
func DirectSubcontractorFactFromContractorRecord(record labor.ContractorLaborRecord, attributions []Attribution) Fact {
	return Fact{
		FactID:       "CONTRACTOR-" + record.ID,
		Period:       record.Period,
		Date:         &record.Date,
		Component:    ComponentDirectSubcontractor,
		Amount:       record.Amount,
		Attributions: attributions,
		SourceType:   "accounting/labor.ContractorLaborRecord",
		SourceID:     record.ID,
		Currency:     record.Currency,
	}
}
