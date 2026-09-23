package ledger

import (
	"sort"
	"strconv"
)

// TrialBalanceInput is a portable representation of a trial balance a
// caller already has (e.g. exported from an external bookkeeping system)
// with no underlying journal-entry detail. NormalizeTrialBalance validates
// and normalizes this into a NormalizedTrialBalance; this package never
// fabricates JournalEntry values to represent one — see the package doc
// comment's TB-import-path section.
type TrialBalanceInput struct {
	// Period is the reporting period this trial balance represents (e.g.
	// "2025", "2025-Q4"). Required.
	Period string `json:"period"`
	// Lines is one row per account. Required, at least one entry.
	Lines []TrialBalanceInputLine `json:"lines"`
}

// TrialBalanceInputLine is one account's row in an imported
// TrialBalanceInput.
type TrialBalanceInputLine struct {
	// AccountID identifies the account this row is for. Required, and must
	// match an Account.ID in the chart of accounts NormalizeTrialBalance is
	// called with.
	AccountID string `json:"account_id"`
	// Debit and Credit are this row's closing-balance amounts (the
	// conventional two-column trial-balance presentation: exactly one of
	// these is expected to be nonzero per line, mirroring JournalLine).
	// Must both be >= 0.
	Debit  float64 `json:"debit"`
	Credit float64 `json:"credit"`
	// OpeningBalance is an optional starting balance for this account, if
	// the imported source distinguishes opening from closing (many simple
	// TB exports do not, in which case leave this at the zero value).
	OpeningBalance *OpeningBalance `json:"opening_balance,omitempty"`
	// Currency is the ISO 4217 currency this row is denominated in.
	// Optional; see the package doc comment's multi-currency section.
	Currency string `json:"currency,omitempty"`
	// Dimensions is zero or more optional analysis tags carried on this
	// imported row.
	Dimensions []Dimension `json:"dimensions,omitempty"`
	// SourceRef is an opaque caller-defined pointer to this row's origin
	// (e.g. the source file's row number or an external system's line ID).
	SourceRef string `json:"source_ref,omitempty"`
}

// NormalizedTrialBalanceLine is one validated, normalized row of an
// imported trial balance.
type NormalizedTrialBalanceLine struct {
	AccountID      string      `json:"account_id"`
	AccountNumber  string      `json:"account_number,omitempty"`
	AccountName    string      `json:"account_name,omitempty"`
	AccountType    AccountType `json:"account_type"`
	OpeningDebit   float64     `json:"opening_debit"`
	OpeningCredit  float64     `json:"opening_credit"`
	ClosingDebit   float64     `json:"closing_debit"`
	ClosingCredit  float64     `json:"closing_credit"`
	RawBalance     float64     `json:"raw_balance"`
	DisplayBalance float64     `json:"display_balance"`
	Currency       string      `json:"currency,omitempty"`
	SourceRef      string      `json:"source_ref,omitempty"`
}

// NormalizedTrialBalance is the validated, normalized result of importing
// a TrialBalanceInput. Its shape deliberately mirrors TrialBalance so
// downstream code can treat an imported TB the same way as one built from
// journal entries.
type NormalizedTrialBalance struct {
	SchemaVersion string                       `json:"schema_version"`
	Period        string                       `json:"period"`
	Lines         []NormalizedTrialBalanceLine `json:"lines"`

	TotalDebits  float64 `json:"total_debits"`
	TotalCredits float64 `json:"total_credits"`
	Difference   float64 `json:"difference"`
	Balanced     bool    `json:"balanced"`

	// BaseCurrency is the currency NormalizeTrialBalance's totals were
	// computed in — the first non-empty Currency encountered across
	// chart/input lines in AccountID/input order. Lines whose Currency
	// differs from BaseCurrency are excluded from TotalDebits/TotalCredits
	// and flagged with IssueMixedCurrency — see currency.go.
	BaseCurrency string `json:"base_currency,omitempty"`

	Issues []Issue `json:"issues,omitempty"`
}

