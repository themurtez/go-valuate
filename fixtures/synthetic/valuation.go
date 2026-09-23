package synthetic

import (
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/valuation/applicability"
	"github.com/themurtez/go-valuate/valuation/capitalization"
	"github.com/themurtez/go-valuate/valuation/dcf"
	"github.com/themurtez/go-valuate/valuation/ebitda"
	"github.com/themurtez/go-valuate/valuation/netassets"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
	"github.com/themurtez/go-valuate/valuation/profile"
	"github.com/themurtez/go-valuate/valuation/sde"
)

// BuildProfile returns a valuation/profile.Profile describing Meridian
// SaaS for applicability scoring: not owner-operated, $8.7M revenue,
// ~45 employees, low asset intensity, high recurring-revenue share, and
// strong recent growth — the shape that should favor DCF/EBITDA-multiple
// methods over SDE.
func BuildProfile() profile.Profile {
	ownerOperated := false
	revenue := 8_724_000.0
	employees := 45
	assetIntensity := 0.09
	recurringPercent := 0.987 // 8,610,000 / 8,724,000
	growthRate := 0.28
	return profile.Profile{
		Industry:                profile.IndustryTechnologySaaS,
		OwnerOperated:           &ownerOperated,
		AnnualRevenue:           &revenue,
		EmployeeCount:           &employees,
		AssetIntensity:          &assetIntensity,
		RecurringRevenuePercent: &recurringPercent,
		HistoricalGrowthRate:    &growthRate,
		Profitability:           profile.ProfitabilityStrong,
		DataAvailability: profile.DataAvailability{
			HasMultiYearFinancials: true,
			HasBalanceSheet:        true,
			HasForecast:            true,
		},
	}
}

// meridianMaintainableEBITDA and meridianMaintainableSDE run
// analytics/qoe.Calculate once over BuildQoEInput and cache nothing (a
// fresh call every time, per this package's no-shared-state doc comment)
// to derive the maintainable earnings figures BuildOrchestratorRequest
// feeds into every valuation method — so the valuation stage is genuinely
// connected to the QoE stage rather than using independently hand-picked
// numbers.
func meridianMaintainableEBITDA() float64 {
	res := qoe.Calculate(BuildQoEInput(), qoe.Options{})
	return res.MaintainableEBITDA.Value
}

// BuildOrchestratorRequest returns valuation/orchestrator.Request for
// Meridian SaaS, running all 5 valuation methods: SDE multiple, EBITDA
// multiple, capitalization of earnings, a 3-year DCF (reusing
// BuildForecastInput's Base Case free cash flow figures), and adjusted net
// asset value (from BuildDataset's 2025 balance sheet). Applicability is
// always populated (never nil), matching this repository's established
// convention (see examples/full_flow and valuation/e2e's worked examples).
func BuildOrchestratorRequest() orchestrator.Request {
	maintainableEBITDA := meridianMaintainableEBITDA()
	// SDE is not a natural fit for a non-owner-operated SaaS business, but
	// this package computes it anyway (as a caller legitimately might, to
	// let applicability scoring demonstrate why SDE ranks lower) using the
	// same maintainable-earnings figure as a stand-in.
	maintainableSDE := maintainableEBITDA

	applicabilityResults := applicability.Calculate(BuildProfile())

	return orchestrator.Request{
		Applicability: &applicabilityResults,
		SDE: &sde.Input{
			MaintainableSDE: maintainableSDE,
			Multiple:        4.5,
		},
		EBITDA: &ebitda.Input{
			MaintainableEBITDA: maintainableEBITDA,
			Multiple:           6.5,
			EquityBridge: ebitda.EquityBridgeInput{
				Requested:     true,
				ExcessCash:    2_940_000,
				ShortTermDebt: 110_000,
				LongTermDebt:  940_000,
			},
		},
		Capitalization: &capitalization.Input{
			MaintainableEarnings: maintainableEBITDA,
			CapitalizationRate:   0.14,
		},
		DCF: &dcf.Input{
			ForecastPeriods: []dcf.ForecastPeriod{
				{Period: "2026", FreeCashFlow: 1_950_000},
				{Period: "2027", FreeCashFlow: 2_480_000},
				{Period: "2028", FreeCashFlow: 3_020_000},
			},
			DiscountRate:       0.16,
			TerminalGrowthRate: 0.04,
			EquityBridge: dcf.EquityBridgeInput{
				Requested:     true,
				ExcessCash:    2_940_000,
				ShortTermDebt: 110_000,
				LongTermDebt:  940_000,
			},
		},
		NetAssets: &netassets.Input{
			Assets: []netassets.AssetItem{
				{Label: "Cash", Amount: 7_223_656, SourceCode: "BS_CASH"},
				{Label: "Accounts Receivable", Amount: 672_000, SourceCode: "BS_ACCOUNTS_RECEIVABLE"},
				{Label: "Prepaid Expenses", Amount: 64_000, SourceCode: "BS_PREPAID"},
				{Label: "Fixed Assets, net", Amount: 129_000, SourceCode: "BS_FIXED_ASSETS", Notes: "$361,000 gross less $232,000 accumulated depreciation"},
				{Label: "Intangible Assets", Amount: 420_000, SourceCode: "BS_INTANGIBLE_ASSETS"},
			},
			Liabilities: []netassets.LiabilityItem{
				{Label: "Accounts Payable", Amount: 261_000, SourceCode: "BS_ACCOUNTS_PAYABLE"},
				{Label: "Other Current Liabilities", Amount: 96_000, SourceCode: "BS_CURRENT_LIABILITY_OTHER"},
				{Label: "Short-Term Debt", Amount: 110_000, SourceCode: "BS_SHORT_TERM_DEBT"},
				{Label: "Long-Term Debt", Amount: 940_000, SourceCode: "BS_LONG_TERM_DEBT"},
			},
		},
	}
}
