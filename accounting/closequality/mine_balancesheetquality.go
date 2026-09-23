package closequality

// mineBalanceSheetQuality covers the two statement/rollforward-level
// balance-sheet checks that are not per-account AccountExpectation rules
// (those live in DimensionAccountBalancePlausibility instead — see
// mine_balancesheet.go): an explicit equity-rollforward review signal
// derived from statements' own balance-sheet reconciliation, and
// caller-supplied opening+movement=closing rollforward checks. Neither
// check fabricates a balancing adjustment or auto-posts net income —
// see package doc comment.
func mineBalanceSheetQuality(in Input, policy Policy) ([]Finding, DimensionResult) {
	b := newDimensionBuilder(DimensionBalanceSheetQuality)

	haveReconciliation := in.statementsAvailable() && len(in.Statements.Reconciliation) > 0
	havePriorBalances := len(policy.PriorPeriodBalances) > 0
	if !haveReconciliation && !havePriorBalances {
		b.unavailable("no balance-sheet reconciliation and no prior-period balances supplied")
		return nil, b.build()
	}
	b.markAssessed()

	var findings []Finding

	for _, r := range in.Statements.Reconciliation {
		if r.Balanced {
			continue
		}
		// The statement-level out-of-balance condition itself is already
		// reported as FindingBalanceSheetOutOfBalance under
		// DimensionFinancialStatementIntegrity (see mine_statements.go).
		// Here we add a neutral, distinct signal specifically inviting
		// review of the equity rollforward, since an unexplained
		// asset/liability/equity imbalance is frequently attributable to
		// current-period earnings not yet rolled into equity.
		f := Finding{
			Code:      FindingEquityRollforwardReview,
			Dimension: DimensionBalanceSheetQuality,
			Severity:  SeverityWarning,
			Message:   "balance sheet does not balance for period " + string(r.Period) + " — review whether current-period earnings have been rolled into equity",
			Evidence: Evidence{
				Difference: AvailableValue(r.Difference),
				Tolerance:  AvailableValue(r.Tolerance),
			},
			SourceModule: SourceStatements,
		}
		findings = append(findings, f)
		b.record(f)
	}
	if !haveReconciliation {
		b.note("no balance-sheet reconciliation supplied")
	} else if len(findings) == 0 {
		b.note("balance sheet reconciliation clean for all periods — no equity rollforward review needed")
	}

	for _, pb := range policy.PriorPeriodBalances {
		tol := pb.Tolerance
		expectedClosing := pb.OpeningBalance + pb.PeriodMovement
		diff := pb.ClosingBalance - expectedClosing
		if abs(diff) <= tol {
			continue
		}
		f := Finding{
			Code:      FindingBalanceRollforwardMismatch,
			Dimension: DimensionBalanceSheetQuality,
			Severity:  SeverityWarning,
			Message:   "account's opening balance plus period movement does not tie to its supplied closing balance",
			Evidence: Evidence{
				OpeningBalance:         AvailableValue(pb.OpeningBalance),
				Movement:               AvailableValue(pb.PeriodMovement),
				ClosingBalance:         AvailableValue(pb.ClosingBalance),
				ExpectedClosingBalance: AvailableValue(expectedClosing),
				Difference:             AvailableValue(diff),
				Tolerance:              AvailableValue(tol),
			},
			SourceModule: SourceCloseQuality,
			AccountIDs:   []string{pb.AccountID},
		}
		findings = append(findings, f)
		b.record(f)
	}

	return findings, b.build()
}
