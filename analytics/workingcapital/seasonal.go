package workingcapital

import (
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// calculateSeasonalProfile groups history's NWCPercentOfRevenue
// observations by PeriodInfo.SequenceInYear, restricted to whichever of
// PeriodTypeQuarter/PeriodTypeMonth is present in meta — never both at
// once, and never PeriodTypeFiscalYear/PeriodTypeYTD periods, which have no
// seasonal position within a year. Returns the zero SeasonalProfile (empty
// Periods) if meta contains no quarter or month periods, or if quarter and
// month periods are mixed (this package does not attempt to reconcile two
// different granularities into one seasonal profile — a caller with both
// should call this analysis separately per granularity by supplying only
// one PeriodType's PeriodMeta at a time).
func calculateSeasonalProfile(history []PeriodNWC, meta map[financial.Period]PeriodInfo) SeasonalProfile {
	byPeriod := make(map[financial.Period]PeriodNWC, len(history))
	for _, p := range history {
		byPeriod[p.Period] = p
	}

	quarterCount, monthCount := 0, 0
	for period := range byPeriod {
		info, ok := meta[period]
		if !ok {
			continue
		}
		switch info.Type {
		case PeriodTypeQuarter:
			quarterCount++
		case PeriodTypeMonth:
			monthCount++
		}
	}

	var target PeriodType
	switch {
	case quarterCount > 0 && monthCount == 0:
		target = PeriodTypeQuarter
	case monthCount > 0 && quarterCount == 0:
		target = PeriodTypeMonth
	default:
		return SeasonalProfile{}
	}

	type bucket struct {
		sum   float64
		count int
	}
	buckets := make(map[int]*bucket)
	for period, pnwc := range byPeriod {
		info, ok := meta[period]
		if !ok || info.Type != target || !pnwc.NWCPercentOfRevenue.Available {
			continue
		}
		b, exists := buckets[info.SequenceInYear]
		if !exists {
			b = &bucket{}
			buckets[info.SequenceInYear] = b
		}
		b.sum += pnwc.NWCPercentOfRevenue.Value
		b.count++
	}

	if len(buckets) == 0 {
		return SeasonalProfile{}
	}

	sequences := make([]int, 0, len(buckets))
	for seq := range buckets {
		sequences = append(sequences, seq)
	}
	sort.Ints(sequences)

	periods := make([]SeasonalPeriod, 0, len(sequences))
	for _, seq := range sequences {
		b := buckets[seq]
		periods = append(periods, SeasonalPeriod{
			SequenceInYear:             seq,
			AverageNWCPercentOfRevenue: AvailableValue(b.sum / float64(b.count)),
			SampleSize:                 b.count,
		})
	}

	return SeasonalProfile{PeriodType: target, Periods: periods}
}
