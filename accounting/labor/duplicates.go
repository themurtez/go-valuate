package labor

// DuplicateGroup is one pair of PayrollRecords sharing the same worker,
// pay date, and every pay component amount — a conservative, factual
// signal, never asserted as a duplicate payment. See the task's section
// 38.
type DuplicateGroup struct {
	RecordIDA string `json:"record_id_a"`
	RecordIDB string `json:"record_id_b"`
	WorkerID  string `json:"worker_id"`
	PayDate   string `json:"pay_date"`
}

// economicSignature returns a comparable key for "same worker, same pay
// date, same component amounts" — the task's section 38 conservative
// possible-duplicate detection. IDs are excluded from the key
// deliberately (identical IDs are handled separately as
// IssueDuplicatePayrollRecord, not this economic-similarity check).
func economicSignature(r PayrollRecord) string {
	return r.WorkerID + "|" + r.PayDate.Format("2006-01-02") + "|" +
		formatAmount(r.RegularPay) + "|" + formatAmount(r.OvertimePay) + "|" +
		formatAmount(r.BonusPay) + "|" + formatAmount(r.CommissionPay) + "|" +
		formatAmount(r.OtherPay) + "|" + formatAmount(r.EmployerTaxes) + "|" +
		formatAmount(r.BenefitsCost) + "|" + formatAmount(r.OtherEmployerCost)
}

func formatAmount(v float64) string {
	// A simple, deterministic fixed-precision representation is sufficient
	// here — this key is only used for exact-match grouping, never
	// parsed back.
	return floatToFixedString(v, 2)
}

// floatToFixedString formats v with the given number of decimal places
// without relying on fmt (kept dependency-free and allocation-light for
// the large-N duplicate scan — see benchmark_test.go).
func floatToFixedString(v float64, decimals int) string {
	neg := v < 0
	if neg {
		v = -v
	}
	scale := 1.0
	for i := 0; i < decimals; i++ {
		scale *= 10
	}
	scaled := int64(v*scale + 0.5)
	whole := scaled / int64(scale)
	frac := scaled % int64(scale)
	s := itoa(whole) + "." + itoaPadded(frac, decimals)
	if neg {
		return "-" + s
	}
	return s
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func itoaPadded(v int64, width int) string {
	s := itoa(v)
	for len(s) < width {
		s = "0" + s
	}
	return s
}

// findPossibleDuplicatePayrollRecords scans payroll for pairs of distinct
// records sharing the same economic signature, returning one
// DuplicateGroup per pair found (grouped by signature, all pairwise
// combinations within a group of 2+ matching records). This is O(N) in
// the number of records (a single map pass) despite reporting O(k^2)
// pairs within a matching group, which is expected to be very small in
// practice.
func findPossibleDuplicatePayrollRecords(payroll []PayrollRecord) []DuplicateGroup {
	bySignature := map[string][]PayrollRecord{}
	for _, r := range payroll {
		sig := economicSignature(r)
		bySignature[sig] = append(bySignature[sig], r)
	}

	var groups []DuplicateGroup
	sigs := sortedStringKeys(bySignature)
	for _, sig := range sigs {
		records := bySignature[sig]
		if len(records) < 2 {
			continue
		}
		for i := 0; i < len(records); i++ {
			for j := i + 1; j < len(records); j++ {
				a, b := records[i].ID, records[j].ID
				if b < a {
					a, b = b, a
				}
				groups = append(groups, DuplicateGroup{
					RecordIDA: a, RecordIDB: b,
					WorkerID: records[i].WorkerID, PayDate: records[i].PayDate.Format("2006-01-02"),
				})
			}
		}
	}
	sortDuplicateGroups(groups)
	return groups
}
