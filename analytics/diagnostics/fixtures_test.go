package diagnostics

import (
	"github.com/themurtez/go-valuate/analytics/anomalies"
	"github.com/themurtez/go-valuate/analytics/benchmarks"
	"github.com/themurtez/go-valuate/analytics/cashflow"
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/covenants"
	"github.com/themurtez/go-valuate/analytics/debt"
	"github.com/themurtez/go-valuate/analytics/forecast"
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/analytics/valuedrivers"
	"github.com/themurtez/go-valuate/analytics/variance"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/transactions/salereadiness"
	"github.com/themurtez/go-valuate/valuation/consensus"
)

// healthyFixture builds an Input describing a well-performing business:
// improving profitability, stable liquidity/leverage, strong cash
// conversion, no anomalies, no covenant breaches, favorable benchmarks,
// and sale-readiness strengths — every optional module supplied, none
// carrying a negative signal. Used by tests expecting Strengths and no (or
// few) Concerns.
func healthyFixture() Input {
	return Input{
		Metrics: metrics.Result{
			Snapshots: []metrics.Snapshot{
				{Period: "FY2024", TotalRevenue: metrics.AvailableValue(10_000_000), EBITDA: metrics.AvailableValue(2_200_000)},
			},
		},
		Ratios: ratios.Result{
			Available: true,
			Signals: []ratios.Signal{
				{Code: "IMPROVING_PROFITABILITY", Severity: "info", Period: "FY2024", Message: "EBITDA margin improved from 18.0% to 22.0%", Value: 0.22},
			},
		},
		QoE: qoe.Result{
			Available:        true,
			EBITDAVolatility: metrics.VolatilityResult{Value: metrics.AvailableValue(0.05)},
		},
		WorkingCapital: workingcapital.Result{
			Available: true,
			Trend:     workingcapital.Trend{Direction: workingcapital.TrendStable},
			NWCPercentOfRevenueStatistics: workingcapital.Statistics{
				Volatility: workingcapital.NWCValue{Available: true, Value: 0.05},
			},
		},
		CashFlow: cashflow.Result{Available: true},
		RevenueQuality: revenuequality.Result{
			Available: true,
		},
		Concentration: concentration.Result{Available: true},
		Anomalies:     anomalies.Result{Available: true},
		Variance:      variance.Result{Available: true},
		Forecast:      forecast.Result{Available: true},
		Debt: debt.Result{
			Available: true,
			BaseCase:  debt.CoverageResult{DSCR: debt.AvailableValue(2.5)},
		},
		Covenants: covenants.Result{
			Available: true,
			Tests: []covenants.TestResult{
				{CovenantID: "MIN_DSCR", Metric: covenants.MetricDSCR, Operator: covenants.OperatorGTE, Threshold: 1.25,
					Actual: covenants.AvailableValue(2.5), Status: covenants.StatusPass, WarningBufferStatus: covenants.WarningBufferOutsideBuffer,
					Explanation: "DSCR of 2.50x comfortably exceeds the 1.25x minimum"},
			},
		},
		Benchmarks: benchmarks.Result{
			Available: true,
			Comparisons: []benchmarks.Comparison{
				{MetricID: "GROSS_MARGIN", Favorable: benchmarks.FavorableYes, CompanyValue: benchmarks.AvailableValue(0.45), BenchmarkMedian: benchmarks.AvailableValue(0.35)},
			},
		},
		ValueDrivers: valuedrivers.Result{
			Available: true,
			Baseline:  valuedrivers.Baseline{Consensus: consensus.Result{Available: true, Statistics: consensus.Statistics{SimpleMean: 5_000_000}}},
		},
		SaleReadiness: salereadiness.Result{
			Available: true,
			Strengths: []salereadiness.Strength{
				{Dimension: "EARNINGS_STABILITY", Message: "earnings are stable and well-documented"},
			},
		},
	}
}

