package diagnostics

// stableBusiness returns a snapshot with no concerning signals and a
// healthy Prior — used as a "no findings expected" baseline across tests.
func stableBusiness(id string) BusinessSnapshot {
	prior := &BusinessSnapshot{
		ID:     id,
		Period: "FY2023",
		Metrics: SelectedMetrics{
			Revenue:      AvailableValue(1_000_000),
			EBITDA:       AvailableValue(200_000),
			EBITDAMargin: AvailableValue(0.20),
		},
		RatioHealth: RatioHealthSummary{
			Available:       true,
			EBITDAMargin:    AvailableValue(0.20),
			NetDebtToEBITDA: AvailableValue(1.5),
		},
		Concentration: ConcentrationSummary{
			Available:          true,
			LargestEntityShare: AvailableValue(0.15),
			LargestShareTrend:  DirectionStable,
		},
		CashFlow: CashFlowSummary{
			Available:            true,
			EBITDAToFreeCashFlow: AvailableValue(0.70),
			ConversionTrend:      DirectionStable,
		},
		Valuation: ValuationSummary{
			Available:      true,
			IndicatedValue: AvailableValue(2_000_000),
		},
	}

	return BusinessSnapshot{
		ID:     id,
		Label:  "Stable Co",
		Period: "FY2024",
		Metrics: SelectedMetrics{
			Revenue:      AvailableValue(1_030_000),
			EBITDA:       AvailableValue(206_000),
			EBITDAMargin: AvailableValue(0.20),
		},
		QoE: QoESummary{
			Available:          true,
			FormulaVersion:     "1.0.0",
			EBITDAVolatility:   AvailableValue(0.05),
			AdjustmentToEBITDA: AvailableValue(0.10),
			HasCriticalFlags:   false,
			OpenFlagCount:      0,
		},
		RatioHealth: RatioHealthSummary{
			Available:       true,
			FormulaVersion:  "1.0.0",
			EBITDAMargin:    AvailableValue(0.20),
			NetDebtToEBITDA: AvailableValue(1.5),
			CurrentRatio:    AvailableValue(1.8),
		},
		Concentration: ConcentrationSummary{
			Available:          true,
			FormulaVersion:     "1.0.0",
			LargestEntityShare: AvailableValue(0.15),
			HHI:                AvailableValue(1200),
			LargestShareTrend:  DirectionStable,
		},
		CashFlow: CashFlowSummary{
			Available:            true,
			FormulaVersion:       "1.0.0",
			EBITDAToFreeCashFlow: AvailableValue(0.71),
			ConversionTrend:      DirectionStable,
			MonthsOfRunway:       Unavailable(),
		},
		Valuation: ValuationSummary{
			Available:       true,
			FormulaVersion:  "1.0.0",
			IndicatedValue:  AvailableValue(2_050_000),
			RangeLow:        AvailableValue(1_800_000),
			RangeHigh:       AvailableValue(2_300_000),
			MethodCount:     3,
			DispersionLevel: "HIGH_CONSENSUS",
		},
		SaleReadiness: SaleReadinessSummary{
			Available:       true,
			FormulaVersion:  "1.0.0",
			OverallScore:    AvailableValue(88),
			BlockerCount:    0,
			StrengthCount:   6,
			CoveragePercent: AvailableValue(1.0),
		},
		Prior: prior,
	}
}

