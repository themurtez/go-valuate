package closechecklist_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closechecklist/fixtures"
)

// TestImmutability_CalculateNeverMutatesInput — section 40: Calculate
// must never mutate templates, task definitions, task states, evidence,
// sign-offs, exceptions, gates, or policy.
func TestImmutability_CalculateNeverMutatesInput(t *testing.T) {
	in := fixtures.CleanInstance()
	policy := closechecklist.DefaultPolicy()

	before, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}
	policyBefore, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal policy before: %v", err)
	}

	_ = closechecklist.Calculate(in, policy)

	after, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	policyAfter, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal policy after: %v", err)
	}

	if string(before) != string(after) {
		t.Errorf("Instance was mutated by Calculate")
	}
	if string(policyBefore) != string(policyAfter) {
		t.Errorf("Policy was mutated by Calculate")
	}
}

// TestImmutability_CalculateWithPrior_NeverMutatesPrior — section 25/40:
// PriorClose must never be mutated by CalculateWithPrior.
func TestImmutability_CalculateWithPrior_NeverMutatesPrior(t *testing.T) {
	current := fixtures.CleanInstance()
	priorInstance := fixtures.CleanInstance()
	priorInstance.Period.PeriodID = "2024-12"
	priorResult := closechecklist.Calculate(priorInstance, closechecklist.DefaultPolicy())
	prior := closechecklist.PriorClose{Instance: priorInstance, Result: priorResult}

	before, err := json.Marshal(prior)
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}

	_ = closechecklist.CalculateWithPrior(current, closechecklist.DefaultPolicy(), prior)

	after, err := json.Marshal(prior)
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("PriorClose was mutated by CalculateWithPrior")
	}
}
