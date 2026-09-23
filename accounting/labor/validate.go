package labor

import "sort"

// validatePeriods checks each PeriodInfo for structural validity, returns
// issues plus the deduplicated (first-occurrence-wins), chronologically
// sorted list of valid periods.
func validatePeriods(periods []PeriodInfo) ([]Issue, []PeriodInfo) {
	var issues []Issue
	seen := map[string]bool{}
	var valid []PeriodInfo

	for _, p := range periods {
		if !p.Valid() {
			issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: SeverityError,
				Message: "period has an empty label, missing dates, or end date before start date", Period: p.Period})
			continue
		}
		if seen[p.Period] {
			issues = append(issues, Issue{Code: IssueDuplicatePeriod, Severity: SeverityError,
				Message: "duplicate period label: " + p.Period, Period: p.Period})
			continue
		}
		seen[p.Period] = true
		valid = append(valid, p)
	}

	sort.SliceStable(valid, func(i, j int) bool { return valid[i].StartDate.Before(valid[j].StartDate) })
	return issues, valid
}

// validateWorkers checks each Worker for structural validity, returns
// issues plus the deduplicated (first-occurrence-wins) valid worker set,
// keyed by WorkerID for downstream lookups.
func validateWorkers(workers []Worker) ([]Issue, map[string]Worker, []string) {
	var issues []Issue
	seen := map[string]bool{}
	byID := map[string]Worker{}
	var order []string

	for _, w := range workers {
		if w.WorkerID == "" {
			issues = append(issues, Issue{Code: IssueMissingWorkerID, Severity: SeverityError,
				Message: "worker missing worker_id"})
			continue
		}
		if seen[w.WorkerID] {
			issues = append(issues, Issue{Code: IssueDuplicateWorker, Severity: SeverityError,
				Message: "duplicate worker ID: " + w.WorkerID, WorkerID: w.WorkerID})
			continue
		}
		seen[w.WorkerID] = true

		if w.WorkerType != "" && !isRecognizedWorkerType(w.WorkerType) {
			issues = append(issues, Issue{Code: IssueMissingWorkerID, Severity: SeverityWarning,
				Message: "unrecognized worker_type", WorkerID: w.WorkerID})
		}
		if w.HireDate != nil && w.TerminationDate != nil && w.TerminationDate.Before(*w.HireDate) {
			issues = append(issues, Issue{Code: IssueInvalidWorkerDates, Severity: SeverityWarning,
				Message: "termination date precedes hire date", WorkerID: w.WorkerID})
		}

		byID[w.WorkerID] = w
		order = append(order, w.WorkerID)
	}

	return issues, byID, order
}

