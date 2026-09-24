package closechecklist_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closechecklist/fixtures"
)

func TestDeterminism_RepeatedCalculate_ByteIdentical(t *testing.T) {
	in := fixtures.CleanInstance()
	policy := closechecklist.DefaultPolicy()

	var first []byte
	for i := 0; i < 25; i++ {
		res := closechecklist.Calculate(in, policy)
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if first == nil {
			first = b
			continue
		}
		if string(b) != string(first) {
			t.Fatalf("iteration %d produced different JSON than the first call", i)
		}
	}
}

func TestDeterminism_FailedScenario_ByteIdentical(t *testing.T) {
	in := fixtures.CleanInstance()
	in.Gates = fixtures.GatesWithFailedBankReconciliation()
	policy := closechecklist.DefaultPolicy()

	var first []byte
	for i := 0; i < 10; i++ {
		res := closechecklist.Calculate(in, policy)
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if first == nil {
			first = b
			continue
		}
		if string(b) != string(first) {
			t.Fatalf("iteration %d produced different JSON than the first call", i)
		}
	}
}
