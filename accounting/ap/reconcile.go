package ap

// buildAgingReconciliation computes the always-checked invariant: bucket
// amounts must sum to total open payables. Never forces balance — reports
// whatever was actually computed.
func buildAgingReconciliation(totalOpen float64, buckets []BucketAmount) AgingReconciliation {
	var sum float64
	for _, b := range buckets {
		sum += b.Amount
	}
	diff := totalOpen - sum
	return AgingReconciliation{
		TotalOpenPayables: totalOpen,
		SumOfBuckets:      sum,
		Difference:        diff,
		Balanced:          diff > -amountTolerance && diff < amountTolerance,
	}
}

// buildControlAccountReconciliation compares the subledger aging total
// against a caller-supplied GL control account balance. Available only if
// controlBalance is non-nil. Never adjusts either side.
func buildControlAccountReconciliation(subledgerBalance float64, controlBalance *float64, tolerance float64) ControlAccountReconciliation {
	if controlBalance == nil {
		return ControlAccountReconciliation{}
	}
	diff := subledgerBalance - *controlBalance
	tol := resolvedControlAccountTolerance(tolerance)
	return ControlAccountReconciliation{
		Available:             true,
		SubledgerBalance:      subledgerBalance,
		ControlAccountBalance: *controlBalance,
		Difference:            diff,
		Tolerance:             tol,
		Reconciled:            diff > -tol && diff < tol,
	}
}
