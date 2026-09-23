package labor

import "sort"

// Input bundles every caller-supplied source Calculate needs. Only
// Periods is effectively required for any output at all; every other
// slice is optional and Calculate computes whatever subset of the
// analysis the supplied input supports — see the task's section 2 "two
// supported input levels" (this package converges both the detailed
// worker/payroll-register path and any caller-pre-aggregated data onto
// one internal period-level representation via PayrollRecord/
// ContractorLaborRecord, which a caller populating from period summaries
// can construct as one synthetic record per period/category rather than
// true worker-level rows).
type Input struct {
	// Periods is the explicit set of periods to analyze. Required for any
	// PeriodSummary output.
	Periods []PeriodInfo `json:"periods"`

	// Workers is the optional worker/workforce roster. Headcount,
	// workforce movement, turnover, worker-date consistency, and
	// key-worker concentration are all unavailable without it (unless
	// Policy.PayrollActiveWorkerApproximation is set for a
	// headcount-only approximation).
	Workers []Worker `json:"workers,omitempty"`

	PayrollRecords    []PayrollRecord         `json:"payroll_records,omitempty"`
	ContractorRecords []ContractorLaborRecord `json:"contractor_records,omitempty"`

	// WorkerPeriodFTEs is optional caller-supplied FTE data — see fte.go.
	WorkerPeriodFTEs []WorkerPeriodFTE `json:"worker_period_ftes,omitempty"`

	// BusinessMetrics is optional per-period Revenue/GrossProfit/EBITDA,
	// used only for Productivity and the labor-cost-vs-revenue trend
	// signals.
	BusinessMetrics []BusinessMetrics `json:"business_metrics,omitempty"`

	// GLControls is optional per-period GL payroll control balances, used
	// only for ReconciliationSummary.
	GLControls []GLPayrollControl `json:"gl_controls,omitempty"`
}

