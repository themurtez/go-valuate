package synthetic

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
)

// BuildConfirmedAdjustments returns Meridian SaaS's one confirmed
// normalization adjustment: the 2023 one-time rebranding cost folded into
// that year's OPEX_OTHER (see meridianYears' doc comment). Targets EBITDA
// only, since this is a genuine one-time operating cost with no
// owner-discretionary component for this non-owner-operated business.
func BuildConfirmedAdjustments() []adjustments.Adjustment {
	return []adjustments.Adjustment{
		{
			ID:         "adj-2023-rebrand",
			Period:     "2023",
			Type:       adjustments.TypeOneTimeExpense,
			Amount:     120_000,
			Effect:     adjustments.EffectIncrease,
			Targets:    []adjustments.Target{adjustments.TargetEBITDA, adjustments.TargetSDE},
			Reason:     "One-time corporate rebranding and website relaunch, not expected to recur",
			SourceCode: financial.CodeOpexOther,
			Included:   true,
		},
	}
}
