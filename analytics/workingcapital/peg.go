package workingcapital

import (
	"fmt"

	"github.com/themurtez/go-valuate/financial"
)

// calculateSuggestedPeg derives a SuggestedPeg from history under
// opts.PegMethod. Returns the zero SuggestedPeg (Method == "") with no
// issue if opts.PegMethod is empty — this package never picks a default
// peg method on the caller's behalf (see the package doc comment). If
// PegMethod is set but its required inputs are unavailable, Value is
// Unavailable and a descriptive *Issue is returned.
func calculateSuggestedPeg(history []PeriodNWC, opts Options) (SuggestedPeg, *Issue) {
	if opts.PegMethod == "" {
		return SuggestedPeg{}, nil
	}

	available := availableNWCPeriods(history)
	peg := SuggestedPeg{Method: opts.PegMethod}

	switch opts.PegMethod {
	case PegMethodLatest:
		if len(available) == 0 {
			return unavailablePeg(peg, "no available NWC observations in History")
		}
		last := available[len(available)-1]
		peg.Value = last.NWC
		peg.PeriodsUsed = []financial.Period{last.Period}
		return peg, nil

	case PegMethodSimpleAverage:
		if len(available) == 0 {
			return unavailablePeg(peg, "no available NWC observations in History")
		}
		peg.Value = AvailableValue(averageOf(available))
		peg.PeriodsUsed = periodsOf(available)
		return peg, nil

	case PegMethodMedian:
		if len(available) == 0 {
			return unavailablePeg(peg, "no available NWC observations in History")
		}
		values := make([]float64, len(available))
		for i, p := range available {
			values[i] = p.NWC.Value
		}
		peg.Value = AvailableValue(median(values))
		peg.PeriodsUsed = periodsOf(available)
		return peg, nil

	case PegMethodTrailingAverage:
		if opts.TrailingPeriods <= 0 {
			return unavailablePeg(peg, "TrailingPeriods must be > 0 for PegMethodTrailingAverage")
		}
		if len(available) < opts.TrailingPeriods {
			return unavailablePeg(peg, fmt.Sprintf("TrailingPeriods (%d) exceeds the number of available NWC observations (%d)", opts.TrailingPeriods, len(available)))
		}
		window := available[len(available)-opts.TrailingPeriods:]
		peg.Value = AvailableValue(averageOf(window))
		peg.PeriodsUsed = periodsOf(window)
		return peg, nil

	case PegMethodFixed:
		if !opts.FixedPeg.Available {
			return unavailablePeg(peg, "Options.FixedPeg is not Available for PegMethodFixed")
		}
		peg.Value = opts.FixedPeg
		return peg, nil

	default:
		return unavailablePeg(peg, fmt.Sprintf("unrecognized PegMethod %q", opts.PegMethod))
	}
}

// unavailablePeg returns peg with Value left Unavailable and a matching
// IssuePegMethodUnavailable warning.
func unavailablePeg(peg SuggestedPeg, detail string) (SuggestedPeg, *Issue) {
	return peg, &Issue{
		Code:     IssuePegMethodUnavailable,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("suggested peg unavailable: %s", detail),
	}
}

// availableNWCPeriods filters history to periods with an Available NWC,
// preserving history's own (assumed chronological) order.
func availableNWCPeriods(history []PeriodNWC) []PeriodNWC {
	var out []PeriodNWC
	for _, p := range history {
		if p.NWC.Available {
			out = append(out, p)
		}
	}
	return out
}

func averageOf(periods []PeriodNWC) float64 {
	sum := 0.0
	for _, p := range periods {
		sum += p.NWC.Value
	}
	return sum / float64(len(periods))
}

func periodsOf(periods []PeriodNWC) []financial.Period {
	out := make([]financial.Period, len(periods))
	for i, p := range periods {
		out[i] = p.Period
	}
	return out
}
