package labor

// LedgerAccountBalance is a minimal, portable shape sufficient to
// aggregate caller-identified labor-related GL account balances into a
// GLPayrollControl, without a compile-time dependency on
// accounting/ledger — see the task's section 30 "caller supplies account
// IDs/category mapping — do not infer payroll accounts from account
// names" instruction. A caller with an accounting/ledger.TrialBalance
// maps its own balances into this shape.
type LedgerAccountBalance struct {
	AccountID string  `json:"account_id"`
	Balance   float64 `json:"balance"`
}

// LedgerAccountCategory is the caller-declared payroll-related category
// one GL account belongs to, for GLControlFromLedgerBalances. Never
// inferred from account name/number.
type LedgerAccountCategory string

const (
	LedgerCategoryGrossWages      LedgerAccountCategory = "GROSS_WAGES"
	LedgerCategoryEmployerTaxes   LedgerAccountCategory = "EMPLOYER_TAXES"
	LedgerCategoryBenefits        LedgerAccountCategory = "BENEFITS"
	LedgerCategoryContractorLabor LedgerAccountCategory = "CONTRACTOR_LABOR"
	LedgerCategoryOtherLabor      LedgerAccountCategory = "OTHER_LABOR"
)

// LedgerAccountMapping declares one GL AccountID's payroll category —
// entirely caller-declared.
type LedgerAccountMapping struct {
	AccountID string                `json:"account_id"`
	Category  LedgerAccountCategory `json:"category"`
}

// GLControlFromLedgerBalances aggregates balances (keyed by AccountID)
// into a GLPayrollControl for period, using only the caller-supplied
// mapping — an account with no mapping entry contributes to no category
// and is silently excluded (not summed into any total), matching the
// task's "do not infer payroll accounts from account names" instruction.
// TotalLaborCost is the sum of every categorized account's balance,
// regardless of which category it fell into.
func GLControlFromLedgerBalances(period string, balances []LedgerAccountBalance, mapping []LedgerAccountMapping) GLPayrollControl {
	categoryByAccount := make(map[string]LedgerAccountCategory, len(mapping))
	for _, m := range mapping {
		if _, exists := categoryByAccount[m.AccountID]; !exists {
			categoryByAccount[m.AccountID] = m.Category
		}
	}

	var grossWages, employerTaxes, benefits, contractorLabor, otherLabor float64
	var haveGross, haveTaxes, haveBenefits, haveContractor, haveOther bool
	var total float64
	var haveAny bool

	for _, b := range balances {
		cat, ok := categoryByAccount[b.AccountID]
		if !ok {
			continue
		}
		haveAny = true
		total += b.Balance
		switch cat {
		case LedgerCategoryGrossWages:
			grossWages += b.Balance
			haveGross = true
		case LedgerCategoryEmployerTaxes:
			employerTaxes += b.Balance
			haveTaxes = true
		case LedgerCategoryBenefits:
			benefits += b.Balance
			haveBenefits = true
		case LedgerCategoryContractorLabor:
			contractorLabor += b.Balance
			haveContractor = true
		case LedgerCategoryOtherLabor:
			otherLabor += b.Balance
			haveOther = true
		}
	}

	control := GLPayrollControl{Period: period}
	if haveGross {
		control.GrossWages = AvailableValue(grossWages)
	}
	if haveTaxes {
		control.EmployerTaxes = AvailableValue(employerTaxes)
	}
	if haveBenefits {
		control.Benefits = AvailableValue(benefits)
	}
	if haveContractor {
		control.ContractorLabor = AvailableValue(contractorLabor)
	}
	if haveOther {
		control.OtherLabor = AvailableValue(otherLabor)
	}
	if haveAny {
		control.TotalLaborCost = AvailableValue(total)
	}
	return control
}
