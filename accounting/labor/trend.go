package labor

// TrendPoint is one period's value in a TrendSeries.
type TrendPoint struct {
	Period string  `json:"period"`
	Value  float64 `json:"value"`
}

// TrendSeries is one metric's chronological series plus adjacent-period
// percentage-change points — the task's section 49. Available only when
// at least Policy.MinimumTrendPeriods periods have a value for this
// metric.
type TrendSeries struct {
	Available bool         `json:"available"`
	Points    []TrendPoint `json:"points,omitempty"`
}

// TrendResult is the task's section 49 chronological trend series for
// every metric the task lists. Each series is computed independently —
// a metric unavailable for some periods simply omits those points rather
// than making the whole series unavailable, except that a series with
// fewer than Policy.MinimumTrendPeriods available points is reported
// entirely unavailable (Available: false), since a "trend" of one point
// is not meaningful.
type TrendResult struct {
	TotalLaborCost          TrendSeries `json:"total_labor_cost"`
	GrossPay                TrendSeries `json:"gross_pay"`
	EmployerBurden          TrendSeries `json:"employer_burden"`
	ContractorLabor         TrendSeries `json:"contractor_labor"`
	Headcount               TrendSeries `json:"headcount"`
	FTE                     TrendSeries `json:"fte"`
	OvertimeShare           TrendSeries `json:"overtime_share"`
	LaborCostPercentRevenue TrendSeries `json:"labor_cost_percent_revenue"`
	RevenuePerFTE           TrendSeries `json:"revenue_per_fte"`
	GrossProfitPerFTE       TrendSeries `json:"gross_profit_per_fte"`
	EBITDAPerFTE            TrendSeries `json:"ebitda_per_fte"`

	// RevenueGrowth/LaborCostGrowth/HeadcountGrowth/FTEGrowth/
	// RevenuePerFTEChange/LaborCostPercentRevenueChange are adjacent-period
	// (last-vs-second-to-last) percentage-point or fractional changes —
	// the task's section 20. Each is unavailable if the corresponding
	// series has fewer than two points.
	RevenueGrowth                 Value `json:"revenue_growth"`
	LaborCostGrowth               Value `json:"labor_cost_growth"`
	HeadcountGrowth               Value `json:"headcount_growth"`
	FTEGrowth                     Value `json:"fte_growth"`
	RevenuePerFTEChange           Value `json:"revenue_per_fte_change"`
	LaborCostPercentRevenueChange Value `json:"labor_cost_percent_revenue_change"`
}

func buildTrendSeries(points []TrendPoint, minPeriods int) TrendSeries {
	if len(points) < minPeriods {
		return TrendSeries{}
	}
	return TrendSeries{Available: true, Points: points}
}

// adjacentChange returns the fractional change from first to last of a
// TrendSeries's values (last - first) / first, unavailable if the series
// is unavailable, has fewer than 2 points, or first is zero.
func adjacentChange(series TrendSeries) Value {
	if !series.Available || len(series.Points) < 2 {
		return Unavailable()
	}
	first := series.Points[len(series.Points)-2].Value
	last := series.Points[len(series.Points)-1].Value
	if first == 0 {
		return Unavailable()
	}
	return AvailableValue((last - first) / first)
}

// absoluteAdjacentChange returns last - first (not a percentage), used for
// metrics already expressed as a percentage/ratio (e.g.
// LaborCostPercentRevenue, OvertimeShare) where a percentage-point delta
// is more meaningful than a percent-of-percent.
func absoluteAdjacentChange(series TrendSeries) Value {
	if !series.Available || len(series.Points) < 2 {
		return Unavailable()
	}
	first := series.Points[len(series.Points)-2].Value
	last := series.Points[len(series.Points)-1].Value
	return AvailableValue(last - first)
}

