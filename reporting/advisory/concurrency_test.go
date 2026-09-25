package advisory

import (
	"sync"
	"testing"

	"github.com/themurtez/go-valuate/accounting/cashforecast"
)

// TestBuild_ConcurrentSafety runs many concurrent Build calls against the
// same Input/Policy value and against ExamplePolicy() repeatedly, to
// catch a shared-mutable-state bug like the portfolio/diagnostics
// DefaultPolicy.SeverityWeights class of bug this package was
// specifically told to avoid — task section 125. Run with -race.
func TestBuild_ConcurrentSafety(t *testing.T) {
	in := Input{
		Company: CompanyContext{CurrentPeriod: "2026-02", PriorPeriod: "2026-01"},
		Operating: OperatingInputs{
			CashForecast: cashforecast.Result{
				Available: true, ForecastStartDate: "2026-02-01", HorizonWeeks: 13,
				BaseScenario: cashforecast.ScenarioResult{
					Summary: cashforecast.LiquiditySummary{
						EndingCash: 20000, LowestCashBalance: 5000, LowestCashWeek: 3,
						ThresholdAvailable: true, MinimumCashThreshold: 25000, WeeksBelowMinimum: 4,
					},
				},
			},
		},
	}

	const goroutines = 50
	var wg sync.WaitGroup
	results := make([]Result, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			policy := ExamplePolicy()
			results[idx] = Build(in, policy)
		}(i)
	}
	wg.Wait()

	first := mustJSON(t, results[0])
	for i := 1; i < goroutines; i++ {
		got := mustJSON(t, results[i])
		if got != first {
			t.Fatalf("goroutine %d produced a different result than goroutine 0 — non-deterministic or shared-state bug", i)
		}
	}
}
