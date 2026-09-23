package labor

// LaborCostBridge is one period's deterministic labor-cost bridge — the
// task's section 7 identity:
//
//	RegularPay + OvertimePay + BonusPay + CommissionPay + OtherPay = GrossEmployeePay
//	GrossEmployeePay + EmployerTaxes + Benefits + OtherEmployerCosts = TotalEmployeeCost
//	TotalEmployeeCost + ContractorLabor = TotalLaborCost
//
// Every component is a caller-supplied fact — this package never derives
// one component from another. Employee deductions are never included as
// employer labor cost; only EmployerTaxes/BenefitsCost/OtherEmployerCost
// (fields the caller must supply as employer expense) contribute to
// EmployerBurden — see the task's section 7's explicit "do not include
// employee deductions as employer labor cost" instruction.
type LaborCostBridge struct {
	RegularPay       float64 `json:"regular_pay"`
	OvertimePay      float64 `json:"overtime_pay"`
	BonusPay         float64 `json:"bonus_pay"`
	CommissionPay    float64 `json:"commission_pay"`
	OtherPay         float64 `json:"other_pay"`
	GrossEmployeePay float64 `json:"gross_employee_pay"`

	EmployerTaxes      float64 `json:"employer_taxes"`
	Benefits           float64 `json:"benefits"`
	OtherEmployerCosts float64 `json:"other_employer_costs"`
	// EmployerBurden is EmployerTaxes + Benefits + OtherEmployerCosts —
	// see the task's section 26.
	EmployerBurden    float64 `json:"employer_burden"`
	TotalEmployeeCost float64 `json:"total_employee_cost"`

	ContractorLabor float64 `json:"contractor_labor"`
	TotalLaborCost  float64 `json:"total_labor_cost"`

	// BurdenRate is EmployerBurden / GrossEmployeePay when GrossEmployeePay
	// is nonzero — an observed accounting ratio, not a statutory payroll
	// burden calculation. See the task's section 26.
	BurdenRate Value `json:"burden_rate"`

	// CostMix is the percentage-of-TotalLaborCost breakdown, available only
	// when TotalLaborCost is nonzero.
	CostMix CostMix `json:"cost_mix"`
}

// CostMix is the percentage mix of LaborCostBridge components as a
// fraction (0-1) of TotalLaborCost — the task's section 9 "return
// percentage mix where denominator is available" requirement. Each field
// is independently Value-wrapped so a caller can distinguish "0%" from
// "unavailable" even though in practice all fields share the same
// denominator-availability.
type CostMix struct {
	RegularPayPercent      Value `json:"regular_pay_percent"`
	OvertimePayPercent     Value `json:"overtime_pay_percent"`
	BonusPayPercent        Value `json:"bonus_pay_percent"`
	CommissionPayPercent   Value `json:"commission_pay_percent"`
	OtherPayPercent        Value `json:"other_pay_percent"`
	EmployerBurdenPercent  Value `json:"employer_burden_percent"`
	ContractorLaborPercent Value `json:"contractor_labor_percent"`
}

// buildLaborCostBridge sums payroll and contractor records for one period
// into a LaborCostBridge. No NaN/Inf can reach the output because callers
// (buildPeriodSummary) only pass already-validated records.
func buildLaborCostBridge(payroll []PayrollRecord, contractors []ContractorLaborRecord) LaborCostBridge {
	var b LaborCostBridge
	for _, r := range payroll {
		b.RegularPay += r.RegularPay
		b.OvertimePay += r.OvertimePay
		b.BonusPay += r.BonusPay
		b.CommissionPay += r.CommissionPay
		b.OtherPay += r.OtherPay
		b.EmployerTaxes += r.EmployerTaxes
		b.Benefits += r.BenefitsCost
		b.OtherEmployerCosts += r.OtherEmployerCost
	}
	for _, r := range contractors {
		b.ContractorLabor += r.Amount
	}

	b.GrossEmployeePay = b.RegularPay + b.OvertimePay + b.BonusPay + b.CommissionPay + b.OtherPay
	b.EmployerBurden = b.EmployerTaxes + b.Benefits + b.OtherEmployerCosts
	b.TotalEmployeeCost = b.GrossEmployeePay + b.EmployerBurden
	b.TotalLaborCost = b.TotalEmployeeCost + b.ContractorLabor

	if b.GrossEmployeePay != 0 {
		b.BurdenRate = AvailableValue(b.EmployerBurden / b.GrossEmployeePay)
	}

	if b.TotalLaborCost != 0 {
		b.CostMix = CostMix{
			RegularPayPercent:      AvailableValue(b.RegularPay / b.TotalLaborCost),
			OvertimePayPercent:     AvailableValue(b.OvertimePay / b.TotalLaborCost),
			BonusPayPercent:        AvailableValue(b.BonusPay / b.TotalLaborCost),
			CommissionPayPercent:   AvailableValue(b.CommissionPay / b.TotalLaborCost),
			OtherPayPercent:        AvailableValue(b.OtherPay / b.TotalLaborCost),
			EmployerBurdenPercent:  AvailableValue(b.EmployerBurden / b.TotalLaborCost),
			ContractorLaborPercent: AvailableValue(b.ContractorLabor / b.TotalLaborCost),
		}
	}

	return b
}

// VariableCompensation is the task's section 25 bonus/commission/other
// variable-pay summary.
type VariableCompensation struct {
	TotalVariablePay            float64 `json:"total_variable_pay"`
	VariableCompPercentGrossPay Value   `json:"variable_comp_percent_gross_pay"`
}

func buildVariableCompensation(b LaborCostBridge) VariableCompensation {
	total := b.BonusPay + b.CommissionPay + b.OtherPay
	v := VariableCompensation{TotalVariablePay: total}
	if b.GrossEmployeePay != 0 {
		v.VariableCompPercentGrossPay = AvailableValue(total / b.GrossEmployeePay)
	}
	return v
}