// validatePayrollRecords checks each PayrollRecord for structural
// validity. Returns issues plus the deduplicated (first-occurrence-wins),
// non-excluded record set. workersKnown is true only when Input.Workers
// was actually supplied (so IssueUnknownWorker is skipped entirely when
// no roster exists at all — the task's explicit "worker roster is
// optional" design).
func validatePayrollRecords(records []PayrollRecord, workers map[string]Worker, workersKnown bool) ([]Issue, []PayrollRecord) {
	var issues []Issue
	seen := map[string]bool{}
	var valid []PayrollRecord

	for _, r := range records {
		if r.ID == "" {
			issues = append(issues, Issue{Code: IssueMissingWorkerID, Severity: SeverityError,
				Message: "payroll record missing id"})
			continue
		}
		if seen[r.ID] {
			issues = append(issues, Issue{Code: IssueDuplicatePayrollRecord, Severity: SeverityError,
				Message: "duplicate payroll record ID: " + r.ID, RecordID: r.ID})
			continue
		}
		seen[r.ID] = true

		if r.WorkerID == "" {
			issues = append(issues, Issue{Code: IssueMissingWorkerID, Severity: SeverityError,
				Message: "payroll record missing worker_id", RecordID: r.ID})
			continue
		}
		if workersKnown {
			if _, ok := workers[r.WorkerID]; !ok {
				issues = append(issues, Issue{Code: IssueUnknownWorker, Severity: SeverityWarning,
					Message: "payroll record references a worker not present in the supplied roster", RecordID: r.ID, WorkerID: r.WorkerID})
			}
		}
		if r.PayDate.IsZero() {
			issues = append(issues, Issue{Code: IssueInvalidDate, Severity: SeverityError,
				Message: "payroll record missing pay_date", RecordID: r.ID, WorkerID: r.WorkerID})
			continue
		}
		if r.Period == "" {
			issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: SeverityError,
				Message: "payroll record missing period", RecordID: r.ID, WorkerID: r.WorkerID})
			continue
		}

		amounts := []float64{r.RegularPay, r.OvertimePay, r.BonusPay, r.CommissionPay, r.OtherPay,
			r.EmployerTaxes, r.BenefitsCost, r.OtherEmployerCost}
		nonFinite := false
		for _, a := range amounts {
			if isNonFinite(a) {
				nonFinite = true
				break
			}
		}
		if r.HoursRegular.Available && isNonFinite(r.HoursRegular.Amount) {
			nonFinite = true
		}
		if r.HoursOvertime.Available && isNonFinite(r.HoursOvertime.Amount) {
			nonFinite = true
		}
		if nonFinite {
			issues = append(issues, Issue{Code: IssueNonFiniteAmount, Severity: SeverityError,
				Message: "payroll record has a non-finite amount or hours value", RecordID: r.ID, WorkerID: r.WorkerID})
			continue
		}

		if resolvedRecordType(r.RecordType) == RecordTypeNormal {
			negative := false
			for _, a := range amounts {
				if a < 0 {
					negative = true
					break
				}
			}
			if negative {
				issues = append(issues, Issue{Code: IssueNegativeAmount, Severity: SeverityError,
					Message: "payroll record has a negative labor-cost component without an explicit REVERSAL/ADJUSTMENT record_type", RecordID: r.ID, WorkerID: r.WorkerID})
				continue
			}
		}

		if r.HoursRegular.Available && r.HoursRegular.Amount < 0 {
			issues = append(issues, Issue{Code: IssueInvalidHours, Severity: SeverityError,
				Message: "payroll record has negative hours_regular", RecordID: r.ID, WorkerID: r.WorkerID})
			continue
		}
		if r.HoursOvertime.Available && r.HoursOvertime.Amount < 0 {
			issues = append(issues, Issue{Code: IssueInvalidHours, Severity: SeverityError,
				Message: "payroll record has negative hours_overtime", RecordID: r.ID, WorkerID: r.WorkerID})
			continue
		}

		valid = append(valid, r)
	}

	return issues, valid
}

// validateContractorRecords mirrors validatePayrollRecords for
// ContractorLaborRecord.
func validateContractorRecords(records []ContractorLaborRecord) ([]Issue, []ContractorLaborRecord) {
	var issues []Issue
	seen := map[string]bool{}
	var valid []ContractorLaborRecord

	for _, r := range records {
		if r.ID == "" {
			issues = append(issues, Issue{Code: IssueMissingWorkerID, Severity: SeverityError,
				Message: "contractor labor record missing id"})
			continue
		}
		if seen[r.ID] {
			issues = append(issues, Issue{Code: IssueDuplicateContractorRecord, Severity: SeverityError,
				Message: "duplicate contractor labor record ID: " + r.ID, RecordID: r.ID})
			continue
		}
		seen[r.ID] = true

		if r.ContractorID == "" {
			issues = append(issues, Issue{Code: IssueMissingWorkerID, Severity: SeverityError,
				Message: "contractor labor record missing contractor_id", RecordID: r.ID})
			continue
		}
		if r.Date.IsZero() {
			issues = append(issues, Issue{Code: IssueInvalidDate, Severity: SeverityError,
				Message: "contractor labor record missing date", RecordID: r.ID, WorkerID: r.ContractorID})
			continue
		}
		if r.Period == "" {
			issues = append(issues, Issue{Code: IssueInvalidPeriod, Severity: SeverityError,
				Message: "contractor labor record missing period", RecordID: r.ID, WorkerID: r.ContractorID})
			continue
		}
		if isNonFinite(r.Amount) || (r.Hours.Available && isNonFinite(r.Hours.Amount)) {
			issues = append(issues, Issue{Code: IssueNonFiniteAmount, Severity: SeverityError,
				Message: "contractor labor record has a non-finite amount or hours value", RecordID: r.ID, WorkerID: r.ContractorID})
			continue
		}
		if r.Amount < 0 {
			issues = append(issues, Issue{Code: IssueNegativeAmount, Severity: SeverityError,
				Message: "contractor labor record has a negative amount", RecordID: r.ID, WorkerID: r.ContractorID})
			continue
		}
		if r.Hours.Available && r.Hours.Amount < 0 {
			issues = append(issues, Issue{Code: IssueInvalidHours, Severity: SeverityError,
				Message: "contractor labor record has negative hours", RecordID: r.ID, WorkerID: r.ContractorID})
			continue
		}

		valid = append(valid, r)
	}

	return issues, valid
}

