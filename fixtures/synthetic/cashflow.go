package synthetic

import (
	"github.com/themurtez/go-valuate/analytics/cashflow"
	"github.com/themurtez/go-valuate/financial"
)

// BuildCashFlowInput returns analytics/cashflow.Input for Meridian SaaS's
// full 5-year history, with caller-supplied operating cash flow (a modest
// haircut off reported EBITDA, reflecting normal working-capital drag for
// a growing company), no capex/distributions/tax figures (left
// unavailable — this package derives what it can from Dataset alone
// otherwise), and 2025 debt service matching BuildDebtInput's term loan
// (its actual year-1 principal/interest split: see debt.go's doc
// comment).
func BuildCashFlowInput() cashflow.Input {
	return cashflow.Input{
		Dataset:    BuildDataset(),
		PeriodMeta: BuildPeriodMeta(),
		OperatingCashFlow: map[financial.Period]cashflow.CashFlowValue{
			"2021": cashflow.Reported(560_000),
			"2022": cashflow.Reported(870_000),
			"2023": cashflow.Reported(760_000), // dips with the 2023 margin compression
			"2024": cashflow.Reported(1_540_000),
			"2025": cashflow.Reported(2_460_000),
		},
		DebtService: map[financial.Period]cashflow.DebtServiceFigure{
			"2025": {
				Principal: cashflow.Reported(118_532),
				Interest:  cashflow.Reported(74_730),
			},
		},
	}
}
