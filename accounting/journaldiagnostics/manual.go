package journaldiagnostics

// manualEntries returns the analyzed entries whose resolved
// EntryMetadata.Source == SourceManual, in a's order.
func manualEntries(all []analyzedEntry) []analyzedEntry {
	var out []analyzedEntry
	for _, a := range all {
		if a.hasMeta && a.meta.Source == SourceManual {
			out = append(out, a)
		}
	}
	return out
}

// findMaterialManualEntries emits FindingMaterialManualEntry for every
// manual entry whose EntryMagnitude meets Policy.MaterialAmount — see the
// package doc's "Manual journal entries" section: this package never flags
// every manual entry by default, only manual entries that also meet a
// caller policy (materiality here; manual+period-end and manual+revenue/
// equity are separate combination findings in periodend.go/accounts.go).
// Unavailable (not merely empty) when no entry in all has Source metadata
// at all — an empty result under "no metadata supplied" would otherwise
// look identical to "checked, found none."
func findMaterialManualEntries(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	anyMeta := false
	for _, a := range all {
		if a.hasMeta && a.meta.Source != "" {
			anyMeta = true
			break
		}
	}
	if !anyMeta {
		return nil, RuleUnavailable
	}
	if policy.MaterialAmount <= 0 {
		return nil, RuleDisabled
	}

	for _, a := range manualEntries(all) {
		if a.magnitude < policy.MaterialAmount {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingMaterialManualEntry, Severity: SeverityWarning,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount:   AvailableValue(a.magnitude),
			Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date, Source: SourceManual},
			Message:  msgMaterialManualEntry,
		})
	}
	return findings, RuleAvailable
}
