package inventory

// UOMConversion is an optional, explicit caller-supplied unit-of-measure
// conversion factor — task section 63. This package never infers a
// conversion (e.g. it does not know that 1 "CASE" is 24 "EA" unless told):
// without an explicit UOMConversion covering a From/To pair, quantities in
// different units of measure are never summed — see resolveItemQuantity
// and PortfolioSummary's doc comment.
type UOMConversion struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Factor float64 `json:"factor"`
}

// uomConversionTable indexes UOMConversion rows for O(1) lookup by (From,
// To). Both directions are NOT auto-derived (a caller supplying "CASE" ->
// "EA" does not get "EA" -> "CASE" for free) — task section 63 requires
// explicit conversions only, and auto-deriving the inverse would be an
// inference this package does not make on the caller's behalf.
type uomConversionTable map[[2]string]float64

func buildUOMConversionTable(conversions []UOMConversion) uomConversionTable {
	t := make(uomConversionTable, len(conversions))
	for _, c := range conversions {
		if c.From == "" || c.To == "" || isNonFinite(c.Factor) || c.Factor <= 0 {
			continue // invalid rows are flagged separately by validateUOMConversions.
		}
		t[[2]string{c.From, c.To}] = c.Factor
	}
	return t
}

// convert converts amount from unit "from" to unit "to" using an explicit
// table entry only. Returns (0, false) if from == to (handled by the
// caller as a no-op, not via this table) is not what this checks — convert
// itself only reports whether an explicit conversion row exists; callers
// short-circuit the same-unit case themselves (see resolveItemQuantity).
func (t uomConversionTable) convert(amount float64, from, to string) (float64, bool) {
	factor, ok := t[[2]string{from, to}]
	if !ok {
		return 0, false
	}
	return amount * factor, true
}
