package closequality_test

import (
	"sync"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
)

// TestConcurrentCalculate proves Calculate is safe to call concurrently
// against the same, immutable Input/Policy — run with -race.
func TestConcurrentCalculate(t *testing.T) {
	in := fixtures.CleanCloseInput()
	policy := fixtures.CleanPolicy()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			result := closequality.Calculate(in, policy)
			if result.Status != closequality.StatusReady {
				t.Errorf("status = %s, want READY", result.Status)
			}
		}()
	}
	wg.Wait()
}