// Calculate derives a full Result from in under policy. It never mutates
// any slice/map within in or policy, and performs no I/O — see
// immutability_test.go.
func Calculate(in Input, policy Policy) Result {
	policy = resolvePolicy(policy)

	var issues []Issue
	issues = append(issues, validatePolicy(policy)...)

	periodIssues, periods := validatePeriods(in.Periods)
	issues = append(issues, periodIssues...)

	workerIssues, workersByID, _ := validateWorkers(in.Workers)
	issues = append(issues, workerIssues...)
	workersKnown := len(in.Workers) > 0

	payrollIssues, validPayroll := validatePayrollRecords(in.PayrollRecords, workersByID, workersKnown)
	issues = append(issues, payrollIssues...)

	contractorIssues, validContractors := validateContractorRecords(in.ContractorRecords)
	issues = append(issues, contractorIssues...)

	reportingCurrency, currencyIssues := resolveReportingCurrency(validPayroll, validContractors)
	issues = append(issues, currencyIssues...)
	validPayroll = filterPayrollByCurrency(validPayroll, reportingCurrency)
	validContractors = filterContractorByCurrency(validContractors, reportingCurrency)

	fteIssues := validateWorkerPeriodFTEs(in.WorkerPeriodFTEs)
	issues = append(issues, fteIssues...)

	metricsIssues := validateBusinessMetrics(in.BusinessMetrics)
	issues = append(issues, metricsIssues...)

	glIssues := validateGLControls(in.GLControls)
	issues = append(issues, glIssues...)

	metricsByPeriod := map[string]BusinessMetrics{}
	for _, m := range in.BusinessMetrics {
		if _, exists := metricsByPeriod[m.Period]; !exists {
			metricsByPeriod[m.Period] = m
		}
	}
	glByPeriod := map[string]GLPayrollControl{}
	for _, g := range in.GLControls {
		if _, exists := glByPeriod[g.Period]; !exists {
			glByPeriod[g.Period] = g
		}
	}

	workers := make([]Worker, 0, len(workersByID))
	for _, id := range sortedStringKeysFromMapWorker(workersByID) {
		workers = append(workers, workersByID[id])
	}

	expenseByPeriod := buildExpenseBasisByPeriod(validPayroll)
	cashByPeriod := buildCashBasisByPeriod(validPayroll, periods)

	var fteAnyAvailable bool
	var summaries []PeriodSummary

	// Partition validPayroll/validContractors by period label once (O(N))
	// rather than re-scanning the full slice once per period (O(N*P)) —
	// the latter was a real quadratic-in-practice bug caught by
	// benchmark_test.go's large-population benchmark at 60 periods x
	// 600,000 records.
	payrollByPeriod := partitionPayrollByPeriod(validPayroll)
	contractorsByPeriod := partitionContractorsByPeriod(validContractors)

	for _, p := range periods {
		periodPayroll := payrollByPeriod[p.Period]
		periodContractors := contractorsByPeriod[p.Period]
		periodWorkers := workersActiveInOrKnownForPeriod(workers, periodPayroll)

		bridge := buildLaborCostBridge(periodPayroll, periodContractors)

		var headcount Headcount
		if len(workers) > 0 {
			headcount = buildHeadcount(p, workers)
		} else if policy.PayrollActiveWorkerApproximation {
			headcount = buildHeadcountApproximation(periodPayroll)
		}

		movement := buildWorkforceMovement(p, workers, headcount)

		fte := buildFTE(p.Period, periodPayroll, in.WorkerPeriodFTEs, policy.StandardFullTimeHoursPerPeriod)
		if fte.Available {
			fteAnyAvailable = true
		}

		overtime := buildOvertimeSummary(periodPayroll)
		contractorMix := buildContractorMix(bridge)
		directIndirect := buildDirectIndirectSplit(periodPayroll)
		avgCost := buildAverageLaborCost(bridge, headcount, fte)
		variableComp := buildVariableCompensation(bridge)

		metrics := metricsByPeriod[p.Period]
		productivity := buildProductivity(bridge, directIndirect, headcount, fte, metrics)

		var glPtr *GLPayrollControl
		if gl, ok := glByPeriod[p.Period]; ok {
			glPtr = &gl
		}
		reconciliation := buildReconciliation(bridge, glPtr, policy)

		keyWorker := buildKeyWorkerConcentration(periodPayroll, workersByID, bridge.TotalEmployeeCost)

		summaries = append(summaries, PeriodSummary{
			Period:                 p,
			LaborCostBridge:        bridge,
			PayrollBasis:           buildPayrollBasis(p.Period, expenseByPeriod, cashByPeriod),
			Headcount:              headcount,
			WorkforceMovement:      movement,
			FTE:                    fte,
			Overtime:               overtime,
			ContractorMix:          contractorMix,
			DirectIndirectSplit:    directIndirect,
			AverageLaborCost:       avgCost,
			VariableCompensation:   variableComp,
			Productivity:           productivity,
			Reconciliation:         reconciliation,
			KeyWorkerConcentration: keyWorker,
			Provenance:             buildProvenance(periodPayroll, periodContractors, periodWorkers),
		})
	}

	result := Result{
		SchemaVersion:  SchemaVersion,
		FormulaVersion: FormulaVersion,
		Periods:        summaries,
	}

	result.DepartmentSummaries = buildGroupSummaries(validPayroll, validContractors, departmentKey, contractorDepartmentKey, workersByID, policy.StandardFullTimeHoursPerPeriod)
	result.LocationSummaries = buildGroupSummaries(validPayroll, validContractors, locationKey, contractorLocationKey, workersByID, policy.StandardFullTimeHoursPerPeriod)
	result.CostCenterSummaries = buildGroupSummaries(validPayroll, validContractors, costCenterKey, contractorCostCenterKey, workersByID, policy.StandardFullTimeHoursPerPeriod)
	result.OvertimeByDepartment = buildOvertimeByDepartment(validPayroll)

	result.Trend = buildTrend(summaries, metricsByPeriod, policy.MinimumTrendPeriods)

	result.ReconciliationSummary = latestReconciliation(summaries)

	result.PaySchedule = buildPayScheduleSummary(validPayroll)

	result.WorkerDateFindings = buildWorkerDateFindings(validPayroll, workersByID, policy.PostTerminationPayGraceDays)
	result.DuplicateGroups = findPossibleDuplicatePayrollRecords(validPayroll)

	result.Coverage = buildCoverage(validPayroll, workers, validContractors, in.BusinessMetrics, in.GLControls, fteAnyAvailable)

	issues = append(issues, dedupeIssuesForWorkerDates(result.WorkerDateFindings)...)

	result.Issues = dedupeAndSortIssues(issues)
	result.Flags = computeFlags(computeFlagsInput{
		periods:            summaries,
		trend:              result.Trend,
		reconciliation:     result.ReconciliationSummary,
		overtimeByDept:     result.OvertimeByDepartment,
		workerDateFindings: result.WorkerDateFindings,
		duplicates:         result.DuplicateGroups,
		policy:             policy,
	})

	return result
}

