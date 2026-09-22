package metrics

import (
	"math"

	"github.com/themurtez/go-valuate/financial"
)

// Trend holds historical/trend metrics computed across a dataset's periods:
// growth rates, CAGR, margin trends, and volatility statistics.
//
// MVP restriction: every calculation in Trend compares fiscal-year periods
// to other fiscal-year periods only (PeriodTypeFiscalYear). Quarters,
// months, and YTD periods are excluded from these calculations even when
// present in the dataset, because naively mixing granularities (e.g.
// treating consecutive quarters as if they were consecutive years) would
// produce misleading growth/CAGR figures. A future version may add
// dedicated quarter-over-quarter or month-over-month calculations; for now,
// only fiscal-year comparisons are made, and Error explains when even that
// was not possible.
type Trend struct {
	// Error explains why NO trend metrics could be computed at all — e.g.
	// PeriodInfo was not supplied, or fewer than two comparable fiscal-year
	// periods exist. When Error is non-nil, every other field is zero-value.
	// This is deliberately an explicit field (as opposed to Calculate
	// returning a Go error) since a missing trend does not invalidate the
	// per-period Snapshots Calculate also returns.
	Error *PeriodOrderError `json:"error,omitempty"`

	// FiscalYearsUsed lists, in chronological order, the fiscal-year periods
	// that participated in these calculations (for traceability).
	FiscalYearsUsed []string `json:"fiscal_years_used,omitempty"`

	// RevenueYoYGrowth is year-over-year total revenue growth for each
	// fiscal-year transition, e.g. one entry for 2024->2025. See
	// GrowthPoint.
	RevenueYoYGrowth []GrowthPoint `json:"revenue_yoy_growth,omitempty"`
	// RevenueCAGR is the compound annual growth rate of total revenue across
	// the full fiscal-year span used. See CAGRResult and calculateCAGR's
	// doc comment for the exact formula and validity rules.
	RevenueCAGR CAGRResult `json:"revenue_cagr"`
	// RevenueVolatility is the volatility of the total-revenue series. See
	// VolatilityResult for the exact statistic used.
	RevenueVolatility VolatilityResult `json:"revenue_volatility"`

	// EBITDAYoYGrowth is year-over-year EBITDA growth.
	EBITDAYoYGrowth []GrowthPoint `json:"ebitda_yoy_growth,omitempty"`
	// EBITDAMarginTrend is the EBITDA margin for each fiscal year used (not
	// a growth rate — the margin level itself, so callers can plot or
	// compare it directly).
	EBITDAMarginTrend []MarginPoint `json:"ebitda_margin_trend,omitempty"`
	// EBITDAVolatility is the volatility of the EBITDA series.
	EBITDAVolatility VolatilityResult `json:"ebitda_volatility"`

	// SDEYoYGrowth is year-over-year SDE growth.
	SDEYoYGrowth []GrowthPoint `json:"sde_yoy_growth,omitempty"`

	// EarningsCAGR is the CAGR of net income across the full fiscal-year
	// span used, where "meaningful" per the calculateCAGR validity rules
	// (requires a positive starting net income).
	EarningsCAGR CAGRResult `json:"earnings_cagr"`
	// EarningsVolatility is the volatility of the net-income series.
	EarningsVolatility VolatilityResult `json:"earnings_volatility"`

	// GrossMarginTrend is the gross margin for each fiscal year used.
	GrossMarginTrend []MarginPoint `json:"gross_margin_trend,omitempty"`
}

// GrowthPoint is a single year-over-year growth calculation between two
// adjacent fiscal years. Every []GrowthPoint series on Trend (e.g.
// RevenueYoYGrowth) is ordered chronologically, matching
// Trend.FiscalYearsUsed — never relies on map iteration order.
type GrowthPoint struct {
	// FromPeriod and ToPeriod are the fiscal-year periods compared.
	FromPeriod string `json:"from_period"`
	ToPeriod   string `json:"to_period"`
	// Growth is (to - from) / |from|, as a decimal (0.10 = 10% growth).
	// Available is false if either value is Unavailable, or if from is
	// exactly 0 (growth from a zero base is undefined as a percentage; see
	// GrowthFromZeroBase for the explicit signal instead).
	Growth MetricValue `json:"growth"`
	// GrowthFromZeroBase is true when FromPeriod's value was exactly 0,
	// explaining why Growth is Unavailable in that specific case (as
	// opposed to being unavailable because an input metric itself was
	// unavailable).
	GrowthFromZeroBase bool `json:"growth_from_zero_base,omitempty"`
}

// MarginPoint is a single period's margin level (not a growth rate).
// Every []MarginPoint series on Trend (e.g. EBITDAMarginTrend) is ordered
// chronologically, matching Trend.FiscalYearsUsed.
type MarginPoint struct {
	Period string      `json:"period"`
	Margin MetricValue `json:"margin"`
}

