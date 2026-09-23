package closequality

// mineARControl translates ar.Result's own reconciliation outcomes into
// DimensionARControl findings. It never judges overdue customers or
// collection performance — that is ar's own domain, not a close-quality
// concern (see package doc comment's "bookkeeping/close quality, not
// collection performance" boundary). Only AgingReconciliation and
// ControlAccountReconciliation are read; ar.Result.Issues are not
// re-surfaced here since they are input/data-quality problems already
// visible in ar's own output, not close-readiness conditions distinct
// from the two reconciliation checks below.
func mineARControl(in Input, policy Policy) ([]Finding, DimensionResult) {
	b := newDimensionBuilder(DimensionARControl)

	if !in.arAvailable() {
		b.unavailable("AR result not supplied or not available")
		return nil, b.build()
	}
	b.markAssessed()

	var findings []Finding

	agingRec := in.AR.AgingReconciliation
	if !agingRec.Balanced {
		f := Finding{
			Code:      FindingARAgingNotReconciled,
			Dimension: DimensionARControl,
			Severity:  SeverityBlocking,
			Message:   "AR aging buckets do not sum to total open receivables",
			Evidence: Evidence{
				SubledgerBalance: AvailableValue(agingRec.TotalOpenReceivables),
				ControlBalance:   AvailableValue(agingRec.SumOfBuckets),
				Difference:       AvailableValue(agingRec.Difference),
			},
			SourceModule: SourceAR,
		}
		findings = append(findings, f)
		b.record(f)
	} else {
		b.note("AR aging reconciliation balanced")
	}

	ctrl := in.AR.ControlAccountReconciliation
	if ctrl.Available {
		if !ctrl.Reconciled {
			material := isMaterial(ctrl.Difference, in.TotalAssets, in.TotalRevenue, policy.Materiality)
			sev := SeverityWarning
			if material {
				sev = SeverityBlocking
			}
			f := Finding{
				Code:      FindingARControlMismatch,
				Dimension: DimensionARControl,
				Severity:  sev,
				Message:   "AR subledger balance does not match the supplied GL control account balance",
				Evidence: Evidence{
					SubledgerBalance:     AvailableValue(ctrl.SubledgerBalance),
					ControlBalance:       AvailableValue(ctrl.ControlAccountBalance),
					Difference:           AvailableValue(ctrl.Difference),
					Tolerance:            AvailableValue(ctrl.Tolerance),
					MaterialityThreshold: materialityEvidence(policy.Materiality),
				},
				SourceModule: SourceAR,
			}
			findings = append(findings, f)
			b.record(f)
		} else {
			b.note("AR control account reconciled")
		}
	} else {
		b.note("AR control account balance not supplied — control reconciliation unavailable")
	}

	return findings, b.build()
}
