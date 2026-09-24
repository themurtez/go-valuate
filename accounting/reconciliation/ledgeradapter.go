package reconciliation

import (
	"strconv"

	"github.com/themurtez/go-valuate/accounting/ledger"
)

// LedgerBookItemsOptions configures BookItemsFromLedger.
type LedgerBookItemsOptions struct {
	// AccountID is the single ledger.Account.ID to convert entries for.
	// Required — this adapter never aggregates multiple accounts into one
	// reconciliation (task section 32: "Caller explicitly provides:
	// AccountID").
	AccountID string
	// StartDate/EndDate bound which entries are converted, inclusive,
	// "YYYY-MM-DD", compared against ledger.JournalEntry.Date lexically
	// (matching ledger's own plain-string date convention). Both
	// required — this adapter never infers a period/date range.
	StartDate string
	EndDate   string
	// IncludeDraftAndVoided, when true, includes StatusDraft/StatusVoided
	// entries. Default (false) excludes them — task section 32: "Exclude
	// draft/voided by default."
	IncludeDraftAndVoided bool
	// SourceType is stamped onto every resulting BookItem.SourceType.
	// Defaults to "ledger" if empty.
	SourceType string
}

func (o LedgerBookItemsOptions) sourceType() string {
	if o.SourceType != "" {
		return o.SourceType
	}
	return "ledger"
}

// BookItemsFromLedger converts every JournalLine posted to
// opts.AccountID within [opts.StartDate, opts.EndDate] into a BookItem,
// preserving EntryID/LineID/reference/source per task section 32.
// Direction/Amount are derived from the ledger's raw debit/credit
// convention: a net-debit line contributes DirectionInflow, a net-credit
// line contributes DirectionOutflow, UNLESS the account's own
// NormalBalance is CREDIT — see the doc comment on
// ledgerLineDirectionAndAmount below and task section 33's "do not
// assume cash-asset semantics for liabilities." One JournalLine produces
// one BookItem (a multi-line entry produces multiple BookItems, one per
// line touching this account) — ItemID is EntryID + "/" + LineID (or
// EntryID + a positional suffix if the line has no ID), so items remain
// individually addressable for matching.
//
// entries and chart are never mutated. An entry outside the date range,
// for a different account, or excluded by IncludeDraftAndVoided is
// skipped. This function performs no validation of entries/chart itself
// — a caller should run ledger.ValidateEntries separately if desired
// (per this repository's "reuse, don't recalculate" convention).
func BookItemsFromLedger(entries []ledger.JournalEntry, chart ledger.ChartOfAccounts, opts LedgerBookItemsOptions) []BookItem {
	acct, found := chart.Lookup(opts.AccountID)
	if !found {
		return nil
	}
	normal, _ := ledger.NormalBalance(acct.Type)

	var items []BookItem
	for _, e := range entries {
		status := e.EffectiveStatus()
		if !opts.IncludeDraftAndVoided {
			if status == ledger.StatusDraft || status == ledger.StatusVoided {
				continue
			}
		}
		if e.Date < opts.StartDate || e.Date > opts.EndDate {
			continue
		}
		for i, line := range e.Lines {
			if line.AccountID != opts.AccountID {
				continue
			}
			direction, amount := ledgerLineDirectionAndAmount(line, normal)
			if amount == 0 {
				continue
			}
			itemID := e.ID
			if line.ID != "" {
				itemID = e.ID + "/" + line.ID
			} else {
				itemID = e.ID + "/L" + strconv.Itoa(i)
			}
			items = append(items, BookItem{
				ItemID:      itemID,
				Date:        e.Date,
				Amount:      amount,
				Direction:   direction,
				Reference:   e.Reference,
				Description: e.Description,
				AccountID:   opts.AccountID,
				Currency:    acct.Currency,
				SourceType:  opts.sourceType(),
				SourceID:    e.ID,
				SourceRef:   SourceRef{System: "ledger", ID: e.ID},
			})
		}
	}
	return items
}

// ledgerLineDirectionAndAmount converts one JournalLine's raw
// debit/credit into a (Direction, Amount) pair using this package's
// INFLOW=+/OUTFLOW=- convention, oriented so that a movement which
// increases the account's DisplayBalance (i.e. moves in the account's
// own normal-balance direction) is DirectionInflow, and one that
// decreases it is DirectionOutflow. This intentionally does NOT hard-code
// "debit=inflow" — for a CREDIT-normal account (LIABILITY/EQUITY/REVENUE,
// e.g. a credit-card or loan control account), a credit line (which
// increases what is owed) is the inflow-equivalent movement, and a debit
// line (a payment, decreasing what is owed) is the outflow-equivalent —
// task section 33: "Correctly map debit/credit based on account
// semantics... Do not assume cash-asset semantics for liabilities."
func ledgerLineDirectionAndAmount(line ledger.JournalLine, normal ledger.DebitCredit) (Direction, float64) {
	net := line.Debit - line.Credit // raw convention: debit-positive
	if normal == ledger.Credit {
		net = -net
	}
	if net >= 0 {
		return DirectionInflow, net
	}
	return DirectionOutflow, -net
}
