package synthetic

import "github.com/themurtez/go-valuate/analytics/debt"

// meridian2025EBITDA is Meridian SaaS's approximate 2025 EBITDA (revenue
// minus COGS and opex, before depreciation/amortization), used as the
// evaluation base for BuildDebtInput/BuildAcquisitionInput/
// BuildForecastAssumptions. Computed once here so every fixture file that
// needs "2025 EBITDA" uses the identical figure rather than each
// recomputing it slightly differently.
const meridian2025EBITDA = 2_854_600

// BuildDebtInput returns analytics/debt.Input for Meridian SaaS's existing
// term loan, sized to match BuildDataset's 2025 BS_LONG_TERM_DEBT +
// BS_SHORT_TERM_DEBT ending balance ($940,000 + $110,000 = $1,050,000), a
// LenderPolicy with realistic covenant-style thresholds, and one downside
// scenario.
func BuildDebtInput() debt.Input {
	return debt.Input{
		EBITDA: debt.AvailableValue(meridian2025EBITDA),
		ExistingDebt: []debt.LoanTerms{
			{
				Label:              "Term loan — expansion capital (2022)",
				Principal:          1_050_000,
				AnnualInterestRate: 0.075,
				AmortizationYears:  7,
				Frequency:          debt.FrequencyMonthly,
			},
		},
		// Matches BuildDataset's actual 2025 BS_CASH exactly (not an
		// independently chosen figure), so this package's NetDebtToEBITDA
		// and financial/metrics.Snapshot.NetDebt agree when fed the
		// dataset's real total-debt/cash figures — see
		// analytics/smoketest's net-debt cross-module invariant test.
		CashAndEquivalents: debt.AvailableValue(7_223_656),
		Policy: debt.LenderPolicy{
			MinimumDSCR:            1.25,
			MaximumDebtToEBITDA:    3.0,
			MaximumNetDebtToEBITDA: 2.5,
		},
		DownsideScenarios: []debt.DownsideScenario{
			{
				Label:                  "Revenue growth stalls",
				EBITDAHaircutPercent:   0.30,
				CashFlowHaircutPercent: 0.30,
			},
		},
	}
}
