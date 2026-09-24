package kpi

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/accounting/cashforecast"
	"github.com/themurtez/go-valuate/accounting/inventory"
	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/vendorspend"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// This file proves each *adapter.go file actually bridges its sibling
// package's REAL Calculate output (not a hand-built stand-in) into
// MetricValues that this engine can evaluate a KPI against end to end —
// task section 32's "lightweight adapters/tests for representative
// outputs" and section 49's "multi-module adapter fixture."

func TestAdapter_Financial(t *testing.T) {
	snap := metrics.Snapshot{
		TotalRevenue: metrics.AvailableValue(1000000),
		GrossProfit:  metrics.AvailableValue(400000),
		GrossMargin:  metrics.AvailableValue(0.4),
		EBITDA:       metrics.Unavailable(),
	}
	mvs := FinancialMetricValues(snap, "2025", "USD")

	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: FinancialMetricRevenue})}
	res := Calculate(Input{Definitions: []Definition{def}, Metrics: mvs, Periods: []Period{{Code: "2025", Sequence: 1}}}, Options{})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 1000000 {
		t.Fatalf("got %+v", kr.Value)
	}

	// EBITDA was unavailable in the source Snapshot -- must stay
	// unavailable through the adapter, never silently become 0.
	defEBITDA := Definition{Code: "ebitda", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: FinancialMetricEBITDA})}
	res2 := Calculate(Input{Definitions: []Definition{defEBITDA}, Metrics: mvs, Periods: []Period{{Code: "2025", Sequence: 1}}}, Options{})
	kr2 := firstResult(t, res2, "ebitda")
	if kr2.Value.Available {
		t.Fatalf("EBITDA should remain unavailable through the adapter, got %+v", kr2.Value)
	}
}

