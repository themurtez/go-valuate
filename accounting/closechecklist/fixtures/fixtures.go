// Package fixtures provides synthetic accounting/closechecklist data for
// tests: a real ledger -> reconciliation -> statements ->
// journaldiagnostics -> closequality -> closechecklist chain built on
// the same ServiceBusiness scenario shared by accounting/ledger/fixtures,
// accounting/reconciliation/fixtures, and accounting/closequality/fixtures.
// Nothing here is real financial data.
package fixtures

import (
	"time"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closequality"
	closequalityfixtures "github.com/themurtez/go-valuate/accounting/closequality/fixtures"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// Period returns the closechecklist.Period for the shared ServiceBusiness
// 2025-01 scenario.
func Period() closechecklist.Period {
	return closechecklist.Period{
		PeriodID:        "2025-01",
		Label:           "January 2025",
		StartDate:       date("2025-01-01"),
		EndDate:         date("2025-01-31"),
		TargetCloseDate: date("2025-02-05"),
	}
}

// cleanBankReconciliationInput is a small, deliberately-clean bank
// reconciliation scenario (two exact-matching items, balances tie
// exactly) — unlike accounting/reconciliation/fixtures.BankInput, which
// is intentionally built to exercise findings (stale items, a balance
// mismatch) rather than represent a reconciled period. This package
// needs a genuinely clean pass for its own "everything satisfied ->
// READY_TO_CLOSE" integration fixture (section 31).
func cleanBankReconciliationInput() reconciliation.Input {
	ending := 10000.0
	return reconciliation.Input{
		AccountID:         "1000",
		ExternalAccountID: "BANK-ACCT-001",
		Period:            "2025-01",
		AsOfDate:          "2025-01-31",
		Type:              reconciliation.TypeBank,
		BookItems: []reconciliation.BookItem{
			{ItemID: "BK-1", Date: "2025-01-06", Amount: 5000, Direction: reconciliation.DirectionInflow, Reference: "DEP-1"},
			{ItemID: "BK-2", Date: "2025-01-15", Amount: 2000, Direction: reconciliation.DirectionOutflow, Reference: "CHK-1"},
		},
		ExternalItems: []reconciliation.ExternalItem{
			{ItemID: "STMT-1", Date: "2025-01-06", Amount: 5000, Direction: reconciliation.DirectionInflow, Reference: "DEP-1"},
			{ItemID: "STMT-2", Date: "2025-01-15", Amount: 2000, Direction: reconciliation.DirectionOutflow, Reference: "CHK-1"},
		},
		BookBalance:     reconciliation.BalanceInput{EndingBalance: &ending},
		ExternalBalance: reconciliation.BalanceInput{EndingBalance: &ending},
		Policy: reconciliation.MatchingPolicy{
			AmountTolerance:        0.01,
			DateWindowDays:         3,
			ReferenceNormalization: reconciliation.ReferenceNormalization{Trim: true, CaseFold: true},
			StaleDaysThreshold:     30,
			Materiality:            reconciliation.MaterialityPolicy{AbsoluteAmount: 50},
		},
	}
}

// CleanBankReconciliation runs the real accounting/reconciliation engine
// against cleanBankReconciliationInput, which reconciles cleanly (no
// findings, RECONCILED).
func CleanBankReconciliation() reconciliation.Result {
	return reconciliation.Calculate(cleanBankReconciliationInput())
}

// UnreconciledBankReconciliation mutates the clean scenario so it fails
// to tie (section 32): the external side's ending balance is forced far
// out of tolerance.
func UnreconciledBankReconciliation() reconciliation.Result {
	in := cleanBankReconciliationInput()
	skewed := *in.ExternalBalance.EndingBalance + 100000
	in.ExternalBalance.EndingBalance = &skewed
	return reconciliation.Calculate(in)
}

// CleanCloseQualityResult runs the real closequality engine against its
// own clean-close fixture chain (ledger/statements/AR/AP/journal
// diagnostics), matching accounting/closequality/fixtures.CleanCloseInput.
func CleanCloseQualityResult() closequality.Result {
	return closequality.Calculate(closequalityfixtures.CleanCloseInput(), closequalityfixtures.CleanPolicy())
}

// CloseQualityResultReadyWithWarnings mutates the clean closequality
// input so it resolves READY_WITH_WARNINGS instead of READY (section 33):
// an AR aging reconciliation difference within warning territory.
func CloseQualityResultReadyWithWarnings() closequality.Result {
	in := closequalityfixtures.CleanCloseInput()
	in.JournalDiagnostics = closequalityfixtures.JournalDiagnosticsResultWithPostCloseEntry()
	policy := closequalityfixtures.CleanPolicy()
	policy.TreatLockedPostCloseAsBlocking = false
	return closequality.Calculate(in, policy)
}

// Gates assembles the full gate-fact set a checklist instance would
// receive from the closequality + reconciliation adapters, using the
// clean scenario throughout.
func Gates() []closechecklist.GateFact {
	facts := closechecklist.CloseQualityGateFacts(CleanCloseQualityResult())
	facts = append(facts, closechecklist.ReconciliationGateFacts("reconciliation.cash_main", CleanBankReconciliation()))
	return facts
}

// GatesWithFailedBankReconciliation is Gates but with the bank
// reconciliation gate forced to FAIL — section 32.
func GatesWithFailedBankReconciliation() []closechecklist.GateFact {
	facts := closechecklist.CloseQualityGateFacts(CleanCloseQualityResult())
	facts = append(facts, closechecklist.ReconciliationGateFacts("reconciliation.cash_main", UnreconciledBankReconciliation()))
	return facts
}

// GatesWithWarnings is Gates but with closequality resolving
// READY_WITH_WARNINGS instead of READY — section 33.
func GatesWithWarnings() []closechecklist.GateFact {
	facts := closechecklist.CloseQualityGateFacts(CloseQualityResultReadyWithWarnings())
	facts = append(facts, closechecklist.ReconciliationGateFacts("reconciliation.cash_main", CleanBankReconciliation()))
	return facts
}

// CleanCloseTemplate is a small, integration-focused Template exercising
// bank rec -> AR rec -> AP rec -> journal review -> statement review ->
// controller review -> final approval, per task spec section 31.
func CleanCloseTemplate() closechecklist.Template {
	return closechecklist.Template{
		TemplateID: "integration_clean_close",
		Name:       "Integration Clean Close",
		Version:    "1.0",
		Sections: []closechecklist.SectionDefinition{
			{SectionCode: "CASH", Name: "Cash"},
			{SectionCode: "AR", Name: "Accounts Receivable"},
			{SectionCode: "AP", Name: "Accounts Payable"},
			{SectionCode: "JOURNAL_REVIEW", Name: "Journal Review"},
			{SectionCode: "FINANCIAL_STATEMENTS", Name: "Financial Statements"},
			{SectionCode: "MANAGEMENT_REVIEW", Name: "Management Review"},
			{SectionCode: "FINAL_CLOSE", Name: "Final Close"},
		},
		Tasks: []closechecklist.TaskDefinition{
			{
				TaskCode: "bank_rec", SectionCode: "CASH", Name: "Bank reconciliation", Required: true,
				Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				GateRules:     []closechecklist.GateRule{{GateCode: "reconciliation.cash_main", Require: closechecklist.GateRequirePass, Required: true}},
			},
			{
				TaskCode: "ar_rec", SectionCode: "AR", Name: "AR control reconciliation", Required: true,
				Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				GateRules:     []closechecklist.GateRule{{GateCode: "closequality.ar_control", Require: closechecklist.GateRequirePassOrWarning, Required: true}},
			},
			{
				TaskCode: "ap_rec", SectionCode: "AP", Name: "AP control reconciliation", Required: true,
				Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				GateRules:     []closechecklist.GateRule{{GateCode: "closequality.ap_control", Require: closechecklist.GateRequirePassOrWarning, Required: true}},
			},
			{
				TaskCode: "journal_review", SectionCode: "JOURNAL_REVIEW", Name: "Journal review", Required: true,
				Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				GateRules:     []closechecklist.GateRule{{GateCode: "closequality.journal_review", Require: closechecklist.GateRequirePassOrWarning, Required: true}},
				Dependencies: []closechecklist.TaskDependency{
					{DependsOnTaskCode: "bank_rec", Type: closechecklist.DependencyMustBeCompleted},
				},
			},
			{
				TaskCode: "statement_review", SectionCode: "FINANCIAL_STATEMENTS", Name: "Statement review", Required: true,
				Applicability:  closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				EvidencePolicy: closechecklist.EvidencePolicy{Type: closechecklist.EvidenceAtLeastOne},
				GateRules:      []closechecklist.GateRule{{GateCode: "closequality.statement_integrity", Require: closechecklist.GateRequirePass, Required: true}},
				Dependencies: []closechecklist.TaskDependency{
					{DependsOnTaskCode: "ar_rec", Type: closechecklist.DependencyMustBeCompleted},
					{DependsOnTaskCode: "ap_rec", Type: closechecklist.DependencyMustBeCompleted},
					{DependsOnTaskCode: "journal_review", Type: closechecklist.DependencyMustBeCompleted},
				},
			},
			{
				TaskCode: "controller_review", SectionCode: "MANAGEMENT_REVIEW", Name: "Controller review", Required: true,
				Applicability: closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				ReviewPolicy:  closechecklist.ReviewPolicy{Type: closechecklist.ReviewSpecificRoles, RequiredRoles: []closechecklist.SignOffRole{closechecklist.SignOffController}},
				GateRules:     []closechecklist.GateRule{{GateCode: "closequality.overall", Require: closechecklist.GateRequirePassOrWarning, Required: true}},
				Dependencies: []closechecklist.TaskDependency{
					{DependsOnTaskCode: "bank_rec", Type: closechecklist.DependencyMustBeCompleted},
					{DependsOnTaskCode: "statement_review", Type: closechecklist.DependencyMustBeCompleted},
				},
			},
			{
				TaskCode: "final_approval", SectionCode: "FINAL_CLOSE", Name: "Final close approval", Required: true,
				Applicability:   closechecklist.ApplicabilityRule{Type: closechecklist.ApplicabilityAlways},
				ReviewPolicy:    closechecklist.ReviewPolicy{Type: closechecklist.ReviewSpecificRoles, RequiredRoles: []closechecklist.SignOffRole{closechecklist.SignOffApprover}},
				ExceptionPolicy: closechecklist.ExceptionAllowedWithApproval,
				Dependencies: []closechecklist.TaskDependency{
					{DependsOnTaskCode: "controller_review", Type: closechecklist.DependencyMustBeCompleted},
				},
			},
		},
	}
}

func completedAt(s string) *time.Time {
	t := date(s)
	return &t
}

// CleanCloseTaskStates has every task in CleanCloseTemplate marked
// COMPLETED with sufficient evidence/sign-offs, matching section 31's
// "all requirements satisfied -> READY_TO_CLOSE".
func CleanCloseTaskStates() []closechecklist.TaskState {
	return []closechecklist.TaskState{
		{TaskCode: "bank_rec", Status: closechecklist.TaskCompleted, CompletedAt: completedAt("2025-02-01")},
		{TaskCode: "ar_rec", Status: closechecklist.TaskCompleted, CompletedAt: completedAt("2025-02-01")},
		{TaskCode: "ap_rec", Status: closechecklist.TaskCompleted, CompletedAt: completedAt("2025-02-01")},
		{TaskCode: "journal_review", Status: closechecklist.TaskCompleted, CompletedAt: completedAt("2025-02-02")},
		{
			TaskCode: "statement_review", Status: closechecklist.TaskCompleted, CompletedAt: completedAt("2025-02-02"),
			Evidence: []closechecklist.EvidenceRef{{EvidenceID: "EV-1", Type: "WORKPAPER", Reference: "statement-review.xlsx"}},
		},
		{
			TaskCode: "controller_review", Status: closechecklist.TaskCompleted, CompletedAt: completedAt("2025-02-03"),
			SignOffs: []closechecklist.SignOff{{SignOffID: "SO-1", Role: closechecklist.SignOffController, ActorRef: "controller@example.com", SignedAt: date("2025-02-03")}},
		},
		{
			TaskCode: "final_approval", Status: closechecklist.TaskCompleted, CompletedAt: completedAt("2025-02-04"),
			SignOffs: []closechecklist.SignOff{{SignOffID: "SO-2", Role: closechecklist.SignOffApprover, ActorRef: "cfo@example.com", SignedAt: date("2025-02-04")}},
		},
	}
}

// CleanInstance is the full assembled Instance for section 31's clean
// close integration fixture.
func CleanInstance() closechecklist.Instance {
	return closechecklist.Instance{
		Template:       CleanCloseTemplate(),
		Period:         Period(),
		PeriodState:    closechecklist.PeriodStateInProgress,
		EvaluationDate: date("2025-02-05"),
		TaskStates:     CleanCloseTaskStates(),
		Gates:          Gates(),
	}
}
