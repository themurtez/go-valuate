package kpi

// UnitKind is this package's small, explicit unit taxonomy — task section
// 11. This is deliberately not a full symbolic units engine: task section
// 12 explicitly calls that unnecessary and asks for "a small explicit
// system with a declared expected output unit and compatibility checks"
// instead. See Unit.Compatible/CombinedUnit for the compatibility rules.
type UnitKind string

const (
	UnitCurrency UnitKind = "CURRENCY"
	UnitCount    UnitKind = "COUNT"
	UnitHours    UnitKind = "HOURS"
	UnitDays     UnitKind = "DAYS"
	UnitPercent  UnitKind = "PERCENT"
	UnitRatio    UnitKind = "RATIO"
	UnitQuantity UnitKind = "QUANTITY"
	UnitArea     UnitKind = "AREA"
	UnitUnitless UnitKind = "UNITLESS"
	UnitCustom   UnitKind = "CUSTOM"
)

func isRecognizedUnitKind(k UnitKind) bool {
	switch k {
	case UnitCurrency, UnitCount, UnitHours, UnitDays, UnitPercent, UnitRatio,
		UnitQuantity, UnitArea, UnitUnitless, UnitCustom:
		return true
	default:
		return false
	}
}

// Unit is one value's unit of measure. Currency carries an explicit
// CurrencyCode (task section 11: "do not hard-code USD" — this package has
// no default currency at all; a Unit{Kind: UnitCurrency} with an empty
// CurrencyCode is itself an incomplete unit, see Unit.Valid). Custom units
// (e.g. a caller's own "SQUARE_FEET"-flavored per-count unit not covered
// by the fixed kinds) use UnitCustom with a CustomLabel; this package
// never interprets CustomLabel's text, only compares it for exact
// equality — two UnitCustom Units are compatible for ADD/SUBTRACT only
// when CustomLabel matches exactly.
type Unit struct {
	Kind UnitKind `json:"kind"`
	// CurrencyCode is required (non-empty) when Kind == UnitCurrency, and
	// ignored otherwise. An ISO 4217-style code (e.g. "USD", "CAD"); this
	// package never validates it against a real currency list, only
	// compares it for exact equality (task section 12's currency-mismatch
	// rule).
	CurrencyCode string `json:"currency_code,omitempty"`
	// CustomLabel is required (non-empty) when Kind == UnitCustom, and
	// ignored otherwise.
	CustomLabel string `json:"custom_label,omitempty"`
}

// Valid reports whether u is a structurally complete Unit: Kind is
// recognized, CurrencyCode is set iff Kind == UnitCurrency, and
// CustomLabel is set iff Kind == UnitCustom.
func (u Unit) Valid() bool {
	if !isRecognizedUnitKind(u.Kind) || u.Kind == "" {
		return false
	}
	switch u.Kind {
	case UnitCurrency:
		return u.CurrencyCode != ""
	case UnitCustom:
		return u.CustomLabel != ""
	default:
		return true
	}
}

// Equal reports whether u and o denote the exact same unit (same Kind,
// and matching CurrencyCode/CustomLabel where relevant).
func (u Unit) Equal(o Unit) bool {
	if u.Kind != o.Kind {
		return false
	}
	switch u.Kind {
	case UnitCurrency:
		return u.CurrencyCode == o.CurrencyCode
	case UnitCustom:
		return u.CustomLabel == o.CustomLabel
	default:
		return true
	}
}

// String returns a stable, human-readable label for u — display/trace use
// only, never parsed back.
func (u Unit) String() string {
	switch u.Kind {
	case UnitCurrency:
		if u.CurrencyCode != "" {
			return string(u.Kind) + ":" + u.CurrencyCode
		}
		return string(u.Kind)
	case UnitCustom:
		if u.CustomLabel != "" {
			return string(u.Kind) + ":" + u.CustomLabel
		}
		return string(u.Kind)
	default:
		return string(u.Kind)
	}
}

// unitCompatibility is the result of checking whether two Units may be
// combined under a given arithmetic operator, and what the combined
// result's Unit is.
//
// Every combineXxx function below requires both inputs to themselves be
// Unit.Valid() — a structurally malformed Unit (e.g. UnitCurrency with an
// empty CurrencyCode) is always reported incompatible, regardless of its
// Kind, rather than silently propagating a malformed shape into the
// combined result (found via fuzzing — FuzzUnitCompatibility — after
// buildMetricIndex separately started rejecting invalid MetricValue.Unit
// values at the point a caller actually supplies one; this is the
// corresponding defense at the primitive level itself, so these
// functions are safe standalone and not merely safe by caller
// discipline).
type unitCompatibility struct {
	compatible bool
	result     Unit
	// currencyMismatch/unitMismatch distinguish the two invalid-combination
	// reasons ADD/SUBTRACT can produce, so evaluate.go can report the more
	// specific of AvailabilityCurrencyMismatch vs AvailabilityUnitMismatch
	// (task section 35's "CAD + USD -> currency mismatch" /
	// "Currency + Hours -> unit mismatch" distinction).
	currencyMismatch bool
}

// combineAdditive checks ADD/SUBTRACT compatibility between a and b — task
// section 12: "CAD + CAD -> valid", "CAD + USD -> invalid without
// conversion", "Currency + Hours -> invalid". Two Units are additively
// compatible only when Equal (same Kind, same CurrencyCode/CustomLabel);
// the result's Unit is then simply a (== b, by Equal). PERCENT and RATIO
// are treated as their own distinct Kinds for this purpose (never silently
// interchanged — task section 13's "do not mix them" rule).
func combineAdditive(a, b Unit) unitCompatibility {
	if !a.Valid() || !b.Valid() {
		return unitCompatibility{compatible: false}
	}
	if a.Equal(b) {
		return unitCompatibility{compatible: true, result: a}
	}
	if a.Kind == UnitCurrency && b.Kind == UnitCurrency {
		return unitCompatibility{compatible: false, currencyMismatch: true}
	}
	return unitCompatibility{compatible: false}
}

