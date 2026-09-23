package labor

// ReconciliationStatus is the overall payroll-register-vs-GL comparison
// outcome for a period — the task's section 28.
type ReconciliationStatus string

const (
	ReconciliationStatusReconciled                ReconciliationStatus = "RECONCILED"
	ReconciliationStatusReconciledWithDifferences ReconciliationStatus = "RECONCILED_WITH_DIFFERENCES"
	ReconciliationStatusUnreconciled              ReconciliationStatus = "UNRECONCILED"
	ReconciliationStatusUnavailable               ReconciliationStatus = "UNAVAILABLE"
)

// ComponentReconciliation is one labor-cost component's register-vs-GL
// comparison — the task's section 59 "do not hide component-level
// differences behind one total" requirement.
type ComponentReconciliation struct {
	Component  string  `json:"component"`
	Register   Value   `json:"register"`
	GL         Value   `json:"gl"`
	Difference Value   `json:"difference"`
	Tolerance  float64 `json:"tolerance"`
	Reconciled bool    `json:"reconciled"`
}

// ReconciliationSummary is one period's full payroll-register-to-GL
// reconciliation — the task's sections 27-28.
type ReconciliationSummary struct {
	Status     ReconciliationStatus      `json:"status"`
	Components []ComponentReconciliation `json:"components,omitempty"`
}

// reconciliationTolerance resolves the absolute tolerance used to compare
// one component: the caller's explicit ReconciliationTolerance.
// AbsoluteTolerance if set (optionally combined with PercentTolerance
// against the GL amount), otherwise Policy.MaterialPayrollDifference,
// otherwise 0 (exact match required).
func reconciliationTolerance(glAmount float64, policy Policy) float64 {
	tol := policy.ReconciliationTolerance.AbsoluteTolerance
	if policy.ReconciliationTolerance.PercentTolerance > 0 {
		pctTol := policy.ReconciliationTolerance.PercentTolerance * absFloat(glAmount)
		if pctTol > tol {
			tol = pctTol
		}
	}
	if tol == 0 {
		tol = policy.MaterialPayrollDifference
	}
	return tol
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func compareComponent(name string, register Value, gl Value, policy Policy) ComponentReconciliation {
	c := ComponentReconciliation{Component: name, Register: register, GL: gl}
	if !register.Available || !gl.Available {
		return c
	}
	c.Tolerance = reconciliationTolerance(gl.Amount, policy)
	diff := register.Amount - gl.Amount
	c.Difference = AvailableValue(diff)
	c.Reconciled = absFloat(diff) <= c.Tolerance
	return c
}

// buildReconciliation compares a period's LaborCostBridge-derived register
// totals against the caller-supplied GLPayrollControl for the same
// period, when both are available.
func buildReconciliation(b LaborCostBridge, gl *GLPayrollControl, policy Policy) ReconciliationSummary {
	if gl == nil {
		return ReconciliationSummary{Status: ReconciliationStatusUnavailable}
	}

	components := []ComponentReconciliation{
		compareComponent("gross_wages", AvailableValue(b.GrossEmployeePay), gl.GrossWages, policy),
		compareComponent("employer_taxes", AvailableValue(b.EmployerTaxes), gl.EmployerTaxes, policy),
		compareComponent("benefits", AvailableValue(b.Benefits), gl.Benefits, policy),
		compareComponent("contractor_labor", AvailableValue(b.ContractorLabor), gl.ContractorLabor, policy),
		compareComponent("other_labor", AvailableValue(b.OtherEmployerCosts), gl.OtherLabor, policy),
		compareComponent("total_labor_cost", AvailableValue(b.TotalLaborCost), gl.TotalLaborCost, policy),
	}

	var anyCompared, anyReconciled, anyUnreconciled bool
	for _, c := range components {
		if !c.Register.Available || !c.GL.Available {
			continue
		}
		anyCompared = true
		if c.Reconciled {
			anyReconciled = true
		} else {
			anyUnreconciled = true
		}
	}

	status := ReconciliationStatusUnavailable
	switch {
	case !anyCompared:
		status = ReconciliationStatusUnavailable
	case anyUnreconciled && anyReconciled:
		status = ReconciliationStatusReconciledWithDifferences
	case anyUnreconciled:
		status = ReconciliationStatusUnreconciled
	default:
		status = ReconciliationStatusReconciled
	}

	return ReconciliationSummary{Status: status, Components: components}
}
