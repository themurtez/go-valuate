package vendorspend

// SpendBridge is the gross/credit/net spend decomposition — task section
// 5. Locked invariant (see invariants_test.go):
//
//	GrossSpend - CreditsRefunds = NetSpend
//
// GrossSpend sums only EffectNormal records; CreditsRefunds sums the
// magnitude of every EffectCredit/EffectRefund/EffectReversal record
// (reported as a positive "amount of credits/refunds/reversals," not a
// signed offset) so gross purchasing is never hidden through netting —
// task section 5's explicit "do not hide gross purchasing through
// netting" rule.
type SpendBridge struct {
	GrossSpend     float64 `json:"gross_spend"`
	CreditsRefunds float64 `json:"credits_refunds"`
	NetSpend       float64 `json:"net_spend"`
}

// addRecord folds one included SpendRecord's Amount into b according to
// its resolved Effect.
func (b *SpendBridge) addRecord(r SpendRecord) {
	amt := r.Amount
	if isReducingEffect(r.resolvedEffect()) {
		b.CreditsRefunds += absFloat(amt)
		b.NetSpend -= absFloat(amt)
		return
	}
	b.GrossSpend += amt
	b.NetSpend += amt
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// computeBridge folds every record in records into one SpendBridge.
func computeBridge(records []SpendRecord) SpendBridge {
	var b SpendBridge
	for _, r := range records {
		b.addRecord(r)
	}
	return b
}
