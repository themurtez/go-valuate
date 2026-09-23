package dealstructure

import "math"

// aggregateAnnualDebtService sums principal/interest/balloon across
// every schedule in schedules, year by year, from year 1 through the
// longest schedule's term. A tranche that has already been paid off (its
// TermYears is shorter than another tranche's) simply contributes
// nothing to years past its own term — its Periods slice is exhausted,
// not padded with zero-value periods.
func aggregateAnnualDebtService(schedules []AmortizationSchedule) []AnnualDebtServicePeriod {
	maxYears := 0
	for _, s := range schedules {
		years := int(math.Ceil(s.Terms.TermYears))
		if years > maxYears {
			maxYears = years
		}
	}
	if maxYears == 0 {
		return nil
	}

	result := make([]AnnualDebtServicePeriod, maxYears)
	for y := 0; y < maxYears; y++ {
		result[y] = AnnualDebtServicePeriod{Year: y + 1}
	}

	for _, s := range schedules {
		ppy := s.PaymentsPerYear
		if ppy == 0 {
			continue
		}
		for i, period := range s.Periods {
			year := i / ppy
			if year >= maxYears {
				continue
			}
			result[year].Payment += period.Payment
			result[year].Interest += period.Interest
			result[year].Principal += period.Principal
			result[year].Balloon += period.Balloon
		}
	}

	return result
}
