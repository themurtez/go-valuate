package reconciliation

import "github.com/themurtez/go-valuate/financial"

// ReportedPeriodTotals holds externally reported subtotal figures for one
// period, supplied by the caller so income-statement checks can compare a
// reconstructed figure against what the source statement actually reported.
//
// This exists because financial.FinancialDataset has no way to carry these
// values itself: financial.Normalize discards subtotal/total rows entirely
// (see financial.RowStatusSubtotal/RowStatusTotal) specifically to avoid
// double-counting them during aggregation, and the canonical taxonomy has
// no codes for them either, since they are computed aggregates rather than
// classifiable accounts. A caller that has access to the original
// statement's reported subtotals (e.g. from the financial.RawLineItem rows
// tagged RowStatusSubtotal/RowStatusTotal before they were dropped) passes
// them here; a caller that doesn't have them simply leaves the
// corresponding field nil, and the associated Check reports
// StatusNotApplicable rather than a fabricated comparison.
//
// Every field is a pointer so nil unambiguously means "not supplied" —
// consistent with settings.Settings' nil-means-unset convention elsewhere
// in this module — as opposed to a reported value that happens to be
// exactly 0.
type ReportedPeriodTotals struct {
	// GrossProfit is the reported gross profit for this period, if known.
	GrossProfit *float64 `json:"gross_profit,omitempty"`
	// OperatingIncome is the reported operating income (EBIT) for this
	// period, if known.
	OperatingIncome *float64 `json:"operating_income,omitempty"`
	// EBITDA is the reported EBITDA for this period, if known. Most source
	// statements do not report EBITDA directly (it's not a GAAP line item),
	// so this is expected to be nil far more often than not.
	EBITDA *float64 `json:"ebitda,omitempty"`
	// NetIncome is the reported net income ("bottom line") for this period,
	// if known.
	NetIncome *float64 `json:"net_income,omitempty"`
}

// ReportedTotals maps period to that period's ReportedPeriodTotals. A period
// present in the dataset but absent from this map is treated identically to
// a period present with every field nil: every reported-vs-reconstructed
// check for it returns StatusNotApplicable.
type ReportedTotals map[financial.Period]ReportedPeriodTotals

// Options controls Run's behavior.
type Options struct {
	// Tolerance controls how large a difference can be while still counting
	// as StatusPass, for every numeric comparison Run performs. See
	// Tolerance. A zero-value Tolerance (both fields 0) is replaced with
	// DefaultTolerance so callers who don't think about tolerance at all
	// still get sane rounding-noise handling rather than a tolerance of
	// exactly 0, which would fail on essentially any real dataset. Callers
	// who genuinely want zero tolerance should pass a Tolerance with a
	// tiny nonzero Absolute (e.g. 0.005 to allow sub-cent float rounding)
	// rather than relying on the zero value.
	Tolerance Tolerance
	// Reported supplies externally reported subtotal figures, keyed by
	// period, for checks that compare a reconstructed figure against what
	// the source statement actually reported (gross profit, operating
	// income, EBITDA, net income). See ReportedTotals. Nil (the zero value)
	// means no reported totals were supplied at all; every
	// reported-vs-reconstructed check then returns StatusNotApplicable.
	Reported ReportedTotals
}

func (o Options) tolerance() Tolerance {
	if o.Tolerance.Absolute == 0 && o.Tolerance.RelativePercent == 0 {
		return DefaultTolerance
	}
	return o.Tolerance
}

func (o Options) reportedFor(period financial.Period) ReportedPeriodTotals {
	if o.Reported == nil {
		return ReportedPeriodTotals{}
	}
	return o.Reported[period]
}
