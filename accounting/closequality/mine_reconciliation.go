package closequality

// reconciliationIssues validates Input.Reconciliations for duplicates
// and non-finite values, returning the deduplicated-by-first-seen map
// (keyed by AccountID) alongside any Issues found. This package performs
// no matching itself — see ReconciliationStatus's doc comment.
func reconciliationIssues(statuses []ReconciliationStatus) (map[string]ReconciliationStatus, []Issue) {
	byAccount := make(map[string]ReconciliationStatus, len(statuses))
	var issues []Issue
	for _, s := range statuses {
		if isNonFinite(s.Difference.Amount) || isNonFinite(s.BookBalance.Amount) || isNonFinite(s.ReconciledBalance.Amount) {
			issues = append(issues, Issue{
				Code:      IssueNonFiniteValue,
				Severity:  IssueSeverityError,
				Message:   "reconciliation status has a non-finite amount and was excluded",
				AccountID: s.AccountID,
			})
			continue
		}
		if _, dup := byAccount[s.AccountID]; dup {
			issues = append(issues, Issue{
				Code:      IssueDuplicateReconciliationStatus,
				Severity:  IssueSeverityWarning,
				Message:   "duplicate reconciliation status for account; only the first is used",
				AccountID: s.AccountID,
			})
			continue
		}
		byAccount[s.AccountID] = s
	}
	return byAccount, issues
}

// ReconciliationCoverage summarizes Input.Reconciliations against
// Policy.RequiredReconciliationAccounts. Accounts not named as required
// are not counted in the denominator — this package never assumes every
// account requires reconciliation.
type ReconciliationCoverage struct {
	RequiredAccounts            []string `json:"required_accounts,omitempty"`
	ReconciledAccounts          []string `json:"reconciled_accounts,omitempty"`
	ReconciledWithItemsAccounts []string `json:"reconciled_with_items_accounts,omitempty"`
	UnreconciledAccounts        []string `json:"unreconciled_accounts,omitempty"`
	UnavailableAccounts         []string `json:"unavailable_accounts,omitempty"`
	CoveragePercent             float64  `json:"coverage_percent"`
}

func mineReconciliationCoverage(in Input, policy Policy) ([]Finding, DimensionResult, ReconciliationCoverage, []Issue) {
	b := newDimensionBuilder(DimensionReconciliationCoverage)

	if len(policy.RequiredReconciliationAccounts) == 0 {
		b.unavailable("no required reconciliation accounts declared in policy")
		return nil, b.build(), ReconciliationCoverage{}, nil
	}
	b.markAssessed()

	byAccount, issues := reconciliationIssues(in.Reconciliations)
	required := uniqueSortedStrings(policy.RequiredReconciliationAccounts)

	cov := ReconciliationCoverage{RequiredAccounts: required}
	var findings []Finding

	for _, accountID := range required {
		status, ok := byAccount[accountID]
		if !ok || status.Status == ReconciliationUnavailable || status.Status == "" {
			cov.UnavailableAccounts = append(cov.UnavailableAccounts, accountID)
			sev := SeverityWarning
			if policy.isCriticalAccount(accountID) {
				sev = SeverityBlocking
			}
			f := Finding{
				Code:         FindingUnreconciledCriticalAccount,
				Dimension:    DimensionReconciliationCoverage,
				Severity:     sev,
				Message:      "required reconciliation account has no reconciliation status available",
				SourceModule: SourceCloseQuality,
				AccountIDs:   []string{accountID},
			}
			findings = append(findings, f)
			b.record(f)
			continue
		}

		switch status.Status {
		case ReconciliationReconciled:
			cov.ReconciledAccounts = append(cov.ReconciledAccounts, accountID)
		case ReconciliationNotRequired:
			// Declared required by policy but the reconciler marked it
			// not-required for this period — trust the caller's status,
			// count as reconciled for coverage purposes.
			cov.ReconciledAccounts = append(cov.ReconciledAccounts, accountID)
		case ReconciliationReconciledWithItems:
			cov.ReconciledWithItemsAccounts = append(cov.ReconciledWithItemsAccounts, accountID)
			if status.UnresolvedCount > policy.MaxAllowedUnreconciledItems {
				f := Finding{
					Code:      FindingReconciliationItemsOutstanding,
					Dimension: DimensionReconciliationCoverage,
					Severity:  SeverityWarning,
					Message:   "reconciliation completed with unresolved outstanding items",
					Evidence: Evidence{
						UnresolvedCount: AvailableValue(float64(status.UnresolvedCount)),
						Difference:      status.Difference,
					},
					SourceModule: SourceCloseQuality,
					AccountIDs:   []string{accountID},
				}
				if policy.isCriticalAccount(accountID) {
					f.Severity = SeverityBlocking
				}
				findings = append(findings, f)
				b.record(f)
			}
		case ReconciliationUnreconciled:
			cov.UnreconciledAccounts = append(cov.UnreconciledAccounts, accountID)
			sev := SeverityWarning
			if policy.isCriticalAccount(accountID) {
				sev = SeverityBlocking
			}
			f := Finding{
				Code:      FindingUnreconciledCriticalAccount,
				Dimension: DimensionReconciliationCoverage,
				Severity:  sev,
				Message:   "required reconciliation account is unreconciled",
				Evidence: Evidence{
					Difference: status.Difference,
					AsOfDate:   status.AsOfDate,
				},
				SourceModule: SourceCloseQuality,
				AccountIDs:   []string{accountID},
			}
			findings = append(findings, f)
			b.record(f)
		}
	}

	total := len(required)
	settled := len(cov.ReconciledAccounts) + len(cov.ReconciledWithItemsAccounts)
	if total > 0 {
		cov.CoveragePercent = float64(settled) / float64(total)
	}
	if len(findings) == 0 {
		b.note("all required reconciliation accounts reconciled")
	}

	return findings, b.build(), cov, issues
}
