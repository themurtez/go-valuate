package labor

// GroupSummary is one deterministic grouping dimension's (department,
// location, or cost center) period labor summary — the task's section 16.
// A few fixed dimensions, not a generic OLAP cube, per the task's explicit
// instruction.
type GroupSummary struct {
	// GroupKey is the raw Department/Location/CostCenter value this
	// summary was grouped by.
	GroupKey string `json:"group_key"`

	Headcount       int     `json:"headcount"`
	FTE             Value   `json:"fte"`
	GrossPay        float64 `json:"gross_pay"`
	EmployerBurden  float64 `json:"employer_burden"`
	ContractorLabor float64 `json:"contractor_labor"`
	TotalLaborCost  float64 `json:"total_labor_cost"`
	OvertimeHours   Value   `json:"overtime_hours"`

	// ShareOfTotalLaborCost is this group's TotalLaborCost as a fraction
	// of the period's overall TotalLaborCost — used for the task's section
	// 33 concentration-by-department reporting (top 1/top 3 are simply the
	// first entries of a summary slice sorted by TotalLaborCost
	// descending, which callers can derive themselves; this package does
	// not duplicate that sort here since GroupSummary's canonical order is
	// by normalized name, not by cost).
	ShareOfTotalLaborCost Value `json:"share_of_total_labor_cost"`
}

// groupKeyFunc extracts the grouping key from a PayrollRecord/
// ContractorLaborRecord.
type payrollGroupKeyFunc func(PayrollRecord) string
type contractorGroupKeyFunc func(ContractorLaborRecord) string

func departmentKey(r PayrollRecord) string                   { return r.Department }
func locationKey(r PayrollRecord) string                     { return r.Location }
func costCenterKey(r PayrollRecord) string                   { return r.CostCenter }
func contractorDepartmentKey(r ContractorLaborRecord) string { return r.Department }
func contractorLocationKey(r ContractorLaborRecord) string   { return r.Location }
func contractorCostCenterKey(r ContractorLaborRecord) string { return r.CostCenter }

// buildGroupSummaries groups payroll+contractor records by the given key
// functions, returning one GroupSummary per distinct non-empty key value,
// sorted by normalized group name.
func buildGroupSummaries(payroll []PayrollRecord, contractors []ContractorLaborRecord,
	payrollKey payrollGroupKeyFunc, contractorKey contractorGroupKeyFunc, workersByID map[string]Worker, standardHours float64) []GroupSummary {

	type acc struct {
		headcount       map[string]bool
		grossPay        float64
		employerBurden  float64
		contractorLabor float64
		hoursTotal      float64
		haveHours       bool
		otHours         float64
		haveOT          bool
	}
	groups := map[string]*acc{}
	ensure := func(k string) *acc {
		g, ok := groups[k]
		if !ok {
			g = &acc{headcount: map[string]bool{}}
			groups[k] = g
		}
		return g
	}

	var totalLaborCost float64

	for _, r := range payroll {
		k := payrollKey(r)
		if k == "" {
			continue
		}
		g := ensure(k)
		g.headcount[r.WorkerID] = true
		g.grossPay += r.RegularPay + r.OvertimePay + r.BonusPay + r.CommissionPay + r.OtherPay
		g.employerBurden += r.EmployerTaxes + r.BenefitsCost + r.OtherEmployerCost
		if r.HoursRegular.Available {
			g.hoursTotal += r.HoursRegular.Amount
			g.haveHours = true
		}
		if r.HoursOvertime.Available {
			g.hoursTotal += r.HoursOvertime.Amount
			g.haveHours = true
			g.otHours += r.HoursOvertime.Amount
			g.haveOT = true
		}
	}
	for _, r := range contractors {
		k := contractorKey(r)
		if k == "" {
			continue
		}
		g := ensure(k)
		g.contractorLabor += r.Amount
	}

	names := sortedStringKeys(groups)
	out := make([]GroupSummary, 0, len(names))
	for _, name := range names {
		g := groups[name]
		total := g.grossPay + g.employerBurden + g.contractorLabor
		totalLaborCost += total
		s := GroupSummary{
			GroupKey:        name,
			Headcount:       len(g.headcount),
			GrossPay:        g.grossPay,
			EmployerBurden:  g.employerBurden,
			ContractorLabor: g.contractorLabor,
			TotalLaborCost:  total,
		}
		if g.haveHours && standardHours > 0 {
			s.FTE = AvailableValue(g.hoursTotal / standardHours)
		}
		if g.haveOT {
			s.OvertimeHours = AvailableValue(g.otHours)
		}
		out = append(out, s)
	}

	if totalLaborCost != 0 {
		for i := range out {
			out[i].ShareOfTotalLaborCost = AvailableValue(out[i].TotalLaborCost / totalLaborCost)
		}
	}

	return out
}

// DirectIndirectSplit is the task's section 17 direct-vs-indirect labor
// summary, classified strictly from caller-supplied PayrollRecord.
// LaborClass — never inferred from department names.
type DirectIndirectSplit struct {
	DirectLaborCost       float64 `json:"direct_labor_cost"`
	IndirectLaborCost     float64 `json:"indirect_labor_cost"`
	UnclassifiedLaborCost float64 `json:"unclassified_labor_cost"`
	// DirectLaborPercent is DirectLaborCost / (Direct + Indirect +
	// Unclassified), available only when that denominator is nonzero.
	DirectLaborPercent Value `json:"direct_labor_percent"`
}

func buildDirectIndirectSplit(payroll []PayrollRecord) DirectIndirectSplit {
	var s DirectIndirectSplit
	for _, r := range payroll {
		cost := r.RegularPay + r.OvertimePay + r.BonusPay + r.CommissionPay + r.OtherPay +
			r.EmployerTaxes + r.BenefitsCost + r.OtherEmployerCost
		switch resolvedLaborClass(r.LaborClass) {
		case LaborClassDirect:
			s.DirectLaborCost += cost
		case LaborClassIndirect:
			s.IndirectLaborCost += cost
		default:
			s.UnclassifiedLaborCost += cost
		}
	}
	total := s.DirectLaborCost + s.IndirectLaborCost + s.UnclassifiedLaborCost
	if total != 0 {
		s.DirectLaborPercent = AvailableValue(s.DirectLaborCost / total)
	}
	return s
}
