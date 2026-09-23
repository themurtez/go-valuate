package journaldiagnostics

import (
	"sort"
	"strings"
)

// normalizeDescription lowercases and trims d for generic-description exact
// matching — see the package doc's "exact normalized caller-supplied
// strings" instruction (never a broad text heuristic).
func normalizeDescription(d string) string {
	return strings.ToLower(strings.TrimSpace(d))
}

// findBlankDescriptionFindings implements section 30's blank-description
// half: always available (a blank ledger.JournalEntry.Description needs no
// optional metadata to detect).
func findBlankDescriptionFindings(all []analyzedEntry) (findings []Finding, state RuleState) {
	for _, a := range all {
		if strings.TrimSpace(a.entry.Description) != "" {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingBlankDescription, Severity: SeverityInfo,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount:   AvailableValue(a.magnitude),
			Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date},
			Message:  msgBlankDescription,
		})
	}
	return findings, RuleAvailable
}

// findGenericDescriptionFindings implements section 30's generic-
// description half: fires only for an entry whose normalized Description
// exactly matches one of policy.GenericDescriptions (also normalized).
// Disabled (not merely empty) when the caller supplies no
// GenericDescriptions — this package never hard-codes its own broad
// text-heuristic list.
func findGenericDescriptionFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if len(policy.GenericDescriptions) == 0 {
		return nil, RuleDisabled
	}
	generic := make(map[string]bool, len(policy.GenericDescriptions))
	for _, g := range policy.GenericDescriptions {
		generic[normalizeDescription(g)] = true
	}
	for _, a := range all {
		norm := normalizeDescription(a.entry.Description)
		if norm == "" || !generic[norm] {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingGenericDescription, Severity: SeverityInfo,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount:   AvailableValue(a.magnitude),
			Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date, Description: a.entry.Description},
			Message:  msgGenericDescription,
		})
	}
	return findings, RuleAvailable
}

// hasReference reports whether a has at least one of the reference-like
// fields named in fields: "external_reference" (ledger.JournalEntry.
// ExternalReference), "reference" (ledger.JournalEntry.Reference),
// "external_ref" (EntryMetadata.ExternalRef), "batch_id"
// (EntryMetadata.BatchID). An unrecognized field name is ignored (treated
// as never satisfied), rather than causing a panic on caller typos.
func hasReference(a analyzedEntry, fields []string) bool {
	for _, f := range fields {
		switch f {
		case "external_reference":
			if a.entry.ExternalReference != "" {
				return true
			}
		case "reference":
			if a.entry.Reference != "" {
				return true
			}
		case "external_ref":
			if a.hasMeta && a.meta.ExternalRef != "" {
				return true
			}
		case "batch_id":
			if a.hasMeta && a.meta.BatchID != "" {
				return true
			}
		}
	}
	return false
}

// findMissingReferenceFindings implements section 31: only opted into when
// policy.RequiredReferenceFields is non-empty, and only material entries
// are checked (an immaterial entry missing a reference is not itself
// noteworthy).
func findMissingReferenceFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	if len(policy.RequiredReferenceFields) == 0 {
		return nil, RuleDisabled
	}
	if policy.MaterialAmount <= 0 {
		return nil, RuleDisabled
	}
	for _, a := range all {
		if a.magnitude < policy.MaterialAmount {
			continue
		}
		if hasReference(a, policy.RequiredReferenceFields) {
			continue
		}
		findings = append(findings, Finding{
			Code: FindingMissingReference, Severity: SeverityInfo,
			Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
			Amount:   AvailableValue(a.magnitude),
			Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date},
			Message:  msgMissingReference,
		})
	}
	return findings, RuleAvailable
}

// findPreparerApproverFindings implements section 32: same-preparer-
// approver, missing-approver, and high-volume-by-preparer. Uses opaque IDs
// exactly as caller-supplied — never resolves or profiles a real identity.
// Unavailable when no analyzed entry has PreparerID metadata at all.
func findPreparerApproverFindings(all []analyzedEntry, policy Policy) (findings []Finding, state RuleState) {
	anyPreparer := false
	for _, a := range all {
		if a.hasMeta && a.meta.PreparerID != "" {
			anyPreparer = true
			break
		}
	}
	if !anyPreparer {
		return nil, RuleUnavailable
	}

	byPreparer := make(map[string][]analyzedEntry)
	for _, a := range all {
		if !a.hasMeta || a.meta.PreparerID == "" {
			continue
		}
		byPreparer[a.meta.PreparerID] = append(byPreparer[a.meta.PreparerID], a)

		if a.meta.ApproverID == "" {
			findings = append(findings, Finding{
				Code: FindingMissingApprover, Severity: SeverityInfo,
				Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
				Amount:   AvailableValue(a.magnitude),
				Evidence: Evidence{EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date, PreparerID: a.meta.PreparerID},
				Message:  msgMissingApprover,
			})
		} else if a.meta.ApproverID == a.meta.PreparerID {
			findings = append(findings, Finding{
				Code: FindingSamePreparerApprover, Severity: SeverityWarning,
				Period: a.period, EntryIDs: []string{a.entry.ID}, AccountIDs: entryAccountIDs(a.entry),
				Amount: AvailableValue(a.magnitude),
				Evidence: Evidence{
					EntryAmount: AvailableValue(a.magnitude), EntryDate: a.date,
					PreparerID: a.meta.PreparerID, ApproverID: a.meta.ApproverID,
				},
				Message: msgSamePreparerApprover,
			})
		}
	}

	if policy.HighVolumePreparerCount > 0 {
		var preparers []string
		for p := range byPreparer {
			preparers = append(preparers, p)
		}
		sort.Strings(preparers)
		for _, p := range preparers {
			members := byPreparer[p]
			if len(members) < policy.HighVolumePreparerCount {
				continue
			}
			ids := make([]string, len(members))
			var total float64
			for i, m := range members {
				ids[i] = m.entry.ID
				total += m.magnitude
			}
			ids = sortStrings(ids)
			findings = append(findings, Finding{
				Code: FindingHighVolumeByPreparer, Severity: SeverityInfo,
				Period: members[0].period, EntryIDs: ids,
				Amount: AvailableValue(total),
				Evidence: Evidence{
					PreparerID: p, PreparerEntryCount: AvailableValue(float64(len(members))),
					EntryIDs: ids, CombinedAmount: AvailableValue(total),
				},
				Message: msgHighVolumeByPreparer,
			})
		}
	}

	return findings, RuleAvailable
}
