package cashforecast

import "math"

func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}

// validatedEvent reports whether e passes structural validation, and every
// Issue found. An event failing validation is excluded from every
// downstream calculation — see the task's section 45.
func validatedEvent(e CashFlowEvent) (bool, []Issue) {
	var issues []Issue
	ok := true

	if e.ID == "" || e.Date.IsZero() {
		issues = append(issues, Issue{Code: IssueInvalidEvent, Severity: SeverityError,
			Message: "event has an empty ID or zero-value Date", EventID: e.ID})
		ok = false
	}
	if !isRecognizedDirection(e.Direction) {
		issues = append(issues, Issue{Code: IssueInvalidDirection, Severity: SeverityError,
			Message: "event has an unrecognized Direction", EventID: e.ID})
		ok = false
	}
	if _, recognized := categoryDirection(e.Category); !recognized {
		issues = append(issues, Issue{Code: IssueInvalidCategory, Severity: SeverityError,
			Message: "event has an unrecognized Category", EventID: e.ID})
		ok = false
	} else if expected, _ := categoryDirection(e.Category); isRecognizedDirection(e.Direction) && expected != e.Direction {
		issues = append(issues, Issue{Code: IssueCategoryDirectionMismatch, Severity: SeverityError,
			Message: "event Category's expected direction does not match Direction", EventID: e.ID})
		ok = false
	}
	if !isRecognizedBasis(e.Basis) {
		issues = append(issues, Issue{Code: IssueInvalidEvent, Severity: SeverityError,
			Message: "event has an unrecognized Basis", EventID: e.ID})
		ok = false
	}
	if isNonFinite(e.Amount) {
		issues = append(issues, Issue{Code: IssueNonFiniteAmount, Severity: SeverityError,
			Message: "event Amount is NaN or Inf", EventID: e.ID})
		ok = false
	} else if e.Amount < 0 {
		issues = append(issues, Issue{Code: IssueNegativeAmount, Severity: SeverityError,
			Message: "event Amount is negative; Direction determines sign, Amount must be >= 0", EventID: e.ID})
		ok = false
	}
	if !isRecognizedCertainty(e.Certainty) {
		issues = append(issues, Issue{Code: IssueInvalidEvent, Severity: SeverityWarning,
			Message: "event has an unrecognized Certainty", EventID: e.ID})
	}
	if !isRecognizedCommitment(e.Commitment) {
		issues = append(issues, Issue{Code: IssueInvalidEvent, Severity: SeverityWarning,
			Message: "event has an unrecognized Commitment", EventID: e.ID})
	}
	if !isRecognizedPriority(e.Priority) {
		issues = append(issues, Issue{Code: IssueInvalidEvent, Severity: SeverityWarning,
			Message: "event has an unrecognized Priority", EventID: e.ID})
	}

	return ok, issues
}

// validateAndDedupeEvents validates every event, drops invalid ones, and
// drops duplicate IDs (keeping the first occurrence in input order) —
// returns the clean slice plus every Issue found, in generation order.
func validateAndDedupeEvents(events []CashFlowEvent) ([]CashFlowEvent, []Issue) {
	var issues []Issue
	seen := make(map[string]bool, len(events))
	out := make([]CashFlowEvent, 0, len(events))
	for _, e := range events {
		if seen[e.ID] && e.ID != "" {
			issues = append(issues, Issue{Code: IssueDuplicateEvent, Severity: SeverityWarning,
				Message: "duplicate event ID; only the first occurrence is used", EventID: e.ID})
			continue
		}
		if e.ID != "" {
			seen[e.ID] = true
		}
		ok, eventIssues := validatedEvent(e)
		issues = append(issues, eventIssues...)
		if ok {
			out = append(out, e)
		}
	}
	return out, issues
}

// validateOpeningCash checks Input.OpeningCash/CashAccounts structural
// validity and currency consistency against reportingCurrency.
func validateOpeningCash(opening OpeningCash, accounts []CashAccount, reportingCurrency string) []Issue {
	var issues []Issue
	if opening.Currency == "" && len(accounts) == 0 {
		issues = append(issues, Issue{Code: IssueInvalidOpeningCash, Severity: SeverityError,
			Message: "OpeningCash.Currency is required when CashAccounts is not supplied"})
	}
	if opening.Currency != "" && reportingCurrency != "" && opening.Currency != reportingCurrency {
		issues = append(issues, Issue{Code: IssueMixedCurrency, Severity: SeverityWarning,
			Message: "OpeningCash.Currency does not match the resolved reporting currency"})
	}
	seenAccounts := make(map[string]bool, len(accounts))
	var sum float64
	for _, a := range accounts {
		if a.AccountID == "" {
			issues = append(issues, Issue{Code: IssueInvalidOpeningCash, Severity: SeverityError,
				Message: "CashAccount has an empty AccountID"})
			continue
		}
		if seenAccounts[a.AccountID] {
			issues = append(issues, Issue{Code: IssueInvalidOpeningCash, Severity: SeverityWarning,
				Message: "duplicate CashAccount.AccountID", SourceID: a.AccountID})
			continue
		}
		seenAccounts[a.AccountID] = true
		if reportingCurrency != "" && a.Currency != "" && a.Currency != reportingCurrency {
			issues = append(issues, Issue{Code: IssueMixedCurrency, Severity: SeverityWarning,
				Message: "CashAccount.Currency does not match the resolved reporting currency", SourceID: a.AccountID})
			continue
		}
		sum += a.Balance
	}
	if len(accounts) > 0 && opening.Amount != 0 && math.Abs(sum-opening.Amount) > amountTolerance {
		issues = append(issues, Issue{Code: IssueOpeningCashMismatch, Severity: SeverityWarning,
			Message: "sum of CashAccounts balances does not match OpeningCash.Amount"})
	}
	return issues
}