// partitionPayrollByPeriod groups payroll into one slice per Period
// label in a single O(N) pass, preserving each group's relative input
// order — used instead of re-filtering the full slice once per period.
func partitionPayrollByPeriod(payroll []PayrollRecord) map[string][]PayrollRecord {
	out := make(map[string][]PayrollRecord, len(payroll))
	for _, r := range payroll {
		out[r.Period] = append(out[r.Period], r)
	}
	return out
}

func partitionContractorsByPeriod(contractors []ContractorLaborRecord) map[string][]ContractorLaborRecord {
	out := make(map[string][]ContractorLaborRecord, len(contractors))
	for _, r := range contractors {
		out[r.Period] = append(out[r.Period], r)
	}
	return out
}

// workersActiveInOrKnownForPeriod returns the distinct workers referenced
// by periodPayroll, intersected with the known workers slice when
// non-empty (for provenance only — this does not gate any calculation).
func workersActiveInOrKnownForPeriod(workers []Worker, periodPayroll []PayrollRecord) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range periodPayroll {
		if !seen[r.WorkerID] {
			seen[r.WorkerID] = true
			out = append(out, r.WorkerID)
		}
	}
	sort.Strings(out)
	return out
}

func buildProvenance(payroll []PayrollRecord, contractors []ContractorLaborRecord, workerIDs []string) Provenance {
	p := Provenance{WorkerIDs: workerIDs}
	for _, r := range payroll {
		p.PayrollRecordIDs = append(p.PayrollRecordIDs, r.ID)
	}
	for _, r := range contractors {
		p.ContractorRecordIDs = append(p.ContractorRecordIDs, r.ID)
	}
	sort.Strings(p.PayrollRecordIDs)
	sort.Strings(p.ContractorRecordIDs)
	return p
}

// sortedStringKeysFromMapWorker returns workersByID's keys sorted
// ascending (plain string sort — worker IDs are opaque identifiers, not
// display names, so normalizeGroupName's case-folding is not applied
// here).
func sortedStringKeysFromMapWorker(m map[string]Worker) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// latestReconciliation returns the last chronological period's
// ReconciliationSummary, or an Unavailable summary if there are no
// periods.
func latestReconciliation(summaries []PeriodSummary) ReconciliationSummary {
	if len(summaries) == 0 {
		return ReconciliationSummary{Status: ReconciliationStatusUnavailable}
	}
	return summaries[len(summaries)-1].Reconciliation
}

// dedupeIssuesForWorkerDates converts WORKER_DATE_INCONSISTENCY-severity
// warnings into Issues too, so a caller inspecting Issues alone (rather
// than WorkerDateFindings) still sees a warning-severity finding recorded
// — informational findings are NOT duplicated into Issues (they are not a
// problem, just a note already fully captured in WorkerDateFindings).
func dedupeIssuesForWorkerDates(findings []WorkerDateFinding) []Issue {
	var issues []Issue
	for _, f := range findings {
		if f.Severity != FlagSeverityWarning {
			continue
		}
		issues = append(issues, Issue{Code: IssueInvalidWorkerDates, Severity: SeverityWarning,
			Message: f.Message, RecordID: f.RecordID, WorkerID: f.WorkerID})
	}
	return issues
}
