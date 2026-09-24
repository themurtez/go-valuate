package reconciliation

import (
	"encoding/json"
	"sync"
	"testing"
)

// TestConcurrency_CalculateSafeUnderParallelCalls runs many concurrent
// Calculate calls against the SAME immutable Input and confirms every
// result is byte-for-byte identical — locking "concurrent Calculate
// calls with immutable inputs must be safe" (task section 75).
func TestConcurrency_CalculateSafeUnderParallelCalls(t *testing.T) {
	in := buildDeterminismInput()
	const goroutines = 50

	var wg sync.WaitGroup
	results := make([]string, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r := Calculate(in)
			b, err := json.Marshal(r)
			if err != nil {
				t.Errorf("marshal error: %v", err)
				return
			}
			results[idx] = string(b)
		}(i)
	}
	wg.Wait()

	first := results[0]
	for i, r := range results {
		if r != first {
			t.Fatalf("goroutine %d produced different result than goroutine 0", i)
		}
	}
}

// TestConcurrency_DistinctInputsIndependentUnderRace confirms distinct
// concurrent Calculate calls (different inputs) do not interfere with
// each other (e.g. via any accidental package-level mutable state).
func TestConcurrency_DistinctInputsIndependentUnderRace(t *testing.T) {
	var wg sync.WaitGroup
	const goroutines = 20
	errs := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			amt := float64(idx + 1)
			in := Input{
				AccountID: "A",
				AsOfDate:  "2025-01-01",
				BookItems: []BookItem{
					{ItemID: "B1", Date: "2025-01-01", Amount: amt, Direction: DirectionOutflow},
				},
				ExternalItems: []ExternalItem{
					{ItemID: "E1", Date: "2025-01-01", Amount: amt, Direction: DirectionOutflow},
				},
			}
			r := Calculate(in)
			if len(r.MatchedGroups) != 1 {
				errs <- "expected 1 matched group"
				return
			}
			if r.MatchedGroups[0].BookAmount != -amt {
				errs <- "cross-goroutine contamination detected in BookAmount"
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}
