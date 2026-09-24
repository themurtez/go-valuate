package vendorspend_test

import (
	"github.com/themurtez/go-valuate/accounting/vendorspend"
	"github.com/themurtez/go-valuate/accounting/vendorspend/fixtures"
)

// fullInput returns a rich Input exercising most fields at once — the
// canonical input used by determinism/immutability/JSON round-trip
// tests, mirroring accounting/profitability's identical fullInput()
// convention.
func fullInput() vendorspend.Input {
	spend := append([]vendorspend.SpendRecord{}, fixtures.MultiSupplierSameProductSpend()...)
	spend = append(spend, fixtures.VendorCreditSpend()...)
	spend = append(spend, fixtures.DeclaredRecurringSpend()...)
	spend = append(spend, fixtures.CommitmentMixSpend()...)
	spend = append(spend, fixtures.TailSpendRecords()...)
	spend = append(spend, fixtures.DuplicateLikeSpend()...)

	suppliers := append([]vendorspend.Supplier{}, fixtures.MultiSupplierSameProductSuppliers()...)
	suppliers = append(suppliers, fixtures.VendorCreditSuppliers()...)
	suppliers = append(suppliers, fixtures.DeclaredRecurringSuppliers()...)
	suppliers = append(suppliers, fixtures.CommitmentMixSuppliers()...)
	suppliers = append(suppliers, fixtures.TailSpendSuppliers()...)
	suppliers = append(suppliers, fixtures.DuplicateLikeSuppliers()...)

	purchases := 500000.0
	return vendorspend.Input{
		Periods:      fixtures.SixMonthPeriods(),
		Suppliers:    suppliers,
		SpendRecords: spend,
		Controls:     vendorspend.ControlTotals{Purchases: &purchases},
		Policy: vendorspend.Policy{
			TailSpend: vendorspend.TailSpendPolicy{BelowAmount: 1000},
		},
	}
}

func fullOptions() vendorspend.Options {
	return vendorspend.Options{}
}
