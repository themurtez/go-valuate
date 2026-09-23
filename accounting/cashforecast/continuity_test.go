package cashforecast

import "testing"

// assertWeeklyContinuity checks, for every week in weeks:
//   - OpeningCash + Inflows.Total - Outflows.Total == EndingCash
//   - week[n].EndingCash == week[n+1].OpeningCash for all n
//
// within floating-point tolerance — see the task's section 58.
func assertWeeklyContinuity(t *testing.T, label string, weeks []WeeklyForecast) {
	t.Helper()
	const tol = 1e-9
	for i, w := range weeks {
		got := w.OpeningCash + w.NetCashFlow
		if abs(got-w.EndingCash) > tol {
			t.Errorf("%s week %d: OpeningCash(%v) + NetCashFlow(%v) = %v, want EndingCash %v",
				label, w.WeekNumber, w.OpeningCash, w.NetCashFlow, got, w.EndingCash)
		}
		if abs(w.NetCashFlow-(w.Inflows.Total-w.Outflows.Total)) > tol {
			t.Errorf("%s week %d: NetCashFlow(%v) != Inflows.Total(%v) - Outflows.Total(%v)",
				label, w.WeekNumber, w.NetCashFlow, w.Inflows.Total, w.Outflows.Total)
		}
		if i+1 < len(weeks) {
			next := weeks[i+1]
			if abs(w.EndingCash-next.OpeningCash) > tol {
				t.Errorf("%s: week %d EndingCash(%v) != week %d OpeningCash(%v)",
					label, w.WeekNumber, w.EndingCash, next.WeekNumber, next.OpeningCash)
			}
		}
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// TestContinuity_HorizonWideIdentity proves the entire-horizon identity:
// OpeningCash(week 1) + TotalInflows - TotalOutflows == EndingCash(last
// week), matching LiquiditySummary's own StartingCash/NetChange/
// EndingCash fields.
func assertHorizonIdentity(t *testing.T, label string, summary LiquiditySummary) {
	t.Helper()
	const tol = 1e-9
	got := summary.StartingCash + summary.NetChange
	if abs(got-summary.EndingCash) > tol {
		t.Errorf("%s: StartingCash(%v) + NetChange(%v) = %v, want EndingCash %v",
			label, summary.StartingCash, summary.NetChange, got, summary.EndingCash)
	}
	if abs(summary.NetChange-(summary.TotalInflows-summary.TotalOutflows)) > tol {
		t.Errorf("%s: NetChange(%v) != TotalInflows(%v) - TotalOutflows(%v)",
			label, summary.NetChange, summary.TotalInflows, summary.TotalOutflows)
	}
}

func TestContinuity_BaseAndAllScenarios(t *testing.T) {
	in := fullInput(t)
	opts := fullOptions()
	result := Calculate(in, opts)

	if !result.Available {
		t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
	}

	assertWeeklyContinuity(t, "base", result.BaseScenario.Weekly)
	assertHorizonIdentity(t, "base", result.BaseScenario.Summary)

	for _, sc := range result.Scenarios {
		assertWeeklyContinuity(t, sc.Label, sc.Weekly)
		assertHorizonIdentity(t, sc.Label, sc.Summary)
	}
}

// TestContinuity_ManyVariedScenarios exercises the continuity invariant
// across a range of input shapes (empty, single event, many events,
// negative-cash-inducing, large horizon) to catch an edge case a single
// fixture might miss.
func TestContinuity_ManyVariedScenarios(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		opts Options
	}{
		{
			name: "no events at all",
			in:   Input{ForecastStartDate: testDate(t, "2025-01-06"), OpeningCash: OpeningCash{Amount: 5000, Currency: "USD"}},
		},
		{
			name: "single large outflow exceeding opening cash",
			in: Input{
				ForecastStartDate: testDate(t, "2025-01-06"),
				OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
				Events: []CashFlowEvent{
					{ID: "big", Date: testDate(t, "2025-01-08"), Amount: 50000, Direction: DirectionOutflow, Category: CategoryAPPayment, Basis: BasisKnown},
				},
			},
			opts: Options{MinimumCash: MinimumCashPolicy{MinimumCashBalance: 5000}},
		},
		{
			name: "52-week horizon with monthly recurring",
			in: Input{
				ForecastStartDate: testDate(t, "2025-01-06"),
				OpeningCash:       OpeningCash{Amount: 200000, Currency: "USD"},
				RecurringRules: []RecurringRule{
					{ID: "rent", Amount: 4000, Direction: DirectionOutflow, Category: CategoryRent, Basis: BasisScheduled,
						StartDate: testDate(t, "2025-01-31"), Frequency: FrequencyMonthly},
				},
			},
			opts: Options{HorizonWeeks: 52},
		},
		{
			name: "1-week horizon",
			in: Input{
				ForecastStartDate: testDate(t, "2025-01-06"),
				OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
				Events: []CashFlowEvent{
					{ID: "e1", Date: testDate(t, "2025-01-07"), Amount: 500, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
				},
			},
			opts: Options{HorizonWeeks: 1},
		},
		{
			name: "events exactly on week boundaries",
			in: Input{
				ForecastStartDate: testDate(t, "2025-01-06"),
				OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
				Events: []CashFlowEvent{
					{ID: "first-day", Date: testDate(t, "2025-01-06"), Amount: 100, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
					{ID: "last-day-w1", Date: testDate(t, "2025-01-12"), Amount: 200, Direction: DirectionOutflow, Category: CategoryRent, Basis: BasisKnown},
					{ID: "first-day-w2", Date: testDate(t, "2025-01-13"), Amount: 300, Direction: DirectionInflow, Category: CategoryCashSale, Basis: BasisKnown},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := Calculate(c.in, c.opts)
			if !result.Available {
				t.Fatalf("expected Available=true, got Issues=%+v", result.Issues)
			}
			assertWeeklyContinuity(t, c.name, result.BaseScenario.Weekly)
			assertHorizonIdentity(t, c.name, result.BaseScenario.Summary)
		})
	}
}
