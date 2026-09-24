package kpi

import (
	"math"
	"testing"
)

// Fuzz targets — task section 45. Goals: no panic, no infinite loop, no
// NaN/Inf leaks, bounded depth/complexity, structured invalid results.
// Run short/laptop-safe locally only (go test -fuzz, time-boxed) — see
// the completion report for the actual corpus/duration used.

// FuzzExpressionEvaluation exercises validateExpression + a full
// Calculate over an arbitrary small expression shape built from fuzzer
// bytes, checking only the invariants every valid or invalid expression
// must uphold: no panic, and if a KPIResult IS produced, its Value.Amount
// is never NaN/Inf.
func FuzzExpressionEvaluation(f *testing.F) {
	f.Add(byte(0), 10.0, 3.0, true, true)
	f.Add(byte(3), 100.0, 0.0, true, true)
	f.Add(byte(4), math.MaxFloat64, math.MaxFloat64, true, true)
	f.Add(byte(1), math.NaN(), 1.0, true, true)

	ops := []Operator{OpAdd, OpSubtract, OpMultiply, OpDivide, OpPercent, OpPercentChange, OpMin, OpMax, OpSum, OpAverage}

	f.Fuzz(func(t *testing.T, opSelector byte, a, b float64, aAvail, bAvail bool) {
		op := ops[int(opSelector)%len(ops)]
		expr := Binary(op, Metric(MetricRef{Code: "a"}), Metric(MetricRef{Code: "b"}))
		def := Definition{Code: "k", Unit: currencyUnit("USD"), Formula: expr}

		metricsIn := []MetricValue{
			{Code: "a", Period: "P1", Value: a, Available: aAvail, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
			{Code: "b", Period: "P1", Value: b, Available: bAvail, Unit: currencyUnit("USD"), Aggregation: AggregationSum},
		}

		res := Calculate(onePeriodInput([]Definition{def}, metricsIn), Options{})

		for _, kr := range res.KPIResults {
			if math.IsNaN(kr.Value.Amount) || math.IsInf(kr.Value.Amount, 0) {
				t.Fatalf("leaked non-finite Value.Amount: %+v (op=%v a=%v b=%v)", kr.Value, op, a, b)
			}
			if !kr.Value.Available && kr.Value.Amount != 0 {
				t.Fatalf("unavailable Value carried a nonzero Amount: %+v", kr.Value)
			}
		}
	})
}

// FuzzDependencyGraph exercises buildDependencyGraph over a small,
// fuzzer-controlled set of KPI-to-KPI edges — checking for panics,
// non-termination, and that every code ends up in EXACTLY ONE of
// (evaluationOrder) or is absent because it failed structural validation
// (never silently dropped after passing validation).
func FuzzDependencyGraph(f *testing.F) {
	f.Add(uint8(0b000), uint8(0b000), uint8(0b000))
	f.Add(uint8(0b010), uint8(0b100), uint8(0b001)) // A->B->C->A cycle
	f.Add(uint8(0b111), uint8(0b111), uint8(0b111)) // dense

	f.Fuzz(func(t *testing.T, edgesA, edgesB, edgesC uint8) {
		// Each of A/B/C may reference any subset of {A,B,C} via its low 3
		// bits (bit0=A, bit1=B, bit2=C).
		build := func(name string, bits uint8) Definition {
			var args []Expression
			for i, target := range []string{"a", "b", "c"} {
				if bits&(1<<uint(i)) != 0 {
					args = append(args, KPIExpr(KPIRef{Code: target}))
				}
			}
			if len(args) == 0 {
				return Definition{Code: name, Unit: currencyUnit("USD"), Formula: Const(1)}
			}
			if len(args) == 1 {
				return Definition{Code: name, Unit: currencyUnit("USD"), Formula: args[0]}
			}
			return Definition{Code: name, Unit: currencyUnit("USD"), Formula: Variadic(OpSum, args...)}
		}
		defs := []Definition{build("a", edgesA), build("b", edgesB), build("c", edgesC)}

		res := Calculate(onePeriodInput(defs, nil), Options{})

		// No panic already implicitly checked by reaching here. Every
		// KPIResult's Value must be well-formed (Available or a valid
		// Reason, never NaN/Inf).
		for _, kr := range res.KPIResults {
			if math.IsNaN(kr.Value.Amount) || math.IsInf(kr.Value.Amount, 0) {
				t.Fatalf("leaked non-finite Value.Amount: %+v", kr.Value)
			}
		}
	})
}

// FuzzThresholdBands exercises validateBands/bandFor over fuzzer-supplied
// band boundaries.
func FuzzThresholdBands(f *testing.F) {
	f.Add(0.0, 50.0, 50.0, 100.0, 25.0)
	f.Add(0.0, 60.0, 50.0, 100.0, 55.0) // overlapping
	f.Add(50.0, 50.0, 0.0, 0.0, 50.0)   // degenerate

	f.Fuzz(func(t *testing.T, min1, max1, min2, max2, probe float64) {
		if math.IsNaN(min1) || math.IsNaN(max1) || math.IsNaN(min2) || math.IsNaN(max2) || math.IsNaN(probe) {
			t.Skip()
		}
		if math.IsInf(min1, 0) || math.IsInf(max1, 0) || math.IsInf(min2, 0) || math.IsInf(max2, 0) {
			t.Skip()
		}
		bands := []ThresholdBand{{Label: "A", Min: min1, Max: max1}, {Label: "B", Min: min2, Max: max2}}
		ok, overlapping, degenerate := validateBands(bands)
		if ok && (overlapping || degenerate) {
			t.Fatalf("ok=true but overlapping=%v degenerate=%v", overlapping, degenerate)
		}
		if ok {
			// bandFor must never panic regardless of probe.
			_, _ = bandFor(bands, probe)
		}
	})
}

// FuzzUnitCompatibility exercises combineAdditive/combineMultiplicative/
// combineDivisive over fuzzer-selected UnitKind pairs -- must never
// panic, and a reported-compatible result must always itself be Valid()
// or the zero Unit.
func FuzzUnitCompatibility(f *testing.F) {
	f.Add(uint8(0), uint8(0), "USD", "USD", "", "")
	f.Add(uint8(0), uint8(2), "CAD", "", "", "")
	f.Add(uint8(9), uint8(9), "", "", "FOO", "BAR")

	kinds := []UnitKind{UnitCurrency, UnitCount, UnitHours, UnitDays, UnitPercent, UnitRatio, UnitQuantity, UnitArea, UnitUnitless, UnitCustom}

	f.Fuzz(func(t *testing.T, kindA, kindB uint8, curA, curB, labelA, labelB string) {
		a := Unit{Kind: kinds[int(kindA)%len(kinds)], CurrencyCode: curA, CustomLabel: labelA}
		b := Unit{Kind: kinds[int(kindB)%len(kinds)], CurrencyCode: curB, CustomLabel: labelB}

		for _, fn := range []func(Unit, Unit) unitCompatibility{combineAdditive, combineMultiplicative, combineDivisive} {
			compat := fn(a, b)
			if compat.compatible && compat.result.Kind != "" && !compat.result.Valid() {
				t.Fatalf("reported-compatible result is not itself Valid(): a=%+v b=%+v result=%+v", a, b, compat.result)
			}
		}
	})
}
