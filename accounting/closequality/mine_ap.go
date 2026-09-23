package closequality

// mineAPControl mirrors mineARControl for ap.Result. Operational payment
// pressure (DueSchedule/PaymentPressure) is deliberately not surfaced
// here — it is not a bookkeeping defect (see package doc comment's
// "operational payment pressure is not a bookkeeping defect" boundary).
func mineAPControl(in Input, policy Policy) ([]Finding, DimensionResult) {
	b := newDimensionBuilder(DimensionAPControl)

	if !in.apAvailable() {
		b.unavailable("AP result not supplied or not available")
		return nil, b.build()
	}
	b.markAssessed()

	var findings []Finding

	agingRec := in.AP.AgingReconciliation
	if !agingRec.Balanced {
		f := Finding{
			Code:      FindingAPAgingNotReconciled,
			Dimension: DimensionAPControl,
			Severity:  SeverityBlocking,
			Message:   "AP aging buckets do not sum to total open payables",
			Evidence: Evidence{
				SubledgerBalance: AvailableValue(agingRec.TotalOpenPayables),
				ControlBalance:   AvailableValue(agingRec.SumOfBuckets),
				Difference:       AvailableValue(agingRec.Difference),
			},
			SourceModule: SourceAP,
		}
		findings = append(findings, f)
		b.record(f)
	} else {
		b.note("AP aging reconciliation balanced")
	}

	ctrl := in.AP.ControlAccountReconciliation
	if ctrl.Available {
		if !ctrl.Reconciled {
			material := isMaterial(ctrl.Difference, in.TotalAssets, in.TotalRevenue, policy.Materiality)
			sev := SeverityWarning
			if material {
				sev = SeverityBlocking
			}
			f := Finding{
				Code:      FindingAPControlMismatch,
				Dimension: DimensionAPControl,
				Severity:  sev,
				Message:   "AP subledger balance does not match the supplied GL control account balance",
				Evidence: Evidence{
					SubledgerBalance:     AvailableValue(ctrl.SubledgerBalance),
					ControlBalance:       AvailableValue(ctrl.ControlAccountBalance),
					Difference:           AvailableValue(ctrl.Difference),
					Tolerance:            AvailableValue(ctrl.Tolerance),
					MaterialityThreshold: materialityEvidence(policy.Materiality),
				},
				SourceModule: SourceAP,
			}
			findings = append(findings, f)
			b.record(f)
		} else {
			b.note("AP control account reconciled")
		}
	} else {
		b.note("AP control account balance not supplied — control reconciliation unavailable")
	}

	return findings, b.build()
}
