package cashforecast

// buildWeeklyForecast rolls forward openingCash across weeks, bucketing
// every event (already filtered to the horizon range) into its week by
// Date, and returns the WeeklyForecast slice plus a sorted
// DetailedSchedule of every event with its resolved WeekNumber — see the
// task's sections 10-12 and 58's continuity invariant, which this
// function's construction guarantees by design: each week's EndingCash is
// assigned as the next week's OpeningCash directly (never recomputed
// independently).
func buildWeeklyForecast(weeks []weekBounds, openingCash float64, events []CashFlowEvent, minimumThreshold float64, haveThreshold bool) ([]WeeklyForecast, []ScheduleEntry) {
	// Bucket events by week index first (single pass), then process weeks
	// in order — avoids an O(weeks x events) scan.
	byWeek := make([][]CashFlowEvent, len(weeks))
	for _, e := range events {
		idx, ok := weekIndexForDate(weeks, e.Date)
		if !ok {
			continue // should not happen: events are pre-filtered to horizon range.
		}
		byWeek[idx] = append(byWeek[idx], e)
	}

	forecasts := make([]WeeklyForecast, 0, len(weeks))
	var schedule []ScheduleEntry
	runningCash := openingCash

	for i, wb := range weeks {
		weekEvents := byWeek[i]
		sortEvents(weekEvents)

		inflows := buildFlowTotal(weekEvents, DirectionInflow)
		outflows := buildFlowTotal(weekEvents, DirectionOutflow)

		wf := WeeklyForecast{
			WeekNumber:  wb.WeekNumber,
			StartDate:   wb.StartDate.Format("2006-01-02"),
			EndDate:     wb.EndDate.Format("2006-01-02"),
			OpeningCash: runningCash,
			Inflows:     inflows,
			Outflows:    outflows,
		}
		wf.NetCashFlow = inflows.Total - outflows.Total
		wf.EndingCash = wf.OpeningCash + wf.NetCashFlow
		wf.Threshold = buildThresholdPosition(wf.EndingCash, minimumThreshold, haveThreshold)

		for _, ca := range inflows.ByCategory {
			cls := categoryClass(ca.Category)
			switch cls {
			case ClassInvesting:
				wf.InvestingInflows += ca.Amount
			case ClassFinancing:
				wf.FinancingInflows += ca.Amount
			default:
				wf.OperatingInflows += ca.Amount
			}
		}
		for _, ca := range outflows.ByCategory {
			cls := categoryClass(ca.Category)
			switch cls {
			case ClassInvesting:
				wf.InvestingOutflows += ca.Amount
			case ClassFinancing:
				wf.FinancingOutflows += ca.Amount
			default:
				wf.OperatingOutflows += ca.Amount
			}
		}

		for _, e := range weekEvents {
			schedule = append(schedule, ScheduleEntry{
				Date: e.Date.Format("2006-01-02"), Amount: e.Amount, Direction: e.Direction,
				Category: e.Category, Description: e.Description, SourceType: e.SourceType,
				SourceID: e.SourceID, Basis: e.Basis, Certainty: e.Certainty,
				CounterpartyID: e.CounterpartyID, WeekNumber: wb.WeekNumber,
			})
		}

		forecasts = append(forecasts, wf)
		runningCash = wf.EndingCash // Week N ending cash == Week N+1 opening cash, by construction.
	}

	return forecasts, schedule
}

// buildFlowTotal aggregates weekEvents matching direction into a
// FlowTotal: total, deterministic category breakdown (fixed enumeration
// order, only categories with a nonzero count included), and basis
// breakdown.
func buildFlowTotal(weekEvents []CashFlowEvent, direction CashDirection) FlowTotal {
	catAmounts := make(map[CashCategory]float64)
	catCounts := make(map[CashCategory]int)
	catEventIDs := make(map[CashCategory]map[string]bool)
	basisAmounts := make(map[CashBasis]float64)
	basisCounts := make(map[CashBasis]int)

	var total float64
	for _, e := range weekEvents {
		if e.Direction != direction {
			continue
		}
		total += e.Amount
		catAmounts[e.Category] += e.Amount
		catCounts[e.Category]++
		if catEventIDs[e.Category] == nil {
			catEventIDs[e.Category] = make(map[string]bool)
		}
		catEventIDs[e.Category][e.ID] = true
		basisAmounts[e.Basis] += e.Amount
		basisCounts[e.Basis]++
	}

	ft := FlowTotal{Total: total}
	for _, cat := range allCategoriesOrder {
		if catCounts[cat] == 0 {
			continue
		}
		ft.ByCategory = append(ft.ByCategory, CategoryAmount{
			Category: cat, Amount: catAmounts[cat], Count: catCounts[cat],
			EventIDs: sortedEventIDs(catEventIDs[cat]),
		})
	}
	for _, basis := range []CashBasis{BasisKnown, BasisScheduled, BasisAssumed, BasisScenario} {
		if basisCounts[basis] == 0 {
			continue
		}
		ft.ByBasis = append(ft.ByBasis, BasisAmount{Basis: basis, Amount: basisAmounts[basis], Count: basisCounts[basis]})
	}
	return ft
}
