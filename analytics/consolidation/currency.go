package consolidation

import (
	"math"

	"github.com/themurtez/go-valuate/financial"
)

// rateKey identifies one (FromCurrency, ToCurrency, Period) a CurrencyRate
// can match against.
type rateKey struct {
	from   string
	to     string
	period financial.Period
}

// buildRateIndex validates rates and returns a lookup keyed by
// (FromCurrency, ToCurrency, Period), plus the issues raised for any
// invalid entry. An entry with a non-finite or non-positive Rate is
// omitted from the index (and reported) rather than included — using an
// invalid rate would be worse than treating it as absent, since a
// caller-visible warning of "no rate available" is more honest than
// silently applying e.g. a negative multiplier.
//
// If more than one valid CurrencyRate targets the same key, the last one
// in rates wins — mirroring a caller supplying a corrected/superseding
// rate later in the same slice; Calculate does not treat this as an error
// since a caller assembling rates from more than one source may
// legitimately want "last wins" override semantics.
func buildRateIndex(rates []CurrencyRate) (map[rateKey]float64, []Issue) {
	index := make(map[rateKey]float64, len(rates))
	var issues []Issue
	for _, r := range rates {
		if math.IsNaN(r.Rate) || math.IsInf(r.Rate, 0) || r.Rate <= 0 {
			issues = append(issues, Issue{
				Code:     IssueInvalidCurrencyRate,
				Severity: SeverityWarning,
				Period:   r.Period,
				Message:  "currency rate " + r.FromCurrency + "->" + r.ToCurrency + " for period \"" + string(r.Period) + "\" is not a finite positive number and was ignored",
			})
			continue
		}
		index[rateKey{from: r.FromCurrency, to: r.ToCurrency, period: r.Period}] = r.Rate
	}
	return index, issues
}
