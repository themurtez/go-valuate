package labor

// OvertimeSummary is one period's regular-vs-overtime hours/pay
// breakdown — the task's section 12. Available only when at least one
// included payroll record in the period supplies hours.
type OvertimeSummary struct {
	Available bool `json:"available"`

	RegularHours  float64 `json:"regular_hours"`
	OvertimeHours float64 `json:"overtime_hours"`
	TotalHours    float64 `json:"total_hours"`

	// OvertimeHoursPercent is OvertimeHours / TotalHours, available only
	// when TotalHours is nonzero.
	OvertimeHoursPercent Value `json:"overtime_hours_percent"`
	// OvertimePayPercent is OvertimePay / (RegularPay + OvertimePay),
	// available only when that denominator is nonzero. This package never
	// determines whether overtime was legally required or correctly
	// paid — analytics only.
	OvertimePayPercent Value `json:"overtime_pay_percent"`
}

func buildOvertimeSummary(payroll []PayrollRecord) OvertimeSummary {
	var regHours, otHours, regPay, otPay float64
	var haveHours bool
	for _, r := range payroll {
		if r.HoursRegular.Available {
			regHours += r.HoursRegular.Amount
			haveHours = true
		}
		if r.HoursOvertime.Available {
			otHours += r.HoursOvertime.Amount
			haveHours = true
		}
		regPay += r.RegularPay
		otPay += r.OvertimePay
	}
	if !haveHours {
		return OvertimeSummary{}
	}
	total := regHours + otHours
	s := OvertimeSummary{Available: true, RegularHours: regHours, OvertimeHours: otHours, TotalHours: total}
	if total != 0 {
		s.OvertimeHoursPercent = AvailableValue(otHours / total)
	}
	if payDenom := regPay + otPay; payDenom != 0 {
		s.OvertimePayPercent = AvailableValue(otPay / payDenom)
	}
	return s
}

// OvertimeByDepartment reports one department's overtime hours share —
// used to detect concentration, the task's section 23.
type OvertimeByDepartment struct {
	Department           string  `json:"department"`
	OvertimeHours        float64 `json:"overtime_hours"`
	ShareOfTotalOvertime Value   `json:"share_of_total_overtime"`
}

// buildOvertimeByDepartment groups overtime hours by department (only
// records with a non-empty Department and available overtime hours
// contribute), sorted by normalized department name.
func buildOvertimeByDepartment(payroll []PayrollRecord) []OvertimeByDepartment {
	totals := map[string]float64{}
	var grandTotal float64
	for _, r := range payroll {
		if r.Department == "" || !r.HoursOvertime.Available {
			continue
		}
		totals[r.Department] += r.HoursOvertime.Amount
		grandTotal += r.HoursOvertime.Amount
	}
	if len(totals) == 0 {
		return nil
	}
	names := sortedStringKeys(totals)
	out := make([]OvertimeByDepartment, 0, len(names))
	for _, name := range names {
		v := OvertimeByDepartment{Department: name, OvertimeHours: totals[name]}
		if grandTotal != 0 {
			v.ShareOfTotalOvertime = AvailableValue(totals[name] / grandTotal)
		}
		out = append(out, v)
	}
	return out
}
