package reconciliation

import "time"

const dateLayout = "2006-01-02"

func parseDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// validReconciliationKey reports whether accountID/externalAccountID are
// non-empty enough to identify a reconciliation — task section 31. At
// least AccountID must be non-empty; ExternalAccountID is recommended but
// not strictly required (some reconciliation types, e.g. GENERIC
// balance-only, may not have a distinct external account identifier).
func validReconciliationKey(accountID string) bool {
	return accountID != ""
}

// dedupBookItems filters items to the deduplicated-by-first-seen set
// (keyed by ItemID) with structural validation, returning the valid,
// deduplicated slice (input order preserved) plus every Issue found.
// Non-finite amounts and unrecognized directions exclude an item
// entirely (never silently coerced) — invalid items are counted in
// neither MatchedGroups nor UnmatchedBookItems, since they cannot be
// meaningfully compared at all; this is a deliberately small, explicit
// exclusion set (only two conditions), not a broad silent-repair pass.
func dedupBookItems(items []BookItem, reportingCurrency string) ([]BookItem, []Issue) {
	seen := make(map[string]bool, len(items))
	var out []BookItem
	var issues []Issue
	for _, it := range items {
		if it.ItemID == "" {
			issues = append(issues, Issue{
				Code: IssueDuplicateBookItem, Severity: IssueSeverityError,
				Message: "book item has an empty item_id and was excluded",
			})
			continue
		}
		if seen[it.ItemID] {
			issues = append(issues, Issue{
				Code: IssueDuplicateBookItem, Severity: IssueSeverityWarning,
				Message: "duplicate book item_id; only the first occurrence is used", ItemID: it.ItemID,
			})
			continue
		}
		seen[it.ItemID] = true

		if isNonFinite(it.Amount) {
			issues = append(issues, Issue{
				Code: IssueNonFiniteAmount, Severity: IssueSeverityError,
				Message: "book item has a non-finite amount and was excluded", ItemID: it.ItemID,
			})
			continue
		}
		if it.Amount < 0 {
			issues = append(issues, Issue{
				Code: IssueInvalidAmount, Severity: IssueSeverityWarning,
				Message: "book item amount is negative; direction should carry the sign", ItemID: it.ItemID,
			})
		}
		if !isRecognizedDirection(it.Direction) {
			issues = append(issues, Issue{
				Code: IssueInvalidDirection, Severity: IssueSeverityError,
				Message: "book item has an unrecognized direction and was excluded", ItemID: it.ItemID,
			})
			continue
		}
		if _, ok := parseDate(it.Date); it.Date != "" && !ok {
			issues = append(issues, Issue{
				Code: IssueInvalidAmount, Severity: IssueSeverityWarning,
				Message: "book item date could not be parsed", ItemID: it.ItemID,
			})
		}
		if reportingCurrency != "" && it.Currency != "" && it.Currency != reportingCurrency {
			issues = append(issues, Issue{
				Code: IssueMixedCurrency, Severity: IssueSeverityWarning,
				Message: "book item currency does not match the declared reporting currency", ItemID: it.ItemID,
			})
		}

		out = append(out, it)
	}
	return out, issues
}

// dedupExternalItems mirrors dedupBookItems for ExternalItem.
func dedupExternalItems(items []ExternalItem, reportingCurrency string) ([]ExternalItem, []Issue) {
	seen := make(map[string]bool, len(items))
	var out []ExternalItem
	var issues []Issue
	for _, it := range items {
		if it.ItemID == "" {
			issues = append(issues, Issue{
				Code: IssueDuplicateExternalItem, Severity: IssueSeverityError,
				Message: "external item has an empty item_id and was excluded",
			})
			continue
		}
		if seen[it.ItemID] {
			issues = append(issues, Issue{
				Code: IssueDuplicateExternalItem, Severity: IssueSeverityWarning,
				Message: "duplicate external item_id; only the first occurrence is used", ItemID: it.ItemID,
			})
			continue
		}
		seen[it.ItemID] = true

		if isNonFinite(it.Amount) {
			issues = append(issues, Issue{
				Code: IssueNonFiniteAmount, Severity: IssueSeverityError,
				Message: "external item has a non-finite amount and was excluded", ItemID: it.ItemID,
			})
			continue
		}
		if it.Amount < 0 {
			issues = append(issues, Issue{
				Code: IssueInvalidAmount, Severity: IssueSeverityWarning,
				Message: "external item amount is negative; direction should carry the sign", ItemID: it.ItemID,
			})
		}
		if !isRecognizedDirection(it.Direction) {
			issues = append(issues, Issue{
				Code: IssueInvalidDirection, Severity: IssueSeverityError,
				Message: "external item has an unrecognized direction and was excluded", ItemID: it.ItemID,
			})
			continue
		}
		if _, ok := parseDate(it.Date); it.Date != "" && !ok {
			issues = append(issues, Issue{
				Code: IssueInvalidAmount, Severity: IssueSeverityWarning,
				Message: "external item date could not be parsed", ItemID: it.ItemID,
			})
		}
		if reportingCurrency != "" && it.Currency != "" && it.Currency != reportingCurrency {
			issues = append(issues, Issue{
				Code: IssueMixedCurrency, Severity: IssueSeverityWarning,
				Message: "external item currency does not match the declared reporting currency", ItemID: it.ItemID,
			})
		}

		out = append(out, it)
	}
	return out, issues
}

// dedupReconcilingItems filters ReconcilingItems for empty/duplicate
// ItemID and structurally invalid Type/Side/Amount.
func dedupReconcilingItems(items []ReconcilingItem) ([]ReconcilingItem, []Issue) {
	seen := make(map[string]bool, len(items))
	var out []ReconcilingItem
	var issues []Issue
	for _, it := range items {
		if it.ItemID == "" {
			issues = append(issues, Issue{
				Code: IssueInvalidReconcilingItem, Severity: IssueSeverityError,
				Message: "reconciling item has an empty item_id and was excluded",
			})
			continue
		}
		if seen[it.ItemID] {
			issues = append(issues, Issue{
				Code: IssueInvalidReconcilingItem, Severity: IssueSeverityWarning,
				Message: "duplicate reconciling item_id; only the first occurrence is used", ItemID: it.ItemID,
			})
			continue
		}
		seen[it.ItemID] = true

		if !isRecognizedReconcilingType(it.Type) {
			issues = append(issues, Issue{
				Code: IssueInvalidReconcilingItem, Severity: IssueSeverityError,
				Message: "reconciling item has an unrecognized type and was excluded", ItemID: it.ItemID,
			})
			continue
		}
		if it.Side != ReconcilingSideBook && it.Side != ReconcilingSideExternal {
			issues = append(issues, Issue{
				Code: IssueInvalidReconcilingItem, Severity: IssueSeverityError,
				Message: "reconciling item has an unrecognized side and was excluded", ItemID: it.ItemID,
			})
			continue
		}
		if isNonFinite(it.Amount) {
			issues = append(issues, Issue{
				Code: IssueInvalidReconcilingItem, Severity: IssueSeverityError,
				Message: "reconciling item has a non-finite amount and was excluded", ItemID: it.ItemID,
			})
			continue
		}
		out = append(out, it)
	}
	return out, issues
}