// NormalizeTrialBalance validates and normalizes in against chart,
// checking: known account identity, valid debit/credit shape, duplicate
// account rows, non-finite values, and currency consistency. It never
// auto-corrects an unbalanced total — Balanced simply reports what the
// input actually sums to, and an out-of-tolerance total is additionally
// flagged via IssueUnbalancedTrialBalance in Issues. It does not mutate in
// or chart.
func NormalizeTrialBalance(in TrialBalanceInput, chart ChartOfAccounts, tolerance float64) NormalizedTrialBalance {
	out := NormalizedTrialBalance{
		SchemaVersion: SchemaVersion,
		Period:        in.Period,
	}

	if in.Period == "" {
		out.Issues = append(out.Issues, Issue{
			Code:     IssueInvalidPeriod,
			Severity: SeverityError,
			Message:  "trial balance input has an empty Period",
		})
	}

	seen := make(map[string]bool, len(in.Lines))
	baseCurrency := ""

	for i, line := range in.Lines {
		ref := line.AccountID
		if ref == "" {
			ref = "line[" + strconv.Itoa(i) + "]"
		}

		if line.AccountID == "" {
			out.Issues = append(out.Issues, Issue{
				Code:     IssueUnknownAccount,
				Severity: SeverityError,
				Message:  "trial balance line " + ref + " has an empty account ID",
			})
			continue
		}
		if seen[line.AccountID] {
			out.Issues = append(out.Issues, Issue{
				Code:     IssueDuplicateAccount,
				Severity: SeverityError,
				Message:  "duplicate account row in trial balance: " + line.AccountID,
				Account:  line.AccountID,
			})
			continue
		}
		seen[line.AccountID] = true

		acct, ok := chart.Lookup(line.AccountID)
		if !ok {
			out.Issues = append(out.Issues, Issue{
				Code:     IssueUnknownAccount,
				Severity: SeverityError,
				Message:  "trial balance references unknown account: " + line.AccountID,
				Account:  line.AccountID,
			})
			continue
		}

		if isNonFinite(line.Debit) || isNonFinite(line.Credit) {
			out.Issues = append(out.Issues, Issue{
				Code:     IssueNonFiniteAmount,
				Severity: SeverityError,
				Message:  "trial balance line " + line.AccountID + " has a non-finite amount",
				Account:  line.AccountID,
			})
			continue
		}
		if line.Debit < 0 || line.Credit < 0 {
			out.Issues = append(out.Issues, Issue{
				Code:     IssueNegativeAmount,
				Severity: SeverityError,
				Message:  "trial balance line " + line.AccountID + " has a negative debit or credit",
				Account:  line.AccountID,
			})
			continue
		}
		if line.Debit != 0 && line.Credit != 0 {
			out.Issues = append(out.Issues, Issue{
				Code:     IssueInvalidDebitCredit,
				Severity: SeverityError,
				Message:  "trial balance line " + line.AccountID + " has both debit and credit set",
				Account:  line.AccountID,
			})
			continue
		}

		lineCurrency := line.Currency
		if lineCurrency == "" {
			lineCurrency = acct.Currency
		}
		if baseCurrency == "" && lineCurrency != "" {
			baseCurrency = lineCurrency
		}
		mixedCurrency := lineCurrency != "" && baseCurrency != "" && lineCurrency != baseCurrency
		if mixedCurrency {
			out.Issues = append(out.Issues, Issue{
				Code:     IssueMixedCurrency,
				Severity: SeverityError,
				Message:  "trial balance line " + line.AccountID + " currency (" + lineCurrency + ") does not match base currency (" + baseCurrency + ")",
				Account:  line.AccountID,
			})
		}

		norm := NormalizedTrialBalanceLine{
			AccountID:     line.AccountID,
			AccountNumber: acct.Number,
			AccountName:   acct.Name,
			AccountType:   acct.Type,
			ClosingDebit:  line.Debit,
			ClosingCredit: line.Credit,
			Currency:      lineCurrency,
			SourceRef:     line.SourceRef,
		}
		if line.OpeningBalance != nil {
			ob := *line.OpeningBalance
			if !isNonFinite(ob.Debit) && !isNonFinite(ob.Credit) && ob.Debit >= 0 && ob.Credit >= 0 {
				norm.OpeningDebit = ob.Debit
				norm.OpeningCredit = ob.Credit
			} else {
				out.Issues = append(out.Issues, Issue{
					Code:     IssueInvalidOpeningBalance,
					Severity: SeverityError,
					Message:  "trial balance line " + line.AccountID + " has an invalid opening balance",
					Account:  line.AccountID,
				})
			}
		}
		norm.RawBalance = norm.ClosingDebit - norm.ClosingCredit
		if nb, ok := NormalBalance(acct.Type); ok {
			norm.DisplayBalance = displayBalance(norm.RawBalance, nb)
		}

		if mixedCurrency {
			continue // excluded from totals, per currency.go's rule
		}
		out.Lines = append(out.Lines, norm)
	}

	sort.Slice(out.Lines, func(i, j int) bool { return out.Lines[i].AccountID < out.Lines[j].AccountID })

	for _, l := range out.Lines {
		out.TotalDebits += l.ClosingDebit
		out.TotalCredits += l.ClosingCredit
	}
	out.BaseCurrency = baseCurrency
	out.Difference = out.TotalDebits - out.TotalCredits
	if tolerance < 0 {
		tolerance = 0
	}
	diff := out.Difference
	if diff < 0 {
		diff = -diff
	}
	out.Balanced = diff <= tolerance
	if !out.Balanced {
		out.Issues = append(out.Issues, Issue{
			Code:     IssueUnbalancedTrialBalance,
			Severity: SeverityError,
			Message:  "imported trial balance total debits do not equal total credits within tolerance",
		})
	}

	return out
}
