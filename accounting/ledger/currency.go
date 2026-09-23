package ledger

// resolveAccountCurrency returns acct.Currency if set, otherwise
// fallbackBase — the shared "account currency, or report base currency"
// resolution rule used by rollups and multi-account aggregation. This
// package never fetches or infers an FX rate — see the package doc
// comment's multi-currency section; a caller wanting cross-currency
// aggregation must supply already-converted values or explicit rates
// itself, upstream of this package.
func resolveAccountCurrency(acct Account, fallbackBase string) string {
	if acct.Currency != "" {
		return acct.Currency
	}
	return fallbackBase
}
