package synthetic

import (
	"github.com/themurtez/go-valuate/analytics/debt"
	"github.com/themurtez/go-valuate/transactions/acquisition"
)

// BuildAcquisitionInput returns transactions/acquisition.Input for a buyer
// screening Meridian SaaS as an acquisition target at a $19.5M asking
// price (~6.8x 2025 EBITDA, a plausible lower-middle-market SaaS
// multiple), financed with a $12M senior term loan plus $7.5M buyer cash,
// against a consensus valuation of $18M (so the asking price carries a
// modest ~8.3% premium to consensus, not a red flag under the thresholds
// below).
func BuildAcquisitionInput() acquisition.Input {
	return acquisition.Input{
		Target: acquisition.TargetFinancials{
			Revenue:          acquisition.AvailableValue(8_724_000),
			NormalizedEBITDA: acquisition.AvailableValue(meridian2025EBITDA),
		},
		Consensus: acquisition.ConsensusValuation{
			Value: acquisition.AvailableValue(18_000_000),
			Basis: "enterprise_value",
		},
		AskingPrice: acquisition.AvailableValue(19_500_000),
		Fees: acquisition.TransactionFees{
			LegalAndAdvisory: acquisition.AvailableValue(85_000),
			DueDiligence:     acquisition.AvailableValue(45_000),
			Other:            acquisition.AvailableValue(60_000),
		},
		Financing: acquisition.Financing{
			DebtTranches: []debt.LoanTerms{
				{
					Label:              "Senior acquisition term loan",
					Principal:          12_000_000,
					AnnualInterestRate: 0.085,
					AmortizationYears:  10,
					Frequency:          debt.FrequencyMonthly,
				},
			},
			BuyerCashContribution: acquisition.AvailableValue(7_500_000),
		},
		WorkingCapital: acquisition.WorkingCapitalRequirement{
			Amount: acquisition.AvailableValue(450_000),
		},
		Capex: acquisition.CapexAssumption{
			AnnualAmount: acquisition.AvailableValue(220_000),
		},
		RedFlags: acquisition.RedFlagThresholds{
			MinimumDSCR:                      1.25,
			MaximumPriceToEBITDA:             8.0,
			MaximumPremiumToConsensusPercent: 0.25,
			MinimumCashOnCashReturn:          0.12,
			MaximumPaybackYears:              8,
			MaximumDebtToEBITDA:              4.0,
		},
	}
}
