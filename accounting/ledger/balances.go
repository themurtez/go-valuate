package ledger

import "sort"

// OpeningBalance is an optional, caller-supplied starting balance for an
// account, used when a caller does not have (or does not want to replay)
// the journal history before a given range — see the package doc
// comment's opening-balances section. This package never fabricates a
// journal entry to represent one; an OpeningBalance is applied directly as
// the starting point for balance calculation.
type OpeningBalance struct {
	// AccountID is the Account.ID this opening balance applies to. Required.
	AccountID string `json:"account_id"`
	// Debit is the opening debit-side amount. Must be >= 0.
	Debit float64 `json:"debit"`
	// Credit is the opening credit-side amount. Must be >= 0.
	Credit float64 `json:"credit"`
	// Currency is the ISO 4217 currency this opening balance is denominated
	// in. Optional; when empty, assumed to match the account's own
	// Currency (or the report's base currency if the account has none) —
	// see currency.go.
	Currency string `json:"currency,omitempty"`
	// SourceRef is an opaque caller-defined pointer to where this opening
	// balance came from (e.g. a prior period's closing trial balance row
	// ID). Never interpreted by this package.
	SourceRef string `json:"source_ref,omitempty"`
}

// validateOpeningBalances checks each OpeningBalance for a known account
// and finite, non-negative Debit/Credit. Invalid entries are excluded from
// balance calculation (their contribution to a Balance is zero), never
// silently included.
func validateOpeningBalances(openings []OpeningBalance, chart ChartOfAccounts) []Issue {
	var issues []Issue
	for _, o := range openings {
		if _, ok := chart.Lookup(o.AccountID); !ok {
			issues = append(issues, Issue{
				Code:     IssueUnknownAccount,
				Severity: SeverityError,
				Message:  "opening balance references unknown account: " + o.AccountID,
				Account:  o.AccountID,
			})
			continue
		}
		if isNonFinite(o.Debit) || isNonFinite(o.Credit) {
			issues = append(issues, Issue{
				Code:     IssueNonFiniteAmount,
				Severity: SeverityError,
				Message:  "opening balance for account " + o.AccountID + " has a non-finite amount",
				Account:  o.AccountID,
			})
			continue
		}
		if o.Debit < 0 || o.Credit < 0 {
			issues = append(issues, Issue{
				Code:     IssueNegativeAmount,
				Severity: SeverityError,
				Message:  "opening balance for account " + o.AccountID + " has a negative debit or credit",
				Account:  o.AccountID,
			})
			continue
		}
		if o.Debit != 0 && o.Credit != 0 {
			issues = append(issues, Issue{
				Code:     IssueInvalidOpeningBalance,
				Severity: SeverityError,
				Message:  "opening balance for account " + o.AccountID + " has both debit and credit set",
				Account:  o.AccountID,
			})
		}
	}
	return issues
}

// validOpeningsByAccount returns a map of AccountID -> the net raw
// (debit-positive, credit-negative) opening amount, summing multiple
// OpeningBalance entries for the same account, but skipping any entry that
// validateOpeningBalances would flag (unknown account, non-finite,
// negative). Used internally by balance calculation.
func validOpeningsByAccount(openings []OpeningBalance, chart ChartOfAccounts) map[string]float64 {
	out := make(map[string]float64, len(openings))
	for _, o := range openings {
		if _, ok := chart.Lookup(o.AccountID); !ok {
			continue
		}
		if isNonFinite(o.Debit) || isNonFinite(o.Credit) {
			continue
		}
		if o.Debit < 0 || o.Credit < 0 {
			continue
		}
		out[o.AccountID] += o.Debit - o.Credit
	}
	return out
}

// Balance is the calculated activity for one account over a selected
// period/range: opening position, period movement, and closing position.
// All Debit/Credit/Net fields follow this package's canonical raw
// convention (debit-positive, credit-negative) — see the package doc
// comment. DisplayBalance additionally flips sign for natural-credit
// account types so it reads positive in the account's normal position.
type Balance struct {
	// AccountID is the Account.ID this Balance is for.
	AccountID string `json:"account_id"`
	// AccountType echoes the account's Type, for convenient display without
	// a second chart lookup.
	AccountType AccountType `json:"account_type"`
	// NormalBalance echoes NormalBalance(AccountType) — DEBIT or CREDIT —
	// or "" if AccountType was unrecognized.
	NormalBalance DebitCredit `json:"normal_balance,omitempty"`

	// OpeningDebit and OpeningCredit are the resolved opening-side amounts
	// (from an OpeningBalance, or 0 if none supplied and no prior-period
	// entries were included — see Calculate*Balances' doc comments for
	// exactly how opening is resolved).
	OpeningDebit  float64 `json:"opening_debit"`
	OpeningCredit float64 `json:"opening_credit"`
	// OpeningNet is OpeningDebit - OpeningCredit (raw convention).
	OpeningNet float64 `json:"opening_net"`

	// PeriodDebits and PeriodCredits are the sums of Debit/Credit across
	// every included JournalLine posted to this account within the
	// selected range.
	PeriodDebits  float64 `json:"period_debits"`
	PeriodCredits float64 `json:"period_credits"`
	// Movement is PeriodDebits - PeriodCredits (raw convention) — the net
	// change during the period, independent of OpeningNet.
	Movement float64 `json:"movement"`

	// RawBalance is OpeningNet + Movement — the canonical debit-positive,
	// credit-negative closing balance.
	RawBalance float64 `json:"raw_balance"`
	// DisplayBalance is RawBalance, sign-flipped for natural-credit account
	// types (LIABILITY, EQUITY, REVENUE) so a healthy balance in an
	// account's normal position always displays as positive. Equal to
	// RawBalance for ASSET/EXPENSE accounts.
	DisplayBalance float64 `json:"display_balance"`

	// SourceEntryCount and SourceLineCount are the number of distinct
	// JournalEntry/JournalLine values that contributed to PeriodDebits/
	// PeriodCredits, for provenance/sanity-check display.
	SourceEntryCount int `json:"source_entry_count"`
	SourceLineCount  int `json:"source_line_count"`

	// Currency is the account's resolved currency (Account.Currency, or
	// empty if the account did not declare one).
	Currency string `json:"currency,omitempty"`
}