// CAGRResult is a compound annual growth rate calculation plus enough
// context to explain why it may be unavailable.
type CAGRResult struct {
	Value MetricValue `json:"value"`
	// Years is the number of years spanned (ToPeriod fiscal year minus
	// FromPeriod fiscal year). Meaningful only when Value.Available.
	Years int `json:"years,omitempty"`
	// Invalid explains why CAGR could not be validly computed even though
	// both endpoint values exist, e.g. a non-positive starting value. Empty
	// when Value.Available is true, or when an endpoint metric itself was
	// simply unavailable (in which case CAGR is unavailable for the
	// ordinary "missing data" reason and Invalid adds no further
	// information).
	Invalid string `json:"invalid,omitempty"`
}

// VolatilityResult is a volatility calculation plus its sample size.
type VolatilityResult struct {
	Value MetricValue `json:"value"`
	// SampleSize is the number of year-over-year growth observations the
	// statistic was computed from. Volatility requires at least 2
	// observations (3 fiscal years); see calculateVolatility.
	SampleSize int `json:"sample_size,omitempty"`
}

// calculateTrend computes every Trend field from a chronologically-sortable
// set of Snapshots. snapshots is in dataset order (not necessarily
// chronological); meta supplies the ordering used to sort and to restrict
// calculations to fiscal-year periods.
func calculateTrend(snapshots []Snapshot, meta map[financial.Period]PeriodInfo) Trend {
	periods := make([]financial.Period, 0, len(snapshots))
	byPeriod := make(map[financial.Period]Snapshot, len(snapshots))
	for _, s := range snapshots {
		periods = append(periods, s.Period)
		byPeriod[s.Period] = s
	}

	ordered, orderErr := resolvePeriodOrder(periods, meta)
	if orderErr != nil {
		return Trend{Error: orderErr}
	}

	fy := fiscalYearPeriods(ordered)
	if len(fy) < 2 {
		return Trend{Error: &PeriodOrderError{
			Reason: "fewer than two comparable fiscal-year periods available for trend calculations",
		}}
	}

	fySnapshots := make([]Snapshot, 0, len(fy))
	years := make([]string, 0, len(fy))
	fiscalYears := make([]int, 0, len(fy))
	for _, p := range fy {
		fySnapshots = append(fySnapshots, byPeriod[p.period])
		years = append(years, string(p.period))
		fiscalYears = append(fiscalYears, p.info.FiscalYear)
	}

	t := Trend{FiscalYearsUsed: years}

	revenueSeries := extractSeries(fySnapshots, func(s Snapshot) MetricValue { return s.TotalRevenue })
	ebitdaSeries := extractSeries(fySnapshots, func(s Snapshot) MetricValue { return s.EBITDA })
	sdeSeries := extractSeries(fySnapshots, func(s Snapshot) MetricValue { return s.SDE })
	netIncomeSeries := extractSeries(fySnapshots, func(s Snapshot) MetricValue { return s.NetIncome })
	grossMarginSeries := extractSeries(fySnapshots, func(s Snapshot) MetricValue { return s.GrossMargin })
	ebitdaMarginSeries := extractSeries(fySnapshots, func(s Snapshot) MetricValue { return s.EBITDAMargin })

	t.RevenueYoYGrowth = calculateYoYGrowth(years, revenueSeries)
	t.RevenueCAGR = calculateCAGR(fiscalYears, revenueSeries)
	t.RevenueVolatility = calculateVolatility(revenueSeries)

	t.EBITDAYoYGrowth = calculateYoYGrowth(years, ebitdaSeries)
	t.EBITDAMarginTrend = marginPoints(years, ebitdaMarginSeries)
	t.EBITDAVolatility = calculateVolatility(ebitdaSeries)

	t.SDEYoYGrowth = calculateYoYGrowth(years, sdeSeries)

	t.EarningsCAGR = calculateCAGR(fiscalYears, netIncomeSeries)
	t.EarningsVolatility = calculateVolatility(netIncomeSeries)

	t.GrossMarginTrend = marginPoints(years, grossMarginSeries)

	return t
}

// extractSeries pulls one MetricValue per snapshot using getter, preserving
// order.
func extractSeries(snapshots []Snapshot, getter func(Snapshot) MetricValue) []MetricValue {
	series := make([]MetricValue, len(snapshots))
	for i, s := range snapshots {
		series[i] = getter(s)
	}
	return series
}

