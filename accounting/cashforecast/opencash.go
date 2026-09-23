package cashforecast

import "time"

// OpeningCash is the required starting cash position the forecast rolls
// forward from. Every WeeklyForecast's Week 1 OpeningCash is derived from
// this value (plus CashAccounts, when supplied — see
// resolveOpeningPosition) — opening cash is authoritative, and this
// package never derives it from CashFlowEvent history (see Options'
// "events before forecast start" handling in calculate.go).
type OpeningCash struct {
	// Amount is total cash on hand as of AsOfDate. Used directly only when
	// CashAccounts is empty; when CashAccounts is supplied, Amount is
	// expected to reconcile with the sum of CashAccounts' balances (a
	// mismatch is flagged, not silently overridden — see
	// IssueOpeningCashMismatch) and CashAccounts drives the
	// restricted/unrestricted split.
	Amount float64 `json:"amount"`
	// Currency is this figure's ISO 4217-style currency code. Required;
	// must equal Options.ReportingCurrency when both are set.
	Currency string `json:"currency"`
	// AsOfDate is when Amount was true. Required; used only for staleness
	// reporting (see Coverage) — never for calculation, since every
	// forecast week is measured from Input.ForecastStartDate regardless of
	// how recent AsOfDate is.
	AsOfDate time.Time `json:"as_of_date"`
	// SourceRef is an opaque pointer back to the originating bank/GL
	// record.
	SourceRef string `json:"source_ref,omitempty"`
}

// CashAccount is one optional named cash account/bank balance making up
// OpeningCash, used when a caller wants unrestricted, restricted, and
// total cash reported distinctly rather than as a single blended figure —
// see the task's "do not silently treat restricted cash as available
// liquidity" instruction.
type CashAccount struct {
	AccountID string  `json:"account_id"`
	Name      string  `json:"name,omitempty"`
	Balance   float64 `json:"balance"`
	Currency  string  `json:"currency"`
	// Restricted marks a balance not available for general use (e.g. a
	// payroll trust account, a customer deposit holdback, a loan reserve
	// account). Restricted balances are excluded from
	// OpeningPosition.UnrestrictedCash and from the cash balance every
	// weekly funding-gap/minimum-cash comparison uses, but are still
	// visible in OpeningPosition.TotalCash and OpeningPosition.RestrictedCash.
	Restricted bool `json:"restricted,omitempty"`
	// MinimumReserve is an optional per-account reserve floor, folded into
	// the overall minimum-cash policy when Options.MinimumCashBalance is
	// unset — see resolvedMinimumCashPolicy.
	MinimumReserve float64 `json:"minimum_reserve,omitempty"`
}

// OpeningPosition is the resolved starting-cash figures this package
// actually uses, computed once from OpeningCash and CashAccounts.
type OpeningPosition struct {
	// TotalCash is every CashAccount balance summed (or OpeningCash.Amount
	// when CashAccounts is empty).
	TotalCash float64 `json:"total_cash"`
	// UnrestrictedCash is TotalCash minus every Restricted account's
	// balance. This is the figure every weekly cash-balance rollforward
	// and minimum-cash/funding-gap comparison is measured against.
	UnrestrictedCash float64 `json:"unrestricted_cash"`
	// RestrictedCash is the sum of every Restricted account's balance.
	RestrictedCash float64 `json:"restricted_cash"`
	// HasAccountBreakdown is true when CashAccounts was supplied (so
	// UnrestrictedCash/RestrictedCash reflect a real restricted/
	// unrestricted split), false when only OpeningCash.Amount was supplied
	// (in which case UnrestrictedCash == TotalCash and RestrictedCash ==
	// 0 — no restriction information was available, not "confirmed
	// unrestricted").
	HasAccountBreakdown bool `json:"has_account_breakdown"`
	// AccountReserveTotal is the sum of every CashAccount.MinimumReserve,
	// used as a fallback minimum-cash policy when Options.MinimumCashBalance
	// is unset — see resolvedMinimumCashPolicy.
	AccountReserveTotal float64 `json:"account_reserve_total,omitempty"`
}

// resolveOpeningPosition computes OpeningPosition from opening and
// accounts. accounts takes precedence for the restricted/unrestricted
// split when non-empty; opening.Amount is always what TotalCash falls back
// to when accounts is empty.
//
// An account whose Currency is set and differs from reportingCurrency is
// excluded from every sum here, mirroring validateOpeningCash's identical
// exclusion rule for its IssueMixedCurrency/IssueOpeningCashMismatch
// checks — those two functions must agree on which accounts count,
// otherwise a currency-mismatched balance could be flagged as suspect by
// validation while still being silently included in the actual cash
// figure every weekly balance rolls forward from.
func resolveOpeningPosition(opening OpeningCash, accounts []CashAccount, reportingCurrency string) OpeningPosition {
	if len(accounts) == 0 {
		return OpeningPosition{
			TotalCash:        opening.Amount,
			UnrestrictedCash: opening.Amount,
		}
	}
	var total, unrestricted, restricted, reserve float64
	for _, a := range accounts {
		if reportingCurrency != "" && a.Currency != "" && a.Currency != reportingCurrency {
			continue
		}
		total += a.Balance
		reserve += a.MinimumReserve
		if a.Restricted {
			restricted += a.Balance
		} else {
			unrestricted += a.Balance
		}
	}
	return OpeningPosition{
		TotalCash:           total,
		UnrestrictedCash:    unrestricted,
		RestrictedCash:      restricted,
		HasAccountBreakdown: true,
		AccountReserveTotal: reserve,
	}
}
