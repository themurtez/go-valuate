package cashforecast

import (
	"fmt"
	"testing"
	"time"
)

func genEvents(n int, start time.Time) []CashFlowEvent {
	events := make([]CashFlowEvent, n)
	for i := 0; i < n; i++ {
		direction := DirectionInflow
		category := CategoryCashSale
		if i%2 == 0 {
			direction = DirectionOutflow
			category = CategoryOtherOperatingOutflow
		}
		events[i] = CashFlowEvent{
			ID: fmt.Sprintf("evt-%d", i), Date: start.AddDate(0, 0, i%90), Amount: float64(100 + i%500),
			Direction: direction, Category: category, Basis: BasisKnown,
		}
	}
	return events
}

func BenchmarkCalculate_100kEvents(b *testing.B) {
	start := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	in := Input{
		ForecastStartDate: start,
		OpeningCash:       OpeningCash{Amount: 1000000, Currency: "USD"},
		Events:            genEvents(100000, start),
	}
	opts := Options{MinimumCash: MinimumCashPolicy{MinimumCashBalance: 50000}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(in, opts)
	}
}

func BenchmarkCalculate_100kARScheduledReceipts(b *testing.B) {
	start := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	n := 100000
	sources := make([]ARReceivableSource, n)
	collections := make([]ARCollectionAssumption, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("r-%d", i)
		sources[i] = ARReceivableSource{ReceivableID: id, OpenAmount: 1000}
		collections[i] = ARCollectionAssumption{
			ID: fmt.Sprintf("c-%d", i), ReceivableID: id,
			ExpectedReceiptDate: start.AddDate(0, 0, i%90), ExpectedAmount: 1000,
		}
	}
	in := Input{
		ForecastStartDate: start,
		OpeningCash:       OpeningCash{Amount: 1000000, Currency: "USD"},
		ARSources:         sources,
		ARCollections:     collections,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(in, Options{})
	}
}

func BenchmarkCalculate_100kAPPlannedPayments(b *testing.B) {
	start := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	n := 100000
	sources := make([]APPayableSource, n)
	plans := make([]APPaymentPlan, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("b-%d", i)
		sources[i] = APPayableSource{PayableID: id, OpenAmount: 500, DueDate: start.AddDate(0, 0, i%90)}
		plans[i] = APPaymentPlan{ID: fmt.Sprintf("p-%d", i), PayableID: id, PaymentDate: start.AddDate(0, 0, i%90), Amount: 500}
	}
	in := Input{
		ForecastStartDate: start,
		OpeningCash:       OpeningCash{Amount: 1000000, Currency: "USD"},
		APSources:         sources,
		APPlans:           plans,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(in, Options{})
	}
}

func BenchmarkCalculate_20Scenarios(b *testing.B) {
	start := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	scenarios := make([]Scenario, 20)
	for i := 0; i < 20; i++ {
		scenarios[i] = Scenario{
			Label: fmt.Sprintf("SCENARIO_%d", i),
			Transforms: []EventTransform{
				{Kind: TransformDelayInflows, DelayDays: i + 1, Category: CategoryARCollection},
				{Kind: TransformScaleCategory, ScaleFactor: 0.9, Category: CategoryAPPayment, Direction: DirectionOutflow},
			},
		}
	}
	in := Input{
		ForecastStartDate: start,
		OpeningCash:       OpeningCash{Amount: 500000, Currency: "USD"},
		Events:            genEvents(5000, start),
		Scenarios:         scenarios,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Calculate(in, Options{})
	}
}

func BenchmarkRecurring_MonthlyExpansion52Weeks(b *testing.B) {
	rule := RecurringRule{
		ID: "rent", Amount: 5000, Direction: DirectionOutflow, Category: CategoryRent, Basis: BasisScheduled,
		StartDate: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), Frequency: FrequencyMonthly,
	}
	rangeStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generateRecurringEvents(rule, rangeStart, rangeEnd)
	}
}

// BenchmarkCalculate_ScalingCheck runs Calculate at 10x and 100x event
// counts so a manual before/after comparison can confirm the 13-week
// aggregation stays close to O(N) rather than O(N^2) — see the task's
// section 69's "check for accidental O(N^2)" instruction. Run with:
//
//	go test -bench=ScalingCheck -run='^$' ./accounting/cashforecast
//
// and confirm the 100x case takes roughly 10x (not ~100x) the 10x case's
// time per operation.
func BenchmarkCalculate_ScalingCheck(b *testing.B) {
	start := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{1000, 10000, 100000} {
		n := n
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			in := Input{
				ForecastStartDate: start,
				OpeningCash:       OpeningCash{Amount: 1000000, Currency: "USD"},
				Events:            genEvents(n, start),
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				Calculate(in, Options{})
			}
		})
	}
}
