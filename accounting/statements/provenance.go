package statements

import (
	"sort"

	"github.com/themurtez/go-valuate/financial"
)

// sourceRefsForPeriod converts a mappedAccountFigures' RowContributor
// slice into financial.SourceRef entries for one Period — the
// provenance task section 19/20 requires: every resulting canonical item
// traceable to its source account ID/number/name, and (via
// financial.SourceRef.Label, which carries the account's display name)
// readable even without a second chart lookup. financial.SourceRef has
// no dedicated account-ID field of its own (it was designed for
// financial.RawLineItem row provenance, not account provenance — see
// financial/types.go), so this package uses SourceRef.RowID to carry the
// contributing ledger.Account.ID directly, which is exactly analogous:
// both are "the stable identifier of whatever thing contributed this
// amount."
//
// Sorted by RowID for determinism (contributors is already sorted by
// AccountID from aggregateMappedAccounts, so this mirrors that order,
// but is re-sorted explicitly here so this function's own contract does
// not silently depend on caller ordering).
func sourceRefsForPeriod(contributors []RowContributor, period financial.Period) []financial.SourceRef {
	var refs []financial.SourceRef
	for _, c := range contributors {
		amount, ok := c.Amounts[period]
		if !ok {
			continue
		}
		label := c.AccountName
		if c.AccountNumber != "" {
			label = c.AccountNumber + " " + label
		}
		refs = append(refs, financial.SourceRef{
			RowID:  c.AccountID,
			Label:  label,
			Period: period,
			Amount: amount,
		})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].RowID < refs[j].RowID })
	return refs
}
