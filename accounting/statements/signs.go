package statements

import "github.com/themurtez/go-valuate/accounting/ledger"

// canonicalAmount is the single centralized sign-normalization function
// every code path in this package uses to turn one account's raw ledger
// balance into a canonical financial.NormalizedItem-ready amount. No other
// function in this package may apply its own ad hoc sign flip — see the
// task's explicit "use a centralized sign-normalization function"
// instruction and types.go's SignTreatment doc comment for the convention
// this converts into (positive "as reported" magnitude, contra accounts
// included).
//
// rawBalance is debit-positive/credit-negative, exactly
// ledger.Balance.RawBalance's convention. acctType is the mapped
// account's own ledger.AccountType, used only to resolve SignNatural via
// ledger.NormalBalance — never used to override an explicit treatment.
func canonicalAmount(rawBalance float64, acctType ledger.AccountType, treatment SignTreatment) float64 {
	natural := naturalDisplay(rawBalance, acctType)

	switch resolvedSignTreatment(treatment) {
	case SignNormal:
		return rawBalance
	case SignInvert:
		return -natural
	default: // SignNatural
		return natural
	}
}

// naturalDisplay converts rawBalance into ledger's own DisplayBalance
// convention for acctType: positive when the account is in its normal
// position (per ledger.NormalBalance), sign-flipped otherwise. Mirrors
// ledger's unexported displayBalance exactly (same formula, computed
// independently here since this package intentionally does not reach into
// ledger's unexported internals) — see ledger.Balance.DisplayBalance's
// doc comment for why this specific flip is correct for every
// non-contra account: a REVENUE/LIABILITY/EQUITY account naturally
// carries a CREDIT (raw-negative) balance, and flipping it makes it read
// as the positive magnitude financial.Normalize's downstream consumers
// expect (see financial/metrics/income_statement.go's sign-convention
// note).
func naturalDisplay(rawBalance float64, acctType ledger.AccountType) float64 {
	normal, ok := ledger.NormalBalance(acctType)
	if !ok || normal == ledger.Debit {
		return rawBalance
	}
	return -rawBalance
}
