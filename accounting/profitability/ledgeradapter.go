package profitability

import "github.com/themurtez/go-valuate/accounting/ledger"

// LedgerAccountMapping is one caller-declared explicit mapping from a
// ledger.Account.ID to a profitability Component and dimension
// attribution — task section 42. This package never infers a Component
// or Attribution from an account's Name/AccountType; a caller supplies
// this mapping explicitly, exactly as
// accounting/labor.LedgerAccountMapping/accounting/statements.AccountMapping
// require explicit account-level mapping rather than name-based
// inference.
type LedgerAccountMapping struct {
	AccountID    string        `json:"account_id"`
	Component    Component     `json:"component"`
	Attributions []Attribution `json:"attributions,omitempty"`
}

// FactsFromLedgerBalances converts a set of ledger.Balance rows into
// Facts using the caller-supplied mapping — task section 42. A
// ledger.Balance whose AccountID has no entry in mapping is skipped
// entirely (never guessed); this adapter has no compile-time role in
// Calculate and a caller with no accounting/ledger usage never needs it.
// Facts are given the period's activity magnitude
// (abs(PeriodDebits - PeriodCredits), i.e. Balance.Movement's absolute
// value — this package's Amount semantics are non-negative magnitudes,
// see Fact's doc comment); the caller's mapped Component determines the
// resulting economic effect, so mapping.Component must already reflect
// the correct sign convention for that account (e.g. a contra-revenue
// account mapped to ComponentReturn/ComponentDiscount, not
// ComponentGrossRevenue).
func FactsFromLedgerBalances(balances []ledger.Balance, mapping []LedgerAccountMapping, period string) []Fact {
	byAccount := map[string]LedgerAccountMapping{}
	for _, m := range mapping {
		if _, exists := byAccount[m.AccountID]; !exists {
			byAccount[m.AccountID] = m
		}
	}

	var facts []Fact
	for _, b := range balances {
		m, ok := byAccount[b.AccountID]
		if !ok {
			continue
		}
		amount := b.Movement
		if amount < 0 {
			amount = -amount
		}
		facts = append(facts, Fact{
			FactID:       "LEDGER-" + b.AccountID + "-" + period,
			Period:       period,
			Component:    m.Component,
			Amount:       amount,
			Attributions: m.Attributions,
			SourceType:   "accounting/ledger.Balance",
			SourceID:     b.AccountID,
		})
	}
	return facts
}