// stressedFixture builds an Input describing a business under financial
// stress: margin compression, rising leverage, weak liquidity, weak cash
// conversion, high concentration, anomalies, material variances, a
// covenant breach, an unfavorable benchmark, high downside value
// sensitivity, and sale-readiness blockers/risks. Used by tests expecting
// multiple Concerns across most categories and a low health score.
func stressedFixture() Input {
	return Input{
		Metrics: metrics.Result{
			Snapshots: []metrics.Snapshot{
				{Period: "FY2024", TotalRevenue: metrics.AvailableValue(8_000_000), EBITDA: metrics.AvailableValue(720_000)},
			},
		},
		Ratios: ratios.Result{
			Available: true,
			Signals: []ratios.Signal{
				{Code: "MARGIN_COMPRESSION", Severity: "critical", Period: "FY2024", Message: "EBITDA margin fell from 18.0% to 9.0%", Value: 0.09, Threshold: 0.15},
				{Code: "RISING_LEVERAGE", Severity: "warning", Period: "FY2024", Message: "Debt/EBITDA rose from 2.0x to 4.5x", Value: 4.5, Threshold: 3.0},
				{Code: "WEAKENING_LIQUIDITY", Severity: "warning", Period: "FY2024", Message: "Current ratio fell from 1.8x to 0.9x", Value: 0.9, Threshold: 1.2},
			},
		},
		QoE: qoe.Result{
			Available:        true,
			EBITDAVolatility: metrics.VolatilityResult{Value: metrics.AvailableValue(0.45)},
			Flags: []qoe.Flag{
				{Code: qoe.FlagLargeNormalizationBurden, Severity: "critical", Period: "FY2024", Message: "adjustments total 62% of reported EBITDA", Value: 0.62, Threshold: 0.30},
				{Code: qoe.FlagVolatileEarnings, Severity: "warning", Message: "EBITDA volatility of 45% across 4 fiscal years", Value: 0.45, Threshold: 0.25},
			},
		},
		WorkingCapital: workingcapital.Result{
			Available: true,
			Trend: workingcapital.Trend{
				Direction:   workingcapital.TrendIncreasing,
				FirstValue:  workingcapital.NWCValue{Available: true, Value: 200_000},
				LastValue:   workingcapital.NWCValue{Available: true, Value: 600_000},
				FirstPeriod: "FY2022", LastPeriod: "FY2024",
			},
			NWCPercentOfRevenueStatistics: workingcapital.Statistics{
				Volatility: workingcapital.NWCValue{Available: true, Value: 0.35},
			},
		},
		CashFlow: cashflow.Result{
			Available: true,
			Flags: []cashflow.Flag{
				{Code: cashflow.FlagWeakCashConversion, Severity: "warning", Period: "FY2024", Message: "EBITDA-to-free-cash-flow conversion of 35%", Value: 0.35, Threshold: 0.60},
				{Code: cashflow.FlagLowCashRunway, Severity: "critical", Message: "8.2 months of cash runway remaining", Value: 8.2, Threshold: 12},
			},
		},
		RevenueQuality: revenuequality.Result{
			Available: true,
			Flags: []revenuequality.Flag{
				{Code: revenuequality.FlagDecliningRecurringMix, Severity: "warning", Period: "FY2024", Message: "recurring revenue share fell from 55% to 30%", Value: 0.30, Threshold: 0.40},
			},
		},
		Concentration: concentration.Result{
			Available: true,
			Flags: []concentration.Flag{
				{Code: concentration.FlagHighLargestEntityConcentration, Severity: "critical", Period: "FY2024", Message: "largest customer is 48% of revenue", Value: 0.48, Threshold: 0.25},
			},
		},
		Anomalies: anomalies.Result{
			Available: true,
			Anomalies: []anomalies.Anomaly{
				{Code: "ABSOLUTE_AMOUNT_SPIKE", Severity: "warning", Account: "opex_marketing", Period: "FY2024",
					Observed: anomalies.AvailableValue(45_000), Baseline: anomalies.AvailableValue(10_000),
					Explanation: "OPEX_MARKETING rose from $10,000.00 to $45,000.00 (350.0%), exceeding the 50.0% spike threshold"},
			},
		},
		Variance: variance.Result{
			Available: true,
			MaterialExceptions: []variance.MaterialException{
				{LineVariance: variance.LineVariance{
					AccountCode: "opex_marketing", Label: "Marketing expense", Period: "FY2024",
					Actual: 45_000, BaselineAvailable: true, Baseline: 20_000,
					AbsoluteVariance: variance.AvailableValue(25_000),
				}},
			},
		},
		Forecast: forecast.Result{Available: true},
		Debt: debt.Result{
			Available: true,
			BaseCase:  debt.CoverageResult{DSCR: debt.AvailableValue(0.95)},
			Flags: []debt.Flag{
				{Code: debt.FlagBelowMinimumDSCR, Severity: "critical", Message: "DSCR of 0.95x is below the 1.25x minimum", Value: 0.95, Threshold: 1.25},
			},
		},
		Covenants: covenants.Result{
			Available: true,
			Tests: []covenants.TestResult{
				{CovenantID: "MIN_DSCR", Metric: covenants.MetricDSCR, Operator: covenants.OperatorGTE, Threshold: 1.25,
					Actual: covenants.AvailableValue(0.95), Status: covenants.StatusFail,
					Explanation: "DSCR of 0.95x is below the required minimum of 1.25x (operator >=); breach of 0.30x"},
			},
		},
		Benchmarks: benchmarks.Result{
			Available: true,
			Comparisons: []benchmarks.Comparison{
				{MetricID: "GROSS_MARGIN", Label: "Gross Margin %", Favorable: benchmarks.FavorableNo,
					CompanyValue: benchmarks.AvailableValue(0.20), BenchmarkMedian: benchmarks.AvailableValue(0.35),
					Source: benchmarks.BenchmarkSource{Name: "Industry Survey"}},
			},
		},
		ValueDrivers: valuedrivers.Result{
			Available: true,
			Baseline:  valuedrivers.Baseline{Consensus: consensus.Result{Available: true, Statistics: consensus.Statistics{SimpleMean: 5_000_000}}},
			Scenarios: []valuedrivers.ScenarioResult{
				{ScenarioID: "DOWNSIDE", Label: "Downside case", Available: true, ConsensusDeltaAvailable: true,
					ConsensusPercentDelta: -0.30, Consensus: consensus.Result{Statistics: consensus.Statistics{SimpleMean: 3_500_000}}},
			},
		},
		SaleReadiness: salereadiness.Result{
			Available: true,
			Blockers: []salereadiness.Blocker{
				{Dimension: "EARNINGS_STABILITY", Severity: "critical", Message: "most recent normalized EBITDA is negative"},
			},
			Risks: []salereadiness.Risk{
				{Dimension: "CUSTOMER_CONCENTRATION", Severity: "warning", Message: "largest customer is 48% of revenue against a 25% threshold"},
			},
		},
	}
}