// decliningBusiness returns a snapshot that should trigger most
// change-based findings: margin deterioration, revenue decline, cash
// conversion weakening, leverage increase, concentration increase, and
// valuation movement (a decline).
func decliningBusiness(id string) BusinessSnapshot {
	prior := &BusinessSnapshot{
		ID:     id,
		Period: "FY2023",
		Metrics: SelectedMetrics{
			Revenue:      AvailableValue(1_000_000),
			EBITDA:       AvailableValue(200_000),
			EBITDAMargin: AvailableValue(0.20),
		},
		RatioHealth: RatioHealthSummary{
			Available:       true,
			EBITDAMargin:    AvailableValue(0.20),
			NetDebtToEBITDA: AvailableValue(1.5),
		},
		Concentration: ConcentrationSummary{
			Available:          true,
			LargestEntityShare: AvailableValue(0.20),
		},
		CashFlow: CashFlowSummary{
			Available:            true,
			EBITDAToFreeCashFlow: AvailableValue(0.70),
		},
		Valuation: ValuationSummary{
			Available:      true,
			IndicatedValue: AvailableValue(2_000_000),
		},
	}

	return BusinessSnapshot{
		ID:     id,
		Label:  "Declining Co",
		Period: "FY2024",
		Metrics: SelectedMetrics{
			Revenue:      AvailableValue(850_000), // -15%
			EBITDA:       AvailableValue(102_000),
			EBITDAMargin: AvailableValue(0.12), // -40% vs 0.20
		},
		QoE: QoESummary{
			Available:          true,
			AdjustmentToEBITDA: AvailableValue(0.55), // above unresolved threshold
			HasCriticalFlags:   true,
			OpenFlagCount:      3,
		},
		RatioHealth: RatioHealthSummary{
			Available:               true,
			EBITDAMargin:            AvailableValue(0.12),
			NetDebtToEBITDA:         AvailableValue(2.4), // +60% vs 1.5
			HasRisingLeverageSignal: true,
			CriticalSignalCount:     1,
		},
		Concentration: ConcentrationSummary{
			Available:          true,
			LargestEntityShare: AvailableValue(0.32), // +60% vs 0.20
			LargestShareTrend:  DirectionDeteriorating,
		},
		CashFlow: CashFlowSummary{
			Available:                  true,
			EBITDAToFreeCashFlow:       AvailableValue(0.40), // -43% vs 0.70
			ConversionTrend:            DirectionDeteriorating,
			HasDecliningConversionFlag: true,
		},
		Valuation: ValuationSummary{
			Available:      true,
			IndicatedValue: AvailableValue(1_500_000), // -25%
		},
		SaleReadiness: SaleReadinessSummary{
			Available:    true,
			OverallScore: AvailableValue(45),
			BlockerCount: 2,
		},
		Prior: prior,
	}
}

// noPriorBusiness returns a snapshot with rich current-period summaries but
// no Prior, exercising every single-period-only finding
// (FindingUnresolvedFinancialQuality, FindingSaleReadinessOpportunity) plus
// the signal-only variants of the change-based findings.
func noPriorBusiness(id string) BusinessSnapshot {
	return BusinessSnapshot{
		ID:     id,
		Period: "FY2024",
		QoE: QoESummary{
			Available:          true,
			HasCriticalFlags:   true,
			AdjustmentToEBITDA: AvailableValue(0.6),
		},
		RatioHealth: RatioHealthSummary{
			Available:                   true,
			HasMarginCompressionSignal:  true,
			HasRisingLeverageSignal:     true,
			HasWeakeningLiquiditySignal: true,
			CriticalSignalCount:         2,
		},
		Concentration: ConcentrationSummary{
			Available:          true,
			LargestShareTrend:  DirectionDeteriorating,
			LargestEntityShare: AvailableValue(0.4),
		},
		CashFlow: CashFlowSummary{
			Available:                  true,
			ConversionTrend:            DirectionDeteriorating,
			HasDecliningConversionFlag: true,
		},
		SaleReadiness: SaleReadinessSummary{
			Available:    true,
			OverallScore: AvailableValue(40),
			BlockerCount: 1,
		},
	}
}

// emptyBusiness returns a snapshot with nothing available beyond ID/Period
// — every optional summary at its zero value — exercising the
// "missing modules" degradation path.
func emptyBusiness(id string) BusinessSnapshot {
	return BusinessSnapshot{ID: id, Period: "FY2024"}
}