// validateScenarios checks Scenario.Label uniqueness/non-emptiness and
// that no Scenario.Label collides with BaseScenarioLabel, returning only
// the scenarios that passed (matching validateFacilities' identical
// "return the valid subset plus every Issue" shape) — a single invalid
// scenario must never silently drop every other, otherwise-valid,
// scenario from the Result along with it.
func validateScenarios(scenarios []Scenario) ([]Scenario, []Issue) {
	var issues []Issue
	var valid []Scenario
	seen := make(map[string]bool, len(scenarios))
	for _, s := range scenarios {
		if s.Label == "" {
			issues = append(issues, Issue{Code: IssueInvalidScenario, Severity: SeverityError,
				Message: "scenario has an empty Label"})
			continue
		}
		if s.Label == BaseScenarioLabel {
			issues = append(issues, Issue{Code: IssueInvalidScenario, Severity: SeverityError,
				Message: "scenario Label collides with the reserved base-scenario label", SourceID: s.Label})
			continue
		}
		if seen[s.Label] {
			issues = append(issues, Issue{Code: IssueInvalidScenario, Severity: SeverityError,
				Message: "duplicate scenario Label", SourceID: s.Label})
			continue
		}
		seen[s.Label] = true
		for _, tr := range s.Transforms {
			if !isRecognizedTransformKind(tr.Kind) {
				issues = append(issues, Issue{Code: IssueInvalidScenario, Severity: SeverityWarning,
					Message: "scenario has a transform with an unrecognized Kind", SourceID: s.Label})
			}
		}
		valid = append(valid, s)
	}
	return valid, issues
}

// validateFacilities checks CreditFacility structural validity; an
// invalid facility is excluded from totalFacilityCapacity.
func validateFacilities(facilities []CreditFacility) ([]CreditFacility, []Issue) {
	var issues []Issue
	var valid []CreditFacility
	for _, f := range facilities {
		if f.FacilityID == "" {
			issues = append(issues, Issue{Code: IssueInvalidFacility, Severity: SeverityWarning,
				Message: "credit facility has an empty FacilityID"})
			continue
		}
		if f.AvailableToDraw < 0 {
			issues = append(issues, Issue{Code: IssueInvalidFacility, Severity: SeverityWarning,
				Message: "credit facility has a negative AvailableToDraw", SourceID: f.FacilityID})
			continue
		}
		if f.MinimumDraw > 0 && f.MaximumDraw > 0 && f.MinimumDraw > f.MaximumDraw {
			issues = append(issues, Issue{Code: IssueInvalidFacility, Severity: SeverityWarning,
				Message: "credit facility MinimumDraw exceeds MaximumDraw", SourceID: f.FacilityID})
			continue
		}
		valid = append(valid, f)
	}
	return valid, issues
}

// validateRecurringRule checks one RecurringRule's structural validity.
func validateRecurringRule(rule RecurringRule) []Issue {
	var issues []Issue
	if rule.Amount <= 0 || isNonFinite(rule.Amount) {
		issues = append(issues, Issue{Code: IssueInvalidRecurringRule, Severity: SeverityError,
			Message: "recurring rule Amount must be > 0 and finite", SourceID: rule.ID})
	}
	if rule.StartDate.IsZero() {
		issues = append(issues, Issue{Code: IssueInvalidRecurringRule, Severity: SeverityError,
			Message: "recurring rule StartDate is required", SourceID: rule.ID})
	}
	if !rule.EndDate.IsZero() && rule.EndDate.Before(rule.StartDate) {
		issues = append(issues, Issue{Code: IssueInvalidRecurringRule, Severity: SeverityError,
			Message: "recurring rule EndDate is before StartDate", SourceID: rule.ID})
	}
	if !isRecognizedFrequency(rule.Frequency) {
		issues = append(issues, Issue{Code: IssueInvalidRecurringRule, Severity: SeverityError,
			Message: "recurring rule has an unrecognized Frequency", SourceID: rule.ID})
	}
	if _, recognized := categoryDirection(rule.Category); !recognized {
		issues = append(issues, Issue{Code: IssueInvalidRecurringRule, Severity: SeverityError,
			Message: "recurring rule has an unrecognized Category", SourceID: rule.ID})
	}
	return issues
}
