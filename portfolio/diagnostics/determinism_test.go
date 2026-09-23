package diagnostics

import (
	"encoding/json"
	"testing"
)

// fullFixture builds an Input exercising every finding code and coverage
// field at once, reused by the determinism/round-trip tests.
func fullFixture() Input {
	return Input{
		Portfolio: []BusinessSnapshot{
			stableBusiness("stable1"),
			decliningBusiness("declining1"),
			noPriorBusiness("noprior1"),
			emptyBusiness("empty1"),
		},
		Policy: Policy{
			MaterialChangePercent: 0.05,
			SeverityWeights: map[Severity]float64{
				SeverityCritical: 10,
				SeverityWarning:  5,
				SeverityInfo:     1,
			},
		},
	}
}

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output.
func TestCalculate_Deterministic(t *testing.T) {
	in := fullFixture()

	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}

// TestCalculate_DeterministicAcrossPortfolioPermutations proves
// Result.Findings/Counts/Coverage do not depend on Input.Portfolio's
// caller-supplied order, only on ranked priority.
func TestCalculate_DeterministicAcrossPortfolioPermutations(t *testing.T) {
	base := fullFixture()
	reversed := Input{Policy: base.Policy}
	for i := len(base.Portfolio) - 1; i >= 0; i-- {
		reversed.Portfolio = append(reversed.Portfolio, base.Portfolio[i])
	}

	got1, err := json.Marshal(Calculate(base))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	got2, err := json.Marshal(Calculate(reversed))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if string(got1) != string(got2) {
		t.Fatalf("Calculate output differs by portfolio input order:\nforward: %s\nreversed: %s", got1, got2)
	}
}

// TestCalculate_ConcurrentCallsAreSafe proves Calculate has no shared
// mutable state by running it concurrently against the same Input.
func TestCalculate_ConcurrentCallsAreSafe(t *testing.T) {
	in := fullFixture()
	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	const n = 20
	results := make(chan string, n)
	for i := 0; i < n; i++ {
		go func() {
			b, err := json.Marshal(Calculate(in))
			if err != nil {
				results <- "ERROR: " + err.Error()
				return
			}
			results <- string(b)
		}()
	}
	for i := 0; i < n; i++ {
		got := <-results
		if got != string(first) {
			t.Fatalf("concurrent run produced different output than the first run")
		}
	}
}

// TestCalculate_ConcurrentZeroValuePolicyDoesNotCorruptDefaultPolicy runs
// Calculate concurrently under a zero-value Input.Policy — the specific
// path resolvePolicy's SeverityWeights nil-branch used to alias directly
// onto the package-level DefaultPolicy.SeverityWeights map (see
// TestResolvePolicy_ZeroValueSeverityWeightsNeverAliasesDefaultPolicy in
// diagnostics_test.go for the sequential version of this regression).
// Unlike TestCalculate_ConcurrentCallsAreSafe above (which always sets
// Policy.SeverityWeights explicitly), this test exercises the exact
// branch that was vulnerable: many goroutines simultaneously requesting
// the default weights, each mutating its own returned copy, must never
// observe or corrupt another goroutine's copy or the shared
// DefaultPolicy — run under go test -race to catch any aliasing this
// black-box output comparison alone might miss.
func TestCalculate_ConcurrentZeroValuePolicyDoesNotCorruptDefaultPolicy(t *testing.T) {
	in := Input{Portfolio: []BusinessSnapshot{stableBusiness("z1")}} // Policy left zero-value

	const n = 20
	done := make(chan Severity, n)
	for i := 0; i < n; i++ {
		go func() {
			res := Calculate(in)
			// Mutate this goroutine's own returned map — if it were
			// aliased to DefaultPolicy.SeverityWeights (or to another
			// goroutine's copy), this write would race or corrupt a
			// value another goroutine reads below.
			for k := range res.Policy.SeverityWeights {
				res.Policy.SeverityWeights[k] = -1
			}
			done <- SeverityCritical
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}

	if got := DefaultPolicy.SeverityWeights[SeverityCritical]; got != 10 {
		t.Fatalf("concurrent zero-value-Policy calls corrupted package-level DefaultPolicy: SeverityCritical weight = %v, want 10", got)
	}

	// A fresh call after the concurrent storm must still see the
	// uncorrupted defaults.
	after := Calculate(in)
	if got := after.Policy.SeverityWeights[SeverityCritical]; got != 10 {
		t.Fatalf("a call after the concurrent storm returned a corrupted SeverityCritical weight = %v, want 10", got)
	}
}