// calculateYoYGrowth computes (to-from)/|from| for each adjacent pair in
// series, keyed by the corresponding fiscal-year labels in years. Both
// slices must be the same length and chronologically ordered.
func calculateYoYGrowth(years []string, series []MetricValue) []GrowthPoint {
	if len(series) < 2 {
		return nil
	}
	points := make([]GrowthPoint, 0, len(series)-1)
	for i := 1; i < len(series); i++ {
		from, to := series[i-1], series[i]
		point := GrowthPoint{FromPeriod: years[i-1], ToPeriod: years[i]}
		switch {
		case !from.Available || !to.Available:
			// Growth left Unavailable.
		case from.Value == 0:
			point.GrowthFromZeroBase = true
		default:
			point.Growth = AvailableValue((to.Value - from.Value) / math.Abs(from.Value))
		}
		points = append(points, point)
	}
	return points
}

// calculateCAGR computes the compound annual growth rate across the full
// span of series:
//
//	CAGR = (End / Start)^(1/Years) - 1
//
// where Years is fiscalYears[last] - fiscalYears[0], taken directly from
// caller-supplied PeriodInfo.FiscalYear (never re-derived from a period's
// string label, since financial.Period carries no guaranteed format).
//
// Validity rules, applied in order:
//   - if the first or last value is Unavailable, CAGR is Unavailable (no
//     Invalid reason set — this is ordinary missing-data unavailability).
//   - if the fiscal-year span is not positive (e.g. duplicate/misordered
//     fiscal years), CAGR is Unavailable with Invalid explaining why.
//   - if the starting value is <= 0, CAGR is mathematically undefined as a
//     real-valued growth rate (a negative or zero base makes the ratio
//     End/Start either undefined, negative, or not meaningfully
//     interpretable as compound growth) — CAGR is Unavailable with Invalid
//     explaining why, rather than computing a misleading number.
//
// A negative ending value is allowed through the formula: it produces a
// negative ratio, and math.Pow of a negative base to a fractional exponent
// is NaN, which is explicitly caught below and reported as Invalid rather
// than returned as a silent NaN.
func calculateCAGR(fiscalYears []int, series []MetricValue) CAGRResult {
	if len(series) < 2 {
		return CAGRResult{}
	}
	start := series[0]
	end := series[len(series)-1]
	if !start.Available || !end.Available {
		return CAGRResult{}
	}

	span := fiscalYears[len(fiscalYears)-1] - fiscalYears[0]
	if span <= 0 {
		return CAGRResult{Invalid: "fiscal year span is not positive"}
	}

	if start.Value <= 0 {
		return CAGRResult{
			Years:   span,
			Invalid: "starting value must be positive to compute a meaningful CAGR",
		}
	}

	ratio := end.Value / start.Value
	cagr := math.Pow(ratio, 1.0/float64(span)) - 1
	if math.IsNaN(cagr) || math.IsInf(cagr, 0) {
		return CAGRResult{
			Years:   span,
			Invalid: "computed CAGR is not a finite real number",
		}
	}

	return CAGRResult{Value: AvailableValue(cagr), Years: span}
}

// calculateVolatility computes the sample standard deviation of the
// year-over-year percentage-change series derived from series (not the
// standard deviation of the raw levels), expressed as a decimal (0.25 =
// 25%). This is the standard "earnings volatility" statistic used in
// valuation contexts: it measures how much the growth rate itself swings
// year to year, which is more meaningful for risk assessment than the
// standard deviation of raw dollar levels (which would scale with the size
// of the business rather than its stability).
//
// Requires at least 3 fiscal years (2 growth-rate observations) with every
// underlying value Available and no zero-base growth points; if fewer than
// 2 valid growth observations exist, Volatility is Unavailable.
//
// Sample (not population) standard deviation is used (divide by n-1),
// consistent with treating the observed years as a sample of the
// business's underlying earnings behavior rather than its entire
// population of possible outcomes.
func calculateVolatility(series []MetricValue) VolatilityResult {
	var growthRates []float64
	for i := 1; i < len(series); i++ {
		from, to := series[i-1], series[i]
		if !from.Available || !to.Available || from.Value == 0 {
			continue
		}
		growthRates = append(growthRates, (to.Value-from.Value)/math.Abs(from.Value))
	}
	if len(growthRates) < 2 {
		return VolatilityResult{SampleSize: len(growthRates)}
	}

	mean := 0.0
	for _, g := range growthRates {
		mean += g
	}
	mean /= float64(len(growthRates))

	sumSquares := 0.0
	for _, g := range growthRates {
		diff := g - mean
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(growthRates)-1)
	stddev := math.Sqrt(variance)

	return VolatilityResult{Value: AvailableValue(stddev), SampleSize: len(growthRates)}
}

// marginPoints converts a series of margin MetricValues into MarginPoint
// entries labeled by year.
func marginPoints(years []string, series []MetricValue) []MarginPoint {
	points := make([]MarginPoint, len(series))
	for i, v := range series {
		points[i] = MarginPoint{Period: years[i], Margin: v}
	}
	return points
}
