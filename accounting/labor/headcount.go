package labor

import "time"

// Headcount is one period's workforce-count snapshot — the task's section
// 10. Computed only when a worker roster is supplied (or, if the caller
// has explicitly opted into it via Policy.PayrollActiveWorkerApproximation,
// approximated from distinct WorkerIDs appearing in that period's payroll
// records — see buildHeadcountApproximation). BeginningHeadcount is the
// count of workers Active as of (or hired before/on, and not yet
// terminated before) the period's StartDate; EndingHeadcount is the same
// evaluated as of EndDate. AverageHeadcount is the simple average of the
// two — this package uses no daily-weighted average, since PeriodInfo
// only guarantees start/end boundaries, not a daily activity feed.
type Headcount struct {
	Available bool `json:"available"`
	// Approximated is true when this Headcount was derived from distinct
	// payroll WorkerIDs rather than an actual worker roster — see the
	// task's section 10 "only if caller explicitly opts into a
	// payroll-active-worker approximation" instruction.
	Approximated bool `json:"approximated"`

	BeginningHeadcount    int     `json:"beginning_headcount"`
	EndingHeadcount       int     `json:"ending_headcount"`
	AverageHeadcount      float64 `json:"average_headcount"`
	ActiveEmployeeCount   int     `json:"active_employee_count"`
	ActiveContractorCount int     `json:"active_contractor_count"`
}

// workerActiveAsOf reports whether w is considered part of the workforce
// as of date d: HireDate (if known) is on or before d, and
// TerminationDate (if known) is after d. A worker with no HireDate is
// assumed already active (conservatively counted in), matching the "roster
// may not have full history" caller reality; a worker with no
// TerminationDate is assumed still active.
func workerActiveAsOf(w Worker, d time.Time) bool {
	if w.HireDate != nil && w.HireDate.After(d) {
		return false
	}
	if w.TerminationDate != nil && !w.TerminationDate.After(d) {
		return false
	}
	return true
}

// buildHeadcount computes Headcount for one period from a validated
// worker roster.
func buildHeadcount(period PeriodInfo, workers []Worker) Headcount {
	if len(workers) == 0 {
		return Headcount{}
	}
	var beginning, ending, activeEmployees, activeContractors int
	for _, w := range workers {
		if workerActiveAsOf(w, period.StartDate) {
			beginning++
		}
		endActive := workerActiveAsOf(w, period.EndDate)
		if endActive {
			ending++
			switch w.WorkerType {
			case WorkerTypeContractor:
				activeContractors++
			default:
				activeEmployees++
			}
		}
	}
	return Headcount{
		Available:             true,
		BeginningHeadcount:    beginning,
		EndingHeadcount:       ending,
		AverageHeadcount:      float64(beginning+ending) / 2,
		ActiveEmployeeCount:   activeEmployees,
		ActiveContractorCount: activeContractors,
	}
}

// buildHeadcountApproximation approximates EndingHeadcount (only) from the
// distinct WorkerIDs appearing in this period's payroll records, per the
// task's section 10 opt-in approximation. BeginningHeadcount/
// AverageHeadcount are not meaningfully derivable from a single period's
// payroll rows alone, so they are left at zero with Available still true
// (EndingHeadcount/ActiveEmployeeCount are the only fields this
// approximation can responsibly populate).
func buildHeadcountApproximation(payroll []PayrollRecord) Headcount {
	if len(payroll) == 0 {
		return Headcount{}
	}
	seen := map[string]bool{}
	for _, r := range payroll {
		seen[r.WorkerID] = true
	}
	count := len(seen)
	return Headcount{
		Available:           true,
		Approximated:        true,
		EndingHeadcount:     count,
		ActiveEmployeeCount: count,
	}
}

// WorkforceMovement is the task's section 14 hires/departures summary for
// one period, computed only from actual roster hire/termination dates
// (never approximated).
type WorkforceMovement struct {
	Available bool `json:"available"`

	Hires              int `json:"hires"`
	Departures         int `json:"departures"`
	BeginningHeadcount int `json:"beginning_headcount"`
	EndingHeadcount    int `json:"ending_headcount"`
	NetHeadcountChange int `json:"net_headcount_change"`

	// TurnoverRate is Departures / AverageHeadcount, available only when
	// AverageHeadcount is nonzero and roster history (hire/termination
	// dates) is sufficient — see the task's sections 14/15.
	TurnoverRate Value `json:"turnover_rate"`
}

// buildWorkforceMovement computes hires/departures within
// [period.StartDate, period.EndDate] and turnover, from a validated
// worker roster. Requires at least one worker to carry a non-nil
// HireDate or TerminationDate to be meaningful; with none at all, hires
// and departures are correctly zero but Available is still true (the
// roster is known, it simply shows no movement) — turnover
// unavailability is signaled separately per worker-date sufficiency via
// TurnoverRate itself, not Available.
func buildWorkforceMovement(period PeriodInfo, workers []Worker, headcount Headcount) WorkforceMovement {
	if len(workers) == 0 {
		return WorkforceMovement{}
	}
	var hires, departures int
	var haveAnyDateData bool
	for _, w := range workers {
		if w.HireDate != nil {
			haveAnyDateData = true
			if !w.HireDate.Before(period.StartDate) && !w.HireDate.After(period.EndDate) {
				hires++
			}
		}
		if w.TerminationDate != nil {
			haveAnyDateData = true
			if !w.TerminationDate.Before(period.StartDate) && !w.TerminationDate.After(period.EndDate) {
				departures++
			}
		}
	}

	m := WorkforceMovement{
		Available:          true,
		Hires:              hires,
		Departures:         departures,
		BeginningHeadcount: headcount.BeginningHeadcount,
		EndingHeadcount:    headcount.EndingHeadcount,
		NetHeadcountChange: headcount.EndingHeadcount - headcount.BeginningHeadcount,
	}
	if haveAnyDateData && headcount.AverageHeadcount != 0 {
		m.TurnoverRate = AvailableValue(float64(departures) / headcount.AverageHeadcount)
	}
	return m
}