// buildTrend assembles every TrendResult series from the chronologically
// ordered PeriodSummary slice (Calculate already sorts Periods
// chronologically via validatePeriods) plus adjacent-period growth/change
// figures. metricsByPeriod supplies the raw Revenue figures RevenueGrowth
// needs directly, rather than reconstructing revenue by dividing
// TotalLaborCost by LaborCostPercentRevenue (which would be both fragile
// and wrong whenever LaborCostPercentRevenue is zero or unavailable for a
// period that nonetheless has a known revenue).
func buildTrend(summaries []PeriodSummary, metricsByPeriod map[string]BusinessMetrics, minPeriods int) TrendResult {
	var t TrendResult

	var totalLabor, grossPay, burden, contractorLabor, headcountPts, ftePts, otShare, lcpr, revPerFTE, gpPerFTE, ebitdaPerFTE, revenuePts []TrendPoint

	for _, s := range summaries {
		p := s.Period.Period
		totalLabor = append(totalLabor, TrendPoint{Period: p, Value: s.LaborCostBridge.TotalLaborCost})
		grossPay = append(grossPay, TrendPoint{Period: p, Value: s.LaborCostBridge.GrossEmployeePay})
		burden = append(burden, TrendPoint{Period: p, Value: s.LaborCostBridge.EmployerBurden})
		contractorLabor = append(contractorLabor, TrendPoint{Period: p, Value: s.LaborCostBridge.ContractorLabor})

		if s.Headcount.Available {
			headcountPts = append(headcountPts, TrendPoint{Period: p, Value: float64(s.Headcount.EndingHeadcount)})
		}
		if s.FTE.Available {
			ftePts = append(ftePts, TrendPoint{Period: p, Value: s.FTE.FTE})
		}
		if s.Overtime.Available && s.Overtime.OvertimeHoursPercent.Available {
			otShare = append(otShare, TrendPoint{Period: p, Value: s.Overtime.OvertimeHoursPercent.Amount})
		}
		if s.Productivity.LaborCostPercentRevenue.Available {
			lcpr = append(lcpr, TrendPoint{Period: p, Value: s.Productivity.LaborCostPercentRevenue.Amount})
		}
		if s.Productivity.RevenuePerFTE.Available {
			revPerFTE = append(revPerFTE, TrendPoint{Period: p, Value: s.Productivity.RevenuePerFTE.Amount})
		}
		if s.Productivity.GrossProfitPerFTE.Available {
			gpPerFTE = append(gpPerFTE, TrendPoint{Period: p, Value: s.Productivity.GrossProfitPerFTE.Amount})
		}
		if s.Productivity.EBITDAPerFTE.Available {
			ebitdaPerFTE = append(ebitdaPerFTE, TrendPoint{Period: p, Value: s.Productivity.EBITDAPerFTE.Amount})
		}
		if m, ok := metricsByPeriod[p]; ok && m.Revenue.Available {
			revenuePts = append(revenuePts, TrendPoint{Period: p, Value: m.Revenue.Amount})
		}
	}

	t.TotalLaborCost = buildTrendSeries(totalLabor, minPeriods)
	t.GrossPay = buildTrendSeries(grossPay, minPeriods)
	t.EmployerBurden = buildTrendSeries(burden, minPeriods)
	t.ContractorLabor = buildTrendSeries(contractorLabor, minPeriods)
	t.Headcount = buildTrendSeries(headcountPts, minPeriods)
	t.FTE = buildTrendSeries(ftePts, minPeriods)
	t.OvertimeShare = buildTrendSeries(otShare, minPeriods)
	t.LaborCostPercentRevenue = buildTrendSeries(lcpr, minPeriods)
	t.RevenuePerFTE = buildTrendSeries(revPerFTE, minPeriods)
	t.GrossProfitPerFTE = buildTrendSeries(gpPerFTE, minPeriods)
	t.EBITDAPerFTE = buildTrendSeries(ebitdaPerFTE, minPeriods)

	revenueSeries := buildTrendSeries(revenuePts, minPeriods)
	t.RevenueGrowth = adjacentChange(revenueSeries)
	t.LaborCostGrowth = adjacentChange(t.TotalLaborCost)
	t.HeadcountGrowth = adjacentChange(t.Headcount)
	t.FTEGrowth = adjacentChange(t.FTE)
	t.RevenuePerFTEChange = adjacentChange(t.RevenuePerFTE)
	t.LaborCostPercentRevenueChange = absoluteAdjacentChange(t.LaborCostPercentRevenue)

	return t
}
