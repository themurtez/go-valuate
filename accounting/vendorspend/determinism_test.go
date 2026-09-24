package vendorspend_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

// TestDeterminism_RepeatedCallsIdentical proves Calculate returns
// byte-for-byte identical JSON across repeated calls against identical
// input — no map-order nondeterminism, no time.Now() dependency.
func TestDeterminism_RepeatedCallsIdentical(t *testing.T) {
	in := fullInput()
	opts := fullOptions()

	first, err := json.Marshal(vendorspend.Calculate(in, opts))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for i := 0; i < 20; i++ {
		b, err := json.Marshal(vendorspend.Calculate(in, opts))
		if err != nil {
			t.Fatalf("marshal iteration %d: %v", i, err)
		}
		if string(b) != string(first) {
			t.Fatalf("iteration %d produced different JSON than the first call", i)
		}
	}
}

// TestDeterminism_ConcurrentCalls proves Calculate is safe to call
// concurrently and every concurrent call returns the same result.
func TestDeterminism_ConcurrentCalls(t *testing.T) {
	in := fullInput()
	opts := fullOptions()
	want, err := json.Marshal(vendorspend.Calculate(in, opts))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	const n = 20
	var wg sync.WaitGroup
	results := make([][]byte, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			b, err := json.Marshal(vendorspend.Calculate(in, opts))
			if err != nil {
				t.Errorf("goroutine %d marshal: %v", idx, err)
				return
			}
			results[idx] = b
		}(i)
	}
	wg.Wait()

	for i, b := range results {
		if string(b) != string(want) {
			t.Errorf("goroutine %d result differs from expected", i)
		}
	}
}
