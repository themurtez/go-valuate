package closequality

import "github.com/themurtez/go-valuate/accounting/journaldiagnostics"

// mineJournalReview translates journaldiagnostics.Result.Findings into
// DimensionJournalReview close-quality findings, disposition controlled
// entirely by Policy.JournalFindingRules (see
// Policy.journalDisposition/DefaultJournalFindingRules) — this package
// never hard-codes which journal findings are blocking. POST_CLOSE_ENTRY
// findings are excluded here and instead handled by
// minePeriodLockPostCloseActivity, since post-close severity depends on
// PeriodInfo.Lock/CloseDate rather than the generic disposition table.
// journaldiagnostics never asserts fraud or wrongdoing, and neither does
// this translation — see docs/CLOSE_QUALITY.md's neutral-language
// safeguards.
func mineJournalReview(in Input, policy Policy) ([]Finding, DimensionResult) {
	b := newDimensionBuilder(DimensionJournalReview)

	if !in.journalDiagnosticsAvailable() {
		b.unavailable("journal diagnostics result not supplied")
		return nil, b.build()
	}
	b.markAssessed()

	var findings []Finding
	for _, jf := range in.JournalDiagnostics.Findings {
		if jf.Code == journaldiagnostics.FindingPostCloseEntry {
			continue
		}
		disposition := policy.journalDisposition(string(jf.Code))
		if disposition == JournalFindingIgnore {
			continue
		}
		f := Finding{
			Code:         FindingMaterialJournalReviewRequired,
			Dimension:    DimensionJournalReview,
			Severity:     journalDispositionSeverity(disposition),
			Message:      jf.Message,
			Evidence:     Evidence{TotalAmount: journalAmountToValue(jf.Amount)},
			SourceModule: SourceJournalDiagnostics,
			SourceCode:   string(jf.Code),
			EntryIDs:     sortedStrings(jf.EntryIDs),
			AccountIDs:   sortedStrings(jf.AccountIDs),
		}
		findings = append(findings, f)
		b.record(f)
	}
	if len(findings) == 0 {
		b.note("no journal findings mapped to a review disposition other than IGNORE")
	}
	return findings, b.build()
}

func journalDispositionSeverity(d JournalFindingDisposition) Severity {
	switch d {
	case JournalFindingBlocking:
		return SeverityBlocking
	case JournalFindingWarning:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

func journalAmountToValue(v journaldiagnostics.Value) Value {
	if !v.Available {
		return Unavailable()
	}
	return AvailableValue(v.Amount)
}

// minePeriodLockPostCloseActivity translates
// journaldiagnostics.FindingPostCloseEntry findings into
// DimensionPeriodLockPostCloseActivity findings. Severity depends on
// materiality and on whether PeriodInfo.Lock indicates the period was
// supposed to already be closed — this package never guesses whether
// posting was authorized (see package doc comment).
func minePeriodLockPostCloseActivity(in Input, policy Policy) ([]Finding, DimensionResult) {
	b := newDimensionBuilder(DimensionPeriodLockPostCloseActivity)

	if !in.journalDiagnosticsAvailable() {
		b.unavailable("journal diagnostics result not supplied")
		return nil, b.build()
	}
	b.markAssessed()

	var findings []Finding
	for _, jf := range in.JournalDiagnostics.Findings {
		if jf.Code != journaldiagnostics.FindingPostCloseEntry {
			continue
		}
		amount := journalAmountToValue(jf.Amount)
		material := true
		if amount.Available {
			material = isMaterial(amount.Amount, in.TotalAssets, in.TotalRevenue, policy.Materiality)
		}
		sev := SeverityInfo
		switch {
		case in.Period.isLocked() && policy.TreatLockedPostCloseAsBlocking:
			sev = SeverityBlocking
		case material:
			sev = SeverityWarning
		}
		f := Finding{
			Code:      FindingPostCloseActivity,
			Dimension: DimensionPeriodLockPostCloseActivity,
			Severity:  sev,
			Message:   jf.Message,
			Evidence: Evidence{
				TotalAmount: amount,
				CloseDate:   jf.Evidence.CloseDate,
			},
			SourceModule: SourceJournalDiagnostics,
			SourceCode:   string(jf.Code),
			EntryIDs:     sortedStrings(jf.EntryIDs),
			AccountIDs:   sortedStrings(jf.AccountIDs),
		}
		findings = append(findings, f)
		b.record(f)
	}
	if len(findings) == 0 {
		b.note("no post-close journal activity reported")
	}
	return findings, b.build()
}
