package earnings

// calculateSimpleAverage is the unweighted arithmetic mean of every
// available, comparable observation's Value.
func calculateSimpleAverage(result Result, comparable []Observation) Result {
	var excluded []ExcludedObservation
	available := partitionAvailable(comparable, &excluded)
	result.ExcludedPeriods = append(result.ExcludedPeriods, excluded...)

	if len(available) == 0 {
		result.Errors = append(result.Errors, "no available comparable observations to average")
		return result
	}

	sum := 0.0
	rawValues := make([]float64, 0, len(available))
	for _, obs := range available {
		sum += obs.Value
		rawValues = append(rawValues, obs.Value)
	}

	result.IncludedPeriods = available
	result.RawValues = rawValues
	result.Available = true
	result.Value = sum / float64(len(available))
	if len(available) == 1 {
		result.Warnings = append(result.Warnings, "only one comparable period available; a simple average of one value is just that value")
	}
	return result
}