// combineMultiplicative checks MULTIPLY compatibility between a and b.
// This package's small explicit system supports only the practically
// useful multiplicative combinations task section 12 calls out
// (Currency * Ratio/Percent/Unitless -> Currency scaling; Count/Quantity *
// Ratio/Percent/Unitless -> Count/Quantity scaling) rather than a full
// symbolic system deriving compound units like Currency*Hours. Multiplying
// two Units that are not one of these recognized shapes is reported
// incompatible — a caller needing a genuinely compound unit sets
// Definition.Unit explicitly and relies on ExpectedOutputUnit validation
// instead (see checkExpectedOutputUnit).
func combineMultiplicative(a, b Unit) unitCompatibility {
	if !a.Valid() || !b.Valid() {
		return unitCompatibility{compatible: false}
	}
	if isScalarKind(a.Kind) && isScalarKind(b.Kind) {
		// RATIO/PERCENT/UNITLESS combined with each other stays scalar;
		// prefer RATIO's presence, then PERCENT, else UNITLESS, matching
		// this package's percent-convention precedence (percent is a
		// scaled ratio, task section 13).
		return unitCompatibility{compatible: true, result: firstScalar(a, b)}
	}
	if isScalarKind(a.Kind) {
		return unitCompatibility{compatible: true, result: b}
	}
	if isScalarKind(b.Kind) {
		return unitCompatibility{compatible: true, result: a}
	}
	return unitCompatibility{compatible: false}
}

// combineDivisive checks DIVIDE compatibility between numerator a and
// denominator b — task section 12's three explicit examples:
//
//	Currency / Count    -> currency-per-count (UnitCustom, "<currency>_PER_<denominator-kind>")
//	Currency / Currency -> ratio (same currency only; cross-currency is a
//	                        currency mismatch, same as combineAdditive)
//	<any> / <same kind>  -> ratio, when a and b share the exact same Unit
//	<any> / scalar        -> a's own unit unchanged (dividing by a plain
//	                        count/ratio/percent/unitless scales, does not
//	                        change kind)
//
// Beyond those three explicit examples, Currency divided by any other
// non-scalar, non-currency denominator kind (Hours, Days, Quantity, Area,
// Count, ...) is also supported, generalizing the same
// "currency-per-<denominator>" UnitCustom shape — task section 33's
// worked "Revenue per Square Foot" (Currency/Area) and "Average Ticket"
// (Currency/Count) examples both require this: a per-unit-of-something
// currency rate is exactly the kind of "practical" combination task
// section 12 asks this package to support, even though the task's own
// worked example list under section 12 only spells out Count explicitly.
// Two non-currency, non-equal, non-scalar Units dividing each other
// (e.g. Hours / Count) remains incompatible — that combination has no
// single obviously-correct result Unit the way currency-per-X does, so
// per task section 12's "do not silently cast incompatible units"
// instruction, a caller needing it sets Definition.Unit explicitly and
// accepts evaluate.go's declared-vs-actual mismatch check will not apply
// (a KPI whose formula root is not itself Currency-denominated cannot
// currently express that Unit through DIVIDE's own inference).
func combineDivisive(a, b Unit) unitCompatibility {
	if !a.Valid() || !b.Valid() {
		return unitCompatibility{compatible: false}
	}
	if a.Equal(b) {
		return unitCompatibility{compatible: true, result: Unit{Kind: UnitRatio}}
	}
	if isScalarKind(b.Kind) {
		return unitCompatibility{compatible: true, result: a}
	}
	if a.Kind == UnitCurrency && b.Kind == UnitCurrency {
		// Different currencies: same-currency case already handled by
		// a.Equal(b) above, so reaching here means a mismatch.
		return unitCompatibility{compatible: false, currencyMismatch: true}
	}
	if a.Kind == UnitCurrency && b.Kind != UnitCurrency {
		return unitCompatibility{compatible: true, result: currencyPerUnit(a, b)}
	}
	return unitCompatibility{compatible: false}
}

// isScalarKind reports whether k is one of the three "dimensionless
// multiplier" kinds (RATIO, PERCENT, UNITLESS) that MULTIPLY/DIVIDE treat
// as scaling factors rather than combining into a new unit kind.
func isScalarKind(k UnitKind) bool {
	return k == UnitRatio || k == UnitPercent || k == UnitUnitless
}

// firstScalar returns a if it is RATIO, else b if it is RATIO, else a if
// PERCENT, else b if PERCENT, else a (both must be UNITLESS at that
// point).
func firstScalar(a, b Unit) Unit {
	if a.Kind == UnitRatio {
		return a
	}
	if b.Kind == UnitRatio {
		return b
	}
	if a.Kind == UnitPercent {
		return a
	}
	if b.Kind == UnitPercent {
		return b
	}
	return a
}

// currencyPerUnit builds the UnitCustom result for Currency / (Count or
// Quantity) — e.g. "USD_PER_COUNT".
func currencyPerUnit(currency, denom Unit) Unit {
	return Unit{Kind: UnitCustom, CustomLabel: currency.CurrencyCode + "_PER_" + string(denom.Kind)}
}
