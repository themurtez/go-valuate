package closequality

import "github.com/themurtez/go-valuate/accounting/ledger"

// resolvedLedgerValidation returns the ledger validation issues to use:
// Input.LedgerValidation verbatim if supplied (never re-run — "reuse,
// don't recalculate"), otherwise a fresh ledger.Ledger.Validate call
// against Input.Ledger with zero-value ledger.ValidateOptions, matching
// this package's documented default when a caller hands over a raw
// Ledger without having validated it themselves.
func resolvedLedgerValidation(in Input) []ledger.Issue {
	if in.LedgerValidation != nil {
		return in.LedgerValidation
	}
	if !in.LedgerProvided {
		return nil
	}
	return in.Ledger.Validate(ledger.ValidateOptions{})
}

// ledgerIssueSeverity translates a ledger.Issue into a close-quality
// Severity. Ledger errors (unbalanced entries, unknown accounts,
// invalid debit/credit) are structural defects in the books themselves,
// so they are always BLOCKING; ledger warnings become close-quality
// WARNING.
func ledgerIssueSeverity(sev ledger.IssueSeverity) Severity {
	if sev == ledger.SeverityError {
		return SeverityBlocking
	}
	return SeverityWarning
}

// mineLedgerIntegrity translates ledger validation issues into
// DimensionLedgerIntegrity findings, preserving each Issue's own Code.
// It does not re-derive ledger validation logic — see
// resolvedLedgerValidation.
func mineLedgerIntegrity(in Input) ([]Finding, DimensionResult) {
	b := newDimensionBuilder(DimensionLedgerIntegrity)

	if !in.LedgerProvided && in.LedgerValidation == nil {
		b.unavailable("no ledger or pre-computed ledger validation supplied")
		return nil, b.build()
	}
	b.markAssessed()

	issues := resolvedLedgerValidation(in)
	var findings []Finding
	for _, iss := range issues {
		if iss.Code == ledger.IssueUnbalancedTrialBalance {
			// Trial-balance-level issues are attributed to the
			// TrialBalanceIntegrity dimension instead — see mineTrialBalanceIntegrity.
			continue
		}
		f := Finding{
			Code:         FindingLedgerValidationBlocker,
			Dimension:    DimensionLedgerIntegrity,
			Severity:     ledgerIssueSeverity(iss.Severity),
			Message:      iss.Message,
			SourceModule: SourceLedger,
			SourceCode:   string(iss.Code),
		}
		if iss.Account != "" {
			f.AccountIDs = []string{iss.Account}
		}
		if iss.Entry != "" {
			f.EntryIDs = []string{iss.Entry}
		}
		findings = append(findings, f)
		b.record(f)
	}
	if len(issues) == 0 {
		b.note("ledger validation returned no issues")
	}
	return findings, b.build()
}

// mineTrialBalanceIntegrity checks debits == credits at the ledger
// level, using the ledger's own IssueUnbalancedTrialBalance issue (if
// present in validation) as the single source of truth — this package
// never fabricates a balancing adjustment or recomputes the trial
// balance itself.
func mineTrialBalanceIntegrity(in Input) ([]Finding, DimensionResult) {
	b := newDimensionBuilder(DimensionTrialBalanceIntegrity)

	if !in.LedgerProvided && in.LedgerValidation == nil {
		b.unavailable("no ledger or pre-computed ledger validation supplied")
		return nil, b.build()
	}
	b.markAssessed()

	issues := resolvedLedgerValidation(in)
	var findings []Finding
	for _, iss := range issues {
		if iss.Code != ledger.IssueUnbalancedTrialBalance {
			continue
		}
		f := Finding{
			Code:         FindingTrialBalanceOutOfBalance,
			Dimension:    DimensionTrialBalanceIntegrity,
			Severity:     SeverityBlocking,
			Message:      iss.Message,
			SourceModule: SourceLedger,
			SourceCode:   string(iss.Code),
		}
		if iss.Account != "" {
			f.AccountIDs = []string{iss.Account}
		}
		findings = append(findings, f)
		b.record(f)
	}
	if len(findings) == 0 {
		b.note("no trial-balance-out-of-balance issue reported by ledger validation")
	}
	return findings, b.build()
}
