package labor

// FTESummary is one period's full-time-equivalent computation — the task's
// section 11. Supported only when hours or caller-defined FTE facts are
// supplied; this package never assumes 40 hours/week universally.
type FTESummary struct {
	Available bool `json:"available"`
	// Method records which of the two supported approaches produced this
	// value.
	Method FTEMethod `json:"method,omitempty"`
	FTE    float64   `json:"fte"`
}

// FTEMethod distinguishes the two FTE computation paths the task's
// section 11 describes.
type FTEMethod string

const (
	// FTEMethodHoursBased means FTE = total eligible hours / caller-
	// supplied Policy.StandardFullTimeHoursPerPeriod.
	FTEMethodHoursBased FTEMethod = "HOURS_BASED"
	// FTEMethodCallerSupplied means the caller directly supplied FTE
	// values via WorkerPeriodFTE.
	FTEMethodCallerSupplied FTEMethod = "CALLER_SUPPLIED"
)

// WorkerPeriodFTE is one caller-supplied explicit FTE value for one worker
// in one period — the task's section 11(b) "caller-supplied FTE" path.
// When present for a period, this takes precedence over the hours-based
// computation for that period (a caller who has already computed FTE its
// own way should not have it silently second-guessed).
type WorkerPeriodFTE struct {
	WorkerID string  `json:"worker_id"`
	Period   string  `json:"period"`
	FTE      float64 `json:"fte"`
}

func validateWorkerPeriodFTEs(entries []WorkerPeriodFTE) []Issue {
	var issues []Issue
	for _, e := range entries {
		if isNonFinite(e.FTE) || e.FTE < 0 {
			issues = append(issues, Issue{Code: IssueInvalidFTE, Severity: SeverityError,
				Message: "caller-supplied FTE value is negative or non-finite", WorkerID: e.WorkerID, Period: e.Period})
		}
	}
	return issues
}

// buildFTE computes FTESummary for one period: caller-supplied FTE values
// (summed across workers) take precedence when present for the period;
// otherwise hours-based FTE is used if StandardFullTimeHoursPerPeriod is
// positive and at least one payroll record in the period has hours data;
// otherwise unavailable.
func buildFTE(period string, payroll []PayrollRecord, callerFTE []WorkerPeriodFTE, standardHours float64) FTESummary {
	var callerSum float64
	var haveCaller bool
	for _, e := range callerFTE {
		if e.Period != period || isNonFinite(e.FTE) || e.FTE < 0 {
			continue
		}
		callerSum += e.FTE
		haveCaller = true
	}
	if haveCaller {
		return FTESummary{Available: true, Method: FTEMethodCallerSupplied, FTE: callerSum}
	}

	if standardHours <= 0 {
		return FTESummary{}
	}
	var totalHours float64
	var haveHours bool
	for _, r := range payroll {
		if r.Period != period {
			continue
		}
		if r.HoursRegular.Available {
			totalHours += r.HoursRegular.Amount
			haveHours = true
		}
		if r.HoursOvertime.Available {
			totalHours += r.HoursOvertime.Amount
			haveHours = true
		}
	}
	if !haveHours {
		return FTESummary{}
	}
	return FTESummary{Available: true, Method: FTEMethodHoursBased, FTE: totalHours / standardHours}
}
