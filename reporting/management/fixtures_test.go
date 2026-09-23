package management

import (
	"github.com/themurtez/go-valuate/analytics/anomalies"
	"github.com/themurtez/go-valuate/analytics/cashflow"
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/covenants"
	"github.com/themurtez/go-valuate/analytics/debt"
	"github.com/themurtez/go-valuate/analytics/forecast"
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/analytics/variance"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/valuation/consensus"
)

// fullFixture builds an Input exercising every section and every optional
// module, reused by the determinism/round-trip/behavioral tests.
func fullFixture() Input {
	return Input{
		PeriodMeta: map[financial.Period]metrics.PeriodInfo{
			"FY2023": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2023},
			"FY2024": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2024},
		},
		Metrics: metrics.Result{
			FormulaVersion: "1.0.0",
			Snapshots: []metrics.Snapshot{
				{
					Period:             "FY2023",
					TotalRevenue:       metrics.AvailableValue(4_000_000),
					TotalCOGS:          metrics.AvailableValue(2_400_000),
					GrossProfit:        metrics.AvailableValue(1_600_000),
					GrossMargin:        metrics.AvailableValue(0.40),
					TotalOpex:          metrics.AvailableValue(1_100_000),
					EBITDA:             metrics.AvailableValue(500_000),
					EBITDAMargin:       metrics.AvailableValue(0.125),
					NetIncome:          metrics.AvailableValue(300_000),
					Cash:               metrics.AvailableValue(200_000),
					AccountsReceivable: metrics.AvailableValue(350_000),
					Inventory:          metrics.AvailableValue(150_000),
					CurrentAssets:      metrics.AvailableValue(700_000),
					AccountsPayable:    metrics.AvailableValue(250_000),
					CurrentLiabilities: metrics.AvailableValue(400_000),
					WorkingCapital:     metrics.AvailableValue(300_000),
					TotalDebt:          metrics.AvailableValue(1_500_000),
					NetDebt:            metrics.AvailableValue(1_300_000),
				},
				{
					Period:             "FY2024",
					TotalRevenue:       metrics.AvailableValue(5_000_000),
					TotalCOGS:          metrics.AvailableValue(2_900_000),
					GrossProfit:        metrics.AvailableValue(2_100_000),
					GrossMargin:        metrics.AvailableValue(0.42),
					TotalOpex:          metrics.AvailableValue(1_300_000),
					EBITDA:             metrics.AvailableValue(700_000),
					EBITDAMargin:       metrics.AvailableValue(0.14),
					NetIncome:          metrics.AvailableValue(420_000),
					Cash:               metrics.AvailableValue(280_000),
					AccountsReceivable: metrics.AvailableValue(400_000),
					Inventory:          metrics.AvailableValue(180_000),
					CurrentAssets:      metrics.AvailableValue(860_000),
					AccountsPayable:    metrics.AvailableValue(300_000),
					CurrentLiabilities: metrics.AvailableValue(480_000),
					WorkingCapital:     metrics.AvailableValue(380_000),
					TotalDebt:          metrics.AvailableValue(1_400_000),
					NetDebt:            metrics.AvailableValue(1_120_000),
				},
			},
		},
		Ratios: ratios.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			History: []ratios.PeriodRatios{
				{
					Period:           "FY2023",
					GrossMargin:      ratios.Ratio{Value: metrics.AvailableValue(0.40)},
					OperatingMargin:  ratios.Ratio{Value: metrics.AvailableValue(0.15)},
					EBITDAMargin:     ratios.Ratio{Value: metrics.AvailableValue(0.125)},
					NetMargin:        ratios.Ratio{Value: metrics.AvailableValue(0.075)},
					ReturnOnAssets:   ratios.Ratio{Value: metrics.AvailableValue(0.10)},
					ReturnOnEquity:   ratios.Ratio{Value: metrics.AvailableValue(0.22)},
					CurrentRatio:     ratios.Ratio{Value: metrics.AvailableValue(1.75)},
					QuickRatio:       ratios.Ratio{Value: metrics.AvailableValue(1.20)},
					CashRatio:        ratios.Ratio{Value: metrics.AvailableValue(0.50)},
					DebtToEquity:     ratios.Ratio{Value: metrics.AvailableValue(1.10)},
					DebtToAssets:     ratios.Ratio{Value: metrics.AvailableValue(0.45)},
					DebtToEBITDA:     ratios.Ratio{Value: metrics.AvailableValue(3.0)},
					NetDebtToEBITDA:  ratios.Ratio{Value: metrics.AvailableValue(2.6)},
					InterestCoverage: ratios.Ratio{Value: metrics.AvailableValue(4.5)},
				},
				{
					Period:           "FY2024",
					GrossMargin:      ratios.Ratio{Value: metrics.AvailableValue(0.42)},
					OperatingMargin:  ratios.Ratio{Value: metrics.AvailableValue(0.16)},
					EBITDAMargin:     ratios.Ratio{Value: metrics.AvailableValue(0.14)},
					NetMargin:        ratios.Ratio{Value: metrics.AvailableValue(0.084)},
					ReturnOnAssets:   ratios.Ratio{Value: metrics.AvailableValue(0.12)},
					ReturnOnEquity:   ratios.Ratio{Value: metrics.AvailableValue(0.25)},
					CurrentRatio:     ratios.Ratio{Value: metrics.AvailableValue(1.79)},
					QuickRatio:       ratios.Ratio{Value: metrics.AvailableValue(1.25)},
					CashRatio:        ratios.Ratio{Value: metrics.AvailableValue(0.58)},
					DebtToEquity:     ratios.Ratio{Value: metrics.AvailableValue(0.95)},
					DebtToAssets:     ratios.Ratio{Value: metrics.AvailableValue(0.40)},
					DebtToEBITDA:     ratios.Ratio{Value: metrics.AvailableValue(2.0)},
					NetDebtToEBITDA:  ratios.Ratio{Value: metrics.AvailableValue(1.6)},
					InterestCoverage: ratios.Ratio{Value: metrics.AvailableValue(5.5)},
				},
			},
		},
		CashFlow: cashflow.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			History: []cashflow.Bridge{
				{
					Period:            "FY2023",
					EBITDA:            metrics.AvailableValue(500_000),
					OperatingCashFlow: cashflow.CashFlowValue{Available: true, Value: 420_000},
					Capex:             cashflow.CashFlowValue{Available: true, Value: 80_000},
					FreeCashFlow:      cashflow.CashFlowValue{Available: true, Value: 340_000},
				},
				{
					Period:            "FY2024",
					EBITDA:            metrics.AvailableValue(700_000),
					OperatingCashFlow: cashflow.CashFlowValue{Available: true, Value: 600_000},
					Capex:             cashflow.CashFlowValue{Available: true, Value: 100_000},
					FreeCashFlow:      cashflow.CashFlowValue{Available: true, Value: 500_000},
				},
			},
			Conversion: []cashflow.ConversionRatios{
				{Period: "FY2023", EBITDAToOperatingCashFlow: metrics.AvailableValue(0.84), EBITDAToFreeCashFlow: metrics.AvailableValue(0.68)},
				{Period: "FY2024", EBITDAToOperatingCashFlow: metrics.AvailableValue(0.857), EBITDAToFreeCashFlow: metrics.AvailableValue(0.714)},
			},
			CashRunway: cashflow.CashRunway{Available: true, MonthsOfRunway: metrics.AvailableValue(18)},
		},
		WorkingCapital: workingcapital.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			History: []workingcapital.PeriodNWC{
				{Period: "FY2023", NWC: workingcapital.NWCValue{Available: true, Value: 300_000}, NWCPercentOfRevenue: workingcapital.NWCValue{Available: true, Value: 0.075}},
				{Period: "FY2024", NWC: workingcapital.NWCValue{Available: true, Value: 380_000}, NWCPercentOfRevenue: workingcapital.NWCValue{Available: true, Value: 0.076}},
			},
			NWCStatistics: workingcapital.Statistics{Average: workingcapital.NWCValue{Available: true, Value: 340_000}},
			SuggestedPeg:  workingcapital.SuggestedPeg{Value: workingcapital.NWCValue{Available: true, Value: 350_000}},
		},
		QoE: qoe.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			Flags: []qoe.Flag{
				{Code: qoe.FlagLargeOwnerDiscretionaryComponent, Severity: qoe.FlagSeverityWarning, Period: "FY2024", Message: "large owner-discretionary component in FY2024"},
			},
		},
		Variance: variance.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			LineVariances: []variance.LineVariance{
				{
					AccountCode:       "REVENUE_PRODUCT",
					Label:             "Product Revenue",
					Period:            "FY2024",
					Actual:            5_000_000,
					BaselineAvailable: true,
					Baseline:          4_700_000,
					AbsoluteVariance:  variance.VarianceValue{Available: true, Value: 300_000},
					PercentVariance:   variance.VarianceValue{Available: true, Value: 0.0638},
					Favorability:      variance.FavorabilityFavorable,
				},
			},
			TopFavorable: []variance.LineVariance{
				{
					AccountCode:       "REVENUE_PRODUCT",
					Label:             "Product Revenue",
					Period:            "FY2024",
					Actual:            5_000_000,
					BaselineAvailable: true,
					Baseline:          4_700_000,
					AbsoluteVariance:  variance.VarianceValue{Available: true, Value: 300_000},
					PercentVariance:   variance.VarianceValue{Available: true, Value: 0.0638},
					Favorability:      variance.FavorabilityFavorable,
				},
			},
		},
		Forecast: forecast.Result{
			Available:       true,
			FormulaVersion:  "1.0.0",
			Horizon:         2,
			ForecastPeriods: []string{"FY2025", "FY2026"},
			ScenarioResults: []forecast.ScenarioResult{
				{
					Name: "Base Case",
					Type: forecast.ScenarioTypeBase,
					ProjectedPeriods: []forecast.PeriodPL{
						{
							Period:       "FY2025",
							PeriodNumber: 1,
							TotalRevenue: forecast.ForecastValue{Available: true, Value: 5_500_000},
							GrossProfit:  forecast.ForecastValue{Available: true, Value: 2_310_000},
							EBITDA:       forecast.ForecastValue{Available: true, Value: 770_000},
							NetIncome:    forecast.ForecastValue{Available: true, Value: 460_000},
						},
						{
							Period:       "FY2026",
							PeriodNumber: 2,
							TotalRevenue: forecast.ForecastValue{Available: true, Value: 6_000_000},
							GrossProfit:  forecast.ForecastValue{Available: true, Value: 2_520_000},
							EBITDA:       forecast.ForecastValue{Available: true, Value: 840_000},
							NetIncome:    forecast.ForecastValue{Available: true, Value: 500_000},
						},
					},
					CashFlow: []forecast.CashFlowPeriod{
						{Period: "FY2025", FreeCashFlow: forecast.ForecastValue{Available: true, Value: 550_000}},
						{Period: "FY2026", FreeCashFlow: forecast.ForecastValue{Available: true, Value: 600_000}},
					},
				},
			},
		},
		Anomalies: anomalies.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			Anomalies: []anomalies.Anomaly{
				{
					Code:        anomalies.RuleMarginDeterioration,
					Severity:    anomalies.AnomalySeverityWarning,
					Account:     "OPEX_MARKETING",
					Period:      "FY2024",
					Explanation: "marketing expense grew faster than revenue in FY2024",
				},
			},
		},
		Concentration: concentration.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			History: []concentration.PeriodConcentration{
				{Period: "FY2023", LargestEntityShare: concentration.ConcentrationValue{Available: true, Value: 0.28}},
				{Period: "FY2024", LargestEntityShare: concentration.ConcentrationValue{Available: true, Value: 0.31}},
			},
		},
		RevenueQuality: revenuequality.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			TotalRevenueHistory: []revenuequality.PeriodRevenue{
				{Period: "FY2023", RecurringPercent: revenuequality.RevenueValue{Available: true, Value: 0.33}},
				{Period: "FY2024", RecurringPercent: revenuequality.RevenueValue{Available: true, Value: 0.37}},
			},
		},
		Debt: debt.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			BaseCase: debt.CoverageResult{
				DSCR:            debt.Value{Available: true, Amount: 1.8},
				DebtToEBITDA:    debt.Value{Available: true, Amount: 2.0},
				NetDebtToEBITDA: debt.Value{Available: true, Amount: 1.6},
			},
			Flags: []debt.Flag{
				{Code: debt.FlagBelowMinimumFixedChargeCoverage, Severity: debt.FlagSeverityWarning, Message: "fixed charge coverage is below the policy minimum"},
			},
		},
		Covenants: covenants.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			Tests: []covenants.TestResult{
				{
					CovenantID:          "MAX_LEVERAGE",
					Label:               "Maximum Net Debt / EBITDA",
					Period:              "FY2024",
					Status:              covenants.StatusPass,
					WarningBufferStatus: covenants.WarningBufferWithinBuffer,
					Explanation:         "net debt / EBITDA of 1.6x is within the warning buffer of the 2.0x maximum",
				},
			},
		},
		Consensus: consensus.Result{
			Available:      true,
			FormulaVersion: "1.0.0",
			WeightsValid:   true,
			Statistics: consensus.Statistics{
				Count:        3,
				WeightedMean: 4_200_000,
				SimpleMean:   4_100_000,
			},
		},
	}
}