func TestAdapter_Labor(t *testing.T) {
	ps := labor.PeriodSummary{
		Period:          labor.PeriodInfo{Period: "2025"},
		LaborCostBridge: labor.LaborCostBridge{TotalLaborCost: 500000},
		FTE:             labor.FTESummary{Available: true, FTE: 15},
	}
	mvs := LaborMetricValues(ps, "USD")

	def := Definition{Code: "k", Unit: Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"}, Formula: Binary(OpDivide, Metric(MetricRef{Code: "revenue"}), Metric(MetricRef{Code: LaborMetricFTE}))}
	allMetrics := append(mvs, metricInput("revenue", "2025", 1500000, true, currencyUnit("USD")))
	res := Calculate(Input{Definitions: []Definition{def}, Metrics: allMetrics, Periods: []Period{{Code: "2025", Sequence: 1}}}, Options{})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 100000 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestAdapter_AR(t *testing.T) {
	res := ar.Result{
		Available: true,
		Buckets: []ar.BucketDefinition{
			{Code: "CURRENT", MinDaysPastDue: 0},
			{Code: "1_30", MinDaysPastDue: 1, MaxDaysPastDue: 30, HasMax: true},
			{Code: "31_60", MinDaysPastDue: 31, MaxDaysPastDue: 60, HasMax: true},
			{Code: "61_90", MinDaysPastDue: 61, MaxDaysPastDue: 90, HasMax: true},
			{Code: "91_PLUS", MinDaysPastDue: 91},
		},
		PortfolioSummary: ar.PortfolioSummary{
			TotalOpenReceivables: 100000,
			Buckets: []ar.BucketAmount{
				{BucketCode: "CURRENT", Amount: 60000},
				{BucketCode: "1_30", Amount: 20000},
				{BucketCode: "31_60", Amount: 10000},
				{BucketCode: "61_90", Amount: 6000},
				{BucketCode: "91_PLUS", Amount: 4000},
			},
		},
		DSO: ar.DSOResult{Available: true, Value: 42},
	}
	mvs := ARMetricValues(res, "2025", "USD")

	def := Definition{Code: "k", Unit: Unit{Kind: UnitPercent}, Formula: Binary(OpPercent, Metric(MetricRef{Code: ARMetricOver60}), Metric(MetricRef{Code: ARMetricTotalOpenAR}))}
	calcRes := Calculate(Input{Definitions: []Definition{def}, Metrics: mvs, Periods: []Period{{Code: "2025", Sequence: 1}}}, Options{})
	kr := firstResult(t, calcRes, "k")
	// over-60 = 61_90 (6000) + 91_PLUS (4000) = 10000; 10000/100000*100 = 10%
	if !kr.Value.Available || kr.Value.Amount != 10 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestAdapter_AP(t *testing.T) {
	res := ap.Result{
		Available:        true,
		PortfolioSummary: ap.PortfolioSummary{TotalOpenPayables: 50000},
		DPO:              ap.DPOResult{Available: true, Value: 35},
	}
	mvs := APMetricValues(res, "2025", "USD")
	def := Definition{Code: "k", Unit: Unit{Kind: UnitDays}, Formula: Metric(MetricRef{Code: APMetricDPO})}
	calcRes := Calculate(Input{Definitions: []Definition{def}, Metrics: mvs, Periods: []Period{{Code: "2025", Sequence: 1}}}, Options{})
	kr := firstResult(t, calcRes, "k")
	if !kr.Value.Available || kr.Value.Amount != 35 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestAdapter_Inventory(t *testing.T) {
	ps := inventory.PeriodSummary{
		Period:   inventory.PeriodInfo{Period: "2025"},
		Turnover: inventory.TurnoverResult{Available: true, Value: 6},
		DIO:      inventory.DIOResult{Available: true, Value: 60},
	}
	mvs := InventoryMetricValues(ps, 250000, "USD")

	def := Definition{Code: "turns", Unit: Unit{Kind: UnitRatio}, Formula: Metric(MetricRef{Code: InventoryMetricTurnover})}
	res := Calculate(Input{Definitions: []Definition{def}, Metrics: mvs, Periods: []Period{{Code: "2025", Sequence: 1}}}, Options{})
	kr := firstResult(t, res, "turns")
	if !kr.Value.Available || kr.Value.Amount != 6 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestAdapter_Profitability(t *testing.T) {
	bpt := profitability.BusinessPeriodTotals{
		Period:  "2025",
		Margins: profitability.Margins{ContributionMargin: profitability.Value{Available: true, Amount: 28.5}},
	}
	mvs := ProfitabilityMetricValues(bpt)
	def := Definition{Code: "k", Unit: Unit{Kind: UnitPercent}, Formula: Metric(MetricRef{Code: ProfitabilityMetricContributionMargin})}
	res := Calculate(Input{Definitions: []Definition{def}, Metrics: mvs, Periods: []Period{{Code: "2025", Sequence: 1}}}, Options{})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 28.5 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestAdapter_VendorSpend(t *testing.T) {
	ps := vendorspend.PeriodSummary{Period: "2025", Bridge: vendorspend.SpendBridge{NetSpend: 750000}}
	mvs := VendorSpendMetricValues(ps, "USD")
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: VendorSpendMetricTotalSpend})}
	res := Calculate(Input{Definitions: []Definition{def}, Metrics: mvs, Periods: []Period{{Code: "2025", Sequence: 1}}}, Options{})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 750000 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestAdapter_CashForecast(t *testing.T) {
	sr := cashforecast.ScenarioResult{
		Summary: cashforecast.LiquiditySummary{EndingCash: 120000, ThresholdAvailable: true, MaximumFundingGap: 15000},
	}
	mvs := CashForecastMetricValues(sr, "2025-W01", "USD")
	def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: Metric(MetricRef{Code: CashForecastMetricEndingCash})}
	res := Calculate(Input{Definitions: []Definition{def}, Metrics: mvs, Periods: []Period{{Code: "2025-W01", Sequence: 1}}}, Options{})
	kr := firstResult(t, res, "k")
	if !kr.Value.Available || kr.Value.Amount != 120000 {
		t.Fatalf("got %+v", kr.Value)
	}
}

func TestAdapter_CashForecast_FundingGapUnavailable(t *testing.T) {
	// ThresholdAvailable=false must make MaximumFundingGap unavailable,
	// not a misleading 0.
	sr := cashforecast.ScenarioResult{
		Summary: cashforecast.LiquiditySummary{EndingCash: 120000, ThresholdAvailable: false},
	}
	mvs := CashForecastMetricValues(sr, "2025-W01", "USD")
	for _, mv := range mvs {
		if mv.Code == CashForecastMetricMaximumFundingGap && mv.Available {
			t.Fatalf("MaximumFundingGap should be unavailable when ThresholdAvailable is false, got %+v", mv)
		}
	}
}

// TestAdapter_MultiModuleComposite proves a KPI can combine metrics from
// multiple different adapters in one formula -- task section 49's
// "multi-module adapter fixture."
func TestAdapter_MultiModuleComposite(t *testing.T) {
	finMVs := FinancialMetricValues(metrics.Snapshot{TotalRevenue: metrics.AvailableValue(2000000)}, "2025", "USD")
	laborMVs := LaborMetricValues(labor.PeriodSummary{
		Period: labor.PeriodInfo{Period: "2025"},
		FTE:    labor.FTESummary{Available: true, FTE: 20},
	}, "USD")

	var all []MetricValue
	all = append(all, finMVs...)
	all = append(all, laborMVs...)

	def := Definition{Code: "revenue_per_fte", Unit: Unit{Kind: UnitCustom, CustomLabel: "USD_PER_COUNT"},
		Formula: Binary(OpDivide, Metric(MetricRef{Code: FinancialMetricRevenue}), Metric(MetricRef{Code: LaborMetricFTE}))}
	res := Calculate(Input{Definitions: []Definition{def}, Metrics: all, Periods: []Period{{Code: "2025", Sequence: 1}}}, Options{})
	kr := firstResult(t, res, "revenue_per_fte")
	if !kr.Value.Available || kr.Value.Amount != 100000 {
		t.Fatalf("got %+v", kr.Value)
	}
}
