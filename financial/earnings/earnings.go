package earnings

import (
	"fmt"
	"math"
)

// Calculate derives a single maintainable-earnings figure from observations
// under the strategy and configuration in opts. observations must be
// supplied in chronological order (oldest first) — Calculate does not sort
// them, consistent with financial/metrics' explicit no-guessing rule for
// period order.
//
// Comparable periods. Calculate never blindly averages observations of
// different PeriodType together (e.g. three full fiscal years and one
// trailing YTD stub). It first narrows observations to a comparable subset:
// if Options.ComparablePeriodType is set, only observations with that
// exact PeriodType are eligible; otherwise the comparable type is inferred
// as whichever PeriodType the largest number of supplied observations
// share (see inferComparableType) — the common shape of three full fiscal
// years plus one trailing YTD period correctly resolves to "fiscal year"
// as comparable, excluding the YTD stub, rather than the reverse. Every
// observation whose PeriodType doesn't match is reported in
// Result.ExcludedPeriods with ExclusionIncomparablePeriodType — Calculate
// never averages across them implicitly. A caller that genuinely wants to
// mix granularities (e.g. annualizing a YTD figure before calling this
// package) must do that conversion itself before building the Observation
// slice; this package only ever compares like to like.
func Calculate(observations []Observation, opts Options) Result {
	result := Result{Strategy: opts.Strategy}

	comparableType, ok := inferComparableType(observations, opts)
	if !ok {
		result.Errors = append(result.Errors, "no observations supplied; cannot determine a comparable period type")
		return result
	}

	var comparable []Observation
	for _, obs := range observations {
		if obs.PeriodType != comparableType {
			result.ExcludedPeriods = append(result.ExcludedPeriods, ExcludedObservation{
				Observation: obs,
				Reason:      ExclusionIncomparablePeriodType,
				Detail:      fmt.Sprintf("period type %q does not match comparable type %q", obs.PeriodType, comparableType),
			})
			continue
		}
		comparable = append(comparable, obs)
	}

	switch opts.Strategy {
	case StrategyLatestPeriod:
		return calculateLatestPeriod(result, comparable)
	case StrategySimpleAverage:
		return calculateSimpleAverage(result, comparable)
	case StrategyWeightedAverage:
		return calculateWeightedAverage(result, comparable, opts.Weights)
	case StrategyTrendAdjusted:
		return calculateTrendAdjusted(result, comparable)
	default:
		result.Errors = append(result.Errors, fmt.Sprintf("unrecognized strategy %q", opts.Strategy))
		return result
	}
}

// inferComparableType determines which PeriodType Calculate treats as
// comparable: Options.ComparablePeriodType if set, else the PeriodType
// shared by the largest number of supplied observations. ok is false only
// if observations is empty.
//
// A majority vote, not "the last observation's type," is deliberate: the
// common realistic shape this package must handle well is several full
// fiscal years plus one trailing YTD stub (e.g. three full years and a
// partial current year appended at the end, per this package's explicit
// "don't blindly average YTD with full-year periods" design rule). Keying
// off the last observation's type would make that trailing YTD period
// itself become the comparable baseline and exclude every full year
// instead — exactly backwards from the intended default. Ties are broken
// in favor of PeriodTypeFiscalYear, then PeriodTypeYTD, then
// PeriodTypeQuarter, then PeriodTypeMonth (coarsest first), matching
// metrics.sortOrderedPeriods' granularity ordering; a caller who wants a
// different default should set Options.ComparablePeriodType explicitly
// rather than relying on tie-breaking.
func inferComparableType(observations []Observation, opts Options) (PeriodType, bool) {
	if opts.ComparablePeriodType != "" {
		return opts.ComparablePeriodType, true
	}
	if len(observations) == 0 {
		return "", false
	}

	counts := make(map[PeriodType]int, 4)
	for _, obs := range observations {
		counts[obs.PeriodType]++
	}

	tieBreakOrder := []PeriodType{PeriodTypeFiscalYear, PeriodTypeYTD, PeriodTypeQuarter, PeriodTypeMonth}
	best := PeriodType("")
	bestCount := 0
	for _, pt := range tieBreakOrder {
		if counts[pt] > bestCount {
			best = pt
			bestCount = counts[pt]
		}
	}
	// Cover any PeriodType not in tieBreakOrder (a caller-defined custom
	// granularity) so it can still win on a strict majority.
	for pt, c := range counts {
		if c > bestCount {
			best = pt
			bestCount = c
		}
	}
	return best, true
}

// availableValues splits comparable into available/unavailable
// observations, recording exclusions for the unavailable ones.
func partitionAvailable(comparable []Observation, excluded *[]ExcludedObservation) []Observation {
	var available []Observation
	for _, obs := range comparable {
		if !obs.Available {
			*excluded = append(*excluded, ExcludedObservation{Observation: obs, Reason: ExclusionUnavailable, Detail: "observation has no value for this period"})
			continue
		}
		if math.IsNaN(obs.Value) || math.IsInf(obs.Value, 0) {
			*excluded = append(*excluded, ExcludedObservation{Observation: obs, Reason: ExclusionUnavailable, Detail: "observation value is not finite"})
			continue
		}
		available = append(available, obs)
	}
	return available
}
