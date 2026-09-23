package labor

// AverageLaborCost is the task's section 13 per-employee/per-FTE/
// per-worker average metrics. Every denominator is explicit in the field
// name so employee and contractor denominators are never mixed without
// labeling — see the task's section 13.
type AverageLaborCost struct {
	GrossPayPerEmployee          Value `json:"gross_pay_per_employee"`
	TotalEmployeeCostPerEmployee Value `json:"total_employee_cost_per_employee"`
	// TotalLaborCostPerWorker divides TotalLaborCost by
	// (ActiveEmployeeCount + ActiveContractorCount) — the only average
	// here that intentionally spans both denominators, made explicit by
	// name.
	TotalLaborCostPerWorker Value `json:"total_labor_cost_per_worker"`
	TotalLaborCostPerFTE    Value `json:"total_labor_cost_per_fte"`
}

func buildAverageLaborCost(b LaborCostBridge, headcount Headcount, fte FTESummary) AverageLaborCost {
	var a AverageLaborCost
	if headcount.Available && headcount.ActiveEmployeeCount > 0 {
		n := float64(headcount.ActiveEmployeeCount)
		a.GrossPayPerEmployee = AvailableValue(b.GrossEmployeePay / n)
		a.TotalEmployeeCostPerEmployee = AvailableValue(b.TotalEmployeeCost / n)
	}
	if headcount.Available {
		total := headcount.ActiveEmployeeCount + headcount.ActiveContractorCount
		if total > 0 {
			a.TotalLaborCostPerWorker = AvailableValue(b.TotalLaborCost / float64(total))
		}
	}
	if fte.Available && fte.FTE != 0 {
		a.TotalLaborCostPerFTE = AvailableValue(b.TotalLaborCost / fte.FTE)
	}
	return a
}

// Productivity is the task's section 19 revenue/gross-profit/EBITDA per
// employee or FTE, plus labor-cost-as-percent-of-revenue ratios. Every
// field is unavailable (never Inf/NaN) when its denominator is zero or
// unavailable.
type Productivity struct {
	RevenuePerEmployee     Value `json:"revenue_per_employee"`
	RevenuePerFTE          Value `json:"revenue_per_fte"`
	GrossProfitPerEmployee Value `json:"gross_profit_per_employee"`
	GrossProfitPerFTE      Value `json:"gross_profit_per_fte"`
	EBITDAPerEmployee      Value `json:"ebitda_per_employee"`
	EBITDAPerFTE           Value `json:"ebitda_per_fte"`

	LaborCostPercentRevenue      Value `json:"labor_cost_percent_revenue"`
	EmployeeCostPercentRevenue   Value `json:"employee_cost_percent_revenue"`
	ContractorCostPercentRevenue Value `json:"contractor_cost_percent_revenue"`
	DirectLaborPercentRevenue    Value `json:"direct_labor_percent_revenue"`
}

func perDenominator(numerator Value, denominator float64, denominatorAvailable bool) Value {
	if !numerator.Available || !denominatorAvailable || denominator == 0 {
		return Unavailable()
	}
	return AvailableValue(numerator.Amount / denominator)
}

func buildProductivity(b LaborCostBridge, split DirectIndirectSplit, headcount Headcount, fte FTESummary, metrics BusinessMetrics) Productivity {
	var p Productivity

	employeeCount := float64(headcount.ActiveEmployeeCount)
	haveEmployees := headcount.Available && headcount.ActiveEmployeeCount > 0
	haveFTE := fte.Available && fte.FTE != 0

	p.RevenuePerEmployee = perDenominator(metrics.Revenue, employeeCount, haveEmployees)
	p.RevenuePerFTE = perDenominator(metrics.Revenue, fte.FTE, haveFTE)
	p.GrossProfitPerEmployee = perDenominator(metrics.GrossProfit, employeeCount, haveEmployees)
	p.GrossProfitPerFTE = perDenominator(metrics.GrossProfit, fte.FTE, haveFTE)
	p.EBITDAPerEmployee = perDenominator(metrics.EBITDA, employeeCount, haveEmployees)
	p.EBITDAPerFTE = perDenominator(metrics.EBITDA, fte.FTE, haveFTE)

	if metrics.Revenue.Available && metrics.Revenue.Amount != 0 {
		p.LaborCostPercentRevenue = AvailableValue(b.TotalLaborCost / metrics.Revenue.Amount)
		p.EmployeeCostPercentRevenue = AvailableValue(b.TotalEmployeeCost / metrics.Revenue.Amount)
		p.ContractorCostPercentRevenue = AvailableValue(b.ContractorLabor / metrics.Revenue.Amount)
		p.DirectLaborPercentRevenue = AvailableValue(split.DirectLaborCost / metrics.Revenue.Amount)
	}

	return p
}
