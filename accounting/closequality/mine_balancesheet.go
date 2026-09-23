package closequality

import "github.com/themurtez/go-valuate/accounting/ledger"

// endingBalances computes each account's ending DisplayBalance as of
// Input.Period.EndDate, using ledger.CalculateBalances directly against
// Input.Ledger — this package's own read of ending balances for
// plausibility checks, since ledger.Balance itself is not part of any
// sibling Result. Returns nil if no ledger was provided.
func endingBalances(in Input) map[string]ledger.Balance {
	if !in.LedgerProvided {
		return nil
	}
	chart := in.Ledger.Chart()
	balances := ledger.CalculateBalances(chart, in.Ledger.Entries, ledger.BalanceOptions{
		Range: ledger.PeriodRange{EndDate: in.Period.EndDate},
	})
	out := make(map[string]ledger.Balance, len(balances))
	for _, b := range balances {
		out[b.AccountID] = b
	}
	return out
}

func expectationsByAccount(policy Policy) (map[string]AccountExpectation, []Issue) {
	out := make(map[string]AccountExpectation, len(policy.AccountExpectations))
	var issues []Issue
	for _, e := range policy.AccountExpectations {
		if e.AccountID == "" {
			issues = append(issues, Issue{
				Code:     IssueInvalidAccountExpectation,
				Severity: IssueSeverityError,
				Message:  "account expectation has no account_id",
			})
			continue
		}
		if _, dup := out[e.AccountID]; dup {
			issues = append(issues, Issue{
				Code:      IssueInvalidAccountExpectation,
				Severity:  IssueSeverityWarning,
				Message:   "duplicate account expectation for account; only the first is used",
				AccountID: e.AccountID,
			})
			continue
		}
		out[e.AccountID] = e
	}
	return out, issues
}

// mineAccountBalancePlausibility evaluates Policy.AccountExpectations
// against each named account's ending balance: sign checks
// (ExpectedSign/AllowZero/AllowNegative), Min/MaxBalance range checks,
// and ShouldClear (routed through FindingClearingAccountNotCleared
// instead — see mineClearingAndStaleBalances). Opposite-sign balances
// are never treated as automatically wrong; a check only fires when the
// caller has explicitly declared an expectation for that account (see
// AccountExpectation's doc comment).
func mineAccountBalancePlausibility(in Input, policy Policy) ([]Finding, DimensionResult, []Issue) {
	b := newDimensionBuilder(DimensionAccountBalancePlausibility)

	expectations, issues := expectationsByAccount(policy)
	if len(expectations) == 0 {
		b.unavailable("no account expectations declared in policy")
		return nil, b.build(), issues
	}
	balances := endingBalances(in)
	if balances == nil {
		b.unavailable("no ledger supplied — account expectations cannot be evaluated against ending balances")
		return nil, b.build(), issues
	}
	b.markAssessed()

	var findings []Finding
	for _, accountID := range sortedExpectationAccountIDs(expectations) {
		exp := expectations[accountID]
		if exp.ShouldClear {
			continue // handled by mineClearingAndStaleBalances
		}
		bal, ok := balances[accountID]
		if !ok {
			issues = append(issues, Issue{
				Code:      IssueUnknownAccount,
				Severity:  IssueSeverityWarning,
				Message:   "account expectation references an account with no ledger activity or opening balance",
				AccountID: accountID,
			})
			continue
		}
		if f, violated := evaluateExpectation(exp, bal.DisplayBalance); violated {
			findings = append(findings, f)
			b.record(f)
		}
	}
	if len(findings) == 0 {
		b.note("all declared account expectations satisfied")
	}
	return findings, b.build(), issues
}

func sortedExpectationAccountIDs(m map[string]AccountExpectation) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return uniqueSortedStrings(ids)
}

