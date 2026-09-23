package closequality

import "strings"

// mineDataCompleteness evaluates Applicability against Coverage,
// producing FindingMissingRequiredInput for every required-but-missing
// module. A missing module that is not required never produces a
// finding — see Applicability's doc comment.
func mineDataCompleteness(cov Coverage, policy Policy) ([]Finding, DimensionResult) {
	b := newDimensionBuilder(DimensionDataCompleteness)
	b.markAssessed() // always assessable: Coverage is always computed

	moduleLabel := map[string]string{
		"ledger":              "ledger",
		"statements":          "statements",
		"ar":                  "AR",
		"ap":                  "AP",
		"journal_diagnostics": "journal diagnostics",
		"reconciliations":     "reconciliations",
		"close_tasks":         "close tasks",
	}

	var findings []Finding
	for _, m := range cov.MissingModules {
		sev := SeverityWarning
		if policy.TreatMissingRequiredInputAsBlocker {
			sev = SeverityBlocking
		}
		f := Finding{
			Code:         FindingMissingRequiredInput,
			Dimension:    DimensionDataCompleteness,
			Severity:     sev,
			Message:      "required input module (" + moduleLabel[m] + ") was not supplied",
			SourceModule: SourceCloseQuality,
			SourceCode:   m,
		}
		findings = append(findings, f)
		b.record(f)
	}
	if len(findings) == 0 {
		b.note("all required input modules available")
	}
	if len(cov.NotApplicableModules) > 0 {
		b.note("modules not required and not supplied: " + strings.Join(cov.NotApplicableModules, ", "))
	}
	return findings, b.build()
}

// mineExpectedActivity evaluates Policy.ExpectedActivityRules against
// Input.Ledger entries. Unavailable (rather than "missing") when no
// ledger is supplied, since the rule genuinely cannot be evaluated.
// Findings from this check are folded into DimensionDataCompleteness
// (it is fundamentally a completeness question: did expected activity
// occur).
func mineExpectedActivity(in Input, policy Policy) []Finding {
	if len(policy.ExpectedActivityRules) == 0 || !in.LedgerProvided {
		return nil
	}
	accountSet := make(map[string]bool)
	for _, e := range in.Ledger.Entries {
		if !in.Period.dateInRange(e.Date) {
			continue
		}
		for _, l := range e.Lines {
			accountSet[l.AccountID] = true
		}
	}

	var findings []Finding
	for _, rule := range policy.ExpectedActivityRules {
		count := 0
		var absMove float64
		for _, e := range in.Ledger.Entries {
			if !in.Period.dateInRange(e.Date) {
				continue
			}
			matched := false
			for _, l := range e.Lines {
				if containsString(rule.AccountIDs, l.AccountID) {
					matched = true
					absMove += abs(l.Debit - l.Credit)
				}
			}
			if matched {
				count++
			}
		}
		if count >= rule.MinEntryCount && absMove >= rule.MinAbsoluteMove {
			continue
		}
		f := Finding{
			Code:      FindingExpectedPeriodEntryMissing,
			Dimension: DimensionDataCompleteness,
			Severity:  SeverityWarning,
			Message:   "expected period activity was not observed for the declared account set",
			Evidence: Evidence{
				Label:    rule.Label,
				Movement: AvailableValue(absMove),
			},
			SourceModule: SourceCloseQuality,
			AccountIDs:   uniqueSortedStrings(rule.AccountIDs),
		}
		findings = append(findings, f)
	}
	return findings
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// mineExpectedPeriodEntries evaluates Policy.ExpectedPeriodEntries
// against Input.Ledger entries using only exact, deterministic matching
// (account membership, exact normalized description, source, amount
// range) — no fuzzy/NLP matching (see ExpectedPeriodEntry's doc
// comment). A rule with no usable criteria, or with no ledger supplied,
// is unavailable and produces no finding either way.
func mineExpectedPeriodEntries(in Input, policy Policy) []Finding {
	if len(policy.ExpectedPeriodEntries) == 0 || !in.LedgerProvided {
		return nil
	}

	var findings []Finding
	for _, rule := range policy.ExpectedPeriodEntries {
		if !expectedPeriodEntryEvaluable(rule) {
			continue
		}
		if expectedPeriodEntryMatches(in, rule) {
			continue
		}
		f := Finding{
			Code:      FindingExpectedPeriodEntryMissing,
			Dimension: DimensionDataCompleteness,
			Severity:  SeverityWarning,
			Message:   "expected recurring close entry was not found in this period's ledger activity",
			Evidence: Evidence{
				Label: rule.Label,
			},
			SourceModule: SourceCloseQuality,
			AccountIDs:   uniqueSortedStrings(rule.AccountIDs),
		}
		findings = append(findings, f)
	}
	return findings
}

func expectedPeriodEntryEvaluable(rule ExpectedPeriodEntry) bool {
	return len(rule.AccountIDs) > 0 || rule.Source != "" || rule.DescriptionExact != ""
}

func expectedPeriodEntryMatches(in Input, rule ExpectedPeriodEntry) bool {
	normDesc := ""
	if rule.DescriptionExact != "" {
		normDesc = normalizeDescription(rule.DescriptionExact)
	}
	for _, e := range in.Ledger.Entries {
		if !in.Period.dateInRange(e.Date) {
			continue
		}
		if rule.Source != "" && e.Source != rule.Source {
			continue
		}
		if normDesc != "" && normalizeDescription(e.Description) != normDesc {
			continue
		}
		if len(rule.AccountIDs) > 0 {
			matched := false
			for _, l := range e.Lines {
				if containsString(rule.AccountIDs, l.AccountID) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if rule.MinAmount != nil || rule.MaxAmount != nil {
			amount := e.TotalDebits()
			if rule.MinAmount != nil && amount < *rule.MinAmount {
				continue
			}
			if rule.MaxAmount != nil && amount > *rule.MaxAmount {
				continue
			}
		}
		return true
	}
	return false
}

func (p PeriodInfo) dateInRange(date string) bool {
	if date == "" {
		return false
	}
	if p.StartDate != "" && date < p.StartDate {
		return false
	}
	if p.EndDate != "" && date > p.EndDate {
		return false
	}
	return true
}
