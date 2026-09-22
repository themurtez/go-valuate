package metrics

import "github.com/themurtez/go-valuate/financial"

// PeriodType identifies the granularity of a reporting period, used to
// decide which periods are comparable for trend calculations.
type PeriodType string

const (
	PeriodTypeFiscalYear PeriodType = "fiscal_year"
	PeriodTypeYTD        PeriodType = "ytd"
	PeriodTypeQuarter    PeriodType = "quarter"
	PeriodTypeMonth      PeriodType = "month"
)

// PeriodInfo is caller-supplied metadata about a single financial.Period,
// used to determine chronological order and comparability for trend
// calculations. financial.Period is intentionally just a string with no
// guaranteed sort order (see financial.FinancialDataset.Periods' doc
// comment); this package requires PeriodInfo rather than inferring order
// from the string, per this package's explicit no-guessing rule for trends.
type PeriodInfo struct {
	// Type is this period's granularity.
	Type PeriodType
	// FiscalYear is the fiscal year this period falls within (e.g. 2025 for
	// both a "2025" fiscal-year period and a "2025-Q1" quarter). Required
	// for ordering.
	FiscalYear int
	// SequenceInYear orders periods that share the same FiscalYear and Type
	// (e.g. Q1=1, Q2=2, Q3=3, Q4=4, or month 1-12). Ignored for
	// PeriodTypeFiscalYear and PeriodTypeYTD, where FiscalYear alone is
	// sufficient to order same-type periods.
	SequenceInYear int
}

// orderedPeriod pairs a financial.Period with its resolved PeriodInfo, used
// internally once ordering has been validated.
type orderedPeriod struct {
	period financial.Period
	info   PeriodInfo
}

// resolvePeriodOrder looks up PeriodInfo for every period and returns them
// sorted chronologically (ascending). It returns a non-nil *PeriodOrderError,
// rather than guessing, if any period is missing from meta — trend
// calculations must not silently assume lexical or input order is
// chronological.
func resolvePeriodOrder(periods []financial.Period, meta map[financial.Period]PeriodInfo) ([]orderedPeriod, *PeriodOrderError) {
	if len(meta) == 0 {
		return nil, &PeriodOrderError{Reason: "no PeriodMeta supplied; period order cannot be determined"}
	}

	ordered := make([]orderedPeriod, 0, len(periods))
	var missing []financial.Period
	for _, p := range periods {
		info, ok := meta[p]
		if !ok {
			missing = append(missing, p)
			continue
		}
		ordered = append(ordered, orderedPeriod{period: p, info: info})
	}
	if len(missing) > 0 {
		return nil, &PeriodOrderError{
			Reason:         "one or more periods have no PeriodMeta entry",
			MissingPeriods: missing,
		}
	}

	sortOrderedPeriods(ordered)
	return ordered, nil
}

// sortOrderedPeriods sorts by FiscalYear, then by a type rank that places
// coarser granularities before finer ones within the same year (fiscal
// year/YTD, then quarter, then month), then by SequenceInYear. This is a
// simple insertion sort since period counts are small (a handful of years
// at most for realistic valuation datasets).
func sortOrderedPeriods(periods []orderedPeriod) {
	rank := func(t PeriodType) int {
		switch t {
		case PeriodTypeFiscalYear, PeriodTypeYTD:
			return 0
		case PeriodTypeQuarter:
			return 1
		case PeriodTypeMonth:
			return 2
		default:
			return 3
		}
	}
	less := func(a, b orderedPeriod) bool {
		if a.info.FiscalYear != b.info.FiscalYear {
			return a.info.FiscalYear < b.info.FiscalYear
		}
		if rank(a.info.Type) != rank(b.info.Type) {
			return rank(a.info.Type) < rank(b.info.Type)
		}
		return a.info.SequenceInYear < b.info.SequenceInYear
	}
	for i := 1; i < len(periods); i++ {
		j := i
		for j > 0 && less(periods[j], periods[j-1]) {
			periods[j], periods[j-1] = periods[j-1], periods[j]
			j--
		}
	}
}

// PeriodOrderError explains why chronological period order could not be
// determined. Trend calculations return this (wrapped in Trend.Error) rather
// than guessing an order from financial.Period's string value.
type PeriodOrderError struct {
	Reason         string
	MissingPeriods []financial.Period
}

func (e *PeriodOrderError) Error() string {
	if len(e.MissingPeriods) == 0 {
		return "metrics: " + e.Reason
	}
	return "metrics: " + e.Reason + ": " + joinPeriods(e.MissingPeriods)
}

func joinPeriods(periods []financial.Period) string {
	s := ""
	for i, p := range periods {
		if i > 0 {
			s += ", "
		}
		s += string(p)
	}
	return s
}

// fiscalYearPeriods filters ordered to only PeriodTypeFiscalYear entries,
// preserving chronological order. Several trend calculations (YoY growth,
// CAGR) restrict themselves to fiscal-year-to-fiscal-year comparisons for
// MVP simplicity — see Trend's doc comment for why quarters/months are not
// yet mixed into growth/CAGR calculations.
func fiscalYearPeriods(ordered []orderedPeriod) []orderedPeriod {
	var out []orderedPeriod
	for _, p := range ordered {
		if p.info.Type == PeriodTypeFiscalYear {
			out = append(out, p)
		}
	}
	return out
}
