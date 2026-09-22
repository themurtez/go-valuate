package consensus

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

// benchmarkInputs builds n consensus.Input values, alternating enterprise-
// and equity-value methods (with a populated Bridge on the enterprise-
// value ones), so a TargetBasis-driven conversion pass has real work to do
// for every included input, not just a same-basis pass-through.
func benchmarkInputs(n int) []Input {
	inputs := make([]Input, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			inputs[i] = Input{
				Method: valuation.Code(fmt.Sprintf("METHOD_%d", i)), ValueType: valuation.ValueTypeEnterprise,
				Value: 1000000 + float64(i)*1000, Weight: 1,
				Bridge: valuation.Bridge{Available: true, EnterpriseValue: 1000000 + float64(i)*1000, ExcessCash: 50000, TotalDebt: 100000, EquityValue: 950000 + float64(i)*1000},
			}
		} else {
			inputs[i] = Input{Method: valuation.Code(fmt.Sprintf("METHOD_%d", i)), ValueType: valuation.ValueTypeEquity, Value: 900000 + float64(i)*1000, Weight: 1}
		}
	}
	return inputs
}

// BenchmarkCalculate_100Methods exercises Calculate with basis conversion
// active (TargetBasis set) over a realistically-sized included-method set
// — the candidate hot path named in docs/V1_CONTRACTS.md's performance
// sanity checks. 100 methods is already far beyond any real valuation run
// (this module ships exactly 5 methods); this benchmark deliberately
// over-scales to make any O(n^2) regression in basis conversion or
// dispersion statistics visible.
func BenchmarkCalculate_100Methods(b *testing.B) {
	inputs := benchmarkInputs(100)
	opts := Options{TargetBasis: valuation.ValueTypeEquity}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(inputs, opts)
	}
}
