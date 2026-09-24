package closechecklist_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closechecklist/fixtures"
)

// TestConcurrency_CalculateIsRaceSafe runs many concurrent Calculate
// calls against the same shared Instance/Policy values and asserts every
// result is byte-identical — see section 48. Run with -race.
func TestConcurrency_CalculateIsRaceSafe(t *testing.T) {
	in := fixtures.CleanInstance()
	policy := closechecklist.DefaultPolicy()

	const workers = 50
	results := make([][]byte, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			res := closechecklist.Calculate(in, policy)
			b, err := json.Marshal(res)
			if err != nil {
				t.Errorf("marshal: %v", err)
				return
			}
			results[i] = b
		}()
	}
	wg.Wait()

	for i := 1; i < workers; i++ {
		if string(results[i]) != string(results[0]) {
			t.Fatalf("worker %d produced different JSON than worker 0", i)
		}
	}
}
