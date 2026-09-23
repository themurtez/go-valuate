package closequality_test

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
)

// largeInput builds a synthetic Input scaled to numAccounts accounts,
// numJournalFindings journaldiagnostics findings, numReconciliations
// reconciliation statuses, and numCloseTasks close tasks — used to
// benchmark linear-aggregation performance and check for accidental
// O(N^2) dedup/search logic (see BenchmarkCalculate_Scaling).
func largeInput(numAccounts, numJournalFindings, numReconciliations, numCloseTasks int) (closequality.Input, closequality.Policy) {
	period := closequality.PeriodInfo{Period: "2025-01", StartDate: "2025-01-01", EndDate: "2025-01-31", CloseDate: "2025-02-03"}

	var findings []journaldiagnostics.Finding
	for i := 0; i < numJournalFindings; i++ {
		findings = append(findings, journaldiagnostics.Finding{
			Code:     journaldiagnostics.FindingRoundDollarEntry,
			Severity: journaldiagnostics.SeverityInfo,
			Period:   "2025-01",
			EntryIDs: []string{fmt.Sprintf("JE-%d", i)},
			Amount:   journaldiagnostics.AvailableValue(float64(i)),
		})
	}
	jd := journaldiagnostics.Result{
		Period:   "2025-01",
		Findings: findings,
	}

	var reconciliations []closequality.ReconciliationStatus
	var requiredAccounts []string
	for i := 0; i < numReconciliations; i++ {
		acctID := fmt.Sprintf("ACCT-%d", i)
		requiredAccounts = append(requiredAccounts, acctID)
		reconciliations = append(reconciliations, closequality.ReconciliationStatus{
			AccountID: acctID,
			Status:    closequality.ReconciliationReconciled,
			AsOfDate:  "2025-01-31",
		})
	}

	var closeTasks []closequality.CloseTaskStatus
	for i := 0; i < numCloseTasks; i++ {
		closeTasks = append(closeTasks, closequality.CloseTaskStatus{
			Code:     fmt.Sprintf("task-%d", i),
			Required: i%2 == 0,
			Status:   closequality.CloseTaskCompleted,
		})
	}

	var expectations []closequality.AccountExpectation
	materiality := 1.0
	for i := 0; i < numAccounts; i++ {
		expectations = append(expectations, closequality.AccountExpectation{
			AccountID:    fmt.Sprintf("ACCT-%d", i),
			ExpectedSign: closequality.ExpectedSignPositive,
			Materiality:  &materiality,
		})
	}

	policy := closequality.DefaultPolicy()
	policy.RequiredReconciliationAccounts = requiredAccounts
	policy.AccountExpectations = expectations

	in := closequality.Input{
		Period:             period,
		JournalDiagnostics: jd,
		Reconciliations:    reconciliations,
		CloseTasks:         closeTasks,
	}
	return in, policy
}

func BenchmarkCalculate_Scaling(b *testing.B) {
	sizes := []struct {
		name                                       string
		accounts, findings, reconciliations, tasks int
	}{
		{"Small", 10, 100, 10, 10},
		{"Large", 1000, 10000, 1000, 1000},
	}
	for _, sz := range sizes {
		in, policy := largeInput(sz.accounts, sz.findings, sz.reconciliations, sz.tasks)
		b.Run(sz.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = closequality.Calculate(in, policy)
			}
		})
	}
}