// resolveReportingCurrency determines the single currency an analysis
// proceeds under: the most common currency among included records (ties
// broken by currency code ascending). Returns ("", nil) if there are no
// included records with a currency at all. Mirrors ar/ap's identical
// resolution algorithm.
func resolveReportingCurrency(payroll []PayrollRecord, contractors []ContractorLaborRecord) (string, []Issue) {
	counts := map[string]int{}
	for _, r := range payroll {
		if r.Currency != "" {
			counts[r.Currency]++
		}
	}
	for _, r := range contractors {
		if r.Currency != "" {
			counts[r.Currency]++
		}
	}
	if len(counts) == 0 {
		return "", nil
	}

	codes := make([]string, 0, len(counts))
	for c := range counts {
		codes = append(codes, c)
	}
	sort.Strings(codes)

	best := codes[0]
	for _, c := range codes[1:] {
		if counts[c] > counts[best] {
			best = c
		}
	}

	if len(codes) > 1 {
		return best, []Issue{{Code: IssueMixedCurrency, Severity: SeverityWarning,
			Message: "records use more than one currency; only " + best + " included in aggregate totals"}}
	}
	return best, nil
}

// filterByCurrency excludes records whose Currency does not match
// reporting (empty Currency is treated as matching — this package does
// not require Currency on every record, only flags a genuine mismatch).
func filterPayrollByCurrency(records []PayrollRecord, reporting string) []PayrollRecord {
	if reporting == "" {
		return records
	}
	var out []PayrollRecord
	for _, r := range records {
		if r.Currency == "" || r.Currency == reporting {
			out = append(out, r)
		}
	}
	return out
}

func filterContractorByCurrency(records []ContractorLaborRecord, reporting string) []ContractorLaborRecord {
	if reporting == "" {
		return records
	}
	var out []ContractorLaborRecord
	for _, r := range records {
		if r.Currency == "" || r.Currency == reporting {
			out = append(out, r)
		}
	}
	return out
}

// validateBusinessMetrics checks each BusinessMetrics entry.
func validateBusinessMetrics(metrics []BusinessMetrics) []Issue {
	var issues []Issue
	for _, m := range metrics {
		if m.Revenue.Available && isNonFinite(m.Revenue.Amount) {
			issues = append(issues, Issue{Code: IssueInvalidFinancialMetric, Severity: SeverityError,
				Message: "business metrics revenue is non-finite", Period: m.Period})
		}
		if m.GrossProfit.Available && isNonFinite(m.GrossProfit.Amount) {
			issues = append(issues, Issue{Code: IssueInvalidFinancialMetric, Severity: SeverityError,
				Message: "business metrics gross profit is non-finite", Period: m.Period})
		}
		if m.EBITDA.Available && isNonFinite(m.EBITDA.Amount) {
			issues = append(issues, Issue{Code: IssueInvalidFinancialMetric, Severity: SeverityError,
				Message: "business metrics EBITDA is non-finite", Period: m.Period})
		}
	}
	return issues
}

// validateGLControls checks each GLPayrollControl entry.
func validateGLControls(controls []GLPayrollControl) []Issue {
	var issues []Issue
	for _, c := range controls {
		vals := []Value{c.GrossWages, c.EmployerTaxes, c.Benefits, c.ContractorLabor, c.OtherLabor, c.TotalLaborCost}
		for _, v := range vals {
			if v.Available && isNonFinite(v.Amount) {
				issues = append(issues, Issue{Code: IssueInvalidGLControl, Severity: SeverityError,
					Message: "GL payroll control has a non-finite value", Period: c.Period})
				break
			}
		}
	}
	return issues
}

// dedupeAndSortIssues sorts issues deterministically by Code, then
// RecordID, then WorkerID, then Period — never Go map order.
func dedupeAndSortIssues(issues []Issue) []Issue {
	out := append([]Issue{}, issues...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		ra, rb := issueRank(a.Code), issueRank(b.Code)
		if ra != rb {
			return ra < rb
		}
		if a.RecordID != b.RecordID {
			return a.RecordID < b.RecordID
		}
		if a.WorkerID != b.WorkerID {
			return a.WorkerID < b.WorkerID
		}
		return a.Period < b.Period
	})
	return out
}