// BalanceOptions configures CalculateBalances.
type BalanceOptions struct {
	// Range selects which entries count as "period" activity — see
	// PeriodRange. The zero value (empty PeriodRange) includes every
	// supplied entry as period activity with no separate opening window.
	Range PeriodRange
	// Openings supplies explicit starting balances per account, applied as
	// OpeningDebit/OpeningCredit directly rather than derived by replaying
	// entries before Range — see the package doc comment's opening-
	// balances section. If an account has no matching OpeningBalance,
	// its opening is computed instead from any supplied entries whose Date
	// is strictly before Range.StartDate (when Range.StartDate is set);
	// if Range.StartDate is empty, an account with no OpeningBalance simply
	// starts at zero.
	Openings []OpeningBalance
	// IncludeStatuses restricts which EntryStatus values are included. The
	// zero value (nil) defaults to PostedStatuses() (POSTED and REVERSED).
	// VOIDED is never included, even if explicitly listed here, since a
	// voided entry never affects balances by design — see
	// JournalEntry.Status's doc comment.
	IncludeStatuses []EntryStatus
}

func (o BalanceOptions) includeStatuses() map[EntryStatus]bool {
	statuses := o.IncludeStatuses
	if len(statuses) == 0 {
		statuses = PostedStatuses()
	}
	out := make(map[EntryStatus]bool, len(statuses))
	for _, s := range statuses {
		if s != StatusVoided {
			out[s] = true
		}
	}
	return out
}

// CalculateBalances computes one Balance per account in chart, from
// entries, per opts. It does not mutate chart, entries, or opts.Openings.
// Accounts are returned sorted by AccountID for determinism.
func CalculateBalances(chart ChartOfAccounts, entries []JournalEntry, opts BalanceOptions) []Balance {
	include := opts.includeStatuses()
	openingByAccount := validOpeningsByAccount(opts.Openings, chart)
	explicitOpening := make(map[string]bool, len(opts.Openings))
	for _, o := range opts.Openings {
		if _, ok := chart.Lookup(o.AccountID); ok {
			explicitOpening[o.AccountID] = true
		}
	}

	type accum struct {
		openingDebit, openingCredit float64
		periodDebit, periodCredit   float64
		entrySet                    map[string]bool
		lineCount                   int
	}
	accums := make(map[string]*accum, chart.Len())
	get := func(id string) *accum {
		a, ok := accums[id]
		if !ok {
			a = &accum{entrySet: make(map[string]bool)}
			accums[id] = a
		}
		return a
	}

	// Deterministic entry order: iterate entries in caller-supplied order
	// (already deterministic — a slice) and accumulate directly; float sums
	// are order-sensitive, so this fixed input order (not a map) is what
	// makes the result reproducible.
	for _, e := range entries {
		if !include[e.EffectiveStatus()] {
			continue
		}
		inRange := opts.Range.Matches(e)
		isOpeningEntry := !inRange && opts.Range.before(e)
		if !inRange && !isOpeningEntry {
			continue
		}
		for _, l := range e.Lines {
			if isNonFinite(l.Debit) || isNonFinite(l.Credit) {
				continue
			}
			if l.Debit < 0 || l.Credit < 0 {
				continue
			}
			if _, ok := chart.Lookup(l.AccountID); !ok {
				continue
			}
			a := get(l.AccountID)
			if isOpeningEntry {
				if !explicitOpening[l.AccountID] {
					a.openingDebit += l.Debit
					a.openingCredit += l.Credit
				}
				continue
			}
			a.periodDebit += l.Debit
			a.periodCredit += l.Credit
			a.entrySet[e.ID] = true
			a.lineCount++
		}
	}

	out := make([]Balance, 0, chart.Len())
	for _, id := range chart.IDs() {
		acct, _ := chart.Lookup(id)
		a := get(id)

		b := Balance{
			AccountID:   id,
			AccountType: acct.Type,
			Currency:    acct.Currency,
		}
		if nb, ok := NormalBalance(acct.Type); ok {
			b.NormalBalance = nb
		}

		if net, ok := openingByAccount[id]; ok && explicitOpening[id] {
			if net >= 0 {
				b.OpeningDebit = net
			} else {
				b.OpeningCredit = -net
			}
		} else {
			b.OpeningDebit = a.openingDebit
			b.OpeningCredit = a.openingCredit
		}
		b.OpeningNet = b.OpeningDebit - b.OpeningCredit

		b.PeriodDebits = a.periodDebit
		b.PeriodCredits = a.periodCredit
		b.Movement = b.PeriodDebits - b.PeriodCredits

		b.RawBalance = b.OpeningNet + b.Movement
		b.DisplayBalance = displayBalance(b.RawBalance, b.NormalBalance)

		b.SourceEntryCount = len(a.entrySet)
		b.SourceLineCount = a.lineCount

		out = append(out, b)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out
}

// displayBalance flips raw's sign when normal is CREDIT, so a balance in
// its account's normal position always displays as positive — see
// Balance.DisplayBalance's doc comment.
func displayBalance(raw float64, normal DebitCredit) float64 {
	if normal == Credit {
		return -raw
	}
	return raw
}
