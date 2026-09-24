package vendorspend

import (
	"github.com/themurtez/go-valuate/accounting/ap"
)

// SpendRecordsFromPayables converts accounting/ap Payables into
// SpendRecords under BasisAccrual (a bill's BillDate is an accrual-basis
// purchase event) — task section 25. This is a typed adapter, not a
// compile-time dependency baked into Calculate: this package's core
// types never import accounting/ap, and a caller with no AP-package
// usage never needs this function.
//
// Uses each Payable's OriginalAmount — never OpenAmount, which reflects
// today's remaining balance, not the period's original economic spend
// (see the package doc comment's AP-boundary rule and
// ap_boundary_test.go's permanent regression test: a supplier's annual
// spend must never become its ending AP balance). A
// DocumentTypeVendorCredit Payable maps to EffectCredit with a
// non-negative Amount (this package's Amount is always expected
// non-negative regardless of Effect — see SpendEffect's doc comment),
// converting accounting/ap's own "credit is a negative OriginalAmount"
// convention into this package's "Effect determines sign" convention.
//
// Timing/accrual differences: a Payable's BillDate is normally the same
// economic event as a SpendRecord's Date under BasisAccrual, but the two
// packages diverge the moment a bill is partially paid, disputed, or
// carries a vendor credit — accounting/ap tracks OpenAmount (what
// remains owed today), while this package tracks Amount (what was
// economically purchased), and this package never derives one from the
// other.
//
// periodOf is the caller-resolved Period label each Payable's BillDate
// falls into — this package never infers period boundaries on its own,
// mirroring every other adapter in this repository's identical
// "caller resolves periods" rule.
func SpendRecordsFromPayables(payables []ap.Payable, periodOf func(billDate string) string) []SpendRecord {
	out := make([]SpendRecord, 0, len(payables))
	for _, p := range payables {
		amt := p.OriginalAmount
		effect := EffectNormal
		if p.DocumentType == ap.DocumentTypeVendorCredit {
			effect = EffectCredit
			amt = -amt // a vendor credit's OriginalAmount is <= 0 by accounting/ap's convention; this package's Amount is a non-negative magnitude.
		}
		billDateStr := p.BillDate.Format("2006-01-02")
		out = append(out, SpendRecord{
			SpendID:     p.ID,
			SupplierID:  p.SupplierID,
			Period:      periodOf(billDateStr),
			Date:        p.BillDate,
			Amount:      amt,
			Currency:    p.Currency,
			Effect:      effect,
			Basis:       BasisAccrual,
			ReferenceID: p.BillNumber,
		})
	}
	return out
}