func evaluateExpectation(exp AccountExpectation, balance float64) (Finding, bool) {
	condition, violated := "", false
	switch exp.ExpectedSign {
	case ExpectedSignPositive, ExpectedSignNormal:
		if balance < 0 || (balance == 0 && !exp.AllowZero && !exp.AllowNegative) {
			condition, violated = "expected balance >= 0", balance < 0
		}
	case ExpectedSignNegative:
		if balance > 0 || (balance == 0 && !exp.AllowZero) {
			condition, violated = "expected balance <= 0", balance > 0
		}
	}
	if !violated && exp.MinBalance != nil && balance < *exp.MinBalance {
		condition, violated = "expected balance >= declared minimum", true
	}
	if !violated && exp.MaxBalance != nil && balance > *exp.MaxBalance {
		condition, violated = "expected balance <= declared maximum", true
	}
	if !violated {
		return Finding{}, false
	}
	sev := SeverityWarning
	if exp.Materiality != nil && abs(balance) < *exp.Materiality {
		sev = SeverityInfo
	}
	f := Finding{
		Code:      FindingUnexpectedAccountBalance,
		Dimension: DimensionAccountBalancePlausibility,
		Severity:  sev,
		Message:   "account balance does not satisfy its declared expectation",
		Evidence: Evidence{
			EndingBalance:     AvailableValue(balance),
			ExpectedCondition: condition,
			Label:             exp.Label,
		},
		SourceModule: SourceCloseQuality,
		AccountIDs:   []string{exp.AccountID},
	}
	return f, true
}

// mineClearingAndStaleBalances evaluates ShouldClear expectations
// (FindingClearingAccountNotCleared) and, when a matching BalanceAge is
// supplied, staleness (FindingStaleAccountBalance). Suspense/clearing
// accounts are never auto-detected from name — only accounts the caller
// explicitly marked ShouldClear are considered.
func mineClearingAndStaleBalances(in Input, policy Policy) ([]Finding, []string) {
	expectations, _ := expectationsByAccount(policy)
	balances := endingBalances(in)
	ages := make(map[string]BalanceAge, len(policy.BalanceAges))
	for _, a := range policy.BalanceAges {
		ages[a.AccountID] = a
	}

	var findings []Finding
	var notes []string
	for _, accountID := range sortedExpectationAccountIDs(expectations) {
		exp := expectations[accountID]
		if !exp.ShouldClear {
			continue
		}
		var balance float64
		var haveBalance bool
		if balances != nil {
			if bal, ok := balances[accountID]; ok {
				balance, haveBalance = bal.DisplayBalance, true
			}
		}
		if !haveBalance {
			notes = append(notes, "clearing account "+accountID+": ending balance unavailable (no ledger)")
			continue
		}
		if balance == 0 {
			notes = append(notes, "clearing account "+accountID+" cleared to zero")
			continue
		}
		material := true
		if exp.Materiality != nil {
			material = abs(balance) >= *exp.Materiality
		}
		sev := SeverityWarning
		if !material {
			sev = SeverityInfo
		}
		f := Finding{
			Code:      FindingClearingAccountNotCleared,
			Dimension: DimensionAccountBalancePlausibility,
			Severity:  sev,
			Message:   "clearing/suspense account has a nonzero ending balance",
			Evidence: Evidence{
				EndingBalance:        AvailableValue(balance),
				MaterialityThreshold: materialityFromPointer(exp.Materiality),
				Label:                exp.Label,
			},
			SourceModule: SourceCloseQuality,
			AccountIDs:   []string{accountID},
		}
		findings = append(findings, f)

		if age, ok := ages[accountID]; ok && exp.MaxAgeDays != nil {
			days, ok := balanceAgeDays(age.OldestOpenDate, in.Period.EndDate)
			if ok && days > *exp.MaxAgeDays {
				sf := Finding{
					Code:      FindingStaleAccountBalance,
					Dimension: DimensionAccountBalancePlausibility,
					Severity:  SeverityWarning,
					Message:   "clearing/suspense account balance has been outstanding longer than the declared maximum age",
					Evidence: Evidence{
						EndingBalance:  AvailableValue(age.Amount),
						OldestOpenDate: age.OldestOpenDate,
						AsOfDate:       in.Period.EndDate,
						Label:          exp.Label,
					},
					SourceModule: SourceCloseQuality,
					AccountIDs:   []string{accountID},
				}
				findings = append(findings, sf)
			}
		}
	}
	return findings, notes
}

func materialityFromPointer(p *float64) Value {
	if p == nil {
		return Unavailable()
	}
	return AvailableValue(*p)
}
