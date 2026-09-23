package labor

import "time"

// BasisView is one accounting-basis presentation of total labor
// cost/pay for a period — the task's section 8 "do not assume labor
// expense == cash paid in same period" requirement. ExpenseBasis groups
// records by their accrual Period field; CashBasis groups by the calendar
// month/period containing PayDate. When the caller's PayDate values all
// fall within their own Period (the common case for a caller not tracking
// the distinction), the two bases naturally coincide; this package never
// assumes they do and never invents an accrual adjustment to reconcile
// them — see the task's section 8's explicit "if only one basis is
// available, other basis unavailable" instruction.
type BasisView struct {
	Available      bool    `json:"available"`
	TotalLaborCost float64 `json:"total_labor_cost"`
}

// PayrollBasis bundles a period's expense-basis and cash-basis views. Both
// are always computable from PayrollRecord (Period drives ExpenseBasis,
// PayDate drives CashBasis) as long as at least one payroll record exists
// for the period under either grouping — see buildPayrollBasis.
type PayrollBasis struct {
	ExpenseBasis BasisView `json:"expense_basis"`
	CashBasis    BasisView `json:"cash_basis"`
}

// buildExpenseBasisByPeriod sums every payroll record's employee+employer
// cost (RegularPay..OtherEmployerCost, matching LaborCostBridge's
// TotalEmployeeCost component set) keyed by its accrual Period field.
// Contractor labor is intentionally excluded from cash/expense basis
// views since ContractorLaborRecord carries only one Date, not a separate
// accrual/cash distinction.
func buildExpenseBasisByPeriod(payroll []PayrollRecord) map[string]float64 {
	out := map[string]float64{}
	for _, r := range payroll {
		out[r.Period] += employeeCostTotal(r)
	}
	return out
}

// buildCashBasisByPeriod sums the same per-record total, keyed by the
// period label matching PayDate's occurrence — this requires the caller's
// PeriodInfo set to cover PayDate, which buildPayrollBasis resolves via
// periodForDate.
func buildCashBasisByPeriod(payroll []PayrollRecord, periods []PeriodInfo) map[string]float64 {
	out := map[string]float64{}
	for _, r := range payroll {
		label, ok := periodForDate(periods, r.PayDate)
		if !ok {
			continue
		}
		out[label] += employeeCostTotal(r)
	}
	return out
}

func employeeCostTotal(r PayrollRecord) float64 {
	return r.RegularPay + r.OvertimePay + r.BonusPay + r.CommissionPay + r.OtherPay +
		r.EmployerTaxes + r.BenefitsCost + r.OtherEmployerCost
}

// periodForDate returns the Period label of the PeriodInfo whose
// [StartDate, EndDate] window contains d, and true if found.
func periodForDate(periods []PeriodInfo, d time.Time) (string, bool) {
	for _, p := range periods {
		if !d.Before(p.StartDate) && !d.After(p.EndDate) {
			return p.Period, true
		}
	}
	return "", false
}

// buildPayrollBasis returns the ExpenseBasis/CashBasis view for one
// period label, given both basis maps.
func buildPayrollBasis(period string, expenseByPeriod, cashByPeriod map[string]float64) PayrollBasis {
	var basis PayrollBasis
	if v, ok := expenseByPeriod[period]; ok {
		basis.ExpenseBasis = BasisView{Available: true, TotalLaborCost: v}
	}
	if v, ok := cashByPeriod[period]; ok {
		basis.CashBasis = BasisView{Available: true, TotalLaborCost: v}
	}
	return basis
}