// debtResultNoDebtService builds an analytics/debt.Result carrying only
// FlagNoDebtService — a debt-free business, used by the regression test
// proving this never maps to the DSCR-breach FindingCode.
func debtResultNoDebtService() debt.Result {
	return debt.Result{
		Available: true,
		Flags: []debt.Flag{
			{Code: debt.FlagNoDebtService, Severity: "info", Message: "no debt or loan terms supplied; annual debt service is treated as zero and DSCR/leverage ratios are unavailable"},
		},
	}
}

// salereadinessResultWithOpportunity builds a
// transactions/salereadiness.Result carrying exactly one Opportunity with
// a distinct OpportunityCode, used by the regression test proving
// Finding.SourceCode echoes it rather than only the Dimension.
func salereadinessResultWithOpportunity() salereadiness.Result {
	return salereadiness.Result{
		Available: true,
		Opportunities: []salereadiness.Opportunity{
			{Code: "REDUCE_LEVERAGE", Dimension: "DEBT_LEVERAGE", Message: "Pay down debt or grow EBITDA to reduce the net-debt-to-EBITDA multiple."},
		},
	}
}

// covenantsResultCustomMetric builds an analytics/covenants.Result with a
// single failing MetricCustom test carrying a CustomMetricLabel and no
// separate Label, used by the regression test proving MetricLabel comes
// from CustomMetricLabel rather than the raw "CUSTOM" literal.
func covenantsResultCustomMetric() covenants.Result {
	return covenants.Result{
		Available: true,
		Tests: []covenants.TestResult{
			{CovenantID: "MIN_LIQUIDITY", Metric: covenants.MetricCustom, CustomMetricLabel: "Minimum Liquidity",
				Operator: covenants.OperatorGTE, Threshold: 100_000, Actual: covenants.AvailableValue(40_000),
				Status: covenants.StatusFail, Explanation: "Minimum Liquidity of $40,000 is below the required minimum of $100,000"},
		},
	}
}

// qoeResultDecliningEBITDA builds an analytics/qoe.Result with a single
// FlagDecliningEBITDADespiteRevenueGrowth flag carrying its documented
// Threshold: 0, used by the regression test proving that deliberate zero
// survives as an Available Comparison.
func qoeResultDecliningEBITDA() qoe.Result {
	return qoe.Result{
		Available: true,
		Flags: []qoe.Flag{
			{Code: qoe.FlagDecliningEBITDADespiteRevenueGrowth, Severity: "warning", Period: "FY2024",
				Message: "revenue grew 12.0% from FY2023 to FY2024 while reported EBITDA declined from 500000.00 to 480000.00",
				Value:   -20_000, Threshold: 0},
		},
	}
}

// workingcapitalResultIncreasingNoValues builds an
// analytics/workingcapital.Result whose Trend reports Direction ==
// "increasing" but leaves FirstValue/LastValue both unavailable — an
// inconsistent-but-possible upstream state, used by the regression test
// proving mineWorkingCapital does not fabricate 0.00 evidence from it.
func workingcapitalResultIncreasingNoValues() workingcapital.Result {
	return workingcapital.Result{
		Available: true,
		Trend:     workingcapital.Trend{Direction: workingcapital.TrendIncreasing},
	}
}
