package ledger

import "sort"

// TrialBalanceMode selects what a TrialBalanceLine's Debit/Credit
// (period-activity) figures represent.
type TrialBalanceMode string

const (
	// ModePeriodActivity reports PeriodDebit/PeriodCredit as the account's
	// activity within the selected range (BalanceOptions.Range), alongside
	// opening and closing.
	ModePeriodActivity TrialBalanceMode = "period_activity"
	// ModeEndingBalances reports only closing positions: PeriodDebit/
	// PeriodCredit are zeroed and ClosingDebit/ClosingCredit are the only
	// meaningful debit/credit figures, matching how a traditional "ending
	// trial balance" report is presented (one debit or credit column per
	// account, no separate activity column).
	ModeEndingBalances TrialBalanceMode = "ending_balances"
)

// TrialBalanceLine is one account's row in a TrialBalance.
type TrialBalanceLine struct {
	// AccountID, AccountNumber, AccountName, AccountType identify the
	// account this row is for — carried directly on the line (rather than
	// requiring a second chart lookup) so a TrialBalance is self-contained
	// for display/export.
	AccountID     string      `json:"account_id"`
	AccountNumber string      `json:"account_number,omitempty"`
	AccountName   string      `json:"account_name,omitempty"`
	AccountType   AccountType `json:"account_type"`

	OpeningDebit  float64 `json:"opening_debit"`
	OpeningCredit float64 `json:"opening_credit"`

	// PeriodDebit and PeriodCredit are the period's activity, present under
	// ModePeriodActivity and zeroed under ModeEndingBalances.
	PeriodDebit  float64 `json:"period_debit"`
	PeriodCredit float64 `json:"period_credit"`

	// ClosingDebit and ClosingCredit present the closing RawBalance split
	// back into a debit column and a credit column (whichever side the
	// balance falls on carries the amount; the other is zero) — the
	// conventional two-column trial-balance presentation.
	ClosingDebit  float64 `json:"closing_debit"`
	ClosingCredit float64 `json:"closing_credit"`

	// RawBalance and DisplayBalance echo Balance's same-named fields — see
	// Balance's doc comment for the sign convention.
	RawBalance     float64 `json:"raw_balance"`
	DisplayBalance float64 `json:"display_balance"`

	Currency string `json:"currency,omitempty"`
}

// TrialBalance is the full trial-balance report: one TrialBalanceLine per
// account plus totals. Totals are computed from Lines exactly as shown —
// this package never adjusts a line to force Balanced to true (see
// Balanced's doc comment).
type TrialBalance struct {
	SchemaVersion string             `json:"schema_version"`
	Mode          TrialBalanceMode   `json:"mode"`
	Lines         []TrialBalanceLine `json:"lines"`

	// TotalDebits and TotalCredits sum, respectively, every line's
	// ClosingDebit and ClosingCredit (the closing position is what a trial
	// balance conventionally proves in balance, regardless of Mode).
	TotalDebits  float64 `json:"total_debits"`
	TotalCredits float64 `json:"total_credits"`
	// Difference is TotalDebits - TotalCredits, signed (not absolute) so a
	// caller can see which side is over.
	Difference float64 `json:"difference"`
	// Balanced is true only when |Difference| <= the tolerance the trial
	// balance was built with. Never forced to true — an unbalanced trial
	// balance is reported as such, with IssueUnbalancedTrialBalance in
	// Issues, rather than silently corrected.
	Balanced bool `json:"balanced"`

	Issues []Issue `json:"issues,omitempty"`
}

// BuildTrialBalance computes a TrialBalance from chart/entries per opts —
// combining CalculateBalances with the conventional two-column trial-
// balance presentation. It does not mutate chart, entries, or opts.
func BuildTrialBalance(chart ChartOfAccounts, entries []JournalEntry, opts BalanceOptions, mode TrialBalanceMode, tolerance float64) TrialBalance {
	var issues []Issue
	issues = append(issues, ValidateRange(opts.Range)...)
	issues = append(issues, validateOpeningBalances(opts.Openings, chart)...)

	balances := CalculateBalances(chart, entries, opts)

	tb := TrialBalance{
		SchemaVersion: SchemaVersion,
		Mode:          mode,
		Lines:         make([]TrialBalanceLine, 0, len(balances)),
	}

	for _, b := range balances {
		acct, _ := chart.Lookup(b.AccountID)
		line := TrialBalanceLine{
			AccountID:      b.AccountID,
			AccountNumber:  acct.Number,
			AccountName:    acct.Name,
			AccountType:    b.AccountType,
			OpeningDebit:   b.OpeningDebit,
			OpeningCredit:  b.OpeningCredit,
			RawBalance:     b.RawBalance,
			DisplayBalance: b.DisplayBalance,
			Currency:       b.Currency,
		}
		if mode == ModePeriodActivity {
			line.PeriodDebit = b.PeriodDebits
			line.PeriodCredit = b.PeriodCredits
		}
		if b.RawBalance >= 0 {
			line.ClosingDebit = b.RawBalance
		} else {
			line.ClosingCredit = -b.RawBalance
		}
		tb.Lines = append(tb.Lines, line)
	}

	sort.Slice(tb.Lines, func(i, j int) bool { return tb.Lines[i].AccountID < tb.Lines[j].AccountID })

	for _, l := range tb.Lines {
		tb.TotalDebits += l.ClosingDebit
		tb.TotalCredits += l.ClosingCredit
	}
	tb.Difference = tb.TotalDebits - tb.TotalCredits
	if tolerance < 0 {
		tolerance = 0
	}
	diff := tb.Difference
	if diff < 0 {
		diff = -diff
	}
	tb.Balanced = diff <= tolerance
	if !tb.Balanced {
		issues = append(issues, Issue{
			Code:     IssueUnbalancedTrialBalance,
			Severity: SeverityError,
			Message:  "trial balance total debits do not equal total credits within tolerance",
		})
	}

	tb.Issues = issues
	return tb
}
