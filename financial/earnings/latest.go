package earnings

// calculateLatestPeriod selects the chronologically last available,
// comparable observation (the last element of comparable, since callers
// supply observations in chronological order) as maintainable earnings.
// Every other comparable observation is excluded with ExclusionNotLatest.
func calculateLatestPeriod(result Result, comparable []Observation) Result {
	var excluded []ExcludedObservation
	available := partitionAvailable(comparable, &excluded)
	result.ExcludedPeriods = append(result.ExcludedPeriods, excluded...)

	if len(available) == 0 {
		result.Errors = append(result.Errors, "no available comparable observation to use as the latest period")
		return result
	}

	latest := available[len(available)-1]
	for _, obs := range available[:len(available)-1] {
		result.ExcludedPeriods = append(result.ExcludedPeriods, ExcludedObservation{
			Observation: obs,
			Reason:      ExclusionNotLatest,
			Detail:      "latest_period strategy uses only the most recent comparable period",
		})
	}

	result.IncludedPeriods = []Observation{latest}
	result.RawValues = []float64{latest.Value}
	result.Available = true
	result.Value = latest.Value
	return result
}
