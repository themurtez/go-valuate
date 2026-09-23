package labor

// ContractorMix is one period's employee-vs-contractor labor-cost split —
// the task's section 24. This package makes no worker-classification or
// legal conclusion about whether contractor labor is properly classified
// — see safety_test.go.
type ContractorMix struct {
	ContractorLabor float64 `json:"contractor_labor"`
	EmployeeLabor   float64 `json:"employee_labor"`
	// ContractorShareOfLaborCost is ContractorLabor / TotalLaborCost,
	// available only when TotalLaborCost is nonzero.
	ContractorShareOfLaborCost Value `json:"contractor_share_of_labor_cost"`
}

func buildContractorMix(b LaborCostBridge) ContractorMix {
	m := ContractorMix{ContractorLabor: b.ContractorLabor, EmployeeLabor: b.TotalEmployeeCost}
	if b.TotalLaborCost != 0 {
		m.ContractorShareOfLaborCost = AvailableValue(b.ContractorLabor / b.TotalLaborCost)
	}
	return m
}
