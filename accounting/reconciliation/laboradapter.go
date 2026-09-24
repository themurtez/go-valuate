package reconciliation

import "github.com/themurtez/go-valuate/accounting/labor"

// PayrollClearingBookBalance builds a BalanceInput for the book
// (payroll-register) side of a payroll-clearing reconciliation from a
// caller-supplied control.TotalLaborCost — task section 37: "Support
// payroll clearing/control reconciliation from supplied labor control
// totals. Do not calculate payroll." This adapter performs no
// computation of its own; it only republishes an already-computed
// labor.GLPayrollControl value (typically produced by
// labor.GLControlFromLedgerBalances or a caller's own payroll-register
// total) into this package's BalanceInput shape.
func PayrollClearingBookBalance(control labor.GLPayrollControl, asOfDate string) BalanceInput {
	if !control.TotalLaborCost.Available {
		return BalanceInput{}
	}
	bal := control.TotalLaborCost.Amount
	return BalanceInput{EndingBalance: &bal, EndingBalanceDate: asOfDate}
}

// PayrollClearingExternalBalance builds a BalanceInput for the external
// (GL clearing-account) side of a payroll-clearing reconciliation from a
// caller-supplied GL clearing-account balance. Kept as a thin, explicit
// wrapper (rather than requiring the caller to build BalanceInput by
// hand) purely for symmetry with the book-side helper above.
func PayrollClearingExternalBalance(glClearingBalance float64, asOfDate string) BalanceInput {
	bal := glClearingBalance
	return BalanceInput{EndingBalance: &bal, EndingBalanceDate: asOfDate}
}
