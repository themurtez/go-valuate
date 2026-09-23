package synthetic

import "github.com/themurtez/go-valuate/transactions/dealstructure"

// BuildDealStructureInput returns transactions/dealstructure.Input for the
// same $19.5M Meridian SaaS acquisition as BuildAcquisitionInput, but
// structured with dealstructure's richer seller-note and earnout
// modeling: a $12M senior term loan (identical terms to
// BuildAcquisitionInput's tranche), a $1.5M seller note, a 2-year
// deterministic earnout ($750K + $500K), and $6M buyer equity — a fully
// funded sources-and-uses.
func BuildDealStructureInput() dealstructure.Input {
	return dealstructure.Input{
		PurchasePrice: dealstructure.AvailableValue(19_500_000),
		BuyerEquity:   dealstructure.AvailableValue(6_000_000),
		DebtTranches: []dealstructure.DebtTranche{
			{
				Label:              "Senior acquisition term loan",
				Amount:             12_000_000,
				AnnualInterestRate: 0.085,
				AmortizationYears:  10,
			},
		},
		SellerNote: dealstructure.SellerNote{
			Included: true,
			Terms: dealstructure.DebtTranche{
				Label:              "Seller note",
				Amount:             1_500_000,
				AnnualInterestRate: 0.06,
				AmortizationYears:  5,
			},
		},
		Earnout: dealstructure.Earnout{
			Included: true,
			Payments: []dealstructure.EarnoutPayment{
				{PeriodNumber: 1, Amount: 750_000, Label: "Year 1 earnout — revenue retention milestone"},
				{PeriodNumber: 2, Amount: 500_000, Label: "Year 2 earnout — revenue growth milestone"},
			},
		},
		Fees: dealstructure.TransactionFees{
			LegalAndAdvisory: dealstructure.AvailableValue(85_000),
			DueDiligence:     dealstructure.AvailableValue(45_000),
			FinancingFees:    dealstructure.AvailableValue(30_000),
			Other:            dealstructure.AvailableValue(30_000),
		},
		WorkingCapital: dealstructure.WorkingCapitalContribution{
			Amount: dealstructure.AvailableValue(450_000),
		},
	}
}
