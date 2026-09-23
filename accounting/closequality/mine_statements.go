package closequality

import (
	"strconv"

	"github.com/themurtez/go-valuate/accounting/statements"
)

func statementsIssueSeverity(sev statements.IssueSeverity) Severity {
	if sev == statements.SeverityError {
		return SeverityWarning // most mapping issues are review items, not automatic blockers — see per-code overrides below
	}
	return SeverityInfo
}

// mineFinancialStatementIntegrity translates statements.Result into
// DimensionFinancialStatementIntegrity findings: build/dataset
// availability, unmapped/invalid mapping issues (materiality-gated), and
// balance-sheet reconciliation failures. It never recomputes mapping
// coverage or reconciliation itself — both are read directly off
// statements.Result.
func mineFinancialStatementIntegrity(in Input, policy Policy) ([]Finding, DimensionResult) {
	b := newDimensionBuilder(DimensionFinancialStatementIntegrity)

	if !in.statementsAvailable() {
		b.unavailable("statements result not supplied or not requested")
		return nil, b.build()
	}
	b.markAssessed()

	var findings []Finding

	if in.Statements.DatasetAvailability == statements.AvailabilityInvalid ||
		in.Statements.IncomeStatementAvailability == statements.AvailabilityInvalid ||
		in.Statements.BalanceSheetAvailability == statements.AvailabilityInvalid {
		f := Finding{
			Code:         FindingStatementBuildInvalid,
			Dimension:    DimensionFinancialStatementIntegrity,
			Severity:     SeverityBlocking,
			Message:      "statement build reported an invalid result",
			SourceModule: SourceStatements,
		}
		findings = append(findings, f)
		b.record(f)
	}

	cov := in.Statements.Coverage
	if cov.TotalAccounts > 0 && cov.UnmappedAccounts > 0 {
		unmappedAmount := cov.UnmappedBalanceAmount
		material := isMaterial(unmappedAmount, in.TotalAssets, in.TotalRevenue, policy.Materiality)
		sev := SeverityInfo
		if material {
			sev = SeverityBlocking
		} else {
			sev = SeverityWarning
		}
		f := Finding{
			Code:      FindingMaterialUnmappedAccounts,
			Dimension: DimensionFinancialStatementIntegrity,
			Severity:  sev,
			Message:   "one or more accounts are not mapped to a financial statement line",
			Evidence: Evidence{
				EndingBalance:        AvailableValue(unmappedAmount),
				MaterialityThreshold: materialityEvidence(policy.Materiality),
			},
			SourceModule: SourceStatements,
		}
		findings = append(findings, f)
		b.record(f)
		b.note("unmapped accounts: count=" + strconv.Itoa(cov.UnmappedAccounts))
	} else if cov.TotalAccounts > 0 {
		b.note("all accounts mapped")
	}

	for _, iss := range in.Statements.Issues {
		if iss.Code == statements.IssueUnmappedAccount {
			continue // already summarized via Coverage above
		}
		sev := statementsIssueSeverity(iss.Severity)
		if iss.Code == statements.IssueUnbalancedBalanceSheet ||
			iss.Code == statements.IssueSourceTrialBalanceUnbalanced {
			sev = SeverityBlocking
		}
		f := Finding{
			Code:         FindingStatementBuildInvalid,
			Dimension:    DimensionFinancialStatementIntegrity,
			Severity:     sev,
			Message:      iss.Message,
			SourceModule: SourceStatements,
			SourceCode:   string(iss.Code),
		}
		if iss.AccountID != "" {
			f.AccountIDs = []string{iss.AccountID}
		}
		findings = append(findings, f)
		b.record(f)
	}

	for _, r := range in.Statements.Reconciliation {
		if r.Balanced {
			continue
		}
		f := Finding{
			Code:      FindingBalanceSheetOutOfBalance,
			Dimension: DimensionFinancialStatementIntegrity,
			Severity:  SeverityBlocking,
			Message:   "balance sheet does not balance for period " + string(r.Period),
			Evidence: Evidence{
				Difference: AvailableValue(r.Difference),
				Tolerance:  AvailableValue(r.Tolerance),
			},
			SourceModule: SourceStatements,
		}
		findings = append(findings, f)
		b.record(f)
	}
	if len(in.Statements.Reconciliation) == 0 {
		b.note("no balance-sheet reconciliation results supplied")
	}

	return findings, b.build()
}

func materialityEvidence(p MaterialityPolicy) Value {
	if p.AbsoluteAmount > 0 {
		return AvailableValue(p.AbsoluteAmount)
	}
	return Unavailable()
}
